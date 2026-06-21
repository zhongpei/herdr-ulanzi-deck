#!/usr/bin/env bash
# deploy-deck.sh — build and start herdr-deck
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
DECK_DIR="$REPO_ROOT/deck"

echo "=== Building herdr-deck ==="
cd "$DECK_DIR"
make build

echo "=== Starting herdr-deck ==="
./build/herdr-deck --debug "$@"
