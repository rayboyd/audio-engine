// SPDX-License-Identifier: MIT
package udp

import (
	applog "audio/internal/log"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
)

// UDPSender handles sending data packets over a UDP connection.
// It uses a "connected" UDP socket (via net.DialUDP) for potentially
// better performance and simpler sending logic, as the destination address
// is fixed upon creation. It also handles graceful closing and provides
// conditional logging for common network errors like "connection refused".
type UDPSender struct {
	conn       *net.UDPConn // The underlying "connected" UDP socket.
	targetAddr *net.UDPAddr // The resolved target UDP address.
	mu         sync.Mutex   // Protects conn and closed status during concurrent access (e.g., Send vs Close).
	closed     bool         // Flag indicating if the sender has been closed.
	debug      bool         // Controls logging verbosity, specifically for connection refused errors.
}

// NewUDPSender creates and initializes a new UDPSender targeting the specified address.
// It resolves the address string and establishes a "connected" UDP socket using net.DialUDP.
// The debug flag controls whether transient "connection refused" errors are logged (at Debug level)
// or suppressed during Send operations. Other errors are always logged at Error level.
func NewUDPSender(targetAddress string, debug bool) (*UDPSender, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", targetAddress)
	if err != nil {
		applog.Errorf("UDP Sender: Failed to resolve target address '%s': %v", targetAddress, err)
		return nil, fmt.Errorf("failed to resolve UDP target address '%s': %w", targetAddress, err)
	}

	// Use DialUDP to create a UDP socket that is "connected" to the target address.
	// This associates the remote address with the socket, allowing the use of Write()
	// instead of WriteToUDP(), and potentially enabling the OS to report ICMP errors
	// like "connection refused" (though this behavior can vary by OS).
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		applog.Errorf("UDP Sender: Failed to dial UDP for target '%s': %v", targetAddress, err)
		return nil, fmt.Errorf("failed to dial UDP for target '%s': %w", targetAddress, err)
	}
	applog.Infof("UDP Sender: Connection established to %s (Debug logging: %v)", conn.RemoteAddr().String(), debug)

	return &UDPSender{
		conn:       conn,
		targetAddr: udpAddr,
		debug:      debug,
		// mu, closed have zero values (unlocked, false)
	}, nil
}

// Send transmits the given byte slice as a single UDP packet to the pre-configured target address.
// It is safe for concurrent use.  If a "connection refused" error occurs (e.g., ICMP port unreachable),
// it is only logged if the debug flag was set to true during initialization; otherwise, it's suppressed
// to avoid log spam.  Other network errors encountered during sending are always logged at the Error level.
func (s *UDPSender) Send(data []byte) error {
	s.mu.Lock() // Lock to prevent racing with Close()
	// Check if the sender has been closed already.
	if s.closed {
		s.mu.Unlock()
		// Return an error immediately if closed.
		return fmt.Errorf("UDP sender is closed")
	}

	// Use Write() on the "connected" UDP socket.
	// This sends the data directly to the target address associated during DialUDP.
	_, err := s.conn.Write(data)
	s.mu.Unlock() // Unlock immediately after the network operation.

	// Handle potential errors from the Write operation.
	if err != nil {
		isConnRefused := false
		// Check specifically for "connection refused" errors. This often indicates
		// that no process is listening on the target port, causing the OS to send
		// back an ICMP Port Unreachable message, which Go surfaces as ECONNREFUSED.
		if opError, ok := err.(*net.OpError); ok {
			if sysErr, ok := opError.Err.(*os.SyscallError); ok {
				// Use errors.Is for robust checking against the specific syscall error.
				if errors.Is(sysErr.Err, syscall.ECONNREFUSED) {
					isConnRefused = true
				}
			}
			// Fallback: Check the error string as well, as error wrapping might vary.
			if !isConnRefused && strings.Contains(opError.Err.Error(), "connection refused") {
				isConnRefused = true
			}
		}

		// Conditional logging based on error type and debug flag.
		if isConnRefused {
			// Only log "connection refused" if debug mode is enabled.
			if s.debug {
				applog.Debugf("UDP Sender: Send error (connection refused): %v", err)
			}
			// If not debugging, suppress this specific error to avoid log noise.
		} else {
			// Log all other types of send errors at the Error level.
			applog.Errorf("UDP Sender: Send error: %v", err)
		}
		// Wrap the original error and return it to the caller.
		return fmt.Errorf("failed to send UDP packet: %w", err)
	}

	// No error occurred.
	return nil
}

// Close closes the underlying UDP connection.
// It ensures the connection is closed only once and is safe for concurrent use.
// Returns an error if closing the connection fails.
func (s *UDPSender) Close() error {
	s.mu.Lock() // Lock to ensure exclusive access during close
	defer s.mu.Unlock()

	// Check if already closed to prevent redundant closing operations.
	if s.closed {
		applog.Debugf("UDP Sender: Close called but already closed.")
		return nil
	}

	// Mark as closed immediately within the lock.
	s.closed = true

	// Check if the connection exists before trying to close it.
	if s.conn != nil {
		applog.Infof("UDP Sender: Closing connection to %s", s.conn.RemoteAddr().String())
		// Close the UDP socket.
		err := s.conn.Close()
		s.conn = nil // Set to nil after closing to prevent further use.

		// Handle potential errors during the close operation.
		if err != nil {
			applog.Errorf("UDP Sender: Error closing connection: %v", err)
			// Wrap the error and return it.
			return fmt.Errorf("failed to close UDP connection: %w", err)
		}
		// Close successful.
		return nil
	}

	// Connection was already nil (shouldn't happen in normal flow but handle defensively).
	applog.Warnf("UDP Sender: Close called but connection was already nil.")
	return nil
}

// Ensure UDPSender satisfies the io.Closer interface at compile time.
var _ interface{ Close() error } = (*UDPSender)(nil)
