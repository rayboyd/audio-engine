package config

// Core configuration constants that define the boundaries and defaults
// for the audio processing engine.
const (
	// Default values for the audio engine configuration
	DefaultChannels          = 1           // Mono audio
	DefaultDeviceID          = MinDeviceID // Default to system default device
	DefaultFormat            = "wav"       // WAV file format for recordings
	DefaultFramesPerBuffer   = 512         // Balanced latency/performance
	DefaultLowLatency        = false       // Standard latency mode
	DefaultSampleRate        = 44100       // CD-quality audio
	DefaultRecordInputStream = false       // Don't record by default
	DefaultOutputFile        = ""          // Auto-generated filename
	DefaultCommand           = ""          // No command by default
	DefaultVerbosity         = false       // Quiet operation

	// Hardware and processing limits
	MinDeviceID     = -1     // -1 represents system default device
	MinSampleRate   = 8000   // Minimum usable sample rate (Hz)
	MaxSampleRate   = 192000 // Maximum supported sample rate (Hz)
	MaxBufferFrames = 8192   // Maximum frames per buffer (power of 2)

	// Error handling configuration
	DefaultMaxConsecutiveWriteFailures = 5 // Max failures before stopping
)

// Config holds all runtime configuration options for the audio engine.
// It is constructed via command line flags and/or configuration files.
type Config struct {
	// Audio Device Settings
	Channels        int     // Number of audio channels (1=mono, 2=stereo)
	DeviceID        int     // Input device identifier
	Format          string  // Recording format (wav only for now)
	FramesPerBuffer int     // Buffer size in frames
	LowLatency      bool    // Use low latency mode
	SampleRate      float64 // Sample rate in Hz

	// Recording Options
	RecordInputStream bool   // Whether to record input
	OutputFile        string // Output file path for recordings

	// Debug Options
	Verbose bool   // Enable verbose logging
	Command string // One-off command to execute
	TUIMode bool   // Terminal UI mode enabled

	// FFT Analysis Settings
	FFTBands int // Number of frequency bands
}

// NewConfig creates a new Config instance with default values.
// This is typically used as the base configuration before
// applying command line arguments or config file settings.
func NewConfig() *Config {
	return &Config{
		Channels:          DefaultChannels,
		DeviceID:          DefaultDeviceID,
		Format:            DefaultFormat,
		FramesPerBuffer:   DefaultFramesPerBuffer,
		LowLatency:        DefaultLowLatency,
		SampleRate:        DefaultSampleRate,
		RecordInputStream: DefaultRecordInputStream,
		OutputFile:        DefaultOutputFile,
		Command:           DefaultCommand,
		Verbose:           DefaultVerbosity,
	}
}
