# Server Package

HTTP server for the goback backup automation system.

## Overview

The server provides a REST API to monitor and control PBS backup operations. It manages:

- PBS power state via IPMI
- Backup run triggering and status
- Configuration management
- Run history

## Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Simple health check, returns `ok` |
| `/ipmi` | GET | Returns PBS power state via IPMI |
| `/config` | GET | Returns current configuration as YAML |
| `/reload` | POST | Reloads configuration from disk |
| `/run` | POST | Triggers a backup run |
| `/status` | GET | Returns status of current/last run |
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
