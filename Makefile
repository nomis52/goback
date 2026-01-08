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
CONFIG_DIR := /etc/$(APP_NAME)
BIN_DIR := /usr/local/bin
LOG_DIR := /var/log/$(APP_NAME)

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

# Install locally (requires sudo)
.PHONY: install
install: build
	@echo "Installing $(APP_NAME)..."
	sudo mkdir -p $(CONFIG_DIR) $(BIN_DIR) $(LOG_DIR)
	sudo cp $(BUILD_DIR)/$(APP_NAME) $(BIN_DIR)/
	sudo chmod +x $(BIN_DIR)/$(APP_NAME)
	
	# Copy config if it doesn't exist
	@if [ ! -f $(CONFIG_DIR)/config.yaml ]; then \
		echo "Installing example config..."; \
		sudo cp config.yaml $(CONFIG_DIR)/config.yaml.example; \
		echo "Please copy $(CONFIG_DIR)/config.yaml.example to $(CONFIG_DIR)/config.yaml and edit it"; \
	fi
	
	# Install systemd service files
	@if [ -d /etc/systemd/system ]; then \
		echo "Installing systemd service and timer..."; \
		sudo cp scripts/$(APP_NAME).service /etc/systemd/system/; \
		sudo cp scripts/$(APP_NAME).timer /etc/systemd/system/; \
		sudo systemctl daemon-reload; \
		echo ""; \
		echo "Installation complete! Next steps:"; \
		echo "  1. sudo cp $(CONFIG_DIR)/config.yaml.example $(CONFIG_DIR)/config.yaml"; \
		echo "  2. sudo nano $(CONFIG_DIR)/config.yaml"; \
		echo "  3. sudo systemctl enable $(APP_NAME).timer"; \
		echo "  4. sudo systemctl start $(APP_NAME).timer"; \
	fi

# Install as cron job (alternative to systemd)
.PHONY: install-cron
install-cron: install
	@echo "Setting up cron job..."
	@echo "# PBS Backup Automation - runs daily at 2 AM" | sudo tee /etc/cron.d/$(APP_NAME)
	@echo "0 2 * * * root $(BIN_DIR)/$(APP_NAME) --config $(CONFIG_DIR)/config.yaml >> $(LOG_DIR)/$(APP_NAME).log 2>&1" | sudo tee -a /etc/cron.d/$(APP_NAME)
	@echo "Cron job installed. Logs will be written to $(LOG_DIR)/$(APP_NAME).log"

# Uninstall
.PHONY: uninstall
uninstall:
	@echo "Uninstalling $(APP_NAME)..."
	sudo rm -f $(BIN_DIR)/$(APP_NAME)
	sudo rm -f /etc/systemd/system/$(APP_NAME).service
	sudo rm -f /etc/systemd/system/$(APP_NAME).timer
	sudo rm -f /etc/cron.d/$(APP_NAME)
	sudo systemctl daemon-reload 2>/dev/null || true
	@echo "$(APP_NAME) uninstalled. Config and logs preserved in $(CONFIG_DIR) and $(LOG_DIR)"

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
	@echo "  build         Build for current platform"
	@echo "  test          Run tests"
	@echo "  clean         Clean build artifacts"
	@echo ""
	@echo "Installation:"
	@echo "  install       Install locally with systemd (requires sudo)"
	@echo "  install-cron  Install with cron job instead of systemd"
	@echo "  uninstall     Remove installation"
	@echo ""
	@echo "Development:"
	@echo "  fmt           Format code and tidy modules"
	@echo ""
	@echo "Utilities:"
	@echo "  info          Show build information"
	@echo "  help          Show this help message"
