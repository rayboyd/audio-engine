// SPDX-License-Identifier: MIT
package audio

import (
	"fmt"
	"time"

	"github.com/gordonklaus/portaudio"
)

// TODO:
// Document this struct.
type Device struct {
	ID                       int
	Name                     string
	DefaultSampleRate        float64
	DefaultLowInputLatency   time.Duration
	DefaultHighInputLatency  time.Duration
	DefaultLowOutputLatency  time.Duration
	DefaultHighOutputLatency time.Duration
	MaxInputChannels         int
	MaxOutputChannels        int
	HostApiName              string
	IsDefaultInput           bool
	IsDefaultOutput          bool
}

// TODO:
// Document this function.
// This should be called ONCE at application startup.
func Initialize() error {
	if err := paLibInitialize(); err != nil {
		return err
	}
	return nil
}

// TODO:
// Document this function.
// This should be called ONCE at application shutdown.
func Terminate() error {
	if err := paLibTerminate(); err != nil {
		return err
	}
	return nil
}

// TODO:
// Document this function.
func HostDevices() ([]Device, error) {
	paDevs, err := paDevicesFunc()
	if err != nil {
		return nil, err
	}

	defaultInInfo, errIn := portaudio.DefaultInputDevice()
	defaultOutInfo, errOut := portaudio.DefaultOutputDevice()

	deviceList := make([]Device, len(paDevs))
	for i, info := range paDevs {
		hostApiName := "Unknown"
		if info.HostApi != nil {
			hostApiName = info.HostApi.Name
		}

		isDefaultIn := errIn == nil && defaultInInfo != nil && info.Name == defaultInInfo.Name
		isDefaultOut := errOut == nil && defaultOutInfo != nil && info.Name == defaultOutInfo.Name

		deviceList[i] = Device{
			ID:                       i,
			Name:                     info.Name,
			DefaultSampleRate:        info.DefaultSampleRate,
			DefaultLowInputLatency:   info.DefaultLowInputLatency,
			DefaultHighInputLatency:  info.DefaultHighInputLatency,
			DefaultLowOutputLatency:  info.DefaultLowOutputLatency,
			DefaultHighOutputLatency: info.DefaultHighOutputLatency,
			MaxInputChannels:         info.MaxInputChannels,
			MaxOutputChannels:        info.MaxOutputChannels,
			HostApiName:              hostApiName,
			IsDefaultInput:           isDefaultIn,
			IsDefaultOutput:          isDefaultOut,
		}
	}

	return deviceList, nil
}

// InputDevice retrieves the audio input device for the given device ID.
// If deviceID is MinDeviceID (-1), returns the system default input device.
// Returns an error if the device ID is invalid or no such device exists.
func InputDevice(deviceID int) (*portaudio.DeviceInfo, error) {
	paDevs, err := paDevicesFunc()
	if err != nil {
		return nil, err
	}

	if deviceID == -1 {
		defDevice, err := paLibDefaultInputDeviceFunc()
		if err != nil {
			return nil, err
		}
		return defDevice, nil
	}

	if deviceID < 0 || deviceID >= len(paDevs) {
		// TODO:
		// Preallocate this error message.
		return nil, fmt.Errorf(
			"invalid device ID: %d (must be between 0 and %d, or %d for default)",
			deviceID, len(paDevs)-1, -1)
	}

	if paDevs[deviceID].MaxInputChannels == 0 {
		// TODO:
		// Preallocate this error message.
		return nil, fmt.Errorf(
			"device ID %d (%s) does not support input",
			deviceID, paDevs[deviceID].Name)
	}

	return paDevs[deviceID], nil
}

// TODO:
// Document this function.
func paDevices() ([]*portaudio.DeviceInfo, error) {
	devices, err := paLibDevicesFunc()
	if err != nil {
		// TODO:
		// This assumes PortAudio is already initialized, should we check
		// if the error is "PortAudio not initialized" and return a specific
		// error? ... ??? needs tested.
		return nil, err
	}

	if devices == nil {
		return []*portaudio.DeviceInfo{}, nil
	}

	return devices, nil
}

// Helpers

// listDevices lists audio devices using standard fmt for direct output.
func ListDevices() error {
	devices, err := HostDevices()
	if err != nil {
		return fmt.Errorf("failed to get host devices: %w", err)
	}
	if len(devices) == 0 {
		fmt.Println("No audio devices found.")
		return nil
	}
	fmt.Println("Available Audio Devices:")
	fmt.Println("------------------------")
	for _, d := range devices {
		printDeviceDetails(d)
	}
	fmt.Println("------------------------")
	return nil
}

// printDeviceDetails formats and prints information about a single audio device using standard fmt.
func printDeviceDetails(device Device) {
	deviceType := "Unknown"
	if device.MaxInputChannels > 0 && device.MaxOutputChannels > 0 {
		deviceType = "Input/Output"
	} else if device.MaxInputChannels > 0 {
		deviceType = "Input"
	} else if device.MaxOutputChannels > 0 {
		deviceType = "Output"
	}

	defaultMarker := ""
	if device.IsDefaultInput && device.IsDefaultOutput {
		defaultMarker = " (Default Input & Output)"
	} else if device.IsDefaultInput {
		defaultMarker = " (Default Input)"
	} else if device.IsDefaultOutput {
		defaultMarker = " (Default Output)"
	}

	// Print basic info
	fmt.Printf("[%d] %s%s\n", device.ID, device.Name, defaultMarker)
	fmt.Printf("    Type: %s, Host API: %s\n", deviceType, device.HostApiName)
	fmt.Printf("    Channels: Input=%d, Output=%d\n", device.MaxInputChannels, device.MaxOutputChannels)
	fmt.Printf("    Default Sample Rate: %.0f Hz\n", device.DefaultSampleRate)

	// Print latency info if applicable
	if device.MaxInputChannels > 0 {
		fmt.Printf("    Default Input Latency: Low=%.2fms, High=%.2fms\n",
			device.DefaultLowInputLatency.Seconds()*1000,
			device.DefaultHighInputLatency.Seconds()*1000)
	}
	if device.MaxOutputChannels > 0 {
		fmt.Printf("    Default Output Latency: Low=%.2fms, High=%.2fms\n",
			device.DefaultLowOutputLatency.Seconds()*1000,
			device.DefaultHighOutputLatency.Seconds()*1000)
	}
	fmt.Println()
}

// Mockable functions for testing

var paDevicesFunc = paDevices
var paLibDevicesFunc = portaudio.Devices
var paLibDefaultInputDeviceFunc = portaudio.DefaultInputDevice
var paLibInitialize = portaudio.Initialize
var paLibTerminate = portaudio.Terminate
