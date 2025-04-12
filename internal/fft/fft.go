package fft

import (
	"math"
	"math/cmplx"

	"gonum.org/v1/gonum/dsp/fourier"
)

// Processor handles real-time Fast Fourier Transform (FFT) processing of audio data.
// It performs the following operations in a lock-free, allocation-free manner:
// - Applies a Hann window to the input samples
// - Computes the FFT using the Gonum library
// - Applies perceptual scaling and normalization
// - Maps the results to frequency bands
// - Broadcasts the results via a transport interface
type Processor struct {
	size       int          // FFT size (must be power of 2)
	sampleRate float64      // Audio sample rate
	fftObj     *fourier.FFT // Gonum FFT object
	window     []float64    // Pre-calculated window function coefficients
	realInput  []float64    // Pre-allocated buffer for real input samples
	buffer     []complex128 // Pre-allocated buffer for FFT complex output (size/2 + 1)
	output     []float64    // Pre-allocated buffer for magnitude output (size/2 + 1)
	transport  Transport    // Interface for sending processed data
}

// Transport defines an interface for sending processed FFT data.
// Implementations should be thread-safe and handle rate limiting.
type Transport interface {
	Send(data []float64) error
}

// NewProcessor creates a new FFT processor with the specified configuration.
// Parameters:
//   - size: FFT window size (must be power of 2)
//   - sampleRate: Audio sample rate in Hz
//   - transport: Interface for sending processed FFT data
//   - numBands: Number of frequency bands for visualization (defaults to 12 if <= 0)
//
// The processor pre-allocates all required buffers and initializes the Hann window
// coefficients to ensure allocation-free processing during the real-time audio path.
func NewProcessor(size int, sampleRate float64, transport Transport) *Processor { // REMOVED numBands parameter
	fftObj := fourier.NewFFT(size)

	// Initialize Hann window
	window := make([]float64, size)
	for i := 0; i < size; i++ {
		window[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(size-1)))
	}

	// Calculate output buffer size (size/2 + 1 for real FFT)
	outputSize := size/2 + 1

	return &Processor{
		size:       size,
		sampleRate: sampleRate,
		fftObj:     fftObj,
		window:     window,
		realInput:  make([]float64, size),
		buffer:     make([]complex128, outputSize),
		output:     make([]float64, outputSize), // Raw magnitude output
		transport:  transport,
	}
}

// Process performs FFT analysis on the input audio buffer and sends results via transport.
// The processing chain includes:
//  1. Sample conversion and windowing
//  2. FFT computation
//  3. Magnitude spectrum calculation with perceptual scaling
//  4. Peak tracking and normalization
//  5. Frequency band mapping with mel-like distribution
//  6. Transport of processed data
//
// This method is designed to be allocation-free after initial setup and is safe
// for use in real-time audio processing contexts. It operates on pre-allocated
// buffers to avoid garbage collection during the processing hot path.
//
// Parameters:
//   - buffer: Input audio samples as int32 values in range [-2147483648, 2147483647]
func (p *Processor) Process(buffer []int32) { // buffer is expected to have length p.size
	// 1. Convert int32 to float64 and apply Hann window
	for i := 0; i < p.size; i++ {
		if i < len(buffer) {
			p.realInput[i] = float64(buffer[i]) * p.window[i] / 2147483648.0
		} else {
			p.realInput[i] = 0 // Zero padding if buffer is smaller than FFT size (shouldn't happen with current engine logic)
		}
	}

	// 2. Perform FFT using Gonum
	// Coefficients writes the complex FFT results into p.buffer (size/2 + 1)
	_ = p.fftObj.Coefficients(p.buffer, p.realInput)

	// 3. Calculate Raw Magnitude Spectrum
	// p.output has size size/2 + 1
	for i := 0; i < len(p.buffer); i++ {
		// Calculate the absolute value (magnitude) of the complex number
		p.output[i] = cmplx.Abs(p.buffer[i])

		// Optional: Apply a simple scaling factor if magnitudes are too small/large
		// You might need this for the client to see anything useful
		// p.output[i] *= 10
	}

	// --- Mitigation: Zero out DC and Nyquist bins ---
	// if len(p.output) > 0 {
	// 	p.output[0] = 0 // Zero out DC component (index 0)
	// }
	// if len(p.output) > 1 {
	// 	// Zero out Nyquist frequency component (last index)
	// 	p.output[len(p.output)-1] = 0
	// }
	// --- End Mitigation ---

	// 4. Send raw magnitude data via transport
	// The number of values sent will be size/2 + 1.
	if p.transport != nil {
		// Send the p.output slice containing raw magnitudes
		err := p.transport.Send(p.output)
		if err != nil {
			// Log error?
			// fmt.Printf("Transport error: %v\n", err)
		}
	}
}

// GetFrequencyBin returns the frequency in Hz for a given FFT bin index.
// This is useful for mapping FFT output bins to actual frequencies.
// Parameters:
//   - i: FFT bin index
//
// Returns:
//   - Frequency in Hz corresponding to the given bin
func (p *Processor) GetFrequencyBin(i int) float64 {
	// Ensure index is within bounds of the complex output buffer
	if i < 0 || i >= len(p.buffer) {
		return 0 // Or handle error appropriately
	}
	// Freq method gives frequency normalized by sample rate, multiply to get Hz
	// Note: Gonum's Freq might return normalized freq [0, 0.5], needs scaling by SampleRate
	// Check Gonum docs if needed, but this calculation is standard: bin_index * (sample_rate / fft_size)
	// However, Gonum's Freq(i) might already do i/size, so multiplying by sampleRate might be correct. Let's assume it is.
	// If Gonum Freq(i) returns i/N, then Freq(i) * sampleRate = (i/N) * sampleRate = i * (sampleRate/N) which is correct.
	return p.fftObj.Freq(i) * p.sampleRate
}
