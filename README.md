# herdr-deck

Display [herdr](https://herdr.dev) AI agent status on **Ulanzi D200X** macro keypad.

> [中文说明](./docs/README.zh.md)

<p align="center">
  <img src="./assets/deck-photo.jpg" width="600" alt="Herdr agents on Ulanzi D200X">
</p>

## Features

- **Real-time agent status** — Reads herdr workspaces and agents, displays on D200X LCD keys
- **Multi-machine** — Supports multiple herdr instances (local + remote via SSH tunnel)
- **Priority sorting** — Agents sorted by status: BLOCKED → DONE → WORKING → IDLE → UNKNOWN
- **Filter navigation** — K11=ALL, K12=machine cycle, K13=space cycle  
- **Brand colors** — Each agent type has its own brand color (Pi=purple, Claude=warm brown, Cursor=teal...)
- **Machine colors** — Each connection has a defined color (shown on K12 button background)

## Architecture

Two implementations are provided:

### Go (recommended) — `go/`

```
Standalone binary, no Node.js needed.
Compiled to a single executable.

Go binary → WebSocket → UlanziDeck D200X
```

- **Single binary**: 6.5MB arm64, zero external dependencies
- **No plugin directory needed**: runs from anywhere
- **No npm install**: built-in SVG→PNG rendering via `tdewolff/canvas`
- **Languages**: Go 1.22+, uses gorilla/websocket + tdewolff/canvas

### JavaScript (original) — `src/`

```
Node.js plugin → WebSocket → UlanziDeck D200X
```

- Requires Node.js 20+ and npm dependencies (sharp, ws)
- Runs inside UlanziDeck's plugin directory
- Deployed by copying files to plugin dir + `npm install`

## Quick Start (Go)

```bash
# Build and run
cd go && make build
./build/herdrdeck 127.0.0.1 3906

# Or use deploy script (kills old processes, builds, starts)
bash scripts/deploy-go.sh
```

The K11 (ALL) button shows a green **Go** badge to confirm the Go version is active.

### Requirements (Go)

- [herdr](https://herdr.dev) running (local or remote)
- [Ulanzi Studio](https://www.ulanzi.com) 3.1.9+
- Ulanzi D200X device
- Go 1.22+ (for building)

## Quick Start (JavaScript)

```bash
# Copy plugin to UlanziDeck plugins directory
cp -r herdr-deck \
  ~/Library/Application\ Support/Ulanzi/UlanziDeck/Plugins/com.ulanzi.herdr.agentview.ulanziPlugin

# Install dependencies
cd ~/Library/Application\ Support/Ulanzi/UlanziDeck/Plugins/com.ulanzi.herdr.agentview.ulanziPlugin
npm install

# Run
node src/index.js 127.0.0.1 3906 zh_CN

# Or use deploy script
bash scripts/deploy-and-run.sh
```

### Requirements (JavaScript)

- [herdr](https://herdr.dev) running
- [Ulanzi Studio](https://www.ulanzi.com) 3.1.9+
- Ulanzi D200X device
- Node.js 20+

## How it works

```
Herdr Unix Socket → client → bridge → StateManager → ButtonMapper → SVG→PNG → WebSocket → UlanziDeck
```

1. Reads herdr data via Unix socket JSON-line API
2. Merges multi-machine data into a unified workspace tree
3. Sorts agents by status priority, filtered by current mode
4. Generates SVG icons → converts to PNG
5. Sends state commands to UlanziDeck via WebSocket (port 3906)

**Key difference between Go and JS**:

| Aspect | Go | JavaScript |
|--------|----|-----------|
| Runtime | Compiled binary | Node.js 20+ |
| Dependencies | None (vendored) | npm (ws, sharp) |
| SVG→PNG | tdewolff/canvas (pure Go) | sharp (C++ addon) |
| Deployment | `./herdrdeck` from anywhere | Files must be in plugin dir |
| Binary size | ~6.5MB | 184MB+ (Node.js + node_modules) |
| Plugin manifest | Generated + stub index.js | Full plugin with CodePath |

## Configuration

Create `~/.config/herdr-deck/connections.json`:

```json
{
  "connections": [
    {
      "name": "local",
      "abbr": "LCL",
      "color": "#4ADE80",
      "type": "local"
    },
    {
      "name": "dev-server",
      "abbr": "DEV",
      "color": "#1E3A5F",
      "type": "ssh",
      "host": "user@hostname",
      "remoteSocket": "/home/user/.config/herdr/herdr.sock"
    }
  ]
}
```

## Key Bindings (D200X)

| Key | Function |
|-----|----------|
| K1-K10 | Agent status (sorted by priority) |
| K11 | **ALL** — show all machines (green Go badge in bottom-right) |
| K12 | **Machine cycle** — switch between machines (bg = machine color) |
| K13 | **Space cycle** — switch spaces within current machine |
| K14 | **Global stats** — D/I/W/B/? counts (right-aligned) |

## Agent Status Priority

1. **BLOCKED** — ❌ Highest priority (red background)
2. **DONE** — ✅ Completed (green background)
3. **WORKING** — ⏳ In progress (amber background)
4. **IDLE** — ⏸ Waiting (gray background)
5. **UNKNOWN** — ❓ (gray background)

## Development

```bash
# Go tests
cd go && make test

# JavaScript tests
node tests/filter-buttons.test.js

# Deploy Go version
bash scripts/deploy-go.sh

# Deploy JS version (legacy)
bash scripts/deploy-and-run.sh
```

See [AGENTS.md](./AGENTS.md) for development rules.
