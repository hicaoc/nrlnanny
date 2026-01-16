//go:build windows

package main

import (
	"log"
	"math"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

// FilterState 存储滤波器的状态以保持块之间的连续性
type FilterState struct {
	history []int16
}

// MicRun 启动麦克风采集 (Windows WASAPI)
func MicRun() {
	if err := runCapture(); err != nil {
		log.Printf("❌ 麦克风采集失败 (Windows): %v", err)
	}
}

// removeDCOffset 去除直流偏移 (使用一阶高通滤波器)
func removeDCOffset(samples []int16, lastInput, lastOutput *float64) []int16 {
	if len(samples) == 0 {
		return samples
	}
	alpha := 0.995 // 接近 1，截止频率很低
	dst := make([]int16, len(samples))
	for i, s := range samples {
		in := float64(s)
		out := alpha * (*lastOutput + in - *lastInput)
		*lastInput = in
		*lastOutput = out

		val := out
		if val > 32767 {
			val = 32767
		} else if val < -32768 {
			val = -32768
		}
		dst[i] = int16(val)
	}
	return dst
}

// lowPassFilter 对 int16 音频进行抗混叠低通滤波 (使用状态保持以确保连续性)
func lowPassFilter(src []int16, state *FilterState, cutoffRatio float64) []int16 {
	if len(src) == 0 || cutoffRatio >= 1.0 {
		return src
	}

	taps := 63
	mid := taps / 2

	// 初始化状态历史记录，确保它足够长（taps-1）
	if len(state.history) < taps-1 {
		state.history = make([]int16, taps-1)
	}

	h := make([]float64, taps)
	cutoff := cutoffRatio * 0.42 // 进一步降低截止频率以获得色彩更温暖、更干净的语音

	// 计算 sinc 滤波器系数
	for i := 0; i < taps; i++ {
		n := float64(i - mid)
		if i == mid {
			h[i] = 2 * cutoff
		} else {
			h[i] = math.Sin(2*math.Pi*cutoff*n) / (math.Pi * n)
		}
		// Blackman-Harris 窗口
		a0 := 0.35875
		a1 := 0.48829
		a2 := 0.14128
		a3 := 0.01168
		x := 2 * math.Pi * float64(i) / float64(taps-1)
		h[i] *= a0 - a1*math.Cos(x) + a2*math.Cos(2*x) - a3*math.Cos(3*x)
	}

	// 归一化
	hTotal := 0.0
	for _, v := range h {
		hTotal += v
	}
	if hTotal != 0 {
		for i := range h {
			h[i] /= hTotal
		}
	}

	dst := make([]int16, len(src))

	// 合并历史数据和当前数据进行卷积
	// 历史数据存储前一个块的最后 taps-1 个点
	combined := make([]int16, 0, (taps-1)+len(src))
	combined = append(combined, state.history...)
	combined = append(combined, src...)

	for i := 0; i < len(src); i++ {
		sum := 0.0
		for j := 0; j < taps; j++ {
			sum += float64(combined[i+j]) * h[j]
		}
		if sum > 32767 {
			sum = 32767
		} else if sum < -32768 {
			sum = -32768
		}
		dst[i] = int16(sum)
	}

	// 更新历史记录（保存当前块的最后 taps-1 个点给下一次处理）
	if len(src) >= taps-1 {
		state.history = make([]int16, taps-1)
		copy(state.history, src[len(src)-(taps-1):])
	} else {
		// 如果块特别短，合并并截断
		state.history = append(state.history, src...)
		if len(state.history) > taps-1 {
			state.history = state.history[len(state.history)-(taps-1):]
		}
	}

	return dst
}

// cubicInterpolate 三次插值
func cubicInterpolate(y0, y1, y2, y3, mu float64) float64 {
	mu2 := mu * mu
	a0 := y3 - y2 - y0 + y1
	a1 := y0 - y1 - a0
	a2 := y2 - y0
	a3 := y1
	return a0*mu*mu2 + a1*mu2 + a2*mu + a3
}

// cubicResample 对单声道 int16 音频进行三次插值重采样，返回 int16
func cubicResample(src []int16, srcRate, dstRate int, phase *float64) []int16 {
	if len(src) < 4 || dstRate <= 0 || srcRate <= 0 {
		return nil
	}
	ratio := float64(srcRate) / float64(dstRate)

	// 计算输出长度，考虑累积相位
	numOutput := int((float64(len(src)) - *phase) / ratio)
	if numOutput <= 0 {
		return nil
	}

	dst := make([]int16, numOutput)
	for i := 0; i < numOutput; i++ {
		pos := *phase + float64(i)*ratio
		idx := int(pos)
		mu := pos - float64(idx)

		// 获取四个相邻采样点 (保持边界安全)
		var y0, y1, y2, y3 float64
		if idx > 0 {
			y0 = float64(src[idx-1])
		} else {
			y0 = float64(src[0])
		}
		y1 = float64(src[idx])
		if idx+1 < len(src) {
			y2 = float64(src[idx+1])
		} else {
			y2 = float64(src[len(src)-1])
		}
		if idx+2 < len(src) {
			y3 = float64(src[idx+2])
		} else {
			y3 = float64(src[len(src)-1])
		}

		val := cubicInterpolate(y0, y1, y2, y3, mu)

		if val > 32767 {
			val = 32767
		} else if val < -32768 {
			val = -32768
		}
		dst[i] = int16(val)
	}

	// 更新相位
	*phase = (*phase + float64(numOutput)*ratio) - float64(len(src))
	return dst
}

func runCapture() error {
	// 1. 初始化 COM
	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		return err
	}
	defer ole.CoUninitialize()

	// 2. 获取默认捕捉设备
	var mmDeviceEnumerator *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(
		wca.CLSID_MMDeviceEnumerator,
		0,
		wca.CLSCTX_ALL,
		wca.IID_IMMDeviceEnumerator,
		&mmDeviceEnumerator,
	); err != nil {
		return err
	}
	defer mmDeviceEnumerator.Release()

	var mmDevice *wca.IMMDevice
	if err := mmDeviceEnumerator.GetDefaultAudioEndpoint(wca.ECapture, wca.EConsole, &mmDevice); err != nil {
		return err
	}
	defer mmDevice.Release()

	// 3. 激活 AudioClient
	var audioClient *wca.IAudioClient
	if err := mmDevice.Activate(wca.IID_IAudioClient, wca.CLSCTX_ALL, nil, &audioClient); err != nil {
		return err
	}
	defer audioClient.Release()

	// 4. 获取设备默认格式
	var mixFormat *wca.WAVEFORMATEX
	if err := audioClient.GetMixFormat(&mixFormat); err != nil {
		return err
	}
	defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(mixFormat)))

	sampleRate := mixFormat.NSamplesPerSec
	nChannels := mixFormat.NChannels

	format := &wca.WAVEFORMATEX{
		WFormatTag:      wca.WAVE_FORMAT_PCM,
		NSamplesPerSec:  sampleRate,
		NChannels:       nChannels,
		WBitsPerSample:  16,
		NBlockAlign:     nChannels * 2,
		NAvgBytesPerSec: sampleRate * uint32(nChannels) * 2,
	}

	log.Printf("设备原生格式: %dHz, %d位, %d声道",
		format.NSamplesPerSec,
		format.WBitsPerSample,
		format.NChannels)

	// 5. 初始化 AudioClient
	bufferDuration := wca.REFERENCE_TIME(1000000) // 100ms
	if err := audioClient.Initialize(
		wca.AUDCLNT_SHAREMODE_SHARED,
		wca.AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM,
		bufferDuration,
		0,
		format,
		nil,
	); err != nil {
		log.Printf("❌ AudioClient初始化失败: %v", err)
		return err
	}

	// 6. 获取 CaptureClient
	var captureClient *wca.IAudioCaptureClient
	if err := audioClient.GetService(wca.IID_IAudioCaptureClient, &captureClient); err != nil {
		return err
	}
	defer captureClient.Release()

	// 7. 开始采集
	if err := audioClient.Start(); err != nil {
		return err
	}
	defer audioClient.Stop()

	log.Printf("✅ 麦克风采集已启动 (Windows WASAPI, %dHz → 8000Hz 单声道)",
		format.NSamplesPerSec)

	sourceSampleRate := int(format.NSamplesPerSec)
	sourceChannels := int(format.NChannels)

	// 状态维护
	var lastIn, lastOut float64
	lpState := &FilterState{}
	var resamplePhase float64

	// 输入累积缓冲区 (用于处理足够大的块)
	var rawAccumBuffer []int16
	minProcessingSize := sourceSampleRate / 10 // 100ms 作为一个处理单元

	// 缓冲区：累积重采样后的 int 数据
	var outputBuffer []int
	targetSampleRate := 8000

	// 8. 主循环
	for {
		var pData *byte
		var numFramesToRead uint32
		var flags uint32
		var devicePosition uint64
		var qpcPosition uint64

		var packetLength uint32
		if err := captureClient.GetNextPacketSize(&packetLength); err != nil {
			time.Sleep(1 * time.Millisecond)
			continue
		}
		if packetLength == 0 {
			time.Sleep(1 * time.Millisecond)
			continue
		}

		if err := captureClient.GetBuffer(&pData, &numFramesToRead, &flags, &devicePosition, &qpcPosition); err != nil {
			continue
		}

		if numFramesToRead > 0 {
			bytesPerSample := 2 // 16-bit
			totalBytes := int(numFramesToRead) * sourceChannels * bytesPerSample
			dataBytes := unsafe.Slice(pData, totalBytes)

			// 1. 转为单声道
			currentMono := make([]int16, numFramesToRead)
			for i := 0; i < int(numFramesToRead); i++ {
				var sum int32
				for ch := 0; ch < sourceChannels; ch++ {
					offset := (i*sourceChannels + ch) * bytesPerSample
					sample := int16(uint16(dataBytes[offset]) | uint16(dataBytes[offset+1])<<8)
					sum += int32(sample)
				}
				currentMono[i] = int16(sum / int32(sourceChannels))
			}

			// 2. 累积原始音频
			rawAccumBuffer = append(rawAccumBuffer, currentMono...)

			// 只有积累了足够数据才进行后续处理，以减少边缘噪音
			if len(rawAccumBuffer) >= minProcessingSize {
				// 3. 去除DC偏移
				noDC := removeDCOffset(rawAccumBuffer, &lastIn, &lastOut)

				// 4. 抗混叠低通滤波
				cutoffFreq := 3400.0 // 降低截止频率更加丝滑
				nyquistIn := float64(sourceSampleRate) / 2.0
				cutoffRatio := cutoffFreq / nyquistIn
				filtered := lowPassFilter(noDC, lpState, cutoffRatio)

				// 5. 重采样到 8000Hz
				resampled := cubicResample(filtered, sourceSampleRate, targetSampleRate, &resamplePhase)

				if resampled != nil {
					// 转为 []int 并添加到输出缓冲
					for _, v := range resampled {
						outputBuffer = append(outputBuffer, int(v))
					}
				}

				// 清空累积缓冲区
				rawAccumBuffer = rawAccumBuffer[:0]
			}

			// 6. 分块发送（500 点/帧，约 62.5ms @ 8000Hz）
			for len(outputBuffer) >= 500 {
				chunk := make([]int, 500)
				copy(chunk, outputBuffer[:500])
				select {
				case micPCM <- [][]int{chunk}:
				default:
				}
				outputBuffer = outputBuffer[500:]
			}
		}

		_ = captureClient.ReleaseBuffer(numFramesToRead)
	}
}
