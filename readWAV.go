package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-audio/wav"
)

var g711buf []byte

var lastAudioFileModTime time.Time

func readWAV() {

	fileInfo, err := os.Stat(conf.System.AudioFile)
	if err != nil {
		log.Fatal(err)
	}

	// 获取修改时间
	modTime := fileInfo.ModTime()

	if modTime.Equal(lastAudioFileModTime) {
		fmt.Println("文件 " + conf.System.AudioFile + " 未变化，无需重新加载,直接使用上次加载的文件")
		return
	}

	lastAudioFileModTime = modTime

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

}
