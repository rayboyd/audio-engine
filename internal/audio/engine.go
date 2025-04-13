package audio

import (
	"audio/internal/config"
	"audio/internal/fft"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/gordonklaus/portaudio"
)

// Engine implements a real-time audio processing pipeline with the following features:
// - Lock-free audio capture using PortAudio
// - Real-time FFT analysis with configurable bands
// - Noise gate with branchless implementation
// - WAV recording with atomic state management
//
// The processing chain consists of three main phases:
// 1. Audio Capture: Input device handling and buffer management
// 2. Signal Processing: Noise gate and FFT analysis
// 3. Output Handling: WAV recording and visualization
//
// Thread Safety:
// - Uses atomic operations for state management
// - Pre-allocates buffers to avoid GC in hot path
// - Locks OS thread during audio processing
type Engine struct {
	// Core configuration and state
	config     *config.Config
	frameCount int64 // Atomic frame count for tracking processed frames

	// Audio input handling
	inputBuffer  []int32               // Pre-allocated buffer for incoming audio
	inputDevice  *portaudio.DeviceInfo // PortAudio device information
	inputLatency time.Duration         // Input device latency setting
	inputStream  *portaudio.Stream     // Active PortAudio stream

	// FFT processing for real-time analysis
	fftProcessor *fft.Processor // Handles FFT computation and visualization
	fftMonoInput []int32        // Mono input buffer for FFT processing

	// Noise gate for signal conditioning
	gateEnabled   bool  // Gate state (true=enabled)
	gateThreshold int32 // Absolute amplitude threshold (0-2147483647)

	// Recording state and buffers
	isRecording int32            // Atomic flag for thread-safe state
	outputFile  *os.File         // Active recording file handle
	wavEncoder  *wav.Encoder     // WAV file encoder
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

	// Pre-allocate mono input buffer for FFT processing, we don't need a stereo
	// buffer for FFT processing, just the first channel. This is a performance
	// optimization to avoid unnecessary allocations in the hot path.
	fftMonoInput := make([]int32, config.FramesPerBuffer)

	// Pre-allocate I/O buffers which are the size of the frames per buffer
	// multiplied by the number of channels. A frame is a set of samples that
	// occur simultaneously. For a stereo stream, a frame is two samples. The
	// buffer size must be a power of 2 and greater than 0 (this was verified
	// defaulted in the ValidateAndDefault function).
	inputSize := config.FramesPerBuffer * config.Channels

	engine = &Engine{
		config:        config,
		inputBuffer:   make([]int32, inputSize),
		inputDevice:   inputDevice,
		fftProcessor:  fftProcessor,
		fftMonoInput:  fftMonoInput,
		gateEnabled:   true,              // Enable gate by default
		gateThreshold: 2147483647 / 1000, // Default to ~0.1% of max value
	}

	// Latency is the time it takes for the audio to travel from the input
	// device to the output device. The lower the latency, the faster the
	// audio will be processed but the higher the CPU usage. This is useful
	// for real-time applications like dsp audio processing.
	if engine.config.LowLatency {
		engine.inputLatency = engine.inputDevice.DefaultLowInputLatency
	} else {
		engine.inputLatency = engine.inputDevice.DefaultHighInputLatency
	}

	return engine, nil
}

// StartInputStream initializes and starts the PortAudio input stream.
func (e *Engine) StartInputStream() error {
	params := portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Channels: e.config.Channels,
			Device:   e.inputDevice,
			Latency:  e.inputLatency,
		},
		Output: portaudio.StreamDeviceParameters{
			Channels: 0,   // No output device
			Device:   nil, // No output device
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

// StopInputStream stops and closes the PortAudio input stream, releasing
// any resources associated with it. This is a blocking call and should
// be used to clean up the input stream when it is no longer needed.
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
// Called by PortAudio when new audio data is available.
//
// Performance Critical:
// - Runs in a dedicated OS thread (LockOSThread)
// - Uses pre-allocated buffers only
// - No dynamic allocations in the hot path
// - Minimal branching in signal processing
//
// Processing Chain:
// 1. Copy input to pre-allocated buffer
// 2. Apply DSP operations (noise gate, etc)
// 3. Perform FFT analysis if enabled
// 4. Record to WAV if enabled
func (e *Engine) processInputStream(in []int32) {
	// Lock this goroutine to the current OS thread to prevent migration
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// --- process audio buffer

	// Copy input to our pre-allocated buffer
	// Using copy() here to avoid any allocations
	copy(e.inputBuffer, in)

	// Process buffer in-place
	e.processBuffer(e.inputBuffer)

	// --- process audio buffer

	// Write to WAV file if recording
	if atomic.LoadInt32(&e.isRecording) == 1 && e.wavEncoder != nil {
		// Convert int32 samples to int for go-audio
		for i, sample := range e.inputBuffer {
			// Convert from int32 to int (may need scaling depending on your setup)
			e.sampleBuf.Data[i] = int(sample)
		}

		// Set the buffer length to match actual data
		e.sampleBuf.Data = e.sampleBuf.Data[:len(e.inputBuffer)]

		// Write to WAV file
		if err := e.wavEncoder.Write(e.sampleBuf); err != nil {
			// Log error but continue - don't disrupt audio processing
			log.Printf("Error writing to WAV file: %v", err)
		}
	}
}

// processBuffer performs all DSP operations on the audio buffer in-place.
//
// Performance Critical (Hot Path):
// - No allocations
// - Branchless noise gate implementation
// - Single conditional for gate threshold
// - Direct FFT processing call
//
// Buffer Format:
// - int32 samples in range [-2147483648, 2147483647]
// - Interleaved channels if stereo
// - Length = FramesPerBuffer * Channels
func (e *Engine) processBuffer(buffer []int32) {
	// Hot path. The hottest path in the entire application,
	// literally the next candidate for Americas Top Model.

	// Determine if FFT processing should occur based on gate
	shouldProcessFFT := false
	if e.gateEnabled {
		var maxAmplitude int32
		for i := 0; i < len(buffer); i++ {
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
		// Gate disabled, always process if processor exists
		shouldProcessFFT = (e.fftProcessor != nil)
	}

	// Process FFT if needed
	if shouldProcessFFT && e.fftProcessor != nil {
		// Prepare the correct buffer for the FFT processor (length = FramesPerBuffer)
		var fftInputBuffer []int32
		if e.config.Channels == 1 {
			// If mono, pass the buffer directly (it's already the correct size)
			fftInputBuffer = buffer
		} else {
			// If stereo (or more channels), extract the first channel into the pre-allocated mono buffer
			for i := 0; i < e.config.FramesPerBuffer; i++ {
				// Check bounds just in case buffer length is unexpected
				if i*e.config.Channels < len(buffer) {
					e.fftMonoInput[i] = buffer[i*e.config.Channels] // Copy sample from the first channel
				} else {
					e.fftMonoInput[i] = 0 // Safety fallback
				}
			}
			fftInputBuffer = e.fftMonoInput // Use the prepared mono buffer
		}

		// Call FFT processor with the correctly sized buffer
		e.fftProcessor.Process(fftInputBuffer)
	}
}

// Gate Operations

// The noise gate helps eliminate background noise by silencing
// audio below a certain amplitude threshold. Uses a branchless
// implementation for maximum performance in the hot path.

// EnableGate activates the noise gate processing.
// Thread-safe: Can be called from any goroutine.
func (e *Engine) EnableGate() {
	e.gateEnabled = true
}

// DisableGate deactivates the noise gate processing.
// When disabled, all audio passes through unmodified.
//
// Thread Safety:
// - Can be called from any goroutine
// - Simple boolean flag, no synchronization needed
func (e *Engine) DisableGate() {
	e.gateEnabled = false
}

// SetGateThreshold adjusts the noise gate threshold.
// Parameters:
// - threshold: Value between 0.0 and 1.0, where:
//   - 0.0 = Gate always open (passes all audio)
//   - 1.0 = Gate always closed (silences all audio)
//
// Thread-safe: Can be called from any goroutine.
func (e *Engine) SetGateThreshold(threshold float64) {
	if threshold < 0.0 {
		threshold = 0.0
	}
	if threshold > 1.0 {
		threshold = 1.0
	}

	// Convert from percentage to absolute value
	// int32 max value is 2147483647
	e.gateThreshold = int32(threshold * float64(math.MaxInt32)) // what is the hit with main.MaxInt32?
}

// GetGateThreshold returns the current threshold as a percentage (0.0-1.0)
func (e *Engine) GetGateThreshold() float64 {
	return float64(e.gateThreshold) / float64(math.MaxInt32) // what is the hit with main.MaxInt32?
}

// Recording Operations

// WAV recording is implemented with atomic state management and
// pre-allocated buffers to maintain real-time performance while
// writing to disk.

// StartRecording begins capturing audio to a WAV file.
// Thread-safe: Uses atomic operations for state management.
// Parameters:
// - filename: Path to the output WAV file
func (e *Engine) StartRecording(filename string) error {
	// Don't start if already recording
	if atomic.LoadInt32(&e.isRecording) == 1 {
		return fmt.Errorf("already recording")
	}

	// Create output file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	e.outputFile = file

	// Create WAV encoder
	e.wavEncoder = wav.NewEncoder(file, int(e.config.SampleRate),
		32, e.config.Channels, 1)

	// Create reusable sample buffer for writing frames
	e.sampleBuf = &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: e.config.Channels,
			SampleRate:  int(e.config.SampleRate),
		},
		Data: make([]int, e.config.FramesPerBuffer*e.config.Channels),
	}

	// Set recording flag
	atomic.StoreInt32(&e.isRecording, 1)

	return nil
}

// StopRecording safely stops the current recording session and finalizes the WAV file.
// This method is thread-safe and idempotent - it can be called multiple times safely.
//
// Thread Safety:
// - Uses atomic operations to coordinate recording state
// - Safe to call from any goroutine
// - Handles cleanup of file resources
func (e *Engine) StopRecording() error {
	// Check if we're recording
	if atomic.LoadInt32(&e.isRecording) == 0 {
		return nil // Not recording, nothing to do
	}

	// Reset the recording flag first to stop processing new frames
	atomic.StoreInt32(&e.isRecording, 0)

	// Clean up WAV encoder and file
	if e.wavEncoder != nil {
		// Close encoder (finalizes WAV header)
		if err := e.wavEncoder.Close(); err != nil {
			return fmt.Errorf("failed to close WAV encoder: %w", err)
		}
		e.wavEncoder = nil
	}

	if e.outputFile != nil {
		if err := e.outputFile.Close(); err != nil {
			return fmt.Errorf("failed to close output file: %w", err)
		}
		e.outputFile = nil
	}

	return nil
}

// Close performs a clean shutdown of the Engine.
// This includes:
// - Stopping any active recording
// - Closing the input stream
// - Cleaning up PortAudio resources
// - Finalizing any open files
//
// This should be called when the Engine is no longer needed to prevent resource leaks.
// After Close() is called, the Engine cannot be reused - create a new instance instead.
func (e *Engine) Close() error {
	// Stop recording if active
	if atomic.LoadInt32(&e.isRecording) == 1 {
		if err := e.StopRecording(); err != nil {
			return err
		}
	}

	// Stop input stream
	if err := e.StopInputStream(); err != nil {
		return err
	}

	return nil
}
