// SPDX-License-Identifier: MIT
package audio

import (
	"audio/internal/analysis"
	"audio/internal/config"
	applog "audio/internal/log"
	udpTransport "audio/internal/transport/udp"
	"fmt"
	"runtime" // Required for LockOSThread
	"sync"
	"time"

	"github.com/gordonklaus/portaudio"
)

// Engine manages the audio input stream, processing pipeline, and data transport.
// It orchestrates the flow of audio data from the input device, through registered
// AudioProcessors, and potentially out via transport mechanisms like UDP.
// It handles the lifecycle of the audio stream and associated components.
type Engine struct {
	config       *config.Config               // Application configuration.
	stream       *portaudio.Stream            // The active PortAudio stream.
	inputDevice  *portaudio.DeviceInfo        // Information about the selected input device.
	inputLatency time.Duration                // Configured input latency for the stream.
	processors   []analysis.AudioProcessor    // Slice of processors to apply to the audio data.
	closables    []interface{ Close() error } // Components needing graceful shutdown (processors, transports).
	streamActive bool                         // Flag indicating if the audio stream is currently running.
	streamMu     sync.Mutex                   // Mutex protecting stream and streamActive state.

	// Transport components (optional, based on config)
	udpSender    *udpTransport.UDPSender    // UDP sender instance (if enabled).
	udpPublisher *udpTransport.UDPPublisher // UDP publisher instance (if enabled).
}

// NewEngine creates and initializes a new audio Engine based on the provided configuration.
// It selects the input device, determines latency, sets up audio processors (like FFT),
// and initializes transport mechanisms (like UDP) if configured.
// Note: PortAudio library itself must be initialized (portaudio.Initialize()) before calling NewEngine.
func NewEngine(config *config.Config) (*Engine, error) {
	applog.Infof("Engine Core: Initializing...")

	// --- 1. Select Input Device ---

	inputDevice, err := InputDevice(config.Audio.InputDevice)
	if err != nil {
		// Log error before returning
		applog.Errorf("Engine Core: Failed to get input device (ID: %d): %v", config.Audio.InputDevice, err)
		return nil, fmt.Errorf("engine: failed to get input device: %w", err)
	}
	applog.Infof("Engine Core: Selected input device: [%d] %s", inputDevice.Index, inputDevice.Name)

	// --- 2. Determine Latency ---

	var latency time.Duration
	if config.Audio.LowLatency {
		latency = inputDevice.DefaultLowInputLatency
		applog.Infof("Engine Core: Using low input latency: %s", latency)
	} else {
		latency = inputDevice.DefaultHighInputLatency
		applog.Infof("Engine Core: Using high input latency: %s", latency)
	}

	// --- 3. Create Engine Instance ---

	engine := &Engine{
		config:       config,
		inputDevice:  inputDevice,
		inputLatency: latency,
		processors:   make([]analysis.AudioProcessor, 0),
		closables:    make([]interface{ Close() error }, 0),
		// stream, streamActive, streamMu, udpSender, udpPublisher initialized later or zero-value ready
	}

	// --- 4. Setup Processors ---

	applog.Infof("Engine Core: Setting up audio processors...")
	// Parse FFT window function name from config
	fftWindowFunc, err := analysis.ParseWindowFunc(engine.config.Audio.FFTWindow)
	if err != nil {
		// Log warning but continue with default (Hann)
		applog.Warnf("Engine Core: %v. Using default FFT window (Hann).", err)
	}

	// Create FFT Processor (assuming it's always needed if UDP is enabled, adjust if needed)
	fftProcessor, err := analysis.NewFFTProcessor(
		engine.config.Audio.FramesPerBuffer, // Use config field
		engine.config.Audio.SampleRate,      // Use config field
		fftWindowFunc,
	)
	if err != nil {
		// Log error before returning
		applog.Errorf("Engine Core: Failed to create FFT processor: %v", err)
		return nil, fmt.Errorf("engine: failed to create FFT processor: %w", err)
	}
	engine.RegisterProcessor(fftProcessor) // Register adds to processors and closables

	// --- 5. Setup Transport ---

	if config.Transport.UDPEnabled {
		applog.Infof("Engine Core: Setting up UDP transport...")
		// Create UDP Sender
		sender, err := udpTransport.NewUDPSender(config.Transport.UDPTargetAddress, config.Debug)
		if err != nil {
			applog.Errorf("Engine Core: Failed to create UDP sender: %v", err)
			engine.Close() // Attempt to clean up already registered processors
			return nil, fmt.Errorf("engine: failed to create UDP sender: %w", err)
		}
		engine.udpSender = sender
		engine.closables = append(engine.closables, sender) // Add sender for graceful shutdown

		// Create UDP Publisher, linking it to the sender and FFT processor
		publisher, err := udpTransport.NewUDPPublisher(
			config.Transport.UDPSendInterval,
			sender,
			fftProcessor, // Provide the FFT processor instance
		)
		if err != nil {
			applog.Errorf("Engine Core: Failed to create UDP publisher: %v", err)
			engine.Close() // Attempt to clean up sender and processors
			return nil, fmt.Errorf("engine: failed to create UDP publisher: %w", err)
		}
		engine.udpPublisher = publisher
		engine.closables = append(engine.closables, publisher) // Add publisher for graceful shutdown

		applog.Infof("Engine Core: UDP transport initialized (Target: %s, Interval: %s)",
			config.Transport.UDPTargetAddress, config.Transport.UDPSendInterval)
	} else {
		applog.Infof("Engine Core: UDP transport is disabled.")
	}

	// --- 6. Log Final Configuration ---

	applog.Infof("Engine Core: Initialized successfully.")
	applog.Debugf("Engine Core: Config - SampleRate=%.1f Hz, BufferSize=%d frames, Channels=%d, Latency=%s",
		config.Audio.SampleRate, config.Audio.FramesPerBuffer, config.Audio.InputChannels, engine.inputLatency)

	return engine, nil
}

// RegisterProcessor adds an AudioProcessor to the engine's processing chain.
// If the processor implements the io.Closer interface, it's also added to the
// list of closables for graceful shutdown during Engine.Close().
func (e *Engine) RegisterProcessor(processor analysis.AudioProcessor) {
	e.processors = append(e.processors, processor)

	// Check if the processor needs closing
	if closable, ok := processor.(interface{ Close() error }); ok {
		e.closables = append(e.closables, closable)
		applog.Debugf("Engine Core: Registered closable processor: %T", processor)
	} else {
		applog.Debugf("Engine Core: Registered processor: %T", processor)
	}
}

// processInputStream is the callback function passed to PortAudio.
// It's executed by PortAudio's audio thread whenever a new buffer of input audio data is available.
// IMPORTANT: This is a real-time audio callback (HOT PATH).
// Avoid allocations, blocking operations, and excessive logging within this function.
// It locks the OS thread to improve real-time performance guarantees.
func (e *Engine) processInputStream(in []int32) {
	// Lock the OS thread. This is crucial for real-time audio callbacks
	// to prevent the Go runtime scheduler from preempting the audio processing.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread() // Ensure unlock happens even if a processor panics

	// Iterate through registered processors and pass the input buffer.
	// Note: Reading e.processors here is generally safe if processors are only
	// added via RegisterProcessor *before* the stream starts. If processors could
	// be added/removed concurrently while the stream is active, a read lock
	// (e.streamMu.RLock/RUnlock) around this loop would be necessary.
	for _, processor := range e.processors {
		processor.Process(in)
	}
}

// StartInputStream opens and starts the PortAudio input stream using the configured
// device, sample rate, buffer size, and latency. It also starts any associated
// components like the UDP publisher if enabled.
// It is safe to call multiple times; subsequent calls are no-ops if the stream is already active.
func (e *Engine) StartInputStream() error {
	e.streamMu.Lock() // Lock to protect stream state
	defer e.streamMu.Unlock()

	// Check if already active
	if e.streamActive {
		applog.Warnf("Engine Core: StartInputStream called but stream already active.")
		return nil
	}

	// --- 1. Define Stream Parameters ---

	streamParameters := portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   e.inputDevice,
			Channels: e.config.Audio.InputChannels,
			Latency:  e.inputLatency,
		},
		SampleRate:      e.config.Audio.SampleRate,
		FramesPerBuffer: e.config.Audio.FramesPerBuffer,
	}
	applog.Infof("Engine Core: Configuring PortAudio stream: SR=%.1f, Buf=%d, Lat=%s, Ch=%d",
		streamParameters.SampleRate, streamParameters.FramesPerBuffer, streamParameters.Input.Latency, streamParameters.Input.Channels)

	// --- 2. Open PortAudio Stream ---

	applog.Infof("Engine Core: Opening PortAudio stream...")
	stream, err := portaudio.OpenStream(streamParameters, e.processInputStream) // Pass the callback
	if err != nil {
		applog.Errorf("Engine Core: Failed to open PortAudio stream: %v", err)
		return fmt.Errorf("engine: failed to open PortAudio stream: %w", err)
	}
	e.stream = stream // Store the stream instance

	// --- 3. Start PortAudio Stream ---

	applog.Infof("Engine Core: Starting PortAudio stream...")
	if err := e.stream.Start(); err != nil {
		applog.Errorf("Engine Core: Failed to start PortAudio stream: %v", err)
		// Attempt to close the stream if starting failed
		_ = e.stream.Close() // Ignore close error here as start error is primary
		e.stream = nil
		return fmt.Errorf("engine: failed to start PortAudio stream: %w", err)
	}
	e.streamActive = true // Mark stream as active
	applog.Infof("Engine Core: PortAudio stream started successfully.")

	// --- 4. Start Associated Components ---

	if e.udpPublisher != nil {
		applog.Infof("Engine Core: Starting UDP publisher...")
		e.udpPublisher.Start() // UDPPublisher logs its own start messages
	}

	return nil
}

// StopInputStream stops the active PortAudio input stream and associated components
// like the UDP publisher. It attempts to stop components gracefully.
// It is safe to call multiple times; subsequent calls are no-ops if the stream is not active.
func (e *Engine) StopInputStream() error {
	e.streamMu.Lock() // Lock to protect stream state
	defer e.streamMu.Unlock()

	// Check if already stopped or never started
	if !e.streamActive || e.stream == nil {
		applog.Infof("Engine Core: StopInputStream called but stream not active or already stopped.")
		return nil
	}

	applog.Infof("Engine Core: Stopping input stream...")
	var firstErr error // Track the first error encountered

	// --- 1. Stop Associated Components First ---

	if e.udpPublisher != nil {
		applog.Infof("Engine Core: Stopping UDP publisher...")
		if err := e.udpPublisher.Stop(); err != nil {
			applog.Errorf("Engine Core: Error stopping UDP publisher: %v", err)
			firstErr = err // Record the first error
		}
	}

	// --- 2. Stop PortAudio Stream ---

	applog.Infof("Engine Core: Stopping PortAudio stream...")
	err := e.stream.Stop()
	if err != nil {
		applog.Errorf("Engine Core: Error stopping PortAudio stream: %v", err)
		if firstErr == nil {
			firstErr = err // Record if it's the first error
		}
	} else {
		applog.Infof("Engine Core: PortAudio stream stopped.")
	}

	// --- 3. Close PortAudio Stream ---

	applog.Infof("Engine Core: Closing PortAudio stream...")
	closeErr := e.stream.Close()
	if closeErr != nil {
		applog.Errorf("Engine Core: Error closing PortAudio stream: %v", closeErr)
		if firstErr == nil {
			firstErr = closeErr // Keep a reference to the first error
		}
	} else {
		applog.Infof("Engine Core: PortAudio stream closed.")
	}

	// --- 4. Update Engine State ---

	e.stream = nil         // Clear the stream reference
	e.streamActive = false // Mark stream as inactive

	applog.Infof("Engine Core: Input stream stop sequence finished.")
	return firstErr
}

// Close gracefully shuts down the audio engine. It stops the audio stream
// (if active) and then closes all registered closable components (processors, transports)
// in reverse order of registration.
// Note: It does *not* terminate the PortAudio library itself; that should be handled separately.
func (e *Engine) Close() error {
	applog.Infof("Engine Core: Closing engine...")
	var firstErr error

	// --- 1. Stop the Input Stream ---

	// StopInputStream handles locking and checks if the stream is active.
	applog.Infof("Engine Core: Ensuring input stream is stopped...")
	if err := e.StopInputStream(); err != nil {
		// Log error from stopping the stream, but continue closing other components.
		applog.Errorf("Engine Core: Error during StopInputStream on Close: %v", err)
		firstErr = err // Keep a reference to the first error
	}

	// --- 2. Close Registered Closables ---

	applog.Infof("Engine Core: Closing %d registered closable components...", len(e.closables))

	// Close in reverse order of registration (LIFO)
	for i := len(e.closables) - 1; i >= 0; i-- {
		closable := e.closables[i]
		applog.Debugf("Engine Core: Closing component %T...", closable)
		if err := closable.Close(); err != nil {
			applog.Errorf("Engine Core: Error closing component %T: %v", closable, err)
			if firstErr == nil {
				firstErr = err // Keep a reference to the first error
			}
		}
	}

	applog.Infof("Engine Core: Close sequence finished.")
	return firstErr
}
