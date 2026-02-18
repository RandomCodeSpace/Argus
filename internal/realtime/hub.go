package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// LogEntry is a lightweight struct for WebSocket broadcast payloads.
// It mirrors storage.Log but avoids importing the storage package for loose coupling.
type LogEntry struct {
	ID             uint      `json:"id"`
	TraceID        string    `json:"trace_id"`
	SpanID         string    `json:"span_id"`
	Severity       string    `json:"severity"`
	Body           string    `json:"body"`
	ServiceName    string    `json:"service_name"`
	AttributesJSON string    `json:"attributes_json"`
	AIInsight      string    `json:"ai_insight,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// Hub is a buffered WebSocket broadcast hub.
//
// Instead of broadcasting each log individually (which would freeze the UI at high throughput),
// it buffers logs and flushes them as a JSON array when either:
//   - Buffer size >= maxBufferSize (default: 100)
//   - Flush ticker fires (default: every 500ms)
type Hub struct {
	clients    map[*client]struct{}
	register   chan *client
	unregister chan *client
	broadcast  chan LogEntry

	buffer        []LogEntry
	bufferMu      sync.Mutex
	maxBufferSize int
	flushInterval time.Duration

	stopCh chan struct{}
	wg     sync.WaitGroup

	// onConnectionChange is called when the number of active connections changes.
	// Used to update Prometheus gauge.
	onConnectionChange func(count int)
}

// client represents a single WebSocket connection.
type client struct {
	conn *websocket.Conn
	send chan []byte
}

// NewHub creates a new buffered WebSocket hub.
func NewHub(onConnectionChange func(count int)) *Hub {
	return &Hub{
		clients:            make(map[*client]struct{}),
		register:           make(chan *client),
		unregister:         make(chan *client),
		broadcast:          make(chan LogEntry, 5000),
		buffer:             make([]LogEntry, 0, 100),
		maxBufferSize:      100,
		flushInterval:      500 * time.Millisecond,
		stopCh:             make(chan struct{}),
		onConnectionChange: onConnectionChange,
	}
}

// Run starts the hub's main event loop. Should be called in a goroutine.
func (h *Hub) Run() {
	h.wg.Add(1)
	defer h.wg.Done()

	flushTicker := time.NewTicker(h.flushInterval)
	defer flushTicker.Stop()

	for {
		select {
		case <-h.stopCh:
			// Flush remaining buffer before exit
			h.flush()
			return

		case c := <-h.register:
			h.clients[c] = struct{}{}
			slog.Info("🔌 WebSocket client connected", "total", len(h.clients))
			if h.onConnectionChange != nil {
				h.onConnectionChange(len(h.clients))
			}

		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
				slog.Info("🔌 WebSocket client disconnected", "total", len(h.clients))
				if h.onConnectionChange != nil {
					h.onConnectionChange(len(h.clients))
				}
			}

		case entry := <-h.broadcast:
			h.bufferMu.Lock()
			h.buffer = append(h.buffer, entry)
			shouldFlush := len(h.buffer) >= h.maxBufferSize
			h.bufferMu.Unlock()

			if shouldFlush {
				h.flush()
			}

		case <-flushTicker.C:
			h.flush()
		}
	}
}

// flush sends the buffered logs as a JSON array to all connected clients.
func (h *Hub) flush() {
	h.bufferMu.Lock()
	if len(h.buffer) == 0 {
		h.bufferMu.Unlock()
		return
	}
	// Swap buffer
	batch := h.buffer
	h.buffer = make([]LogEntry, 0, h.maxBufferSize)
	h.bufferMu.Unlock()

	data, err := json.Marshal(batch)
	if err != nil {
		slog.Error("Hub: failed to marshal batch", "error", err)
		return
	}

	for c := range h.clients {
		select {
		case c.send <- data:
		default:
			// Client is too slow, disconnect it
			delete(h.clients, c)
			close(c.send)
			slog.Warn("Hub: slow client removed", "total", len(h.clients))
			if h.onConnectionChange != nil {
				h.onConnectionChange(len(h.clients))
			}
		}
	}
}

// Broadcast adds a log entry to the broadcast buffer.
func (h *Hub) Broadcast(entry LogEntry) {
	select {
	case h.broadcast <- entry:
	default:
		// Drop if internal channel is full to avoid blocking ingestion
	}
}

// Stop gracefully shuts down the hub.
func (h *Hub) Stop() {
	close(h.stopCh)
	h.wg.Wait()
	slog.Info("🛑 WebSocket hub stopped")
}

// HandleWebSocket is the HTTP handler that upgrades connections to WebSocket.
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Allow cross-origin for dev mode
	})
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}

	c := &client{
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.register <- c

	// Writer goroutine
	go func() {
		defer func() {
			h.unregister <- c
			conn.Close(websocket.StatusNormalClosure, "closing")
		}()

		for msg := range c.send {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := conn.Write(ctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				slog.Debug("WebSocket write failed", "error", err)
				return
			}
		}
	}()

	// Reader goroutine — keeps connection alive, handles close
	for {
		_, _, err := conn.Read(context.Background())
		if err != nil {
			break
		}
	}
}
