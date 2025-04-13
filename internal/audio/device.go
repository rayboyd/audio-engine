package audio

// Device represents an audio device
type Device struct {
	ID                int
	Name              string
	MaxInputChannels  int
	MaxOutputChannels int
	DefaultSampleRate float64
}

// GetDevices returns all available audio devices
func GetDevices() ([]Device, error) {
	// Initialize PortAudio if needed
	err := Initialize()
	if err != nil {
		return nil, err
	}
	defer Terminate()

	// Get devices from PortAudio
	paDeviceInfos, err := paDevices()
	if err != nil {
		return nil, err
	}

	// Convert PortAudio devices to our Device struct
	devices := make([]Device, len(paDeviceInfos))
	for i, info := range paDeviceInfos {
		devices[i] = Device{
			ID:                i,
			Name:              info.Name,
			MaxInputChannels:  info.MaxInputChannels,
			MaxOutputChannels: info.MaxOutputChannels,
			DefaultSampleRate: info.DefaultSampleRate,
		}
	}

	return devices, nil
}
