# herdr-deck

Display [herdr](https://herdr.dev) AI agent status on **Ulanzi D200X** macro keypad.

> [中文说明](./docs/README.zh.md)
> Development guide: [docs/development-guide.md](./docs/development-guide.md)

<p align="center">
  <img src="./assets/deck-photo.jpg" width="600" alt="Herdr agents on Ulanzi D200X">
</p>

## Platform Support

**Only macOS and Linux are supported.** herdr itself only runs on these platforms.
The author primarily tests on **macOS (arm64)**.

## Features

- **Real-time agent status** — Reads herdr workspaces and agents, displays on D200X LCD keys
- **Multi-machine** — Supports multiple herdr instances (local + remote via SSH tunnel)
- **Priority sorting** — Agents sorted by status: BLOCKED → DONE → WORKING → IDLE → UNKNOWN
- **Filter navigation** — K11=ALL, K12=machine cycle, K13=space cycle
- **Brand colors** — Each agent type has its own brand color (Pi=purple, Claude=warm brown, Cursor=teal…)
- **Machine colors** — Each connection has a defined color (shown on K12 button background)
- **Auto-refresh** — herdr data is polled every 2 seconds; only re-renders when state changes

## Implementations

Two implementations are provided:

### Go (recommended) — `go/`

```
Standalone binary, no Node.js needed.
Compiled to a single executable.

Go binary → WebSocket → UlanziDeck D200X
```

- **Single binary**: ~15MB arm64, zero runtime dependencies
- **No plugin directory needed**: runs from anywhere
- **No npm install**: built-in SVG→PNG rendering via `tdewolff/canvas`
- **Language**: Go 1.25+
- **Dependencies**: gorilla/websocket, tdewolff/canvas, zerolog, cobra

### JavaScript (original) — `src/`

```
Node.js plugin → WebSocket → UlanziDeck D200X
```

- Requires Node.js 20+ and npm dependencies (ws, sharp)
- Runs inside UlanziDeck's plugin directory
- Deployed by copying files to plugin dir + `npm install`

## Quick Start (Go)

```bash
# Build
cd go && make build
./build/herdrdeck --addr 127.0.0.1 --port 3906

# Or run with debug logging
./build/herdrdeck -d

# Full deploy script (kills old processes, builds, starts)
bash scripts/deploy-go.sh
```

### Requirements (Go)

- [herdr](https://herdr.dev) running (local or remote)
- [Ulanzi Studio](https://www.ulanzi.com) 3.1.9+
- Ulanzi D200X device
- Go 1.25+ (for building)

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
herdr Unix Socket → client → bridge → StateManager → ButtonMapper → SVG→PNG → WebSocket → UlanziDeck
```

1. Reads herdr data via Unix socket JSON-line API
2. Merges multi-machine data into a unified workspace tree
3. Sorts agents by status priority, filtered by current mode
4. Generates SVG icons → converts to PNG
5. Sends state commands to UlanziDeck via WebSocket (port 3906)

**Key differences between Go and JS:**

| Aspect | Go | JavaScript |
|--------|----|-----------|
| Runtime | Compiled binary | Node.js 20+ |
| Dependencies | None at runtime (cobra, zerolog, websocket, canvas) | npm (ws, sharp) |
| SVG→PNG | tdewolff/canvas (pure Go) | sharp (C++ addon) |
| CLI | cobra (`--addr`, `--port`, `--debug`) | Positional args |
| Deployment | `./herdrdeck` from anywhere | Files must be in plugin dir |
| Binary size | ~15MB | 184MB+ (Node.js + node_modules) |
| Plugin manifest | Generated stub + standalone binary | Full plugin with CodePath |

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
      "remoteSocket": "/home/user/.config/herdr/herdr.sock",
      "localPort": 19999
    }
  ]
}
```

### SSH Tunnel

For remote herdr connections, the Go implementation spawns `ssh -NL <localPort>:<remoteSocket> <host>` and connects to the forwarded TCP port. The JS implementation uses `ssh -L <localUnixSocket>:<remoteSocket> <host> -N` to forward the remote Unix socket directly.

## Key Bindings (D200X)

| Key | Function |
|-----|----------|
| K1-K10 | Agent status (sorted by priority) |
| K11 | **ALL** — show all machines |
| K12 | **Machine cycle** — switch between machines |
| K13 | **Space cycle** — switch spaces within current machine |
| K14 | **Global stats** — D / I / W / B / ? |

### K14 Stats Bar

The wide key shows status counts. Each **letter** uses its status color (D=green, I=gray, W=amber, B=red, ?=gray), each **number** is white. Items are spaced for readability.

```
┌──────────────────────────────────┐
│                    B 1  W 4  I 2  D 3 │  ← colored letters, white numbers
└──────────────────────────────────┘
```

## Agent Status Priority

1. **BLOCKED** — Highest priority (red)
2. **DONE** — Completed (green)
3. **WORKING** — In progress (amber)
4. **IDLE** — Waiting (gray)
5. **UNKNOWN** — (gray)

## Development

```bash
# Go tests
cd go && make test

# Build and run (ad-hoc)
cd go && make run

# Deploy Go version
bash scripts/deploy-go.sh

# Deploy JS version (legacy)
bash scripts/deploy-and-run.sh
```

See [docs/development-guide.md](./docs/development-guide.md) for detailed development rules.
See [docs/architecture.md](./docs/architecture.md) for system architecture.
See [docs/modules.md](./docs/modules.md) for module-level documentation.
