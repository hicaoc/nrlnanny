package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

//var userlist = make(map[string]userinfo, 1000) //key 用户id

var (
	lastcallsign string
	lastssid     byte
	lasttime     time.Time
)

var dev *deviceInfo

func udpClient() error {

	dev = new(deviceInfo)
	//dev.ISOnline = true
	dev.CallSign = conf.System.Callsign
	//dev.SSID = conf.System.SSID
	dev.DevModel = 250
	dev.SSID = 250

	//创建到服务器的连接

	udpAddr, err := net.ResolveUDPAddr("udp", conf.System.Server+":"+conf.System.Port)

	if err != nil {
		log.Printf("Failed to resolve UDP address for server %v.", dev.udpSocket)
		return err
	}

	dev.udpSocket, err = net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "连接服务器失败: %v\n", err)
		os.Exit(1)
	}
	log.Printf("已连接服务器: %v %v\n", udpAddr, conf.System.Server)

	defer dev.udpSocket.Close()

	//conn.SetReadBuffer(5000)

	if err != nil {
		fmt.Println("read from connect failed, err:" + err.Error())
		os.Exit(1)
	}

	// /defer dev.udpSocket.Close()

	//启动服务器互联

	go dev.sendHeartbear()
	udpProcess(dev.udpSocket)

	return nil

}

func udpProcess(conn *net.UDPConn) {

	data := make([]byte, 1460)

	for {
		n, remoteaddr, err := conn.ReadFromUDP(data)
		if err != nil {
			fmt.Println("failed read udp msg, error: ", err)
			continue
		}

		nrl := &NRL21packet{}
		// nrl.UDPAddr = remoteaddr
		// nrl.UDPAddrStr = remoteaddr.String()
		// nrl.timeStamp = time.Now()

		err = nrl.decodeNRL21(data[:n])

		if err != nil {

			log.Printf("from %v, decode err %v  % X:", remoteaddr, err, data[:n])
			continue
			//break
			// <-limitChan
			// return
		}

		NRL21parser(nrl)

	}

}

func NRL21parser(nrl *NRL21packet) {

	switch nrl.Type {

	case 0: //控制指令，用户远程控制设备

	case 1: //G711音频数据
		PlayAndSaveVoice(nrl)

	case 2: // 心跳
		//log.Printf("recive heartbeat:%v-%v\n", nrl.CallSign, nrl.SSID)

	case 3:

	case 4:

	case 5: //文本消息

		DisplayMsg(nrl)

	case 6: //设备到设备控制通道

	case 7: //设备端操作指令

	case 8: // Opus音频数据

	case 9: //服务器互联

	default:
		fmt.Println("unknow data:", nrl.Type, nrl)
		//conn.WriteToUDP(packet, n.Addr)

	}

}

func PlayAndSaveVoice(nrl *NRL21packet) {

	if nrl.CallSign != lastcallsign || nrl.SSID != lastssid || time.Since(lasttime) > time.Second*2 {
		fmt.Println()
		log.Printf("Recived new voice %s-%d\n", nrl.CallSign, nrl.SSID)
		recorder.Stop()
		recorder = NewRecorder(fmt.Sprintf("%s-%d", nrl.CallSign, nrl.SSID))

		go func() {
			for {
				time.Sleep(time.Second * 2)
				if time.Since(lasttime) > time.Second*2 {
					recorder.Stop()
				}

			}

		}()

		// go func() {
		// 	<-time.After(3 * time.Second)
		// 	if time.Since(lasttime) > time.Second*2 {
		// 		recorder.Stop()
		// 	}
		// }()

	}

	lasttime = time.Now()
	lastcallsign = nrl.CallSign
	lastssid = nrl.SSID

	//pcmbuffer := make([]int16, len(nrl.DATA))

	chunkBytes := make([]byte, len(nrl.DATA)*2)

	for i := range nrl.DATA {
		//pcmbuffer[i] = alaw2linear(nrl.DATA[i])
		binary.LittleEndian.PutUint16(chunkBytes[i*2:], uint16(alaw2linear(nrl.DATA[i])))

	}

	//log.Println("play voice", nrl.CallSign, nrl.SSID)

	streamReader.WriteChunk(chunkBytes)

	recorder.ProcessPCMData(chunkBytes)

}

// 文本消息
func DisplayMsg(nrl *NRL21packet) {
	fmt.Println("recived msg:", string(nrl.DATA))

}

// forwardCtl forwardCtl
