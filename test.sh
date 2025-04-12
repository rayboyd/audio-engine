#!/bin/bash

# Run all tests in the project
set -e

printf "\nRunning all tests...\n\n"
go test -v -race ./...

printf "\nRunning benchmarks...\n\n"
go test -bench=. -benchmem -race ./...

printf "\nRunning memory profiling...\n\n"
go test -memprofile=mem.prof -run=TestFFTHotPath ./internal/fft

printf "\nAll tests and benchmarks completed successfully.\n"
