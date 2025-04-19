// SPDX-License-Identifier: MIT
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main application configuration structure, loaded from YAML.
type Config struct {
	Debug     bool            `yaml:"debug"`             // Enable debug mode (verbose logging, potentially other debug features).
	LogLevel  string          `yaml:"log_level"`         // Logging level (e.g., "debug", "info", "warn", "error").
	Command   string          `yaml:"command,omitempty"` // A one-off command to execute instead of running the engine (e.g., "list", "version").
	Audio     AudioConfig     `yaml:"audio"`             // Audio processing settings.
	Recording RecordingConfig `yaml:"recording"`         // Audio recording settings.
	Transport TransportConfig `yaml:"transport"`         // Data transport settings (e.g., UDP).
}

// AudioConfig holds settings related to audio input/output and processing.
type AudioConfig struct {
	InputDevice     int     `yaml:"input_device"`      // PortAudio device index for audio input (-1 for default).
	OutputDevice    int     `yaml:"output_device"`     // PortAudio device index for audio output (-1 for default, currently unused).
	SampleRate      float64 `yaml:"sample_rate"`       // Sample rate in Hz (e.g., 44100, 48000).
	FramesPerBuffer int     `yaml:"frames_per_buffer"` // Number of audio frames per processing buffer (affects latency and FFT resolution).
	LowLatency      bool    `yaml:"low_latency"`       // Request low latency settings from PortAudio device.
	InputChannels   int     `yaml:"input_channels"`    // Number of input channels to capture (e.g., 1 for mono, 2 for stereo).
	OutputChannels  int     `yaml:"output_channels"`   // Number of output channels (currently unused).
	FFTWindow       string  `yaml:"fft_window"`        // Name of the window function for FFT analysis (e.g., "Hann", "Hamming").
}

// RecordingConfig holds settings related to audio recording functionality.
type RecordingConfig struct {
	Enabled     bool    `yaml:"enabled"`              // Enable audio recording to file.
	OutputDir   string  `yaml:"output_dir"`           // Directory to save recorded audio files.
	Format      string  `yaml:"format"`               // File format for recordings (e.g., "wav").
	BitDepth    int     `yaml:"bit_depth"`            // Bit depth for recorded audio (e.g., 16, 24).
	MaxDuration int     `yaml:"max_duration_seconds"` // Maximum duration of a single recording file in seconds (0 for unlimited).
	SilenceTh   float64 `yaml:"silence_threshold"`    // Silence threshold for potential silence detection features (currently unused).
}

// TransportConfig holds settings related to sending processed data over the network.
type TransportConfig struct {
	UDPEnabled       bool          `yaml:"udp_enabled"`        // Enable sending FFT data over UDP.
	UDPTargetAddress string        `yaml:"udp_target_address"` // Target address and port for UDP packets (e.g., "127.0.0.1:9090").
	UDPSendInterval  time.Duration `yaml:"udp_send_interval"`  // Interval between sending UDP packets.
}

// LoadConfig loads configuration from a YAML file specified by path. If path is empty,
// it searches default locations ("config.yaml"). If no file is found, it uses built-in
// defaults.  After loading defaults or from file, it applies environment variable
// overrides and validates the final configuration.
func LoadConfig(path string) (*Config, error) {
	cfg := Config{
		Debug:    false,
		LogLevel: "info",
		Audio: AudioConfig{
			InputDevice:     -1, // -1 for default device.
			OutputDevice:    -1,
			SampleRate:      44100,
			FramesPerBuffer: 1024,
			LowLatency:      false,
			InputChannels:   2,
			OutputChannels:  2,
			FFTWindow:       "Hann",
		},
		Recording: RecordingConfig{
			Enabled:     false,
			OutputDir:   "./recordings",
			Format:      "wav",
			BitDepth:    16,
			MaxDuration: 0, // 0 for unlimited.
			SilenceTh:   0.01,
		},
		Transport: TransportConfig{
			UDPEnabled:       false, // Default UDP to false.
			UDPTargetAddress: "127.0.0.1:9090",
			UDPSendInterval:  33 * time.Millisecond, // Default ~30Hz.
		},
	}

	if path == "" {
		// Define potential locations for the config file.
		candidates := []string{
			"config.yaml",
			// TODO: Add platform-specific paths if desired
			// filepath.Join(os.Getenv("HOME"), ".config/phase4/config.yaml"),
			// "/etc/phase4/config.yaml",
		}
		found := false
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				found = true
				break
			}
		}
		if !found {
			cfg.applyEnvOverrides()
			if err := cfg.Validate(); err != nil {
				// TODO:
				// Preallocate this error message.
				return nil, fmt.Errorf("invalid default configuration: %w", err)
			}
			return &cfg, nil
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		// TODO:
		// Preallocate this error message.
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		// TODO:
		// Preallocate this error message.
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides AFTER loading from file.
	cfg.applyEnvOverrides()

	if err := cfg.Validate(); err != nil {
		// TODO:
		// Preallocate this error message.
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	// Transport Validation
	// if c.Transport.UDPEnabled {
	// 	if c.Transport.UDPTargetAddress == "" {
	// 		return fmt.Errorf("transport.udp_target_address must be set when UDP is enabled")
	// 	}
	// 	if !strings.Contains(c.Transport.UDPTargetAddress, ":") {
	// 		return fmt.Errorf("transport.udp_target_address '%s' appears invalid (missing port?)", c.Transport.UDPTargetAddress)
	// 	}
	// 	if c.Transport.UDPSendInterval <= 0 {
	// 		return fmt.Errorf("transport.udp_send_interval must be positive when UDP is enabled")
	// 	}
	// }

	return nil
}

// TODO:
// Document this function.
func (cfg *Config) applyEnvOverrides() {
	// ENV_{...}
	// These are general overrides.

	// ENV_DEBUG
	if val, ok := os.LookupEnv("ENV_DEBUG"); ok {
		if bVal, err := strconv.ParseBool(val); err == nil {
			cfg.Debug = bVal
			fmt.Printf("configuration: Overriding debug from env: %v", bVal)
		}
	}

	// ENV_UDP_{...}
	// These are specific to the transport layer.

	// ENV_UDP_ENABLED
	if val, ok := os.LookupEnv("ENV_UDP_ENABLED"); ok {
		if bVal, err := strconv.ParseBool(val); err == nil {
			cfg.Transport.UDPEnabled = bVal
			fmt.Printf("configuration: Overriding transport.udp_enabled from env: %v", bVal)
		}
	}
	// ENV_UDP_TARGET_ADDRESS
	if val, ok := os.LookupEnv("ENV_UDP_TARGET_ADDRESS"); ok {
		cfg.Transport.UDPTargetAddress = val
		fmt.Printf("configuration: Overriding transport.udp_target_address from env: %s", val)
	}
	// ENV_UDP_SEND_INTERVAL
	if val, ok := os.LookupEnv("ENV_UDP_SEND_INTERVAL"); ok {
		if dur, err := time.ParseDuration(val); err == nil {
			cfg.Transport.UDPSendInterval = dur
			fmt.Printf("configuration: Overriding transport.udp_send_interval from env: %s", dur)
		}
	}
}
