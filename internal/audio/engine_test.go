// SPDX-License-Identifier: MIT
package audio

import (
	"os"
	"testing"
)

const (
	testFrameSize  = 1024
	testSampleRate = 44100
)

var (
	testBuffer    []int32
	quietBuffer   []int32
	loudBuffer    []int32
	lowThreshold  int32
	highThreshold int32
)

func TestMain(m *testing.M) {
	testBuffer = make([]int32, testFrameSize)
	quietBuffer = make([]int32, testFrameSize)
	loudBuffer = make([]int32, testFrameSize)

	for i := range testBuffer {
		testBuffer[i] = int32((i % 100) * 10000000) // ~10% of int32 max
		quietBuffer[i] = int32((i % 100) * 10000)   // ~0.01% of int32 max - much quieter
		loudBuffer[i] = int32((i % 100) * 50000000) // ~50% of int32 max
	}

	lowThreshold = int32(1000000)     // ~0.05% of max int32
	highThreshold = int32(2147000000) // ~99.9% of max int32 - needs to be high enough to block all signals

	// Run all tests.
	exitCode := m.Run()

	// Clean up if needed.
	// (No specific cleanup needed in this case)

	os.Exit(exitCode)
}

func TestBranchlessAbsHotPath(t *testing.T) {
	// Warm-up call before allocation testing
	for i, sample := range testBuffer {
		mask := sample >> 31
		testBuffer[i] = (sample ^ mask) - mask
	}

	allocs := testing.AllocsPerRun(100, func() {
		for i, sample := range testBuffer {
			mask := sample >> 31
			testBuffer[i] = (sample ^ mask) - mask
		}
	})

	if allocs > 0 {
		t.Errorf("Branchless abs allocated memory: got %.1f allocs, want 0", allocs)
	}
}

func TestBranchlessMaxHotPath(t *testing.T) {
	tests := []struct {
		name    string
		current int32
		new     int32
		max     int32
	}{
		{"New > Current", 10, 20, 20},
		{"Current > New", 20, 10, 20},
		{"Equal Values", 15, 15, 15},
		{"Negative Current", -5, 10, 10},
		{"Negative New", 10, -5, 10},
		{"Both Negative", -10, -5, -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff := tt.new - tt.current
			result := tt.current + ((diff & (diff >> 31)) ^ diff)

			if result != tt.max {
				t.Errorf("BranchlessMax(%d, %d) = %d, want %d",
					tt.current, tt.new, result, tt.max)
			}
		})
	}

	allocs := testing.AllocsPerRun(100, func() {
		var max int32
		for _, sample := range testBuffer {
			// Get absolute value without branching.
			mask := sample >> 31
			amplitude := (sample ^ mask) - mask

			// Update max using math instead of branching.
			diff := amplitude - max
			max += (diff & (diff >> 31)) ^ diff
		}
	})

	if allocs > 0 {
		t.Errorf("Branchless max calculation allocated memory: got %.1f allocs, want 0", allocs)
	}
}

func TestGateThreshold(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{-0.1, 0.0}, // Below min
		{0.0, 0.0},  // Minimum
		{0.5, 0.5},  // Middle
		{1.0, 1.0},  // Maximum
		{1.5, 1.0},  // Above max
	}

	engine := &Engine{
		gateEnabled:   true,
		gateThreshold: 0,
	}

	for _, tt := range tests {
		t.Run(formatFloat(tt.input), func(t *testing.T) {
			engine.SetGateThreshold(tt.input)
			got := engine.GetGateThreshold()

			if absFloat(got-tt.expected) > 0.001 {
				t.Errorf("Gate threshold conversion: got %.3f, want %.3f", got, tt.expected)
			}
		})
	}
}

func TestGateDetection(t *testing.T) {
	tests := []struct {
		desc          string
		buffer        []int32
		threshold     int32
		shouldTrigger bool
	}{
		{"Loud signal above high threshold", loudBuffer, lowThreshold, true},
		{"Normal signal above low threshold", testBuffer, lowThreshold, true},
		{"Quiet signal below low threshold", quietBuffer, lowThreshold, false},
		{"All signals below high threshold", loudBuffer, highThreshold, false},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			var maxAmplitude int32
			for i := range tt.buffer {
				// Get absolute value without branching.
				sample := tt.buffer[i]
				mask := sample >> 31
				amplitude := (sample ^ mask) - mask

				// Update max using math instead of branching.
				diff := amplitude - maxAmplitude
				maxAmplitude += (diff & (diff >> 31)) ^ diff
			}

			triggered := maxAmplitude > tt.threshold

			if triggered != tt.shouldTrigger {
				t.Errorf("Gate detection error: got triggered=%v, want %v (max amplitude=%d, threshold=%d)",
					triggered, tt.shouldTrigger, maxAmplitude, tt.threshold)
			}
		})
	}
}

func BenchmarkBranchlessAbsHotPath(b *testing.B) {
	benchmarks := []struct {
		name   string
		buffer []int32
	}{
		{"Normal signal", testBuffer},
		{"Quiet signal", quietBuffer},
		{"Loud signal", loudBuffer},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				for i, sample := range bm.buffer {
					mask := sample >> 31
					bm.buffer[i] = (sample ^ mask) - mask
				}
			}
		})
	}
}

func BenchmarkNoiseGateHotPath(b *testing.B) {
	benchmarks := []struct {
		name      string
		buffer    []int32
		threshold int32
	}{
		{"Normal signal/Low threshold", testBuffer, lowThreshold},
		{"Quiet signal/Low threshold", quietBuffer, lowThreshold},
		{"Loud signal/High threshold", loudBuffer, highThreshold},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				var maxAmplitude int32
				for _, sample := range bm.buffer {
					// Get absolute value without branching.
					mask := sample >> 31
					amplitude := (sample ^ mask) - mask

					// Update max using math instead of branching.
					diff := amplitude - maxAmplitude
					maxAmplitude += (diff & (diff >> 31)) ^ diff
				}

				// Gate check (discard result to prevent optimization).
				_ = maxAmplitude > bm.threshold
			}
		})
	}
}

// absFloat returns absolute difference between a and b.
func absFloat(a float64) float64 {
	if a < 0 {
		return -a
	}
	return a
}

// formatFloat converts a float to a descriptive string for test names.
func formatFloat(v float64) string {
	switch {
	case v <= 0:
		return "Zero or negative"
	case v < 0.001:
		return "Near zero"
	case v > 0.999 && v < 1.001:
		return "Unity"
	case v > 1:
		return "Above max"
	default:
		return "Mid-range"
	}
}
