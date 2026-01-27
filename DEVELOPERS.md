# DEVELOPERS.md

Developer documentation for the Goback PBS backup automation system.

## Package Structure

### Root-Level Directories

```
goback/
├── activity/           # Activity-scoped status reporting
├── buildinfo/          # Build-time metadata (version, commit)
├── clients/            # External service client packages
│   ├── ipmiclient/     # IPMI power management
│   ├── pbsclient/      # PBS HTTP API
│   ├── proxmoxclient/  # Proxmox VE HTTP API
│   └── sshclient/      # SSH for file-based backups
├── cmd/                # Executable entry points
│   ├── cli/            # One-time backup CLI tool
│   ├── power_off/      # Manual PBS shutdown utility
│   └── server/         # HTTP server with web UI
├── config/             # YAML configuration loading
├── logging/            # Structured logging (slog-based)
├── metrics/            # Prometheus/VictoriaMetrics integration
├── workflow/           # Core dependency-resolved execution engine (orchestrator)
├── server/             # HTTP server implementation
│   ├── config/         # Server-specific configuration
│   ├── cron/           # Cron-based scheduling
│   ├── handlers/       # HTTP endpoint handlers (one per file)
│   ├── runner/         # Run execution and state management
│   └── static/         # Embedded web UI
├── systemd/            # Systemd service definition
└── workflows/          # Application-specific workflows
    ├── backup/         # Backup workflow and activities
    ├── demo/           # Demo workflow
    └── poweroff/       # Power-off workflow
```

### Package Descriptions

#### Core Engine

| Package | Description |
|---------|-------------|
| `workflow/` | Dependency-resolved activity execution engine (orchestrator). The heart of the application. See `workflow/doc.go` for comprehensive documentation. |
| `activity/` | Activity-scoped status reporting with `CaptureError()` helper for wrapping Execute logic. |

#### Workflows

| Package | Description |
|---------|-------------|
| `workflows/` | Contains `Params` struct for workflow construction and dependency injection. |
| `workflows/backup/` | Backup workflow: PowerOnPBS → BackupDirs → BackupVMs activities. |
| `workflows/demo/` | Demo workflow for development/testing purposes. |
| `workflows/poweroff/` | Power-off workflow: PowerOffPBS activity. |

#### Client Packages

All clients follow the same pattern: main type is `Client`, constructor is `New()`, options pattern for configuration.

| Package | Description |
|---------|-------------|
| `clients/ipmiclient/` | IPMI controller using `ipmitool` command-line. Power on/off/status operations. |
| `clients/pbsclient/` | PBS HTTP API client. Implements `Ping()` for availability checks. |
| `clients/proxmoxclient/` | Proxmox VE API client. Implements `ListComputeResources()`, `ListBackups()`, `Backup()`. |
| `clients/sshclient/` | SSH client for file-based backups. Supports multiple commands over single connection. |

#### Server

| Package | Description |
|---------|-------------|
| `server/` | Main HTTP server setup and routing. Manages server-level and run-level dependencies. |
| `server/handlers/` | One file per HTTP endpoint. Testable via interfaces defined in `interfaces.go`. |
| `server/runner/` | Backup run execution with concurrent run prevention. Tracks status and history. |
| `server/cron/` | Cron-based scheduling trigger. |
| `server/static/` | Embedded single-page web UI (HTML with inline CSS/JS). |
| `server/config/` | Server-specific configuration loading. |

#### Supporting Packages

| Package | Description |
|---------|-------------|
| `config/` | YAML configuration loading with validation and defaults. |
| `logging/` | Structured logging with slog. Supports JSON/text, configurable levels, capturing handler. |
| `metrics/` | Push and scrape registries for Prometheus/VictoriaMetrics. |
| `buildinfo/` | Build-time metadata injected via ldflags. |

#### Executables

| Package | Description |
|---------|-------------|
| `cmd/cli/` | CLI entry point. Creates workflows, executes once, exits. |
| `cmd/server/` | Server entry point. HTTP server with optional cron scheduling. |
| `cmd/power_off/` | Standalone power-off utility for manual PBS shutdown. |

## Key Architecture Patterns

### Orchestrator Pattern

The orchestrator manages dependency-resolved execution of activities:

1. **Activities** implement `Activity` interface with `Init()` and `Execute(ctx)` methods
2. **Dependency injection** via struct fields:
   - Named fields (e.g., `PowerOnPBS *PowerOnPBS`) provide ordering AND access
   - Unnamed fields (e.g., `_ *SomeActivity`) provide ordering only
3. **Configuration injection** via `config:"path.to.value"` struct tags
4. **State progression:** `NotStarted → Pending → Running → (Completed|Skipped)`

### Server Dependencies

The server maintains two dependency sets:

- **Server-level** (swapped atomically on reload): config, IPMI controller
- **Run-level** (created fresh per backup run): IPMI, PBS client, Proxmox client, metrics client

### Client Package Pattern

```go
// clients/fooclient/client.go
type Client struct { ... }

func New(opts ...Option) *Client { ... }

type Option func(*Client)
func WithLogger(l *slog.Logger) Option { ... }
func WithTimeout(d time.Duration) Option { ... }
```

## Adding New Components

### Adding a New Activity

1. Create struct in appropriate workflow package with dependencies and config tags
2. Implement `Init()` for structural validation
3. Implement `Execute(ctx)` for actual work
4. Add to workflow in `NewWorkflow()` factory via `AddActivity()`
5. Use `activity.CaptureError()` helper for error status reporting

### Adding a New Client Package

1. Create directory `clients/fooclient/`
2. Create `client.go` with `Client` type and `New()` constructor
3. Create `types.go` for return types (public first, internal last)
4. Use Options pattern for configuration
5. Document with godoc comments

### Adding HTTP Endpoints

1. Create handler in `server/handlers/` (one file per endpoint)
2. Implement handler with JSON responses
3. Register route in `server/server.go` `registerRoutes()`
4. Add tests in `server/handlers/*_test.go`

## Testing

- Use testify for all assertions (`require` for critical, `assert` for non-critical)
- Test files in same directory as implementation (`*_test.go`)
- Mock interfaces for unit testing (see `server/handlers/interfaces.go`)

```bash
# Run all tests
make test

# Run tests for a specific package
go test ./orchestrator
go test -v ./server/handlers

# Run a single test
go test -v -run TestBackupVMs ./workflows/backup
```

## Code Style

Follow the [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).

Key rules:
- Time values as named constants
- Pre-size maps when size is known
- Directory name matches package name (except `main` in `cmd/*/`)
- `main()` only calls `run()` and handles errors
- Use standard library constants (`http.MethodPost` not `"POST"`)
