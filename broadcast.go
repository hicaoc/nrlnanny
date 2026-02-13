package main

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	MsgTypeAudio      byte = 0x01
	MsgTypeVoiceStart byte = 0x02
	MsgTypeVoiceEnd   byte = 0x03
)

type LiveBroadcastHub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]struct{}

	activeCallSign string
	activeSSID     byte
	isVoiceActive  bool
}

var liveHub *LiveBroadcastHub

func init() {
	liveHub = &LiveBroadcastHub{
		clients: make(map[*websocket.Conn]struct{}),
	}
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (h *LiveBroadcastHub) AddClient(conn *websocket.Conn) {
	h.mu.Lock()
	h.clients[conn] = struct{}{}
	active := h.isVoiceActive
	cs := h.activeCallSign
	ssid := h.activeSSID
	h.mu.Unlock()

	if active {
		frame := buildFrame(MsgTypeVoiceStart, cs, ssid, nil)
		conn.WriteMessage(websocket.BinaryMessage, frame)
	}

	log.Printf("Live broadcast client connected. Total: %d", len(h.clients))
}

func (h *LiveBroadcastHub) RemoveClient(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
	conn.Close()
}

func (h *LiveBroadcastHub) BroadcastAudio(callsign string, ssid byte, pcmData []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.clients) == 0 {
		return
	}

	frame := buildFrame(MsgTypeAudio, callsign, ssid, pcmData)

	for conn := range h.clients {
		conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
		if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
			go h.RemoveClient(conn)
		}
	}
}

func (h *LiveBroadcastHub) NotifyVoiceStart(callsign string, ssid byte) {
	h.mu.Lock()
	h.isVoiceActive = true
	h.activeCallSign = callsign
	h.activeSSID = ssid
	h.mu.Unlock()

	h.broadcast(buildFrame(MsgTypeVoiceStart, callsign, ssid, nil))
}

func (h *LiveBroadcastHub) NotifyVoiceEnd(callsign string, ssid byte) {
	h.mu.Lock()
	h.isVoiceActive = false
	h.mu.Unlock()

	h.broadcast(buildFrame(MsgTypeVoiceEnd, callsign, ssid, nil))
}

func (h *LiveBroadcastHub) broadcast(frame []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for conn := range h.clients {
		conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
		if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
			go h.RemoveClient(conn)
		}
	}
}

func buildFrame(msgType byte, callsign string, ssid byte, data []byte) []byte {
	frame := make([]byte, 8+len(data))
	frame[0] = msgType

	cs := []byte(callsign)
	for i := 0; i < 6; i++ {
		if i < len(cs) {
			frame[1+i] = cs[i]
		}
	}

	frame[7] = ssid

	if len(data) > 0 {
		copy(frame[8:], data)
	}

	return frame
}

func handleLiveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	liveHub.AddClient(conn)
	defer liveHub.RemoveClient(conn)

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
