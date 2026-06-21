#!/usr/bin/env bash
# deploy-all.sh — build and start collector + deck + panel (everything)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== Killing old processes ==="
pkill -f "herdr-collector" 2>/dev/null || true
pkill -f "herdr-deck" 2>/dev/null || true
pkill -f "herdr-panel" 2>/dev/null || true
sleep 1

# ── Collector ──────────────────────────────────────────────
echo "=== Building + starting collector ==="
cd "$REPO_ROOT/collector"
make build
./build/herdr-collector --debug &
COLLECTOR_PID=$!
echo "Collector PID: $COLLECTOR_PID"

echo "=== Waiting for NATS port (up to 12s) ==="
COLLECTOR_OK=false
for i in $(seq 1 12); do
	if nc -z 127.0.0.1 4222 2>/dev/null; then
		echo "NATS ready after ${i}s"
		COLLECTOR_OK=true
		break
	fi
	if ! kill -0 $COLLECTOR_PID 2>/dev/null; then
		echo "Collector died prematurely"
		exit 1
	fi
	sleep 1
done

if [ "$COLLECTOR_OK" != "true" ]; then
	echo "Collector NATS not ready within timeout"
	exit 1
fi

# ── Deck ──────────────────────────────────────────────────
echo "=== Building deck ==="
cd "$REPO_ROOT/deck"
make build

echo "=== Starting deck ==="
./build/herdr-deck --debug &
DECK_PID=$!
echo "Deck PID: $DECK_PID"

sleep 3
if ! kill -0 $DECK_PID 2>/dev/null; then
	echo "Deck failed to start — retrying once"
	./build/herdr-deck --debug &
	DECK_PID=$!
	echo "Deck PID: $DECK_PID"
fi

# ── Panel (Gio) ───────────────────────────────────────────
echo "=== Building panel (Gio) ==="
cd "$REPO_ROOT/panel-gio"
make build

echo "=== Starting panel ==="
./build/herdr-panel --debug &
PANEL_PID=$!
echo "Panel PID: $PANEL_PID"

echo "=== Full stack running (collector + deck + panel) ==="
wait $COLLECTOR_PID $DECK_PID $PANEL_PID
