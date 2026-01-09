package main

import (
	"fmt"
	"time"
)

// fmt.Println("heck database support ipv4:", db.IsIPv4())     // check database support ip type
// fmt.Println("check database support ip type:", db.IsIPv6()) // check database support ip type
// fmt.Println("database build time:", db.BuildTime())         // database build time
// fmt.Println("database support language:", db.Languages())   // database support language
// fmt.Println("database support fields:", db.Fields())        // database support fields
func main() {

	conf.init()

	cpuid = calculateCpuId(fmt.Sprintf("%s-%d", conf.System.Callsign, conf.System.SSID))

	go StartRecoder()

	go newplay()

	time.Sleep(time.Second * 1)

	go startcron()

	go play()

	go playAudio()

	go playMusic()

	udpClient()
}
