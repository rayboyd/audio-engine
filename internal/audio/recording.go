package audio

import (
	"fmt"
	"os"
	"sync/atomic"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

func (e *Engine) StartRecording(filename string) error {
	if atomic.LoadInt32(&e.isRecording) == 1 {
		return fmt.Errorf("already recording")
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	e.outputFile = file

	e.wavEncoder = wav.NewEncoder(file, int(e.config.SampleRate),
		32, e.config.Channels, 1)

	e.sampleBuf = &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: e.config.Channels,
			SampleRate:  int(e.config.SampleRate),
		},
		Data: make([]int, e.config.FramesPerBuffer*e.config.Channels),
	}

	atomic.StoreInt32(&e.isRecording, 1)

	return nil
}

func (e *Engine) StopRecording() error {
	if atomic.LoadInt32(&e.isRecording) == 0 {
		return nil
	}

	atomic.StoreInt32(&e.isRecording, 0)

	if e.wavEncoder != nil {
		if err := e.wavEncoder.Close(); err != nil {
			return err
		}
		e.wavEncoder = nil
	}

	if e.outputFile != nil {
		if err := e.outputFile.Close(); err != nil {
			return err
		}
		e.outputFile = nil
	}

	return nil
}

func (e *Engine) Close() error {
	if atomic.LoadInt32(&e.isRecording) == 1 {
		if err := e.StopRecording(); err != nil {
			return err
		}
	}

	if err := e.StopInputStream(); err != nil {
		return err
	}

	return nil
}
