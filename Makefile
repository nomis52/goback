# PBS Backup Automation - Simplified Makefile

# Variables
APP_NAME := goback
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS := -ldflags="-s -w -X github.com/nomis52/goback/buildinfo.buildTime=$(BUILD_TIME) -X github.com/nomis52/goback/buildinfo.gitCommit=$(GIT_COMMIT)"

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

# Install goback-server binary and systemd service (requires sudo)
.PHONY: daemon-install
daemon-install: build-server
	@echo "Installing $(APP_NAME)-server..."
	@# Check goback user exists
	@if ! id -u goback >/dev/null 2>&1; then \
		echo "Error: goback user does not exist"; \
		echo "Run: sudo useradd -r -s /sbin/nologin goback"; \
		exit 1; \
	fi
	@# Get commit sha from binary
	$(eval SHA := $(shell $(BUILD_DIR)/$(APP_NAME)-server --version | grep Commit | awk '{print $$2}'))
	@echo "Installing version: $(SHA)"
	@# Create bin directory if needed
	sudo mkdir -p $(INSTALL_DIR)/bin
	@# Copy binary with sha suffix
	sudo cp $(BUILD_DIR)/$(APP_NAME)-server $(INSTALL_DIR)/bin/$(APP_NAME)-server.$(SHA)
	sudo chmod +x $(INSTALL_DIR)/bin/$(APP_NAME)-server.$(SHA)
	@# Create/update symlink
	sudo ln -sf $(APP_NAME)-server.$(SHA) $(INSTALL_DIR)/bin/$(APP_NAME)-server
	sudo chown -R goback:goback $(INSTALL_DIR)/bin
	@# Install systemd service
	sudo cp systemd/$(APP_NAME)-server.service /etc/systemd/system/
	sudo systemctl daemon-reload
	@echo ""
	@echo "Installed $(INSTALL_DIR)/bin/$(APP_NAME)-server.$(SHA)"
	@echo ""
	@echo "Next steps:"
	@echo "  sudo systemctl enable $(APP_NAME)-server"
	@echo "  sudo systemctl restart $(APP_NAME)-server"

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
