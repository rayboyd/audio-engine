package udp

import (
	"fmt"
	"log"
	"net"
	"sync"
)

// UDPSender handles sending data packets over UDP.
type UDPSender struct {
	conn       *net.UDPConn
	targetAddr *net.UDPAddr
	mu         sync.Mutex // Protects conn during Close
	closed     bool
}

// NewUDPSender creates a new UDPSender targeting the specified address.
// The address should be in the format "host:port", e.g., "127.0.0.1:9090".
func NewUDPSender(targetAddress string) (*UDPSender, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", targetAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve UDP target address '%s': %w", targetAddress, err)
	}

	// We don't need to bind to a specific local port for sending,
	// so we use nil for the local address in DialUDP.
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial UDP for target '%s': %w", targetAddress, err)
	}

	log.Printf("UDP Sender: Connection established to %s", conn.RemoteAddr().String())

	return &UDPSender{
		conn:       conn,
		targetAddr: udpAddr, // Store for logging/reference if needed
	}, nil
}

// Send transmits the given byte slice as a UDP packet.
// It is safe for concurrent use, although typically called sequentially by the publisher.
func (s *UDPSender) Send(data []byte) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("UDP sender is closed")
	}
	// Keep the lock during the write operation to prevent concurrent Close/Write issues.
	// UDP Write is generally fast but can block under certain OS/network conditions.
	_, err := s.conn.Write(data)
	s.mu.Unlock() // Unlock immediately after write

	if err != nil {
		// Log errors but don't necessarily stop everything unless it's critical
		log.Printf("UDP Sender: Error sending packet: %v", err)
		return fmt.Errorf("failed to send UDP packet: %w", err)
	}
	// log.Printf("UDP Sender: Sent %d bytes", n) // Optional: Log successful sends (can be noisy)
	return nil
}

// Close closes the underlying UDP connection.
func (s *UDPSender) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil // Already closed
	}

	s.closed = true
	if s.conn != nil {
		log.Printf("UDP Sender: Closing connection to %s", s.conn.RemoteAddr().String())
		err := s.conn.Close()
		s.conn = nil // Prevent further use
		if err != nil {
			log.Printf("UDP Sender: Error closing connection: %v", err)
			return fmt.Errorf("failed to close UDP connection: %w", err)
		}
		return nil
	}
	return nil
}

// Ensure UDPSender satisfies the io.Closer interface (useful for engine closables)
var _ interface{ Close() error } = (*UDPSender)(nil)
