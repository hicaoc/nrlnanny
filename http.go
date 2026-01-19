package main

import (
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
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/dirs", listDirs)       // 获取所有日期目录
	http.HandleFunc("/dir/", listFilesInDir) // 获取某目录下文件
	http.Handle("/recordings/", http.StripPrefix("/recordings/", http.FileServer(http.Dir(conf.System.RecoderFilePath))))

	log.Printf("服务器启动中：http://0.0.0.0:%s\n", conf.System.WebPort)
	log.Fatal(http.ListenAndServe(":"+conf.System.WebPort, nil))
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, "./play.html")
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
