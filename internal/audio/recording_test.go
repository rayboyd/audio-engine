// SPDX-License-Identifier: MIT
package audio

import (
	"audio/internal/config"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

var testRecordingDir string

func init() {
	var err error
	testRecordingDir, err = os.MkdirTemp("", "test_recording")
	if err != nil {
		panic("Failed to create temp dir for recording tests: " + err.Error())
	}
}

func newTestEngine() *Engine {
	return &Engine{
		config: &config.Config{
			SampleRate:      testSampleRate,
			Channels:        2,
			FramesPerBuffer: testFrameSize,
		},
	}
}

func TestRecordingStartStopHotPath(t *testing.T) {
	filename := filepath.Join(testRecordingDir, "test_recording.wav")
	engine := newTestEngine()

	if err := engine.StartRecording(filename); err != nil {
		t.Fatalf("Failed to start recording: %v", err)
	}

	if atomic.LoadInt32(&engine.isRecording) != 1 {
		t.Error("Engine should be in recording state")
	}

	if engine.outputFile == nil {
		t.Error("Output file should be initialized")
	}

	if engine.wavEncoder == nil {
		t.Error("WAV encoder should be initialized")
	}

	if engine.sampleBuf == nil {
		t.Error("Sample buffer should be initialized")
	}

	if engine.sampleBuf.Format.NumChannels != engine.config.Channels {
		t.Errorf("Buffer channels mismatch: got %d, want %d",
			engine.sampleBuf.Format.NumChannels, engine.config.Channels)
	}

	if engine.sampleBuf.Format.SampleRate != int(engine.config.SampleRate) {
		t.Errorf("Buffer sample rate mismatch: got %d, want %d",
			engine.sampleBuf.Format.SampleRate, int(engine.config.SampleRate))
	}

	if len(engine.sampleBuf.Data) != engine.config.FramesPerBuffer*engine.config.Channels {
		t.Errorf("Buffer size mismatch: got %d, want %d",
			len(engine.sampleBuf.Data), engine.config.FramesPerBuffer*engine.config.Channels)
	}

	// Store reference to check file closure.
	outputFile := engine.outputFile

	if err := engine.StopRecording(); err != nil {
		t.Fatalf("Failed to stop recording: %v", err)
	}

	if atomic.LoadInt32(&engine.isRecording) != 0 {
		t.Error("Engine should not be in recording state after stopping")
	}

	if engine.outputFile != nil {
		t.Error("Output file should be nil after stopping")
	}

	if engine.wavEncoder != nil {
		t.Error("WAV encoder should be nil after stopping")
	}

	if err := outputFile.Close(); err == nil {
		t.Error("File should already be closed")
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Error("Recording file was not created")
	}

	os.Remove(filename)
}

func TestRecordingErrorCases(t *testing.T) {
	tests := []struct {
		desc          string
		filename      string
		isRecording   int32
		expectError   bool
		errorContains string
	}{
		{"Already recording", "valid.wav", 1, true, "already recording"},
		{"Invalid path", "/nonexistent/path/file.wav", 0, true, ""},
		{"Valid path", "test.wav", 0, false, ""},
		{"Stop when not recording", "", 0, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			var err error
			engine := newTestEngine()

			atomic.StoreInt32(&engine.isRecording, tt.isRecording) // Set recording state

			if tt.desc == "Stop when not recording" {
				err = engine.StopRecording()
			} else {
				filename := tt.filename
				if tt.errorContains == "" && !tt.expectError {
					filename = filepath.Join(testRecordingDir, tt.filename)
				}

				err = engine.StartRecording(filename)
				if err == nil {
					_ = engine.StopRecording()
				}
			}

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.errorContains != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error %q does not contain %q", err.Error(), tt.errorContains)
				}
			}
		})
	}
}

func TestCloseEngineWithRecording(t *testing.T) {
	filename := filepath.Join(testRecordingDir, "test_close_engine.wav")
	engine := newTestEngine()

	if err := engine.StartRecording(filename); err != nil {
		t.Fatalf("Failed to start recording: %v", err)
	}

	if err := engine.Close(); err != nil {
		t.Fatalf("Failed to close engine: %v", err)
	}

	if atomic.LoadInt32(&engine.isRecording) != 0 {
		t.Error("Engine should not be in recording state after Close()")
	}

	if engine.outputFile != nil {
		t.Error("Output file should be nil after Close()")
	}

	if engine.wavEncoder != nil {
		t.Error("WAV encoder should be nil after Close()")
	}
}

func TestRecordingNoAllocsHotPath(t *testing.T) {
	engine := newTestEngine()
	engine.inputBuffer = make([]int32, engine.config.FramesPerBuffer*engine.config.Channels)

	filename := filepath.Join(testRecordingDir, "test_alloc.wav")
	if err := engine.StartRecording(filename); err != nil {
		t.Fatalf("Failed to start recording: %v", err)
	}
	defer engine.StopRecording()

	// Test for zero allocations during audio processing.
	allocs := testing.AllocsPerRun(100, func() {
		// Simulate audio processing with branchless algorithms.
		var maxAmplitude int32
		for _, sample := range testBuffer {
			mask := sample >> 31
			amplitude := (sample ^ mask) - mask
			diff := amplitude - maxAmplitude
			maxAmplitude += (diff & (diff >> 31)) ^ diff
		}

		// Simulate buffer conversion without actually writing.
		if atomic.LoadInt32(&engine.isRecording) == 1 && engine.sampleBuf != nil {
			for i := 0; i < 10 && i < len(engine.sampleBuf.Data); i++ {
				engine.sampleBuf.Data[i] = int(testBuffer[i])
			}
		}
	})

	if allocs > 0 {
		t.Errorf("Recording hot path allocated memory: got %.1f allocs, want 0", allocs)
	}
}

func BenchmarkRecordingStartStopHotPath(b *testing.B) {
	engine := newTestEngine()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		filename := filepath.Join(testRecordingDir, "bench.wav")
		_ = os.Remove(filename) // Ensure clean state for each iteration
		_ = engine.StartRecording(filename)
		_ = engine.StopRecording()
	}
}

func BenchmarkRecordingProcessHotPath(b *testing.B) {
	engine := newTestEngine()
	engine.inputBuffer = make([]int32, engine.config.FramesPerBuffer*engine.config.Channels)

	filename := filepath.Join(testRecordingDir, "bench_process.wav")
	_ = engine.StartRecording(filename)
	defer engine.StopRecording()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		// Simulate the recording hot path.
		if atomic.LoadInt32(&engine.isRecording) == 1 && engine.sampleBuf != nil {
			for i := 0; i < len(testBuffer) && i < len(engine.sampleBuf.Data); i++ {
				engine.sampleBuf.Data[i] = int(testBuffer[i])
			}
		}
	}
}
