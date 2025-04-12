package fft

import (
	"math"
	"testing"
)

// TestFFTHotPath tests the core FFT processing for zero allocations
func TestFFTHotPath(t *testing.T) {
	// Create a simple processor with nil transport to avoid any network code
	processor := NewProcessor(1024, 44100, nil, 12)

	// Create test buffer
	buffer := make([]int32, 1024)
	for i := range buffer {
		buffer[i] = int32(i * 1000000)
	}

	// Warm up - first call will allocate
	processor.Process(buffer)

	// Test core processing only
	allocs := testing.AllocsPerRun(100, func() {
		// Create a no-op version of Send to avoid transport allocations
		oldTransport := processor.transport
		processor.transport = nil

		// Run the processing code
		processor.Process(buffer)

		// Restore transport (though it's nil anyway)
		processor.transport = oldTransport
	})

	if allocs > 0 {
		t.Errorf("Expected zero allocations in FFT hot path, got %.1f", allocs)
	}
}

// TestGetFrequencyBinZeroAllocs verifies that frequency bin mapping has no allocations
func TestGetFrequencyBinZeroAllocs(t *testing.T) {
	processor := NewProcessor(1024, 44100, nil, 12)

	// Test multiple bin calculations for zero allocations
	allocs := testing.AllocsPerRun(100, func() {
		// Get frequency for various bins
		_ = processor.GetFrequencyBin(0)   // DC component
		_ = processor.GetFrequencyBin(10)  // Low frequency
		_ = processor.GetFrequencyBin(100) // Mid frequency
		_ = processor.GetFrequencyBin(500) // High frequency
	})

	if allocs > 0 {
		t.Errorf("Expected zero allocations in frequency bin mapping, got %.1f", allocs)
	}
}

// BenchmarkFastFourierTransform measures performance with allocation reporting
func BenchmarkFastFourierTransform(b *testing.B) {
	processor := NewProcessor(1024, 44100, nil, 12)
	buffer := make([]int32, 1024)

	// Generate test signal (sine wave with harmonics)
	for i := range buffer {
		t := float64(i) / 44100.0
		// Fundamental at 440Hz plus harmonics
		buffer[i] = int32(
			(math.Sin(2*math.Pi*440*t)*0.5 +
				math.Sin(2*math.Pi*880*t)*0.3 +
				math.Sin(2*math.Pi*1320*t)*0.2) *
				math.MaxInt32 * 0.9)
	}

	// Disable transport to isolate FFT performance
	processor.transport = nil

	// Reset and enable allocation reporting
	b.ResetTimer()
	b.ReportAllocs()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		processor.Process(buffer)
	}
}

// BenchmarkFFTProcessing benchmarks just the core FFT computation
func BenchmarkFFTProcessing(b *testing.B) {
	processor := NewProcessor(1024, 44100, nil, 12)
	buffer := make([]int32, 1024)

	// Generate a realistic audio signal (sine wave)
	for i := range buffer {
		t := float64(i) / 44100.0
		buffer[i] = int32(math.Sin(2*math.Pi*440*t) * 2147483647 * 0.5)
	}

	// Reset timer and remove transport to avoid network code
	processor.transport = nil
	b.ResetTimer()

	// Run benchmark
	for i := 0; i < b.N; i++ {
		processor.Process(buffer)
	}
}
