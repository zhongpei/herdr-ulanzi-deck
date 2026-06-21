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
- **Filter navigation** — K11=ALL/ACT, K12=machine cycle, K13=space cycle (global)
- **Brand colors** — Each agent type has its own brand color (Pi=purple, Claude=warm brown, Cursor=teal…)
- **Machine colors** — Each connection has a defined color (shown on K12 button background)
- **NATS-based state push** — Collector polls herdr every 2s and pushes snapshot via embedded NATS; deck only re-renders when state changes

## Project Structure

Three-process architecture with shared modules:

```
herdr-collector (state collection + embedded NATS)
      │
      │ NATS subjects
      ▼
herdr-deck (Ulanzi D200X display)

Shared:
  protocol/     — types, enums, NATS subjects
  displaymodel/ — view state, filtering, navigation, stats model
```

## Quick Start

### Dependencies

- [herdr](https://herdr.dev) running (local or remote)
- [Ulanzi Studio](https://www.ulanzi.com) 3.1.9+
- Ulanzi D200X device
- Go 1.26+

### Build & Run

```bash
# Build all modules
cd collector && make build
cd deck      && make build

# Start collector first
./build/herdr-collector --debug

# In another terminal, start deck
./build/herdr-deck --debug --k11-toggle

# Or use deploy scripts
bash scripts/deploy-all.sh
```

### Configuration

Collector reads `~/.config/herdr-deck/connections.json`:

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

Deck uses CLI flags: `--nats`, `--addr`, `--port`, `--k11-toggle`, `--debug`.

## How it works

```
herdr-collector (2s fetch)
  → bridge.FetchAll()
  → fleet.Store
  → publisher.PublishSnapshot (herdr.v1.snapshot.full)
  → embedded NATS
    → herdr-deck subscriber
    → fleet.Manager (duration/health/sysstats)
    → displaymodel.Builder (ViewState → Model)
    → viewmodel.Adapt (Model → 14 KeyCommand)
    → render (SVG)
    → deckclient (SVG→PNG→WebSocket)
    → Ulanzi D200X
```

## Key Bindings (D200X)

| Key | Function |
|-----|----------|
| K1-K10 | Agent status (sorted by priority) |
| K11 | **ALL / ACTIVE** — toggle between all agents and filtered (blocked/working/done). Blue=ALL, amber=ACTIVE |
| K12 | **Machine cycle** — switch between machines. Clears space filter |
| K13 | **Space cycle** — switch workspaces globally (all machines). Label-based matching |
| K14 | **Global stats** — D / I / W / B / ? counts, CPU/MEM usage |

## Agent Status Priority

1. **BLOCKED** — Highest priority (red)
2. **DONE** — Completed (green)
3. **WORKING** — In progress (amber)
4. **IDLE** — Waiting (gray)
5. **UNKNOWN** — (gray)

## Development

```bash
# Test per module
cd protocol     && go test ./...
cd displaymodel && go test ./...
cd collector    && make test
cd deck         && make test

# Build
cd collector && make build
cd deck      && make build
```

See [docs/development-guide.md](./docs/development-guide.md) for detailed rules.
See [AGENTS.md](./AGENTS.md) for architecture and module reference.
