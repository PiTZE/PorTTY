#!/usr/bin/env bash
set -e

# Get version from git tag or default to "dev"
VERSION=$(git describe --tags --exact-match 2>/dev/null || echo "dev")
echo "Building PorTTY version: $VERSION"

# Format Go code
echo "Formatting Go code..."
go fmt ./...

# Vet Go code
echo "Vetting Go code..."
go vet ./...

# Build the binary
echo "Building PorTTY binary..."
go build -ldflags="-s -w" -o portty ./cmd/portty

echo "Build complete. Run './portty help' for usage information."

# Create release archives if this is a tagged version
if [[ "$VERSION" != "dev" ]]; then
    echo "Creating release archives for $VERSION..."
    
    # Determine OS and architecture
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    # Map architecture names
    if [[ "$ARCH" == "x86_64" ]]; then
        ARCH="amd64"
    elif [[ "$ARCH" == "aarch64" ]]; then
        ARCH="arm64"
    elif [[ "$ARCH" == "armv7l" ]]; then
        ARCH="arm"
    fi
    
    # Create archive name
    ARCHIVE_NAME="portty-${VERSION}-${OS}-${ARCH}.tar.gz"
    
    # Create tar.gz archive
    echo "Creating archive: $ARCHIVE_NAME"
    tar -czvf "$ARCHIVE_NAME" portty
    
    # Create SHA256 checksum
    echo "Creating SHA256 checksum"
    sha256sum "$ARCHIVE_NAME" > "${ARCHIVE_NAME}.sha256"
    
    echo "Release artifacts created:"
    echo "- $ARCHIVE_NAME"
    echo "- ${ARCHIVE_NAME}.sha256"
fi