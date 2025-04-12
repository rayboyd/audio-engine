# GREC-V2

## Overview

GREC-V2 is a real-time audio processing engine designed for high-performance applications. It features:

- Real-time FFT analysis using the Gonum library
- Noise gate with branchless implementation
- WAV recording with atomic state management
- WebSocket transport for FFT visualization

## Project Structure

- `cmd/`: Command-line interface for the engine
- `internal/audio/`: Core audio engine implementation
- `internal/fft/`: FFT processing and transport logic
- `pkg/`: Utility packages

## Running the Project

To build and run the project:

```bash
./build.sh
./build/grec
```

## Testing

To run all tests, benchmarks, and memory profiling:

```bash
./test.sh
```

### Key Tests

- **Hot Path Allocation Tests**: Ensures zero allocations in performance-critical paths.
- **FFT Processing Tests**: Verifies FFT correctness and allocation-free behavior.

## Benchmarks

Run benchmarks to measure performance:

```bash
go test -bench=. -benchmem ./...
```

## Memory Profiling

Generate memory profiles to analyze allocations:

```bash
go test -memprofile=mem.prof -run=TestFFTHotPath ./internal/fft
go tool pprof -alloc_space mem.prof
```

## License

This project is licensed under the MIT License.
