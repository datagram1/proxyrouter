#!/bin/bash

# Master build script for all platforms
set -e

echo "🚀 ProxyRouter Multi-Platform Build Script"
echo "=========================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Build functions
build_macos_arm64() {
    print_status "Building macOS ARM64..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        bash "$SCRIPT_DIR/build-macos-arm64.sh"
        print_success "macOS ARM64 build completed"
    else
        print_warning "Skipping macOS ARM64 build (not on macOS)"
    fi
}

build_macos_amd64() {
    print_status "Building macOS AMD64..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        bash "$SCRIPT_DIR/build-macos-amd64.sh"
        print_success "macOS AMD64 build completed"
    else
        print_warning "Skipping macOS AMD64 build (not on macOS)"
    fi
}

build_windows_amd64() {
    print_status "Building Windows AMD64..."
    bash "$SCRIPT_DIR/build-windows-amd64.sh"
    print_success "Windows AMD64 build completed"
}

build_linux_amd64() {
    print_status "Building Linux AMD64..."
    bash "$SCRIPT_DIR/build-linux-amd64.sh"
    print_success "Linux AMD64 build completed"
}

build_linux_arm64() {
    print_status "Building Linux ARM64..."
    bash "$SCRIPT_DIR/build-linux-arm64.sh"
    print_success "Linux ARM64 build completed"
}

build_docker() {
    print_status "Preparing Docker build files..."
    bash "$SCRIPT_DIR/build-docker.sh"
    print_success "Docker build files prepared"
}

# Show build summary
show_summary() {
    echo ""
    print_status "Build Summary:"
    echo "================"
    
    for platform in macos-arm64 macos-amd64 windows-amd64 linux-amd64 linux-arm64 docker; do
        if [[ -d "$SCRIPT_DIR/$platform" ]]; then
            if [[ "$platform" == "docker" ]]; then
                echo "✅ $platform - Docker files prepared"
            else
                binary_name="proxyrouter"
                if [[ "$platform" == "windows-amd64" ]]; then
                    binary_name="proxyrouter.exe"
                fi
                if [[ -f "$SCRIPT_DIR/$platform/$binary_name" ]]; then
                    size=$(du -h "$SCRIPT_DIR/$platform/$binary_name" | cut -f1)
                    echo "✅ $platform - $size"
                else
                    echo "❌ $platform - Build failed"
                fi
            fi
        else
            echo "⏭️  $platform - Skipped"
        fi
    done
    
    echo ""
    print_status "Build locations:"
    echo "  macOS ARM64:   $SCRIPT_DIR/macos-arm64/"
    echo "  macOS AMD64:   $SCRIPT_DIR/macos-amd64/"
    echo "  Windows AMD64: $SCRIPT_DIR/windows-amd64/"
    echo "  Linux AMD64:   $SCRIPT_DIR/linux-amd64/"
    echo "  Linux ARM64:   $SCRIPT_DIR/linux-arm64/"
    echo "  Docker:        $SCRIPT_DIR/docker/"
}

# Clean builds
clean_builds() {
    print_status "Cleaning all builds..."
    rm -rf "$SCRIPT_DIR"/{macos-arm64,macos-amd64,windows-amd64,linux-amd64,linux-arm64,docker}
    print_success "All builds cleaned"
}

# Main execution
main() {
    case "${1:-all}" in
        "all")
            print_status "Building all platforms..."
            build_macos_arm64
            build_macos_amd64
            build_windows_amd64
            build_linux_amd64
            build_linux_arm64
            build_docker
            show_summary
            ;;
        "macos")
            print_status "Building macOS platforms..."
            build_macos_arm64
            build_macos_amd64
            ;;
        "macos-arm64")
            build_macos_arm64
            ;;
        "macos-amd64")
            build_macos_amd64
            ;;
        "windows")
            build_windows_amd64
            ;;
        "linux")
            print_status "Building Linux platforms..."
            build_linux_amd64
            build_linux_arm64
            ;;
        "linux-amd64")
            build_linux_amd64
            ;;
        "linux-arm64")
            build_linux_arm64
            ;;
        "docker")
            build_docker
            ;;
        "clean")
            clean_builds
            ;;
        "summary")
            show_summary
            ;;
        *)
            echo "Usage: $0 {all|macos|macos-arm64|macos-amd64|windows|linux|linux-amd64|linux-arm64|docker|clean|summary}"
            echo ""
            echo "Build targets:"
            echo "  all         - Build all platforms"
            echo "  macos       - Build macOS ARM64 and AMD64"
            echo "  macos-arm64 - Build macOS ARM64 only"
            echo "  macos-amd64 - Build macOS AMD64 only"
            echo "  windows     - Build Windows AMD64"
            echo "  linux       - Build Linux AMD64 and ARM64"
            echo "  linux-amd64 - Build Linux AMD64 only"
            echo "  linux-arm64 - Build Linux ARM64 only"
            echo "  docker      - Prepare Docker build files"
            echo "  clean       - Clean all builds"
            echo "  summary     - Show build summary"
            exit 1
            ;;
    esac
}

# Run main function
main "$@"
