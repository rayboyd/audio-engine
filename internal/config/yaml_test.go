// SPDX-License-Identifier: MIT
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

func TestLoadConfig_UnmarshalError(t *testing.T) {
	t.Parallel()
	path := writeTempConfig(t, ":\n:bad")
	_, err := LoadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "failed to parse config file") {
		t.Error("expected unmarshal error, got nil or wrong error")
	}
}
