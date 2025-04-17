// SPDX-License-Identifier: MIT
package audio

import (
	"fmt"
	"time"

	"github.com/gordonklaus/portaudio"
)

var SampleRates = []float64{
	8000, 16000, 22050, 32000, 44100, 48000, 88200, 96000, 176400, 192000,
}

type Device struct {
	ID                       int
	Name                     string
	HostApiName              string
	MaxInputChannels         int
	MaxOutputChannels        int
	DefaultSampleRate        float64
	DefaultLowInputLatency   time.Duration
	DefaultHighInputLatency  time.Duration
	DefaultLowOutputLatency  time.Duration
	DefaultHighOutputLatency time.Duration
	IsDefaultInput           bool
	IsDefaultOutput          bool
}

// Initialize initializes the PortAudio library.
// This should be called ONCE at application startup.
func Initialize() error {
	if err := portaudio.Initialize(); err != nil {
		return err
	}
	return nil
}

// Terminate terminates the PortAudio library.
// This should be called ONCE at application shutdown.
func Terminate() error {
	if err := portaudio.Terminate(); err != nil {
		return err
	}
	return nil
}

// HostDevices returns a struct containing information about all available audio
// devices on the host system. The function initializes PortAudio.
func HostDevices() ([]Device, error) {
	paDevs, err := paDevices()
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
			HostApiName:              hostApiName,
			MaxInputChannels:         info.MaxInputChannels,
			MaxOutputChannels:        info.MaxOutputChannels,
			DefaultSampleRate:        info.DefaultSampleRate,
			DefaultLowInputLatency:   info.DefaultLowInputLatency,
			DefaultHighInputLatency:  info.DefaultHighInputLatency,
			DefaultLowOutputLatency:  info.DefaultLowOutputLatency,
			DefaultHighOutputLatency: info.DefaultHighOutputLatency,
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
	paDevs, err := paDevices()
	if err != nil {
		return nil, err
	}

	if deviceID == -1 {
		defDevice, err := portaudio.DefaultInputDevice()
		if err != nil {
			return nil, err
		}
		return defDevice, nil
	}

	if deviceID < 0 || deviceID >= len(paDevs) {
		return nil, fmt.Errorf(
			"invalid device ID: %d (must be between 0 and %d, or %d for default)",
			deviceID, len(paDevs)-1, -1)
	}

	if paDevs[deviceID].MaxInputChannels == 0 {
		return nil, fmt.Errorf(
			"device ID %d (%s) does not support input",
			deviceID, paDevs[deviceID].Name)
	}

	return paDevs[deviceID], nil
}

// paDevices returns all available PortAudio devices.
// Assumes PortAudio is already initialized.
func paDevices() ([]*portaudio.DeviceInfo, error) {
	devices, err := portaudio.Devices()
	if err != nil {
		// Check if the error is "PortAudio not initialized" and return a specific error?
		return nil, err // PortAudio must be initialized for this to work
	}

	if devices == nil {
		return []*portaudio.DeviceInfo{}, nil
	}

	return devices, nil
}
