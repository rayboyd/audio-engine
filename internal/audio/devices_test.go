package audio

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gordonklaus/portaudio"
)

func setupPortAudio(t *testing.T) {
	t.Helper()
	if err := Initialize(); err != nil {
		t.Fatalf("Failed to initialize PortAudio: %v", err)
	}
	t.Cleanup(func() {
		if err := Terminate(); err != nil {
			t.Fatalf("Failed to terminate PortAudio: %v", err)
		}
	})
}

func TestHostDevices(t *testing.T) {
	setupPortAudio(t)

	devices, err := HostDevices()
	if err != nil {
		t.Fatalf("HostDevices error: %v", err)
	}
	if len(devices) == 0 {
		t.Skip("No audio devices found on system")
	}
	for i, d := range devices {
		if d.ID != i {
			t.Errorf("Device ID mismatch: got %d, want %d", d.ID, i)
		}
		if d.Name == "" {
			t.Errorf("Device %d has empty name", i)
		}
		if d.DefaultSampleRate <= 0 {
			t.Errorf("Device %d has invalid sample rate: %f", i, d.DefaultSampleRate)
		}
	}
}

func TestHostDevices_paDevicesError(t *testing.T) {
	setupPortAudio(t)

	orig := paDevicesFunc
	defer func() { paDevicesFunc = orig }()
	paDevicesFunc = func() ([]*portaudio.DeviceInfo, error) {
		return nil, fmt.Errorf("mock error")
	}

	_, err := HostDevices()
	if err == nil || !strings.Contains(err.Error(), "mock error") {
		t.Errorf("expected mock error, got %v", err)
	}
}

func TestInputDevice(t *testing.T) {
	setupPortAudio(t)

	devices, err := HostDevices()
	if err != nil {
		t.Fatalf("HostDevices error: %v", err)
	}
	if len(devices) == 0 {
		t.Skip("No audio devices found on system")
	}

	if dev, err := InputDevice(-1); err == nil && dev.Name == "" {
		t.Error("Default input device has empty name")
	}

	validID := -1
	for _, d := range devices {
		if d.MaxInputChannels > 0 {
			validID = d.ID
			break
		}
	}
	if validID == -1 {
		t.Skip("No input devices available")
	}

	t.Run("Valid input device", func(t *testing.T) {
		dev, err := InputDevice(validID)
		if err != nil {
			t.Fatalf("InputDevice(%d) error: %v", validID, err)
		}
		if dev.Name == "" {
			t.Error("Input device has empty name")
		}
	})

	tests := []struct {
		name   string
		id     int
		substr string
	}{
		{"Negative ID", -2, "invalid device ID"},
		{"Too high ID", len(devices) + 10, "invalid device ID"},
		{"Non-input device", findNonInputDeviceID(devices), "does not support input"},
	}
	for _, tt := range tests {
		if tt.id == -100 {
			continue // No non-input device found
		}
		t.Run(tt.name, func(t *testing.T) {
			_, err := InputDevice(tt.id)
			if err == nil {
				t.Errorf("Expected error for ID %d", tt.id)
			} else if !strings.Contains(err.Error(), tt.substr) {
				t.Errorf("Error = %q, want substring %q", err.Error(), tt.substr)
			}
		})
	}
}

func TestInputDevice_paDevicesError(t *testing.T) {
	setupPortAudio(t)

	orig := paDevicesFunc
	defer func() { paDevicesFunc = orig }()
	paDevicesFunc = func() ([]*portaudio.DeviceInfo, error) {
		return nil, fmt.Errorf("mock error")
	}

	_, err := InputDevice(-1)
	if err == nil || !strings.Contains(err.Error(), "mock error") {
		t.Errorf("expected mock error, got %v", err)
	}
}

func TestInputDevice_paDefaultInputDeviceError(t *testing.T) {
	setupPortAudio(t)

	orig := paLibDefaultInputDeviceFunc
	defer func() { paLibDefaultInputDeviceFunc = orig }()
	paLibDefaultInputDeviceFunc = func() (*portaudio.DeviceInfo, error) {
		return nil, fmt.Errorf("mock default input error")
	}

	_, err := InputDevice(-1)
	if err == nil || !strings.Contains(err.Error(), "mock default input error") {
		t.Errorf("expected mock error, got %v", err)
	}
}

func TestErrorInitialize(t *testing.T) {
	orig := paLibInitialize
	defer func() { paLibInitialize = orig }()

	paLibInitialize = func() error { return nil }
	if err := Initialize(); err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	paLibInitialize = func() error { return fmt.Errorf("mock init error") }
	if err := Initialize(); err == nil || !strings.Contains(err.Error(), "mock init error") {
		t.Errorf("expected mock init error, got %v", err)
	}
}

func TestErrorTerminate(t *testing.T) {
	orig := paLibTerminate
	defer func() { paLibTerminate = orig }()

	paLibTerminate = func() error { return nil }
	if err := Terminate(); err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	paLibTerminate = func() error { return fmt.Errorf("mock term error") }
	if err := Terminate(); err == nil || !strings.Contains(err.Error(), "mock term error") {
		t.Errorf("expected mock term error, got %v", err)
	}
}

func TestNilDevices(t *testing.T) {
	setupPortAudio(t)

	orig := paLibDevicesFunc
	defer func() { paLibDevicesFunc = orig }()
	paLibDevicesFunc = func() ([]*portaudio.DeviceInfo, error) {
		return nil, nil
	}

	devices, err := paDevices()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if devices == nil {
		t.Errorf("expected empty slice, got nil")
	}
	if len(devices) != 0 {
		t.Errorf("expected length 0, got %d", len(devices))
	}
}

func TestPortAudioNotInitialized(t *testing.T) {
	orig := paLibDevicesFunc
	defer func() { paLibDevicesFunc = orig }()
	paLibDevicesFunc = func() ([]*portaudio.DeviceInfo, error) {
		return nil, fmt.Errorf("PortAudio not initialized")
	}

	devices, err := paDevices()
	if err == nil || !strings.Contains(err.Error(), "PortAudio not initialized") {
		t.Errorf("expected 'PortAudio not initialized' error, got %v", err)
	}
	if devices != nil {
		t.Errorf("expected devices to be nil on error, got %v", devices)
	}
}

func findNonInputDeviceID(devices []Device) int {
	for _, d := range devices {
		if d.MaxInputChannels == 0 {
			return d.ID
		}
	}
	return -100
}
