#!/usr/bin/env bash
set -e

# ============================================================================
# SCRIPT-LEVEL VARIABLES
# ============================================================================

VERSION=$(git describe --tags --exact-match 2>/dev/null || echo "dev")

# ============================================================================
# UTILITY FUNCTIONS
# ============================================================================

format_go_code() {
    echo "Formatting Go code..."
    go fmt ./...
}

vet_go_code() {
    echo "Vetting Go code..."
    go vet ./...
}

build_binary() {
    echo "Building PorTTY binary..."
    go build -ldflags="-s -w" -o portty ./cmd/portty
}

create_release_archives() {
    echo "Creating release archives for $VERSION..."
    
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    if [[ "$ARCH" == "x86_64" ]]; then
        ARCH="amd64"
    elif [[ "$ARCH" == "aarch64" ]]; then
        ARCH="arm64"
    elif [[ "$ARCH" == "armv7l" ]]; then
        ARCH="arm"
    fi
    
    ARCHIVE_NAME="portty-${VERSION}-${OS}-${ARCH}.tar.gz"
    
    echo "Creating archive: $ARCHIVE_NAME"
    tar -czvf "$ARCHIVE_NAME" portty
    
    echo "Creating SHA256 checksum"
    sha256sum "$ARCHIVE_NAME" > "${ARCHIVE_NAME}.sha256"
    
    echo "Release artifacts created:"
    echo "- $ARCHIVE_NAME"
    echo "- ${ARCHIVE_NAME}.sha256"
}

# ============================================================================
# MAIN EXECUTION LOGIC
# ============================================================================

echo "Building PorTTY version: $VERSION"

format_go_code
vet_go_code
build_binary

echo "Build complete. Run './portty help' for usage information."

if [[ "$VERSION" != "dev" ]]; then
    create_release_archives
fi