# Goback

PBS (Proxmox Backup Server) backup automation that powers on the PBS server, runs backups, and shuts it down to save energy.

## What It Does

Goback automates the complete backup workflow for a homelab setup:

1. **Power on PBS** - Wakes the PBS server via IPMI
2. **Wait for PBS** - Waits until PBS services are available
3. **Backup VMs/LXCs** - Triggers Proxmox VE to backup virtual machines and containers to PBS
4. **Backup files** - Runs file-based backups via SSH using `proxmox-backup-client`
5. **Power off PBS** - Shuts down the PBS server to save power

This is useful when you want backups to run on a schedule but don't want the PBS server running 24/7.

## Installation

### Build from source

```bash
git clone https://github.com/nomis52/goback.git
cd goback
make build
```

Binaries are built to the `build/` directory:
- `goback` - CLI tool for one-time backup runs
- `goback-server` - HTTP server with web UI and optional scheduling
- `goback-poweroff` - Utility to manually power off PBS

### Systemd service

A systemd service file is provided in `systemd/` for running the server as a service.

## Configuration

Create a `config.yaml` file:

```yaml
pbs:
  host: "https://pbs.example.com:8007"
  ipmi:
    host: pbs-bmc.example.com
    username: ADMIN
    password: ADMIN
  boot_timeout: "5m"
  service_wait_time: "30s"
  shutdown_timeout: "2m"

proxmox:
  host: "https://pve.example.com:8006/"
  token: "backup@pve!goback=your-api-token"
  storage: pbs
  backup_timeout: "45m"

compute:
  max_backup_age: "24h"  # Skip VMs/LXCs backed up within this period

files:
  host: pve.example.com
  user: root
  private_key_path: /path/to/ssh/key
  token: proxmox-backup-client-token
  target: user@pbs!datastore@pbs-server:storage
  sources:
    - name.pxar:/path/to/backup

monitoring:
  victoriametrics_url: "https://metrics.example.com:443"
  metrics_prefix: "goback"

logging:
  level: "info"      # debug, info, warn, error
  format: "json"     # json, text
  output: "stdout"   # stdout, stderr, or file path
```

### Configuration sections

| Section | Description |
|---------|-------------|
| `pbs` | PBS server address and IPMI credentials for power management |
| `proxmox` | Proxmox VE API connection for triggering VM/LXC backups |
| `compute` | Settings for VM/LXC backups (skip if recent backup exists) |
| `files` | SSH-based file backups using `proxmox-backup-client` |
| `monitoring` | Optional metrics push to VictoriaMetrics/Prometheus |
| `logging` | Log level, format, and output destination |

## Usage

### CLI mode

Run a one-time backup:

```bash
./goback --config cfg/test.yaml
```

Validate configuration without running:

```bash
./goback --config cfg/test.yaml --validate
```

### Server mode

The server uses a separate config file that references the workflow config.

See `cfg/test.yaml` for an example server config:

```yaml
listener:
  addr: ":8080"
  # tls_cert: /path/to/cert.pem  # Optional TLS
  # tls_key: /path/to/key.pem

cron:
  - workflows:
      - backup
      - poweroff
    schedule: "5 4 * * *"  # Daily at 4:05am

state_dir: "./state"  # location to use for history
log_level: "info"
workflow_config: "./config.yaml"
```

Start the server:

```bash
./goback-server --config cfg/test.yaml
```

### Web UI

Access the dashboard at `http://localhost:8080/` (or your configured address).

The dashboard shows:
- PBS server power state
- Current backup status and progress
- Next scheduled run time
- History of completed runs

### HTTP API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Web UI dashboard |
| `/health` | GET | Health check |
| `/api/status` | GET | Current status (PBS state, run status, next run) |
| `/api/history` | GET | Completed run history |
| `/config` | GET | Current configuration |
| `/reload` | POST | Reload configuration from disk |
| `/run` | POST | Trigger a backup run |

### Manual power off

Power off PBS without running a backup:

```bash
./goback-poweroff --config cfg/test.yaml
```

## Requirements

- Go 1.23+ (for building)
- `ipmitool` installed on the system running goback
- Proxmox VE with API token for backup operations
- PBS server with IPMI-capable BMC
- SSH access for file-based backups (optional)
