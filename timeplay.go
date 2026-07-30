package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

var (
	filenameRegex = regexp.MustCompile(`(?i)-(\d{2})(\d{2})\.(wav|mp3|flac)$`)
)

type AudioFileInfo struct {
	Path   string
	Hour   int
	Minute int
}

// 全局状态（建议后续封装成 Scheduler 结构体）
var (
	trackedFiles   = make(map[string]AudioFileInfo) // 跟踪所有支持的音频文件
	scheduledTasks = make(map[string]*time.Timer)   // 文件路径 -> Timer（用于取消）
	stateMu        sync.RWMutex                     // 读写锁
)

// playAudio 启动调度器
func playAudio() {
	dir := conf.System.AudioFilePath
	if dir == "" {
		log.Println("❌ Audio file path not set.")
		return
	}

	if !Exist(conf.System.AudioFilePath) {
		if err := os.MkdirAll(conf.System.AudioFilePath, 0755); err != nil {
			log.Printf("轮播目录 %s 不存在，并且创建失败: %v\n", conf.System.AudioFilePath, err)
			return
		}
	}

	// 1. 首次全量扫描
	fullRescan(dir)

	// 2. 启动每日零点全量重载
	go startDailyFullRescan(dir)

	// 3. 启动文件监听（增量处理）
	go watchFilesIncremental(dir)

	// 4. 监听开关变化
	go func() {
		for range timeToggleChan {
			if !isTimeEnabled() {
				clearTimeSchedules()
			} else {
				fullRescan(dir)
			}
		}
	}()

	log.Printf("✅ Audio scheduler started. Full rescan at midnight, incremental update on change.")
}

// fullRescan 全量扫描目录，重建 trackedFiles 和 scheduledTasks
func fullRescan(dir string) {
	log.Printf("🔄 开始全量扫描目录: %s", dir)
	if !isTimeEnabled() {
		clearTimeSchedules()
		return
	}

	now := time.Now()

	newTracked := make(map[string]AudioFileInfo)
	var added []AudioFileInfo

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !isSupportedAudioFile(info.Name()) {
			return nil
		}

		matches := filenameRegex.FindStringSubmatch(info.Name())
		if matches == nil {
			return nil
		}

		hour := mustParseInt(matches[1])
		minute := mustParseInt(matches[2])
		if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
			log.Printf("⚠️ 无效音频播放时间 %02d:%02d in %s", hour, minute, info.Name())
			return nil
		}

		fileInfo := AudioFileInfo{
			Path:   path,
			Hour:   hour,
			Minute: minute,
		}
		newTracked[path] = fileInfo

		// 检查是否是今天且未过时间
		playTime := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if playTime.After(now) {
			added = append(added, fileInfo)
		}

		return nil
	})
	if err != nil {
		log.Printf("❌ 扫描音频文件错误: %v", err)
	}

	// 加锁操作状态
	stateMu.Lock()
	defer stateMu.Unlock()

	// 停止所有旧任务
	for _, timer := range scheduledTasks {
		timer.Stop()
	}
	scheduledTasks = make(map[string]*time.Timer)

	// 应用新 trackedFiles
	trackedFiles = newTracked

	// 重新安排今天的任务
	for _, file := range added {
		playTime := time.Date(now.Year(), now.Month(), now.Day(), file.Hour, file.Minute, 0, 0, now.Location())
		duration := playTime.Sub(now)

		timer := time.AfterFunc(duration, func() {
			if !isTimeEnabled() {
				return
			}
			playScheduledAudio(file.Path)
		})

		scheduledTasks[file.Path] = timer

		log.Printf("⏰ Scheduled (full): %s for %s (%v)",
			filepath.Base(file.Path),
			playTime.Format("15:04:05"),
			duration.Round(time.Second))
	}

	log.Printf("✅ 全量扫描完成. 跟踪 %d 个文件，安排 %d 个今日播放任务.", len(trackedFiles), len(scheduledTasks))
	updateScheduleList(trackedFiles)
}

// watchFilesIncremental 增量监听文件变化
func watchFilesIncremental(dir string) {
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
			if !isSupportedAudioFile(path) {
				continue
			}

			switch {

			case event.Has(fsnotify.Create):
				handleFileAdded(path)
			case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
				handleFileRemoved(path)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("⚠️ 监听错误:", err)
		}
	}
}

// handleFileAdded 处理新增文件（只处理今天未来的）
func handleFileAdded(path string) {
	log.Printf("🟢 文件新增: %s", path)
	if !isTimeEnabled() {
		return
	}
	matches := filenameRegex.FindStringSubmatch(filepath.Base(path))
	if matches == nil {
		log.Printf("🟡 跳过非规范命名文件: %s", path)
		return
	}

	hour := mustParseInt(matches[1])
	minute := mustParseInt(matches[2])
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		log.Printf("⚠️ 无效时间 %02d:%02d in %s", hour, minute, path)
		return
	}

	fileInfo := AudioFileInfo{Path: path, Hour: hour, Minute: minute}

	now := time.Now()
	playTime := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())

	// 只安排今天未来的任务
	if playTime.Before(now) || playTime.Equal(now) {
		log.Printf("🕒 已过播放时间，跳过: %s", path)
		return
	}

	stateMu.Lock()
	defer stateMu.Unlock()

	// 记录到跟踪列表
	trackedFiles[path] = fileInfo

	// 设置定时器
	duration := playTime.Sub(now)
	timer := time.AfterFunc(duration, func() {
		if !isTimeEnabled() {
			return
		}
		playScheduledAudio(path)
	})

	scheduledTasks[path] = timer

	log.Printf("⏰ Scheduled (add): %s for %s (%v from now)",
		filepath.Base(path),
		playTime.Format("15:04:05"),
		duration.Round(time.Second))

	updateScheduleList(trackedFiles)
}

// handleFileRemoved 处理文件删除
func handleFileRemoved(path string) {
	log.Printf("🔴 文件删除: %s", path)

	stateMu.Lock()
	defer stateMu.Unlock()

	// 从 tracked 中移除
	delete(trackedFiles, path)

	// 停止定时器
	if timer, exists := scheduledTasks[path]; exists {
		timer.Stop()
		delete(scheduledTasks, path)
		log.Printf("🛑 已取消播放任务: %s", path)
	}
	updateScheduleList(trackedFiles)
}

// startDailyFullRescan 每天 00:00 执行一次全量重扫
func startDailyFullRescan(dir string) {
	for {
		now := time.Now()
		next := now.Add(24 * time.Hour)
		nextMidnight := time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())
		duration := nextMidnight.Sub(now)

		log.Printf("⏳ 等待到明日零点进行全量重载: %v 后", duration.Round(time.Second))

		time.Sleep(duration)

		// 触发全量重扫（自动清理旧任务）
		if isTimeEnabled() {
			fullRescan(dir)
		}
	}
}

func clearTimeSchedules() {
	stateMu.Lock()
	defer stateMu.Unlock()
	for _, timer := range scheduledTasks {
		timer.Stop()
	}
	scheduledTasks = make(map[string]*time.Timer)
	updateScheduleList(trackedFiles)
}

func playScheduledAudio(path string) {
	pcm, err := decodeAudioFile(path)
	if err != nil {
		log.Printf("❌ 无法解码定时音频 %s: %v", path, err)
		return
	}
	for i := 0; i < len(pcm); i += opusFrameSamples {
		if !isTimeEnabled() {
			return
		}
		end := min(i+opusFrameSamples, len(pcm))
		chunk := make([]int, opusFrameSamples)
		copy(chunk, pcm[i:end])
		timePCM <- [][]int{chunk}
		percent := end * 100 / len(pcm)
		updatePlayStatus(fmt.Sprintf("Scheduled Play: %s [%d%%]", path, percent), percent, true)
	}
}

// 辅助函数
func mustParseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}
