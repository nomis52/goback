#!/usr/bin/env bash
#
# demo-start.sh — build and launch the goback web UI against safe demo config.
#
# Picks a free TCP port at launch so multiple agents (each in its own worktree)
# can run demos concurrently without colliding on a fixed port. The runtime
# config is derived from cfg/demo.yaml (fake .invalid hosts, no files: section)
# so nothing real is contacted. Leaves the server running in the background;
# run scripts/demo-stop.sh to tear it down.
#
# Usage:
#   scripts/demo-start.sh          # build + launch
#   scripts/demo-start.sh --run    # also trigger the demo workflow once healthy
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

STATE_DIR="$ROOT/demo-state"
PID_FILE="$STATE_DIR/server.pid"
PORT_FILE="$STATE_DIR/server.port"
LOG_FILE="$STATE_DIR/server.log"
RUNTIME_CFG="$STATE_DIR/server.yaml"

# Refuse if a demo is already running in this worktree.
if [[ -f "$PID_FILE" ]] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
  echo "error: a demo is already running in this worktree (pid $(cat "$PID_FILE"), port $(cat "$PORT_FILE" 2>/dev/null))" >&2
  echo "       run scripts/demo-stop.sh first" >&2
  exit 1
fi

# Pick a free TCP port in a high range (avoids the fixed :8080 collision).
find_free_port() {
  local p
  for _ in $(seq 1 50); do
    p=$(( (RANDOM % 20000) + 20000 ))   # 20000-39999
    if ! lsof -i ":$p" >/dev/null 2>&1; then
      echo "$p"; return 0
    fi
  done
  return 1
}

PORT="$(find_free_port)" || { echo "error: could not find a free port" >&2; exit 1; }

echo "Building binaries..."
make build

mkdir -p "$STATE_DIR"

# Derive the runtime config from cfg/demo.yaml, overriding only the listen port.
# state_dir/workflow_config stay relative to ROOT (we cd there above).
sed -E "s|^  addr: .*|  addr: \":$PORT\"|" cfg/demo.yaml >"$RUNTIME_CFG"

echo "Starting goback-server on :$PORT ..."
nohup ./build/goback-server --config "$RUNTIME_CFG" >"$LOG_FILE" 2>&1 &
echo $! >"$PID_FILE"
echo "$PORT" >"$PORT_FILE"

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
