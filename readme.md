# Audio Engine

![Version](https://img.shields.io/badge/version-0.1.0-red)
![Go Version](https://img.shields.io/badge/Go-1.24.0+-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-MIT-green)

A high-performance, real-time audio analysis engine written in Go that captures audio input, performs Fast Fourier Transform (FFT) analysis, and streams frequency spectrum data over WebSocket. The engine is designed for low-latency audio processing with zero-allocation hot paths and thread-safe concurrent operations.

## Quick Start

```bash
# Clone the repository
git clone https://github.com/rayboyd/audio-engine
cd grec-v2

# Build the application
./bin/build.sh

# Run with visualization
./build/app --tui

# Open html/index.html in your browser to see real-time visualization
```

## Key Features

- Real-time audio capture with configurable device selection and buffer sizes
- Zero-allocation FFT processing in the audio hot path
- Lock-free audio processing with atomic state management
- Branchless noise gate implementation for efficient signal conditioning
- Real-time WebSocket streaming of FFT data with rate limiting
- Optional WAV recording with pre-allocated buffers
- Web-based spectrum visualization using HTML5 Canvas
- Terminal UI mode for system monitoring

## Screenshots

<!-- Consider adding screenshots here:
<p align="center">
  <img src="docs/assets/spectrum.png" width="600" alt="Spectrum Analyzer">
  <br>
  <em>Real-time audio spectrum analyzer</em>
</p>

<p align="center">
  <img src="docs/assets/terminal.png" width="600" alt="Terminal UI">
  <br>
  <em>Terminal UI showing system performance</em>
</p>
-->

## Technical Highlights

- **Low Latency Processing**: Direct memory-mapped audio capture using PortAudio
- **Memory Efficiency**: Pre-allocated buffers and zero-allocation processing chains
- **Thread Safety**: Atomic operations for state management, mutex-protected WebSocket broadcasts
- **Performance Optimizations**:
  - Branchless signal processing implementation
  - OS thread locking for audio callbacks
  - Rate-limited WebSocket broadcasts
  - Power-of-2 optimized FFT sizes

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
git clone https://github.com/rayboyd/audio-engine
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

## Troubleshooting

### Common Issues

#### No Audio Devices Found

- Ensure your audio devices are properly connected
- Try running with elevated privileges (`sudo ./build/app` on Linux/macOS)
- Verify PortAudio is properly installed on your system

#### Visualization Not Working

- Check that WebSocket connection is not blocked by firewall
- Verify you're accessing `html/index.html` while the application is running
- Check browser console for JavaScript errors

#### High CPU Usage

- Try increasing the buffer size with `--buffer-size=4096`
- Reduce FFT size with `--fft-size=1024`
- Limit update rate with `--rate-limit=30`

#### Poor Audio Quality

- Ensure the correct input device is selected
- Try different sample rates with `--sample-rate=48000`
- Adjust the noise gate threshold with `--noise-gate=0.02`

### FAQ

**Q: Can I use this for live performances?**  
A: Yes, the engine is designed for low-latency processing, but test thoroughly with your specific hardware setup first.

**Q: How do I record longer audio sessions?**  
A: Use the `--record` flag with `--output=filename.wav`. Ensure you have sufficient disk space.

**Q: Is it possible to process multiple audio inputs simultaneously?**  
A: Currently, the engine supports a single audio input. Multi-channel support is on the roadmap.

**Q: How can I customize the visualization?**  
A: Edit the HTML/CSS/JavaScript in the `html/index.html` file to match your requirements.
