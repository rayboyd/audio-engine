// SPDX-License-Identifier: MIT
package main

import (
	"audio/internal/audio"
	"audio/internal/config"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

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
	// ------------------------------------------------------------------------
	// STARTUP (Cold Path)
	//
	// 1. Init portaudio
	// 2. Parse command line arguments
	// 3. Execute one-off commands (e.g., device listing)
	// ------------------------------------------------------------------------
	// ...
	// ------------------------------------------------------------------------

	if err := audio.Initialize(); err != nil {
		log.Fatalf("FATAL: Failed to initialize PortAudio: %v", err)
	}
	defer func() {
		// debug
		log.Println("Terminating PortAudio.")
		if err := audio.Terminate(); err != nil {
			// error
			log.Printf("ERROR: Failed to terminate PortAudio cleanly: %v", err)
		} else {
			// debug
			log.Println("PortAudio terminated.")
		}
	}()

	// Parse command line arguments and build configuration
	configPath := flag.String("config", "", "Path to config file")
	listDevices := flag.Bool("list", false, "List available audio devices and exit")
	flag.Parse()

	// Handle the list flag as a special case for backwards compatibility
	if *listDevices {
		config := config.DefaultConfig()
		config.Command = "list"
		if err := executeCommand(config); err != nil {
			log.Fatalf("FATAL: Failed to execute command 'list' %v", err)
		}
		return
	}

	// Load configuration from file
	config, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("FATAL: Failed to load configuration %v", err)
	}

	// Debug mode setup
	if config.Debug {
		log.Println("Debug mode enabled.")
	}

	// ==================== CONCURRENT PHASE (Hot Path) ====================

	// Initialize the audio engine
	engine, err := audio.NewEngine(config)
	if err != nil {
		log.Fatalf("FATAL: Failed to create audio engine: %v", err)
	}
	// Defer engine close AFTER successful creation
	defer engine.Close()

	// CRITICAL: Start of real-time audio processing
	// The first call to StartInputStream triggers PortAudio to begin
	// calling the callback function, marking the start of the hot path
	if err := engine.StartInputStream(); err != nil {
		log.Fatalf("FATAL: Failed to start audio stream: %v", err)
	}
	log.Println("Audio stream started. Waiting for interrupt signal (Ctrl+C)...")

	// --- Graceful Shutdown Handling ---
	// Setup ONE channel to listen for SIGINT (Ctrl+C) or SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block execution until a signal is received
	<-quit
	log.Println("") // Newline for cleaner shutdown logging
	log.Println("Shutdown signal received, stopping engine...")

	// --- Shutdown Phase (Cold Path) ---
	// Engine Close is handled by defer
	log.Println("Engine stopped.")
	// PortAudio Terminate is handled by defer
	log.Println("Grec V2 finished.")
}

// executeCommand handles one-off commands that don't require the audio engine
// to be running, such as listing available audio devices.
func executeCommand(cfg *config.Config) error {
	switch cfg.Command {
	case "list":
		devices, err := audio.HostDevices() // Assumes PortAudio is initialized
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

	// TODO: Add other cases for commands like 'version', 'help' if needed
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
