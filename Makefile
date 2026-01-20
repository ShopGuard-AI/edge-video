# Edge Video V2 - Makefile
# Professional build automation for Windows, Linux, and macOS

# Variables
BINARY_NAME_WIN=producer.exe
BINARY_NAME_LINUX=producer-linux
BINARY_NAME_MAC=producer-mac
VERSION=1.6.0
BUILD_DIR=build
CMD_DIR=cmd/producer
CONFIG_FILE=config.yaml
GO=go

# Build flags
LDFLAGS=-ldflags "-s -w"

.PHONY: all build clean test windows linux mac release help

# Default target
all: windows linux

# Help
help:
	@echo "Edge Video V2 - Build Commands"
	@echo ""
	@echo "Building:"
	@echo "  make windows        - Build for Windows (producer.exe)"
	@echo "  make linux          - Build for Linux (producer-linux)"
	@echo "  make mac            - Build for macOS (producer-mac)"
	@echo "  make all            - Build Windows + Linux"
	@echo "  make release        - Build all platforms + organize releases"
	@echo ""
	@echo "Testing:"
	@echo "  make test           - Run all tests"
	@echo "  make test-unit      - Run unit tests"
	@echo "  make test-integration - Run integration tests"
	@echo ""
	@echo "Utilities:"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make deps           - Download dependencies"
	@echo ""

# Build for Windows
windows:
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME_WIN) ./$(CMD_DIR)
	@echo "✓ Windows build complete: $(BINARY_NAME_WIN)"
	@ls -lh $(BINARY_NAME_WIN) 2>/dev/null || dir $(BINARY_NAME_WIN)

# Build for Linux
linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME_LINUX) ./$(CMD_DIR)
	@echo "✓ Linux build complete: $(BINARY_NAME_LINUX)"
	@ls -lh $(BINARY_NAME_LINUX) 2>/dev/null || echo "Built successfully"

# Build for macOS
mac:
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME_MAC) ./$(CMD_DIR)
	@echo "✓ macOS build complete: $(BINARY_NAME_MAC)"
	@ls -lh $(BINARY_NAME_MAC) 2>/dev/null || echo "Built successfully"

# Build release packages
release: clean all
	@echo "Creating release packages..."
	@mkdir -p $(BUILD_DIR)/windows $(BUILD_DIR)/linux $(BUILD_DIR)/mac

	# Windows release
	@cp $(BINARY_NAME_WIN) $(BUILD_DIR)/windows/
	@cp config.yaml $(BUILD_DIR)/windows/
	@cp setup.ps1 $(BUILD_DIR)/windows/
	@cp DEPLOY_WINDOWS.md $(BUILD_DIR)/windows/
	@cp README.md $(BUILD_DIR)/windows/
	@echo "✓ Windows release ready: $(BUILD_DIR)/windows/"

	# Linux release
	@cp $(BINARY_NAME_LINUX) $(BUILD_DIR)/linux/producer-linux
	@cp config.yaml $(BUILD_DIR)/linux/
	@cp setup-ubuntu.sh $(BUILD_DIR)/linux/
	@cp DEPLOY_UBUNTU.md $(BUILD_DIR)/linux/
	@cp README.md $(BUILD_DIR)/linux/
	@chmod +x $(BUILD_DIR)/linux/setup-ubuntu.sh
	@chmod +x $(BUILD_DIR)/linux/producer-linux
	@echo "✓ Linux release ready: $(BUILD_DIR)/linux/"

	@echo ""
	@echo "Release packages created in $(BUILD_DIR)/"
	@echo "  - Windows: $(BUILD_DIR)/windows/"
	@echo "  - Linux:   $(BUILD_DIR)/linux/"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_NAME_WIN) $(BINARY_NAME_LINUX) $(BINARY_NAME_MAC)
	@rm -rf $(BUILD_DIR)
	@echo "✓ Clean complete"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GO) mod download
	@echo "✓ Dependencies downloaded"

# Run tests
test:
	@echo "Running tests..."
	$(GO) test -v ./...

# Run unit tests
test-unit:
	@echo "Running unit tests..."
	$(GO) test -v -short ./...

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GO) test -v -run Integration ./tests/integration/...
