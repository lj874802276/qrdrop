package service

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 25 * time.Second
	maxMessageSize = 1024
	sendBuffer     = 16
)

// Client is a single subscriber of a session room. Writes go through the
// buffered send channel so that concurrent broadcasts never race on the socket.
type Client struct {
	hub     *Hub
	session string
	conn    *websocket.Conn
	send    chan []byte
}

// Hub routes realtime events to the subscribers of each session.
type Hub struct {
	mu    sync.RWMutex
	rooms map[string]map[*Client]struct{}
}

// NewHub creates an empty hub.
func NewHub() *Hub {
	return &Hub{rooms: make(map[string]map[*Client]struct{})}
}

// Broadcast delivers a JSON payload to every subscriber of session.
// Slow consumers are dropped rather than blocking the uploader.
func (h *Hub) Broadcast(session string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("ws: marshal broadcast: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.rooms[session] {
		select {
		case client.send <- data:
		default:
			log.Printf("ws: dropping message for slow subscriber in session %s", session)
		}
	}
}

// SubscriberCount reports how many hosts are listening on a session.
func (h *Hub) SubscriberCount(session string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[session])
}

// Serve registers conn for session and pumps messages until it disconnects.
// It blocks, so callers should invoke it from their handler goroutine.
func (h *Hub) Serve(session string, conn *websocket.Conn) {
	client := &Client{
		hub:     h,
		session: session,
		conn:    conn,
		send:    make(chan []byte, sendBuffer),
	}

	h.register(client)
	go client.writePump()
	client.readPump()
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[c.session] == nil {
		h.rooms[c.session] = make(map[*Client]struct{})
	}
	h.rooms[c.session][c] = struct{}{}
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[c.session]
	if !ok {
		return
	}
	if _, ok := room[c]; !ok {
		return
	}
	delete(room, c)
	close(c.send)

	if len(room) == 0 {
		delete(h.rooms, c.session)
	}
}

// readPump drains inbound frames (QRDrop only pushes) and detects disconnects.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// writePump serialises outbound frames and keeps the connection alive.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
