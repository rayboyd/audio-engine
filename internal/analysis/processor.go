// SPDX-License-Identifier: MIT
package analysis

// AudioProcessor defines the standard interface for components that process audio buffers.
// Implementations are expected to analyze or transform the provided audio data.
type AudioProcessor interface {
	// Process analyzes or transforms the given audio input buffer.
	// Implementations should be efficient and avoid blocking operations, as this method is
	// often called from within a real-time audio callback (hot path).
	Process(inputBuffer []int32)
}

// ClosableProcessor extends the AudioProcessor interface with a Close method. This allows
// processors that allocate resources (e.g., buffers, goroutines) to release them gracefully.
type ClosableProcessor interface {
	AudioProcessor
	// Close releases any resources held by the processor (e.g., stops goroutines, frees buffers).
	// It should be safe to call Close multiple times.
	Close() error
}

// FFTResultProvider defines an interface for components that perform Fast Fourier Transform (FFT)
// analysis and make the results available. This decouples consumers (like UDP publishers or other
// analysis stages) from the specific FFT implementation details.
type FFTResultProvider interface {
	// GetMagnitudes returns a thread-safe copy of the latest calculated FFT magnitude spectrum.
	// The length of the returned slice is typically N/2 + 1, where N is the FFT size.
	GetMagnitudes() []float64

	// GetMagnitudesInto copies the latest calculated FFT magnitude spectrum into the provided
	// destination slice. This can help reduce allocations if the caller reuses the slice.
	// The destination slice must be large enough to hold the results (N/2 + 1). Returns an error
	// if the destination slice is nil or too small.
	GetMagnitudesInto(dst []float64) error

	// GetFrequencyForBin returns the center frequency (in Hz) corresponding to a specific FFT bin index.
	GetFrequencyForBin(binIndex int) float64

	// GetFFTSize returns the size (number of points, N) used for the FFT calculation.
	GetFFTSize() int

	// GetSampleRate returns the sample rate (in Hz) of the audio data used for the FFT analysis.
	GetSampleRate() float64
}
