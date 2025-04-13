package main

import (
	"audio/cmd"
	"audio/internal/audio"
	"audio/internal/config"
	"audio/pkg/build"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

// Note: TUI is not implemented yet, ignore placeholders
//
// The program flow is divided into three distinct phases:
//
// 1. Startup Phase (Cold Path):
//   - Build info
//   - Runtime settings
//   - Initialize PortAudio
//   - Command line arguments
//   - Execute one-off commands that exit (e.g., device listing)
//
// 2. Concurrent Phase (Hot Path):
//   - Start audio processing engine
//   - Begin input stream processing
//   - Start recording if enabled
//
// 3. Shutdown Phase (Cold Path):
//   - Handle termination signals
//   - Stop recording if active
//   - Clean up resources
func main() {
	// ==================== STARTUP PHASE (Cold Path) ====================

	// Initialize build information including version, commit hash, and build time
	if err := build.Initialize(); err != nil {
		log.Fatal(err)
	}

	// Limit OS threads to optimize for real-time audio processing:
	// - One thread dedicated to audio engine (time-critical)
	// - One thread for UI and I/O operations
	runtime.GOMAXPROCS(2)

	// Initialize PortAudio subsystem
	if err := audio.Initialize(); err != nil {
		log.Fatal(err)
	}
	defer audio.Terminate()

	// Parse command line arguments and build configuration
	config, err := cmd.ParseArgs()
	if err != nil {
		log.Fatal(err)
	}

	// Handle one-off commands (e.g., device listing) that don't require
	// the audio engine to be running
	if config.Command != "" {
		if err := executeCommand(config); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Exit if not running in TUI mode
	if !config.TUIMode {
		return
	}

	// ==================== CONCURRENT PHASE (Hot Path) ====================

	// Setup signal handling for graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	// Initialize and start the audio engine
	engine, err := audio.NewEngine(config)
	if err != nil {
		log.Fatal(err)
	}

	// CRITICAL: Start of real-time audio processing
	// The first call to StartInputStream triggers PortAudio to begin
	// calling the callback function, marking the start of the hot path
	if err := engine.StartInputStream(); err != nil {
		log.Fatal(err)
	}

	// Start recording if enabled in configuration
	if config.RecordInputStream {
		if err := engine.StartRecording(config.OutputFile); err != nil {
			log.Fatal(err)
		}
	}

	if config.TUIMode {
		fmt.Printf("TUI Mode '%s --help' for usage information.\n", build.GetBuildFlags().Name)
	}

	// Block until termination signal is received
	<-done

	// ==================== SHUTDOWN PHASE (Cold Path) ====================

	// Stop recording if active and save the file
	if config.RecordInputStream {
		if err := engine.StopRecording(); err != nil {
			log.Printf("Error stopping recording: %v", err)
		}
		fmt.Printf("\nRecording saved to: %s\n", config.OutputFile)
	}

	// Clean up audio engine resources
	if err := engine.Close(); err != nil {
		log.Printf("Error closing audio engine: %v", err)
	}
}

// executeCommand handles one-off commands that don't require the audio engine
// to be running, such as listing available audio devices.
func executeCommand(cfg *config.Config) error {
	switch cfg.Command {
	case "list":
		devices, err := audio.HostDevices()
		if err != nil {
			return fmt.Errorf("failed to list devices: %w", err)
		}

		if len(devices) == 0 {
			fmt.Println("No audio devices found.")
			return nil
		}

		fmt.Printf("\nAvailable Audio Devices (%d found)\n\n", len(devices))

		// Loop through devices and call the print helper
		for _, device := range devices {
			printDeviceDetails(device)
		}

	// ... (other cases) ...
	default:
		return fmt.Errorf("unknown command: %s", cfg.Command)
	}
	return nil
}

// printDeviceDetails formats and prints information about a single audio device.
func printDeviceDetails(device audio.Device) {
	// Determine device type
	deviceType := "Unknown"
	if device.MaxInputChannels > 0 && device.MaxOutputChannels > 0 {
		deviceType = "Input/Output"
	} else if device.MaxInputChannels > 0 {
		deviceType = "Input"
	} else if device.MaxOutputChannels > 0 {
		deviceType = "Output"
	}

	// Format default marker
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
	fmt.Println() // Add a blank line for separation
}
