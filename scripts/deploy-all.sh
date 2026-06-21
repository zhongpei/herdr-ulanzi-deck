#!/usr/bin/env bash
# deploy-all.sh — build and start both collector + deck
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=== Phase 1: Building collector ==="
bash "$SCRIPT_DIR/deploy-collector.sh" &
COLLECTOR_PID=$!

echo "=== Waiting for NATS server (2s) ==="
sleep 2

echo "=== Phase 2: Building deck ==="
bash "$SCRIPT_DIR/deploy-deck.sh" &
DECK_PID=$!

echo "=== Both processes running ==="
echo "Collector PID: $COLLECTOR_PID"
echo "Deck PID: $DECK_PID"

wait $COLLECTOR_PID $DECK_PID
