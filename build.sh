#!/usr/bin/env bash
set -e

# Format Go code
echo "Formatting Go code..."
go fmt ./...

# Vet Go code
echo "Vetting Go code..."
go vet ./...

# Build the binary
echo "Building PorTTY binary..."
go build -ldflags="-s -w" -o portty ./cmd/portty

echo "Build complete. Run ./portty to start the server."