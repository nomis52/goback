#!/bin/bash
# Workspace status script for Bazel
# This provides build-time variables for x_defs

# Get git commit (short)
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Get build timestamp in the same format as Makefile
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

# Output stable status (cached between builds if values don't change)
echo "STABLE_GIT_COMMIT ${GIT_COMMIT}"

# Output volatile status (always changes)
echo "BUILD_TIMESTAMP ${BUILD_TIME}"
