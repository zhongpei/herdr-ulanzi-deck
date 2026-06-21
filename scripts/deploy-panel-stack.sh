#!/usr/bin/env bash
# deploy-panel-stack.sh — build and start collector + panel (desktop)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== Killing old processes ==="
pkill -f "herdr-collector" 2>/dev/null || true
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

# ── Panel ─────────────────────────────────────────────────
echo "=== Building panel ==="
cd "$REPO_ROOT/panel"
make build

echo "=== Starting panel ==="
# Run panel in background — it's a GUI app, keeps running in its own window
./build/herdr-panel --debug &
PANEL_PID=$!
echo "Panel PID: $PANEL_PID"

echo "=== Desktop stack running ==="
wait $COLLECTOR_PID $PANEL_PID
