# ProxyRouter Build System

This directory contains the multi-platform build system for ProxyRouter.

## Directory Structure

```
builds/
├── build-all.sh           # Master build script
├── build-macos-arm64.sh   # macOS ARM64 (Apple Silicon) build
├── build-macos-amd64.sh   # macOS AMD64 (Intel) build
├── build-windows-amd64.sh # Windows AMD64 build
├── build-linux-amd64.sh   # Linux AMD64 build
├── build-linux-arm64.sh   # Linux ARM64 build
├── build-docker.sh        # Docker build files preparation
├── README.md              # This file
├── macos-arm64/           # macOS ARM64 builds
├── macos-amd64/           # macOS AMD64 builds
├── windows-amd64/         # Windows AMD64 builds
├── linux-amd64/           # Linux AMD64 builds
├── linux-arm64/           # Linux ARM64 builds
└── docker/                # Docker build files
```

## Quick Start

### Build All Platforms
```bash
# From the project root
make builds

# Or directly
./builds/build-all.sh all
```

### Build Specific Platform
```bash
# macOS (ARM64 + AMD64)
./builds/build-all.sh macos

# Linux (AMD64 + ARM64)
./builds/build-all.sh linux

# Windows AMD64
./builds/build-all.sh windows

# Docker files only
./builds/build-all.sh docker
```

### Build Individual Platform
```bash
# macOS ARM64 only
./builds/build-all.sh macos-arm64

# macOS AMD64 only
./builds/build-all.sh macos-amd64

# Linux AMD64 only
./builds/build-all.sh linux-amd64

# Linux ARM64 only
./builds/build-all.sh linux-arm64
```

## Build Outputs

Each platform build creates a directory with:

- **Binary**: Platform-specific executable
- **Configuration**: `config.yaml` file
- **Migrations**: SQL migration files
- **README**: Platform-specific instructions
- **Systemd service** (Linux only): Service file for systemd

### Platform-Specific Details

#### macOS Builds
- **ARM64**: Optimized for Apple Silicon (M1, M2, etc.)
- **AMD64**: Optimized for Intel Macs
- Both include native macOS binaries

#### Windows Builds
- **AMD64**: Windows executable (`.exe`)
- Includes Windows service setup instructions
- Compatible with Windows 10+

#### Linux Builds
- **AMD64**: Standard x86_64 Linux
- **ARM64**: ARM64 Linux (Raspberry Pi 4, ARM servers)
- Both include systemd service files

#### Docker Builds
- Multi-architecture Dockerfile
- Docker Compose configurations
- Testing scripts and documentation

## Build Requirements

### Prerequisites
- Go 1.22 or later
- Cross-compilation support (for non-native builds)
- Docker (for Docker builds)

### Platform-Specific Requirements

#### macOS
- macOS 10.15+ for building
- Xcode Command Line Tools

#### Windows
- Cross-compilation from Linux/macOS
- No Windows machine required

#### Linux
- Cross-compilation from any platform
- No Linux machine required

#### Docker
- Docker Desktop or Docker Engine
- Docker Compose

## Build Process

1. **Dependencies**: Each build script ensures dependencies are available
2. **Cross-compilation**: Uses Go's built-in cross-compilation
3. **Static linking**: All binaries are statically linked (no external dependencies)
4. **Configuration**: Copies relevant configuration files
5. **Documentation**: Generates platform-specific README files

## Build Artifacts

### Binary Sizes (Approximate)
- **macOS ARM64**: ~15MB
- **macOS AMD64**: ~15MB
- **Windows AMD64**: ~15MB
- **Linux AMD64**: ~15MB
- **Linux ARM64**: ~15MB

### Included Files
- Executable binary
- `config.yaml` (default configuration)
- `migrations/` (database schema)
- `README.md` (platform-specific instructions)
- `proxyrouter.service` (Linux systemd service)

## Testing Builds

### Local Testing
```bash
# Test macOS build (if on macOS)
./builds/macos-arm64/proxyrouter -version

# Test Linux build
./builds/linux-amd64/proxyrouter -version

# Test Windows build (on Windows)
./builds/windows-amd64/proxyrouter.exe -version
```

### Docker Testing
```bash
cd builds/docker
chmod +x test-docker.sh
./test-docker.sh start
```

## Distribution

### Release Packages
Each platform directory can be packaged for distribution:

```bash
# Create release packages
cd builds
tar -czf proxyrouter-macos-arm64.tar.gz macos-arm64/
tar -czf proxyrouter-macos-amd64.tar.gz macos-amd64/
tar -czf proxyrouter-linux-amd64.tar.gz linux-amd64/
tar -czf proxyrouter-linux-arm64.tar.gz linux-arm64/
zip -r proxyrouter-windows-amd64.zip windows-amd64/
```

### Docker Distribution
```bash
# Build and push Docker images
cd builds/docker
docker build -t proxyrouter:latest .
docker push proxyrouter:latest
```

## CI/CD Integration

### GitHub Actions Example
```yaml
- name: Build all platforms
  run: |
    make builds
    ./builds/build-all.sh summary

- name: Upload artifacts
  uses: actions/upload-artifact@v3
  with:
    name: proxyrouter-builds
    path: builds/*/
```

## Troubleshooting

### Common Issues

1. **Permission Denied**
   ```bash
   chmod +x builds/*.sh
   ```

2. **Go Version Issues**
   ```bash
   go version  # Ensure Go 1.22+
   ```

3. **Cross-compilation Issues**
   ```bash
   go env GOOS GOARCH  # Check current platform
   ```

4. **Docker Build Issues**
   ```bash
   docker --version  # Ensure Docker is installed
   docker buildx ls  # Check buildx support
   ```

### Build Verification
```bash
# Check all builds
./builds/build-all.sh summary

# Verify binary compatibility
file builds/*/proxyrouter*
```

## Maintenance

### Updating Build Scripts
- Modify individual platform scripts in `builds/`
- Update `build-all.sh` for new platforms
- Test builds on target platforms

### Adding New Platforms
1. Create new build script: `build-{platform}-{arch}.sh`
2. Add to `build-all.sh` master script
3. Update this README
4. Test the build process

### Cleaning Builds
```bash
# Clean all builds
./builds/build-all.sh clean

# Or via Makefile
make clean-builds
```
