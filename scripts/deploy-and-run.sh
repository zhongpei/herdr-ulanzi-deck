#!/bin/bash
set -e

PLUGIN_SRC="/Volumes/sandisk/Work/Work/ulanzi-deck-herdr"
PLUGIN_TARGET="/Users/fofo/Library/Application Support/Ulanzi/UlanziDeck/Plugins/com.ulanzi.herdr.agentview.ulanziPlugin"
LOG_FILE="/tmp/herdr-deck.log"

echo "=== 1. Kill old plugin ==="
OLD_PID=$(pgrep -f "herdr.agentview" 2>/dev/null || true)
if [ -n "$OLD_PID" ]; then
	kill "$OLD_PID" 2>/dev/null
	echo "Killed PID $OLD_PID"
	sleep 1
else
	echo "No old process"
fi

echo ""
echo "=== 2. Sync source files ==="
for f in index.js button-mapper.js icon-renderer.js state-manager.js deck-client.js mock-data.js profile-manager.js; do
	cp "$PLUGIN_SRC/src/$f" "$PLUGIN_TARGET/src/"
done
cp "$PLUGIN_SRC/manifest.json" "$PLUGIN_TARGET/"
echo "Synced 7 source files + manifest"

echo ""
echo "=== 3. Start plugin ==="
cd "$PLUGIN_TARGET"
nohup node src/index.js 127.0.0.1 3906 zh_CN >"$LOG_FILE" 2>&1 &
NEW_PID=$!
echo "Started PID $NEW_PID"

sleep 1
echo ""
echo "=== 4. Log output ==="
cat "$LOG_FILE"

echo ""
echo "=== 5. Status ==="
echo "Plugin running: PID $NEW_PID"
echo "Log: tail -f $LOG_FILE"
echo ""
echo "=== IMPORTANT ==="
echo "1. RESTART Ulanzi Studio (fully quit + reopen)"
echo "2. In Ulanzi Studio, switch profile to \"Herdr Deck\""
echo "3. All 14 keys should then show agent status"
