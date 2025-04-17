package analysis

import (
	"audio/internal/transport"
	"log"
	"math"
)

type AdaptiveFFTProcessor struct {
	*Processor
	controller *AdaptiveFFTParams
	bufferPool [][]float64 // Pool of magnitude buffers for visualization
}

func NewAdaptiveFFTProcessor(initialSize int, sampleRate float64, transport transport.Transport) *AdaptiveFFTProcessor {
	controller := NewAdaptiveController()
	controller.FFTSize = initialSize

	return &AdaptiveFFTProcessor{
		Processor:  NewFFTProcessor(initialSize, sampleRate, transport, controller.WindowType),
		controller: controller,
		bufferPool: make([][]float64, 3), // Keep a few recent frames for smoothing
	}
}

func (a *AdaptiveFFTProcessor) Process(inputBuffer []int32) {
	// Calculate energy for metadata
	var sum float64
	for _, sample := range inputBuffer {
		normalized := float64(sample) / float64(0x7FFFFFFF)
		sum += normalized * normalized
	}
	rmsEnergy := math.Sqrt(sum / float64(len(inputBuffer)))

	// Handle calibration first
	if a.controller.isCalibrating {
		a.controller.CalibrateThreshold(inputBuffer)
		// Still process the audio during calibration, but don't adapt
		a.Processor.Process(inputBuffer)
		return
	}

	// Get current magnitudes for analysis
	currentMags := a.GetMagnitudes()

	// Let AI adapt parameters based on audio characteristics
	changes := a.controller.AdaptToAudio(inputBuffer, currentMags)

	// Send metadata if anything changed or periodically
	if changes.FFTSizeChanged || changes.WindowChanged ||
		changes.SmootherChanged || changes.ScalerChanged {
		// Send metadata about FFT parameters
		metadata := FFTMetadata{
			FFTSize:    a.controller.FFTSize,
			WindowType: int(a.controller.WindowType),
			Energy:     rmsEnergy,
		}

		if a.transport != nil {
			a.transport.Send(metadata)
		}
	}

	// Only recreate processor if fundamental FFT parameters changed
	if changes.FFTSizeChanged || changes.WindowChanged {
		log.Printf("Recreating FFT processor - size: %d, window: %v",
			a.controller.FFTSize, a.controller.WindowType)

		a.Processor = NewFFTProcessor(
			a.controller.FFTSize,
			a.sampleRate,
			a.transport,
			a.controller.WindowType,
		)
	}

	// Process with current parameters
	a.Processor.Process(inputBuffer)
}

func (a *AdaptiveFFTProcessor) applySmoothing(current []float64) []float64 {
	// Apply temporal smoothing using buffer pool
	smooth := a.controller.SmoothingFactor
	result := make([]float64, len(current))

	// Shift buffer pool to make room for new frame
	if len(a.bufferPool) > 0 {
		// Apply smoothing with previous frames
		for i := range result {
			result[i] = current[i] * a.controller.ScaleFactor
			for j, prev := range a.bufferPool {
				if prev != nil && i < len(prev) {
					weight := smooth * math.Pow(0.5, float64(j+1))
					result[i] = result[i]*(1-weight) + prev[i]*weight
				}
			}
		}

		// Update buffer pool
		copy(a.bufferPool[1:], a.bufferPool)
		a.bufferPool[0] = make([]float64, len(current))
		copy(a.bufferPool[0], current)
	} else {
		// First frame
		for i, mag := range current {
			result[i] = mag * a.controller.ScaleFactor
		}
		a.bufferPool[0] = make([]float64, len(current))
		copy(a.bufferPool[0], current)
	}

	return result
}
