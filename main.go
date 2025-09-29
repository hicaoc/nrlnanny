package main

import (
	"path/filepath"
	"strings"
)

// fmt.Println("heck database support ipv4:", db.IsIPv4())     // check database support ip type
// fmt.Println("check database support ip type:", db.IsIPv6()) // check database support ip type
// fmt.Println("database build time:", db.BuildTime())         // database build time
// fmt.Println("database support language:", db.Languages())   // database support language
// fmt.Println("database support fields:", db.Fields())        // database support fields
func main() {

	conf.init()

	if conf.System.AudioFile != "" && conf.System.CronString != "" {
		switch strings.ToLower(filepath.Ext(conf.System.AudioFile)) {
		case ".wav":
			readWAV()
		case ".mp3":
			ReadMP3()
		}
	}

	StartRecoder()

	go newplay()

	udpClient()
}
