#!/bin/bash
set -e

PLUGIN_SRC="/Volumes/sandisk/Work/Work/ulanzi-deck-herdr"
PLUGIN_TARGET="/Users/fofo/Library/Application Support/Ulanzi/UlanziDeck/Plugins/com.ulanzi.herdr.agentview.ulanziPlugin"
LOG_FILE="/tmp/herdr-deck.log"

echo "=== 1. Kill ALL old plugin processes ==="
# Kill by process name pattern (broad match)
pkill -f "index.js.*3906" 2>/dev/null || true
pkill -f "herdr.agentview" 2>/dev/null || true
pkill -f "ulanziPlugin.*index" 2>/dev/null || true
sleep 2

# Verify dead
REMAINING=$(pgrep -f "agentview" 2>/dev/null || true)
if [ -n "$REMAINING" ]; then
	echo "WARNING: processes still running: $REMAINING"
	kill -9 $REMAINING 2>/dev/null || true
	sleep 1
fi
echo "All old processes killed"

echo ""
echo "=== 2. Wait for UlanziStudio (port 3906) ==="
for i in $(seq 1 30); do
	if lsof -i :3906 -P 2>/dev/null | grep -q LISTEN; then
		echo "UlanziStudio ready on port 3906"
		break
	fi
	if [ $i -eq 30 ]; then
		echo "ERROR: UlanziStudio not running on port 3906"
		exit 1
	fi
	sleep 1
done

echo ""
echo "=== 3. Sync source files ==="
for f in index.js button-mapper.js icon-renderer.js state-manager.js deck-client.js mock-data.js profile-manager.js; do
	cp "$PLUGIN_SRC/src/$f" "$PLUGIN_TARGET/src/"
done
cp "$PLUGIN_SRC/manifest.json" "$PLUGIN_TARGET/"
echo "Synced 7 source files + manifest"

# Ensure node_modules (sharp) is synced
if [ ! -d "$PLUGIN_TARGET/node_modules/sharp" ]; then
	echo "Syncing node_modules..."
	cp -r "$PLUGIN_SRC/node_modules" "$PLUGIN_TARGET/"
fi

echo ""
echo "=== 4. Start plugin ==="
cd "$PLUGIN_TARGET"
rm -f "$LOG_FILE"
nohup node src/index.js 127.0.0.1 3906 zh_CN > "$LOG_FILE" 2>&1 &
NEW_PID=$!
echo "Started PID $NEW_PID"

sleep 3

# Verify running
if ! kill -0 "$NEW_PID" 2>/dev/null; then
	echo "ERROR: plugin died immediately. Check log:"
	cat "$LOG_FILE"
	exit 1
fi

echo ""
echo "=== 5. Log output ==="
cat "$LOG_FILE"

echo ""
echo "=== 6. Status ==="
echo "Plugin running: PID $NEW_PID"
echo "Log: tail -f $LOG_FILE"
echo ""
echo "=== IMPORTANT ==="
echo "Restart Ulanzi Studio if keys still show old content."
echo "Then press K12 (machine cycle) to test filtering."
