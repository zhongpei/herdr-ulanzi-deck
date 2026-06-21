#!/usr/bin/env bash
# deploy-panel-gio.sh — build and start herdr-panel (Gio version)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
PANEL_DIR="$REPO_ROOT/panel-gio"

pkill -f "herdr-panel" 2>/dev/null || true
sleep 1

echo "=== Building herdr-panel (Gio) ==="
cd "$PANEL_DIR"
make build

echo "=== Starting herdr-panel ==="
./build/herdr-panel --debug "$@"
