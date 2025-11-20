package main

import (
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
)

func init() {
	// Initialize random seed
	seed := rand.NewSource(time.Now().UnixNano())
	currentQueue.rnd = rand.New(seed)
}

type MusicFileInfo struct {
	Path string
	ID   int
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
	defer musicstateMu.Unlock()

	// 应用新 trackedFiles
	trackedMusicFiles = newTracked

	// 构建播放队列
	buildMusicQueue(files)

	log.Printf("✅ 音乐文件全量扫描完成. 跟踪 %d 个文件.", len(trackedMusicFiles))
}

func buildMusicQueue(files []MusicFileInfo) {
	// 排序文件
	sort.Slice(files, func(i, j int) bool {
		return files[i].ID < files[j].ID
	})

	// 构建当前队列
	currentQueue.files = files
	currentPlayingID = -1

	// 启动播放器 (如果尚未启动，这里假设 playNextMusic 是单独启动的 goroutine，或者由调用者启动)
	// 注意：buildMusicQueue 被 fullRescanMusic 调用，而 fullRescanMusic 被 playMusic 调用。
	// playMusic 只调用一次 fullRescanMusic，然后启动 playNextMusic。
	// 为了避免重复启动 playNextMusic，我们这里只更新队列。
	// 实际上 playNextMusic 是一个死循环，它会读取 currentQueue。
	// 所以这里不需要再次 go playNextMusic()，除非是第一次。
	// 但为了简单起见，我们在 playMusic 中并没有显式调用 playNextMusic。
	// 让我们检查一下原代码... 原代码在 buildMusicQueue 里调用了 go playNextMusic()。
	// 这会导致每次全量扫描都启动一个新的播放循环，这是个 BUG！
	// 我们应该只在 playMusic 中启动一次 playNextMusic。
}

// 播放下一个音乐
func playNextMusic() {
	// 确保只启动一次，或者通过 context 控制退出。
	// 简单起见，我们用一个 sync.Once 或者假设它只被调用一次。
	// 在 playMusic 中调用它比较合适。
	// 但为了保持兼容性，我们还是放在这里，但要注意调用位置。
	// 修正：原代码在 buildMusicQueue 里调用，确实有问题。
	// 我们改为在 playMusic 中显式调用。

	for {
		musicstateMu.Lock()

		// 获取当前队列
		queue := currentQueue.files
		if len(queue) == 0 {
			musicstateMu.Unlock()
			log.Println("🎵 没有可播放的音乐文件，等待中...")

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

		// 1. 尝试找到比当前 ID 大的最小 ID
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

		// 2. 如果没找到（说明当前 ID 已经是最大，或者刚开始），找整个队列最小的 ID（循环）
		if !foundNext {
			for i, file := range queue {
				if nextIndex == -1 || file.ID < minID {
					minID = file.ID
					nextIndex = i
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

		// 播放文件
		log.Printf("🎵 正在播放: %s (ID: %04d)", fileToPlay.Path, fileToPlay.ID)

		data := readWAV(fileToPlay.Path)
		if data != nil {
			sendG711(data)
		} else {
			log.Printf("❌ 读取文件失败，从队列中移除: %s", fileToPlay.Path)
			handleMusicFileRemoved(fileToPlay.Path)
			time.Sleep(1 * time.Second) // 避免失败死循环过快
		}

		// 稍微暂停一下，避免连续播放太紧凑
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
	defer musicstateMu.Unlock()

	// 更新跟踪列表
	trackedMusicFiles[path] = fileInfo

	// 添加到队列并重新排序
	currentQueue.files = append(currentQueue.files, fileInfo)
	sort.Slice(currentQueue.files, func(i, j int) bool {
		return currentQueue.files[i].ID < currentQueue.files[j].ID
	})

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
	defer musicstateMu.Unlock()

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
