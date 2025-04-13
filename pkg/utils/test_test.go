// SPDX-License-Identifier: MIT
package utils

import (
	"math"
	"os"
	"testing"
)

const (
	testSize       = 1024
	testSampleRate = 44100
	testFrequency  = 440.0 // A4 note
)

var (
	testMagnitudes  []float64
	testComplexWave []int32
	testSineWave    []int32
)

func TestMain(m *testing.M) {
	testMagnitudes = make([]float64, testSize)

	// Create a peaked distribution with a known peak.
	for i := range testMagnitudes {
		// Creates a "hill" with peak at position testSize/4.
		testMagnitudes[i] = math.Exp(-0.01 * math.Pow(float64(i-testSize/4), 2))
	}

	testComplexWave = GenerateComplexWave(testSize, testSampleRate)
	testSineWave = GenerateSineWave(testSize, testSampleRate, testFrequency)

	// Run all tests.
	exitCode := m.Run()

	// Clean up if needed.
	// (No specific cleanup needed in this case)

	os.Exit(exitCode)
}

func TestMockTransport(t *testing.T) {
	tests := []struct {
		name      string
		inputData []float64
	}{
		{"Empty Data", []float64{}},
		{"Single Value", []float64{0.5}},
		{"Multiple Values", []float64{0.1, 0.2, 0.3, 0.4, 0.5}},
		{"Large Dataset", make([]float64, 1024)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt := &MockTransport{}

			err := mt.Send(tt.inputData)
			if err != nil {
				t.Errorf("MockTransport.Send() error = %v", err)
			}

			if len(mt.LastData) != len(tt.inputData) {
				t.Errorf("MockTransport.Send() stored length = %d, want %d",
					len(mt.LastData), len(tt.inputData))
			}

			if len(tt.inputData) > 0 {
				originalValue := tt.inputData[0]
				tt.inputData[0] = 999.999 // Modify original.

				if len(mt.LastData) > 0 && mt.LastData[0] == 999.999 {
					t.Errorf("MockTransport.Send() stored reference instead of copy")
				}

				tt.inputData[0] = originalValue
			}
		})
	}

	// Test allocation behavior.
	mt := &MockTransport{}
	testData := make([]float64, 1024)
	mt.Send(testData)

	allocs := testing.AllocsPerRun(100, func() {
		mt.Send(testData)
	})

	// We expect exactly one allocation (for the copy of the slice).
	if allocs != 1 {
		t.Errorf("MockTransport.Send() allocations = %.1f, want 1", allocs)
	}
}

func TestGenerateComplexWave(t *testing.T) {
	tests := []struct {
		name       string
		size       int
		sampleRate float64
	}{
		{"Standard", 1024, 44100},
		{"Small", 16, 8000},
		{"Large", 8192, 96000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateComplexWave(tt.size, tt.sampleRate)

			if len(result) != tt.size {
				t.Errorf("GenerateComplexWave() buffer size = %d, want %d",
					len(result), tt.size)
			}

			// Check non-zero values (signal should have content).
			hasNonZero := false
			for _, v := range result {
				if v != 0 {
					hasNonZero = true
					break
				}
			}

			if !hasNonZero {
				t.Errorf("GenerateComplexWave() produced all zeros")
			}
		})
	}
}

func TestGenerateSineWave(t *testing.T) {
	tests := []struct {
		name       string
		size       int
		sampleRate float64
		frequency  float64
	}{
		{"A4 Note", 1024, 44100, 440.0},
		{"Middle C", 1024, 44100, 261.63},
		{"High Sample Rate", 1024, 192000, 440.0},
		{"Low Sample Rate", 1024, 8000, 440.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateSineWave(tt.size, tt.sampleRate, tt.frequency)

			if len(result) != tt.size {
				t.Errorf("GenerateSineWave() buffer size = %d, want %d",
					len(result), tt.size)
			}

			// For a sine wave, we expect samplesPerCycle = sampleRate / frequency.
			// We'll verify that the values "cross zero" at approximately the right intervals.
			samplesPerCycle := tt.sampleRate / tt.frequency

			// Check for zero crossings.
			if samplesPerCycle > 2 && float64(tt.size) > samplesPerCycle {
				crossCount := 0
				for i := 1; i < tt.size; i++ {
					if (result[i-1] < 0 && result[i] >= 0) ||
						(result[i-1] >= 0 && result[i] < 0) {
						crossCount++
					}
				}

				// Rough approximation of expected crossings (2 per cycle).
				expectedCrossings := float64(tt.size) / (samplesPerCycle / 2)
				// Allow 20% margin of error due to phase alignment and sampling.
				tolerance := 0.2 * expectedCrossings

				if math.Abs(float64(crossCount)-expectedCrossings) > tolerance {
					t.Errorf("GenerateSineWave() zero crossings = %d, expected approximately %.1f±%.1f",
						crossCount, expectedCrossings, tolerance)
				}
			}
		})
	}
}

func TestFindPeakBin(t *testing.T) {
	tests := []struct {
		name     string
		mags     []float64
		start    int
		end      int
		expected int
	}{
		{"Full Range", testMagnitudes, 0, testSize - 1, testSize / 4},
		{"Partial Range Start", testMagnitudes, testSize / 8, testSize - 1, testSize / 4},
		{"Partial Range End", testMagnitudes, 0, testSize / 3, testSize / 4},
		{"Negative Start", testMagnitudes, -10, testSize - 1, testSize / 4},
		{"Out of Range End", testMagnitudes, 0, testSize * 2, testSize / 4},
		{"Empty Slice", []float64{}, 0, 10, 0},
		{"Single Value", []float64{1.0}, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindPeakBin(tt.mags, tt.start, tt.end)

			if len(tt.mags) == 0 {
				return
			}

			if result != tt.expected {
				t.Errorf("FindPeakBin() = %d, want %d", result, tt.expected)
			}
		})
	}

	allocs := testing.AllocsPerRun(100, func() {
		FindPeakBin(testMagnitudes, 0, len(testMagnitudes)-1)
	})

	if allocs > 0 {
		t.Errorf("FindPeakBin allocated memory: got %.1f allocs, want 0", allocs)
	}
}

func BenchmarkMockTransportSend(b *testing.B) {
	benchmarks := []struct {
		name string
		size int
	}{
		{"Small", 64},
		{"Medium", 1024},
		{"Large", 8192},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			mt := &MockTransport{}
			data := make([]float64, bm.size)

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				mt.Send(data)
			}
		})
	}
}

func BenchmarkGenerateComplexWave(b *testing.B) {
	benchmarks := []struct {
		name string
		size int
	}{
		{"Small", 64},
		{"Standard", 1024},
		{"Large", 8192},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				GenerateComplexWave(bm.size, testSampleRate)
			}
		})
	}
}

func BenchmarkGenerateSineWave(b *testing.B) {
	benchmarks := []struct {
		name string
		size int
	}{
		{"Small", 64},
		{"Standard", 1024},
		{"Large", 8192},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				GenerateSineWave(bm.size, testSampleRate, testFrequency)
			}
		})
	}
}

func BenchmarkFindPeakBin(b *testing.B) {
	benchmarks := []struct {
		name string
		size int
	}{
		{"Small", 64},
		{"Standard", 1024},
		{"Large", 8192},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Create magnitudes array of appropriate size.
			mags := make([]float64, bm.size)
			// Set a known peak in the middle.
			peakPos := bm.size / 2
			for i := range mags {
				mags[i] = math.Exp(-0.01 * math.Pow(float64(i-peakPos), 2))
			}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				FindPeakBin(mags, 0, bm.size-1)
			}
		})
	}
}
