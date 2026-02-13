package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

var (
	MusicfilenameRegex = regexp.MustCompile(`-(\d{4})\.wav$`)
)

// 全局状态
type MusicQueue struct {
	files []MusicFileInfo
	rnd   *rand.Rand
}

var (
	trackedMusicFiles = make(map[string]MusicFileInfo) // 跟踪所有有效 .wav 文件
	musicstateMu      sync.RWMutex                     // 读写锁
	currentQueue      MusicQueue
	currentPlayingID  int = -1
	musicUpdateChan       = make(chan struct{}, 1) // 用于通知播放器有新文件
	manualNextID          = -1                     // Manually selected next song ID
)

// PlayMusicByID schedules a song to be played immediately
func PlayMusicByID(id int) {
	musicstateMu.Lock()
	manualNextID = id
	musicstateMu.Unlock()

	// Interrupt current playback
	select {
	case nextmusic <- true:
	default:
	}
}

func init() {
	// Initialize random seed
	seed := rand.NewSource(time.Now().UnixNano())
	currentQueue.rnd = rand.New(seed)
}

type MusicFileInfo struct {
	Path string `json:"path"`
	ID   int    `json:"id"`
}

// playAudio 启动调度器
func playMusic() {
	dir := conf.System.MusicFilePath
	if dir == "" {
		log.Println("❌ Music file path not set.")
		return
	}

	if !Exist(conf.System.MusicFilePath) {
		if err := os.MkdirAll(conf.System.AudioFilePath, 0755); err != nil {
			log.Printf("轮播目录 %s 不存在，并且创建失败: %v\n", conf.System.MusicFilePath, err)
			return
		}
	}

	// 1. 首次全量扫描
	fullRescanMusic(dir)

	// 2. 启动每日零点全量重载
	go startDailyFullRescanMusic(dir)

	// 3. 启动文件监听（增量处理）
	go watchMusicFilesIncremental(dir)

	log.Printf("✅ Music scheduler started. Full rescan at midnight, incremental update on change.")

	// 4. 启动播放循环
	playNextMusic()
}

// fullRescan 全量扫描目录，重建 trackedFiles
func fullRescanMusic(dir string) {
	log.Printf("🔄 开始全量扫描目录: %s", dir)

	newTracked := make(map[string]MusicFileInfo)
	var files []MusicFileInfo

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".wav") {
			return nil
		}

		matches := MusicfilenameRegex.FindStringSubmatch(info.Name())
		if matches == nil {
			return nil
		}

		id := mustParseInt(matches[1])

		if id < 0 || id > 9999 {
			log.Printf("⚠️ 无效音乐文件ID: %04d in %s", id, info.Name())
			return nil
		}

		fileInfo := MusicFileInfo{
			Path: path,
			ID:   id,
		}
		newTracked[path] = fileInfo
		files = append(files, fileInfo)
		return nil
	})
	if err != nil {
		log.Printf("❌ 扫描音乐错误: %v", err)
	}

	// 加锁操作状态
	musicstateMu.Lock()

	// 应用新 trackedFiles
	trackedMusicFiles = newTracked

	// 构建播放队列
	buildMusicQueue(files)
	musicstateMu.Unlock()

	// Update web state (outside lock)
	updateMusicList(currentQueue.files, currentPlayingID)
	log.Printf("✅ 音乐文件全量扫描完成. 跟踪 %d 个文件.", len(newTracked))
}

func buildMusicQueue(files []MusicFileInfo) {
	// 排序文件
	sort.Slice(files, func(i, j int) bool {
		return files[i].ID < files[j].ID
	})

	// 构建当前队列
	currentQueue.files = files
	currentPlayingID = -1

	log.Printf("✅ 音乐播放队列已更新 (文件数: %d)", len(files))
}

// 播放下一个音乐
func playNextMusic() {
	// 确保只启动一次，或者通过 context 控制退出。
	// 简单起见，我们用一个 sync.Once 或者假设它只被调用一次。
	// 在 playMusic 中调用它比较合适。
	// 但为了保持兼容性，我们还是放在这里，但要注意调用位置。
	// 修正：原代码在 buildMusicQueue 里调用，确实有问题。
	// 我们改为在 playMusic 中显式调用。

	// 我们改为在 playMusic 中显式调用。

	forcePrevious := false

	for {
		musicstateMu.Lock()

		// 获取当前队列
		queue := currentQueue.files
		if len(queue) == 0 {
			musicstateMu.Unlock()
			log.Println("🎵 没有可播放的音乐文件，等待中...")

			updatePlayStatus("Idle", 0, false)
			// 等待新文件通知或超时
			select {
			case <-musicUpdateChan:
				log.Println("🎵 收到新文件通知，尝试播放...")
			case <-time.After(10 * time.Second):
				// 超时继续检查
			}
			continue
		}

		// 找到下一个要播放的文件
		var nextIndex int = -1
		var minID int = -1
		var foundNext bool = false

		// 0. Check for manual override
		if manualNextID != -1 {
			for i, file := range queue {
				if file.ID == manualNextID {
					nextIndex = i
					minID = file.ID // Not used but keeps consistency
					foundNext = true
					break
				}
			}
			// Reset manual override
			manualNextID = -1
		}

		// 1. 尝试找到比当前 ID 大的最小 ID (Only if manual not found/set)
		if !foundNext {
			if forcePrevious {
				// Try to find Max ID < currentPlayingID
				var maxID int = -1
				for i, file := range queue {
					if currentPlayingID == -1 || file.ID < currentPlayingID {
						// Candidate
						if !foundNext || file.ID > maxID {
							maxID = file.ID
							nextIndex = i
							foundNext = true
						}
					}
				}
				// Reset
				forcePrevious = false
			} else {
				// Determine Next (Min ID > Current)
				for i, file := range queue {
					if currentPlayingID == -1 || file.ID > currentPlayingID {
						// 这是一个候选
						if !foundNext || file.ID < minID {
							minID = file.ID
							nextIndex = i
							foundNext = true
						}
					}
				}
			}
		}

		// 2. 如果没找到（说明当前 ID 已经是最大(Next)或最小(Prev)，或者刚开始）
		// Find wrap around
		if !foundNext {
			minID = -1  // Reuse for Next logic
			maxID := -1 // Use for Prev logic

			if forcePrevious {
				// Find absolute Max ID (Wrap to end)
				for i, file := range queue {
					if !foundNext || file.ID > maxID {
						maxID = file.ID
						nextIndex = i
						foundNext = true
					}
				}
				forcePrevious = false
			} else {
				// Find absolute Min ID (Wrap to start)
				for i, file := range queue {
					if nextIndex == -1 || file.ID < minID {
						minID = file.ID
						nextIndex = i
					}
				}
			}
		}

		if nextIndex == -1 {
			// 理论上不应该发生，除非队列为空（前面已检查）
			musicstateMu.Unlock()
			time.Sleep(5 * time.Second)
			continue
		}

		// 更新当前播放ID
		fileToPlay := queue[nextIndex]
		currentPlayingID = fileToPlay.ID

		// 解锁以执行播放操作
		musicstateMu.Unlock()

		// Update current playing highlight state
		updateMusicList(queue, currentPlayingID)

		updateMusicList(queue, currentPlayingID)

		// Streaming read and play
		f, err := os.Open(fileToPlay.Path)
		if err != nil {
			log.Printf("❌ 无法打开音乐文件: %v", err)
			handleMusicFileRemoved(fileToPlay.Path)
			continue
		}

		decoder := wav.NewDecoder(f)
		if !decoder.IsValidFile() || decoder.Format().NumChannels != 1 || decoder.BitDepth != 16 || decoder.SampleRate != 8000 {
			log.Printf("❌ 无效或不支持的 WAV 格式 (仅支持 8k/16bit/单声道): %s", fileToPlay.Path)
			f.Close()
			handleMusicFileRemoved(fileToPlay.Path)
			continue
		}

		// Get total duration for percentage calculation
		stat, _ := f.Stat()
		fileSize := stat.Size()
		// Simple estimation: (File Size - WAV Header ~44 bytes) / (2 bytes per sample)
		totalSamples := int((fileSize - 44) / 2)
		if totalSamples <= 0 {
			totalSamples = 1 // Prevent division by zero
		}

		playstatus := true
		buf := &audio.IntBuffer{Data: make([]int, 160), Format: decoder.Format()}
		processedSamples := 0
		percent := 0

	tag:
		for {
			n, err := decoder.PCMBuffer(buf)
			if err != nil || n == 0 {
				break tag
			}

			percent = processedSamples * 100 / totalSamples
			if percent > 100 {
				percent = 100
			}

			// Handle controls
			select {
			case <-nextmusic:
				break tag
			case <-pausemusic:
				playstatus = !playstatus
				// Report state change immediately
				updatePlayStatus(fmt.Sprintf("%s: %s (ID: %04d) [%d%%]",
					map[bool]string{true: "Playing", false: "Paused"}[playstatus],
					filepath.Base(fileToPlay.Path), fileToPlay.ID, percent), percent, playstatus)
			case <-lastmusic:
				forcePrevious = true
				break tag
			default:
			}

			if !playstatus {
				time.Sleep(time.Millisecond * 100)
				// We don't continue because we still have the chunk in 'buf'.
				// But PCMBuffer already read it. We need to wait until playstatus is true.
				for !playstatus {
					select {
					case <-nextmusic:
						break tag
					case <-pausemusic:
						playstatus = !playstatus
						// Report state change immediately
						updatePlayStatus(fmt.Sprintf("%s: %s (ID: %04d) [%d%%]",
							map[bool]string{true: "Playing", false: "Paused"}[playstatus],
							filepath.Base(fileToPlay.Path), fileToPlay.ID, percent), percent, playstatus)
					case <-lastmusic:
						forcePrevious = true
						break tag
					default:
						time.Sleep(time.Millisecond * 100)
					}
				}
			}

			// Send chunk to PCM channel
			// Note: data is in buf.Data[:n]
			chunkData := make([]int, n)
			copy(chunkData, buf.Data[:n])
			musicPCM <- [][]int{chunkData}

			processedSamples += n

			// Throttle status updates
			if processedSamples%8000 == 0 { // Every ~1 second
				statusText := fmt.Sprintf("Playing: %s (ID: %04d) [%d%%]", filepath.Base(fileToPlay.Path), fileToPlay.ID, percent)
				updatePlayStatus(statusText, percent, playstatus)
			}
			// Check for next track or exit
			select {
			default:
			}
		}
		f.Close()

		// 稍微暂停一下，避免连续播放太紧凑
		time.Sleep(1 * time.Second)
		time.Sleep(1 * time.Second)
	}
}

// watchFilesIncremental 增量监听文件变化
func watchMusicFilesIncremental(dir string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("❌ 无法创建 watcher:", err)
	}
	defer watcher.Close()

	if err := watcher.Add(dir); err != nil {
		log.Printf("❌ 无法监听目录 %s: %v", dir, err)
		return
	}

	log.Printf("👀 开始增量监听目录: %s", dir)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			path := event.Name
			if !strings.HasSuffix(strings.ToLower(path), ".wav") {
				continue
			}

			switch {

			case event.Has(fsnotify.Create):
				handleMusicFileAdded(path)
			case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
				handleMusicFileRemoved(path)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("⚠️ 音乐文件监听错误:", err)
		}
	}
}

// handleFileAdded 处理新增文件
func handleMusicFileAdded(path string) {
	log.Printf("🟢 文件新增: %s", path)
	matches := MusicfilenameRegex.FindStringSubmatch(filepath.Base(path))
	if matches == nil {
		log.Printf("🟡 跳过非规范命名文件: %s", path)
		return
	}

	id := mustParseInt(matches[1])
	if id < 0 || id > 9999 {
		log.Printf("⚠️ 无效音乐文件ID: %04d in %s", id, path)
		return
	}

	fileInfo := MusicFileInfo{
		Path: path,
		ID:   id,
	}

	musicstateMu.Lock()

	// 更新跟踪列表
	trackedMusicFiles[path] = fileInfo

	// 添加到队列并重新排序
	currentQueue.files = append(currentQueue.files, fileInfo)
	sort.Slice(currentQueue.files, func(i, j int) bool {
		return currentQueue.files[i].ID < currentQueue.files[j].ID
	})

	files := currentQueue.files
	playingID := currentPlayingID
	musicstateMu.Unlock()

	updateMusicList(files, playingID)

	// 通知播放器有新文件（非阻塞发送）
	select {
	case musicUpdateChan <- struct{}{}:
	default:
	}
}

// handleFileRemoved 处理文件删除
func handleMusicFileRemoved(path string) {
	log.Printf("🔴 文件删除: %s", path)

	musicstateMu.Lock()

	// 从 tracked 中移除
	delete(trackedMusicFiles, path)

	// 从队列中移除
	newQueue := make([]MusicFileInfo, 0, len(currentQueue.files))
	for _, file := range currentQueue.files {
		if file.Path != path {
			newQueue = append(newQueue, file)
		}
	}
	currentQueue.files = newQueue
	playingID := currentPlayingID
	musicstateMu.Unlock()

	updateMusicList(newQueue, playingID)
}

// startDailyFullRescanMusic 每天 00:00 执行一次全量重扫
func startDailyFullRescanMusic(dir string) {
	for {
		now := time.Now()
		next := now.Add(24 * time.Hour)
		nextMidnight := time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())
		duration := nextMidnight.Sub(now)

		log.Printf("⏳ 等待到明日零点进行音乐全量重载: %v 后", duration.Round(time.Second))

		time.Sleep(duration)

		// 触发全量重扫
		fullRescanMusic(dir)
	}
}
