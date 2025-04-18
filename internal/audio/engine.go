// SPDX-License-Identifier: MIT
package audio

import (
	"audio/internal/analysis"
	"audio/internal/config"
	applog "audio/internal/log"
	udpTransport "audio/internal/transport/udp"
	"fmt"
	"sync"
	"time"

	"github.com/gordonklaus/portaudio"
)

// TODO:
// Document this struct.
type Engine struct {
	config       *config.Config
	stream       *portaudio.Stream
	inputDevice  *portaudio.DeviceInfo
	inputLatency time.Duration
	processors   []analysis.AudioProcessor
	closables    []interface{ Close() error }
	streamActive bool
	streamMu     sync.Mutex

	udpSender    *udpTransport.UDPSender
	udpPublisher *udpTransport.UDPPublisher
}

// TODO:
// Document this function.
func NewEngine(config *config.Config) (*Engine, error) {
	if err := portaudio.Initialize(); err != nil {
		return nil, fmt.Errorf("engine: failed to initialize PortAudio: %w", err)
	}

	inputDevice, err := InputDevice(config.Audio.InputDevice)
	if err != nil {
		portaudio.Terminate()
		return nil, fmt.Errorf("engine: failed to get input device: %w", err)
	}

	var latency time.Duration
	if config.Audio.LowLatency {
		latency = inputDevice.DefaultLowInputLatency
	} else {
		latency = inputDevice.DefaultHighInputLatency
	}

	engine := &Engine{
		config:       config,
		inputDevice:  inputDevice,
		inputLatency: latency,
		processors:   make([]analysis.AudioProcessor, 0),
		closables:    make([]interface{ Close() error }, 0),
	}

	// --- Processor Registration ---

	fftWindowFunc, err := analysis.ParseWindowFunc(engine.config.Audio.FFTWindow)
	if err != nil {
		applog.Warnf("Engine Core: %v. Using default FFT window (Hann).", err) // Use Warnf
	}

	fftProcessor, err := analysis.NewFFTProcessor(
		engine.config.Audio.FramesPerBuffer,
		engine.config.Audio.SampleRate,
		fftWindowFunc,
	)
	if err != nil {
		portaudio.Terminate()
		return nil, fmt.Errorf("engine: failed to create FFT processor: %w", err)
	}
	engine.RegisterProcessor(fftProcessor)

	// --- Transport Setup ---

	if config.Transport.UDPEnabled {
		applog.Info("Engine Core: Setting up UDP transport...")

		// TODO:
		// Pass config.Debug to udpTransport.NewUDPSender
		sender, err := udpTransport.NewUDPSender(config.Transport.UDPTargetAddress)
		if err != nil {
			engine.Close()
			return nil, fmt.Errorf("engine: failed to create UDP sender: %w", err)
		}
		engine.udpSender = sender
		engine.closables = append(engine.closables, sender)

		publisher, err := udpTransport.NewUDPPublisher(
			config.Transport.UDPSendInterval,
			sender,
			fftProcessor,
		)
		if err != nil {
			engine.Close()
			return nil, fmt.Errorf("engine: failed to create UDP publisher: %w", err)
		}

		engine.udpPublisher = publisher
		engine.closables = append(engine.closables, publisher)

		applog.Infof("Engine Core: UDP transport initialized (Target: %s, Interval: %s)",
			config.Transport.UDPTargetAddress, config.Transport.UDPSendInterval)
	} else {
		applog.Info("Engine Core: UDP transport is disabled.")
	}

	applog.Infof("Engine Core: Initialized with %d input channels, %d frames/buffer, %.1f Hz sample rate.",
		config.Audio.InputChannels, config.Audio.FramesPerBuffer, config.Audio.SampleRate)
	applog.Infof("Engine Core: Using input device '%s' with latency %v.", inputDevice.Name, engine.inputLatency)

	return engine, nil
}

// TODO:
// Document this function.
func (e *Engine) RegisterProcessor(processor analysis.AudioProcessor) {
	e.processors = append(e.processors, processor)
	if closable, ok := processor.(interface{ Close() error }); ok {
		e.closables = append(e.closables, closable)
		applog.Debugf("Engine Core: Registered closable processor %T", processor)
	} else {
		applog.Debugf("Engine Core: Registered processor %T", processor)
	}
}

// processInputStream is the PortAudio callback function (HOT PATH).
// TODO:
// Document this function.
func (e *Engine) processInputStream(in []int32) {
	// Keep this path clean - NO LOGGING HERE
	for _, processor := range e.processors {
		processor.Process(in)
	}
}

// TODO:
// Document this function.
func (e *Engine) StartInputStream() error {
	e.streamMu.Lock()
	defer e.streamMu.Unlock()

	if e.streamActive {
		applog.Warn("Engine Core: Stream already active.")
		return nil
	}

	streamParameters := portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   e.inputDevice,
			Channels: e.config.Audio.InputChannels,
			Latency:  e.inputLatency,
		},
		SampleRate:      e.config.Audio.SampleRate,
		FramesPerBuffer: e.config.Audio.FramesPerBuffer,
	}

	applog.Info("Engine Core: Opening PortAudio stream...")
	stream, err := portaudio.OpenStream(streamParameters, e.processInputStream)
	if err != nil {
		return fmt.Errorf("engine: failed to open PortAudio stream: %w", err)
	}
	e.stream = stream

	applog.Info("Engine Core: Starting PortAudio stream...")
	if err := e.stream.Start(); err != nil {
		e.stream.Close()
		e.stream = nil
		return fmt.Errorf("engine: failed to start PortAudio stream: %w", err)
	}
	e.streamActive = true
	applog.Info("Engine Core: PortAudio stream started successfully.")

	if e.udpPublisher != nil {
		applog.Info("Engine Core: Starting UDP publisher...")
		e.udpPublisher.Start()
	}

	return nil
}

// TODO:
// Document this function.
func (e *Engine) StopInputStream() error {
	e.streamMu.Lock()
	defer e.streamMu.Unlock()

	if !e.streamActive || e.stream == nil {
		applog.Info("Engine Core: Stream not active or already stopped.")
		return nil
	}

	applog.Info("Engine Core: Stopping PortAudio stream...")
	if e.udpPublisher != nil {
		applog.Info("Engine Core: Stopping UDP publisher...")
		if err := e.udpPublisher.Stop(); err != nil {
			applog.Errorf("Engine Core: Error stopping UDP publisher: %v", err)
		}
	}

	err := e.stream.Stop()
	if err != nil {
		applog.Errorf("Engine Core: Error stopping PortAudio stream: %v", err)
	} else {
		applog.Info("Engine Core: PortAudio stream stopped.")
	}

	applog.Info("Engine Core: Closing PortAudio stream...")
	closeErr := e.stream.Close()
	if closeErr != nil {
		applog.Errorf("Engine Core: Error closing PortAudio stream: %v", closeErr)
		if err == nil {
			err = closeErr
		}
	} else {
		applog.Info("Engine Core: PortAudio stream closed.")
	}

	e.stream = nil
	e.streamActive = false

	return err
}

// TODO:
// Document this function.
func (e *Engine) Close() error {
	applog.Info("Engine Core: Closing...")
	var firstErr error

	applog.Info("Engine Core: Stopping input stream...")
	if err := e.StopInputStream(); err != nil {
		applog.Errorf("Engine Core: Error during StopInputStream: %v", err)
		firstErr = err
	}

	applog.Infof("Engine Core: Closing %d registered closables...", len(e.closables))
	for i := len(e.closables) - 1; i >= 0; i-- {
		closable := e.closables[i]
		applog.Debugf("Engine Core: Closing %T...", closable)
		if err := closable.Close(); err != nil {
			applog.Errorf("Engine Core: Error closing %T: %v", closable, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	applog.Info("Engine Core: Close sequence finished.")

	applog.Info("Terminating PortAudio.")
	if err := portaudio.Terminate(); err != nil {
		applog.Errorf("Engine Core: Error terminating PortAudio: %v", err)
		if firstErr == nil {
			firstErr = err
		}
	} else {
		applog.Info("PortAudio terminated.")
	}

	return firstErr
}
