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

	if conf.System.AudioFile != "" && conf.System.CronString != "" {
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

	//关闭着计划任务, 但是不能关闭已经在执行中的任务.
	//defer c.Stop()
	log.Println("自动发送信标语音功能启动", id1)

	//SELECT {}
}

func (o sendvoice) Run() {

	log.Print("读取信标文件，准备播放信标...")

	switch strings.ToLower(filepath.Ext(conf.System.AudioFile)) {
	case ".wav":
		readWAV()

	case ".mp3":
		ReadMP3()
	}

	cpuid := calculateCpuId(fmt.Sprintf("%s-250", conf.System.Callsign))

	log.Print("信标文件加载完成，信标开始发送.")

	for i := 0; i < len(g711buf); i += 500 {

		if i+500 > len(g711buf) {
			break
		}

		packet := encodeNRL21(conf.System.Callsign, conf.System.SSID, 1, 250, cpuid, g711buf[i:i+500])
		dev.udpSocket.Write(packet)

		//log.Printf("Sample send ... %d \n", i) // At(sampleIdx, channel)

		time.Sleep(time.Microsecond * 62500)
		fmt.Print(".")

	}

	log.Println("\n信标发送完成")

}
