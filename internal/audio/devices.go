// SPDX-License-Identifier: MIT
package audio

import (
	"audio/internal/config"
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

func Initialize() error {
	if err := portaudio.Initialize(); err != nil {
		return err
	}
	return nil
}

func Terminate() error {
	if err := portaudio.Terminate(); err != nil {
		return err
	}
	return nil
}

// HostDevices returns a struct containing information about all available audio
// devices on the host system. The function initializes PortAudio.
func HostDevices() ([]Device, error) {
	if err := Initialize(); err != nil {
		return nil, err
	}
	defer Terminate()

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
	if err := Initialize(); err != nil {
		return nil, err
	}
	defer Terminate()

	paDevs, err := paDevices()
	if err != nil {
		return nil, err
	}

	if deviceID == config.DefaultDeviceID {
		defDevice, err := portaudio.DefaultInputDevice()
		if err != nil {
			return nil, err
		}
		return defDevice, nil
	}

	if deviceID < 0 || deviceID >= len(paDevs) {
		return nil, fmt.Errorf(
			"invalid device ID: %d (must be between 0 and %d, or %d for default)",
			deviceID, len(paDevs)-1, config.DefaultDeviceID)
	}

	if paDevs[deviceID].MaxInputChannels == 0 {
		return nil, fmt.Errorf(
			"device ID %d (%s) does not support input",
			deviceID, paDevs[deviceID].Name)
	}

	return paDevs[deviceID], nil
}

// paDevices returns all available PortAudio devices.
func paDevices() ([]*portaudio.DeviceInfo, error) {
	devices, err := portaudio.Devices()
	if err != nil {
		return nil, err
	}

	if devices == nil {
		return []*portaudio.DeviceInfo{}, nil
	}

	return devices, nil
}
