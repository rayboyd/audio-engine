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

// Engine manages the PortAudio input stream and dispatches buffers to registered processors.
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

// NewEngine creates and initializes a new audio engine instance.
func NewEngine(config *config.Config) (*Engine, error) {
	if err := portaudio.Initialize(); err != nil {
		// Errorf might be better here than Fatalf, let caller decide fatality
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
		applog.Info("Engine Core: Setting up UDP transport...") // Use Info
		// Pass config.Debug to NewUDPSender
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
		applog.Infof("Engine Core: UDP transport initialized (Target: %s, Interval: %s)", // Use Infof
			config.Transport.UDPTargetAddress, config.Transport.UDPSendInterval)
	} else {
		applog.Info("Engine Core: UDP transport is disabled.") // Use Info
	}

	applog.Infof("Engine Core: Initialized with %d input channels, %d frames/buffer, %.1f Hz sample rate.", // Use Infof
		config.Audio.InputChannels, config.Audio.FramesPerBuffer, config.Audio.SampleRate)
	applog.Infof("Engine Core: Using input device '%s' with latency %v.", inputDevice.Name, engine.inputLatency) // Use Infof

	return engine, nil
}

// RegisterProcessor adds an AudioProcessor to the engine's processing chain.
func (e *Engine) RegisterProcessor(processor analysis.AudioProcessor) {
	e.processors = append(e.processors, processor)
	if closable, ok := processor.(interface{ Close() error }); ok {
		e.closables = append(e.closables, closable)
		applog.Debugf("Engine Core: Registered closable processor %T", processor) // Use Debugf
	} else {
		applog.Debugf("Engine Core: Registered processor %T", processor) // Use Debugf
	}
}

// processInputStream is the PortAudio callback function (HOT PATH).
func (e *Engine) processInputStream(in []int32) {
	// Keep this path clean - NO LOGGING HERE
	for _, processor := range e.processors {
		processor.Process(in)
	}
}

// StartInputStream opens and starts the PortAudio input stream.
func (e *Engine) StartInputStream() error {
	e.streamMu.Lock()
	defer e.streamMu.Unlock()

	if e.streamActive {
		applog.Warn("Engine Core: Stream already active.") // Use Warn
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

	applog.Info("Engine Core: Opening PortAudio stream...") // Use Info
	stream, err := portaudio.OpenStream(streamParameters, e.processInputStream)
	if err != nil {
		return fmt.Errorf("engine: failed to open PortAudio stream: %w", err)
	}
	e.stream = stream

	applog.Info("Engine Core: Starting PortAudio stream...") // Use Info
	if err := e.stream.Start(); err != nil {
		e.stream.Close()
		e.stream = nil
		return fmt.Errorf("engine: failed to start PortAudio stream: %w", err)
	}
	e.streamActive = true
	applog.Info("Engine Core: PortAudio stream started successfully.") // Use Info

	if e.udpPublisher != nil {
		applog.Info("Engine Core: Starting UDP publisher...") // Use Info
		e.udpPublisher.Start()                                // Start itself logs
	}

	return nil
}

// StopInputStream stops and closes the active PortAudio stream.
func (e *Engine) StopInputStream() error {
	e.streamMu.Lock()
	defer e.streamMu.Unlock()

	if !e.streamActive || e.stream == nil {
		applog.Info("Engine Core: Stream not active or already stopped.") // Use Info
		return nil
	}

	applog.Info("Engine Core: Stopping PortAudio stream...") // Use Info
	if e.udpPublisher != nil {
		applog.Info("Engine Core: Stopping UDP publisher...") // Use Info
		if err := e.udpPublisher.Stop(); err != nil {
			applog.Errorf("Engine Core: Error stopping UDP publisher: %v", err) // Use Errorf
		}
	}

	err := e.stream.Stop()
	if err != nil {
		applog.Errorf("Engine Core: Error stopping PortAudio stream: %v", err) // Use Errorf
	} else {
		applog.Info("Engine Core: PortAudio stream stopped.") // Use Info
	}

	applog.Info("Engine Core: Closing PortAudio stream...") // Use Info
	closeErr := e.stream.Close()
	if closeErr != nil {
		applog.Errorf("Engine Core: Error closing PortAudio stream: %v", closeErr) // Use Errorf
		if err == nil {
			err = closeErr
		}
	} else {
		applog.Info("Engine Core: PortAudio stream closed.") // Use Info
	}

	e.stream = nil
	e.streamActive = false

	return err
}

// Close gracefully shuts down the engine.
func (e *Engine) Close() error {
	applog.Info("Engine Core: Closing...") // Use Info
	var firstErr error

	applog.Info("Engine Core: Stopping input stream...") // Use Info
	if err := e.StopInputStream(); err != nil {
		applog.Errorf("Engine Core: Error during StopInputStream: %v", err) // Use Errorf
		firstErr = err
	}

	applog.Infof("Engine Core: Closing %d registered closables...", len(e.closables)) // Use Infof
	for i := len(e.closables) - 1; i >= 0; i-- {
		closable := e.closables[i]
		applog.Debugf("Engine Core: Closing %T...", closable) // Use Debugf
		if err := closable.Close(); err != nil {
			applog.Errorf("Engine Core: Error closing %T: %v", closable, err) // Use Errorf
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	applog.Info("Engine Core: Close sequence finished.") // Use Info

	applog.Info("Terminating PortAudio.") // Use Info
	if err := portaudio.Terminate(); err != nil {
		applog.Errorf("Engine Core: Error terminating PortAudio: %v", err) // Use Errorf
		if firstErr == nil {
			firstErr = err
		}
	} else {
		applog.Info("PortAudio terminated.") // Use Info
	}

	return firstErr
}
