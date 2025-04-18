// SPDX-License-Identifier: MIT
package main

import (
	"audio/internal/audio"
	"audio/internal/config"
	applog "audio/internal/log"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gordonklaus/portaudio"
)

func main() {
	// ------------------------------------------------------------------------
	// STARTUP (Cold Path)
	// ------------------------------------------------------------------------

	// --- Initialize PortAudio Early ---

	if err := portaudio.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to initialize PortAudio: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		fmt.Println("Terminating PortAudio.")
		if err := portaudio.Terminate(); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to terminate PortAudio cleanly: %v\n", err)
		} else {
			fmt.Println("PortAudio terminated.")
		}
	}()

	// --- Parse Flags ---

	configPath := flag.String("config", "", "Path to config file")
	flag.Parse()

	// --- Handle One-Off Commands ---

	if len(flag.Args()) > 0 {
		switch flag.Args()[0] {
		case "list":
			if err := listDevices(); err != nil {
				fmt.Fprintf(os.Stderr, "Error listing devices: %v\n", err)
				os.Exit(1)
			}
			return
		case "version":
			fmt.Println("Audio Engine version 1.0.0")
			return
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n", flag.Args()[0])
			fmt.Println("Available commands: list, version")
			os.Exit(1)
		}
	}

	// --- Load Configuration ---

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// --- Setup Logging ---

	logLevel, ok := applog.ParseLevel(cfg.LogLevel)
	if !ok {
		fmt.Fprintf(os.Stderr, "WARN: Invalid log_level '%s' in config, using INFO\n", cfg.LogLevel)
		logLevel = applog.LevelInfo
	}

	// Override log level if debug flag is explicitly set via command line.
	if debugFlag := flag.Lookup("debug"); debugFlag != nil && debugFlag.Value.String() == "true" {
		cfg.Debug = true
		logLevel = applog.LevelDebug
		fmt.Println("INFO: Debug mode enabled via command line flag, setting log level to DEBUG.")
	} else if cfg.Debug {
		// If debug is set in config but not overridden by flag, ensure level is Debug.
		logLevel = applog.LevelDebug
	}
	applog.SetLevel(logLevel)

	// The applog is ready to use!
	applog.Infof("Configuration loaded successfully. Log level set to %s.", logLevel)
	if cfg.Debug {
		applog.Debugf("Debug mode enabled.")
	}

	// --- Initialize Audio Engine ---

	applog.Info("Initializing audio engine...")
	engine, err := audio.NewEngine(cfg)
	if err != nil {
		applog.Fatalf("Failed to initialize audio engine: %v", err)
	}
	defer engine.Close()

	// --- Hot Path Initialization ---

	// CRITICAL: Start of real-time audio processing
	applog.Info("Starting audio stream...")
	if err := engine.StartInputStream(); err != nil {
		applog.Fatalf("Failed to start audio stream: %v", err)
	}
	applog.Info("Audio stream started. Waiting for interrupt signal (Ctrl+C)...")

	// --- Graceful Shutdown Handling ---

	blockUntilSigTerm := make(chan os.Signal, 1)
	signal.Notify(blockUntilSigTerm, syscall.SIGINT, syscall.SIGTERM)

	// CRITICAL: This will block until a signal is received.
	<-blockUntilSigTerm
	applog.Info("")
	applog.Info("Shutdown signal received, stopping engine...")

	// --- Shutdown Phase (Cold Path) ---

	// Engine Close is handled by defer.
	// PortAudio Terminate is handled by defer.
}

// listDevices lists audio devices using standard fmt for direct output.
func listDevices() error {
	devices, err := audio.HostDevices()
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
func printDeviceDetails(device audio.Device) {
	// ... (implementation remains the same, using fmt) ...
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
