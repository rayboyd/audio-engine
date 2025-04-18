package udp

import (
	"audio/internal/analysis" // Import analysis package
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"sync"
	"time"
)

// UDPPublisher periodically fetches data from processors, packs it, and sends it via UDPSender.
type UDPPublisher struct {
	sender       *UDPSender
	fftProc      *analysis.FFTProcessor // Concrete processor for GetMagnitudesInto
	interval     time.Duration
	ticker       *time.Ticker   // Ticker managed internally
	doneChan     chan struct{}  // Channel to signal goroutine stop
	stopOnce     sync.Once      // Ensures stop logic runs only once per Start
	wg           sync.WaitGroup // Waits for goroutine to finish
	mu           sync.Mutex     // Protects access to ticker and doneChan during Start/Stop
	sequenceNum  uint32         // Packet sequence number
	udpMagBuffer []float64      // Pre-allocated buffer for magnitudes (float64)
	udpF32Buffer []float32      // Pre-allocated buffer for magnitudes (float32 for packing)
	packetBuffer *bytes.Buffer  // Reusable buffer for packing binary data
}

// NewUDPPublisher creates a new UDPPublisher.
func NewUDPPublisher(interval time.Duration, sender *UDPSender, fftProc *analysis.FFTProcessor) (*UDPPublisher, error) {
	if interval <= 0 {
		interval = 16 * time.Millisecond // Default to ~60Hz if invalid
		log.Printf("UDPPublisher: Invalid interval, defaulting to %s", interval)
	}
	if sender == nil {
		return nil, fmt.Errorf("UDP sender cannot be nil")
	}
	if fftProc == nil {
		return nil, fmt.Errorf("FFT processor cannot be nil")
	}

	requiredLen := fftProc.GetFFTSize()/2 + 1
	log.Printf("UDPPublisher: Initializing (Interval: %s, FFT Bins: %d)", interval, requiredLen)

	return &UDPPublisher{
		sender:   sender,
		fftProc:  fftProc,
		interval: interval,
		// ticker and doneChan initialized in Start
		udpMagBuffer: make([]float64, requiredLen),
		udpF32Buffer: make([]float32, requiredLen),
		packetBuffer: new(bytes.Buffer),
	}, nil
}

// Start begins the periodic publishing process in a separate goroutine.
// It is safe to call multiple times; subsequent calls after the first successful
// start (before Stop) will have no effect.
func (p *UDPPublisher) Start() {
	p.mu.Lock() // Lock to safely check and potentially initialize
	if p.ticker != nil {
		p.mu.Unlock()
		log.Println("UDPPublisher: Already started")
		return // Already running
	}

	// Initialize resources for this run
	p.ticker = time.NewTicker(p.interval)
	p.doneChan = make(chan struct{})
	p.stopOnce = sync.Once{} // Reset stopOnce for this run

	// Capture local variables for the goroutine
	ticker := p.ticker
	doneChan := p.doneChan

	p.mu.Unlock() // Unlock before starting goroutine

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		log.Printf("UDPPublisher: Starting publisher goroutine (Interval: %s)", p.interval)
		for {
			select {
			// Use the captured local ticker variable
			case <-ticker.C:
				p.buildAndSendPacket()
			// Use the captured local doneChan variable
			case <-doneChan:
				log.Println("UDPPublisher: Received stop signal.")
				return // Exit goroutine
			}
		}
	}()
}

// Stop signals the publisher goroutine to stop and waits for it to finish.
// It is safe to call multiple times; subsequent calls after the first successful
// stop (before Start is called again) will have no effect.
func (p *UDPPublisher) Stop() error {
	p.mu.Lock() // Lock to safely access ticker and doneChan
	// Check if already stopped or not started
	if p.ticker == nil {
		p.mu.Unlock()
		return nil
	}

	// Ensure stop logic (closing channel, stopping ticker) runs only once
	p.stopOnce.Do(func() {
		log.Println("UDPPublisher: Initiating stop sequence...")
		close(p.doneChan) // Signal goroutine to stop
		p.ticker.Stop()   // Stop the ticker
		p.ticker = nil    // Mark as stopped
	})

	p.mu.Unlock() // Unlock before waiting

	// Wait outside the lock for the goroutine to finish processing the signal
	log.Println("UDPPublisher: Waiting for publisher goroutine to finish...")
	p.wg.Wait()
	log.Println("UDPPublisher: Publisher goroutine finished.")
	return nil
}

// buildAndSendPacket fetches data, packs it according to the defined structure, and sends it.
func (p *UDPPublisher) buildAndSendPacket() {
	// 1. Get FFT Magnitudes
	err := p.fftProc.GetMagnitudesInto(p.udpMagBuffer)
	if err != nil {
		log.Printf("UDPPublisher: Error getting magnitudes: %v", err)
		return // Skip sending if data retrieval fails
	}

	// 2. Prepare Packet Data
	p.sequenceNum++
	timestamp := time.Now().UnixNano()
	magnitudeCount := uint16(len(p.udpMagBuffer))

	// Convert float64 magnitudes to float32 for packing
	// Ensure udpF32Buffer has the correct length (should match udpMagBuffer)
	if len(p.udpF32Buffer) != len(p.udpMagBuffer) {
		log.Printf("UDPPublisher: ERROR - Mismatched float32 buffer length!")
		// This indicates an initialization error, but we might try to recover
		// or simply return. For now, log and return.
		return
	}
	for i, v := range p.udpMagBuffer {
		p.udpF32Buffer[i] = float32(v)
	}

	// 3. Pack Data into Buffer
	p.packetBuffer.Reset() // Reuse the buffer

	// Write Header (using BigEndian)
	err = binary.Write(p.packetBuffer, binary.BigEndian, p.sequenceNum)
	if err == nil {
		err = binary.Write(p.packetBuffer, binary.BigEndian, timestamp)
	}
	if err == nil {
		err = binary.Write(p.packetBuffer, binary.BigEndian, magnitudeCount)
	}
	// Write Magnitudes Payload
	if err == nil {
		// binary.Write handles writing slices of numeric types directly
		err = binary.Write(p.packetBuffer, binary.BigEndian, p.udpF32Buffer)
	}

	if err != nil {
		log.Printf("UDPPublisher: Error packing data: %v", err)
		return // Skip sending if packing fails
	}

	// 4. Send Packet
	packetBytes := p.packetBuffer.Bytes()
	err = p.sender.Send(packetBytes)
	if err != nil {
		// Sender already logs errors, no need to log again here
		// log.Printf("UDPPublisher: Error sending packet: %v", err)
	} else {
		// log.Printf("UDPPublisher: Sent packet %d (%d bytes)", p.sequenceNum, len(packetBytes)) // Optional: Log success
	}
}

// Close implements io.Closer by calling Stop.
func (p *UDPPublisher) Close() error {
	return p.Stop()
}

// Ensure UDPPublisher satisfies the io.Closer interface
var _ interface{ Close() error } = (*UDPPublisher)(nil)
