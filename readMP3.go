package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/hajimehoshi/go-mp3"
)

func ReadMP3() {
	file, err := os.Open(conf.System.AudioFile)
	if err != nil {
		fmt.Println("Error opening mp3 audio file:", err)
		panic(err)
	}
	defer file.Close()

	// 创建 MP3 解码器
	decoder, err := mp3.NewDecoder(file)
	if err != nil {
		fmt.Println("Error creating mp3 decoder:", err)
		panic(err)
	}

	fmt.Printf("Sample Rate: %d Hz\n", decoder.SampleRate())

	fmt.Printf("Length (in samples): %d\n", decoder.Length())

	// 准备缓冲区来存储 PCM 数据
	const bufferSize = 4096
	pcmBuf := make([]byte, bufferSize)

	// 创建一个切片来保存所有 PCM 样本（可选：用于保存或写入 WAV）
	var allSamples []byte

	// 循环读取 PCM 数据
	for {
		n, err := decoder.Read(pcmBuf)
		if n > 0 {
			// 复制已读取的样本
			allSamples = append(allSamples, pcmBuf[:n]...)
		}
		if err != nil {
			break // 包括 io.EOF 在内的错误都表示结束
		}
	}

	pcmSamples, err := BytesToInt16(allSamples)
	if err != nil {
		fmt.Println("Error converting bytes to int16:", err, len(allSamples))
	}

	fmt.Printf("总共解码了 %d 个 PCM 样本\n", len(allSamples))

	// 示例：打印前 10 个样本
	fmt.Printf("前 10 个 PCM 样本: ")
	for i := 0; i < min(10, len(pcmSamples)); i++ {
		fmt.Printf("%d ", allSamples[i])
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
