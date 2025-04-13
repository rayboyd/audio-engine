package utils

import "math"

// MockTransport implements the Transport interface for testing.
type MockTransport struct {
	LastData []float64
}

// Send stores the data for later inspection instead of transmitting.
func (m *MockTransport) Send(data []float64) error {
	m.LastData = make([]float64, len(data))
	copy(m.LastData, data)
	return nil
}

func GenerateComplexWave(size int, sampleRate float64) []int32 {
	buffer := make([]int32, size)
	for i := range buffer {
		tm := float64(i) / sampleRate
		signal := math.Sin(2*math.Pi*440*tm)*0.5 +
			math.Sin(2*math.Pi*880*tm)*0.3 +
			math.Sin(2*math.Pi*1320*tm)*0.2 // 440Hz fundamental + harmonics
		buffer[i] = int32(signal * math.MaxInt32 * 0.9)
	}
	return buffer
}

func GenerateSineWave(size int, sampleRate, frequency float64) []int32 {
	buffer := make([]int32, size)
	for i := range buffer {
		t := float64(i) / sampleRate
		buffer[i] = int32(math.Sin(2*math.Pi*frequency*t) * math.MaxInt32 * 0.9)
	}
	return buffer
}

func FindPeakBin(magnitudes []float64, startBin, endBin int) int {
	if len(magnitudes) == 0 {
		return 0
	}

	if startBin < 0 {
		startBin = 0
	}

	if endBin >= len(magnitudes) {
		endBin = len(magnitudes) - 1
	}

	peakBin := startBin
	peakValue := magnitudes[startBin]

	for bin := startBin + 1; bin <= endBin; bin++ {
		if magnitudes[bin] > peakValue {
			peakValue = magnitudes[bin]
			peakBin = bin
		}
	}

	return peakBin
}
