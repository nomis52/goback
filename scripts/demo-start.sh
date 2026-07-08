#!/usr/bin/env bash
#
# demo-start.sh — build and launch the goback web UI against safe demo config.
#
# Uses cfg/demo.yaml + cfg/demo-workflows.yaml (fake .invalid hosts, no files:
# section) so nothing real is contacted. Leaves the server running in the
# background so you can inspect the UI; run scripts/demo-stop.sh to tear it down.
#
# Usage:
#   scripts/demo-start.sh          # build + launch
#   scripts/demo-start.sh --run    # also trigger the demo workflow once healthy
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

PORT=8080
STATE_DIR="$ROOT/demo-state"
PID_FILE="$STATE_DIR/server.pid"
LOG_FILE="$STATE_DIR/server.log"

# Refuse to start if the port is already bound (e.g. a leftover demo run).
if lsof -i ":$PORT" >/dev/null 2>&1; then
  echo "error: port $PORT is already in use — run scripts/demo-stop.sh first" >&2
  lsof -i ":$PORT" >&2
  exit 1
fi

echo "Building binaries..."
make build

mkdir -p "$STATE_DIR"

echo "Starting goback-server on :$PORT ..."
nohup ./build/goback-server --config cfg/demo.yaml >"$LOG_FILE" 2>&1 &
echo $! >"$PID_FILE"

# Wait for the server to report healthy.
for _ in $(seq 1 30); do
  if curl -sf "http://localhost:$PORT/health" >/dev/null 2>&1; then
    echo "Server is up: http://localhost:$PORT/"
    if [[ "${1:-}" == "--run" ]]; then
      echo "Triggering the demo workflow..."
      curl -s -X POST "http://localhost:$PORT/run" \
        -H 'Content-Type: application/json' \
        -d '{"workflows":["demo"]}'
      echo "Demo workflow started (sleeps ~2s across 3 steps)."
    fi
    exit 0
  fi
  sleep 0.3
done

echo "error: server did not become healthy — see $LOG_FILE" >&2
exit 1
