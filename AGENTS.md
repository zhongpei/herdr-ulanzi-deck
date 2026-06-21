# herdr-agentview — AI Agent Guide

## Project Overview

Display herdr AI agent status on Ulanzi D200X macro keypad.

**Platform**: macOS, Linux (herdr only supports these)
**Architecture**: Three-process (collector + deck + pet)

## Architecture

```
herdr-collector → embedded NATS → herdr-deck (Ulanzi D200X)
                               → herdr-pet  (desktop companion, future)
```

## Go Modules

```
herdr-agentview/
├── go.work                    ← Go workspace ties 5 modules together
├── protocol/                  ← Shared types, enums, NATS subjects
│   └── go.mod
├── collector/                 ← State collection + embedded NATS server
│   ├── go.mod
│   ├── Makefile
│   ├── cmd/herdr-collector/main.go
│   └── internal/{config,herdrclient,tunnel,bridge,fleet,publisher,natsserver}
├── deck/                      ← Ulanzi D200X hardware display
│   ├── go.mod
│   ├── Makefile
│   ├── cmd/herdr-deck/main.go
│   └── internal/{subscriber,fleet,viewmodel,render,deckclient,controller,profile,sysstats}
├── displaymodel/              ← Shared display semantics (filtering, navigation, stats)
│   ├── go.mod
│   ├── model.go
│   └── builder.go
└── scripts/
    ├── deploy-collector.sh
    ├── deploy-deck.sh
    └── deploy-all.sh
```

## Build & Run

```bash
# Single module
cd protocol     && go test ./...
cd displaymodel && go test ./...
cd collector    && make build && ./build/herdr-collector --debug
cd deck         && make build && ./build/herdr-deck --debug

# Or via workspace
go work sync && go vet ./...
bash scripts/deploy-all.sh
```

## Config

- collector reads: `~/.config/herdr-deck/connections.json`
- deck uses CLI flags: `--nats`, `--addr`, `--port`, `--k11-toggle`, `--debug`

## Data Flow

```
Herdr local socket / SSH tunnel
        │
        ▼
herdr-collector (2s fetch)
  ├── fleet.Store (state + TTL)
  ├── embedded NATS (nats://127.0.0.1:4222)
  │     ├── herdr.v1.snapshot.full (full fleet state)
  │     └── herdr.v1.collector.heartbeat (1s liveness)
  │
  ▼
herdr-deck (50ms render)
  ├── subscriber (NATS → FleetSnapshot)
  ├── fleet.Manager (duration/health/sysstats)
  ├── displaymodel.Builder (ViewState → Model)
  ├── viewmodel.Adapt (Model → 14 KeyCommand)
  ├── render (SVG)
  ├── deckclient (SVG→PNG→WebSocket → UlanziDeck)
  └── profile (D200X profile auto-create)
```

## Dependencies

| Module | Depends on |
|--------|-----------|
| protocol | (none) |
| displaymodel | protocol |
| collector | protocol, nats-server, nats.go, zerolog, cobra |
| deck | protocol, displaymodel, nats.go, gorilla/websocket, tdewolff/canvas, gopsutil, zerolog, cobra |

## Important Rules

1. After modifying Go files → run `go vet ./... && go test ./...` in the affected module
2. Never cross-import between collector and deck
3. Only protocol types on the NATS wire
4. Deck never connects to herdr directly — all state via NATS
5. K11Toggle is a deck-side preference (CLI flag, not in connections.json)
6. Deck and pet share displaymodel — never duplicate filter/navigation logic
