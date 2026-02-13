package main

import (
	"fmt"
	"sync"
	"time"
)

// fmt.Println("heck database support ipv4:", db.IsIPv4())     // check database support ip type
// fmt.Println("check database support ip type:", db.IsIPv6()) // check database support ip type
// fmt.Println("database build time:", db.BuildTime())         // database build time
// fmt.Println("database support language:", db.Languages())   // database support language
// fmt.Println("database support fields:", db.Fields())        // database support fields

var cronPCM = make(chan [][]int, 3)
var timePCM = make(chan [][]int, 3)
var musicPCM = make(chan [][]int, 3)
var micPCM = make(chan [][]int, 3)

var nextmusic = make(chan bool, 1)
var lastmusic = make(chan bool, 1)
var pausemusic = make(chan bool, 1)

var (
	GlobalLogBuffer []string
	logMu           sync.Mutex
)

const maxLogLines = 100

func main() {

	conf.init()
	initWebLogCapture()

	cpuid = calculateCpuId(fmt.Sprintf("%s-%d", conf.System.Callsign, conf.System.SSID))

	go StartRecoder()
	go MicRun()

	// go newplay() // 本地监听已通过浏览器实现，不再需要本地音频输出

	time.Sleep(time.Second * 1)

	go startcron()

	go play()

	go playAudio()

	go playMusic()

	// Remove old keyboard listener
	// go keyboardrun()

	// Run UDP client in background
	go udpClient()

	select {}
}
