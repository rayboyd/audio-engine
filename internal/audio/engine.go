// SPDX-License-Identifier: MIT
package audio

import (
	"audio/internal/analysis"
	"audio/internal/config"
	"audio/internal/transport" // Use websocket transport implementation
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/gordonklaus/portaudio"
)

// Engine manages the PortAudio input stream and dispatches buffers.
type Engine struct {
	config *config.Config

	inputDevice  *portaudio.DeviceInfo
	inputLatency time.Duration
	inputStream  *portaudio.Stream

	processors []any
	procMutex  sync.RWMutex

	transport transport.Transport

	// Store specific processor types if needed for dependencies
	fftProvider transport.FFTResultProvider // Store the provider (which will be the FFT processor instance)
}

// NewEngine creates a new audio engine instance.
func NewEngine(config *config.Config) (*Engine, error) {
	inputDevice, err := InputDevice(config.Audio.InputDevice)
	if err != nil {
		return nil, fmt.Errorf("engine: failed to get input device: %w", err)
	}

	var latency time.Duration
	if config.Audio.LowLatency {
		latency = inputDevice.DefaultLowInputLatency
	} else {
		latency = inputDevice.DefaultHighInputLatency
	}

	// --- Use LoggingTransport ---
	// transport := transport.NewLoggingTransport()
	transport := transport.NewWebSocketTransport(":8080")
	// transport := transport.NewWebSocketTransport(config.WebSocket.Address, config.WebSocket.WebRoot)
	// --- End Transport Change ---

	engine := &Engine{
		config:       config,
		inputDevice:  inputDevice,
		inputLatency: latency,
		processors:   make([]any, 0),
		transport:    transport,
	}

	//
	/// Use Engine config from here on in

	// --- Processor Registration ---

	// FFT Processing
	fftProcessor := analysis.NewFFTProcessor(
		engine.config.Audio.FramesPerBuffer,
		engine.config.Audio.SampleRate,
		engine.transport,
		analysis.BartlettHann,
	)
	engine.RegisterProcessor(fftProcessor)
	engine.fftProvider = fftProcessor

	// Band Energy Processing
	bandEnergyProcessor := analysis.NewBandEnergyProcessor(
		engine.transport,
		engine.fftProvider,
	)
	engine.RegisterProcessor(bandEnergyProcessor)

	// Beat Detection
	beatProcessor := analysis.NewBeatDetector(
		0.2,
		2.0,
		engine.config.Audio.SampleRate,
		engine.config.Audio.FramesPerBuffer,
		engine.transport,
	)
	engine.RegisterProcessor(beatProcessor)

	// --- End Processor Registration ---

	// log.Printf("Engine Core: Initialized with %d channels, %d frames/buffer, %.2f Hz sample rate.",
	// 	config.Channels, config.FramesPerBuffer, config.SampleRate)
	// log.Printf("Engine Core: Using input device '%s' with latency %v.", inputDevice.Name, engine.inputLatency)
	// log.Printf("Engine Core: Using FFT window function: %s", configWindowName)

	return engine, nil
}

// RegisterProcessor adds a processor to the engine's list.
func (e *Engine) RegisterProcessor(p any) {
	if p == nil {
		log.Println("Engine Core: Attempted to register a nil processor.")
		return
	}
	e.procMutex.Lock()
	defer e.procMutex.Unlock()
	log.Printf("Engine Core: Registering processor %T", p)
	e.processors = append(e.processors, p)
}

// processInputStream is the PortAudio callback (HOTPATH).
func (e *Engine) processInputStream(in []int32) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// 1. Process FFT first
	fftProcessed := false
	if e.fftProvider != nil {
		// Check if the stored provider implements the raw audio Processor interface
		if fftProc, ok := e.fftProvider.(transport.Processor); ok {
			// log.Println("DEBUG: Calling FFT Process") // Add temporary debug log
			fftProc.Process(in) // This calculates FFT and should call Send internally
			fftProcessed = true
			// log.Println("DEBUG: FFT Process finished") // Add temporary debug log
		} else {
			log.Printf("Engine Core: ERROR - fftProvider (%T) does not implement transport.Processor!", e.fftProvider)
		}
	} else {
		log.Println("Engine Core: WARNING - fftProvider is nil in processInputStream.")
	}

	// 2. Process other processors
	e.procMutex.RLock()
	processorsToProcess := make([]interface{}, len(e.processors))
	copy(processorsToProcess, e.processors)
	e.procMutex.RUnlock()

	for _, proc := range processorsToProcess {
		switch p := proc.(type) {
		case *analysis.FFTProcessor: // FFT Processor concrete type
			// Already processed above via e.fftProvider, do nothing here.
			// log.Println("DEBUG: Skipping FFT in loop") // Add temporary debug log
		case *analysis.BeatDetector:
			// BeatDetector needs the raw audio buffer
			// log.Println("DEBUG: Calling BeatDetector Process") // Add temporary debug log
			p.Process(in) // Sends beat events via its internal transport
			// log.Println("DEBUG: BeatDetector Process finished") // Add temporary debug log
		case *analysis.BandEnergyProcessor:
			// BandEnergyProcessor depends on FFT results having been calculated
			if fftProcessed {
				// log.Println("DEBUG: Calling BandEnergy Process") // Add temporary debug log
				p.Process() // Uses data from fftProvider internally and sends band energy
				// log.Println("DEBUG: BandEnergy Process finished") // Add temporary debug log
			} else {
				// log.Println("DEBUG: Skipping BandEnergy as FFT not processed") // Add temporary debug log
			}
		default:
			log.Printf("Engine Core: WARNING - Unknown processor type in loop: %T", p)
		}
	}
	// log.Println("DEBUG: processInputStream finished cycle") // Add temporary debug log
}

// StartInputStream opens and starts the PortAudio stream.
func (e *Engine) StartInputStream() error {
	e.procMutex.RLock()
	numProc := len(e.processors)
	e.procMutex.RUnlock()
	if numProc == 0 {
		log.Println("Engine Core: Warning - Starting stream with no registered processors.")
	}

	params := portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Channels: e.config.Audio.InputChannels,
			Device:   e.inputDevice,
			Latency:  e.inputLatency,
		},
		Output: portaudio.StreamDeviceParameters{
			Channels: 0, Device: nil,
		},
		FramesPerBuffer: e.config.Audio.FramesPerBuffer,
		SampleRate:      e.config.Audio.SampleRate,
	}

	log.Printf("Engine Core: Opening stream with params: %+v", params.Input)
	stream, err := portaudio.OpenStream(params, e.processInputStream)
	if err != nil {
		return fmt.Errorf("engine core: failed to open stream: %w", err)
	}
	e.inputStream = stream
	log.Println("Engine Core: Stream opened.")

	log.Println("Engine Core: Starting stream...")
	if err := e.inputStream.Start(); err != nil {
		if closeErr := e.inputStream.Close(); closeErr != nil {
			log.Printf("Engine Core: Failed to close stream after start failure: %v", closeErr)
		}
		e.inputStream = nil
		return fmt.Errorf("engine core: failed to start stream: %w", err)
	}
	log.Println("Engine Core: Stream started successfully.")
	return nil
}

// StopInputStream stops and closes the PortAudio stream.
func (e *Engine) StopInputStream() error {
	log.Println("Engine Core: Stopping input stream...")
	if e.inputStream == nil {
		log.Println("Engine Core: Input stream was already nil.")
		return nil
	}

	var firstErr error
	log.Println("Engine Core: Stopping PortAudio stream...")
	if err := e.inputStream.Stop(); err != nil {
		log.Printf("Engine Core: Error stopping PortAudio stream: %v", err)
		firstErr = fmt.Errorf("engine core: error stopping stream: %w", err)
	} else {
		log.Println("Engine Core: PortAudio stream stopped.")
	}

	log.Println("Engine Core: Closing PortAudio stream...")
	if err := e.inputStream.Close(); err != nil {
		log.Printf("Engine Core: Error closing PortAudio stream: %v", err)
		if firstErr == nil {
			firstErr = fmt.Errorf("engine core: error closing stream: %w", err)
		}
	} else {
		log.Println("Engine Core: PortAudio stream closed.")
	}

	e.inputStream = nil
	return firstErr
}

// Close stops the input stream and potentially cleans up processors.
func (e *Engine) Close() error {
	log.Println("Engine Core: Closing...")
	streamErr := e.StopInputStream()

	e.procMutex.RLock()
	processorsToClose := make([]interface{}, len(e.processors))
	copy(processorsToClose, e.processors)
	e.procMutex.RUnlock()

	var firstProcErr error
	for _, p := range processorsToClose {
		if closer, ok := p.(interface{ Close() error }); ok {
			log.Printf("Engine Core: Closing processor %T", p)
			if err := closer.Close(); err != nil {
				log.Printf("Engine Core: Error closing processor %T: %v", p, err)
				if firstProcErr == nil {
					firstProcErr = err
				}
			}
		}
	}

	var transportErr error
	if e.transport != nil {
		if closer, ok := e.transport.(interface{ Close() error }); ok {
			log.Println("Engine Core: Closing transport...")
			if err := closer.Close(); err != nil {
				log.Printf("Engine Core: Error closing transport: %v", err)
				transportErr = err
			}
		}
	}

	log.Println("Engine Core: Close sequence finished.")
	if streamErr != nil {
		return streamErr
	}
	if transportErr != nil {
		return transportErr
	}
	return firstProcErr
}
