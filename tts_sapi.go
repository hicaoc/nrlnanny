//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func sendTTSVoice(text, voiceName string, rate, pitch float64) error {
	log.Printf("开始TTS合成: %s, voice: %s", text, voiceName)

	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, fmt.Sprintf("nrlnanny_tts_%d_%d.wav", os.Getpid(), time.Now().UnixNano()))
	defer os.Remove(tempFile)

	ratePercent := int((rate - 1) * 100)

	escapedText := strings.ReplaceAll(text, "'", "''")
	escapedVoice := strings.ReplaceAll(voiceName, "'", "''")
	escapedFile := strings.ReplaceAll(tempFile, "'", "''")

	psScript := fmt.Sprintf(`
Add-Type -AssemblyName System.Speech
$synth = New-Object System.Speech.Synthesis.SpeechSynthesizer
$synth.Rate = %d
$synth.Volume = 100
$voices = $synth.GetInstalledVoices()
$selected = $false
foreach ($v in $voices) {
    $info = $v.VoiceInfo
    if ($info.Name -eq '%s' -or $info.Description -match '%s') {
        $synth.SelectVoice($info.Name)
        $selected = $true
        break
    }
}
if (-not $selected) {
    try { $synth.SelectVoice('%s') } catch {}
}
$synth.SetOutputToWaveFile('%s')
$synth.Speak('%s')
$synth.Dispose()
`, ratePercent, escapedVoice, escapedVoice, escapedVoice, escapedFile, escapedText)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("PowerShell TTS输出: %s", string(output))
		return fmt.Errorf("PowerShell TTS失败: %v", err)
	}

	f, err := os.Open(tempFile)
	if err != nil {
		return fmt.Errorf("打开临时WAV文件失败: %v", err)
	}
	defer f.Close()

	fileInfo, err := f.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %v", err)
	}
	log.Printf("WAV文件大小: %d 字节", fileInfo.Size())

	wavHeader := make([]byte, 44)
	n, err := f.Read(wavHeader)
	if err != nil || n < 44 {
		return fmt.Errorf("读取WAV头失败: %v", err)
	}

	sampleRate := binary.LittleEndian.Uint32(wavHeader[24:28])
	bitsPerSample := binary.LittleEndian.Uint16(wavHeader[34:36])
	channels := binary.LittleEndian.Uint16(wavHeader[22:24])
	log.Printf("WAV文件格式: %dHz, %d位, %d声道", sampleRate, bitsPerSample, channels)

	pcmData, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("读取PCM数据失败: %v", err)
	}
	log.Printf("读取PCM原始数据长度: %d 字节", len(pcmData))

	var pcmInts []int
	if bitsPerSample == 16 {
		pcmInts = make([]int, len(pcmData)/2)
		for i := 0; i < len(pcmData)/2; i++ {
			pcmInts[i] = int(int16(binary.LittleEndian.Uint16(pcmData[i*2:])))
		}
	} else if bitsPerSample == 8 {
		pcmInts = make([]int, len(pcmData))
		for i := 0; i < len(pcmData); i++ {
			pcmInts[i] = int(int8(pcmData[i])) * 256
		}
	} else {
		return fmt.Errorf("不支持的位深度: %d", bitsPerSample)
	}

	if channels > 1 {
		monoLen := len(pcmInts) / int(channels)
		monoData := make([]int, monoLen)
		for i := 0; i < monoLen; i++ {
			var sum int
			for ch := 0; ch < int(channels); ch++ {
				sum += pcmInts[i*int(channels)+ch]
			}
			monoData[i] = sum / int(channels)
		}
		pcmInts = monoData
		log.Printf("转换为单声道后长度: %d 帧", len(pcmInts))
	}

	log.Printf("重采样前PCM长度: %d 帧", len(pcmInts))
	if sampleRate != 16000 {
		log.Printf("需要重采样: %dHz → 8000Hz", sampleRate)
		var resamplePhase float64
		int16Data := make([]int16, len(pcmInts))
		for i, v := range pcmInts {
			int16Data[i] = int16(v)
		}
		resampled := cubicResample(int16Data, int(sampleRate), 16000, &resamplePhase)
		log.Printf("重采样后长度: %d 帧", len(resampled))
		pcmInts = make([]int, len(resampled))
		for i, v := range resampled {
			pcmInts[i] = int(v)
		}
	}

	log.Printf("TTS PCM数据长度: %d 帧", len(pcmInts)/160)
	startTime := time.Now()
	frameCount := 0
	sentFrames := 0
	for len(pcmInts) >= 320 {
		chunk := make([]int, 320)
		copy(chunk, pcmInts[:320])

		select {
		case ttsPCM <- [][]int{chunk}:
			sentFrames++
			frameCount++
			targetTime := startTime.Add(time.Duration(frameCount) * time.Second * 20 / 1000)
			sleepTime := targetTime.Sub(time.Now())
			if sleepTime > 0 {
				time.Sleep(sleepTime)
			}
		default:
			time.Sleep(time.Millisecond * 1)
		}

		pcmInts = pcmInts[320:]
	}
	log.Printf("TTS发送完成: 共发送 %d 帧", sentFrames)

	log.Printf("TTS合成完成")
	return nil
}
