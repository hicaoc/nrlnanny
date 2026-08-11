package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func apiTTSSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request struct {
		Text  string  `json:"text"`
		Voice string  `json:"voice"`
		Rate  float64 `json:"rate"`
		Pitch float64 `json:"pitch"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || strings.TrimSpace(request.Text) == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}
	if request.Rate == 0 {
		request.Rate = 1
	}
	go sendTTSVoice(request.Text, request.Voice, request.Rate, request.Pitch)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"code": 20000, "message": "TTS started"})
}

func apiTTSTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		ttsTasksLock.Lock()
		tasks := append([]TTScheduledTask(nil), ttsTasks...)
		ttsTasksLock.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 20000, "tasks": tasks})
	case http.MethodPost:
		var task TTScheduledTask
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&task); err != nil || strings.TrimSpace(task.Text) == "" || task.Time == "" {
			http.Error(w, "text and time are required", http.StatusBadRequest)
			return
		}
		if _, err := time.Parse("15:04", task.Time); err != nil {
			http.Error(w, "time must use HH:MM", http.StatusBadRequest)
			return
		}
		task.ID = time.Now().UnixMilli()
		task.Enabled = true
		ttsTasksLock.Lock()
		ttsTasks = append(ttsTasks, task)
		ttsTasksLock.Unlock()
		saveTTSTasks()
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 20000, "task": task})
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func apiTTSTask(w http.ResponseWriter, r *http.Request) {
	idText := strings.TrimPrefix(r.URL.Path, "/api/tts/task/")
	var id int64
	if _, err := fmt.Sscan(idText, &id); err != nil || id <= 0 {
		http.Error(w, "invalid task ID", http.StatusBadRequest)
		return
	}
	found := false
	ttsTasksLock.Lock()
	switch r.Method {
	case http.MethodDelete:
		for i, task := range ttsTasks {
			if task.ID == id {
				ttsTasks = append(ttsTasks[:i], ttsTasks[i+1:]...)
				found = true
				break
			}
		}
	case http.MethodPut:
		var update struct {
			Enabled *bool `json:"enabled"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4*1024)).Decode(&update); err == nil && update.Enabled != nil {
			for i := range ttsTasks {
				if ttsTasks[i].ID == id {
					ttsTasks[i].Enabled = *update.Enabled
					found = true
					break
				}
			}
		}
	}
	ttsTasksLock.Unlock()
	if r.Method != http.MethodDelete && r.Method != http.MethodPut {
		w.Header().Set("Allow", "DELETE, PUT")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !found {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	saveTTSTasks()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"code": 20000})
}
