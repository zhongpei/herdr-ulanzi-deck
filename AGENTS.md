# herdr-agentview — AI Agent Guide

## Project Overview

Display herdr AI agent status on Ulanzi D200X macro keypad and desktop panel.

**Platform**: macOS, Linux, Windows
**Architecture**: Three-process (collector + deck + panel-gio)

## Architecture

```
herdr-collector → embedded NATS → herdr-deck   (Ulanzi D200X)
                               → herdr-panel  (desktop Gio panel)
```

## Go Modules

```
herdr-agentview/
├── go.work                    ← Go workspace ties 6 modules together
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
├── panel-gio/                 ← Desktop Fleet Board (Gio UI, replaces legacy Fyne panel)
│   ├── go.mod
│   ├── Makefile
│   ├── cmd/herdr-panel/main.go
│   └── internal/{subscriber,store,alert,board,command,config}
└── scripts/
    ├── deploy-collector.sh
    ├── deploy-deck.sh
    ├── deploy-panel-gio.sh
    └── deploy-all.sh
```

## Build & Run

```bash
# Single module
cd protocol       && go test ./...
cd displaymodel   && go test ./...
cd collector      && make build && ./build/herdr-collector --debug
cd deck           && make build && ./build/herdr-deck --debug
cd panel-gio      && make build && ./build/herdr-panel --debug

# Or via workspace
go work sync && go vet ./...
bash scripts/deploy-all.sh
```

## Config

- collector reads: `~/.config/herdr-deck/connections.json`
- deck uses CLI flags: `--nats`, `--addr`, `--port`, `--k11-toggle`, `--debug`
- panel-gio uses CLI flags: `--nats`, `--debug`
- panel-gio persists window size to `~/.config/herdr-deck/panel-gio.json`

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
  ├── deckclient (ImageCache → SVG→PNG → WebSocket → UlanziDeck)
  │     └── 3层: latestByKey(同键跳过) / LRU(跨键复用) / SVG→PNG(转换)
  └── profile (D200X profile auto-create)

  ▼
herdr-panel (Gio, ~200ms render)
  ├── subscriber (NATS → FleetSnapshot)
  ├── store (latest snapshot + ViewState + health)
  ├── displaymodel.Builder (ViewState → Model)
  ├── board.LayoutBoard: 5-region Fleet Board
  │   ├── TopHealth   (K14: agent counts + LIVE indicator)
  │   ├── Lens        (K11-K13: ACT/ALL + MACHINE toggles + SPACES toggles)
  │   ├── Attention   (priority agent cards, max 3)
  │   ├── AGENT GRID  (machines × agents status chips)
  │   └── Selected    (selected agent info + shortcuts)
  ├── input/keyboard (A/M/P/R/1-9/Enter/Esc)
  └── config (persistent window size)
```

## Dependencies

| Module | Depends on |
|--------|-----------|
| protocol | (none) |
| displaymodel | protocol |
| collector | protocol, nats-server, nats.go, zerolog, cobra |
| deck | protocol, displaymodel, nats.go, gorilla/websocket, tdewolff/canvas, gopsutil, zerolog, cobra |
| panel-gio | protocol, displaymodel, gioui.org, nats.go, zerolog, cobra |

## Important Rules

1. After modifying Go files → run `go vet ./... && go test ./...` in the affected module
2. Never cross-import between collector and deck/panel-gio
3. Only protocol types on the NATS wire
4. Deck/panel-gio never connects to herdr directly — all state via NATS
5. Deck and panel-gio share displaymodel — never duplicate filter/navigation logic
