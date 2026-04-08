# Bazel Build Configuration

This repository is configured to use Bazel 9 as an alternative to the Makefile-based build system.

## Prerequisites

Install bazelisk (a Bazel version manager):

```bash
# On Linux (arm64)
curl -L https://github.com/bazelbuild/bazelisk/releases/download/v1.25.0/bazelisk-linux-arm64 -o /usr/local/bin/bazel
chmod +x /usr/local/bin/bazel

# On Linux (amd64)
curl -L https://github.com/bazelbuild/bazelisk/releases/download/v1.25.0/bazelisk-linux-amd64 -o /usr/local/bin/bazel
chmod +x /usr/local/bin/bazel

# On macOS (with Homebrew)
brew install bazelisk

# Or install using Go
go install github.com/bazelbuild/bazelisk@latest
```

## Building

### Build all binaries

```bash
bazel build //...
```

### Build individual binaries

```bash
# Build CLI binary
bazel build //cmd/cli

# Build server binary
bazel build //cmd/server

# Build power-off utility
bazel build //cmd/power_off

# Or use convenient aliases
bazel build //:goback
bazel build //:goback-server
bazel build //:goback-poweroff
```

### Build output location

Binaries are created in `bazel-bin/cmd/{cli,server,power_off}/`:
- `bazel-bin/cmd/cli/cli_/cli` → goback CLI
- `bazel-bin/cmd/server/server_/server` → goback-server
- `bazel-bin/cmd/power_off/power_off_/power_off` → goback-poweroff

## Running

### Run binaries directly

```bash
# Run CLI with arguments
bazel run //:goback -- --config config.yaml

# Run server
bazel run //:goback-server -- --config config.yaml --listen :9090

# Check version
bazel run //:goback -- --version
bazel run //:goback-server -- --version
```

## Testing

### Run all tests

```bash
bazel test //...
```

### Run specific test packages

```bash
# Test a specific package
bazel test //orchestrator:orchestrator_test
bazel test //server/handlers:handlers_test

# Test with verbose output
bazel test //... --test_output=all

# Run a specific test function
bazel test //orchestrator:orchestrator_test --test_arg=-test.run=TestBackupVMs
```

## Clean

```bash
# Clean all build artifacts
bazel clean

# Deep clean (including external dependencies)
bazel clean --expunge
```

## Configuration

### .bazelrc

The `.bazelrc` file contains Bazel configuration:
- Pure Go builds (CGO_ENABLED=0)
- Static linking
- Workspace status command for build-time variable injection
- Test configuration

### .bazelversion

Specifies Bazel version (9.0.0). Bazelisk uses this to download the correct version.

### MODULE.bazel

Defines module dependencies using Bazel 9's bzlmod system:
- rules_go v0.54.0
- gazelle v0.41.0
- Go SDK 1.23.2
- All Go dependencies from go.mod

### Build-time Variables

The build injects git commit and build timestamp into binaries via:
- `workspace_status.sh` - Script that provides build variables
- `x_defs` in BUILD.bazel files - Links variables to Go package variables

Variables injected into `buildinfo` package:
- `buildTime` - Build timestamp (format: YYYY-MM-DD_HH:MM:SS)
- `gitCommit` - Git commit hash (short)

## Gazelle (Automatic BUILD file generation)

Gazelle automatically generates BUILD.bazel files from Go source:

```bash
# Regenerate all BUILD files
bazel run //:gazelle

# Update Go dependency repositories
bazel run //:gazelle-update-repos
```

## Makefile Equivalents

| Makefile Command | Bazel Equivalent |
|-----------------|------------------|
| `make build` | `bazel build //...` |
| `make build-cli` | `bazel build //cmd/cli` |
| `make build-server` | `bazel build //cmd/server` |
| `make test` | `bazel test //...` |
| `make clean` | `bazel clean` |
| `make fmt` | `bazel run @rules_go//go fmt ./...` (or use go fmt directly) |

## Advantages of Bazel

1. **Incremental builds** - Only rebuilds changed code and dependencies
2. **Hermetic builds** - Reproducible builds with locked dependencies
3. **Caching** - Shared cache across builds and machines
4. **Parallel execution** - Automatic parallelization of builds and tests
5. **Remote execution** - Can distribute builds across multiple machines
6. **Platform support** - Better cross-compilation support

## Troubleshooting

### Build fails with "no such package"

Run gazelle to regenerate BUILD files:
```bash
bazel run //:gazelle
```

### Workspace status command fails

Ensure `workspace_status.sh` is executable:
```bash
chmod +x workspace_status.sh
```

Update the path in `.bazelrc` if repository location changed.

### Clean rebuild needed

Sometimes cache issues require a full rebuild:
```bash
bazel clean --expunge
bazel build //...
```

### Module extension warnings

To fix indirect dependency warnings:
```bash
bazel mod tidy
```

This updates MODULE.bazel with correct dependency declarations.
