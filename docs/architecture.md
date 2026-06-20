# Herdr Agent Status on UlanziDeck — Architecture

> Version: v2.0 (synchronized with code)
> Device: Ulanzi D200X
> Platform: macOS, Linux

---

## 1. System Architecture

```
Herdr Server (local)          Herdr Server (remote via SSH)
Unix Socket                     Unix Socket
       │                              │
       │ JSON-line API                │ SSH tunnel
       ▼                              ▼
┌──────────────────────────────────────────────┐
│  herdr-client         (per-connection)        │
│  herdr-bridge         (merge multi-machine)   │
│  state-manager        (sort + filter)         │
│  button-mapper        (filter mode → 14 keys) │
│  icon-renderer        (SVG generation)        │
├──────────────────────────────────────────────┤
│  deck-client          (WebSocket → D200X)     │
└────────────────────┬─────────────────────────┘
                     │ state commands (PNG base64)
                     ▼
              UlanziDeck (port 3906)
                     │
                     ▼
              D200X 14× LCD keys
```

### Implementations

| Layer | Go (`go/pkg/`) | JavaScript (`src/`) |
|-------|----------------|---------------------|
| herdr config | `herdr/config.go` | `config.js` |
| herdr client | `herdr/client.go` | `herdr-client.js` |
| SSH tunnel | `herdr/tunnel.go` (TCP port forward) | `connection-manager.js` (Unix socket forward) |
| bridge | `herdr/bridge.go` | `herdr-bridge.js` |
| state manager | `state/state.go` | `state-manager.js` |
| mapper | `mapper/mapper.go` | `button-mapper.js` |
| icon renderer | `render/render.go` | `icon-renderer.js` |
| deck client | `deck/client.go` | `deck-client.js` |
| app state | `appstate/store.go` | (inlined in index.js) |
| profile manager | `profile/manager.go` | `profile-manager.js` |
| entry point | `cmd/herdrdeck/main.go` | `index.js` |

### 1.1 Connection Config

`~/.config/herdr-deck/connections.json`:

```json
{
  "connections": [
    { "name": "local",      "abbr": "LCL", "color": "#4ADE80", "type": "local" },
    { "name": "dev-server", "abbr": "DEV", "color": "#1E3A5F", "type": "ssh",
      "host": "user@host", "remoteSocket": "/home/user/.config/herdr/herdr.sock",
      "localPort": 19999 }
  ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Internal identifier |
| `abbr` | string | yes | Abbreviation shown on K12 |
| `color` | string | yes | Machine color for K12 background |
| `type` | `"local"` \| `"ssh"` | yes | Connection type |
| `host` | string | ssh only | SSH target (matches `~/.ssh/config`) |
| `remoteSocket` | string | ssh only | Remote herdr socket absolute path |
| `localPort` | number | ssh only | Local TCP port for SSH forward |
| `sshPort` | number | ssh only | SSH server port (omit = default 22) |

### 1.2 Protocols

| Connection | Protocol |
|------------|----------|
| herdr↔bridge | Unix socket JSON-line (one connection per request) |
| herdr↔bridge (SSH, Go) | `ssh -NL <localPort>:<remoteSocket> <host>` → TCP `127.0.0.1:localPort` |
| herdr↔bridge (SSH, JS) | `ssh -L /tmp/herdr-<name>.sock:<remoteSocket> <host> -N` → local Unix socket |
| bridge↔UlanziDeck | WebSocket JSON (port 3906) |

### 1.3 SSH Tunnel (Go)

- `ssh -NL <localPort>:<remoteSocket> [-p <sshPort>] <host>`
- `-p <sshPort>` is added when `sshPort > 0` in config
- Forwards remote Unix socket to local TCP port
- Tunnel managed by `herdr/tunnel.go` (Start, WaitReady, Close)
- Waits for TCP port to accept connections (up to 10s timeout)
- Cleans up SSH process on shutdown (SIGINT → 1s → SIGKILL)
- Authentication depends on `~/.ssh/config`

### 1.4 SSH Tunnel (JS)

- `ssh -L /tmp/herdr-<name>.sock:<remoteSocket> <host> -N`
- Forwards remote Unix socket to local Unix socket
- Managed by `connection-manager.js`
- Monitors socket file via `fs.statSync` polling (200ms, 15s timeout)

### 1.5 Data Refresh

- **Go**: Event loop runs two tickers: render tick (50ms) + refresh tick (2s)
- **JS**: State changes trigger `onChange` callback → re-render
- **Dedup**: Visual hash computed from all agents + mode + stats; render skipped when hash unchanged

---

## 2. D200X Physical Key Layout

```
       col0   col1   col2   col3   col4
row0:  K1     K2     K3     K4     K5      ← Agents (K1-K10)
row1:  K6     K7     K8     K9     K10
row2:  K11    K12    K13    [K14  wide]    ← Navigation + stats
```

### 2.1 Key Functions

| Key | Function | Description |
|-----|----------|-------------|
| K1-K10 | Agent status | Top 10 agents by priority. BLOCKED > DONE > WORKING > IDLE > UNKNOWN |
| K11 | ALL mode | Show agents from all machines. Active=blue, inactive=gray |
| K12 | Machine cycle | Switch to next machine (LCL→DEV→PRD→…). Clears space filter |
| K13 | Space cycle | Switch spaces within current machine. No-op in ALL mode |
| K14 | Global stats | Cross-machine agent counts (D/I/W/B/?) |

### 2.2 Encoder Knobs

(Reserved, not implemented)

---

## 3. Key Visual Design

### 3.1 Agent Key (K1-K10)

```
┌──────────────────────┐
│ ▓▓▓▓ PI ▓▓▓  ▓▓ LCL ▓│  ← 48px top bar: agent brand color + machine color
│▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁│  ← 1px white separator
│                      │
│       review         │  ← alias 36px BOLD white
│                      │
│          W           │  ← status letter 20px BOLD white
│      main-proj       │  ← workspace name 26px BOLD white
│                      │
└──────────────────────┘
  ↑ background = status color + black overlay 0.15
```

| Area | Content | Size |
|------|---------|------|
| Top bar left | Agent brand color + agent name | 24px BOLD white |
| Top bar right | Machine color + abbreviation | 24px BOLD white |
| Separator | y=48, 1px white, opacity=0.25 | — |
| Alias | User alias | **36px BOLD white** |
| Status letter | D / I / W / B / ? | 20px BOLD white |
| WS name | Current workspace label | **26px BOLD white** |
| Focus border | 3px white glow | when focused=true |

**Agent Brand Colors:**

| Agent | Color | Agent | Color |
|-------|-------|-------|-------|
| Pi | `#7C3AED` purple | Claude | `#D97757` warm brown |
| Cursor | `#00B884` teal | Cline | `#2563EB` blue |
| Codex | `#1E293B` dark slate | Gemini | `#4285F4` Google blue |
| Copilot | `#8957E5` purple | Devin | `#FF6B35` orange |
| Grok | `#1DA1F2` blue | Kimi | `#FF6B6B` coral |
| Kilo | `#10B981` emerald | Kiro | `#F59E0B` amber |
| OpenCode | `#6366F1` indigo | QoderCLI | `#8B5CF6` violet |
| Amp | `#EC4899` pink | AntiGravity | `#06B6D4` cyan |
| Droid | `#84CC16` lime | Hermes | `#F97316` orange |
| Unknown | `#6B7280` gray | | |

**Status Colors (full background):**

| Status | Background |
|--------|-----------|
| DONE | `#27AE60` green |
| IDLE | `#7F8C8D` gray |
| WORKING | `#F39C12` amber |
| BLOCKED | `#E74C3C` red |
| UNKNOWN | `#95A5A6` gray |

### 3.2 Navigation Keys (K11-K13)

**K11 — ALL:**

```
┌──────────┐
│          │
│   ALL    │  ← 36px BOLD white, blue bg (active) / gray bg (inactive)
│          │
└──────────┘
```

**K12 — Machine cycle:**

```
┌──────────┐
│ ▓▓ LCL ▓▓│  ← bg = machine color (LCL=green, DEV=dark blue)
│  → DEV   │  ← bottom arrow + next abbreviation
└──────────┘
```

**K13 — Space cycle:**

```
┌──────────┐
│   MAIN   │  ← space name 28px BOLD uppercase, auto line-break
│   PROJ   │  (breaks on "-" / "_")
│    WS    │  ← small hint
└──────────┘
```

### 3.3 Global Stats (K14 — wide key)

```
┌──────────────────────────────────┐
│                    B 1  W 4  I 2  D 3 │  ← colored letters, white numbers
└──────────────────────────────────┘
```

Each item is two separate SVG `<text>` elements:

- Letter: colored by status, text-anchor=end
- Number: white, text-anchor=start (4px gap from letter)

Items are spaced 65px apart (was 52px), starting from x=370.

SVG example:

```xml
<text x="370" fill="#E74C3C">B</text>
<text x="374" fill="white">1</text>
```

---

## 4. Filter Logic

### Three Modes

| Mode | K11(ALL) | K12(Machine) | K13(Space) | Content |
|------|----------|-------------|------------|---------|
| ALL | Blue highlight | Gray | Gray | All machines, top 10 |
| Machine | — | Machine color bg | Gray | That machine's top 10 |
| Space | — | Machine color bg | Space name | Machine + space top 10 |

### Sort

1. Status priority: `BLOCKED(0) > DONE(1) > WORKING(2) > IDLE(3) > UNKNOWN(4)`
2. Tiebreaker: connection name alphabetically
3. K1-K10: top 10, truncated

---

## 5. Data Flow

### Startup (Go)

```
  1. Load connections.json
  2. For each connection:
     - local:    findLocalSocket() → Unix socket client
     - ssh:      start SSH tunnel → TCP port client
  3. bridge.FetchAll()
     - Each connection: workspace.list + agent.list
     - Merge → UnifiedWorkspace[]
  4. Create D200X profile (profile-manager)
  5. Connect to UlanziDeck WebSocket
  6. Render all 14 keys
  7. Start event loop (select: renderTick 50ms + refreshTick 2s)
```

### Interaction

```
  K1-K10  → agent.focus(connName, paneId)
  K11     → setAll()
  K12     → nextMachine()
  K13     → nextSpace()
  refresh → bridge.FetchAll() → stateManager.Init() → dirty=true
  render  → Capture() → hash compare → renderAll() if changed
```

---

## 6. File Listing

### Go (`go/`)

| File | Description |
|------|-------------|
| `cmd/herdrdeck/main.go` | Entry point: cobra CLI, event loop, render orchestration |
| `pkg/herdr/config.go` | Read `connections.json`, find local socket |
| `pkg/herdr/client.go` | Unix socket JSON-line protocol client |
| `pkg/herdr/bridge.go` | Multi-connection data merge → UnifiedWorkspace |
| `pkg/herdr/tunnel.go` | SSH port-forwarding (start, wait, close) |
| `pkg/state/state.go` | State tree: sort, filter, stats |
| `pkg/mapper/mapper.go` | Filter mode → 14 key render commands |
| `pkg/render/render.go` | SVG generation for all key types |
| `pkg/render/colors.go` | Agent brand colors + status colors |
| `pkg/render/icons.go` | Agent SVG icon paths |
| `pkg/deck/client.go` | WebSocket → UlanziDeck |
| `pkg/deck/draw.go` | SVG→PNG conversion via tdewolff/canvas |
| `pkg/appstate/store.go` | Central state store (dirty flag, snapshot, hash) |
| `pkg/profile/manager.go` | Auto-create D200X profile |
| `pkg/types/types.go` | Shared data structures |

### JavaScript (`src/`)

| File | Description |
|------|-------------|
| `index.js` | Entry: lifecycle, event routing, config → render |
| `config.js` | Read `connections.json` |
| `connection-manager.js` | SSH tunnel management + multi-connection pool |
| `herdr-client.js` | Unix socket JSON-line client |
| `herdr-bridge.js` | Multi-connection merge → UnifiedWorkspace |
| `state-manager.js` | State tree: sort, filter |
| `button-mapper.js` | Filter mode → 14 key render commands |
| `icon-renderer.js` | SVG generation → PNG via sharp |
| `deck-client.js` | WebSocket → UlanziDeck |
| `profile-manager.js` | Auto-create D200X profile |
| `mock-data.js` | Fallback mock data |

### Scripts

| File | Description |
|------|-------------|
| `scripts/deploy-go.sh` | Kill old processes → create stub plugin → build Go binary → start |
| `scripts/deploy-and-run.sh` | Kill old processes → sync JS source → start Node.js plugin |
