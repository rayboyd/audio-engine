// filepath: /Users/ray/2025/grec-v2/internal/transport/logging.go
package transport

import (
	"log"
)

// LoggingTransport implements the Transport interface by logging data to the console.
type LoggingTransport struct{}

// NewLoggingTransport creates a new LoggingTransport instance.
func NewLoggingTransport() *LoggingTransport {
	log.Println("Transport: Using LoggingTransport")
	return &LoggingTransport{}
}

// Send logs the received data to the standard logger.
func (lt *LoggingTransport) Send(data interface{}) error {
	// Attempt to marshal for pretty printing, but log raw if it fails
	// jsonData, err := json.Marshal(data)
	// if err != nil {
	// 	// Log type and raw data if marshaling fails
	// 	log.Printf("LOG_TRANSPORT: Received (%T): %+v (JSON marshal error: %v)", data, data, err)
	// } else {
	// 	// Log type and JSON representation
	// 	log.Printf("LOG_TRANSPORT: Received (%T): %s", data, string(jsonData))
	// }
	return nil // Logging transport never fails to "send"
}

// Close is a no-op for LoggingTransport.
func (lt *LoggingTransport) Close() error {
	log.Println("LOG_TRANSPORT: Close called.")
	return nil
}

// Ensure LoggingTransport satisfies the interface at compile time.
var _ Transport = (*LoggingTransport)(nil)
