// SPDX-License-Identifier: MIT
package fft

import (
	"audio/pkg/utils"
	"math"
	"os"
	"testing"
)

const (
	testFFTSize    = 1024
	testSampleRate = 44100
)

var (
	testProcessor   *Processor
	mockTransport   *utils.MockTransport
	testInputBuffer []int32
	sineWave440Hz   []int32
	complexWave     []int32
)

func TestMain(m *testing.M) {
	mockTransport = &utils.MockTransport{}
	testProcessor = NewProcessor(testFFTSize, testSampleRate, nil)

	testInputBuffer = make([]int32, testFFTSize)
	for i := range testInputBuffer {
		testInputBuffer[i] = int32((i%256 - 128) * 1000000)
	}

	sineWave440Hz = utils.GenerateSineWave(testFFTSize, testSampleRate, 440.0)
	complexWave = utils.GenerateComplexWave(testFFTSize, testSampleRate)

	// Run all tests.
	exitCode := m.Run()

	// Clean up if needed.
	// (No specific cleanup needed in this case)

	os.Exit(exitCode)
}

func TestFFTHotPath(t *testing.T) {
	testProcessor.Process(testInputBuffer) // Warm-up call before allocation testing

	allocs := testing.AllocsPerRun(100, func() {
		testProcessor.Process(testInputBuffer)
	})

	if allocs > 0 {
		t.Errorf("FFT Process allocated memory: got %.1f allocs, want 0", allocs)
	}
}

func TestGetFrequencyBin(t *testing.T) {
	tests := []struct {
		bin  int
		desc string
	}{
		{0, "DC component"},
		{10, "Low frequency"},
		{testFFTSize / 4, "Mid frequency"},
		{testFFTSize / 2, "Nyquist frequency"},
	}

	allocs := testing.AllocsPerRun(100, func() {
		for _, tt := range tests {
			_ = testProcessor.GetFrequencyBin(tt.bin)
		}
	})

	if allocs > 0 {
		t.Errorf("GetFrequencyBin allocated memory: got %.1f allocs, want 0", allocs)
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			freq := testProcessor.GetFrequencyBin(tt.bin)
			expectedFreq := float64(tt.bin) * testSampleRate / testFFTSize
			if math.Abs(freq-expectedFreq) > 0.001 {
				t.Errorf("GetFrequencyBin(%d) = %.2f Hz, want %.2f Hz",
					tt.bin, freq, expectedFreq)
			}
		})
	}
}

func TestProcessWithMockTransport(t *testing.T) {
	tests := []struct {
		desc      string
		frequency float64
		signal    []int32
	}{
		{"440 Hz (A4 note)", 440.0, sineWave440Hz},
		{"1 kHz tone", 1000.0, utils.GenerateSineWave(testFFTSize, testSampleRate, 1000.0)},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			mt := &utils.MockTransport{}
			p := NewProcessor(testFFTSize, testSampleRate, mt)

			p.Process(tt.signal)

			expectedBin := int(tt.frequency * float64(testFFTSize) / testSampleRate)

			if mt.LastData == nil {
				t.Fatal("No data sent to transport")
			}

			peakBin := utils.FindPeakBin(mt.LastData, 0, len(mt.LastData)-1)

			maxAllowedBinDiff := 2 // Allow for some error due to FFT windowing
			if abs(peakBin-expectedBin) > maxAllowedBinDiff {
				peakFreq := float64(peakBin) * testSampleRate / testFFTSize
				t.Errorf("Expected peak near bin %d (%.1f Hz), but found peak at bin %d (%.1f Hz)",
					expectedBin, tt.frequency, peakBin, peakFreq)
			}
		})
	}
}

func BenchmarkProcess(b *testing.B) {
	processor := NewProcessor(testFFTSize, testSampleRate, nil)

	benchmarks := []struct {
		name   string
		signal []int32
	}{
		{"SineWave440Hz", sineWave440Hz},
		{"ComplexHarmonics", complexWave},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				processor.Process(bm.signal)
			}
		})
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
