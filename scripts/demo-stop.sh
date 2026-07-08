#!/usr/bin/env bash
#
# demo-stop.sh — stop the demo goback-server and remove its state.
#
# Kills the server started by scripts/demo-start.sh (via its PID file, with a
# guarded fallback that only kills a process whose command is goback-server),
# then removes the demo-state directory.
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

PORT=8080
STATE_DIR="$ROOT/demo-state"
PID_FILE="$STATE_DIR/server.pid"

# Preferred: kill the PID we recorded at start.
if [[ -f "$PID_FILE" ]]; then
  PID="$(cat "$PID_FILE")"
  if kill "$PID" 2>/dev/null; then
    echo "Stopped goback-server (pid $PID)"
  fi
fi

# Fallback: anything still on the port, but only if it's actually goback-server.
for pid in $(lsof -ti ":$PORT" 2>/dev/null || true); do
  if ps -p "$pid" -o command= 2>/dev/null | grep -q goback-server; then
    kill "$pid" 2>/dev/null && echo "Stopped stray goback-server (pid $pid)"
  fi
done

rm -rf "$STATE_DIR"
echo "Removed $STATE_DIR"
