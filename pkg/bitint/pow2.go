/*
Package bitint provides bit manipulation functions optimized for
real-time audio processing. The package focuses on power-of-2
operations commonly needed in FFT and buffer sizing.

Design Principles:
- Zero Allocations: All operations use stack memory only
- Predictable Performance: O(1) constant time operations
- Platform Aware: Optimized for both 32-bit and 64-bit platforms
- Real-Time Safe: No locks, syscalls, or blocking operations

Usage:

	// Find next power of 2 for buffer sizing
	bufferSize := bitint.NextPowerOfTwo(1000) // Returns 1024

	// Verify FFT window size is valid
	isValid := bitint.IsPowerOfTwo(windowSize)

----------------------------------------------------------------------

What this code does:

	NextPowerOfTwo returns the next power of 2 greater than or
	equal to size. For powers of 2, it returns the same value.
	For other values, it returns the next higher power of 2.

	The subtraction (size-1) is critical, without the subtraction,
	powers of 2 would be incorrectly doubled.

	WITH subtraction (correct):
	- For input 8 (already a power of 2):
	  size-1 = 7 (binary 0111)
	  bits.Len32(7) = 3 (highest bit position is 2^2)
	  1 << 3 = 8 (correctly preserves original power of 2)

	WITHOUT subtraction (incorrect):
	- For input 8 (already a power of 2):
	  bits.Len32(8) = 4 (binary 1000 has its highest bit position at 2^3)
	  1 << 4 = 16 (incorrectly doubles the input)

	This ensures we get exactly the right shift amount to return
	the same value for powers of 2, and the next power of 2 for
	all other values.
*/
package bitint

import "math/bits"

// NextPowerOfTwo returns the next power of 2 >= size.
// Algorithm explained:
//  1. Subtract 1 from size to handle exact powers of 2
//  2. Find position of highest set bit
//  3. Shift 1 left by that position
//
// Examples:
//
//	Input  Output  Explanation
//	4      4      Already power of 2 (preserved)
//	5      8      Next power after 5
//	0      1      Handle zero case
//	-1     1      Handle negative case
func NextPowerOfTwo(size int) int {
	if size <= 0 {
		return 1
	}

	// 64-bit platforms (where int is 64-bit)
	if ^uint(0)>>63 == 0 {
		return int(1 << (bits.Len64(uint64(size - 1))))
	}

	// 32-bit platforms
	return int(1 << (bits.Len32(uint32(size - 1))))
}

// For 32-bit integers
func NextPowerOfTwo32(size int32) int32 {
	if size <= 0 {
		return 1
	}
	return int32(1 << (bits.Len32(uint32(size - 1))))
}

// For 64-bit integers
func NextPowerOfTwo64(size int64) int64 {
	if size <= 0 {
		return 1
	}
	return int64(1 << (bits.Len64(uint64(size - 1))))
}

// IsPowerOfTwo checks if n is a power of 2 using bit manipulation.
// The expression (n & (n-1)) == 0 works because:
//   - Powers of 2 have exactly one bit set
//   - Subtracting 1 from a power of 2 sets all lower bits
//   - AND operation will be 0 only for powers of 2
//
// Examples:
//
//	Input  Output  Binary
//	8      true    1000 & 0111 = 0000
//	7      false   0111 & 0110 = 0110
//	0      false   Not positive
//	-8     false   Not positive
func IsPowerOfTwo(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}

// For 32-bit integers
func IsPowerOfTwo32(n int32) bool {
	return n > 0 && (n&(n-1)) == 0
}

// For 64-bit integers
func IsPowerOfTwo64(n int64) bool {
	return n > 0 && (n&(n-1)) == 0
}
