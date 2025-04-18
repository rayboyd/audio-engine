package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	if cfg.Audio.SampleRate != 44100 {
		t.Errorf("SampleRate = %v, want 44100", cfg.Audio.SampleRate)
	}
	if !cfg.Audio.UseDefaultDevices {
		t.Errorf("UseDefaultDevices = false, want true")
	}
	if cfg.Recording.Format != "wav" {
		t.Errorf("Recording.Format = %q, want \"wav\"", cfg.Recording.Format)
	}
	if cfg.Recording.OutputDir != "recordings" {
		t.Errorf("OutputDir = %q, want \"recordings\"", cfg.Recording.OutputDir)
	}
}

func TestLoadConfig_EmptyPath(t *testing.T) {
	t.Parallel()
	cfg, err := LoadConfig("")
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if cfg == nil {
		t.Error("expected default config, got nil")
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	t.Parallel()
	cfg, err := LoadConfig("nonexistent.yaml")
	if err == nil {
		t.Errorf("expected error for missing file, got nil")
	}
	if cfg != nil {
		t.Errorf("expected nil config on error, got %+v", cfg)
	}
}

func TestLoadConfig_ValidFile(t *testing.T) {
	t.Parallel()
	content := `
debug: true
log_level: debug
audio:
  input_device: 2
  use_default_devices: false
recording:
  enabled: true
  format: wav
`
	path := writeTempConfig(t, content)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if !cfg.Debug {
		t.Errorf("Debug = false, want true")
	}
	if cfg.Audio.InputDevice != 2 {
		t.Errorf("InputDevice = %d, want 2", cfg.Audio.InputDevice)
	}
	if cfg.Audio.UseDefaultDevices {
		t.Errorf("UseDefaultDevices = true, want false")
	}
	if !cfg.Recording.Enabled {
		t.Errorf("Recording.Enabled = false, want true")
	}
	if cfg.Recording.Format != "wav" {
		t.Errorf("Recording.Format = %q, want \"wav\"", cfg.Recording.Format)
	}
}

func TestLoadConfig_CandidateSelection(t *testing.T) {
	content := `
debug: true
audio:
  use_default_devices: false
`
	tmp := t.TempDir()
	candidate := filepath.Join(tmp, "config.yaml")
	if err := os.WriteFile(candidate, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write candidate config: %v", err)
	}
	oldwd, _ := os.Getwd()
	defer os.Chdir(oldwd)
	os.Chdir(tmp)

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if !cfg.Debug {
		t.Errorf("Debug = false, want true (should load candidate config)")
	}
	if cfg.Audio.UseDefaultDevices {
		t.Errorf("UseDefaultDevices = true, want false (should load candidate config)")
	}
}

func TestLoadConfig_UnmarshalError(t *testing.T) {
	t.Parallel()
	path := writeTempConfig(t, ":\n:bad")
	_, err := LoadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "failed to parse config file") {
		t.Error("expected unmarshal error, got nil or wrong error")
	}
}

func TestSaveConfigAndReload(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "save.yaml")
	cfg := DefaultConfig()
	cfg.Debug = true
	cfg.Audio.InputDevice = 5
	cfg.Recording.Format = "ogg"
	if err := cfg.SaveConfig(path); err != nil {
		t.Fatalf("SaveConfig error: %v", err)
	}
	cfg2, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if !cfg2.Debug || cfg2.Audio.InputDevice != 5 || cfg2.Recording.Format != "ogg" {
		t.Errorf("Reloaded config does not match saved config: %+v", cfg2)
	}
}

func TestSaveConfig_MkdirError(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	invalidPath := string([]byte{0}) + "/file.yaml"
	err := cfg.SaveConfig(invalidPath)
	if err == nil || !strings.Contains(err.Error(), "failed to create config directory") {
		t.Errorf("expected mkdir error, got %v", err)
	}
}

func TestSaveConfig_WriteError(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	dir := t.TempDir()
	err := cfg.SaveConfig(dir)
	if err == nil {
		t.Error("expected error for writing to a directory, got nil")
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	t.Setenv("ENV_DEBUG", "true")
	cfg := DefaultConfig()
	cfg.Debug = false
	cfg.applyEnvOverrides()
	if !cfg.Debug {
		t.Errorf("applyEnvOverrides did not set Debug=true from ENV_DEBUG")
	}
}
