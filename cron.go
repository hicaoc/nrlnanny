package main

import (
	"fmt"
	"log"
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

	// Update web status periodically
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if !isCronEnabled() {
				updateCronInfo("Cron Disabled")
				continue
			}
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
	if !isCronEnabled() {
		return
	}

	log.Printf("\n读取信标文件，准备播放信标...%s\n", conf.System.AudioFile)
	updatePlayStatus("Beacon Playing...", 0, true)

	pcm, err := decodeAudioFile(conf.System.AudioFile)
	if err != nil {
		log.Printf("读取信标音频失败: %v", err)
		updatePlayStatus("Beacon decode failed", 0, false)
		return
	}
	for i := 0; i < len(pcm); i += opusFrameSamples {
		if !isCronEnabled() {
			return
		}
		end := min(i+opusFrameSamples, len(pcm))
		chunk := make([]int, opusFrameSamples)
		copy(chunk, pcm[i:end])
		cronPCM <- [][]int{chunk}
	}
}

func recivePCM() {
	ticket := time.NewTicker(time.Microsecond * 20000) // 20ms
	defer ticket.Stop()

	// 标记是否有麦克风或信标活动
	var hasBeaconActivity bool
	volumeScale := 1.0
	wasSending := false
	lastOpusMode := false
	pcm8 := make([]int, 160)
	pcm16 := make([]int, opusFrameSamples)

	for range ticket.C {
		sendOpus := isSendOpusEnabled()
		pcmbuf := pcm8
		if sendOpus {
			pcmbuf = pcm16
		}
		clear(pcmbuf)

		hasBeaconActivity = false

		// 2. 混音: cronPCM (信标)
		select {
		case wav := <-cronPCM:
			hasBeaconActivity = true
			mix16KSource(pcmbuf, wav[0], 1, sendOpus)
		default:
		}

		// 3. 混音: timePCM
		select {
		case wav := <-timePCM:
			hasBeaconActivity = true
			mix16KSource(pcmbuf, wav[0], 1, sendOpus)
		default:
		}

		// 4. 混音: 本地音乐或网络电台（网络电台优先，两个节目不会同时发送）
		musicSource := musicPCM
		if isRadioPlaying() {
			musicSource = radioPCM
		}
		select {
		case wav := <-musicSource:
			// 计算音乐音量缩放因子
			// 如果有麦克风或信标活动，降低音乐音量
			volumeScale = 1.0
			if hasBeaconActivity && conf.System.DuckMusicPCM {
				volumeScale = conf.System.DuckScale // 降低一个维度
			}

			mix16KSource(pcmbuf, wav[0], volumeScale, sendOpus)
		default:
		}

		// 5. 混音: micPCM
		select {
		case wav := <-micPCM:
			volumeScale = 1.0
			if hasBeaconActivity && conf.System.DuckMicPCM {
				volumeScale = conf.System.DuckScale // 降低一个维度
			}
			mix16KSource(pcmbuf, wav[0], volumeScale, sendOpus)
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
			if dev == nil || dev.udpSocket == nil {
				wasSending = false
				continue
			}
			var packet []byte
			if sendOpus {
				opusData, err := encodeOpusVoice(intsToInt16WithVolume(pcmbuf, conf.System.Volume), !wasSending || !lastOpusMode)
				if err != nil {
					log.Printf("Opus encode failed: %v", err)
					wasSending = false
					continue
				}
				packet = encodeNRL21(conf.System.Callsign, conf.System.SSID, 8, 250, cpuid, opusData)
			} else {
				packet = encodeNRL21(conf.System.Callsign, conf.System.SSID, 1, 250, cpuid, G711Encode(pcmbuf))
			}
			if _, err := dev.udpSocket.Write(packet); err != nil {
				log.Printf("send voice failed: %v", err)
			}
			wasSending = true
			lastOpusMode = sendOpus
		} else {
			wasSending = false
		}

	}
}

func mix16KSource(dst, source []int, scale float64, targetOpus bool) {
	if targetOpus {
		mixPCMSource(dst, source, scale)
		return
	}
	limit := len(source) / 2
	if limit > len(dst) {
		limit = len(dst)
	}
	for i := 0; i < limit; i++ {
		sample := (source[i*2] + source[i*2+1]) / 2
		dst[i] += int(float64(sample) * scale)
	}
}

func mixPCMSource(dst, source []int, scale float64) {
	limit := len(source)
	if limit > len(dst) {
		limit = len(dst)
	}
	for i := 0; i < limit; i++ {
		dst[i] += int(float64(source[i]) * scale)
	}
}
