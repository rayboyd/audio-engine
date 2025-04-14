// SPDX-License-Identifier: MIT
package audio

import (
	"audio/internal/config"
	"audio/internal/fft"
	"runtime"
	"time"

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

func (e *Engine) Close() error {
	err := e.StopInputStream()
	return err
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

// HOTPATH
func (e *Engine) processInputStream(in []int32) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	copy(e.inputBuffer, in)
	e.processBuffer(e.inputBuffer)
}

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
