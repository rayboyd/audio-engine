// SPDX-License-Identifier: MIT
package audio

import (
	"strings"
	"testing"

	"github.com/gordonklaus/portaudio"
)

func TestInitializeTerminate(t *testing.T) {
	_ = portaudio.Terminate()

	if err := Initialize(); err != nil {
		t.Fatalf("Failed to initialize PortAudio: %v", err)
	}

	if err := Initialize(); err != nil {
		t.Fatalf("Failed on second initialization: %v", err)
	}

	if err := Terminate(); err != nil {
		t.Fatalf("Failed to terminate PortAudio: %v", err)
	}

	if err := Terminate(); err != nil {
		t.Fatalf("Failed on second termination: %v", err)
	}
}

func TestHostDevices(t *testing.T) {
	devices, err := HostDevices()
	if err != nil {
		t.Fatalf("Failed to get host devices: %v", err)
	}

	if len(devices) == 0 {
		t.Log("Warning: No audio devices found on system")
		return
	}

	for i, device := range devices {
		if device.ID != i {
			t.Errorf("Device ID mismatch at index %d: got ID %d", i, device.ID)
		}

		if device.Name == "" {
			t.Errorf("Device %d has empty name", i)
		}

		if device.HostApiName == "" || device.HostApiName == "Unknown" {
			t.Logf("Device %d (%s) has unavailable host API: %s",
				i, device.Name, device.HostApiName)
		}

		if device.MaxInputChannels < 0 {
			t.Errorf("Device %d has invalid input channel count: %d",
				i, device.MaxInputChannels)
		}

		if device.MaxOutputChannels < 0 {
			t.Errorf("Device %d has invalid output channel count: %d",
				i, device.MaxOutputChannels)
		}

		if device.DefaultSampleRate <= 0 {
			t.Errorf("Device %d has invalid sample rate: %f",
				i, device.DefaultSampleRate)
		}

		if i == len(devices)-1 && !hasDefaultDevice(devices) {
			t.Log("Warning: No default input device identified")
		}
	}
}

func TestInputDevice(t *testing.T) {
	device, err := InputDevice(-1)
	if err != nil {
		t.Logf("Could not get default input device: %v - some tests skipped", err)
	} else {
		if device.Name == "" {
			t.Error("Default device has empty name")
		}

		if device.MaxInputChannels <= 0 {
			t.Error("Default device has no input channels")
		}

		if device.DefaultSampleRate <= 0 {
			t.Error("Default device has invalid sample rate")
		}
	}

	allDevices, err := HostDevices()
	if err != nil {
		t.Fatalf("Failed to get all devices: %v", err)
	}

	if len(allDevices) == 0 {
		t.Log("No audio devices available, skipping further tests")
		return
	}

	var inputDevice Device
	var validID int
	foundInput := false
	for _, dev := range allDevices {
		if dev.MaxInputChannels > 0 {
			inputDevice = dev
			validID = dev.ID
			foundInput = true
			break
		}
	}
	if !foundInput {
		t.Log("No input devices available, skipping specific ID tests")
		return
	}

	device, err = InputDevice(validID)
	if err != nil {
		t.Errorf("Failed to get device with valid ID %d: %v", validID, err)
	} else if device.Name != inputDevice.Name {
		t.Errorf("Got wrong device: expected %s, got %s",
			inputDevice.Name, device.Name)
	}

	invalidTests := []struct {
		desc    string
		id      int
		errText string
	}{
		{"Negative ID", -2, "invalid device ID"}, // -1 is default
		{"Too high ID", len(allDevices) + 10, "invalid device ID"},
		{"Non-input device", findNonInputDeviceID(allDevices), "does not support input"},
	}

	for _, tt := range invalidTests {
		if tt.id == -100 { // Sentinel for not found
			continue
		}

		t.Run(tt.desc, func(t *testing.T) {
			_, err := InputDevice(tt.id)
			if err == nil {
				t.Errorf("Expected error for ID %d but got nil", tt.id)
			} else if !strings.Contains(err.Error(), tt.errText) {
				t.Errorf("Wrong error for ID %d: got %q, expected to contain %q",
					tt.id, err.Error(), tt.errText)
			}
		})
	}
}

func TestPaDevices(t *testing.T) {
	if err := Initialize(); err != nil {
		t.Fatalf("Failed to initialize PortAudio: %v", err)
	}
	defer Terminate()

	devices, err := paDevices()
	if err != nil {
		t.Fatalf("Failed to get PortAudio devices: %v", err)
	}

	// This should just succeed (may return empty slice but not error).
	if devices == nil {
		t.Error("paDevices returned nil slice")
	}
}

// hasDefaultDevice checks if any device is marked as default input.
func hasDefaultDevice(devices []Device) bool {
	for _, device := range devices {
		if device.IsDefaultInput {
			return true
		}
	}
	return false
}

// findNonInputDeviceID finds a device with no input channels,
// Will return -100 if none found.
func findNonInputDeviceID(devices []Device) int {
	for _, device := range devices {
		if device.MaxInputChannels == 0 {
			return device.ID
		}
	}
	return -100 // Sentinel value
}
