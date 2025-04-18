package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// TODO:
// Document this struct.
type Config struct {
	Debug    bool   `yaml:"debug"`
	LogLevel string `yaml:"log_level"`

	// A one-off command to execute.
	// e.g., "list" to list available audio devices.
	Command string `yaml:"command,omitempty"`

	Audio struct {
		InputDevice       int     `yaml:"input_device"`
		OutputDevice      int     `yaml:"output_device"`
		SampleRate        float64 `yaml:"sample_rate"`
		FramesPerBuffer   int     `yaml:"frames_per_buffer"`
		LowLatency        bool    `yaml:"low_latency"`
		InputChannels     int     `yaml:"input_channels"`
		OutputChannels    int     `yaml:"output_channels"`
		UseDefaultDevices bool    `yaml:"use_default_devices"`
	} `yaml:"audio"`

	Recording struct {
		Enabled     bool    `yaml:"enabled"`
		OutputDir   string  `yaml:"output_dir"`
		Format      string  `yaml:"format"`
		BitDepth    int     `yaml:"bit_depth"`
		MaxDuration int     `yaml:"max_duration_seconds"`
		SilenceTh   float64 `yaml:"silence_threshold"`
	} `yaml:"recording"`
}

// TODO:
// Document this function.
func DefaultConfig() *Config {
	cfg := &Config{
		Debug:    false,
		LogLevel: "info",
	}

	// TODO:
	// Do we need this? can it be something else?
	// idk, but I need to revise how I'm doing defaults.

	cfg.Audio.InputDevice = -1 // default
	cfg.Audio.OutputDevice = -1
	cfg.Audio.SampleRate = 44100
	cfg.Audio.FramesPerBuffer = 1024
	cfg.Audio.InputChannels = 2
	cfg.Audio.OutputChannels = 2
	cfg.Audio.UseDefaultDevices = true

	cfg.Recording.Enabled = false
	cfg.Recording.OutputDir = "recordings"
	cfg.Recording.Format = "wav"
	cfg.Recording.BitDepth = 16
	cfg.Recording.MaxDuration = 0 // unlimited
	cfg.Recording.SilenceTh = 0.01

	return cfg
}

// TODO:
// Document this function.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
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
			return cfg, nil
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		// TODO:
		// Preallocate this error message.
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		// TODO:
		// Preallocate this error message.
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg.applyEnvOverrides()

	return cfg, nil
}

// TODO:
// Document this function.
func (cfg *Config) SaveConfig(path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		// Note: SaveConfig marshal error branch is not covered in
		// test because Config is always marshalable. We could still
		// trigger this error by modifying the struct to include a
		// field that is not marshalable.
		// TODO:
		// Preallocate this error message.
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		// TODO:
		// Preallocate this error message.
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		// TODO:
		// Preallocate this error message.
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// TODO:
// Document this function.
func (cfg *Config) applyEnvOverrides() {
	if env := os.Getenv("ENV_DEBUG"); env != "" {
		cfg.Debug = strings.ToLower(env) == "true"
	}
}
