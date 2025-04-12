#!/bin/bash

# Required
BUILD_NAME="grec"
BUILD_UUID="62a23e9c-1267-469d-b5e8-e00e57b3861a"

# Optional
# i.e BUILD_AWS_REGION="us-east-1"

# Metadata
BUILD_VERSION="${BUILD_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo 'v0.0.0')}"
BUILD_TIME="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
BUILD_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"

# Exit if required variables are not set
if [ -z "$BUILD_NAME" ] || [ -z "$BUILD_UUID" ]; then
    printf "\nError: BUILD_NAME, BUILD_UUID are required.\n\n"
    exit 1
fi

printf "\nBuilding build/${BUILD_NAME}:\n\n"
printf "    BUILD_NAME:        ${BUILD_NAME}\n"
printf "    BUILD_TIME:        ${BUILD_TIME}\n"
printf "    BUILD_COMMIT:      ${BUILD_COMMIT}\n"
printf "    BUILD_VERSION:     ${BUILD_VERSION}\n"
printf "    BUILD_UUID:        ${BUILD_UUID}\n\n"

mkdir -p build
go build -o build/${BUILD_NAME} \
  -ldflags "\
    -X audio/internal/build.buildName=${BUILD_NAME} \
    -X audio/internal/build.buildTime=${BUILD_TIME} \
    -X audio/internal/build.buildCommit=${BUILD_COMMIT} \
    -X audio/internal/build.buildVersion=${BUILD_VERSION} 
    -X audio/internal/build.buildUuid=${BUILD_UUID} \
  "

if [ $? -eq 0 ]; then
  printf "Build successful: $(du -h build/${BUILD_NAME})\n\n"
else
  printf "Build failed\n\n"
  exit 1
fi
