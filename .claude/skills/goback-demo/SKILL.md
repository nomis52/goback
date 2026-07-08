---
name: goback-demo
description: >-
  Launch the goback-server web UI against safe demo-only config (no real
  PBS/Proxmox/SSH infra) so the user can inspect a change, then tear it down.
  Use to visually demo or verify a change before opening a PR.
---

# goback demo

Runs the goback web UI locally against **fake** config (`cfg/demo.yaml` +
`cfg/demo-workflows.yaml` — fake `.invalid` hosts, no `files:` section) so the
user can click around and confirm a change works. Touches nothing real:
`config.yaml`, `cfg/test.yaml`, `cfg/prod.yaml`, and the real `state/` dir are
never used.

Run these from the worktree where the change is being developed.

## Start the demo

```bash
scripts/demo-start.sh          # build + launch, prints the URL
scripts/demo-start.sh --run    # also trigger the safe demo workflow once up
```

A free port is chosen automatically, so multiple agents can demo concurrently
without colliding — read the actual URL from the script's output rather than
assuming `:8080`. The server runs in the background so the user can inspect the
UI. Give them the URL and ask them to reply OK when done. The demo workflow
(started by `--run`, or via the **Run** button on the Workflows tab) just
sleeps ~2s across 3 steps and touches no infra.

## Stop the demo

On the user's OK:

```bash
scripts/demo-stop.sh           # stop the server and remove demo-state
```

If `demo-start.sh` reports a demo is already running in this worktree, run
`scripts/demo-stop.sh` first.
