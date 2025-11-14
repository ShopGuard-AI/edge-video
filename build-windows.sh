#!/bin/bash
# Build script for testing Windows binaries on Linux using cross-compilation

set -e

echo "Building Edge Video for Windows..."

# Set build variables
export GOOS=windows
export GOARCH=amd64
export CGO_ENABLED=0

VERSION=${VERSION:-"1.2.0-dev"}
BUILD_DIR="dist"

# Create build directory
mkdir -p ${BUILD_DIR}

echo "Building Windows service binary..."
go build -ldflags="-s -w -X main.version=${VERSION}" \
    -o "${BUILD_DIR}/edge-video-service.exe" \
    ./cmd/edge-video-service

echo "Building Windows CLI binary..."  
go build -ldflags="-s -w -X main.version=${VERSION}" \
    -o "${BUILD_DIR}/edge-video.exe" \
    ./cmd/edge-video

echo "Copying configuration files..."
mkdir -p ${BUILD_DIR}/config
cp config.toml ${BUILD_DIR}/config/

if [ -d "docs/windows" ]; then
    echo "Copying Windows documentation..."
    cp -r docs/windows ${BUILD_DIR}/
fi

echo "Build completed!"
echo "Binaries available in: ${BUILD_DIR}/"
echo ""
echo "Windows Service binary: ${BUILD_DIR}/edge-video-service.exe"
echo "Windows CLI binary: ${BUILD_DIR}/edge-video.exe"
echo ""
echo "To test the service on Windows:"
echo "1. Copy the dist/ folder to a Windows machine"
echo "2. Run as Administrator: edge-video-service.exe install"
echo "3. Start service: edge-video-service.exe start"
echo "4. Configure cameras in config/config.toml"
echo "5. Restart service: edge-video-service.exe stop && edge-video-service.exe start"