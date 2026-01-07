# Server Package

HTTP server for the goback backup automation system.

## Overview

The server provides a REST API and web UI to monitor and control PBS backup operations. It manages:

- PBS power state via IPMI
- Backup run triggering and status
- Configuration management
- Run history

## Web UI

Access the dashboard at `http://localhost:8080/` (or your configured address).

The dashboard shows:
- **PBS Server status** - Current power state via IPMI (on/off)
- **Backup status** - Whether a backup is running or idle, with timing info
- **Actions** - Button to trigger a new backup run
- **Run history** - Table of past backup runs with status, timing, and errors

The UI auto-refreshes every 5 seconds.

## Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Web UI dashboard |
| `/health` | GET | Simple health check, returns `ok` |
| `/api/status` | GET | Consolidated status endpoint (PBS state, run status, next run, results) |
| `/config` | GET | Returns current configuration as YAML |
| `/reload` | POST | Reloads configuration from disk |
| `/run` | POST | Triggers a backup run |
| `/history` | GET | Returns history of completed runs |

## Package Structure

```
server/
├── handlers/          # HTTP handlers, one per endpoint
│   ├── config.go      # GET /config
│   ├── health.go      # GET /health
│   ├── history.go     # GET /history
│   ├── interfaces.go  # Shared interfaces
│   ├── ipmi.go        # GET /ipmi
│   ├── reload.go      # POST /reload
│   ├── run.go         # POST /run
│   └── run_status.go  # GET /status
├── runner/            # Backup run execution
│   ├── runner.go      # Runner implementation
│   └── types.go       # RunState, RunStatus
├── static/            # Embedded static files
│   └── index.html     # Web UI
└── server.go          # Server setup and routing
```

## Usage

```go
srv, err := server.New(":8080", "/etc/goback/config.yaml")
if err != nil {
    log.Fatal(err)
}

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

if err := srv.Run(ctx); err != nil {
    log.Fatal(err)
}
```

## Configuration

The server loads configuration from the specified YAML file. Configuration can be reloaded at runtime via `POST /reload`.

The server creates its own logger (JSON to stderr) with a configurable log level that can be changed at runtime via `SetLogLevel()`.

## Architecture

### Server Dependencies

The server maintains two sets of dependencies:

1. **Server-level deps** (swapped atomically on reload):
   - Config
   - IPMI controller (for `/ipmi` endpoint)

2. **Run-level deps** (created fresh for each backup run):
   - IPMI controller
   - PBS client
   - Proxmox client
   - Metrics client

This separation ensures that configuration changes take effect on the next run without interrupting any in-progress backup.

### Runner

The runner manages backup execution:

- Prevents concurrent runs (returns `ErrRunInProgress`)
- Tracks current run status
- Maintains history of completed runs (default: last 100)
- Creates fresh dependencies for each run from current config

### Static Files

Static files are embedded into the binary using Go's `embed` package. The web UI is a single-page application with no external dependencies - all CSS and JavaScript are inline in `index.html`.
