# PBS Backup Automation - Simplified Makefile

# Variables
APP_NAME := goback
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS := -ldflags="-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Directories
BUILD_DIR := build
INSTALL_DIR := /opt/goback

# Default target
.PHONY: all
all: clean test build

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BUILD_DIR)/$(APP_NAME) $(BUILD_DIR)/$(APP_NAME)-server $(BUILD_DIR)/$(APP_NAME)-poweroff
	go clean

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

# Build all binaries
.PHONY: build
build: build-cli build-server build-poweroff

# Build CLI binary
.PHONY: build-cli
build-cli:
	@echo "Building $(APP_NAME) CLI v$(VERSION)..."
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) ./cmd/cli

# Build server binary
.PHONY: build-server
build-server:
	@echo "Building $(APP_NAME) server v$(VERSION)..."
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-server ./cmd/server

# Build power-off binary
.PHONY: build-poweroff
build-poweroff:
	@echo "Building $(APP_NAME) power-off utility..."
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/$(APP_NAME)-poweroff ./cmd/power_off

# Install systemd service for goback-server daemon (requires sudo)
.PHONY: daemon-install
daemon-install:
	@echo "Installing $(APP_NAME)-server systemd service..."
	@# Check binary exists
	@if [ ! -f $(INSTALL_DIR)/bin/$(APP_NAME)-server ]; then \
		echo "Error: $(INSTALL_DIR)/bin/$(APP_NAME)-server not found"; \
		echo "Please copy the binary to $(INSTALL_DIR)/bin/ first"; \
		exit 1; \
	fi
	@# Check binary is executable
	@if [ ! -x $(INSTALL_DIR)/bin/$(APP_NAME)-server ]; then \
		echo "Error: $(INSTALL_DIR)/bin/$(APP_NAME)-server is not executable"; \
		echo "Run: sudo chmod +x $(INSTALL_DIR)/bin/$(APP_NAME)-server"; \
		exit 1; \
	fi
	@# Check goback user exists
	@if ! id -u goback >/dev/null 2>&1; then \
		echo "Error: goback user does not exist"; \
		echo "Run: sudo useradd -r -s /sbin/nologin goback"; \
		exit 1; \
	fi
	@# Install systemd service
	sudo cp systemd/$(APP_NAME)-server.service /etc/systemd/system/
	sudo systemctl daemon-reload
	@echo ""
	@echo "Service installed. Next steps:"
	@echo "  sudo systemctl enable $(APP_NAME)-server"
	@echo "  sudo systemctl start $(APP_NAME)-server"

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...
	go mod tidy

# Show build info
.PHONY: info
info:
	@echo "Build Information:"
	@echo "  App Name:    $(APP_NAME)"
	@echo "  Version:     $(VERSION)"
	@echo "  Build Time:  $(BUILD_TIME)"
	@echo "  Git Commit:  $(GIT_COMMIT)"

# Help
.PHONY: help
help:
	@echo "PBS Backup Automation - Available targets:"
	@echo ""
	@echo "Building:"
	@echo "  build          Build all binaries"
	@echo "  build-server   Build server binary only"
	@echo "  test           Run tests"
	@echo "  clean          Clean build artifacts"
	@echo ""
	@echo "Installation:"
	@echo "  daemon-install Install systemd service (requires sudo)"
	@echo ""
	@echo "Development:"
	@echo "  fmt            Format code and tidy modules"
	@echo ""
	@echo "Utilities:"
	@echo "  info           Show build information"
	@echo "  help           Show this help message"
