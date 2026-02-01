# AGENTS.md

This file provides guidance to agents when working with code in this repository.

## Project Overview

Goback is a PBS (Proxmox Backup Server) backup automation system written in Go. It orchestrates the backup workflow: powering on the PBS server via IPMI, backing up VMs/LXCs from Proxmox and file-based backups via SSH, then powering down PBS to save energy. It includes both a CLI tool for one-time runs and an HTTP server with web UI for monitoring and scheduling.

## Build and Test Commands

```bash
# Build all binaries
make build  # Uses git tags for version

# Build individual binaries
go build -o build/goback ./cmd/cli
go build -o build/goback-server ./cmd/server
go build -o build/goback-poweroff ./cmd/power_off

# Run all tests
make test

or

go test ./...

# Run tests for a specific package
go test ./orchestrator
go test -v ./server/handlers

# Run a single test
go test -v -run TestBackupVMs ./workflows/backup

# Format code
make fmt

# Clean build artifacts
make clean
```

## Running the Application

```bash
# CLI mode (one-time backup run)
./build/goback --config cfg/test.yaml

# Server mode with web UI (takes server config, not workflow config)
./build/goback-server --config cfg/test.yaml

# Or run directly with go
go run ./cmd/server --config cfg/test.yaml

# Power-off utility (testing/manual)
./build/goback-poweroff --config cfg/test.yaml

# Validate configuration
./build/goback --config cfg/test.yaml --validate

# Show version
./build/goback --version
```

The server uses a separate config file that references the workflow config. See `cfg/test.yaml` for an example. Server config includes:
- `listener.addr` - HTTP listen address (default `:8080`)
- `listener.tls_cert` / `listener.tls_key` - Optional TLS
- `cron` - List of scheduled workflow triggers
- `state_dir` - Directory for run history persistence
- `log_level` - Server log level
- `workflow_config` - Path to the workflow config file

## Architecture

### Core Design Pattern: Activity Orchestration

The system uses a custom **orchestrator** pattern (in `orchestrator/` package) that manages dependency-resolved execution of activities. This is the heart of the application.

**Key concepts:**

1. **Activities** implement `Activity` interface with `Init()` and `Execute(ctx)` methods
2. **Dependency injection** happens automatically via struct fields:
   - Named fields (e.g., `PowerOnPBS *PowerOnPBS`) provide both ordering AND access
   - Unnamed fields (e.g., `_ *SomeActivity`) provide ordering only
3. **Configuration injection** via struct tags (e.g., `config:"pbs.host"`)
4. **Result tracking** is available immediately after `AddActivity()` and persists after `Execute()`

**State progression:** `NotStarted → Pending → Running → (Completed|Skipped)`

The orchestrator doc.go (orchestrator/doc.go:1) contains comprehensive documentation on this pattern.

### Two Execution Modes

1. **CLI Mode** (`cmd/cli/main.go`): Creates orchestrator, adds activities (PowerOnPBS → BackupDirs → BackupVMs → PowerOffPBS), executes once, exits
2. **Server Mode** (`cmd/server/main.go`): HTTP server with REST API and web UI for triggering runs, viewing status/history, optional cron scheduling

### Server Architecture

The server (server/server.go:1) maintains two dependency sets:

- **Server-level deps** (swapped atomically on reload): config, IPMI controller for `/ipmi` endpoint
- **Run-level deps** (created fresh per backup run): IPMI, PBS client, Proxmox client, metrics client

This separation ensures config changes take effect on next run without interrupting in-progress backups.

The **runner** (server/runner/) prevents concurrent runs, tracks current status, maintains history (default: last 100 runs).

### Key Packages

- `orchestrator/` - Dependency-resolved activity execution engine
- `workflows/backup/` - Backup workflow and activities (PowerOnPBS, BackupVMs, BackupDirs)
- `workflows/poweroff/` - Power-off workflow and activity (PowerOffPBS)
- `config/` - YAML configuration loading and validation
- `clients/ipmiclient/` - IPMI controller for power management
- `clients/pbsclient/` - PBS API client
- `clients/proxmoxclient/` - Proxmox VE API client for backup operations
- `clients/sshclient/` - SSH client for file-based backups
- `metrics/` - VictoriaMetrics/Prometheus metric pushing
- `logging/` - Structured logging (slog) with configurable output
- `activity/` - Activity status reporting with CaptureError helper
- `server/` - HTTP server, handlers, runner, cron trigger
- `server/handlers/` - HTTP endpoint handlers (one file per endpoint)
- `server/runner/` - Backup run execution and state management
- `server/cron/` - Cron-based scheduling trigger
- `server/static/` - Embedded web UI (single-page HTML with inline CSS/JS)

## Code Style Requirements

**Follow the Uber Go Style Guide:** https://github.com/uber-go/guide/blob/master/style.md

### Critical Rules (from .cursor/INSTRUCTIONS.md)

1. **Time values must be constants** - Define all durations/timeouts as named constants
2. **Pre-size maps when size is known** - Use `make(map[K]V, size)` for better performance
3. **Always use testify** for assertions - Use `require` for critical checks, `assert` for non-critical
4. **Program structure:**
   - `main()` only calls `run()` and handles errors
   - `run()` calls `parseArgs()` first, contains all logic
   - `parseArgs()` returns an `Args` struct with all parsed flags
5. **Test files in same directory as code** - Never separate test files into different directories
6. **Directory name matches package name** - Exception: `main` package in `cmd/*/`
7. **All documentation in godoc comments** - Never create separate .md files for API docs (exception: READMEs for overview/setup)
8. **Client packages:**
   - Located in `clients/` directory
   - Named `fooclient` (e.g., `clients/pbsclient`, `clients/proxmoxclient`, `clients/ipmiclient`)
   - Main type is `Client`, constructor is `New()`
   - Separate `types.go` for return types (public types first, internal types at end)
9. **Use Options pattern** for constructors with multiple parameters (e.g., `WithTimeout()`, `WithLogger()`)
10. **Use standard library constants** instead of strings (e.g., `http.MethodPost` not `"POST"`)

## Configuration

Configuration is in YAML format. See config.yaml for example. Key sections:

- `pbs` - PBS server host, IPMI credentials, timeouts
- `proxmox` - Proxmox host, API token, storage name, backup timeout
- `compute` - VM/LXC backup settings (max age, mode, compression)
- `files` - SSH-based file backup configuration
- `monitoring` - VictoriaMetrics URL and metric naming
- `logging` - Log level, format (json/text), output (stdout/stderr/file)

The config package (config/config.go:1) provides validation, defaults, and loading.

## Testing Strategy

- Use testify for all assertions (`require` for critical, `assert` for non-critical)
- Place test files in same directory as implementation
- Test files use same package name as code they test
- Mock interfaces for unit testing (e.g., `handlers/interfaces.go` defines testable interfaces)
- **Table-driven tests:** Use a single `wantErr string` field instead of a boolean `wantErr` and a separate string for the error message. If `wantErr` is non-empty, an error is expected and its message must contain that string.
- **No Sleeps:** Never use `time.Sleep` in unit tests as they cause flakiness.
- **Mock Domains:** Use RFC 6761 reserved domains (e.g., `.test`, `.example`, `.invalid`, `.localhost`) for mock hostnames in tests.

## Common Patterns

### Adding a New Activity

1. Create struct in appropriate workflow package (e.g., `workflows/backup/`) with dependencies and config tags
2. Implement `Init()` for structural validation (check required fields, validate config)
3. Implement `Execute(ctx)` for actual work (check runtime state of dependencies)
4. Dependencies are auto-injected via struct fields (pointer to other activity types)
5. Add to workflow in the workflow's `NewWorkflow()` factory function via `AddActivity()`
6. Use `activity.CaptureError()` helper to wrap Execute logic for error status reporting

### Adding a New Client Package

1. Create directory in `clients/` matching package name (e.g., `clients/fooclient/`)
2. Create `client.go` with `Client` type and `New()` constructor
3. Create `types.go` for return types (public first, internal last)
4. Use Options pattern for constructor configuration
5. Document package, types, and methods with godoc comments

### Adding HTTP Endpoints

1. Create handler in `server/handlers/` (one file per endpoint)
2. Implement handler with proper error handling and JSON responses
3. Register route in `server/server.go` `registerRoutes()`
4. Add tests in `server/handlers/*_test.go`

## Dependencies

- Go 1.23+
- Prometheus client for metrics
- robfig/cron for scheduling
- testify for testing
- golang.org/x/crypto for SSH
- gopkg.in/yaml.v3 for config parsing

## Important Implementation Details

- **Concurrent backups:** BackupVMs runs backups concurrently using goroutines and wait groups
- **Retry logic:** PBS storage access retries with backoff (6 attempts, 5s interval) since PBS may not be ready immediately after power-on
- **Context handling:** All operations respect context cancellation for graceful shutdown
- **Thread safety:** Orchestrator methods and server operations are thread-safe via atomic operations
- **Static files:** Web UI embedded via `//go:embed` directive
- **Version injection:** Build sets version/commit/time via ldflags in Makefile
