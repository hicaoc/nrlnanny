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

// liveClient wraps a websocket.Conn with a buffered send channel.
// A dedicated writePump goroutine drains the channel and writes to the conn,
// preventing concurrent writes and isolating slow clients.
type liveClient struct {
	conn *websocket.Conn
	send chan []byte
	done chan struct{}
	once sync.Once
}

// stop closes the done channel and the underlying connection (idempotent).
func (c *liveClient) stop() {
	c.once.Do(func() {
		close(c.done)
		c.conn.Close()
	})
}

// writePump runs in its own goroutine, draining c.send and writing to the conn.
func (c *liveClient) writePump() {
	defer c.conn.Close()
	for {
		select {
		case msg := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				log.Printf("writePump error: %v", err)
				return
			}
		case <-c.done:
			return
		}
	}
}

type LiveBroadcastHub struct {
	mu      sync.RWMutex
	clients map[*liveClient]struct{}

	activeCallSign string
	activeSSID     byte
	isVoiceActive  bool
}

var liveHub *LiveBroadcastHub

func init() {
	liveHub = &LiveBroadcastHub{
		clients: make(map[*liveClient]struct{}),
	}
}

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func (h *LiveBroadcastHub) AddClient(c *liveClient) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	active := h.isVoiceActive
	cs := h.activeCallSign
	ssid := h.activeSSID
	h.mu.Unlock()

	if active {
		frame := buildFrame(MsgTypeVoiceStart, cs, ssid, nil)
		select {
		case c.send <- frame:
		default:
		}
	}

	h.mu.RLock()
	total := len(h.clients)
	h.mu.RUnlock()
	log.Printf("Live broadcast client connected. Total: %d", total)
}

func (h *LiveBroadcastHub) RemoveClient(c *liveClient) {
	h.mu.Lock()
	_, exists := h.clients[c]
	if exists {
		delete(h.clients, c)
	}
	h.mu.Unlock()

	if exists {
		c.stop()
		h.mu.RLock()
		total := len(h.clients)
		h.mu.RUnlock()
		log.Printf("Live broadcast client removed. Total: %d", total)
	}
}

func (h *LiveBroadcastHub) BroadcastAudio(callsign string, ssid byte, pcmData []byte) {
	h.mu.RLock()
	if len(h.clients) == 0 {
		h.mu.RUnlock()
		return
	}
	clients := make([]*liveClient, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	frame := buildFrame(MsgTypeAudio, callsign, ssid, pcmData)

	for _, c := range clients {
		select {
		case c.send <- frame:
		default:
			// Channel full — client too slow, just drop this frame
		}
	}
}

func (h *LiveBroadcastHub) NotifyVoiceStart(callsign string, ssid byte) {
	h.mu.Lock()
	h.isVoiceActive = true
	h.activeCallSign = callsign
	h.activeSSID = ssid
	h.mu.Unlock()

	frame := buildFrame(MsgTypeVoiceStart, callsign, ssid, nil)
	h.broadcast(frame)
}

func (h *LiveBroadcastHub) NotifyVoiceEnd(callsign string, ssid byte) {
	h.mu.Lock()
	h.isVoiceActive = false
	h.mu.Unlock()

	frame := buildFrame(MsgTypeVoiceEnd, callsign, ssid, nil)
	h.broadcast(frame)
}

func (h *LiveBroadcastHub) broadcast(frame []byte) {
	h.mu.RLock()
	clients := make([]*liveClient, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		select {
		case c.send <- frame:
		default:
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
	log.Printf("Live WS request from %s (UA: %s)", r.RemoteAddr, r.UserAgent())

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error from %s: %v", r.RemoteAddr, err)
		return
	}
	log.Printf("Live WS connected: %s", r.RemoteAddr)

	client := &liveClient{
		conn: conn,
		send: make(chan []byte, 256),
		done: make(chan struct{}),
	}

	liveHub.AddClient(client)

	// Start write pump in background goroutine
	go client.writePump()

	// Read pump (blocks here until connection error)
	defer func() {
		liveHub.RemoveClient(client)
		log.Printf("Live WS disconnected: %s", r.RemoteAddr)
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			log.Printf("Live WS ReadMessage error from %s: %v", r.RemoteAddr, err)
			return
		}
	}
}
