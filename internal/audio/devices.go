package audio

import (
	"audio/internal/config"
	"fmt"

	"github.com/gordonklaus/portaudio"
)

// Initialize sets up the PortAudio subsystem.
// This must be called before any audio operations and paired with a Terminate() call.
func Initialize() error {
	if err := portaudio.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize PortAudio: %w", err)
	}
	return nil
}

// Terminate cleanly shuts down the PortAudio subsystem.
// This should be deferred immediately after Initialize().
func Terminate() error {
	if err := portaudio.Terminate(); err != nil {
		return fmt.Errorf("failed to terminate PortAudio: %w", err)
	}
	return nil
}

// InputDevice retrieves the audio input device for the given device ID.
// If deviceID is MinDeviceID (-1), returns the system default input device.
// Returns an error if the device ID is invalid or no such device exists.
func InputDevice(deviceID int) (*portaudio.DeviceInfo, error) {
	devices, err := paDevices()
	if err != nil {
		return nil, err
	}

	if deviceID == config.MinDeviceID {
		device, err := portaudio.DefaultInputDevice()
		if err != nil {
			return nil, err
		}
		return device, nil
	}

	if deviceID < 0 || deviceID >= len(devices) {
		return nil, fmt.Errorf("invalid device ID: %d", deviceID)
	}
	return devices[deviceID], nil
}

// ListDevices prints information about all available audio devices.
// For each device, it shows:
// - Device ID and name
// - Device type (Input/Output/Input+Output)
// - Channel count
// - Default sample rate
// - Latency ranges
func ListDevices() error {
	devices, err := paDevices()
	if err != nil {
		return err
	}

	fmt.Printf("\nAvailable Audio Devices\n\n")

	for i, device := range devices {
		inputChannels := device.MaxInputChannels
		outputChannels := device.MaxOutputChannels

		deviceType := ""
		if inputChannels > 0 && outputChannels > 0 {
			deviceType = "Input/Output"
		} else if inputChannels > 0 {
			deviceType = "Input"
		} else if outputChannels > 0 {
			deviceType = "Output"
		}

		fmt.Printf("[%d] %s (%s)\n", i, device.Name, deviceType)
		fmt.Printf("    Input channels: %d, Output channels: %d\n", inputChannels, outputChannels)
		fmt.Printf("    Default sample rate: %.0f Hz\n", device.DefaultSampleRate)
		fmt.Printf("    Latency: Low=%.2fms, High=%.2fms\n",
			device.DefaultLowInputLatency.Seconds()*1000,
			device.DefaultHighInputLatency.Seconds()*1000)
		fmt.Println()
	}

	return nil
}

// paDevices returns all available PortAudio devices.
// This is a helper function used internally by InputDevice and ListDevices.
func paDevices() ([]*portaudio.DeviceInfo, error) {
	devices, err := portaudio.Devices()
	if err != nil {
		return nil, err
	}
	return devices, nil
}
