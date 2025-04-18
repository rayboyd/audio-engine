// SPDX-License-Identifier: MIT
package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// TODO:
// Document this struct.
type Config struct {
	Debug     bool            `yaml:"debug"`
	LogLevel  string          `yaml:"log_level"`
	Command   string          `yaml:"command,omitempty"` // A one-off command to execute. e.g., "list" to list available audio devices.
	Audio     AudioConfig     `yaml:"audio"`
	Recording RecordingConfig `yaml:"recording"`
	Transport TransportConfig `yaml:"transport"`
}

type AudioConfig struct {
	InputDevice       int     `yaml:"input_device"`
	OutputDevice      int     `yaml:"output_device"`
	SampleRate        float64 `yaml:"sample_rate"`
	FramesPerBuffer   int     `yaml:"frames_per_buffer"`
	LowLatency        bool    `yaml:"low_latency"`
	InputChannels     int     `yaml:"input_channels"`
	OutputChannels    int     `yaml:"output_channels"`
	UseDefaultDevices bool    `yaml:"use_default_devices"`
	FFTWindow         string  `yaml:"fft_window"`
}

type RecordingConfig struct {
	Enabled     bool    `yaml:"enabled"`
	OutputDir   string  `yaml:"output_dir"`
	Format      string  `yaml:"format"`
	BitDepth    int     `yaml:"bit_depth"`
	MaxDuration int     `yaml:"max_duration_seconds"`
	SilenceTh   float64 `yaml:"silence_threshold"`
}

type TransportConfig struct {
	UDPEnabled       bool          `yaml:"udp_enabled"`
	UDPTargetAddress string        `yaml:"udp_target_address"`
	UDPSendInterval  time.Duration `yaml:"udp_send_interval"`
}

// TODO:
// Document this function.
func LoadConfig(path string) (*Config, error) {
	cfg := Config{
		Debug:    false,
		LogLevel: "info",
		Audio: AudioConfig{
			InputDevice:       -1, // default
			OutputDevice:      -1,
			SampleRate:        44100,
			FramesPerBuffer:   1024,
			LowLatency:        false,
			InputChannels:     2,
			OutputChannels:    2,
			UseDefaultDevices: true,
			FFTWindow:         "Hann",
		},
		Recording: RecordingConfig{
			Enabled:     false,
			OutputDir:   "./recordings",
			Format:      "wav",
			BitDepth:    16,
			MaxDuration: 0, // unlimited
			SilenceTh:   0.01,
		},
		Transport: TransportConfig{
			UDPEnabled:       false, // Default UDP to false
			UDPTargetAddress: "127.0.0.1:9090",
			UDPSendInterval:  33 * time.Millisecond, // Default ~30Hz
		},
	}

	if path == "" {
		// TODO:
		// A list of OS candidates for config file location.
		candidates := []string{
			"config.yaml",
			// filepath.Join(os.Getenv("HOME"), ".config/config.yaml"),
			// "/etc/config.yaml",
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}
		if path == "" {
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

	cfg.applyEnvOverrides()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	// ... existing audio/recording validation ...

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
			cfg.Debug = strings.ToLower(val) == "true"
			log.Printf("Config: Overriding debug from env: %v", bVal)
		}
	}

	// ENV_UDP_{...}
	// These are specific to the transport layer.

	// ENV_UDP_ENABLED
	if val, ok := os.LookupEnv("ENV_UDP_ENABLED"); ok {
		if bVal, err := strconv.ParseBool(val); err == nil {
			cfg.Transport.UDPEnabled = bVal
			log.Printf("Config: Overriding transport.udp_enabled from env: %v", bVal)
		}
	}
	// ENV_UDP_TARGET_ADDRESS
	if val, ok := os.LookupEnv("ENV_UDP_TARGET_ADDRESS"); ok {
		cfg.Transport.UDPTargetAddress = val
		log.Printf("Config: Overriding transport.udp_target_address from env: %s", val)
	}
	// ENV_UDP_SEND_INTERVAL
	if val, ok := os.LookupEnv("ENV_UDP_SEND_INTERVAL"); ok {
		if dur, err := time.ParseDuration(val); err == nil {
			cfg.Transport.UDPSendInterval = dur
			log.Printf("Config: Overriding transport.udp_send_interval from env: %s", dur)
		}
	}
}
