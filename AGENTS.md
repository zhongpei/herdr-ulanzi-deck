# herdr-agentview — AI Agent Guide

## Project Overview

Display herdr AI agent status on Ulanzi D200X macro keypad and desktop panel.

**Platform**: macOS, Linux, Windows
**Architecture**: Three-process (collector + deck + panel)

## Architecture

```
herdr-collector → embedded NATS → herdr-deck   (Ulanzi D200X)
                               → herdr-panel  (desktop reminder panel)
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
├── panel/                     ← Desktop reminder panel (Fyne GUI)
│   ├── go.mod
│   ├── cmd/herdr-panel/main.go
│   └── internal/{subscriber,state,app,ui,alert,sysstats}
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
cd panel        && go build -o build/herdr-panel ./cmd/herdr-panel/

# Or via workspace
go work sync && go vet ./...
bash scripts/deploy-all.sh
```

## Config

- collector reads: `~/.config/herdr-deck/connections.json`
- deck uses CLI flags: `--nats`, `--addr`, `--port`, `--k11-toggle`, `--debug`
- panel uses CLI flags: `--nats`, `--debug`

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

  ▼
herdr-panel (1s refresh)
  ├── subscriber (NATS → FleetSnapshot)
  ├── state.Store (latest snapshot + ViewState + health)
  ├── displaymodel.Builder (ViewState → Model)
  ├── ui/main_window    (Fyne window, close→tray, remember geometry)
  ├── ui/stats_bar      (K14: agent counts + CPU/MEM)
  ├── ui/toolbar        (K11: ALL/ACT buttons + K12/K13 dropdowns)
  ├── ui/card_grid      (2×3 agent status cards, priority truncated)
  ├── ui/tray           (system tray menu)
  ├── alert/monitor     (state-change detection + window popup)
  └── sysstats          (local CPU/MEM)
```

## Dependencies

| Module | Depends on |
|--------|-----------|
| protocol | (none) |
| displaymodel | protocol |
| collector | protocol, nats-server, nats.go, zerolog, cobra |
| deck | protocol, displaymodel, nats.go, gorilla/websocket, tdewolff/canvas, gopsutil, zerolog, cobra |
| panel | protocol, displaymodel, fyne.io/fyne/v2, nats.go, gopsutil, zerolog, cobra |

## Important Rules

1. After modifying Go files → run `go vet ./... && go test ./...` in the affected module
2. Never cross-import between collector and deck/panel
3. Only protocol types on the NATS wire
4. Deck/panel never connects to herdr directly — all state via NATS
5. K11Toggle is a deck-side preference (CLI flag, not in connections.json)
6. Deck and panel share displaymodel — never duplicate filter/navigation logic
