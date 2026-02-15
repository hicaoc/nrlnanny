package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed control.html play.html live.html
var webAssets embed.FS

type AudioFile struct {
	Name      string `json:"name"`
	Timestamp string `json:"timestamp"`
	URL       string `json:"url"`
}

// 从文件名提取时间
func parseTimeFromFilename(filename string) (time.Time, error) {
	base := strings.TrimSuffix(filename, ".wav")
	parts := strings.Split(base, "_")
	if len(parts) < 3 {
		return time.Time{}, fmt.Errorf("格式错误: %s", filename)
	}
	datePart := parts[1]
	timePart := parts[2]
	dtStr := datePart + " " + timePart[:2] + ":" + timePart[2:4] + ":" + timePart[4:6]
	return time.Parse("2006-01-02 15:04:05", dtStr)
}

func play() {
	http.HandleFunc("/", serveIndex)                 // Dashboard
	http.HandleFunc("/play", servePlay)              // Original recordings browser
	http.HandleFunc("/live", serveLive)              // Live broadcast page
	http.HandleFunc("/ws/live", handleLiveWS)        // Live WebSocket (same port, reverse-proxy/mobile friendly)
	http.HandleFunc("/pcm-worklet.js", serveWorklet) // AudioWorklet JS for Safari
	http.HandleFunc("/dirs", listDirs)               // 获取所有日期目录
	http.HandleFunc("/dir/", listFilesInDir)         // 获取某目录下文件
	http.Handle("/recordings/", http.StripPrefix("/recordings/", http.FileServer(http.Dir(conf.System.RecoderFilePath))))

	// Web API
	http.HandleFunc("/api/status", apiStatus)
	http.HandleFunc("/api/music", apiMusic)
	http.HandleFunc("/api/control", apiControl)
	http.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("pong"))
	})

	log.Printf("服务器启动中：http://0.0.0.0:%s\n", conf.System.WebPort)
	server := &http.Server{
		Addr: ":" + conf.System.WebPort,
	}
	log.Fatal(server.ListenAndServe())
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	content, err := webAssets.ReadFile("control.html")
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

func serveWorklet(w http.ResponseWriter, r *http.Request) {
	const js = `class PCMPlayerProcessor extends AudioWorkletProcessor {
    constructor() {
        super();
        this.buffer = new Float32Array(0);
        this.port.onmessage = (e) => {
            const incoming = e.data;
            const newBuf = new Float32Array(this.buffer.length + incoming.length);
            newBuf.set(this.buffer);
            newBuf.set(incoming, this.buffer.length);
            this.buffer = newBuf;
            if (this.buffer.length > 16000) {
                this.buffer = this.buffer.slice(this.buffer.length - 16000);
            }
        };
    }
    process(inputs, outputs) {
        const output = outputs[0][0];
        if (!output) return true;
        if (this.buffer.length >= output.length) {
            output.set(this.buffer.subarray(0, output.length));
            this.buffer = this.buffer.slice(output.length);
        } else {
            output.fill(0);
            if (this.buffer.length > 0) {
                output.set(this.buffer);
                this.buffer = new Float32Array(0);
            }
        }
        return true;
    }
}
registerProcessor('pcm-player', PCMPlayerProcessor);`
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write([]byte(js))
}

func serveLive(w http.ResponseWriter, r *http.Request) {
	content, err := webAssets.ReadFile("live.html")
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(content)
}

func servePlay(w http.ResponseWriter, r *http.Request) {
	content, err := webAssets.ReadFile("play.html")
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

func apiStatus(w http.ResponseWriter, r *http.Request) {
	displayMu.Lock()
	s := statusState
	c := cronState
	p := progressState

	displayMu.Unlock()

	data := map[string]any{
		"volume":         int(conf.System.Volume * 100),
		"status":         s,
		"cron":           c,
		"progress":       p,
		"playing":        conf.System.MusicPlaying,
		"duck_scale":     int(conf.System.DuckScale * 100),
		"duck_mic_pcm":   conf.System.DuckMicPCM,
		"duck_music_pcm": conf.System.DuckMusicPCM,
		"record_mic":     isRecordMicEnabled(),
		"record_voice":   isRecordingEnabled(),
		"cron_enabled":   isCronEnabled(),
		"time_enabled":   isTimeEnabled(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func apiMusic(w http.ResponseWriter, r *http.Request) {
	musicstateMu.Lock()
	files := currentQueue.files
	playingID := currentPlayingID
	musicstateMu.Unlock()

	data := map[string]any{
		"files":     files,
		"playingID": playingID,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func apiControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Action string  `json:"action"`
		Value  float64 `json:"value"`
		ID     int     `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	switch req.Action {
	case "play_id":
		PlayMusicByID(req.ID)
	case "pause":
		select {
		case pausemusic <- true:
		default:
		}
		conf.System.MusicPlaying = !conf.System.MusicPlaying
		saveConfig()
	case "next":
		select {
		case nextmusic <- true:
		default:
		}
	case "prev":
		select {
		case lastmusic <- true:
		default:
		}
	case "volume":
		if req.Value >= 0 && req.Value <= 2 {
			conf.System.Volume = req.Value
			updateVolumeDisplay()
			saveConfig()
		}
	case "duck_scale":
		if req.Value >= 0 && req.Value <= 1 {
			conf.System.DuckScale = req.Value
			log.Printf("Duck Scale updated to: %.2f", req.Value)
			saveConfig()
		}
	case "duck_mic_pcm":
		conf.System.DuckMicPCM = !conf.System.DuckMicPCM
		log.Printf("Duck Mic PCM updated to: %v", conf.System.DuckMicPCM)
		saveConfig()
	case "duck_music_pcm":
		conf.System.DuckMusicPCM = !conf.System.DuckMusicPCM
		log.Printf("Duck Music PCM updated to: %v", conf.System.DuckMusicPCM)
		saveConfig()
	case "record_mic":
		conf.System.RecordMic = !conf.System.RecordMic
		setRecordMicEnabled(conf.System.RecordMic)
		log.Printf("Mic Capture updated to: %v", conf.System.RecordMic)
		saveConfig()
	case "record_voice":
		conf.System.RecordVoice = !conf.System.RecordVoice
		setRecordingEnabled(conf.System.RecordVoice)
		if !conf.System.RecordVoice {
			recorder.Stop()
		}
		log.Printf("Voice Recording updated to: %v", conf.System.RecordVoice)
		saveConfig()
	case "music_toggle":
		conf.System.MusicPlaying = !conf.System.MusicPlaying
		select {
		case pausemusic <- true:
		default:
		}
		log.Printf("Music playing updated to: %v", conf.System.MusicPlaying)
		saveConfig()
	case "cron_toggle":
		conf.System.EnableCron = !conf.System.EnableCron
		setCronEnabled(conf.System.EnableCron)
		if !conf.System.EnableCron {
			updateCronInfo("Cron Disabled")
		}
		log.Printf("Cron enabled updated to: %v", conf.System.EnableCron)
		saveConfig()
	case "time_toggle":
		conf.System.EnableTimePlay = !conf.System.EnableTimePlay
		setTimeEnabled(conf.System.EnableTimePlay)
		log.Printf("Time play enabled updated to: %v", conf.System.EnableTimePlay)
		saveConfig()
	}

	w.WriteHeader(http.StatusOK)
}

// 列出所有日期目录（如 2025-10-13）
func listDirs(w http.ResponseWriter, r *http.Request) {
	var dirs []string

	err := filepath.WalkDir(conf.System.RecoderFilePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(conf.System.RecoderFilePath, path)
		if err != nil || rel == "." {
			return nil
		}
		// 简单判断是否为 YYYY-MM-DD 格式
		if len(rel) == 10 && rel[4] == '-' && rel[7] == '-' {
			dirs = append(dirs, rel)
		}
		return nil
	})

	if err != nil {
		http.Error(w, "扫描目录失败", http.StatusInternalServerError)
		return
	}

	// 按日期排序（升序）
	//sort.Strings(dirs)

	// 按日期排序（降序：最新日期在前）
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dirs)
}

func listFilesInDir(w http.ResponseWriter, r *http.Request) {
	dirName := strings.TrimPrefix(r.URL.Path, "/dir/")
	dirPath := filepath.Join(conf.System.RecoderFilePath, dirName)

	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		http.Error(w, "目录不存在", http.StatusNotFound)
		return
	}

	var files []AudioFile
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".wav") {
			return nil
		}

		tm, err := parseTimeFromFilename(info.Name())
		if err != nil {
			log.Printf("跳过文件 %s: %v", info.Name(), err)
			return nil
		}

		// ✅ 正确构造 URL：/recordings/2025-10-14/filename.wav
		urlPath := "/recordings/" + dirName + "/" + info.Name()

		files = append(files, AudioFile{
			Name:      info.Name(),
			Timestamp: tm.Format("2006-01-02 15:04:05"),
			URL:       urlPath, // ✅ 使用正确路径
		})
		return nil
	})

	if err != nil {
		http.Error(w, "读取目录失败", http.StatusInternalServerError)
		return
	}

	// 按时间排序
	sort.Slice(files, func(i, j int) bool {
		ti, _ := time.Parse("2006-01-02 15:04:05", files[i].Timestamp)
		tj, _ := time.Parse("2006-01-02 15:04:05", files[j].Timestamp)
		return tj.Before(ti)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}
