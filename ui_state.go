package main

import (
	"io"
	"log"
	"os"
	"sync"
)

var (
	displayMu      sync.Mutex
	statusState    = "Idle"
	cronState      = ""
	progressState  = 0
	isPlayingState = false
)

type webLogWriter struct {
	out io.Writer
}

func (w *webLogWriter) Write(p []byte) (int, error) {
	logMu.Lock()
	GlobalLogBuffer = append(GlobalLogBuffer, string(p))
	if len(GlobalLogBuffer) > maxLogLines {
		GlobalLogBuffer = GlobalLogBuffer[len(GlobalLogBuffer)-maxLogLines:]
	}
	logMu.Unlock()

	return w.out.Write(p)
}

func initWebLogCapture() {
	log.SetOutput(&webLogWriter{out: os.Stderr})
}

func updatePlayStatus(text string, percent int, playing bool) {
	displayMu.Lock()
	statusState = text
	progressState = percent
	isPlayingState = playing
	displayMu.Unlock()
}

func updateCronInfo(info string) {
	displayMu.Lock()
	cronState = info
	displayMu.Unlock()
}

func updateMusicList(_ []MusicFileInfo, _ int) {}

func updateScheduleList(_ map[string]AudioFileInfo) {}

func updateVolumeDisplay() {}

