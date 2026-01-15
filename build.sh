#!/bin/bash

# Usage: ./build <GOOS> <GOARCH> <VERSION>

set -e

# Check for correct number of arguments
if [ $# -ne 3 ]; then
    echo "Usage: $0 <GOOS> <GOARCH> <VERSION>"
    echo "Example: $0 linux amd64 v1.0.0"
    echo "Example: $0 darwin arm64 v1.0.0"
    echo "Available architectures: amd64, arm64, arm, mips64, mips64le, mips, mipsle, ppc64, ppc64le, s390x, riscv64"
    echo "Available operating systems: linux, darwin, windows"
    exit 1
fi

GOOS=$1
GOARCH=$2
VERSION=$3

# Build frontend dashboard
echo "Building frontend dashboard..."
if command -v bun &> /dev/null; then
    (cd frontend && bun install && bun run build)
elif command -v npm &> /dev/null; then
    (cd frontend && npm ci && npm run build)
else
    echo "Warning: Neither bun nor npm found, skipping frontend build"
fi

# Copy frontend build to web/dist for embedding
if [ -d "frontend/dist" ]; then
    echo "Copying frontend assets to web/dist..."
    rm -rf web/dist
    cp -r frontend/dist web/dist
else
    echo "Warning: frontend/dist not found, using placeholder"
fi

# Determine if the target is Windows for executable extension
OUTPUT_NAME="ollama-auto-ctx-${GOOS}-${GOARCH}-${VERSION}"
if [ "$GOOS" = "windows" ]; then
    OUTPUT_NAME+=".exe"
fi

# Define the output directory
BUILD_DIR="build/${GOOS}-${GOARCH}"
mkdir -p "$BUILD_DIR"

# Set environment variables for cross-compilation
export GOOS=$GOOS
export GOARCH=$GOARCH
export CGO_ENABLED=0

# For MIPS64 variants, you might need to set GOMIPS. Adjust as needed.
if [[ "$GOARCH" == "mips64le" || "$GOARCH" == "mips64" ]]; then
    export GOMIPS=softfloat
fi

# Build the binary with version information
# Note: This project doesn't currently have a version variable in main.go
# If you add one later, uncomment the -ldflags line
go build -o "$BUILD_DIR/$OUTPUT_NAME" ./cmd/ollama-auto-ctx

echo "Built $OUTPUT_NAME for $GOOS/$GOARCH at $BUILD_DIR/$OUTPUT_NAME"
