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
	// Configuration
	size       int       // FFT window size (must be power of 2)
	sampleRate float64   // Audio sample rate in Hz
	transport  Transport // Interface for sending processed data

	// Working data - pre-allocated buffers for the hot path
	window    []float64    // Pre-computed Hann window coefficients
	realInput []float64    // Real-valued input buffer for FFT
	fftObj    *fourier.FFT // Gonum FFT processor instance
	buffer    []complex128 // Complex FFT output buffer (size/2 + 1)
	output    []float64    // Magnitude spectrum buffer

	// Band processing for visualization
	numBands   int       // Number of frequency bands (e.g., 6, 12, or 24)
	bandOutput []float64 // Pre-allocated band output buffer

	// Peak normalization
	peakValue float64 // Current peak value for normalization
	decayRate float64 // Peak decay rate (0.95-0.99)
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
func NewProcessor(size int, sampleRate float64, transport Transport, numBands int) *Processor {
	// Create and initialize the Gonum FFT processor
	fftObj := fourier.NewFFT(size)
	if numBands <= 0 {
		numBands = 12
	}

	p := &Processor{
		size:       size,
		sampleRate: sampleRate,
		transport:  transport,
		fftObj:     fftObj,
		realInput:  make([]float64, size),
		// Fix buffer size to match what Coefficients expects
		buffer: make([]complex128, size/2+1),
		// Update output size to match
		output: make([]float64, size/2+1),

		numBands:   numBands,
		bandOutput: make([]float64, numBands),
		peakValue:  0.01, // Initial non-zero value to avoid division by zero
		decayRate:  0.95, // Decay rate for peak normalization
	}

	// Initialize Hann window coefficients
	p.window = make([]float64, size)
	for i := 0; i < size; i++ {
		p.window[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(size-1)))
	}

	return p
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
func (p *Processor) Process(buffer []int32) {
	// Convert int32 to float64 with windowing
	for i := 0; i < p.size; i++ {
		if i < len(buffer) {
			// Apply window function and normalize to [-1.0, 1.0]
			p.realInput[i] = float64(buffer[i]) * p.window[i] / 2147483648.0
		} else {
			p.realInput[i] = 0
		}
	}

	// Perform FFT using Gonum
	p.buffer = p.fftObj.Coefficients(p.buffer, p.realInput)

	// Calculate magnitude spectrum with logarithmic scaling and conversion to dB
	for i := 0; i < len(p.buffer); i++ {
		magnitude := cmplx.Abs(p.buffer[i])

		// Apply frequency-dependent scaling (boost higher frequencies)
		freqScaling := 1.0
		if i > 0 {
			// Logarithmic boost for higher frequencies
			// This helps compensate for the natural roll-off in spectral energy
			freqScaling = 1.0 + math.Log10(float64(i+1))/2.0
		}

		// Apply scaling and convert to dB-like scale (log scale)
		scaledMag := magnitude * freqScaling
		if scaledMag > 0.0001 { // Avoid very small values
			// Log scale transformation (similar to dB but normalized)
			p.output[i] = math.Log10(1.0+scaledMag*100) / 3.0
		} else {
			p.output[i] = 0
		}
	}

	// Find current maximum value for normalization
	var currentMax float64
	for _, v := range p.output {
		if v > currentMax {
			currentMax = v
		}
	}

	// Update peak with decay for stable normalization
	if currentMax > p.peakValue {
		p.peakValue = currentMax
	} else {
		p.peakValue = p.peakValue*p.decayRate + currentMax*(1-p.decayRate)
	}

	// Group into frequency bands with improved distribution and scaling
	for i := 0; i < p.numBands; i++ {
		// Reset band value
		p.bandOutput[i] = 0

		// Mel-like frequency distribution (better perceptual mapping)
		// This gives more resolution to lower frequencies but is less extreme
		// than pure logarithmic scaling
		minFreq := 20.0               // Hz
		maxFreq := p.sampleRate / 2.0 // Nyquist frequency

		// Convert band index to frequency using mel-like scale
		bandStartFreq := minFreq * math.Pow(maxFreq/minFreq, float64(i)/float64(p.numBands))
		bandEndFreq := minFreq * math.Pow(maxFreq/minFreq, float64(i+1)/float64(p.numBands))

		// Convert frequencies to FFT bin indices
		startIdx := int((bandStartFreq * float64(len(p.output))) / (p.sampleRate / 2.0))
		endIdx := int((bandEndFreq * float64(len(p.output))) / (p.sampleRate / 2.0))

		// Clamp indices to valid range
		if startIdx < 0 {
			startIdx = 0
		}
		if endIdx >= len(p.output) {
			endIdx = len(p.output) - 1
		}
		if startIdx > endIdx {
			startIdx = endIdx
		}

		// Calculate perceptually-weighted average for this band
		var sum float64
		var weightSum float64

		for j := startIdx; j <= endIdx; j++ {
			// Apply band-dependent weights
			// Higher bands get more weight to compensate for lower energy
			weight := 1.0 + float64(i)/float64(p.numBands)*2.0
			sum += p.output[j] * weight
			weightSum += weight
		}

		// Compute weighted average and normalize
		if weightSum > 0 {
			p.bandOutput[i] = (sum / weightSum) / p.peakValue

			// Apply final scaling - boost higher bands slightly more
			bandBoost := 1.0 + float64(i)/float64(p.numBands)*0.5
			p.bandOutput[i] *= bandBoost

			// Clamp to valid range
			if p.bandOutput[i] > 1.0 {
				p.bandOutput[i] = 1.0
			}
		}
	}

	// Send normalized band data via transport
	if p.transport != nil {
		p.transport.Send(p.bandOutput)
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
	return p.fftObj.Freq(i) * p.sampleRate
}
