// SPDX-License-Identifier: MIT
package audio

import (
	"audio/internal/analysis"
	"audio/internal/config"
	udpTransport "audio/internal/transport/udp"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/gordonklaus/portaudio"
)

// Engine manages the audio input stream, processing pipeline, and data transport.
// It orchestrates the flow of audio data from the input device, through registered
// AudioProcessors, and potentially out via transport mechanisms like UDP.
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
// It selects the input device, determines latency, sets up audio processors (like FFT), and
// initializes transport mechanisms (like UDP) if configured. PortAudio must be initialized
// before calling NewEngine.
func NewEngine(config *config.Config) (*Engine, error) {
	// --- 1. Select Input Device ---

	inputDevice, err := InputDevice(config.Audio.InputDevice)
	if err != nil {
		return nil, fmt.Errorf("engine: failed to get input device: %w", err)
	}

	// --- 2. Determine Latency ---

	var latency time.Duration
	if config.Audio.LowLatency {
		latency = inputDevice.DefaultLowInputLatency
	} else {
		latency = inputDevice.DefaultHighInputLatency
	}

	// --- 3. Create Engine Instance ---

	engine := &Engine{
		config:       config,
		inputDevice:  inputDevice,
		inputLatency: latency,
		processors:   make([]analysis.AudioProcessor, 0),
		closables:    make([]interface{ Close() error }, 0),
		// stream, streamActive, streamMu, udpSender, udpPublisher initialized later or zero-value ready.
	}

	// --- 4. Setup Processors ---

	fftWindowFunc, err := analysis.ParseWindowFunc(engine.config.Audio.FFTWindow)
	if err != nil {
		fmt.Printf("engine: %v. Using default FFT window (Hann).\n", err)
	}

	// Create FFT Processor (assuming it's always needed if UDP is enabled, adjust if needed).
	fftProcessor, err := analysis.NewFFTProcessor(
		engine.config.Audio.FramesPerBuffer,
		engine.config.Audio.SampleRate,
		fftWindowFunc,
	)
	if err != nil {
		return nil, fmt.Errorf("engine: failed to create FFT processor: %w", err)
	}
	engine.RegisterProcessor(fftProcessor)

	// --- 5. Setup Transport ---

	if config.Transport.UDPEnabled {
		// Create the UDP sender.
		sender, err := udpTransport.NewUDPSender(config.Transport.UDPTargetAddress, config.Debug)
		if err != nil {
			engine.Close() // Attempt to clean up already registered processors.
			return nil, fmt.Errorf("engine: failed to create UDP sender: %w", err)
		}
		engine.udpSender = sender
		engine.closables = append(engine.closables, sender)

		// Create the UDP Publisher, linking it to the sender and FFT processor.
		publisher, err := udpTransport.NewUDPPublisher(
			config.Transport.UDPSendInterval,
			sender,
			fftProcessor,
		)
		if err != nil {
			engine.Close() // Attempt to clean up sender and processors
			return nil, fmt.Errorf("engine: failed to create UDP publisher: %w", err)
		}
		engine.udpPublisher = publisher
		engine.closables = append(engine.closables, publisher)

		fmt.Printf("engine: UDP transport initialized (Target: %s, Interval: %s)\n",
			config.Transport.UDPTargetAddress, config.Transport.UDPSendInterval)
	} else {
		fmt.Printf("engine: UDP transport is disabled.\n")
	}

	// --- 6. Log Final Configuration ---

	fmt.Printf("engine: Initialized successfully.\n")
	fmt.Printf("engine: Config - SampleRate=%.1f Hz, BufferSize=%d frames, Channels=%d, Latency=%s\n",
		config.Audio.SampleRate, config.Audio.FramesPerBuffer, config.Audio.InputChannels, engine.inputLatency)

	return engine, nil
}

// RegisterProcessor adds an AudioProcessor to the engine's processing chain.
// If the processor implements the io.Closer interface, it's also added to the
// list of closables for graceful shutdown during Engine.Close().
func (e *Engine) RegisterProcessor(processor analysis.AudioProcessor) {
	e.processors = append(e.processors, processor)

	if closable, ok := processor.(interface{ Close() error }); ok {
		e.closables = append(e.closables, closable)
		fmt.Printf("engine: Registered closable processor: %T\n", processor)
	} else {
		fmt.Printf("engine: Registered processor: %T\n", processor)
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

	if e.streamActive {
		fmt.Printf("engine: StartInputStream called but stream already active.\n")
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
	fmt.Printf("engine: Configuring PortAudio stream: SR=%.1f, Buf=%d, Lat=%s, Ch=%d\n",
		streamParameters.SampleRate, streamParameters.FramesPerBuffer, streamParameters.Input.Latency, streamParameters.Input.Channels)

	// --- 2. Open PortAudio Stream ---

	stream, err := portaudio.OpenStream(streamParameters, e.processInputStream)
	if err != nil {
		return fmt.Errorf("engine: failed to open PortAudio stream: %w", err)
	}
	e.stream = stream

	// --- 3. Start PortAudio Stream ---

	if err := e.stream.Start(); err != nil {
		// Attempt to close the stream if starting failed
		_ = e.stream.Close() // Ignore close error here as start error is primary
		e.stream = nil
		return fmt.Errorf("engine: failed to start PortAudio stream: %w", err)
	}
	e.streamActive = true

	// --- 4. Start Associated Components ---

	if e.udpPublisher != nil {
		e.udpPublisher.Start()
	}

	return nil
}

// StopInputStream stops the active PortAudio input stream and associated components
// like the UDP publisher. It attempts to stop components gracefully. It is safe to
// call multiple times; subsequent calls are no-ops if the stream is not active.
func (e *Engine) StopInputStream() error {
	e.streamMu.Lock()
	defer e.streamMu.Unlock()

	if !e.streamActive || e.stream == nil {
		fmt.Printf("engine: StopInputStream called but stream not active or already stopped.\n")
		return nil
	}

	fmt.Printf("engine: Stopping input stream ...\n")

	// Keep a reference to the first error encountered and continue
	// stopping other components, we can report all errors at the end
	// after attempting to stop everything.
	var firstErr error

	// --- 1. Stop Associated Components First ---

	if e.udpPublisher != nil {
		fmt.Printf("engine: Stopping Stopping UDP publisher ...\n")
		if err := e.udpPublisher.Stop(); err != nil {
			fmt.Printf("engine: Error stopping UDP publisher: %v\n", err)
			firstErr = err
		}
	}

	// --- 2. Stop PortAudio Stream ---

	fmt.Printf("engine: Stopping PortAudio stream ...\n")
	err := e.stream.Stop()
	if err != nil {
		fmt.Printf("engine: Error stopping PortAudio stream: %v\n", err)
		if firstErr == nil {
			firstErr = err
		}
	} else {
		fmt.Printf("engine: PortAudio stream stopped.\n")
	}

	// --- 3. Close PortAudio Stream ---

	fmt.Printf("engine: closing PortAudio stream ...\n")
	closeErr := e.stream.Close()
	if closeErr != nil {
		fmt.Printf("engine: Error closing PortAudio stream: %v\n", closeErr)
		if firstErr == nil {
			firstErr = closeErr
		}
	} else {
		fmt.Printf("engine: PortAudio stream closed.\n")
	}

	// --- 4. Update Engine State ---

	e.stream = nil         // Clear the stream reference
	e.streamActive = false // Mark stream as inactive

	// fmt.Printf("engine: input stream and components stopped.")
	return firstErr
}

// Close gracefully shuts down the audio engine. It stops the audio stream (if active) and then
// closes all registered closable components (processors, transports) in reverse order of registration.
// Note: It does *not* terminate the PortAudio library itself; that should be handled separately.
func (e *Engine) Close() error {
	fmt.Printf("engine: Closing ...\n")
	var firstErr error

	// --- 1. Stop the Input Stream ---

	if err := e.StopInputStream(); err != nil {
		// Log error from stopping the stream, but continue closing other components.
		// fmt.Errorf("Engine Core: Error during StopInputStream on Close: %v\n", err)
		firstErr = err
	}

	// --- 2. Close Registered Closables ---

	// Close in reverse order of registration (LIFO).
	for i := len(e.closables) - 1; i >= 0; i-- {
		closable := e.closables[i]
		fmt.Printf("engine: Closing component %T...\n", closable)
		if err := closable.Close(); err != nil {
			// fmt.Errorf("engine: Error closing component %T: %v", closable, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	fmt.Printf("engine: Close sequence finished.\n")
	return firstErr
}
