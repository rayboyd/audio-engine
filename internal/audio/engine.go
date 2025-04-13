// SPDX-License-Identifier: MIT
/*
Package audio implements a real-time audio processing engine with:
- Lock-free audio capture using PortAudio
- Real-time FFT analysis with configurable bands
- Noise gate with branchless implementation
- WAV recording with atomic state management

Thread Safety:
- Uses atomic operations for state management
- Pre-allocates buffers to avoid GC in hot path
- Locks OS thread during audio processing
*/
package audio

import (
	"audio/internal/config"
	"audio/internal/fft"
	"log"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/gordonklaus/portaudio"
)

type Engine struct {
	// Core configuration and state.
	config *config.Config

	// Audio input handling.
	inputBuffer  []int32
	inputDevice  *portaudio.DeviceInfo
	inputLatency time.Duration
	inputStream  *portaudio.Stream

	// FFT processing for real-time analysis.
	fftProcessor *fft.Processor
	fftMonoInput []int32 // Mono input buffer for FFT processing

	// Noise gate for signal conditioning.
	gateEnabled   bool
	gateThreshold int32 // Absolute amplitude threshold (0-2147483647)

	// Recording state and buffers.
	isRecording int32 // Atomic flag for thread-safe state
	outputFile  *os.File
	wavEncoder  *wav.Encoder
	sampleBuf   *audio.IntBuffer // Reusable buffer for format conversion
}

func NewEngine(config *config.Config) (engine *Engine, err error) {
	inputDevice, err := InputDevice(config.DeviceID)
	if err != nil {
		return nil, err
	}

	wsTransport := fft.NewWebSocketTransport("8080")
	fftProcessor := fft.NewProcessor(
		config.FramesPerBuffer,
		float64(config.SampleRate),
		wsTransport,
	)

	// Pre-allocate mono input buffer for FFT processing.
	fftMonoInput := make([]int32, config.FramesPerBuffer)

	// Pre-allocate I/O buffers sized for frames × channels.
	inputSize := config.FramesPerBuffer * config.Channels

	engine = &Engine{
		config:        config,
		inputBuffer:   make([]int32, inputSize),
		inputDevice:   inputDevice,
		fftProcessor:  fftProcessor,
		fftMonoInput:  fftMonoInput,
		gateEnabled:   true,
		gateThreshold: 2147483647 / 1000, // Default to ~0.1% of max value
	}

	if engine.config.LowLatency {
		engine.inputLatency = engine.inputDevice.DefaultLowInputLatency
	} else {
		engine.inputLatency = engine.inputDevice.DefaultHighInputLatency
	}

	return engine, nil
}

func (e *Engine) StartInputStream() error {
	params := portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Channels: e.config.Channels,
			Device:   e.inputDevice,
			Latency:  e.inputLatency,
		},
		Output: portaudio.StreamDeviceParameters{
			Channels: 0, // No output device
			Device:   nil,
		},
		FramesPerBuffer: e.config.FramesPerBuffer,
		SampleRate:      e.config.SampleRate,
	}

	stream, err := portaudio.OpenStream(params, e.processInputStream)
	if err != nil {
		return err
	}
	e.inputStream = stream

	if err := e.inputStream.Start(); err != nil {
		e.inputStream.Close()
		return err
	}

	return nil
}

func (e *Engine) StopInputStream() error {
	if e.inputStream != nil {
		if err := e.inputStream.Stop(); err != nil {
			return err
		}

		if err := e.inputStream.Close(); err != nil {
			return err
		}

		e.inputStream = nil
	}

	return nil
}

// processInputStream is the core audio processing callback.
// Performance Critical:
// - Runs in a dedicated OS thread (LockOSThread)
// - Uses pre-allocated buffers only
// - No dynamic allocations in the hot path
func (e *Engine) processInputStream(in []int32) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	copy(e.inputBuffer, in)
	e.processBuffer(e.inputBuffer)

	// Write to WAV file if recording
	if atomic.LoadInt32(&e.isRecording) == 1 && e.wavEncoder != nil {
		for i, sample := range e.inputBuffer {
			e.sampleBuf.Data[i] = int(sample)
		}

		e.sampleBuf.Data = e.sampleBuf.Data[:len(e.inputBuffer)]

		if err := e.wavEncoder.Write(e.sampleBuf); err != nil {
			log.Printf("Error writing to WAV file: %v", err)
		}
	}
}

// processBuffer performs all DSP operations on the audio buffer in-place.
// Performance Critical (Hot Path):
// - No allocations
// - Branchless noise gate implementation
// - Direct FFT processing call
func (e *Engine) processBuffer(buffer []int32) {
	// Determine if FFT processing should occur based on gate.
	shouldProcessFFT := false
	if e.gateEnabled {
		var maxAmplitude int32
		for i := range buffer {
			sample := buffer[i]
			mask := sample >> 31
			amplitude := (sample ^ mask) - mask
			diff := amplitude - maxAmplitude
			maxAmplitude += (diff & (diff >> 31)) ^ diff
		}
		if maxAmplitude > e.gateThreshold {
			shouldProcessFFT = true
		}
	} else {
		shouldProcessFFT = (e.fftProcessor != nil)
	}

	// Process FFT if needed.
	if shouldProcessFFT && e.fftProcessor != nil {
		var fftInputBuffer []int32
		if e.config.Channels == 1 {
			fftInputBuffer = buffer
		} else {
			for i := range e.config.FramesPerBuffer {
				if i*e.config.Channels < len(buffer) {
					e.fftMonoInput[i] = buffer[i*e.config.Channels]
				} else {
					e.fftMonoInput[i] = 0 // Safety fallback
				}
			}
			fftInputBuffer = e.fftMonoInput
		}

		e.fftProcessor.Process(fftInputBuffer)
	}
}
