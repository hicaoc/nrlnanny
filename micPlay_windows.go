//go:build windows

package main

import (
	"log"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

// MicRun 启动麦克风采集 (Windows Version using go-wca, no CGO required)
func MicRun() {
	if err := runCapture(); err != nil {
		log.Printf("❌ 麦克风采集失败 (Windows): %v", err)
	}
}

func runCapture() error {
	// 1. 初始化 COM
	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		return err
	}
	defer ole.CoUninitialize()

	// 2. 获取默认捕捉设备
	var mmDeviceEnumerator *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmDeviceEnumerator); err != nil {
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
	// var deviceFormat *wca.WAVEFORMATEX
	// if err := audioClient.GetMixFormat(&deviceFormat); err != nil {
	// 	return err
	// }

	// 4. 获取设备默认格式（仅用于采样率和声道数）
	var mixFormat *wca.WAVEFORMATEX
	if err := audioClient.GetMixFormat(&mixFormat); err != nil {
		return err
	}
	defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(mixFormat))) // 重要：释放 COM 分配的内存

	// 强制使用 16-bit PCM 格式（让 WASAPI 自动转换）
	// 提取并转换类型
	sampleRate := mixFormat.NSamplesPerSec // uint32
	nChannels := mixFormat.NChannels       // uint16

	// 构造 16-bit PCM 格式
	format := &wca.WAVEFORMATEX{
		WFormatTag:      wca.WAVE_FORMAT_PCM,
		NSamplesPerSec:  sampleRate,
		NChannels:       nChannels,
		WBitsPerSample:  16,
		NBlockAlign:     nChannels * 2, // OK: uint16 * untyped int
		NAvgBytesPerSec: sampleRate * uint32(nChannels) * 2,
	}

	// Log the device's native format
	log.Printf("设备原生格式: %dHz, %d位, %d声道",
		format.NSamplesPerSec,
		format.WBitsPerSample,
		format.NChannels)

	// 5. 尝试使用设备原生格式 (通常是 48000Hz, 16bit, 2ch)
	// 我们将在回调中进行降采样和单声道转换
	bufferDuration := wca.REFERENCE_TIME(1000000) // 100ms

	// 使用 AUTOCONVERTPCM 标志让 Windows 自动转换
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

	// 获取 CaptureClient
	var captureClient *wca.IAudioCaptureClient
	if err := audioClient.GetService(wca.IID_IAudioCaptureClient, &captureClient); err != nil {
		return err
	}
	defer captureClient.Release()

	// 开始采集
	if err := audioClient.Start(); err != nil {
		return err
	}
	defer audioClient.Stop()

	log.Printf("✅ 麦克风采集已启动 (Windows WASAPI, %dHz, %d位, %d声道 -> 8000Hz单声道)",
		format.NSamplesPerSec,
		format.WBitsPerSample,
		format.NChannels)

	// 降采样缓冲区
	var resampleBuffer []int16
	var resamplePos int
	targetSampleRate := 8000
	sourceSampleRate := int(format.NSamplesPerSec)
	sourceChannels := int(format.NChannels)

	// 计算降采样比率
	downsampleRatio := sourceSampleRate / targetSampleRate

	// 循环读取
	for {
		var pData *byte
		var numFramesToRead uint32
		var flags uint32
		var devicePosition uint64
		var qpcPosition uint64

		// 获取下一个数据包大小
		var packetLength uint32
		if err := captureClient.GetNextPacketSize(&packetLength); err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		if packetLength == 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		if err := captureClient.GetBuffer(&pData, &numFramesToRead, &flags, &devicePosition, &qpcPosition); err != nil {
			log.Printf("GetBuffer error: %v", err)
			continue
		}

		if numFramesToRead > 0 {
			// 计算字节数: frames * channels * bytes_per_sample
			bytesPerSample := int(format.WBitsPerSample / 8)
			totalBytes := int(numFramesToRead) * sourceChannels * bytesPerSample
			dataBytes := unsafe.Slice(pData, totalBytes)

			//log.Printf("dataBytes: %v %v %v", bytesPerSample, totalBytes, dataBytes[:10])

			// 转换为 int16 并降采样
			for i := 0; i < int(numFramesToRead); i++ {
				// 只在降采样点采样
				if i%downsampleRatio == 0 {
					// 转换为单声道 (取所有声道的平均值)
					var monoSample int32 = 0
					for ch := 0; ch < sourceChannels; ch++ {
						offset := (i*sourceChannels + ch) * bytesPerSample
						if bytesPerSample == 2 {
							// 16-bit
							sample := int16(uint16(dataBytes[offset]) | uint16(dataBytes[offset+1])<<8)
							monoSample += int32(sample)
						}
					}
					monoSample /= int32(sourceChannels)

					resampleBuffer = append(resampleBuffer, int16(monoSample))
					resamplePos++

					// 当累积到500个采样点时发送
					if resamplePos >= 500 {
						dataInt := make([]int, 500)
						for j := 0; j < 500; j++ {
							dataInt[j] = int(resampleBuffer[j])
						}

						//log.Printf("micPCM: %v", resampleBuffer)

						select {
						case micPCM <- [][]int{dataInt}:
							//log.Println("micPCM", dataInt)
						default:
						}

						// 重置缓冲区
						resampleBuffer = resampleBuffer[:0]
						resamplePos = 0
					}
				}
			}
		}

		if err := captureClient.ReleaseBuffer(numFramesToRead); err != nil {
			log.Printf("ReleaseBuffer error: %v", err)
		}
	}
}
