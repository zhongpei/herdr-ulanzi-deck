#!/usr/bin/env bash
# deploy-collector.sh — build and start herdr-collector
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
COLLECTOR_DIR="$REPO_ROOT/collector"

pkill -f "herdr-collector" 2>/dev/null || true
sleep 1

echo "=== Building herdr-collector ==="
cd "$COLLECTOR_DIR"
make build

echo "=== Starting herdr-collector ==="
./build/herdr-collector --debug "$@"
