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

# Run the application
./build/app

# Open html/index.html in your browser to see real-time visualization
```

## Key Features

**roadmap**

## Screenshots

**roadmap**

## Technical Highlights

**roadmap**

## Requirements

- Go 1.24.0 or later
- PortAudio

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

## Development

### Project Structure

**roadmap**

### Building

The project includes a build script with various options:

```bash
./bin/build.sh         # Normal build
./bin/build.sh --test  # Build and run tests
./bin/build.sh --help  # Show build options
```

## Contributing

**roadmap**
