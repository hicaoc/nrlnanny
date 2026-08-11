package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type TTScheduledTask struct {
	ID        int64   `json:"id"`
	Time      string  `json:"time"`
	Repeat    bool    `json:"repeat"`
	SendToNRL bool    `json:"sendToNRL"`
	Text      string  `json:"text"`
	Enabled   bool    `json:"enabled"`
	Voice     string  `json:"voice"`
	Rate      float64 `json:"rate"`
	Pitch     float64 `json:"pitch"`
}

var (
	ttsTasks     []TTScheduledTask
	ttsTasksLock sync.Mutex
	ttsTimerStop chan struct{}
)

func ttsTasksFilePath() string {
	return filepath.Join(conf.System.RecoderFilePath, "tts_tasks.json")
}

func loadTTSTasks() {
	ttsTasksLock.Lock()
	defer ttsTasksLock.Unlock()

	data, err := os.ReadFile(ttsTasksFilePath())
	if err != nil {
		ttsTasks = []TTScheduledTask{}
		return
	}

	if err := json.Unmarshal(data, &ttsTasks); err != nil {
		log.Printf("加载TTS任务失败: %v", err)
		ttsTasks = []TTScheduledTask{}
	}
	log.Printf("已加载 %d 个TTS定时任务", len(ttsTasks))
}

func saveTTSTasks() {
	ttsTasksLock.Lock()
	data, err := json.MarshalIndent(ttsTasks, "", "  ")
	ttsTasksLock.Unlock()
	if err != nil {
		log.Printf("序列化TTS任务失败: %v", err)
		return
	}
	if err := os.WriteFile(ttsTasksFilePath(), data, 0644); err != nil {
		log.Printf("保存TTS任务失败: %v", err)
	}
}

func startTTSTaskScheduler() {
	loadTTSTasks()
	ttsTimerStop = make(chan struct{})

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		executed := make(map[int64]string)

		// 立即执行一次检查
		checkAndExecute(executed)

		for {
			select {
			case <-ttsTimerStop:
				return
			case <-ticker.C:
				checkAndExecute(executed)
			}
		}
	}()

	log.Printf("TTS任务调度器已启动")
}

func checkAndExecute(executed map[int64]string) {
	ttsTasksLock.Lock()
	tasks := make([]TTScheduledTask, len(ttsTasks))
	copy(tasks, ttsTasks)
	ttsTasksLock.Unlock()

	now := time.Now()
	currentTime := now.Format("15:04")
	today := now.Format("2006-01-02")

	for i := range tasks {
		task := &tasks[i]
		if !task.Enabled {
			continue
		}

		if task.Time != currentTime {
			continue
		}

		key := task.Time + today
		if executed[task.ID] == key {
			continue
		}

		executed[task.ID] = key

		log.Printf("执行TTS定时任务 [%d]: %s, text: %s", task.ID, task.Time, task.Text)

		voice := task.Voice
		if voice == "" {
			voice = "zh-CN-XiaoxiaoNeural"
		}
		rate := task.Rate
		if rate == 0 {
			rate = 1.0
		}
		pitch := task.Pitch
		if pitch == 0 {
			pitch = 1.0
		}

		if task.SendToNRL {
			go func(t TTScheduledTask) {
				if err := sendTTSVoice(t.Text, voice, rate, pitch); err != nil {
					log.Printf("TTS定时任务发送失败: %v", err)
				}
			}(*task)
		}

		if !task.Repeat {
			ttsTasksLock.Lock()
			for i := range ttsTasks {
				if ttsTasks[i].ID == task.ID {
					ttsTasks[i].Enabled = false
					break
				}
			}
			ttsTasksLock.Unlock()
			saveTTSTasks()
		}
	}
}
