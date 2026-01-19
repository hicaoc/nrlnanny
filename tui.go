package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	app          *tview.Application
	musicList    *tview.List
	scheduleList *tview.List
	logView      *tview.TextView
	statusView   *tview.TextView
	cronView     *tview.TextView
	volumeView   *tview.TextView
	uiStarted    bool
	logChan      = make(chan string, 500)
	uiUpdateChan = make(chan func(), 500)

	displayMu   sync.Mutex
	statusState string
	cronState   string
)

// TuiLogger implements io.Writer to redirect logs to the TUI
type TuiLogger struct{}

func (t *TuiLogger) Write(p []byte) (n int, err error) {
	msg := string(p)
	if app != nil && uiStarted {
		select {
		case logChan <- msg:
		default:
			// If buffer is full, drop log to avoid hanging the system
		}
	} else {
		fmt.Fprint(os.Stderr, msg)
	}
	return len(p), nil
}

func initTUI() {
	app = tview.NewApplication()

	// 1. Music Area (Left)
	musicList = tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true)
	musicList.SetBorder(true).SetTitle(" Music Playlist (Control: Left/Right/Space) ")

	// 2. Schedule/Cron Area (Right)
	scheduleList = tview.NewList().
		ShowSecondaryText(true)
	scheduleList.SetBorder(true).SetTitle(" Scheduled Playback ")

	cronView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	cronView.SetBorder(true).SetTitle(" Next Beacon ")

	rightFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(cronView, 3, 1, false).
		AddItem(scheduleList, 0, 1, false)

	// 3. Status Area (Bottom of Top Section)
	statusView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	statusView.SetBorder(true).SetTitle(" Status ")

	volumeView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	volumeView.SetBorder(true).SetTitle(" Volume (Control: Up/Down) ")
	updateVolumeDisplay()

	// 4. Log Area (Bottom)
	logView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetMaxLines(1000)
	logView.SetTitle(" System Logs ").SetBorder(true)

	// Layout Composition
	// Top Row: Music (50%) | Right Panel (50%)
	topRow := tview.NewFlex().
		AddItem(musicList, 0, 1, true).
		AddItem(rightFlex, 0, 1, false)

	// Middle Row: Status | Volume
	middleRow := tview.NewFlex().
		AddItem(statusView, 0, 3, false).
		AddItem(volumeView, 0, 1, false)

	// Main Layout: Top | Middle | Bottom (Logs)
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(topRow, 0, 3, true).
		AddItem(middleRow, 3, 1, false).
		AddItem(logView, 0, 1, false)

	app.SetRoot(flex, true).EnableMouse(true)

	// Global Keybinding
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Volume Control
		if event.Key() == tcell.KeyUp {
			if conf.System.Volume < 2 {
				conf.System.Volume += 0.01
				updateVolumeDisplay()
				return nil
			}
		} else if event.Key() == tcell.KeyDown {
			if conf.System.Volume > 0.01 {
				conf.System.Volume -= 0.01
				updateVolumeDisplay()
				return nil
			}
		}

		// Playback Control
		if event.Key() == tcell.KeyLeft {
			// Restart / Prev
			select {
			case lastmusic <- true:
				// log.Println("Command: Restart/Previous")
			default:
			}
			return nil
		} else if event.Key() == tcell.KeyRight {
			// Next
			select {
			case nextmusic <- true:
				// log.Println("Command: Next")
			default:
			}
			return nil
		} else if event.Key() == tcell.KeyRune && event.Rune() == ' ' {
			// Pause
			select {
			case pausemusic <- true:
				// log.Println("Command: Pause/Resume")
			default:
			}
			return nil
		}

		// Tab to switch focus
		if event.Key() == tcell.KeyTab {
			focus := app.GetFocus()
			switch focus {
			case musicList:
				app.SetFocus(scheduleList)
			case scheduleList:
				app.SetFocus(logView)
			default:
				app.SetFocus(musicList)
			}
			return nil
		}

		return event
	})

	// app.SetRoot(flex, true).EnableMouse(true)
	// Keybindings...
}

func updateVolumeDisplay() {
	if volumeView != nil {
		vol := int(conf.System.Volume * 100)
		volumeView.SetText(fmt.Sprintf("[yellow]%d%%[white]", vol))
	}
}

// updateMusicList safely updates the music list in the UI
func updateMusicList(files []MusicFileInfo, currentID int) {
	if musicList == nil {
		return
	}
	if !uiStarted {
		drawMusicList(files, currentID)
		return
	}
	select {
	case uiUpdateChan <- func() { drawMusicList(files, currentID) }:
	default:
		// If queue is full, skip this update to avoid blocking
	}
}

func drawMusicList(files []MusicFileInfo, currentID int) {
	musicList.Clear()
	for i, f := range files {
		title := fmt.Sprintf("%04d %s", f.ID, f.Path)
		if f.ID == currentID {
			title = "[green]▶ " + title + "[white]"
		}
		// Clone f to avoid closure issues
		fileInfo := f
		musicList.AddItem(title, "", 0, func() {
			// On Select (Enter) - force play
			log.Printf("Selected: %s", fileInfo.Path)
			PlayMusicByID(fileInfo.ID)
		})
		if f.ID == currentID {
			musicList.SetCurrentItem(i)
		}
	}
}

func updatePlayStatus(text string) {
	displayMu.Lock()
	statusState = text
	displayMu.Unlock()
}

func updateCronInfo(info string) {
	displayMu.Lock()
	cronState = info
	displayMu.Unlock()
}

func updateScheduleList(tasks map[string]AudioFileInfo) {
	if scheduleList == nil {
		return
	}
	if !uiStarted {
		drawScheduleList(tasks)
		return
	}
	select {
	case uiUpdateChan <- func() { drawScheduleList(tasks) }:
	default:
	}
}

func drawScheduleList(tasks map[string]AudioFileInfo) {
	scheduleList.Clear()
	for _, info := range tasks {
		scheduleList.AddItem(fmt.Sprintf("[%02d:%02d] %s", info.Hour, info.Minute, info.Path), "", 0, nil)
	}
}

func startTUI() {
	// Set log output to TUI
	log.SetOutput(&TuiLogger{})

	uiStarted = true

	// Buffered Log Consumer
	go func() {
		for msg := range logChan {
			if app != nil && uiStarted {
				app.QueueUpdate(func() {
					fmt.Fprint(logView, msg)
					logView.ScrollToEnd()
				})
			}
		}
	}()

	// UI Update Consumer
	go func() {
		for update := range uiUpdateChan {
			if app != nil && uiStarted {
				app.QueueUpdate(update)
			}
		}
	}()

	// UI State Refresher (paints latest status/cron every 200ms)
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if app != nil && uiStarted {
				displayMu.Lock()
				s := statusState
				c := cronState
				displayMu.Unlock()

				app.QueueUpdateDraw(func() {
					if statusView != nil {
						statusView.SetText(s)
					}
					if cronView != nil {
						cronView.SetText(c)
					}
				})
			}
		}
	}()

	if err := app.Run(); err != nil {
		uiStarted = false
		// TUI failed to start (e.g. non-interactive environment)
		log.SetOutput(os.Stderr)
		app = nil // Clear app to signal TuiLogger should use console
		log.Printf("⚠️  TUI failed to start: %v", err)
		log.Println("Background services are still running in console mode.")
		return
	}

	// If TUI exits normally, restore standard logging
	log.SetOutput(os.Stderr)
	log.Println("TUI exited normaly.")
}
