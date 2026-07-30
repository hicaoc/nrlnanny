package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
)

func ReadMP3() {
	pcm, err := decodeAudioFile(conf.System.AudioFile)
	if err != nil {
		log.Printf("读取 MP3 文件失败 %s: %v", conf.System.AudioFile, err)
		return
	}
	for i := 0; i < len(pcm); i += opusFrameSamples {
		end := min(i+opusFrameSamples, len(pcm))
		chunk := make([]int, opusFrameSamples)
		copy(chunk, pcm[i:end])
		cronPCM <- [][]int{chunk}
	}
}

// 辅助函数：保存 PCM 数据为 WAV 文件

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// BytesToInt16 converts a []byte slice (representing 16-bit PCM samples)
// into a []int16 slice. It assumes LittleEndian byte order.
func BytesToInt16(byteData []byte) ([]int16, error) {
	if len(byteData)%2 != 0 {
		return nil, fmt.Errorf("byte data length must be even to represent int16 samples, got %d", len(byteData))
	}

	// 计算将有多少个 int16 样本
	numSamples := len(byteData) / 2
	int16Samples := make([]int16, numSamples)

	// 创建一个 bytes.Buffer 来读取字节数据
	buf := bytes.NewReader(byteData)

	// 使用 binary.Read 将字节数据解析为 int16
	// 大多数音频文件（如 WAV）使用 LittleEndian
	err := binary.Read(buf, binary.LittleEndian, int16Samples)
	if err != nil {
		return nil, fmt.Errorf("failed to read bytes into int16 slice: %w", err)
	}

	return int16Samples, nil
}
