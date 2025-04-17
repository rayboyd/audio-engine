package transport

// Transport defines a generic interface for sending processed data or events.
// Implementations should be thread-safe.
type Transport interface {
	Send(data any) error
	Close() error
}

// Processor defines the interface for components that process raw audio buffers.
// Implementations must be real-time safe (non-blocking, minimal/no allocations).
type Processor interface {
	Process(buffer []int32)
}

// FFTResultProvider is an interface for processors that provide FFT results.
// This allows decoupling other analysis processors from the concrete FFTProcessor.
type FFTResultProvider interface {
	GetMagnitudes() []float64
	GetFrequencyForBin(binIndex int) float64
	// Consider adding:
	// GetSampleRate() float64
	// GetFFTSize() int
}

// DataProcessor defines an interface for processors that operate on data
// derived from other processors (like BandEnergy from FFT).
// Their Process method might not take direct audio buffers.
// We might not need this if the engine uses type assertions.
// type DataProcessor interface {
//  ProcessData() // Or specific method name
// }
