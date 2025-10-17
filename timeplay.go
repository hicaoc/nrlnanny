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
	filenameRegex = regexp.MustCompile(`-(\d{2})(\d{2})\.wav$`)

	scheduledTimers []timerEntry // 新增：保存所有活跃的 timer
	timersMu        sync.Mutex   // 保护并发访问

	//playMu sync.Mutex // 保护播放器
)

type timerEntry struct {
	time  time.Time
	file  AudioFileInfo
	timer *time.Timer
}

type AudioFileInfo struct {
	Path   string
	Hour   int
	Minute int
}

func playAudio() {

	if conf.System.AudioFilePath == "" {
		log.Println("Audio file path is not set. Skipping audio playback.")
		return
	}

	// 首次扫描
	scanAndReschedule(conf.System.AudioFilePath)

	go watchFiles()

	go startDailyReload(conf.System.AudioFilePath)

	// // 扫描间隔（可配置）
	// scanInterval := 1 * time.Hour

	// log.Printf("✅ Audio scheduler started. Scanning every %v...", scanInterval)

	// ticker := time.NewTicker(scanInterval)
	// defer ticker.Stop()

	// // 定期扫描
	// for range ticker.C {
	// 	scanAndReschedule(conf.System.AudioFilePath)
	// }
}

// scanAndSchedule 扫描目录，为今天未过时间的文件安排播放
func scanAndReschedule(dir string) {
	now := time.Now()
	log.Printf("🔄 扫描轮播目录: %s", dir)

	// 🔒 加锁操作定时器列表
	timersMu.Lock()

	// Step 1: 停止并清空所有之前的定时器
	for _, entry := range scheduledTimers {
		if entry.timer != nil {
			entry.timer.Stop()
		}
	}
	scheduledTimers = nil // 清空旧计划

	timersMu.Unlock()

	// Step 2: 扫描新文件
	files, err := scanFiles(dir)
	if err != nil {
		log.Printf("❌ 扫描错误: %v", err)
		return
	}

	if len(files) == 0 {
		log.Printf("🟡 没有找到可以播放的文件.")
		return
	}

	var scheduledCount int

	for _, f := range files {
		file := f
		playTime := time.Date(now.Year(), now.Month(), now.Day(), file.Hour, file.Minute, 0, 0, now.Location())

		if playTime.Before(now) || playTime.Equal(now) {
			continue // 已过，跳过
		}

		duration := playTime.Sub(now)

		// 创建定时器
		timer := time.AfterFunc(duration, func() {
			sendG711(readWAV(file.Path))
		})

		// 保存记录以便后续取消
		timersMu.Lock()
		scheduledTimers = append(scheduledTimers, timerEntry{
			time:  playTime,
			file:  file,
			timer: timer,
		})
		timersMu.Unlock()

		log.Printf("⏰ Scheduled: %s for %s (%v from now)",
			filepath.Base(file.Path),
			playTime.Format("15:04:05"),
			duration.Round(time.Second))
		scheduledCount++
	}

	log.Printf("✅ 扫描完成. 共%d个文件今天要播放.", scheduledCount)
}

// scanFiles 扫描目录，返回有效文件列表
func scanFiles(dir string) ([]AudioFileInfo, error) {
	var files []AudioFileInfo
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		matches := filenameRegex.FindStringSubmatch(name)
		if matches != nil {
			hour := mustParseInt(matches[1])
			minute := mustParseInt(matches[2])
			files = append(files, AudioFileInfo{
				Path:   path,
				Hour:   hour,
				Minute: minute,
			})
		}
		return nil
	})
	return files, err
}

func mustParseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

func watchFiles() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)

	go func() {
		for {
			select {
			case event := <-watcher.Events:

				if event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
					log.Println("event:", event)
					time.Sleep(time.Second * 5)
					scanAndReschedule(conf.System.AudioFilePath)

				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(conf.System.AudioFilePath)
	if err != nil {
		log.Fatal(err)
	}
	<-done
	time.Sleep(time.Second)
	log.Println("watching...")

}

func startDailyReload(dir string) {
	go func() {
		for {
			now := time.Now()
			// 计算到明天零点的时间
			next := now.Add(24 * time.Hour)
			nextMidnight := time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())
			duration := nextMidnight.Sub(now)

			log.Printf("⏳ 等待到明天零点重新加载音频任务: %v 后", duration.Round(time.Second))

			// 等待到零点
			time.Sleep(duration)

			// 触发重新调度（清空旧任务，加载新一天任务）
			scanAndReschedule(dir)
		}
	}()
}
