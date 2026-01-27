package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/insider-one/notification-service/internal/domain"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for development
		// In production, validate against allowed origins
		return true
	},
}

// WebSocketHub maintains active WebSocket connections
type WebSocketHub struct {
	clients    map[*WebSocketClient]bool
	broadcast  chan *StatusUpdate
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	logger     *slog.Logger
	mu         sync.RWMutex
}

// WebSocketClient represents a WebSocket client connection
type WebSocketClient struct {
	hub    *WebSocketHub
	conn   *websocket.Conn
	send   chan []byte
	id     string
	filter *ClientFilter
}

// ClientFilter represents subscription filters
type ClientFilter struct {
	NotificationIDs []uuid.UUID      `json:"notification_ids,omitempty"`
	BatchIDs        []uuid.UUID      `json:"batch_ids,omitempty"`
	Channels        []domain.Channel `json:"channels,omitempty"`
}

// StatusUpdate represents a notification status update
type StatusUpdate struct {
	Type         string               `json:"type"`
	Notification *domain.Notification `json:"notification"`
	Timestamp    time.Time            `json:"timestamp"`
}

// SubscribeMessage represents a subscription request from client
type SubscribeMessage struct {
	Action string       `json:"action"`
	Filter ClientFilter `json:"filter"`
}

// NewWebSocketHub creates a new WebSocketHub
func NewWebSocketHub(logger *slog.Logger) *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*WebSocketClient]bool),
		broadcast:  make(chan *StatusUpdate, 256),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
		logger:     logger,
	}
}

// Run starts the hub's main loop
func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.logger.Info("websocket client connected", "client_id", client.id)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			h.logger.Info("websocket client disconnected", "client_id", client.id)

		case update := <-h.broadcast:
			message, err := json.Marshal(update)
			if err != nil {
				h.logger.Error("failed to marshal status update", "error", err)
				continue
			}

			h.mu.RLock()
			for client := range h.clients {
				if client.shouldReceive(update.Notification) {
					select {
					case client.send <- message:
					default:
						// Client buffer full, skip
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastStatus broadcasts a notification status update
func (h *WebSocketHub) BroadcastStatus(notification *domain.Notification) {
	update := &StatusUpdate{
		Type:         "status_update",
		Notification: notification,
		Timestamp:    time.Now().UTC(),
	}

	select {
	case h.broadcast <- update:
	default:
		h.logger.Warn("broadcast channel full, dropping update")
	}
}

// GetClientCount returns the number of connected clients
func (h *WebSocketHub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// shouldReceive checks if the client should receive the notification update
func (c *WebSocketClient) shouldReceive(notification *domain.Notification) bool {
	if c.filter == nil {
		return true // No filter, receive all
	}

	// Check notification ID filter
	if len(c.filter.NotificationIDs) > 0 {
		for _, id := range c.filter.NotificationIDs {
			if id == notification.ID {
				return true
			}
		}
	}

	// Check batch ID filter
	if len(c.filter.BatchIDs) > 0 && notification.BatchID != nil {
		for _, id := range c.filter.BatchIDs {
			if id == *notification.BatchID {
				return true
			}
		}
	}

	// Check channel filter
	if len(c.filter.Channels) > 0 {
		for _, ch := range c.filter.Channels {
			if ch == notification.Channel {
				return true
			}
		}
	}

	// If filters are set but none match
	if len(c.filter.NotificationIDs) > 0 || len(c.filter.BatchIDs) > 0 || len(c.filter.Channels) > 0 {
		return false
	}

	return true
}

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	hub *WebSocketHub
}

// NewWebSocketHandler creates a new WebSocketHandler
func NewWebSocketHandler(hub *WebSocketHub) *WebSocketHandler {
	return &WebSocketHandler{hub: hub}
}

// HandleWebSocket handles WebSocket upgrade and connection
// @Summary WebSocket connection
// @Description Connect to WebSocket for real-time notification updates
// @Tags websocket
// @Success 101 {string} string "Switching Protocols"
// @Router /ws [get]
func (h *WebSocketHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.hub.logger.Error("failed to upgrade websocket", "error", err)
		return
	}

	client := &WebSocketClient{
		hub:  h.hub,
		conn: conn,
		send: make(chan []byte, 256),
		id:   uuid.New().String(),
	}

	h.hub.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// readPump pumps messages from the websocket connection to the hub
func (c *WebSocketClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.logger.Error("websocket error", "error", err)
			}
			break
		}

		// Parse subscription message
		var subMsg SubscribeMessage
		if err := json.Unmarshal(message, &subMsg); err != nil {
			continue
		}

		if subMsg.Action == "subscribe" {
			c.filter = &subMsg.Filter
			c.hub.logger.Info("client subscribed with filter",
				"client_id", c.id,
				"filter", c.filter,
			)
		} else if subMsg.Action == "unsubscribe" {
			c.filter = nil
		}
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
