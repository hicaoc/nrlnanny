package main

import (
	"log"
	"os"

	"github.com/go-audio/wav"
)

// var g711buf []byte

// var lastAudioFileModTime time.Time

func readWAV(filepath string) []int {

	_, err := os.Stat(filepath)
	if err != nil {
		log.Println("信标文件打开错误：", err)
		return nil
	}

	// 获取修改时间
	//modTime := fileInfo.ModTime()

	// if modTime.Equal(lastAudioFileModTime) {
	// 	fmt.Println("文件 " + conf.System.AudioFile + " 未变化，无需重新加载,直接使用上次加载的文件")
	// 	return g711buf
	// }

	// lastAudioFileModTime = modTime

	file, err := os.Open(filepath)
	if err != nil {
		log.Println("Error opening wav audio file:", err)
		log.Println(err)
		return nil
	}
	defer file.Close()

	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		log.Println("invalid WAV file", filepath)
		return nil
	}

	// 解码
	wavbuf, err := decoder.FullPCMBuffer()
	if err != nil || wavbuf == nil {
		log.Println(err)
		return nil
	}

	// buf 包含 PCM 数据
	log.Printf("读取%s完成，Channels: %d, Sample Rate: %d, Bit Depth: %d, Number of samples: %d\n", filepath, wavbuf.Format.NumChannels, decoder.SampleRate, decoder.BitDepth, wavbuf.NumFrames())

	if wavbuf.Format.NumChannels != 1 {
		log.Println("only mono WAV files are supported")
		return nil
	}

	if decoder.BitDepth != 16 {
		log.Println("only 16-bit WAV files are supported")
		return nil
	}

	if decoder.SampleRate != 8000 {
		log.Println("only 8000 Hz WAV files are supported")
		return nil
	}

	return wavbuf.Data

}
