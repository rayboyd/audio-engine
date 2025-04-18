package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

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

func DefaultConfig() *Config {
	cfg := &Config{
		Debug:    false,
		LogLevel: "info",
	}

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

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	if path == "" {
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
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg.applyEnvOverrides()

	return cfg, nil
}

func (cfg *Config) SaveConfig(path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (cfg *Config) applyEnvOverrides() {
	if env := os.Getenv("ENV_DEBUG"); env != "" {
		cfg.Debug = strings.ToLower(env) == "true"
	}
}
