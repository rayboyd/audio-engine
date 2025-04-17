// filepath: /Users/ray/2025/grec-v2/internal/analysis/kick_detector.go
package analysis

import (
	"audio/internal/transport"
	"log"
	"math"
)

// BeatDetector attempts to detect kick drum hits based on energy changes.
type BeatDetector struct {
	threshold       float64             // Energy threshold for detection
	lastEnergy      float64             // Energy of the previous buffer
	minEnergyRatio  float64             // Minimum ratio increase to trigger detection
	sampleRate      float64             // Needed for potential future frequency analysis
	framesPerBuffer int                 // Size of the buffer
	transport       transport.Transport // Transport for sending events
}

func NewBeatDetector(threshold float64, minEnergyRatio float64, sampleRate float64, framesPerBuffer int, transport transport.Transport) *BeatDetector {
	log.Printf("Analysis: Initializing BeatDetector (Threshold: %.2f, MinRatio: %.2f)", threshold, minEnergyRatio)
	return &BeatDetector{
		threshold:       threshold,
		minEnergyRatio:  minEnergyRatio,
		sampleRate:      sampleRate,
		framesPerBuffer: framesPerBuffer,
		transport:       transport,
		lastEnergy:      0.0,
	}
}

// Process analyzes the buffer for kick drum onsets.
// This is a simplified example using overall energy increase.
func (kd *BeatDetector) Process(buffer []int32) {
	currentEnergy := calculateRMS(buffer)

	if currentEnergy > kd.threshold && (kd.lastEnergy == 0 || (currentEnergy/kd.lastEnergy) > kd.minEnergyRatio) {
		if kd.transport != nil {
			eventData := map[string]interface{}{
				"type": "event",
				"name": "kick",
			}
			if err := kd.transport.Send(eventData); err != nil {
				log.Printf("BeatDetector: ERROR sending kick event: %v", err) // More prominent error log
			} else {
				// log.Println("BeatDetector: Event sent successfully.") // Optional success log
			}
		} else {
			log.Println("BeatDetector: Transport is nil, cannot send event.")
		}
		// Idea?? - Add cooldown logic here if needed to prevent rapid-fire events
	}

	kd.lastEnergy = currentEnergy
}

// calculateRMS calculates the Root Mean Square energy of the buffer.
func calculateRMS(buffer []int32) float64 {
	if len(buffer) == 0 {
		return 0.0
	}

	var sumSquare float64
	for _, sample := range buffer {
		floatSample := float64(sample) / float64(0x7FFFFFFF)
		sumSquare += floatSample * floatSample
	}

	meanSquare := sumSquare / float64(len(buffer))

	return math.Sqrt(meanSquare)
}
