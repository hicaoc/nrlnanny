package main

import (
	"log"
)

// var g711buf []byte

// var lastAudioFileModTime time.Time

func readWAV(filepath string) []int {
	pcm, err := decodeAudioFile(filepath)
	if err != nil {
		log.Printf("读取 WAV 文件失败 %s: %v", filepath, err)
		return nil
	}
	return pcm
}
