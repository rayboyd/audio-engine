package main

import (
	"audio/cmd"
	"audio/internal/audio"
	"audio/internal/build"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

// main is the entry point for the audio processing application.
// The program flow is divided into three distinct phases:
//
// 1. Startup Phase (Cold Path):
//   - Initialize build information
//   - Configure runtime settings
//   - Initialize PortAudio
//   - Parse command line arguments
//   - Execute one-off commands if requested
//
// 2. Concurrent Phase (Hot Path):
//   - Start audio processing engine
//   - Begin input stream processing
//   - Start recording if enabled
//   - Initialize UI components
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
		if err := executeCommand(config.Command); err != nil {
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
func executeCommand(command string) error {
	// Command implementation
	return nil
}
