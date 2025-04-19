// SPDX-License-Identifier: MIT
package udp

import (
	"audio/internal/analysis"
	applog "audio/internal/log"
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// UDPPublisher periodically fetches analysis results (specifically FFT magnitudes),
// packs them into a defined binary format, and sends them over UDP using a UDPSender.
// It runs in a separate goroutine managed by Start and Stop methods.
type UDPPublisher struct {
	sender   *UDPSender             // The underlying UDP sender instance.
	fftProc  *analysis.FFTProcessor // The FFT processor to fetch magnitude data from.
	interval time.Duration          // The interval at which packets are sent.

	ticker   *time.Ticker   // Ticker that triggers packet sending.
	doneChan chan struct{}  // Channel used to signal the publisher goroutine to stop.
	stopOnce sync.Once      // Ensures the stop logic runs only once per Start/Stop cycle.
	wg       sync.WaitGroup // Waits for the publisher goroutine to finish during Stop.
	mu       sync.Mutex     // Protects access to ticker and doneChan during Start/Stop.

	sequenceNum uint32 // Monotonically increasing sequence number for packets.

	// Pre-allocated buffers to reduce allocations in the hot path (buildAndSendPacket).
	udpMagBuffer []float64     // Buffer to receive float64 magnitudes from FFTProcessor.
	udpF32Buffer []float32     // Buffer to hold float32 magnitudes for binary packing.
	packetBuffer *bytes.Buffer // Reusable buffer for constructing the binary packet.
}

// NewUDPPublisher creates and initializes a new UDPPublisher.
// It requires a valid UDPSender and FFTProcessor.
// If the provided interval is invalid (<= 0), it defaults to 16ms (~60Hz).
func NewUDPPublisher(interval time.Duration, sender *UDPSender, fftProc *analysis.FFTProcessor) (*UDPPublisher, error) {
	if sender == nil {
		return nil, fmt.Errorf("UDPPublisher: UDP sender cannot be nil")
	}
	if fftProc == nil {
		return nil, fmt.Errorf("UDPPublisher: FFT processor cannot be nil")
	}

	if interval <= 0 {
		interval = 16 * time.Millisecond // Default to ~60Hz if invalid
		applog.Warnf("UDPPublisher: Invalid interval provided, defaulting to %s", interval)
	}

	// Determine required buffer size based on FFT size (N/2 + 1 bins)
	requiredLen := fftProc.GetFFTSize()/2 + 1
	applog.Infof("UDPPublisher: Initializing (Interval: %s, FFT Bins: %d)", interval, requiredLen)

	return &UDPPublisher{
		sender:       sender,
		fftProc:      fftProc,
		interval:     interval,
		udpMagBuffer: make([]float64, requiredLen), // Pre-allocate based on FFT size
		udpF32Buffer: make([]float32, requiredLen), // Pre-allocate based on FFT size
		packetBuffer: new(bytes.Buffer),            // Initialize the reusable packet buffer
		// mu, sequenceNum are zero-value ready
		// ticker, doneChan, stopOnce, wg are initialized in Start/Stop
	}, nil
}

// Start begins the periodic publishing process.
// It launches a goroutine that ticks at the configured interval, calling
// buildAndSendPacket on each tick until Stop is called.
// It is safe to call Start multiple times; subsequent calls are no-ops if already started.
func (p *UDPPublisher) Start() {
	p.mu.Lock()
	// Prevent starting if already running
	if p.ticker != nil {
		p.mu.Unlock()
		applog.Warnf("UDPPublisher: Start called but already running.")
		return
	}

	// Initialize resources for this run
	p.ticker = time.NewTicker(p.interval)
	p.doneChan = make(chan struct{})
	p.stopOnce = sync.Once{} // Reset stopOnce for this run

	// Capture local variables for the goroutine to avoid data races on p.ticker/p.doneChan
	ticker := p.ticker
	doneChan := p.doneChan

	p.mu.Unlock() // Unlock before starting the potentially long-running goroutine

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		applog.Infof("UDPPublisher: Publisher goroutine started (Interval: %s)", p.interval)
		for {
			select {
			case <-ticker.C:
				// Time to send a packet
				p.buildAndSendPacket()
			case <-doneChan:
				// Stop signal received
				applog.Infof("UDPPublisher: Publisher goroutine received stop signal.")
				return
			}
		}
	}()
}

// Stop gracefully signals the publisher goroutine to terminate and waits for it to exit.
// It stops the internal ticker and closes the done channel.
// It is safe to call Stop multiple times; subsequent calls are no-ops.
func (p *UDPPublisher) Stop() error {
	p.mu.Lock()
	// Check if already stopped or never started
	if p.ticker == nil {
		p.mu.Unlock()
		applog.Debugf("UDPPublisher: Stop called but not running.")
		return nil
	}

	// Use sync.Once to ensure stop logic (closing channel, stopping ticker) runs only once
	p.stopOnce.Do(func() {
		applog.Infof("UDPPublisher: Initiating stop sequence...")
		close(p.doneChan) // Signal the goroutine to exit
		p.ticker.Stop()   // Stop the ticker
		p.ticker = nil    // Mark as stopped
	})

	p.mu.Unlock() // Unlock before waiting

	// Wait for the publisher goroutine to finish processing the stop signal
	applog.Debugf("UDPPublisher: Waiting for publisher goroutine to finish...")
	p.wg.Wait()
	applog.Infof("UDPPublisher: Publisher goroutine finished.")
	return nil
}

/*
UDP Packet Structure (BigEndian) - See visual diagram below

+-----------------------------------------------------------------------------+
| Field             | Data Type      | Size (Bytes) | Description             |
|-------------------|----------------|--------------|-------------------------|
| Sequence Number   | uint32         | 4            | Monotonically increasing|
| Timestamp         | int64          | 8            | Nanoseconds since epoch |
| Magnitude Count   | uint16         | 2            | Number of floats (N)    |
| Magnitudes        | []float32      | N * 4        | Array of FFT magnitudes |
+-----------------------------------------------------------------------------+

Visual Layout:

|<---- 4 Bytes ---->|<------ 8 Bytes ------>|<-- 2 Bytes -->|<----- N * 4 Bytes ----->|
+-------------------+-----------------------+---------------+-------------------------+
|  Sequence Number  |       Timestamp       |   Magnitude   |       Magnitudes        |
|      (uint32)     |        (int64)        |     Count     |      (N * float32)      |
|                   |                       |     (uint16)  |                         |
+-------------------+-----------------------+---------------+-------------------------+
*/

// buildAndSendPacket is the core function executed on each ticker interval.
// It performs the following steps:
// 1. Fetches the latest FFT magnitudes from the processor.
// 2. Converts magnitudes from float64 to float32.
// 3. Packs the sequence number, timestamp, count, and magnitudes into a binary buffer.
// 4. Sends the resulting packet using the UDPSender.
func (p *UDPPublisher) buildAndSendPacket() {
	// --- 1. Fetch Data ---

	// Use GetMagnitudesInto to avoid allocations within the FFT processor.
	err := p.fftProc.GetMagnitudesInto(p.udpMagBuffer)
	if err != nil {
		applog.Errorf("UDPPublisher: Error getting magnitudes: %v", err)
		return // Skip sending this packet
	}

	// --- 2. Convert Data ---

	// Ensure the float32 buffer matches the expected length (should always match if initialized correctly).
	if len(p.udpF32Buffer) != len(p.udpMagBuffer) {
		applog.Errorf("UDPPublisher: Mismatched internal buffer lengths (%d != %d)! Resizing f32 buffer.",
			len(p.udpF32Buffer), len(p.udpMagBuffer))
		// Attempt to recover by resizing, although this indicates an initialization issue.
		p.udpF32Buffer = make([]float32, len(p.udpMagBuffer))
	}
	// Convert fetched float64 magnitudes to float32 for packing.
	for i, v := range p.udpMagBuffer {
		p.udpF32Buffer[i] = float32(v)
	}

	// --- 3. Pack Data ---

	// Prepare metadata for the packet header.
	p.sequenceNum++                               // Increment sequence number for this packet.
	timestamp := time.Now().UnixNano()            // Get current time for the timestamp.
	magnitudeCount := uint16(len(p.udpF32Buffer)) // Get the number of magnitude values.

	// Reset the reusable buffer before writing new packet data.
	p.packetBuffer.Reset()

	// Write header fields (Sequence, Timestamp, Count) using BigEndian byte order.
	// Chain error checks for cleaner code.
	err = binary.Write(p.packetBuffer, binary.BigEndian, p.sequenceNum)
	if err == nil {
		err = binary.Write(p.packetBuffer, binary.BigEndian, timestamp)
	}
	if err == nil {
		err = binary.Write(p.packetBuffer, binary.BigEndian, magnitudeCount)
	}

	// Write payload (Magnitudes) using BigEndian byte order.
	if err == nil {
		err = binary.Write(p.packetBuffer, binary.BigEndian, p.udpF32Buffer)
	}

	if err != nil {
		applog.Errorf("UDPPublisher: Error packing data into binary buffer: %v", err)
		return // Skip sending this packet
	}

	// --- 4. Send Data ---

	// Get the packed bytes from the buffer.
	packetBytes := p.packetBuffer.Bytes()

	// Send the packet using the underlying sender.
	err = p.sender.Send(packetBytes)
	if err != nil {
		// Error logging is handled within sender.Send based on its debug flag.
		// No need to log the same error again here unless more context is needed.
		// applog.Errorf("UDPPublisher: Error sending packet %d: %v", p.sequenceNum, err)
	} else {
		// Log successful sends only at Debug level to avoid flooding logs.
		applog.Debugf("UDPPublisher: Sent packet %d (%d bytes)", p.sequenceNum, len(packetBytes))
	}
}

// Close implements the io.Closer interface. It gracefully stops the publisher goroutine.
func (p *UDPPublisher) Close() error {
	applog.Debugf("UDPPublisher: Close called, stopping publisher...")
	return p.Stop()
}

// Ensure UDPPublisher satisfies the io.Closer interface at compile time.
var _ interface{ Close() error } = (*UDPPublisher)(nil)
