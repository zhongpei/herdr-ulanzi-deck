#!/bin/bash
# run.sh — kill existing herdr-test, rebuild, run.
#
# Usage:
#   ./run.sh <command> [args...]   — same as herdr-test <command> [args...]
#   ./run.sh                       — show help
#
# Examples:
#   ./run.sh svg 0_0
#   ./run.sh anim 0_0 8
#   ./run.sh anim-all 6

set -euo pipefail

cd "$(dirname "$0")"

# Kill any existing herdr-test process
pkill herdr-test 2>/dev/null || true
sleep 0.2

# Build
echo "==> building..."
go build -o /tmp/herdr-test . 2>&1

# Run with all args passed through
echo "==> /tmp/herdr-test $*"
/tmp/herdr-test "$@"
