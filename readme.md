# Phase4

**Note:** This project is currently in alpha development. It is not considered stable and may contain bugs or undergo significant changes.

Phase4 is a real-time audio processing engine written in Go. It captures audio input, performs Fast Fourier Transform (FFT) analysis, and can optionally send the results over UDP.

## Getting Started

### Prerequisites

1.  **Go:** Check `go.mod` for the required version.
2.  **PortAudio Development Libraries:** Required for audio capture.
    - **macOS (Homebrew):** `brew install portaudio`
    - **Debian/Ubuntu:** `sudo apt-get update && sudo apt-get install portaudio19-dev`
    - **Fedora:** `sudo dnf install portaudio-devel`
    - **Windows:** Download from the [PortAudio website](http://www.portaudio.com/download.html) or use a package manager.

### Building

A build script is provided:

```sh
./bin/build.sh
```

This script compiles the application, placing the binary (default name `app`) into the `build/` directory.

Run `bin/build.sh --test` to also run unit tests after building.

### Configuration

The application is configured using `config.yaml`.

Check `internal/config/yaml.go` for details on configuration options and potential environment variable overrides.

## Usage

### Running the Engine

Execute the compiled binary from the project root:

```sh
./build/app
```
