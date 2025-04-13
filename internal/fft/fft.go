// SPDX-License-Identifier: MIT
package fft

import (
	"audio/pkg/bitint"
	"math"
	"math/cmplx"

	"gonum.org/v1/gonum/dsp/fourier"
)

// Transport defines an interface for sending processed FFT data.
// Implementations should be thread-safe and handle rate limiting.
type Transport interface {
	Send(data []float64) error
}

// FFTWorkspace holds pre-allocated buffers for FFT calculations.
type FFTWorkspace struct {
	input     []float64    // ...for real input samples (windowed, scaled)
	fftOutput []complex128 // ...for FFT complex output
	magnitude []float64    // ...for raw magnitude output
	window    []float64    // ...for window function coefficients
}

// Processor holds the FFT processor state and configuration.
type Processor struct {
	fftSize    int
	sampleRate float64
	workspace  FFTWorkspace
	fftObj     *fourier.FFT
	transport  Transport
}

// NewProcessor creates a new FFT processor it will pre-allocate all required
// buffers and initialize the Hann window coefficients.
func NewProcessor(fftSize int, sampleRate float64, transport Transport) *Processor {
	if !bitint.IsPowerOfTwo(fftSize) {
		panic("FFT size must be a power of 2") // panic for now
	}
	fftObj := fourier.NewFFT(fftSize)

	window := make([]float64, fftSize)
	for i := range fftSize {
		window[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(fftSize-1)))
	}

	// Pre-compute the size of the output buffer
	outputSize := fftSize/2 + 1

	return &Processor{
		fftSize:    fftSize,
		sampleRate: sampleRate,
		fftObj:     fftObj,
		transport:  transport,

		// Pre-allocate buffers for FFT processing
		workspace: FFTWorkspace{
			input:     make([]float64, fftSize),
			fftOutput: make([]complex128, outputSize),
			magnitude: make([]float64, outputSize),
			window:    window,
		},
	}
}

// Process performs FFT analysis on the input audio buffer and sends results via
// transport to the next stage after applying a Hann window. The input buffer is
// expected to be of size p.size.
func (p *Processor) Process(inputBuffer []int32) {
	for i := range p.fftSize {
		if i < len(inputBuffer) {
			p.workspace.input[i] = float64(inputBuffer[i]) * p.workspace.window[i] / float64(math.MaxInt32)
		} else {
			p.workspace.input[i] = 0 // 0pad if fft is smaller than size, should not happen
		}
	}

	// Perform FFT on the input buffer, and calculate the magnitude
	_ = p.fftObj.Coefficients(p.workspace.fftOutput, p.workspace.input)
	for i := range p.workspace.fftOutput {
		p.workspace.magnitude[i] = cmplx.Abs(p.workspace.fftOutput[i])
	}

	if p.transport != nil {
		_ = p.transport.Send(p.workspace.magnitude)
	}
}

// GetFrequencyBin returns the frequency in Hz for a given FFT bin index.
func (p *Processor) GetFrequencyBin(i int) float64 {
	if i < 0 || i >= len(p.workspace.fftOutput) {
		return 0
	}
	return p.fftObj.Freq(i) * p.sampleRate
}
