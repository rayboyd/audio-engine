// SPDX-License-Identifier: MIT
package analysis

import (
	"audio/pkg/bitint"
	"fmt"
	"log"
	"math/cmplx"
	"strings"
	"sync"

	"gonum.org/v1/gonum/dsp/fourier"
	"gonum.org/v1/gonum/dsp/window"
)

// WindowFunc defines the type for selecting an FFT window function.
type WindowFunc int

// Enum for available window functions.
const (
	BartlettHann WindowFunc = iota
	Blackman
	BlackmanNuttall
	Hann
	Hamming
	Lanczos
	Nuttall
)

// Pre-allocated buffers for FFT calculations.
type fftWorkspace struct {
	input     []float64    // Buffer for windowed input signal (float64).
	fftOutput []complex128 // Buffer for FFT complex results.
	magnitude []float64    // Buffer for calculated magnitudes.
	window    []float64    // Pre-calculated window coefficients.
	mu        sync.RWMutex // Protects concurrent access to magnitude buffer.
}

// FFTProcessor is a real-time audio processor that performs FFT analysis on input audio data.
// It implements the AudioProcessor interface and provides FFT results via the FFTResultProvider
// interface. The processor can be closed using the ClosableProcessor interface. It uses a
// transport mechanism to send results asynchronously. The processor is designed to be thread-safe
// and efficient for real-time audio processing.
type FFTProcessor struct {
	fftCalculator *fourier.FFT // Reusable FFT calculator instance.
	fftSize       int          // Number of points for the FFT (power of 2).
	sampleRate    float64      // Sample rate of the input audio (Hz).
	workspace     fftWorkspace // Pre-allocated buffers.
}

// Compile-time checks for interface implementations.
var _ AudioProcessor = (*FFTProcessor)(nil)
var _ FFTResultProvider = (*FFTProcessor)(nil)
var _ ClosableProcessor = (*FFTProcessor)(nil)

// TODO:
// Document this function.
func NewFFTProcessor(fftSize int, sampleRate float64, windowType WindowFunc) (*FFTProcessor, error) {
	if !bitint.IsPowerOfTwo(fftSize) {
		// TODO:
		// Preallocate this error message.
		return nil, fmt.Errorf("fft size must be a power of 2, got %d", fftSize)
	}
	if sampleRate <= 0 {
		// TODO:
		// Preallocate this error message.
		return nil, fmt.Errorf("sample rate must be positive, got %f", sampleRate)
	}

	fftCalculator := fourier.NewFFT(fftSize)
	windowCoeffs := make([]float64, fftSize)
	applyWindow(windowCoeffs, windowType)

	// FFT output size for real input is N/2 + 1 complex values.
	magnitudeSize := fftSize/2 + 1

	log.Printf("Analysis: Initializing FFTProcessor (Size: %d, SampleRate: %.1f Hz, Window: %v)", fftSize, sampleRate, windowType)

	return &FFTProcessor{
		fftCalculator: fftCalculator,
		fftSize:       fftSize,
		sampleRate:    sampleRate,
		workspace: fftWorkspace{
			input:     make([]float64, fftSize),
			fftOutput: make([]complex128, magnitudeSize),
			magnitude: make([]float64, magnitudeSize),
			window:    windowCoeffs,
			// mu is zero-value ready.
		},
	}, nil
}

// Process applies windowing, performs FFT, calculates magnitudes, and sends results via transport.
// This is the core real-time processing method implementing analysis.AudioProcessor.
func (p *FFTProcessor) Process(inputBuffer []int32) {
	// --- 1. Prepare Input & Windowing ---
	p.workspace.mu.Lock() // Lock for writing to workspace buffers.

	// Apply window and scale input. Zero-pad if input is shorter than fftSize.
	const normFactor = 1.0 / float64(0x80000000) // Normalization factor for int32 to float64 range [-1.0, 1.0).
	inputLen := len(inputBuffer)
	for i := range p.fftSize {
		if i < inputLen {
			p.workspace.input[i] = float64(inputBuffer[i]) * normFactor * p.workspace.window[i]
		} else {
			p.workspace.input[i] = 0 // Zero-padding.
		}
	}

	// --- 2. Perform FFT --
	p.fftCalculator.Coefficients(p.workspace.fftOutput, p.workspace.input)

	// --- 3. Calculate Magnitudes ---
	for i, c := range p.workspace.fftOutput {
		p.workspace.magnitude[i] = cmplx.Abs(c)
	}

	// --- 4. Unlock Workspace ---
	// Release the lock now that calculations involving shared buffers are done.
	p.workspace.mu.Unlock()
}

// GetMagnitudes returns a thread-safe copy of the latest calculated FFT magnitudes.
// Implements the analysis.FFTResultProvider interface.
// NOTE: This method allocates a new slice for the copy on each call.
// For performance-critical readers wanting to avoid allocation, use GetMagnitudesInto.
func (p *FFTProcessor) GetMagnitudes() []float64 {
	p.workspace.mu.RLock() // Acquire read lock - multiple readers OK.
	defer p.workspace.mu.RUnlock()

	// Return a *copy* to prevent race conditions if the caller modifies the slice
	// or if Process runs concurrently.
	magCopy := make([]float64, len(p.workspace.magnitude))
	copy(magCopy, p.workspace.magnitude)
	return magCopy
}

// GetMagnitudesInto copies the latest calculated FFT magnitudes into the provided destination slice.
// This method avoids allocation within the function itself, assuming the caller provides
// a destination slice of the correct size. It is intended for performance-critical readers.
// The destination slice must have the same length as the internal magnitude buffer (fftSize/2 + 1).
func (p *FFTProcessor) GetMagnitudesInto(dest []float64) error {
	p.workspace.mu.RLock() // Acquire read lock.
	defer p.workspace.mu.RUnlock()

	if len(dest) != len(p.workspace.magnitude) {
		// Consider returning the required size?
		return fmt.Errorf("destination slice length %d does not match required length %d", len(dest), len(p.workspace.magnitude))
	}

	copy(dest, p.workspace.magnitude)
	return nil
}

// GetFrequencyForBin returns the center frequency (Hz) for a given FFT bin index.
// Implements the analysis.FFTResultProvider interface.
func (p *FFTProcessor) GetFrequencyForBin(binIndex int) float64 {
	// Configuration (fftSize, sampleRate) is immutable after creation. Reading len(p.workspace.fftOutput)
	// is safe as length is fixed after creation. A read lock here is technically correct but is this some
	// unnecessary overhead? Should think of usecases where this might need to be mutated.
	// p.workspace.mu.RLock()
	outputLen := len(p.workspace.fftOutput)
	// p.workspace.mu.RUnlock()

	if binIndex < 0 || binIndex >= outputLen {
		return 0.0
	}

	// Frequency resolution = sampleRate / fftSize
	// Bin frequency = binIndex * frequencyResolution
	return float64(binIndex) * (p.sampleRate / float64(p.fftSize))
}

// GetFFTSize returns the configured FFT size (number of points).
// Implements the analysis.FFTResultProvider interface.
func (p *FFTProcessor) GetFFTSize() int {
	return p.fftSize // Immutable after creation, no lock needed.
}

// GetSampleRate returns the configured sample rate (Hz).
// Implements the analysis.FFTResultProvider interface.
func (p *FFTProcessor) GetSampleRate() float64 {
	return p.sampleRate // Immutable after creation, no lock needed.
}

// Close handles any necessary cleanup for the FFTProcessor.
// Currently, this processor doesn't hold resources requiring explicit closing.
// Implements the analysis.ClosableProcessor interface.
func (p *FFTProcessor) Close() error {
	// TODO:
	// Preallocate this error message.
	log.Printf("Analysis: Closing FFTProcessor (no specific resources to release)")
	// If this processor owned exclusive resources, they would be closed here.
	return nil
}

// ParseWindowFunc converts a string name (case-insensitive) to a WindowFunc
// enum, returns a known default (Hann) and an error if the name is unknown.
func ParseWindowFunc(name string) (WindowFunc, error) {
	switch strings.ToLower(name) {
	case "bartletthann":
		return BartlettHann, nil
	case "blackman":
		return Blackman, nil
	case "blackmannuttall":
		return BlackmanNuttall, nil
	case "hann", "hanning":
		return Hann, nil
	case "hamming":
		return Hamming, nil
	case "lanczos":
		return Lanczos, nil
	case "nuttall":
		return Nuttall, nil
	default:
		// TODO:
		// Preallocate this error message.
		return Hann, fmt.Errorf("unknown FFT window function name: '%s'", name)
	}
}

// applyWindow applies the selected window function to the coefficient slice,
// returns the modified slice. Returns the Hann window by default if the type is unknown.
func applyWindow(coeffs []float64, windowType WindowFunc) {
	// Initialize coeffs with 1.0 before applying window,  otherwise window funcs might
	// multiply by zero if the slice wasn't initialized.
	for i := range coeffs {
		coeffs[i] = 1.0
	}
	switch windowType {
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
		// TODO:
		// Preallocate this error message.
		log.Printf("Analysis: Unknown window function type %d, defaulting to Hann", windowType)
		window.Hann(coeffs)
	}
}
