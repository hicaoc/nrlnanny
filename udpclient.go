package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

//var userlist = make(map[string]userinfo, 1000) //key 用户id

var (
	lastcallsign string
	lastssid     byte
	lasttime     time.Time
)

var dev *deviceInfo

func udpClient() {

	dev = new(deviceInfo)
	//dev.ISOnline = true
	dev.CallSign = conf.System.Callsign
	//dev.SSID = conf.System.SSID
	dev.DevModel = 250
	dev.SSID = conf.System.SSID

	//创建到服务器的连接

	udpAddr, err := net.ResolveUDPAddr("udp", conf.System.Server+":"+conf.System.Port)

	if err != nil {
		log.Printf("Failed to resolve UDP address for server %v.", dev.udpSocket)
		return
	}

	dev.udpSocket, err = net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		log.Printf("连接服务器失败: %v\n", err)
		os.Exit(1)
	}
	log.Printf("已连接服务器: %v %v\n", udpAddr, conf.System.Server)

	defer dev.udpSocket.Close()

	//conn.SetReadBuffer(5000)

	// /defer dev.udpSocket.Close()

	//启动服务器互联

	go recivePCM()
	go dev.sendHeartbear()
	udpProcess(dev.udpSocket)

}

func udpProcess(conn *net.UDPConn) {

	data := make([]byte, 1460)

	for {
		n, remoteaddr, err := conn.ReadFromUDP(data)
		if err != nil {
			log.Println("failed read udp msg, error: ", err)
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

	case 11: //NRL AT指令
		//log.Printf("NRL AT指令:%v \n", string(nrl.DATA))

		at := decodeAT(nrl.DATA)
		if at == nil {
			log.Printf("AT指令错误: %v %v \n", string(nrl.DATA), nrl.DATA)
			return
		}

		log.Printf("AT指令:%v=%v \n", at.command, at.value)

		switch at.command {
		case "AT+PLAY_ID":
			id, err := strconv.Atoi(at.value)
			if err != nil {
				return
			}
			PlayMusicByID(id)
		case "AT+PAUSE":
			select {
			case pausemusic <- true:
			default:
			}
		case "AT+NEXT":
			select {
			case nextmusic <- true:
			default:
			}
		case "AT+PREW":
			select {
			case lastmusic <- true:
			default:
			}
		case "AT+VOLUME":

			log.Println("AT+VOLUME", at.value)

			value, err := strconv.Atoi(at.value)
			if err != nil {
				log.Println("AT+VOLUME", err)
				return
			}

			if value < 0 || value > 100 {
				log.Println("AT+VOLUME", value)
				return
			}
			conf.System.Volume = float64(value) / 100
			updateVolumeDisplay()
			log.Println("VOLUME", conf.System.Volume)

		}

		volume := fmt.Sprintf("%d", int(conf.System.Volume*100))
		atcommand := encodeAT([]string{"AT+PLAY_ID=1", "AT+PREW=1", "AT+NEXT=1", "AT+PAUSE=1", "AT+VOLUME=" + volume})

		packet := encodeNRL21(conf.System.Callsign, conf.System.SSID, 11, 250, calculateCpuId(conf.System.Callsign+string(conf.System.SSID)), atcommand)
		dev.udpSocket.Write(packet)

	default:
		log.Println("unknow data:", nrl.Type, nrl)
		//conn.WriteToUDP(packet, n.Addr)

	}

}

type atCommand struct {
	command string
	value   string
}

func decodeAT(data []byte) *atCommand {
	if len(data) < 2 {
		return nil
	}

	if data[0] != 0x01 {
		return nil
	}

	str := strings.Split(string(data[1:]), "=")

	if len(str) != 2 {
		return nil
	}

	at := atCommand{}

	at.command = str[0]

	at.value = strings.TrimSuffix(str[1], "\r\n")

	return &at

}

func encodeAT(atlist []string) []byte {
	at := make([]byte, 0)
	at = append(at, 0x02)
	at = append(at, "NRLNANNY V2.0\r\n"...)
	at = append(at, strings.Join(atlist, "\r\n")...)
	at = append(at, "\r\n"...)

	return at

}

func PlayAndSaveVoice(nrl *NRL21packet) {

	if nrl.CallSign != lastcallsign || nrl.SSID != lastssid || time.Since(lasttime) > time.Second*2 {
		// fmt.Println()
		log.Printf("[%s-%d] 新语音呼叫\n", nrl.CallSign, nrl.SSID)
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
	log.Println("收到文本消息:", string(nrl.DATA))

}

// forwardCtl forwardCtl
