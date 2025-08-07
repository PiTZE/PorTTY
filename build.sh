#!/usr/bin/env bash
set -e

# ============================================================================
# USAGE INFORMATION
# ============================================================================

show_help() {
    echo "PorTTY Build Script"
    echo ""
    echo "Usage: $0 [OPTION]"
    echo ""
    echo "Options:"
    echo "  (no args)  Build binary for current platform only"
    echo "  all        Build binaries for all supported platforms"
    echo "  release    Build all binaries and create release archives"
    echo "  help       Show this help message"
    echo ""
    echo "Supported platforms:"
    echo "  - Linux: amd64, arm64, arm, 386"
    echo "  - macOS: amd64, arm64"
    echo "  - Windows: amd64, arm64, 386"
    echo "  - FreeBSD: amd64, arm64"
    echo "  - OpenBSD: amd64"
    echo "  - NetBSD: amd64"
    echo ""
    echo "All binaries are statically linked for maximum compatibility."
}

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

setup_build_directories() {
    echo "Setting up build directories..."
    mkdir -p build/bin build/release
    echo "Build directories created: build/bin, build/release"
}

build_binary_for_platform() {
    local os=$1
    local arch=$2
    local binary_name="portty"
    local output_path="build/bin/portty-${os}-${arch}"
    
    # Add .exe extension for Windows
    if [ "$os" = "windows" ]; then
        binary_name="portty.exe"
        output_path="build/bin/portty-${os}-${arch}.exe"
    fi
    
    echo "Building PorTTY binary for ${os}/${arch} (static)..."
    CGO_ENABLED=0 GOOS=${os} GOARCH=${arch} go build -ldflags="-s -w -extldflags '-static'" -o "${output_path}" ./cmd/portty
    
    if [ -f "${output_path}" ]; then
        echo "✓ Built: ${output_path}"
    else
        echo "✗ Failed to build: ${output_path}"
        return 1
    fi
}

build_all_binaries() {
    echo "Building PorTTY binaries for all supported platforms..."
    
    setup_build_directories
    
    # Define supported platforms
    declare -a platforms=(
        "linux amd64"
        "linux arm64"
        "linux arm"
        "linux 386"
        "darwin amd64"
        "darwin arm64"
        "windows amd64"
        "windows arm64"
        "windows 386"
        "freebsd amd64"
        "freebsd arm64"
        "openbsd amd64"
        "netbsd amd64"
    )
    
    local build_count=0
    local total_platforms=${#platforms[@]}
    
    # Build for each platform
    for platform in "${platforms[@]}"; do
        read -r os arch <<< "$platform"
        if build_binary_for_platform "$os" "$arch"; then
            build_count=$((build_count + 1))
        fi
    done
    
    echo "Built $build_count/$total_platforms binaries successfully!"
    echo "Binaries located in: build/bin/"
}

build_single_binary() {
    echo "Building PorTTY binary for current platform (static)..."
    local os=$(go env GOOS)
    local arch=$(go env GOARCH)
    
    setup_build_directories
    
    local binary_name="portty"
    local output_path="build/bin/portty"
    
    if [ "$os" = "windows" ]; then
        binary_name="portty.exe"
        output_path="build/bin/portty.exe"
    fi
    
    CGO_ENABLED=0 go build -ldflags="-s -w -extldflags '-static'" -o "${output_path}" ./cmd/portty
    
    if [ -f "${output_path}" ]; then
        echo "✓ Built portty for ${os}/${arch}: ${output_path}"
        if [ "$os" != "windows" ]; then
            ln -sf "${output_path}" portty
            echo "✓ Created symlink: portty -> ${output_path}"
        fi
    else
        echo "✗ Failed to build portty for ${os}/${arch}"
        return 1
    fi
}

create_release_archives() {
    echo "Creating release archives for $VERSION..."
    
    if [ ! -d "build/bin" ]; then
        echo "Error: build/bin directory not found. Run build first."
        return 1
    fi
    
    local archive_count=0
    
    # Create archives for all built binaries
    for binary_path in build/bin/portty-*; do
        if [[ -f "$binary_path" && -x "$binary_path" ]]; then
            # Extract filename and platform info
            local binary_file=$(basename "$binary_path")
            local platform_info=${binary_file#portty-}
            # Remove .exe extension if present for archive naming
            platform_info=${platform_info%.exe}
            
            local archive_name="portty-${VERSION}-${platform_info}.tar.gz"
            local archive_path="build/release/${archive_name}"
            
            echo "Creating archive: $archive_name"
            
            # Create archive with the binary renamed to just 'portty' (or 'portty.exe' for Windows)
            local temp_dir=$(mktemp -d)
            local binary_name="portty"
            if [[ "$binary_file" == *.exe ]]; then
                binary_name="portty.exe"
            fi
            
            cp "$binary_path" "$temp_dir/$binary_name"
            tar -czf "$archive_path" -C "$temp_dir" "$binary_name"
            rm -rf "$temp_dir"
            
            # Create SHA256 checksum
            echo "Creating SHA256 checksum"
            (cd build/release && sha256sum "$archive_name" > "${archive_name}.sha256")
            
            echo "✓ Release artifact created: $archive_path"
            archive_count=$((archive_count + 1))
        fi
    done
    
    # Also create archive for the main portty binary if it exists
    if [[ -f "build/bin/portty" ]]; then
        local os=$(go env GOOS)
        local arch=$(go env GOARCH)
        local archive_name="portty-${VERSION}-${os}-${arch}.tar.gz"
        local archive_path="build/release/${archive_name}"
        
        echo "Creating archive for current platform: $archive_name"
        
        local temp_dir=$(mktemp -d)
        cp "build/bin/portty" "$temp_dir/portty"
        tar -czf "$archive_path" -C "$temp_dir" "portty"
        rm -rf "$temp_dir"
        
        echo "Creating SHA256 checksum"
        (cd build/release && sha256sum "$archive_name" > "${archive_name}.sha256")
        
        echo "✓ Release artifact created: $archive_path"
        archive_count=$((archive_count + 1))
    fi
    
    echo "Created $archive_count release archives in: build/release/"
    
    # Create a combined checksums file
    if [ $archive_count -gt 0 ]; then
        echo "Creating combined checksums file..."
        (cd build/release && cat *.sha256 > checksums.txt)
        echo "✓ Combined checksums: build/release/checksums.txt"
    fi
}

# ============================================================================
# MAIN EXECUTION LOGIC
# ============================================================================

# Check for help option first
if [[ "$1" == "help" || "$1" == "-h" || "$1" == "--help" ]]; then
    show_help
    exit 0
fi

echo "Building PorTTY version: $VERSION"

format_go_code
vet_go_code

# Check command line arguments
if [[ "$1" == "all" ]]; then
    build_all_binaries
    echo "Multi-platform build complete. Binaries created for all supported architectures."
elif [[ "$1" == "release" ]]; then
    build_all_binaries
    create_release_archives
    echo "Release build complete with archives for all platforms."
else
    build_single_binary
    create_release_archives
    echo "Build complete. Run './portty help' for usage information."
fi