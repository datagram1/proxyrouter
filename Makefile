.PHONY: deps build run test docker clean

# Build configuration
BINARY_NAME=proxyrouter
BUILD_DIR=bin
MAIN_PATH=./cmd/proxyrouter

# Default target
all: deps build

# Install dependencies
deps:
	go mod tidy
	go mod download

# Build the binary (static, no CGO)
build:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

# Run the application
run: build
	./$(BUILD_DIR)/$(BINARY_NAME) -config configs/config.yaml

# Run tests
test:
	go test -v -race ./...

# Build Docker image
docker:
	docker build -t proxyrouter .

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	go clean -cache

# Install dependencies for development
dev-deps:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Lint code
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...

# Run with race detection
test-race:
	go test -race ./...

# Show help
help:
	@echo "Available targets:"
	@echo "  deps      - Install dependencies"
	@echo "  build     - Build static binary"
	@echo "  run       - Run the application"
	@echo "  test      - Run tests"
	@echo "  docker    - Build Docker image"
	@echo "  clean     - Clean build artifacts"
	@echo "  dev-deps  - Install development dependencies"
	@echo "  lint      - Run linter"
	@echo "  fmt       - Format code"
	@echo "  test-race - Run tests with race detection"
