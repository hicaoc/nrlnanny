package main

import (
	"sync"
	"sync/atomic"
)

var (
	displayMu      sync.Mutex
	statusState    = "Idle"
	cronState      = ""
	progressState  = 0
	isPlayingState = false
)

var recordMicEnabled uint32 = 0
var musicEnabled uint32 = 1
var cronEnabled uint32 = 1
var timeEnabled uint32 = 1
var recordToggleChan = make(chan struct{}, 1)
var musicToggleChan = make(chan struct{}, 1)
var cronToggleChan = make(chan struct{}, 1)
var timeToggleChan = make(chan struct{}, 1)

func isRecordMicEnabled() bool {
	return atomic.LoadUint32(&recordMicEnabled) == 1
}

func setRecordMicEnabled(enabled bool) {
	if enabled {
		atomic.StoreUint32(&recordMicEnabled, 1)
	} else {
		atomic.StoreUint32(&recordMicEnabled, 0)
	}
	signalRecordToggle()
}

func toggleRecordMicEnabled() bool {
	if atomic.LoadUint32(&recordMicEnabled) == 1 {
		atomic.StoreUint32(&recordMicEnabled, 0)
		signalRecordToggle()
		return false
	}
	atomic.StoreUint32(&recordMicEnabled, 1)
	signalRecordToggle()
	return true
}

func signalRecordToggle() {
	select {
	case recordToggleChan <- struct{}{}:
	default:
	}
}

func isMusicEnabled() bool {
	return atomic.LoadUint32(&musicEnabled) == 1
}

func setMusicEnabled(enabled bool) {
	if enabled {
		atomic.StoreUint32(&musicEnabled, 1)
	} else {
		atomic.StoreUint32(&musicEnabled, 0)
	}
	signalMusicToggle()
}

func toggleMusicEnabled() bool {
	if atomic.LoadUint32(&musicEnabled) == 1 {
		atomic.StoreUint32(&musicEnabled, 0)
		signalMusicToggle()
		return false
	}
	atomic.StoreUint32(&musicEnabled, 1)
	signalMusicToggle()
	return true
}

func signalMusicToggle() {
	select {
	case musicToggleChan <- struct{}{}:
	default:
	}
}

func isCronEnabled() bool {
	return atomic.LoadUint32(&cronEnabled) == 1
}

func setCronEnabled(enabled bool) {
	if enabled {
		atomic.StoreUint32(&cronEnabled, 1)
	} else {
		atomic.StoreUint32(&cronEnabled, 0)
	}
	signalCronToggle()
}

func toggleCronEnabled() bool {
	if atomic.LoadUint32(&cronEnabled) == 1 {
		atomic.StoreUint32(&cronEnabled, 0)
		signalCronToggle()
		return false
	}
	atomic.StoreUint32(&cronEnabled, 1)
	signalCronToggle()
	return true
}

func signalCronToggle() {
	select {
	case cronToggleChan <- struct{}{}:
	default:
	}
}

func isTimeEnabled() bool {
	return atomic.LoadUint32(&timeEnabled) == 1
}

func setTimeEnabled(enabled bool) {
	if enabled {
		atomic.StoreUint32(&timeEnabled, 1)
	} else {
		atomic.StoreUint32(&timeEnabled, 0)
	}
	signalTimeToggle()
}

func toggleTimeEnabled() bool {
	if atomic.LoadUint32(&timeEnabled) == 1 {
		atomic.StoreUint32(&timeEnabled, 0)
		signalTimeToggle()
		return false
	}
	atomic.StoreUint32(&timeEnabled, 1)
	signalTimeToggle()
	return true
}

func signalTimeToggle() {
	select {
	case timeToggleChan <- struct{}{}:
	default:
	}
}

// playing reflects music play/pause state for the UI button.
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
