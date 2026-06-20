# Implementation — Module Reference

> Module-by-module description covering both Go and JavaScript implementations.
> Platform: macOS, Linux

---

## File Map

### Go (`go/`)

```
go/
├── cmd/herdrdeck/main.go       Entry: cobra CLI, event loop
├── pkg/
│   ├── herdr/                   herdr connectivity
│   │   ├── config.go            Config: reads connections.json, finds local socket
│   │   ├── client.go            Unix socket JSON-line protocol client
│   │   ├── bridge.go            Multi-connection data merge → UnifiedWorkspace
│   │   └── tunnel.go            SSH port-forwarding (start, wait, close)
│   ├── state/state.go           State tree: sort (by status), filter (by machine/space)
│   ├── mapper/mapper.go         Filter mode → 14 key render commands
│   ├── render/                  SVG generation
│   │   ├── render.go            SVG template generation for all key types
│   │   ├── colors.go            Agent brand colors + status indicator colors
│   │   └── icons.go             SVG path data for each agent icon
│   ├── deck/                    UlanziDeck WebSocket communication
│   │   ├── client.go            WebSocket state commands + keydown events
│   │   └── draw.go              SVG→PNG conversion via tdewolff/canvas
│   ├── appstate/store.go        Central state store (dirty flag, snapshot, hash)
│   ├── profile/manager.go       Auto-create D200X profile with 14 keys
│   └── types/types.go           Shared data structures (agent info, key commands, etc.)
├── Makefile                     Build targets: build, test, run, fmt, vet, clean
```

### JavaScript (`src/`)

```
src/
├── index.js                     Entry: lifecycle, event routing, render orchestration
├── config.js                    Read ~/.config/herdr-deck/connections.json
├── connection-manager.js        SSH tunnel management + multi-connection pool
├── herdr-client.js              Unix socket JSON-line client (one conn per request)
├── herdr-bridge.js              Multi-connection data merge → UnifiedWorkspace[]
├── state-manager.js             State tree: sort (by status), filter (by machine/space)
├── button-mapper.js             Filter mode → 14 key render commands
├── icon-renderer.js             SVG generation → PNG via sharp
├── deck-client.js               WebSocket → UlanziDeck (state commands + keydown events)
├── profile-manager.js           Auto-create D200X profile with 14 keys assigned
└── mock-data.js                 Fallback mock data when herdr unavailable
```

### Scripts

```
scripts/
├── deploy-go.sh                 Kill old → create stub plugin → build Go binary → start
└── deploy-and-run.sh            Kill old → sync JS source → start Node.js plugin
```

---

## Module Details (Go)

### `cmd/herdrdeck/main.go`

Entry point for the Go binary.

- Uses **cobra** for CLI parsing: `--addr` (default 127.0.0.1), `--port` (default 3906), `--debug` (default false)
- Initializes **zerolog** logger: ConsoleWriter with colored output (DBG=cyan, INF=green, WRN=yellow, ERR=red)
- Default level is Info; `--debug` enables Debug level with source file/line
- Loads config → starts SSH tunnels (if any) → creates bridge → fetches herdr data
- Creates D200X profile → connects to UlanziDeck WebSocket → starts event loop
- **Event loop** uses `select` with two tickers:
  - `renderTick` (50ms): checks dirty flag, captures snapshot, hash-dedup, renders
  - `refreshTick` (2s): re-fetches herdr data via `bridge.FetchAll()`, marks dirty
- `messagePump` goroutine: handles WebSocket reconnection (2s retry, re-seed key actions)

### `pkg/herdr/config.go`

- `LoadConfig()` reads `~/.config/herdr-deck/connections.json`
- Auto-creates default config with single "local" connection if file missing
- `FindLocalSocket()` checks candidates: `$HERDR_SOCKET_PATH`, `~/.config/herdr/herdr.sock`, etc.
- Returns first socket file found (checks both socket mode and regular file)

### `pkg/herdr/client.go`

- `Client` struct holds target (Unix socket path or TCP `"host:port"`)
- `Request(method, params)` → opens new connection, sends JSON-line, reads response
- One connection per request (matches herdr's Rust client behavior)
- `dial()` chooses Unix socket (`net.DialTimeout("unix", ...)`) if target starts with `/`, else TCP
- Convenience methods: `ListWorkspaces()`, `ListAgents()`, `ListPanes()`
- `Subscribe(params, onEvent)` → long-lived connection for event subscription (not currently used)

### `pkg/herdr/bridge.go`

- `Bridge` holds list of `ConnRef` (name, abbr, color, client)
- `AddConnection(name, abbr, color, client)` registers a connection
- `FetchAll()` → queries all connections concurrently (via `fetchConn`), merges into `[]types.UnifiedWorkspace`
- `FocusAgent(connName, paneId)` → sends `agent.focus` request to correct connection
- `fetchConn()` calls `ListWorkspaces()` + `ListAgents()`, parses JSON into typed structs

### `pkg/herdr/tunnel.go`

- `Tunnel` manages an SSH port-forwarding process
- `SSHPort` field: when > 0, `Start()` adds `-p <sshPort>` before the host argument
- `Start()` spawns `ssh -NL <localPort>:<remoteSocket> [-p <sshPort>] <host>`
- `WaitReady(timeout)` polls TCP port until accepting connections (100ms interval)
- `Close()` sends SIGINT, waits 1s, then SIGKILL if process hasn't exited
- Target address: `"127.0.0.1:<localPort>"` — consumed by `herdr.New()` as TCP target

### `pkg/state/state.go`

- `Manager` holds `[]types.UnifiedWorkspace` and listener list
- `Init(unified)` replaces workspace list, notifies listeners
- `GetAllAgents()` flattens all workspaces → `[]types.AgentInfo` with enriched metadata (connName, connAbbr, wsLabel, etc.)
- `GetFilteredAgents(connName, wsId)` → filter, sort by status priority, return top 10
- `GetMachines()` → unique machine references in connection order
- `GetSpaces(connName)` → unique workspace references for a given machine
- `SetK11Mode(mode)` → sets K11 filter mode (`"all"` = show all, `"active"` = only BLOCKED/WORKING/DONE)
- `GetAllSpaces()` → unique workspace labels across ALL machines (deduplicated by label, not wsID)
- `ComputeStats()` → global agent state tallies (D/I/W/B/?)

### `pkg/mapper/mapper.go`

- Three filter modes: `ModeAll`, `ModeMachine`, `ModeSpace`
- `SetAll()` → clears all filters, switches to ALL mode
- `NextMachine()` → cycles to next machine (first call from ALL goes to first machine)
- `NextSpace()` → cycles to next workspace globally (label-based, all machines). Clears machine filter. No longer scoped to current machine
- `RenderAll()` → returns 14 `types.KeyCommand` (10 agents + K11 ALL + K12 machine + K13 space + K14 stats)

### `pkg/render/render.go`

- `RenderAgentKey(data)` → two-segment SVG layout: top bar (agent brand + machine color), status background, alias, status letter, workspace name
- `RenderNavAll(data)` → "ALL" button (blue when active, gray when inactive)
- `RenderNavMachine(data)` → machine color background + abbreviation + next machine hint
- `RenderNavSpace(data)` → space name (auto-line-break on dash/underscore), uppercase
- `RenderStatsKey(stats)` → K14 stats bar:
  - Each item: colored `<text>` for letter (text-anchor=end) + white `<text>` for number (text-anchor=start, 4px gap)
  - Items spaced 65px apart, starting from x=370
  - Zero items skipped (except D, shown for symmetry)
- All SVGs use 200×200 viewBox (K14 uses 400×200 for wide key)
- Final output is `data:image/svg+xml;base64,...` data URI

### `pkg/render/colors.go`

- `AgentColors` map → hex string per agent type
- `StatusColors` map → hex string per status: done, idle, working, blocked, unknown

### `pkg/render/icons.go`

- `AgentIcons()` → map of SVG path strings for each agent type
- All paths are white single-color for 200×200 viewBox

### `pkg/deck/client.go`

- WebSocket client connecting to `ws://<addr>:<port>` (default 127.0.0.1:3906)
- Connects as plugin UUID (`com.ulanzi.herdr.agentview`) — needed for keydown event routing
- `SetKeyImage(key, svgDataURI, wide)` → base64-decode SVG → PNG via tdewolff/canvas → state command
- Handles `add`, `keydown`, `keyup`, `run`, `clear` events from UlanziDeck
- Seeds key→actionid mapping from profile (fallback if `add` events haven't arrived)
- Debug logging via zerolog (internal `c.log()` helper)

### `pkg/deck/draw.go`

- `SVGToPNG(svgData, width, height)` → parses SVG XML, draws elements using `tdewolff/canvas` API
- Supports: `<rect>` (fill color, opacity, rounded corners), `<text>` (font-size, font-weight, text-anchor, fill)
- Handles multi-line SVG element declarations (attributes on separate lines)
- Fonts: system sans-serif (Helvetica/Arial → Liberation Sans fallback)
- Output: PNG bytes (not file)

### `pkg/appstate/store.go`

- Central state store wrapping `state.Manager` and `mapper.Mapper`
- Business methods: `SetAll()`, `NextMachine()`, `NextSpace()`, `RefreshHerdrData()`
- `Capture()` → builds `Snapshot` with top agents, mode, machine/space IDs, stats, and visual hash
- `ChangedSince(prevHash)` → hash comparison for render dedup
- `ForceDirty()` → force re-render (used after reconnect)

### `pkg/profile/manager.go`

- Manages UlanziDeck D200X profile lifecycle
- `Ensure(deviceUUID)` → removes old duplicates, creates fresh profile with all 14 keys assigned
- `GetKeyActionMap()` → reads key→actionID mapping from first page of profile
- `ActivateProfile(deviceUUID)` → writes profile name to Ulanzi's `setting.json`
- Creates 4 pages in profile (same layout on each)
- Profile path: `~/Library/Application Support/Ulanzi/UlanziDeck/ProfilesV2/<uuid>.ulanziProfile/`

### `pkg/types/types.go`

Shared data structures used across packages:

- `AgentStatus` — idle/working/blocked/done/unknown
- `AgentInfo` — pane info with enriched metadata
- `UnifiedWorkspace` — workspace + connection metadata + agents
- `MachineRef` / `SpaceRef` — for navigation
- `AgentStats` — global counts
- `KeyCommand` — union type for 14 key render commands
- `AgentKeyData`, `NavAllData`, `NavMachineData`, `NavSpaceData`, `StatsData`, `EmptyKeyData` — render data per key type

---

## Module Details (JavaScript)

### `index.js`

- Entry point: parses positional args (`addr`, `port`, `locale`)
- Loads config → starts connection manager → creates bridge → fetches herdr data
- Creates D200X profile → connects deck client → renders all 14 keys
- Keydown handler routes K11/K12/K13 to filter functions, K1-K10 to `agent.focus`
- Registers `stateManager.onChange()` for auto-refresh
- Console-based debug output for each render cycle

### `config.js`

- `loadConfig()` → reads `~/.config/herdr-deck/connections.json`
- Auto-creates default config if not found
- `findLocalSocket()` → checks common herdr socket paths (with `$HERDR_SOCKET_PATH` env support)

### `connection-manager.js`

- `startAll(config)` → iterates connections, calls `startConnection()` for each
- `startConnection(cfg)`:
  - local: calls `findLocalSocket()` → creates `HerdrClient`
  - ssh: spawns `ssh -L /tmp/herdr-<name>.sock:<remoteSocket> <host> -N`, waits for socket file, creates `HerdrClient`
- `stopAll()` → kills all SSH tunnel processes
- Returns `{ name, abbr, color, client }` per connection

### `herdr-client.js`

- `request(method, params)` → opens new Unix socket or TCP connection, sends JSON-line, parses response
- `_connect()` → `net.createConnection({ path })` for string target, `net.createConnection({ host, port })` for object target
- Convenience methods: `listWorkspaces()`, `listAgents()`, `listPanes()`
- `subscribe(params, onEvent)` → long-lived subscription connection (acks first response, events subsequent)

### `herdr-bridge.js`

- `addConnection(name, abbr, color, socketOrClient)` → registers connection
- `fetchAll()` → calls `listWorkspaces()` + `listAgents()` on each connection, merges into `UnifiedWorkspace[]`
- `focusAgent(connName, paneId)` → sends `agent.focus` to correct connection

### `state-manager.js`

- `init(unifiedWorkspaces)` → stores all workspaces
- `getFilteredAgents(connName, wsId)` → filter by machine + space, sort by status priority, return top 10
- `getMachines()` → unique machines in config order
- `getSpaces(connName)` → unique workspaces for a given machine
- `computeStats()` → global counts for K14
- `onChange(fn)` → registers listener; called on `init()`

### `button-mapper.js`

- Three filter modes: `"all"` | `"machine"` | `"space"`
- `setAll()` → clears filters
- `nextMachine()` → cycles to next machine, clears space filter
- `nextSpace()` → cycles next space (no-op in ALL mode)
- `renderAll()` → returns 14 key descriptors (10 agents + 4 nav/stats)

### `icon-renderer.js`

- SVG generation for all key types (same layout as Go `render.go`)
- `renderStatsKey(stats)` → generates SVG with status-colored text
- Output: `data:image/svg+xml;base64,...` data URI

### `deck-client.js`

- WebSocket → `ws://127.0.0.1:3906`
- Connects as plugin UUID for keydown event routing
- `setKeyImage(key, svgDataUri, wide)` → SVG→PNG via **sharp** → state command
- Handles `add`, `keydown`, `run`, `clear` events
- Seeds key→actionid mapping from profile
- Auto-reconnect on disconnect (2s retry)

### `profile-manager.js`

- Creates "Herdr Deck" profile with 14 D200X keys assigned to our action
- 4 pages in profile (same layout on each)
- Reads key→actionid map from profile files
- Cleans up old duplicate profiles before creating new one

### `mock-data.js`

- `buildMockUnifiedWorkspaces()` → returns `UnifiedWorkspace[]` with 6 workspaces, 15 agents across 3 machines
- Used as fallback when herdr is unreachable
