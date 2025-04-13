// SPDX-License-Identifier: MIT
package bitint

import (
	"fmt"
	"testing"
)

func TestNextPowerOfTwo(t *testing.T) {
	tests := []struct {
		n        int
		expected int
	}{
		{-10, 1},     // Negative number
		{0, 1},       // Zero
		{8, 8},       // Already power of two
		{10, 16},     // Not power of two
		{1000, 1024}, // Large number
		{3, 4},       // Small non-power
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d→%d", tt.n, tt.expected), func(t *testing.T) {
			result := NextPowerOfTwo(tt.n)
			if result != tt.expected {
				t.Errorf("NextPowerOfTwo(%d) = %d, expected %d", tt.n, result, tt.expected)
			}
		})
	}
}

func TestNextPowerOfTwo32(t *testing.T) {
	tests := []struct {
		n        int32
		expected int32
	}{
		{-10, 1},     // Negative number
		{0, 1},       // Zero
		{16, 16},     // Already power of two
		{31, 32},     // Not power of two
		{1023, 1024}, // Large number
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d→%d", tt.n, tt.expected), func(t *testing.T) {
			result := NextPowerOfTwo32(tt.n)
			if result != tt.expected {
				t.Errorf("NextPowerOfTwo32(%d) = %d, expected %d", tt.n, result, tt.expected)
			}
		})
	}
}

func TestNextPowerOfTwo64(t *testing.T) {
	tests := []struct {
		n        int64
		expected int64
	}{
		{-10, 1},                 // Negative number
		{0, 1},                   // Zero
		{4096, 4096},             // Already power of two
		{5000, 8192},             // Not power of two
		{(1 << 30) + 1, 1 << 31}, // Large number
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d→%d", tt.n, tt.expected), func(t *testing.T) {
			result := NextPowerOfTwo64(tt.n)
			if result != tt.expected {
				t.Errorf("NextPowerOfTwo64(%d) = %d, expected %d", tt.n, result, tt.expected)
			}
		})
	}
}

func TestIsPowerOfTwo(t *testing.T) {
	tests := []struct {
		n        int
		expected bool
	}{
		{-2, false},     // Negative number
		{0, false},      // Zero
		{1, true},       // One
		{8, true},       // Power of two
		{10, false},     // Not power of two
		{1 << 20, true}, // Large power of two
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d→%t", tt.n, tt.expected), func(t *testing.T) {
			result := IsPowerOfTwo(tt.n)
			if result != tt.expected {
				t.Errorf("IsPowerOfTwo(%d) = %v, expected %v", tt.n, result, tt.expected)
			}
		})
	}
}

func TestIsPowerOfTwo32(t *testing.T) {
	tests := []struct {
		n        int32
		expected bool
	}{
		{-2, false},     // Negative number
		{0, false},      // Zero
		{1, true},       // One
		{64, true},      // Power of two
		{33, false},     // Not power of two
		{1 << 20, true}, // Large power of two
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d→%t", tt.n, tt.expected), func(t *testing.T) {
			result := IsPowerOfTwo32(tt.n)
			if result != tt.expected {
				t.Errorf("IsPowerOfTwo32(%d) = %v, expected %v", tt.n, result, tt.expected)
			}
		})
	}
}

func TestIsPowerOfTwo64(t *testing.T) {
	tests := []struct {
		n        int64
		expected bool
	}{
		{-2, false},     // Negative number
		{0, false},      // Zero
		{1, true},       // One
		{1 << 10, true}, // Power of two
		{129, false},    // Not power of two
		{1 << 40, true}, // Large power of two
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d→%t", tt.n, tt.expected), func(t *testing.T) {
			result := IsPowerOfTwo64(tt.n)
			if result != tt.expected {
				t.Errorf("IsPowerOfTwo64(%d) = %v, expected %v", tt.n, result, tt.expected)
			}
		})
	}
}

func BenchmarkNextPowerOfTwo(b *testing.B) {
	var i int
	b.ReportAllocs()
	for b.Loop() {
		NextPowerOfTwo(i % 10000)
		i++
	}
}

func BenchmarkIsPowerOfTwo(b *testing.B) {
	var i int
	b.ReportAllocs()
	for b.Loop() {
		IsPowerOfTwo(i % 10000)
		i++
	}
}
