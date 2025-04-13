// SPDX-License-Identifier: MIT
package audio

import (
	"math"
	"testing"
)

func TestGateEnableHotPath(t *testing.T) {
	engine := &Engine{
		gateEnabled:   false,
		gateThreshold: lowThreshold,
	}

	if engine.gateEnabled {
		t.Error("Gate should be disabled initially")
	}

	engine.EnableGate()
	if !engine.gateEnabled {
		t.Error("Gate should be enabled after EnableGate()")
	}

	engine.DisableGate()
	if engine.gateEnabled {
		t.Error("Gate should be disabled after DisableGate()")
	}

	engine.EnableGate()
	engine.EnableGate() // Multiple calls should be idempotent
	if !engine.gateEnabled {
		t.Error("Gate should remain enabled after multiple EnableGate()")
	}

	engine.DisableGate()
	engine.DisableGate() // Multiple calls should be idempotent
	if engine.gateEnabled {
		t.Error("Gate should remain disabled after multiple DisableGate()")
	}
}

func TestGateThresholdBoundaries(t *testing.T) {
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

func TestGateThresholdPrecisionHotPath(t *testing.T) {
	engine := &Engine{}

	tests := []struct {
		ratio float64
		desc  string
	}{
		{0.0, "Zero"},           // Min boundary
		{0.1, "10%"},            // Low value
		{0.25, "Quarter"},       // 25%
		{0.5, "Half"},           // Midpoint
		{0.75, "Three quarter"}, // 75%
		{0.999, "Near max"},     // Almost max
		{1.0, "Unity"},          // Max boundary
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			engine.SetGateThreshold(tt.ratio)
			result := engine.GetGateThreshold()

			// Verify conversion accuracy.
			if absFloat(result-tt.ratio) > 0.0001 {
				t.Errorf("Threshold conversion error: got %.6f, want %.6f", result, tt.ratio)
			}

			// Verify int32 representation is proportional.
			expectedInt32 := int32(tt.ratio * float64(math.MaxInt32))
			if absInt32(expectedInt32-engine.gateThreshold) > 100 {
				t.Errorf("Int32 threshold mismatch: got %d, want %d",
					engine.gateThreshold, expectedInt32)
			}
		})
	}
}

func TestGateDetectionHotPath(t *testing.T) {
	tests := []struct {
		desc          string
		buffer        []int32
		gateEnabled   bool
		threshold     float64
		shouldTrigger bool
	}{
		{"Gate disabled/Quiet signal", quietBuffer, false, 0.1, true},                // Disabled gate always passes
		{"Gate disabled/Loud signal", loudBuffer, false, 0.1, true},                  // Disabled gate always passes
		{"Gate enabled/Quiet signal/Low threshold", quietBuffer, true, 0.0001, true}, // Very low threshold that quiet signal can pass
		{"Gate enabled/Quiet signal/Mid threshold", quietBuffer, true, 0.1, false},   // Signal below threshold
		{"Gate enabled/Loud signal/Mid threshold", loudBuffer, true, 0.1, true},      // Signal above threshold
		{"Gate enabled/Loud signal/High threshold", loudBuffer, true, 0.999, false},  // Very high threshold that even loud signal can't pass
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			engine := &Engine{
				gateThreshold: 0,
				gateEnabled:   tt.gateEnabled,
			}

			engine.SetGateThreshold(tt.threshold)

			var maxAmplitude int32
			for _, sample := range tt.buffer {
				// Get absolute value without branching.
				mask := sample >> 31
				amplitude := (sample ^ mask) - mask

				// Update max using math instead of branching.
				diff := amplitude - maxAmplitude
				maxAmplitude += (diff & (diff >> 31)) ^ diff
			}

			triggered := !engine.gateEnabled || (maxAmplitude > engine.gateThreshold)

			if triggered != tt.shouldTrigger {
				t.Errorf("Gate detection error: got triggered=%v, want %v (max amplitude=%d, threshold=%d)",
					triggered, tt.shouldTrigger, maxAmplitude, engine.gateThreshold)
			}
		})
	}
}

func BenchmarkGateThresholdConversionHotPath(b *testing.B) {
	engine := &Engine{}
	values := []float64{0.0, 0.25, 0.5, 0.75, 1.0}

	for _, v := range values {
		b.Run(formatFloat(v), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				engine.SetGateThreshold(v)
				_ = engine.GetGateThreshold() // Discard result to prevent optimization
			}
		})
	}
}

func BenchmarkGateProcessingHotPath(b *testing.B) {
	benchmarks := []struct {
		name      string
		buffer    []int32
		threshold int32
		enabled   bool
	}{
		{"Gate disabled/Normal", testBuffer, lowThreshold, false},
		{"Gate enabled/Quiet signal/Low threshold", quietBuffer, lowThreshold, true},
		{"Gate enabled/Normal signal/Low threshold", testBuffer, lowThreshold, true},
		{"Gate enabled/Loud signal/High threshold", loudBuffer, highThreshold, true},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			engine := &Engine{
				gateEnabled:   bm.enabled,
				gateThreshold: bm.threshold,
			}

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
				_ = !engine.gateEnabled || (maxAmplitude > engine.gateThreshold)
			}
		})
	}
}

// absInt32 returns the absolute value of x.
func absInt32(x int32) int32 {
	mask := x >> 31
	return (x ^ mask) - mask
}
