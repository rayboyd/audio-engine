package analysis

import (
	"audio/internal/transport"
	"audio/pkg/bitint"
	"log"
	"math/cmplx"
	"sync"

	"gonum.org/v1/gonum/dsp/fourier"
	"gonum.org/v1/gonum/dsp/window"
)

type WindowFunc int

const (
	BartlettHann WindowFunc = iota
	Blackman
	BlackmanNuttall
	Hann
	Hamming
	Lanczos
	Nuttall
)

type FFTMetadata struct {
	FFTSize    int     `json:"fftSize"`
	WindowType int     `json:"windowType"`
	Energy     float64 `json:"energy"`
}

type FFTWorkspace struct {
	input     []float64
	fftOutput []complex128
	magnitude []float64
	window    []float64
	// Add a mutex to protect access to magnitude if needed by GetMagnitudes
	mu sync.RWMutex
}

// Processor now resides in the analysis package.
type FFTProcessor struct {
	fftObj     *fourier.FFT
	fftSize    int
	sampleRate float64
	transport  transport.Transport
	workspace  FFTWorkspace
}

// NewFFTProcessor creates a new FFT processor. Renamed for clarity.
func NewFFTProcessor(fftSize int, sampleRate float64, transport transport.Transport, fn WindowFunc) *FFTProcessor {
	if !bitint.IsPowerOfTwo(fftSize) {
		log.Panicf("FFT size must be a power of 2, got %d", fftSize)
	}
	fftObj := fourier.NewFFT(fftSize)
	coeffs := make([]float64, fftSize)
	for i := range coeffs {
		coeffs[i] = 1.0
	}
	switch fn {
	case BartlettHann:
		window.BartlettHann(coeffs)
	case Blackman:
		window.Blackman(coeffs)
	case BlackmanNuttall:
		window.BlackmanNuttall(coeffs)
	case Hann:
		window.Hann(coeffs)
	case Hamming:
		window.Hamming(coeffs)
	case Lanczos:
		window.Lanczos(coeffs)
	case Nuttall:
		window.Nuttall(coeffs)
	default:
		window.BartlettHann(coeffs) // Default to BartlettHann
	}
	bufferSize := fftSize/2 + 1

	log.Printf("Analysis: Initializing FFTProcessor (Size: %d, Window: %T)", fftSize, fn) // Added log

	return &FFTProcessor{
		fftObj:     fftObj,
		fftSize:    fftSize,
		sampleRate: sampleRate,
		transport:  transport,
		workspace: FFTWorkspace{
			input:     make([]float64, fftSize),
			fftOutput: make([]complex128, bufferSize),
			magnitude: make([]float64, bufferSize),
			window:    coeffs,
		},
	}
}

// Process performs FFT analysis.
func (p *FFTProcessor) Process(inputBuffer []int32) {
	p.workspace.mu.Lock() // Lock for writing to workspace

	// --- Windowing and Scaling ---
	for i := 0; i < p.fftSize; i++ {
		if i < len(inputBuffer) {
			p.workspace.input[i] = float64(inputBuffer[i]) * p.workspace.window[i] / float64(0x7FFFFFFF)
		} else {
			p.workspace.input[i] = 0
		}
	}

	// --- Perform FFT ---
	p.fftObj.Coefficients(p.workspace.fftOutput, p.workspace.input)

	// --- Calculate Magnitudes ---
	for i := range p.workspace.fftOutput {
		p.workspace.magnitude[i] = cmplx.Abs(p.workspace.fftOutput[i])
	}

	// --- Prepare data for sending WHILE lock is held ---
	var magCopy []float64 // Declare slice outside the if block
	if p.transport != nil {
		// Copy the data needed for sending while the write lock is held
		magCopy = make([]float64, len(p.workspace.magnitude))
		copy(magCopy, p.workspace.magnitude)
		// --- REMOVED the p.workspace.mu.RLock() and RUnlock() here ---
	}

	// --- NOW Unlock ---
	p.workspace.mu.Unlock() // Unlock BEFORE potentially blocking on Send

	// --- Send Magnitudes via Transport AFTER unlocking ---
	if p.transport != nil && magCopy != nil { // Check magCopy is not nil
		if err := p.transport.Send(magCopy); err != nil {
			log.Printf("ERROR: FFTProcessor failed to send magnitude data: %v", err)
		}
	} else if p.transport == nil {
		log.Println("WARN: FFTProcessor transport is nil, cannot send data.")
	}
} // End of Process method

// GetMagnitudes returns a copy of the latest calculated magnitudes.
// This is needed by other processors like BandEnergyProcessor.
func (p *FFTProcessor) GetMagnitudes() []float64 {
	p.workspace.mu.RLock() // Read lock
	defer p.workspace.mu.RUnlock()

	// Return a copy to prevent race conditions if the caller modifies it
	// or if the Process method runs concurrently.
	magCopy := make([]float64, len(p.workspace.magnitude))
	copy(magCopy, p.workspace.magnitude)
	return magCopy
}

// GetFrequencyForBin returns the frequency in Hz for a given FFT bin index.
func (p *FFTProcessor) GetFrequencyForBin(binIndex int) float64 {
	p.workspace.mu.RLock() // Ensure thread safety if called concurrently
	defer p.workspace.mu.RUnlock()

	if binIndex < 0 || binIndex >= len(p.workspace.fftOutput) {
		return 0 // Invalid bin index
	}
	// Calculate frequency: binIndex * (sampleRate / fftSize)
	// Note: gonum's Freq method might return normalized frequency (0 to 0.5)
	// return p.fftObj.Freq(binIndex) * p.sampleRate // If Freq returns normalized
	return float64(binIndex) * (p.sampleRate / float64(p.fftSize)) // Direct calculation
}
