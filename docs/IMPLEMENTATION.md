# Implementation — Code Architecture

> Module-by-module description of the current implementation.

## File Map

```
src/
├── index.js              Entry: lifecycle, event routing, config → render
├── config.js             Read ~/.config/herdr-deck/connections.json
├── connection-manager.js SSH tunnel management + multi-connection pool
├── herdr-client.js       Unix socket JSON-line client (one conn per request)
├── herdr-bridge.js       Multi-connection data merge → UnifiedWorkspace[]
├── state-manager.js      State tree: sort (by status), filter (by machine/space)
├── button-mapper.js      Filter mode machine → 14× key render commands
├── icon-renderer.js      SVG generation → PNG via sharp
├── deck-client.js        WebSocket → UlanziDeck (state commands + keydown events)
├── profile-manager.js    Auto-create D200X profile with 14 keys assigned
├── mock-data.js          Fallback mock data when herdr unavailable
scripts/
├── deploy-and-run.sh     Kill old → sync files → start plugin
tests/
├── filter-buttons.test.js  Unit tests for K11/K12/K13 (20 cases)
```

## Module Details

### config.js

- `loadConfig()` → reads `~/.config/herdr-deck/connections.json`
- Auto-creates default config if not found
- `findLocalSocket()` → checks common herdr socket paths

### connection-manager.js

- `startAll(config)` → iterates connections, calls `startConnection()` for each
- local: calls `findLocalSocket()` → creates `HerdrClient`
- ssh: spawns `ssh -L <localSock>:<remoteSock> <host> -N`, waits for socket, creates `HerdrClient`
- Returns `{ name, abbr, color, client }` per connection

### herdr-client.js

- `request(method, params)` → opens new Unix socket connection, sends JSON-line request, parses response
- One connection per request (matches herdr's Rust API client behavior)
- For SSH tunnel connections: uses `{ host, port }` TCP target instead of socket path
- Convenience methods: `listWorkspaces()`, `listAgents()`

### herdr-bridge.js

- `addConnection(name, abbr, color, socketOrClient)` → registers a connection
- `fetchAll()` → calls `listWorkspaces()` + `listAgents()` on each connection
- Merges agents into workspaces, tags with `connName`/`connAbbr`/`connAbbrColor`
- Returns `UnifiedWorkspace[]`
- `focusAgent(connName, paneId)` → routes agent.focus to correct connection

### state-manager.js

- `init(unifiedWorkspaces)` → stores all workspaces
- `getFilteredAgents(connName, wsId)` → filters by machine + space, sorts by status priority, returns top 10
- `getMachines()` → unique machines in config order
- `getSpaces(connName)` → unique workspaces for a given machine
- `computeStats()` → global counts for K14

### button-mapper.js

- Three filter modes: `"all"` | `"machine"` | `"space"`
- `setAll()` → clears filters
- `nextMachine()` → cycles to next machine, clears space filter
- `nextSpace()` → cycles next space under current machine (no-op in ALL mode)
- `renderAll()` → returns 14 key descriptors (10 agents + 4 nav/stats)

### icon-renderer.js

- `renderAgentKey(data)` → two-segment layout: top bar (agent brand + machine color), status background, alias text, status letter, ws name
- `renderNavAll(data)` → "ALL" button
- `renderNavMachine(data)` → machine color bg + abbreviation
- `renderNavSpace(data)` → auto-line-break space name, uppercase
- `renderStatsKey(stats)` → D/I/W/B/? colored text at bottom-right

### deck-client.js

- WebSocket → `ws://127.0.0.1:3906`
- Connects as plugin UUID (4-seg main service) for keydown event routing
- `setKeyImage(key, svgDataUri, wide)` → SVG→PNG via sharp → state command
- Handles `add`, `keydown`, `run`, `clear` events
- Seeds key→actionid mapping from profile

### profile-manager.js

- Creates "Herdr Deck" profile with all 14 D200X keys assigned to our action
- 4 pages in profile (same layout on each)
- Reads key→actionid map from profile for state commands

### index.js

- Startup: load config → connection-manager → herdr-bridge → state init → profile → deck → render
- Fallback: if no herdr connections, use mock data
- `renderAll()` → generates 14× PNGs → sends as batch via Promise.all
- `handleKeyDown()` → routes K11/K12/K13 to filter functions, K1-K10 to agent.focus
