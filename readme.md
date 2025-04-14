# Audio Engine

## Overview

Audio Engine is a high-performance, real-time audio analysis tool written in Go. It captures audio input, performs Fast Fourier Transform (FFT) analysis, and streams frequency spectrum data over WebSocket. Designed for low-latency audio processing, it features zero-allocation hot paths and thread-safe operations.

## Platform/Requirements

- **Platform**: Currently tested only on macOS. Not yet built or tested on Windows.
- **Requirements**:
  - Go 1.24.0 or later
  - PortAudio (for audio input/output)

## Installation

### Prerequisites

1. Install **Homebrew** (if not already installed):

   ```bash
   /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
   ```

2. Install PortAudio and pkg-config via Homebrew:

   ```bash
   brew install portaudio pkg-config
   ```

3. Install Go (if not already installed):
   ```bash
   brew install go
   ```

### Build and Run

1. Clone the repository:

   ```bash
   git clone https://github.com/rayboyd/audio-engine
   cd grec-v2
   ```

2. Install Go dependencies:

   ```bash
   go mod download
   ```

3. Build the application:

   ```bash
   ./bin/build.sh
   ```

4. Run the application:

   ```bash
   ./build/app
   ```

5. Open the visualization in your browser:
   ```
   Open `html/index.html` in your browser to see the real-time spectrum analyzer.
   ```
