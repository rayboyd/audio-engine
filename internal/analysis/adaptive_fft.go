package analysis

import (
	"log"
	"math"
	"sort"
	"time"
)

// AdaptiveFFTParams holds dynamic FFT parameters controlled by AI
type AdaptiveFFTParams struct {
	WindowType        WindowFunc    // Current window function
	FFTSize           int           // Current FFT size
	MaxFFTSize        int           // Maximum allowed FFT size
	SmoothingFactor   float64       // Temporal smoothing (0-1)
	ScaleFactor       float64       // Magnitude scaling
	EnergyThreshold   float64       // Energy threshold for adaptation
	LastAdaptation    time.Time     // Timestamp of last parameter change
	LastFFTSizeChange time.Time     // Track when FFT size last changed
	AdaptationPeriod  time.Duration // Minimum time between adaptations
	FFTSizeCooldown   time.Duration // Minimum time between FFT size changes
	energySamples     []float64     // Store energy samples for calibration
	isCalibrating     bool          // Whether currently calibrating
}

// AdaptationChanges tracks which aspects of the FFT processing changed
type AdaptationChanges struct {
	WindowChanged   bool
	FFTSizeChanged  bool
	SmootherChanged bool
	ScalerChanged   bool
}

// NewAdaptiveController creates an AI-driven parameter controller
func NewAdaptiveController() *AdaptiveFFTParams {
	controller := &AdaptiveFFTParams{
		WindowType:       Hann,                   // Start with Hann window
		FFTSize:          256,                    // Start with small FFT size
		MaxFFTSize:       4096,                   // Cap the FFT size
		SmoothingFactor:  0.3,                    // Moderate smoothing
		ScaleFactor:      1.0,                    // Default scaling
		EnergyThreshold:  0.01,                   // Initial threshold, will be calibrated
		AdaptationPeriod: time.Millisecond * 500, // Adapt at most every 500ms
		FFTSizeCooldown:  time.Second * 3,        // Wait 3 seconds between FFT size changes
		isCalibrating:    true,                   // Start in calibration mode
		energySamples:    make([]float64, 0, 30), // Reserve space for 30 samples
	}

	log.Printf("FFT Adaptive Controller: Starting calibration phase")
	return controller
}

// AdaptToAudio analyzes audio characteristics and adjusts FFT parameters
func (a *AdaptiveFFTParams) AdaptToAudio(buffer []int32, currentMagnitudes []float64) AdaptationChanges {
	// Initialize changes tracker
	changes := AdaptationChanges{}

	// Don't adapt too frequently to avoid visual jitter
	if time.Since(a.LastAdaptation) < a.AdaptationPeriod {
		return changes
	}

	// Calculate audio energy (RMS)
	var sum float64
	for _, sample := range buffer {
		normalized := float64(sample) / float64(0x7FFFFFFF)
		sum += normalized * normalized
	}
	rmsEnergy := math.Sqrt(sum / float64(len(buffer)))

	// Calculate spectral characteristics
	var lowEnergy, midEnergy, highEnergy float64
	if len(currentMagnitudes) > 0 {
		midBound := len(currentMagnitudes) / 3
		highBound := (len(currentMagnitudes) * 2) / 3

		for i, mag := range currentMagnitudes {
			if i < midBound {
				lowEnergy += mag
			} else if i < highBound {
				midEnergy += mag
			} else {
				highEnergy += mag
			}
		}
	}

	// 1. For high energy transients, reduce smoothing for responsiveness
	if rmsEnergy > a.EnergyThreshold*3 {
		newSmoothing := math.Max(0.1, a.SmoothingFactor*0.8)
		if math.Abs(newSmoothing-a.SmoothingFactor) > 0.05 {
			a.SmoothingFactor = newSmoothing
			changes.SmootherChanged = true
		}
	} else {
		// For low energy, increase smoothing for stability
		newSmoothing := math.Min(0.7, a.SmoothingFactor*1.1)
		if math.Abs(newSmoothing-a.SmoothingFactor) > 0.05 {
			a.SmoothingFactor = newSmoothing
			changes.SmootherChanged = true
		}
	}

	// 2. Adapt window function based on spectral content
	newWindow := a.WindowType
	if highEnergy > lowEnergy*2 && highEnergy > midEnergy*2 {
		// High frequency content - Blackman has good high freq resolution
		newWindow = Blackman
	} else if lowEnergy > highEnergy*2 && lowEnergy > midEnergy*1.5 {
		// Bass-heavy content - Hamming has good main lobe
		newWindow = Hamming
	} else if midEnergy > lowEnergy && midEnergy > highEnergy {
		// Mid-range content - Hann is a good all-around window
		newWindow = Hann
	}

	if newWindow != a.WindowType {
		a.WindowType = newWindow
		changes.WindowChanged = true
	}

	// 3. Adapt FFT size based on energy dynamics but with cooldown and max limit
	if time.Since(a.LastFFTSizeChange) > a.FFTSizeCooldown {
		oldSize := a.FFTSize
		sizeChanged := false

		// Calculate energy ratio relative to threshold for better decisions
		energyRatio := rmsEnergy / a.EnergyThreshold

		if energyRatio > 5.0 && a.FFTSize > 1024 {
			// High energy, reduce FFT size for responsiveness
			a.FFTSize = oldSize / 2
			sizeChanged = true
			log.Printf("Adaptive FFT: Decreasing FFT size to %d (energy ratio: %.2f)",
				a.FFTSize, energyRatio)
		} else if energyRatio < 0.3 && a.FFTSize < a.MaxFFTSize {
			// Low energy, increase FFT size for detail, with hard limit
			// But be more selective about when to increase
			if lowEnergy > (midEnergy+highEnergy)*0.7 {
				// Increase mainly when we have significant low frequency content
				a.FFTSize = oldSize * 2
				sizeChanged = true
				log.Printf("Adaptive FFT: Increasing FFT size to %d (energy ratio: %.2f, bass heavy)",
					a.FFTSize, energyRatio)
			}
		}

		if sizeChanged {
			a.LastFFTSizeChange = time.Now()
			changes.FFTSizeChanged = true
		}
	}

	// 4. Apply dynamic scaling based on overall energy
	newScale := 1.0
	if rmsEnergy > 0 {
		// Scale inversely with energy to prevent clipping
		energyFactor := math.Log10(1 + rmsEnergy*10)
		newScale = 1.0 / math.Max(0.1, energyFactor)
	}

	if math.Abs(newScale-a.ScaleFactor) > 0.1 {
		a.ScaleFactor = 0.7*a.ScaleFactor + 0.3*newScale // Smooth the scale changes
		changes.ScalerChanged = true
	}

	// Update the last adaptation timestamp if any changes occurred
	if changes.WindowChanged || changes.FFTSizeChanged ||
		changes.SmootherChanged || changes.ScalerChanged {
		a.LastAdaptation = time.Now()
	}

	return changes
}

// CalibrateThreshold automatically determines an appropriate energy threshold
func (a *AdaptiveFFTParams) CalibrateThreshold(buffer []int32) {
	// Calculate current energy
	var sum float64
	for _, sample := range buffer {
		normalized := float64(sample) / float64(0x7FFFFFFF)
		sum += normalized * normalized
	}
	energy := math.Sqrt(sum / float64(len(buffer)))

	// Collect samples
	a.energySamples = append(a.energySamples, energy)

	// After collecting enough samples, set the threshold
	if len(a.energySamples) >= 30 {
		// Sort samples
		sort.Float64s(a.energySamples)

		// Use 25th percentile as the threshold
		idx := len(a.energySamples) / 4
		a.EnergyThreshold = a.energySamples[idx] * 2

		log.Printf("Adaptive FFT: Calibrated energy threshold to %.6f", a.EnergyThreshold)

		// Clear samples and end calibration
		a.energySamples = nil
		a.isCalibrating = false
	}
}
