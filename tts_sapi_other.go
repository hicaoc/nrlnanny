//go:build !windows

package main

import "fmt"

func sendTTSVoice(text, voiceName string, rate, pitch float64) error {
	return fmt.Errorf("TTS transmission is supported on Windows only")
}
