package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

type sendvoice struct {
}

func startcron() {

	if conf.System.AudioFile == "" || conf.System.CronString == "" {
		log.Println("未启动自动发送信标语音功能，因为没有配置音频文件路径或者调度字符串没有配置")
		return
	}

	c := cron.New()

	//AddFunc

	//AddJob方法
	id1, err := c.AddJob(conf.System.CronString, sendvoice{})
	if err != nil {
		log.Println("add notifyspec err", err)
	}

	//启动计划任务
	c.Start()

	// Update TUI periodically
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			entries := c.Entries()
			if len(entries) > 0 {
				// Find next run time
				var next time.Time
				found := false
				for _, e := range entries {
					if !found || e.Next.Before(next) {
						next = e.Next
						found = true
					}
				}

				if found {
					updateCronInfo(fmt.Sprintf("Next Beacon: %s (in %v)", next.Format("15:04:05"), time.Until(next).Round(time.Second)))
				}
			}
		}
	}()

	//关闭着计划任务, 但是不能关闭已经在执行中的任务.
	//defer c.Stop()
	log.Println("自动发送信标语音功能启动", id1)

	//SELECT {}
}

func (o sendvoice) Run() {

	log.Printf("\n读取信标文件，准备播放信标...%s\n", conf.System.AudioFile)
	updatePlayStatus("Beacon Playing...", 0, true)

	switch strings.ToLower(filepath.Ext(conf.System.AudioFile)) {
	case ".wav":

		wav := readWAV(conf.System.AudioFile)
		// pcmbuff := make([][]int, 1) // 移除外部定义，避免并发竞争

		for i := 0; i < len(wav); i += 160 {
			if i+160 < len(wav) {
				// 每次创建新的切片结构，防止引用被覆盖
				data := [][]int{wav[i : i+160]}
				cronPCM <- data
			}
		}

		//sendG711(readWAV(conf.System.AudioFile))

	case ".mp3":
		ReadMP3()
	}

}

func recivePCM() {
	ticket := time.NewTicker(time.Microsecond * 20000) // 20ms
	defer ticket.Stop()

	pcmbuf := make([]int, 160)

	// 标记是否有麦克风或信标活动
	var hasBeaconActivity bool
	volumeScale := 1.0

	for range ticket.C {
		// 1. 每一帧开始前必须重置缓冲区为静音，防止残留
		for i := range pcmbuf {
			pcmbuf[i] = 0
		}

		hasBeaconActivity = false

		// 2. 混音: cronPCM (信标)
		select {
		case wav := <-cronPCM:
			hasBeaconActivity = true
			for i, v := range wav[0] {
				if i < len(pcmbuf) {
					pcmbuf[i] += v
				}
			}
		default:
		}

		// 3. 混音: timePCM
		select {
		case wav := <-timePCM:
			hasBeaconActivity = true
			for i, v := range wav[0] {
				if i < len(pcmbuf) {
					pcmbuf[i] += v
				}
			}
		default:
		}

		// 4. 混音: musicPCM
		select {
		case wav := <-musicPCM:
			// 计算音乐音量缩放因子
			// 如果有麦克风或信标活动，降低音乐音量
			volumeScale = 1.0
			if hasBeaconActivity && conf.System.DuckMusicPCM {
				volumeScale = conf.System.DuckScale // 降低一个维度
			}

			for i, v := range wav[0] {
				if i < len(pcmbuf) {
					// 应用音量缩放
					pcmbuf[i] += int(float64(v) * volumeScale)
				}
			}
		default:
		}

		// 5. 混音: micPCM
		select {
		case wav := <-micPCM:
			volumeScale = 1.0
			if hasBeaconActivity && conf.System.DuckMicPCM {
				volumeScale = conf.System.DuckScale // 降低一个维度
			}
			for i, v := range wav[0] {
				if i < len(pcmbuf) {
					pcmbuf[i] += int(float64(v) * volumeScale)
				}
			}
		default:
		}

		// 6. 静音检测
		isSilence := true
		for _, v := range pcmbuf {
			if v != 0 {
				isSilence = false
				break
			}
		}

		if !isSilence {
			packet := encodeNRL21(conf.System.Callsign, conf.System.SSID, 1, 250, cpuid, G711Encode(pcmbuf))
			dev.udpSocket.Write(packet)
		}

	}
}
