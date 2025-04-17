package transport

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// WebSocketTransport implements the Transport interface for WebSocket connections
type WebSocketTransport struct {
	addr      string
	upgrader  websocket.Upgrader
	clients   map[*websocket.Conn]bool
	clientsMu sync.Mutex
	broadcast chan interface{}
	server    *http.Server
}

// NewWebSocketTransport creates a new WebSocketTransport instance
func NewWebSocketTransport(addr string) *WebSocketTransport {
	wst := &WebSocketTransport{
		addr: addr,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for testing
			},
		},
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan interface{}, 256),
	}

	// Start server
	wst.start()
	return wst
}

// start begins the WebSocket server
func (wst *WebSocketTransport) start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wst.handleWebSocket)

	// Create HTTP server
	wst.server = &http.Server{
		Addr:    wst.addr,
		Handler: mux,
	}

	// Start HTTP server in a goroutine
	go func() {
		log.Printf("WebSocketTransport: Starting WebSocket server on %s", wst.addr)
		if err := wst.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("WebSocketTransport: Server error: %v", err)
		}
	}()

	// Start broadcast handler
	go wst.handleBroadcasts()
}

// handleWebSocket upgrades HTTP connections to WebSocket
func (wst *WebSocketTransport) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := wst.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocketTransport: Upgrade error: %v", err)
		return
	}

	// Register client
	wst.clientsMu.Lock()
	wst.clients[conn] = true
	wst.clientsMu.Unlock()
	log.Printf("WebSocketTransport: Client connected, total: %d", len(wst.clients))

	// Handle disconnect
	go func() {
		// Wait for close
		_, _, err := conn.ReadMessage()
		if err != nil {
			wst.clientsMu.Lock()
			delete(wst.clients, conn)
			wst.clientsMu.Unlock()
			conn.Close()
			log.Printf("WebSocketTransport: Client disconnected, total: %d", len(wst.clients))
		}
	}()
}

// handleBroadcasts sends messages to all connected clients
func (wst *WebSocketTransport) handleBroadcasts() {
	for data := range wst.broadcast {
		wst.clientsMu.Lock()
		for client := range wst.clients {
			if err := client.WriteJSON(data); err != nil {
				log.Printf("WebSocketTransport: Error sending to client: %v", err)
				client.Close()
				delete(wst.clients, client)
			}
		}
		wst.clientsMu.Unlock()
	}
}

// Send broadcasts data to all connected WebSocket clients
func (wst *WebSocketTransport) Send(data interface{}) error {
	select {
	case wst.broadcast <- data:
		// Message queued for broadcast
	default:
		// Channel full, drop message
		return nil
	}
	return nil
}

// Close shuts down the WebSocket server
func (wst *WebSocketTransport) Close() error {
	log.Println("WebSocketTransport: Closing server")

	// Close all client connections
	wst.clientsMu.Lock()
	for client := range wst.clients {
		client.Close()
	}
	wst.clients = make(map[*websocket.Conn]bool)
	wst.clientsMu.Unlock()

	// Close server
	if wst.server != nil {
		return wst.server.Close()
	}
	return nil
}

// Ensure WebSocketTransport satisfies the interface
var _ Transport = (*WebSocketTransport)(nil)
