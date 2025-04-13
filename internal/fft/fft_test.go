// SPDX-License-Identifier: MIT
package fft

import (
	"math"
	"testing"
)

const (
	testFFTSize    = 1024
	testSampleRate = 44100
)

func TestFFTHotPath(t *testing.T) {
	// Create a processor with nil transport to isolate Process method allocations.
	processor := NewProcessor(testFFTSize, testSampleRate, nil)

	inputBuffer := make([]int32, testFFTSize)
	for i := range inputBuffer {
		inputBuffer[i] = int32((i%256 - 128) * 1000000) // Arbitrary non-zero data
	}

	// Warm-up call (potential initial allocations). Ensure that the first call to
	// Process does not count towards the allocation count in the benchmark.
	processor.Process(inputBuffer)
	allocs := testing.AllocsPerRun(100, func() {
		processor.Process(inputBuffer)
	})

	if allocs > 0 {
		t.Errorf("Expected zero allocations in FFT Process hot path, got %.1f", allocs)
	}
}

func TestGetFrequencyBinZeroAllocs(t *testing.T) {
	processor := NewProcessor(testFFTSize, testSampleRate, nil)

	// Warm-up call (potential initial allocations).
	allocs := testing.AllocsPerRun(100, func() {
		_ = processor.GetFrequencyBin(0)               // DC component
		_ = processor.GetFrequencyBin(10)              // Low frequency
		_ = processor.GetFrequencyBin(testFFTSize / 4) // Mid frequency
		_ = processor.GetFrequencyBin(testFFTSize / 2) // Nyquist frequency
	})

	if allocs > 0 {
		t.Errorf("Expected zero allocations in GetFrequencyBin, got %.1f", allocs)
	}
}

func BenchmarkProcess(b *testing.B) {
	processor := NewProcessor(testFFTSize, testSampleRate, nil)
	inputBuffer := make([]int32, testFFTSize)

	// Generate a test signal (sine wave with harmonics).
	for i := range inputBuffer {
		tm := float64(i) / testSampleRate

		// Fundamental at 440Hz plus harmonics.
		signal := math.Sin(2*math.Pi*440*tm)*0.5 +
			math.Sin(2*math.Pi*880*tm)*0.3 +
			math.Sin(2*math.Pi*1320*tm)*0.2

		// Scale to int32 range.
		inputBuffer[i] = int32(signal * math.MaxInt32 * 0.9)
	}

	b.ReportAllocs()

	for b.Loop() {
		processor.Process(inputBuffer)
	}
}
