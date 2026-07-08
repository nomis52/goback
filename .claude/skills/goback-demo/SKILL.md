---
name: goback-demo
description: >-
  Build and launch the goback-server web UI against safe demo-only config (no
  real PBS/Proxmox/SSH infra), print the URL, optionally trigger the demo
  workflow, then clean up on the user's OK. Use to visually demo or inspect the
  server UI for a change before opening a PR.
---

# goback demo

Runs the goback web UI locally against **fake** config so the user can click
around the UI and confirm a change works. Uses only `cfg/demo.yaml` +
`cfg/demo-workflows.yaml` (fake `.invalid` hosts, no `files:` section) — it
never touches `config.yaml`, `cfg/test.yaml`, `cfg/prod.yaml`, or the real
`state/` dir.

## Steps

1. **Confirm you're in the worktree root.** The demo configs use paths relative
   to the server's working directory, so run everything from the top of the
   worktree where the change is being developed (not the main checkout):

   ```bash
   git rev-parse --show-toplevel   # cd here if you aren't already
   ```

2. **Build:**

   ```bash
   make build
   ```

3. **Pre-flight the port.** `cfg/demo.yaml` listens on `:8080`; if something is
   already bound there (a leftover demo run or a `test.yaml` server), stop and
   tell the user rather than failing to bind:

   ```bash
   lsof -i :8080   # expect no output; if a PID is shown, ask the user to free it
   ```

4. **Launch in the background** (long-running, so the user can inspect the UI —
   do **not** wrap it in `timeout`). Use the Bash tool's `run_in_background`
   option and record the returned shell/PID for cleanup:

   ```bash
   ./build/goback-server --config cfg/demo.yaml
   ```

5. **Wait until it's serving**, then give the user the URL:

   ```bash
   curl -sf http://localhost:8080/health   # returns "ok" once up
   ```

   Then print: **http://localhost:8080/**

6. **(Optional) Trigger the safe demo workflow.** It just sleeps ~2s across 3
   steps and updates status/history — it touches no infra:

   ```bash
   curl -s -X POST http://localhost:8080/run \
     -H 'Content-Type: application/json' \
     -d '{"workflows":["demo"]}'
   ```

   Alternatively, tell the user to open the **Workflows** tab and click **Run**
   on `demo`.

7. **Ask the user to inspect the UI** and reply OK when they're done.

8. **On the user's OK, clean up:**
   - Kill the background server by the PID/shell you recorded. Fallback if the
     PID was lost: `lsof -i :8080` to find it, then kill it.
   - Remove the demo run history: `rm -rf demo-state`.

   Do not commit `demo-state` — it is gitignored.
