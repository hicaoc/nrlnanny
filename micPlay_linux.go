//go:build !windows

package main

import (
	"log"

	"github.com/gen2brain/malgo"
)

// MicRun 启动麦克风采集
func MicRun() {
	// 1. 初始化 Context
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {
		log.Printf("AUDIO LOG: %v", message)
	})
	if err != nil {
		log.Printf("❌ 音频上下文初始化失败: %v", err)
		return
	}
	defer func() {
		_ = ctx.Uninit()
		ctx.Free()
	}()

	// 2. 配置采集设备
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = 1
	deviceConfig.SampleRate = 8000
	deviceConfig.Alsa.NoMMap = 1 // 某些Linux设备需要

	// 3. 定义数据回调
	onRecvFrames := func(pOutputSample, pInputSamples []byte, framecount uint32) {
		if !isRecordMicEnabled() {
			return
		}
		// pInputSamples 是原始字节流
		// S16格式: 每个采样2字节
		// 单声道: 每次采样1个S16
		// data len = framecount

		sampleCount := int(framecount)
		if sampleCount == 0 {
			return
		}

		data := make([]int, sampleCount)

		for i := range sampleCount {
			// Little Endian encoding for S16
			// low byte = pInputSamples[i*2]
			// high byte = pInputSamples[i*2+1]
			val := int16(uint16(pInputSamples[i*2]) | uint16(pInputSamples[i*2+1])<<8)
			data[i] = int(val)
		}

		// 写入全局变量 micPCM
		// 非阻塞写入，防止阻塞回调
		select {
		case micPCM <- [][]int{data}:
		default:
			// log.Println("Mic buffer full, dropping frames")
		}
	}

	// 4. 初始化设备
	captureCallbacks := malgo.DeviceCallbacks{
		Data: onRecvFrames,
	}

	device, err := malgo.InitDevice(ctx.Context, deviceConfig, captureCallbacks)
	if err != nil {
		log.Printf("❌ 音频设备初始化失败: %v", err)
		return
	}
	defer device.Uninit()

	started := false
	applyRecordState := func() {
		if isRecordMicEnabled() {
			if started {
				return
			}
			if err := device.Start(); err != nil {
				log.Printf("❌ 启动音频采集失败: %v", err)
				return
			}
			started = true
			log.Println("✅ 麦克风采集已启动 (Malgo/Miniaudio, 8000Hz, S16, Mono)")
			return
		}
		if started {
			_ = device.Stop()
			started = false
			log.Println("🛑 麦克风采集已停止")
		}
	}

	applyRecordState()
	for range recordToggleChan {
		applyRecordState()
	}

	// device.Uninit() // Unreached
}
