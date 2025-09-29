package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-audio/wav"
	"github.com/robfig/cron/v3"
)

var g711buf []byte

func readWAV() {

	fmt.Println("Reading WAV file...")
	file, err := os.Open(conf.System.AudioFile)
	if err != nil {
		fmt.Println("Error opening wav audio file:", err)
		fmt.Println(err)
	}
	defer file.Close()

	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		fmt.Println("invalid WAV file")
	}

	// 解码
	wavbuf, err := decoder.FullPCMBuffer()
	if err != nil {
		fmt.Println(err)
	}

	// buf 包含 PCM 数据
	log.Printf("Channels: %d\n", wavbuf.Format.NumChannels)
	log.Printf("Sample Rate: %d\n", decoder.SampleRate)
	log.Printf("Bit Depth: %d\n", decoder.BitDepth)
	log.Printf("Number of samples: %d\n", wavbuf.NumFrames())

	if wavbuf.Format.NumChannels != 1 {
		fmt.Println("only mono WAV files are supported")
		return
	}

	if decoder.BitDepth != 16 {
		fmt.Println("only 16-bit WAV files are supported")
		return
	}

	if decoder.SampleRate != 8000 {
		fmt.Println("only 8000 Hz WAV files are supported")
		return
	}

	g711buf = G711Encode(wavbuf.Data)

	startcron()

}

func startcron() {

	c := cron.New()

	//AddFunc

	//AddJob方法
	id1, err := c.AddJob(conf.System.CronString, sendwav{})
	if err != nil {
		log.Println("add notifyspec err", err)
	}

	//启动计划任务
	c.Start()

	//关闭着计划任务, 但是不能关闭已经在执行中的任务.
	//defer c.Stop()
	log.Println("自动发送信标语音启动", id1)

	//SELECT {}
}

type sendwav struct {
}

func (o sendwav) Run() {

	cpuid := calculateCpuId(fmt.Sprintf("%s-250", conf.System.Callsign))

	fmt.Print("信标开始发送.")

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

	fmt.Println("\n信标发送完成")

}
