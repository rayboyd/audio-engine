package audio

import (
	"testing"
)

// TestBranchlessAbsPerformance verifies the branchless absolute value calculation has no allocations
func TestBranchlessAbsPerformance(t *testing.T) {
	// Sample data with different values to test
	samples := make([]int32, 1024)
	for i := range samples {
		// Mix of positive and negative values
		if i%2 == 0 {
			samples[i] = int32(i * 1000)
		} else {
			samples[i] = int32(-i * 1000)
		}
	}

	// Test allocation-free branchless abs
	allocs := testing.AllocsPerRun(100, func() {
		for i, sample := range samples {
			mask := sample >> 31
			samples[i] = (sample ^ mask) - mask
		}
	})

	if allocs > 0 {
		t.Errorf("Expected zero allocations in branchless abs, got %.1f", allocs)
	}
}

// TestNoiseGateHotPath tests the core noise gate algorithm for zero allocations
func TestNoiseGateHotPath(t *testing.T) {
	// Create input buffer
	buffer := make([]int32, 1024)

	// Fill with varied signal levels
	for i := range buffer {
		buffer[i] = int32((i % 100) * 10000000)
	}

	// Create test threshold
	threshold := int32(500000000)

	// Measure allocations in the core noise gate logic
	allocs := testing.AllocsPerRun(100, func() {
		// Find maximum amplitude using the same algorithm as in processBuffer
		var maxAmplitude int32
		for i := 0; i < len(buffer); i++ {
			// Get absolute value without branching
			sample := buffer[i]
			mask := sample >> 31
			amplitude := (sample ^ mask) - mask

			// Update max using math instead of branching
			diff := amplitude - maxAmplitude
			maxAmplitude += (diff & (diff >> 31)) ^ diff
		}

		// Gate check (no actual processing, just the condition check)
		_ = maxAmplitude > threshold
	})

	if allocs > 0 {
		t.Errorf("Expected zero allocations in noise gate hot path, got %.1f", allocs)
	}
}

// BenchmarkHotPath benchmarks the performance of the core processing operations
func BenchmarkHotPath(b *testing.B) {
	// Create input buffer with realistic sample values
	buffer := make([]int32, 1024)
	for i := range buffer {
		buffer[i] = int32((i % 100) * 10000000)
	}

	threshold := int32(500000000)

	// Reset timer to exclude setup time
	b.ResetTimer()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		var maxAmplitude int32
		for j := 0; j < len(buffer); j++ {
			// Get absolute value without branching
			sample := buffer[j]
			mask := sample >> 31
			amplitude := (sample ^ mask) - mask

			// Update max using math instead of branching
			diff := amplitude - maxAmplitude
			maxAmplitude += (diff & (diff >> 31)) ^ diff
		}

		// Gate check
		if maxAmplitude > threshold {
			// Simulate some processing (but don't actually do it)
			_ = maxAmplitude
		}
	}
}
