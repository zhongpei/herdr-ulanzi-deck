# herdr-ulanzi-deck

Display [herdr](https://herdr.dev) AI agent status on **Ulanzi D200X**.

> [中文说明](./docs/README.zh.md)

<p align="center">
  <img src="./assets/deck-photo.jpg" width="600" alt="Herdr agents on Ulanzi D200X">
</p>

## Features

- **Real-time agent status** — Reads herdr workspaces and agents via Unix socket, displays on D200X LCD keys
- **Multi-machine** — Supports multiple herdr instances (local + remote via SSH tunnel)
- **Priority sorting** — Agents sorted by status: BLOCKED → DONE → WORKING → IDLE → UNKNOWN
- **Filter navigation** — K11=ALL, K12=machine cycle, K13=space cycle
- **Brand colors** — Each agent type has its own brand color as status background (Pi=purple, Claude=warm brown, Cursor=teal...)
- **Machine colors** — Each machine connection has a defined color (shown on K12 button background)

## How it works

```
Herdr Unix Socket  → herdr-client  →  herdr-bridge  →  StateManager
                                                          │
                    UlanziDeck D200X  ←  DeckClient  ←  ButtonMapper
                                                   ←  IconRenderer (SVG→PNG)
```

1. Plugin reads herdr data via Unix socket JSON-line API
2. Merges multi-machine data into a unified workspace tree
3. Sorts agents by status priority, filtered by current mode (ALL/machine/space)
4. Generates SVG icons with brand colors → converts to PNG via `sharp`
5. Sends state commands to UlanziDeck via WebSocket (port 3906)

## Installation

### 1. Install plugin

```bash
# Copy plugin to UlanziDeck plugins directory
cp -r herdr-ulanzi-deck \
  ~/Library/Application\ Support/Ulanzi/UlanziDeck/Plugins/com.ulanzi.herdr.agentview.ulanziPlugin
```

### 2. Install dependencies

```bash
cd ~/Library/Application\ Support/Ulanzi/UlanziDeck/Plugins/com.ulanzi.herdr.agentview.ulanziPlugin
npm install
```

### 3. Configure connections

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

### 4. Run

```bash
node src/index.js 127.0.0.1 3906 zh_CN
```

Or use the deploy script:

```bash
bash scripts/deploy-and-run.sh
```

## Key Bindings (D200X)

| Key | Function |
|-----|----------|
| K1-K10 | Agent status (sorted by priority) |
| K11 | **ALL** — show all machines |
| K12 | **Machine cycle** — switch between machines (bg = machine color) |
| K13 | **Space cycle** — switch spaces within current machine |
| K14 | **Global stats** — D/I/W/B/? counts (bottom-right, colored) |

## Agent Status Priority

1. **BLOCKED** — ❌ Highest priority (red background)
2. **DONE** — ✅ Completed (green background)
3. **WORKING** — ⏳ In progress (amber background)
4. **IDLE** — ⏸ Waiting (gray background)
5. **UNKNOWN** — ❓ (gray background)

## Development

```bash
# Run tests
node tests/filter-buttons.test.js

# Deploy after code changes
bash scripts/deploy-and-run.sh
```

See [AGENTS.md](./AGENTS.md) for development rules.

## Requirements

- [herdr](https://herdr.dev) running (local or remote)
- [Ulanzi Studio](https://www.ulanzi.com) 3.1.9+
- Ulanzi D200X device
- Node.js 20+
- SSH access to remote herdr instances (optional)
