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

### Both (collector + deck)

```bash
bash scripts/deploy-all.sh
```

### Rules

- collector must start before deck (deck connects to collector's embedded NATS)
- After modifying `collector/**/*.go` → `cd collector && make test && make build`
- After modifying `deck/**/*.go` → `cd deck && make test && make build`
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

---

## Tests

```bash
# Per module
cd protocol   && go test ./...
cd collector  && make test
cd deck       && make test

# E2E tests (requires Docker)
cd e2e        && go test -v -count=1 ./...
```

---

## Architecture Reference

- [docs/architecture.md](./architecture.md) — Original single-process architecture
- [docs/go-architecture.md](./go-architecture.md) — Three-process architecture blueprint
- [docs/modules.md](./modules.md) — Module-by-module code documentation
