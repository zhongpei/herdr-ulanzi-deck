#!/bin/bash
set -e

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
GO_DIR="$PROJECT_DIR/go"
BUILD_DIR="$PROJECT_DIR/build"
LOG_FILE="/tmp/herdr-deck.log"

echo "=== 1. Kill ALL old processes (JS + Go) ==="
# Kill JS plugin processes (from old JS deployment + Ulanzi auto-launch)
pkill -f "index.js.*3906" 2>/dev/null || true
pkill -f "herdr.agentview" 2>/dev/null || true
pkill -f "ulanziPlugin.*index" 2>/dev/null || true
pkill -f "NodeJS.*agentview" 2>/dev/null || true

# Kill any previously running Go herdrdeck binary
pkill -f "herdrdeck" 2>/dev/null || true
pkill -f "herdr-deck" 2>/dev/null || true

sleep 2

# Verify all dead
REMAINING=$(pgrep -f "index.js.*3906\|herdr.agentview\|ulanziPlugin\|herdrdeck\b" 2>/dev/null || true)
if [ -n "$REMAINING" ]; then
	echo "WARNING: processes still running: $REMAINING"
	kill -9 $REMAINING 2>/dev/null || true
	sleep 1
fi
echo "All old processes killed"

echo ""
echo "=== 2. Recreate plugin dir with manifest.json only ==="
PLUGIN_DIR="/Users/fofo/Library/Application Support/Ulanzi/UlanziDeck/Plugins/com.ulanzi.herdr.agentview.ulanziPlugin"
rm -rf "$PLUGIN_DIR"
mkdir -p "$PLUGIN_DIR/src"
mkdir -p "$PLUGIN_DIR/assets"

# Create minimal manifest - deck needs this to recognize the action UUID
cat > "$PLUGIN_DIR/manifest.json" << 'EOF'
{
  "Author": "herdr-deck",
  "Name": "Herdr Agent View",
  "Description": "Display Herdr AI agent status on UlanziDeck",
  "Icon": "assets/store_icon.png",
  "Version": "0.1.0",
  "Category": "Developer",
  "CategoryIcon": "assets/category.png",
  "CodePath": "src/index.js",
  "Type": "JavaScript",
  "UUID": "com.ulanzi.herdr.agentview",
  "Actions": [
    {
      "Name": "Agent Monitor",
      "Icon": "assets/action.png",
      "UUID": "com.ulanzi.herdr.agentview.monitor",
      "States": [
        { "Name": "Active", "Image": "assets/action.png" }
      ],
      "Controllers": ["Keypad"],
      "Devices": ["D200X"],
      "SupportedInMultiActions": false,
      "DisableAutomaticStates": true
    }
  ],
  "OS": [
    { "Platform": "mac", "MinimumVersion": "10.11" },
    { "Platform": "windows", "MinimumVersion": "10" }
  ],
  "Software": {
    "MinVersion": "2.1.0"
  }
}
EOF

# Copy real 1x1 PNG icons (deck validates PNG format)
cp "$PROJECT_DIR/assets/store_icon.png" "$PLUGIN_DIR/assets/" 2>/dev/null || touch "$PLUGIN_DIR/assets/store_icon.png"
cp "$PROJECT_DIR/assets/category.png" "$PLUGIN_DIR/assets/" 2>/dev/null || touch "$PLUGIN_DIR/assets/category.png"
cp "$PROJECT_DIR/assets/action.png" "$PLUGIN_DIR/assets/" 2>/dev/null || touch "$PLUGIN_DIR/assets/action.png"

# Create stub index.js - PluginJsManager needs CodePath to exist or it errors out
cat > "$PLUGIN_DIR/src/index.js" << 'STUB'
// Stub: Go binary handles rendering via WebSocket
// This file keeps Node alive to satisfy PluginJsManager's plugin loading
console.log("[ulanziPlugin] stub loaded");
setInterval(() => {}, 60000);
STUB

echo "Recreated plugin dir with manifest + icon placeholders + stub js"

echo ""
echo "=== 3. Wait for UlanziStudio (port 3906) ==="
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
echo "=== 4. Build Go binary ==="
cd "$GO_DIR"
mkdir -p "$BUILD_DIR"
go build -o "$BUILD_DIR/herdrdeck" ./cmd/herdrdeck/
echo "Built: $BUILD_DIR/herdrdeck"

echo ""
echo "=== 5. Start Go plugin ==="
rm -f "$LOG_FILE"
nohup "$BUILD_DIR/herdrdeck" 127.0.0.1 3906 > "$LOG_FILE" 2>&1 &
NEW_PID=$!
echo "Started PID $NEW_PID"

sleep 2

# Verify running
if ! kill -0 "$NEW_PID" 2>/dev/null; then
	echo "ERROR: herdrdeck died immediately. Check log:"
	cat "$LOG_FILE"
	exit 1
fi

echo ""
echo "=== 6. First log output ==="
cat "$LOG_FILE"

echo ""
echo "=== 7. Status ==="
echo "Binary:      $BUILD_DIR/herdrdeck"
echo "PID:         $NEW_PID"
echo "Log:         tail -f $LOG_FILE"
echo "Ulanzi port: 127.0.0.1:3906"
echo ""
echo "No plugin directory needed. Go binary runs standalone."
