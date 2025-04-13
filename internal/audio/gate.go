// SPDX-License-Identifier: MIT
package audio

import "math"

func (e *Engine) EnableGate() {
	e.gateEnabled = true
}

func (e *Engine) DisableGate() {
	e.gateEnabled = false
}

// SetGateThreshold adjusts the noise gate threshold.
// The value is in the range of 0.0-1.0 where 0=always open, 1=always closed.
func (e *Engine) SetGateThreshold(threshold float64) {
	if threshold < 0.0 {
		threshold = 0.0
	}
	if threshold > 1.0 {
		threshold = 1.0
	}

	e.gateThreshold = int32(threshold * float64(math.MaxInt32))
}

// GetGateThreshold returns the current noise gate threshold as a float64.
// The value is in the range of 0.0-1.0 where 0=always open, 1=always closed.
func (e *Engine) GetGateThreshold() float64 {
	return float64(e.gateThreshold) / float64(math.MaxInt32)
}
