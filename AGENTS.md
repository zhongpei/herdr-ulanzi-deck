# herdr-deck — AI Agent Guide

## Project Overview

Display herdr AI agent status on Ulanzi D200X macro keypad.

**Platform**: macOS, Linux (herdr only supports these)
**Go version**: `go/` (recommended, actively developed)
**JS version**: `src/` (demo/reference, matched to Go logic)

## Go Development

### Build & Run

```bash
cd go && make build        # build binary
./build/herdrdeck --addr 127.0.0.1 --port 3906   # run
./build/herdrdeck --debug                         # debug mode
cd go && make run          # build + run in one step
```

### Deploy

```bash
bash scripts/deploy-go.sh
```

This kills old processes, creates Ulanzi stub plugin, builds Go binary, starts it.

### Test

```bash
cd go && make test         # all tests
cd go && go test ./pkg/... # or go test directly
```

### Code Structure

```
go/
├── cmd/herdrdeck/main.go     Entry: cobra CLI, event loop
├── pkg/
│   ├── herdr/                herdr connectivity (config, client, bridge, tunnel)
│   ├── state/                State tree: sort, filter, stats
│   ├── mapper/               Filter mode → 14 key commands
│   ├── render/               SVG generation (render.go, colors.go, icons.go)
│   ├── deck/                 WebSocket → UlanziDeck (client.go, draw.go)
│   ├── appstate/             Central store (dirty flag, snapshot, hash)
│   ├── profile/              D200X profile manager
│   └── types/                Shared data structures
└── Makefile
```

### Architecture

- **Event loop**: select with two tickers — renderTick (50ms) + refreshTick (2s)
- **SSH tunnel**: `ssh -NL <localPort>:<remoteSocket> <host>` via tunnel.go
- **Logger**: zerolog (colored ConsoleWriter, 4 levels)
- **CLI**: cobra (--addr, --port, --debug)

## JS Development

```bash
bash scripts/deploy-and-run.sh    # deploy JS plugin
node tests/herdr-client.test.js   # run herdr-client tests
node tests/connection-manager.test.js
node tests/filter-buttons.test.js
```

## Config

`~/.config/herdr-deck/connections.json` — see `connections.sample.json` for reference.

## Important Rules

1. After modifying `go/**/*.go` → run `bash scripts/deploy-go.sh` or `cd go && make run`
2. After modifying `src/*.js` → run `bash scripts/deploy-and-run.sh`
3. Keep JS implementation in sync with Go (same SSH tunnel approach, same data structures)
4. Run all tests before committing: Go tests + JS tests
5. Don't run multiple plugin instances simultaneously

## Docs

- `docs/architecture.md` — System architecture
- `docs/modules.md` — Module-level documentation (Go + JS)
- `docs/development-guide.md` — Detailed dev guide
