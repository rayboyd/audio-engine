// SPDX-License-Identifier: MIT
package main

import (
	"audio/internal/audio"
	"audio/internal/config"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gordonklaus/portaudio"
)

func main() {
	/*
		---------------------------------------------------------------------------------
		Initialize PortAudio
		---------------------------------------------------------------------------------
	*/
	if err := portaudio.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to initialize PortAudio: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		fmt.Printf("main: Terminating PortAudio ...\n")
		if err := portaudio.Terminate(); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to terminate PortAudio cleanly: %v\n", err)
		} else {
			fmt.Printf("main: PortAudio terminated.\n")
		}
	}()

	/*
		---------------------------------------------------------------------------------
		Parse flags
		- Handle one-off commands (e.g., list devices)
		---------------------------------------------------------------------------------
	*/
	configPath := flag.String("config", "", "Path to config file")
	flag.Parse()

	if len(flag.Args()) > 0 {
		switch flag.Args()[0] {
		case "list":
			if err := audio.ListDevices(); err != nil {
				fmt.Fprintf(os.Stderr, "Error listing devices: %v\n", err)
				os.Exit(1)
			}
			return
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n", flag.Args()[0])
			os.Exit(1)
		}
	}

	/*
		---------------------------------------------------------------------------------
		Load Configuration
		---------------------------------------------------------------------------------
	*/
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("main: Configuration loaded successfully\n")
	fmt.Printf("main: Debug mode is %v\n", cfg.Debug)

	/*
		---------------------------------------------------------------------------------
		Startup
		- Audio Engine
		- Stream (I/O Hot Path)
		- Wait ... (for interrupt signal)
		---------------------------------------------------------------------------------
	*/
	fmt.Printf("main: Initializing the Audio Engine ...\n")
	engine, err := audio.NewEngine(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to initialize audio engine: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	// CRITICAL: Start of real-time audio processing (Hot Path)
	fmt.Printf("main: Starting audio stream ...\n")
	if err := engine.StartInputStream(); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to start audio stream: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("main: Audio stream started. Waiting for interrupt signal (Ctrl+C) ...\n")

	// Set up signal handling for graceful shutdown, using syscall.SIGINT  and syscall.SIGTERM.
	// This will allow the program to handle Ctrl+C and other termination signals gracefully.
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)

	// CRITICAL: This will block until a signal is received.
	<-sigterm

	/*
		---------------------------------------------------------------------------------
		Shutdown
		- PortAudio Terminate is handled by defer
		- Engine Close is handled by defer
		---------------------------------------------------------------------------------
	*/
	fmt.Printf("\nmain: Shutdown signal received ...\n")
}
