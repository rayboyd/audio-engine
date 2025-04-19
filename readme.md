# Phase4

**Note:** This project is currently in alpha development. It is not considered stable and may contain bugs or undergo significant changes.

Phase4 is a real-time audio processing engine written in Go. It captures audio input, performs Fast Fourier Transform (FFT) analysis, and can optionally send the results over UDP.

## Features

- **Real-time Audio Input:** Captures audio using PortAudio via the `internal/audio` package.
- **FFT Analysis:** Performs FFT on incoming audio buffers (`internal/analysis/fft.go`).
- **UDP Transport:** Sends FFT magnitude data over UDP to a specified target address (`internal/transport/udp/publisher.go`).
- **Configuration:** Configurable via a YAML file (`config.yaml`) and environment variables (`internal/config/yaml.go`).
- **Device Management:** List available audio devices (`main.go`, `internal/audio/devices.go`).
- **Logging:** Structured logging (`internal/log/logger.go`).
- **Build System:** Includes a build script (`bin/build.sh`).

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

The application is configured using `config.yaml`. Key settings likely include:

- Debug/Log level settings.
- Audio input device selection, sample rate, buffer size, FFT parameters.
- UDP transport settings (enable, target address, port).

Check `internal/config/yaml.go` for details on configuration options and potential environment variable overrides.

## Usage

### Running the Engine

Execute the compiled binary from the project root:

```sh
./build/app
```

It likely looks for `config.yaml` in the current directory by default. You might be able to specify a different config file via a command-line flag (check `main.go` or run `./build/app --help` if implemented).

### Listing Audio Devices

To see available audio input/output devices:

```sh
./build/app list
```

### Listening to UDP Output

If UDP transport is enabled in your config, you can use the provided listener script:

```sh
./bin/listen.sh
```

This script likely uses `nc` (netcat) to listen on the configured UDP port.

## Development

### Running Tests

Run unit tests using the standard Go tool:

```sh
go test ./...
```

Or use the build script:

```sh
./bin/build.sh --test
```

## License

MIT License

Copyright (c) 2025 [Your Name or Organization]

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
