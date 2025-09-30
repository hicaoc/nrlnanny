package main

import (
	"net"
	"time"
)

type deviceInfo struct {
	ID       int    `json:"id" db:"id"` //设备唯一编号
	Name     string `json:"name" db:"name"`
	CPUID    string `json:"cpuid" db:"cpuid"`       //设备CPUID
	Password string `json:"password" db:"password"` //设备接入密码
	//DevType      byte   `json:"dev_type" db:"dev_type"`   //设备型号
	DevModel byte   `json:"dev_model" db:"dev_model"` //设备型号
	CallSign string `json:"callsign" db:"callsign"`   //所有者呼号
	SSID     byte   `json:"ssid" db:"ssid"`           //所有者SSID

	CallSignSSID string `json:"callsignssid"` //callsign+ssid
	udpSocket    *net.UDPConn
}

func (d *deviceInfo) sendHeartbear() {

	cpuid := calculateCpuId(d.CallSign + "-250")

	packet := encodeNRL21(d.CallSign, 250, 2, 250, cpuid, []byte{})

	for {

		//发送心跳包
		//log.Println("send hb:", d.udpSocket)
		_, err := d.udpSocket.Write(packet)
		if err != nil {
			//log.Println("send hb err:", err)
			time.Sleep(time.Second * 5)
		}
		time.Sleep(time.Second * 2)

	}

}
