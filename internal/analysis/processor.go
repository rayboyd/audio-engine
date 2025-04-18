// SPDX-License-Identifier: MIT
package analysis

// Defines the standard interface for components that process audio buffers.
type AudioProcessor interface {
	// Process analyzes the given audio input buffer. Implementations should be efficient as
	// this is often called from within a hotpath such as the real-time audio callback.
	Process(inputBuffer []int32)
}

// ClosableProcessor combines AudioProcessor with a Close method for resource cleanup.
type ClosableProcessor interface {
	AudioProcessor
	Close() error // Close releases any resources held by the processor.
}

// FFTResultProvider defines an interface for components that can provide FFT magnitude results.
// This decouples consumers (like BandEnergyProcessor) from the specific FFT implementation, and
// was designed with future analysis processors in mind (Band Energy, etc.). These processors will
// use the FFTResultProvider interface to access FFT data and components things decoupled.
type FFTResultProvider interface {
	GetMagnitudes() []float64                // GetMagnitudes returns a thread-safe copy of the latest FFT magnitude spectrum.
	GetFrequencyForBin(binIndex int) float64 // GetFrequencyForBin returns the center frequency (Hz) for a given FFT bin index.
	GetFFTSize() int                         // GetFFTSize returns the size (number of points) of the FFT.
	GetSampleRate() float64                  // GetSampleRate returns the sample rate used for the FFT analysis.
}
