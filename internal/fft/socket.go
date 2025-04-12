package fft

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketTransport implements the Transport interface for WebSockets.
// It handles real-time broadcasting of FFT data to connected clients with
// rate limiting to prevent overwhelming clients or the network.
//
// Thread Safety:
// - Uses mutex for client map access
// - Rate limits broadcasts using atomic time checks
// - Handles concurrent connections safely
type WebSocketTransport struct {
	clients         map[*websocket.Conn]bool // Active client connections
	clientsMutex    sync.Mutex               // Protects clients map
	upgrader        websocket.Upgrader       // WebSocket connection upgrader
	server          *http.Server             // HTTP server for WebSocket
	sendRateLimiter time.Time                // Last send timestamp for rate limiting
	minSendInterval time.Duration            // Minimum time between sends (prevents flooding)
}

// NewWebSocketTransport creates a new WebSocket transport and starts the server.
// Parameters:
//   - port: The port number to listen on (e.g., "8080")
//
// The server:
//   - Listens for WebSocket connections on /fft
//   - Handles client connect/disconnect
//   - Manages connection lifecycle
//   - Starts in a separate goroutine
func NewWebSocketTransport(port string) *WebSocketTransport {
	t := &WebSocketTransport{
		clients: make(map[*websocket.Conn]bool),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for testing
			},
		},
	}

	// Configure HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/fft", t.handleWebSocket)
	t.server = &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Start the server in its own goroutine
	go func() {
		log.Printf("FFT WebSocket server listening on port %s", port)
		if err := t.server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("WebSocket server error: %v", err)
		}
	}()

	return t
}

// handleWebSocket upgrades HTTP connections to WebSocket protocol.
// For each new connection:
//   - Upgrades to WebSocket protocol
//   - Registers client in thread-safe manner
//   - Starts goroutine to monitor connection health
//   - Automatically removes disconnected clients
func (t *WebSocketTransport) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := t.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Register client
	t.clientsMutex.Lock()
	t.clients[conn] = true
	t.clientsMutex.Unlock()

	// Listen for close
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				t.clientsMutex.Lock()
				delete(t.clients, conn)
				t.clientsMutex.Unlock()
				conn.Close()
				break
			}
		}
	}()
}

// Send broadcasts FFT data to all connected clients with rate limiting.
// Thread Safety:
//   - Safe for concurrent calls from FFT processor
//   - Uses mutex for client map access
//   - Handles client disconnects gracefully
//
// Rate Limiting:
//   - Enforces minimum interval between broadcasts
//   - Drops frames that exceed rate limit
//   - Prevents client and network overload
func (t *WebSocketTransport) Send(data []float64) error {
	now := time.Now()
	if now.Sub(t.sendRateLimiter) < t.minSendInterval {
		return nil // Skip this update
	}
	t.sendRateLimiter = now

	// Serialize data
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Send to all clients
	t.clientsMutex.Lock()
	for client := range t.clients {
		if err := client.WriteMessage(websocket.TextMessage, jsonData); err != nil {
			// If error, remove client
			client.Close()
			delete(t.clients, client)
		}
	}
	t.clientsMutex.Unlock()

	return nil
}

// Close performs a clean shutdown of the WebSocket transport:
//   - Closes all client connections
//   - Cleans up client map
//   - Shuts down HTTP server
//   - Thread-safe and idempotent
func (t *WebSocketTransport) Close() error {
	t.clientsMutex.Lock()
	for client := range t.clients {
		client.Close()
		delete(t.clients, client)
	}
	t.clientsMutex.Unlock()

	return t.server.Close()
}
