# herdr-agentview — Development Guide

> Platform: macOS, Linux (herdr only runs on these platforms)

---

## Deployment

### Collector (Go)

```bash
cd collector && make build
./build/herdr-collector --debug
# or
bash scripts/deploy-collector.sh
```

### Deck (Go)

```bash
cd deck && make build
./build/herdr-deck --debug --k11-toggle
# or
bash scripts/deploy-deck.sh
```

### Panel (Gio)

```bash
cd panel-gio && make build
./build/herdr-panel --debug
# or
bash scripts/deploy-panel-gio.sh
```

### All (collector + deck + panel)

```bash
bash scripts/deploy-all.sh
```

### Rules

- collector must start before deck/panel (they connect to collector's embedded NATS)
- After modifying `collector/**/*.go` → `cd collector && make test && make build`
- After modifying `deck/**/*.go` → `cd deck && make test && make build`
- After modifying `panel-gio/**/*.go` → `cd panel-gio && make build`
- After modifying `displaymodel/**/*.go` → `cd displaymodel && go test ./...`
- After modifying `protocol/**/*.go` → `cd protocol && go test ./...`

---

## Debugging

Logs go to stderr with zerolog (colored output):

```bash
# Run with debug logging
./build/herdr-collector --debug 2>&1
./build/herdr-deck --debug 2>&1

# Log levels: DBG(cyan) INF(green) WRN(yellow) ERR(red)
```

For hardware-specific debugging (UlanziDeck WebSocket protocol, key mapping, profile issues),
see [debugging.md](./debugging.md).

---

## Tests

```bash
# Per module
cd protocol     && go test ./...
cd displaymodel && go test ./...
cd collector    && make test
cd deck         && make test

# Full workspace
cd .. && go test ./... 2>&1 | tail -1

# E2E tests (requires Docker)
cd e2e && go test -v -count=1 ./...
```

---

## Architecture Reference

- [AGENTS.md](../AGENTS.md) — Project overview, modules, data flow, dependencies
- [docs/archive/](./archive/) — Archived docs
