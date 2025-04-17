package config

// DeviceID returns the input device ID with compatibility for old code
func (c *Config) DeviceID() int {
	return c.Audio.InputDevice
}

// Channels returns the number of input channels with compatibility for old code
func (c *Config) Channels() int {
	return c.Audio.InputChannels
}

// FramesPerBuffer returns the frames per buffer with compatibility for old code
func (c *Config) FramesPerBuffer() int {
	return c.Audio.FramesPerBuffer
}

// SampleRate returns the sample rate with compatibility for old code
func (c *Config) SampleRate() float64 {
	return c.Audio.SampleRate
}

// LowLatency returns whether to use low latency mode
func (c *Config) LowLatency() bool {
	// Add a field to your config or derive from other settings
	return false // Default to high latency for now
}
