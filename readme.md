# Audio Engine

A real-time audio processing engine with FFT visualization capabilities, built in Go.

## Features

- Real-time audio input processing
- Fast Fourier Transform (FFT) analysis
- Web-based spectrum visualization
- Support for audio device selection
- Optional input stream recording
- Terminal UI mode
- WebSocket-based real-time data streaming

## Requirements

- Go 1.24.1 or later
- PortAudio (for audio capture)
- Web browser supporting WebSocket and Canvas (for visualization)

## Dependencies

- `github.com/go-audio/audio` & `github.com/go-audio/wav` - Audio file handling and WAV format support
- `github.com/gordonklaus/portaudio` - Cross-platform audio I/O library
- `github.com/gorilla/websocket` - WebSocket implementation for real-time data streaming
- `github.com/spf13/cobra` - Modern CLI application framework
- `gonum.org/v1/gonum` - Numerical computing library for FFT calculations

## Installation

1. Clone the repository:

```bash
git clone [repository-url]
cd grec-v2
```

2. Install dependencies:

```bash
go mod download
```

3. Build the project:

```bash
./bin/build.sh
```

## Usage

Run the application:

```bash
./build/app --help
```

### Common Commands

- Start with visualization: `./build/app --tui`
- List audio devices: `./build/app --list-devices`
- Record input: `./build/app --record --output=recording.wav`

## Web Visualization

When running in TUI mode, open `html/index.html` in your web browser to see the real-time spectrum analyzer. The visualization connects to the application via WebSocket on port 8080.

## Development

### Project Structure

- `/cmd` - Command-line interface and argument parsing
- `/internal`
  - `/audio` - Audio processing engine and device management
  - `/config` - Application configuration
  - `/fft` - FFT processing and WebSocket server
- `/pkg`
  - `/bitint` - Bit manipulation utilities
  - `/build` - Build information and flags
- `/html` - Web-based visualization interface

### Building

The project includes a build script with various options:

```bash
./bin/build.sh         # Normal build
./bin/build.sh --test  # Build and run tests
./bin/build.sh --help  # Show build options
```

## Contributing

Contributions are welcome! Here's how you can help:

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Before submitting a Pull Request, please:

- Ensure your code follows the existing style conventions
- Add/update tests as needed
- Update documentation as needed
- Verify all tests pass locally
- Include a clear description of the changes in your PR
- Link any related issues

For major changes, please open an issue first to discuss what you would like to change.

## Acknowledgments

Special thanks to the following projects that make this application possible:

- [PortAudio](https://github.com/gordonklaus/portaudio) - Cross-platform audio I/O library
- [Go Audio](https://github.com/go-audio/audio) - Pure Go audio package
- [Go WAV](https://github.com/go-audio/wav) - WAV encoder/decoder
- [Gorilla WebSocket](https://github.com/gorilla/websocket) - WebSocket implementation for Go
- [Cobra](https://github.com/spf13/cobra) - Modern CLI application framework
- [Gonum](https://github.com/gonum/gonum) - Numerical packages for Go

## License

MIT License

Copyright (c) 2025

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
