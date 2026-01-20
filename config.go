package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

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
		AudioFile       string  `yaml:"AudioFile" json:"audio_file"`
		AudioFilePath   string  `yaml:"AudioFilePath" json:"audio_file_Path"`
		MusicFilePath   string  `yaml:"MusicFilePath" json:"music_file_Path"`
		RecoderFilePath string  `yaml:"RecoderFilePath" json:"Path"`
		CronString      string  `yaml:"CronString" json:"cronString"`
		WebPort         string  `yaml:"WebPort" json:"web_port"`
	} `yaml:"System" json:"system"`
}

var conf = &config{}

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

// Exist 判断文件存在
func Exist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
	// || os.IsExist(err)
}
