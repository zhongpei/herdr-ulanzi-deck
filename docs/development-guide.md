# herdr-deck — Development Guide

> Platform: macOS, Linux (herdr only runs on these platforms)

---

## Deployment

After any code change, use the appropriate deployment script. **Never** manually copy files or start processes.

### Go Version (recommended)

```bash
bash scripts/deploy-go.sh
```

This script:

1. **Kills all old processes** — JS plugin (`index.js.*3906`, `herdr.agentview`, `ulanziPlugin`) and Go binary (`herdrdeck`)
2. **Recreates plugin directory** — Creates minimal manifest.json + stub `index.js` (keeps UlanziDeck's PluginJsManager happy)
3. **Waits for UlanziStudio** — Polls port 3906, up to 30 seconds
4. **Builds Go binary** — `cd go && go build -o build/herdrdeck ./cmd/herdrdeck/`
5. **Starts Go binary** — backgrounded with `nohup`, logs to `/tmp/herdr-deck.log`
6. **Verifies process** — Checks PID is alive

### JavaScript Version (legacy)

```bash
bash scripts/deploy-and-run.sh
```

This script:

1. **Kills all old processes** — JS plugin processes
2. **Waits for UlanziStudio** — Polls port 3906
3. **Syncs source files** — Copies JS files from `src/` to plugin installation directory
4. **Syncs dependencies** — Ensures `node_modules/` (including `sharp`) exists
5. **Starts plugin** — Backgrounded with `nohup node src/index.js`

### Rules

- After modifying `src/*.js` → run `bash scripts/deploy-and-run.sh`
- After modifying `go/**/*.go` → run `bash scripts/deploy-go.sh` (or `cd go && make run` for ad-hoc)
- **Do not** manually cp files to plugin directory
- **Do not** manually kill processes and start `node src/index.js`
- **Do not** run multiple plugin instances (keydown events may route to the wrong process)

### Ad-hoc Build & Run (Go)

```bash
cd go && make run
```

Or step by step:

```bash
cd go && make build
./build/herdrdeck --addr 127.0.0.1 --port 3906
./build/herdrdeck --debug   # with debug logging
```

---

## Debugging

### Go Version

Logs go to stderr with zerolog (colored output by default):

```bash
# Watch live logs (zerolog colored output)
cd go && make run 2>&1 | cat

# Or run in background and check log file
bash scripts/deploy-go.sh
tail -f /tmp/herdr-deck.log
```

Log levels:

- `DBG` (cyan) — debug info (only with `--debug`)
- `INF` (green) — normal lifecycle events
- `WRN` (yellow) — recoverable issues
- `ERR` (red) — must-fix errors

Filtering:

```bash
tail -f /tmp/herdr-deck.log | grep "INF"
tail -f /tmp/herdr-deck.log | grep "ERR"
```

### JavaScript Version

```bash
tail -f /tmp/herdr-deck.log    # live log
grep "input" /tmp/herdr-deck.log  # keydown events
grep "nav" /tmp/herdr-deck.log    # navigation events
grep "error\|Error\|fail" /tmp/herdr-deck.log # errors
```

---

## Tests

```bash
# Go tests (all packages)
cd go && make test

# With race detection
cd go && go test -race -count=1 ./pkg/...
```

---

## Architecture Reference

- [docs/architecture.md](./architecture.md) — System architecture, key layout, data flow
- [docs/modules.md](./modules.md) — Module-by-module code documentation
