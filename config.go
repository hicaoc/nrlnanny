package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	yaml "gopkg.in/yaml.v3"
)

type config struct {
	System struct {
		Server          string  `yaml:"Server" json:"server"`
		Port            string  `yaml:"Port" json:"port"`
		Callsign        string  `yaml:"Callsign" json:"callsign"`
		SSID            byte    `yaml:"SSID" json:"ssid"`
		Volume          float64 `yaml:"Volume" json:"volume"`               // 音量
		DuckScale       float64 `yaml:"DuckScale" json:"duck_scale"`        // 音量降低比例
		DuckMicPCM      bool    `yaml:"DuckMicPCM" json:"duck_mic_pcm"`     // 是否降低麦克风音量
		DuckMusicPCM    bool    `yaml:"DuckMusicPCM" json:"duck_music_pcm"` // 是否降低音乐音量
		RecordMic       bool    `yaml:"RecordMic" json:"record_mic"`        // 是否启用麦克风采集
		RecordVoice     bool    `yaml:"RecordVoice" json:"record_voice"`    // 是否启用通话录音
		EnableMusic     bool    `yaml:"EnableMusic" json:"enable_music"`    // 是否启用音乐播放
		EnableCron      bool    `yaml:"EnableCron" json:"enable_cron"`      // 是否启用信标播放
		EnableTimePlay  bool    `yaml:"EnableTimePlay" json:"enable_time"`  // 是否启用定时点播放
		MusicPlaying    bool    `yaml:"MusicPlaying" json:"music_playing"`  // 是否处于播放状态
		AudioFile       string  `yaml:"AudioFile" json:"audio_file"`
		AudioFilePath   string  `yaml:"AudioFilePath" json:"audio_file_Path"`
		MusicFilePath   string  `yaml:"MusicFilePath" json:"music_file_Path"`
		RecoderFilePath string  `yaml:"RecoderFilePath" json:"Path"`
		CronString      string  `yaml:"CronString" json:"cronString"`
		WebPort         string  `yaml:"WebPort" json:"web_port"`
	} `yaml:"System" json:"system"`
}

var conf = &config{}
var confPath string
var confMu sync.Mutex

func (c *config) init() {

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Printf("get config filepath err #%v ", err)
		os.Exit(1)
	}

	confpath := dir + "/nrlnanny.yaml"

	cc := flag.String("c", confpath, "config file path and name")
	oo := flag.String("o", "", "print config content to stdout and exit , yaml format")

	flag.Parse()

	if *cc != "" {
		confpath = *cc
	}
	confPath = confpath

	conf.System.EnableMusic = true
	conf.System.EnableCron = true
	conf.System.EnableTimePlay = true
	conf.System.MusicPlaying = true
	conf.System.RecordVoice = true

	yamlFile, err := os.ReadFile(confpath)

	if err != nil {
		log.Printf("nrlnanny.yaml open err #%v ", err)
		os.Exit(1)

	}
	err = yaml.Unmarshal(yamlFile, conf)

	if err != nil {
		log.Fatalf("Unmarshal: %v \n %s", err, yamlFile)
	}

	// c.Parm.iDCfilterIPMap = make(map[uint32]bool, 0)
	// for _, v := range c.Parm.IDCfilterIP {
	// 	c.Parm.iDCfilterIPMap[ipstrToUInt32(v)] = true
	// }

	if *oo != "" {
		j, _ := yaml.Marshal(conf)
		fmt.Println(string(j))
		os.Exit(0)
	}

	if conf.System.WebPort == "" {
		conf.System.WebPort = "8080"
	}

}

func saveConfig() {
	confMu.Lock()
	defer confMu.Unlock()

	if confPath == "" {
		log.Printf("config path is empty, skip saving")
		return
	}

	data, err := yaml.Marshal(conf)
	if err != nil {
		log.Printf("marshal config failed: %v", err)
		return
	}
	if err := os.WriteFile(confPath, data, 0644); err != nil {
		log.Printf("write config failed: %v", err)
	}
}

// Exist 判断文件存在
func Exist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
	// || os.IsExist(err)
}
