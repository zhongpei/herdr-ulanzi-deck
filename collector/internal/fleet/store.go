// Package fleet maintains the canonical fleet state inside the collector.
// Responsibilities:
//   - Accept FetchResult from bridge (success+error per connection)
//   - Convert to protocol.FleetSnapshot
//   - Track machine health (online/offline)
//   - Track monotonic sequence numbers
//   - Compute agent stats
package fleet

import (
	"sync"
	"time"

	"github.com/herdr-deck/herdrdeck/collector/internal/bridge"
	"github.com/herdr-deck/herdrdeck/protocol"
)

// Store holds the authoritative fleet state.
type Store struct {
	mu       sync.RWMutex
	snapshot *protocol.FleetSnapshot
	machines map[string]*machineState
	order    []string // insertion order of machine keys, for deterministic output
	seq      uint64
}

type machineState struct {
	name       string
	abbr       string
	color      string
	health     string // "online" or "offline"
	lastError  string
	lastSeenAt string
	agents     []protocol.AgentState
}

// NewStore creates an empty fleet store.
func NewStore() *Store {
	return &Store{
		snapshot: &protocol.FleetSnapshot{
			Version: protocol.SchemaVersion,
		},
		machines: make(map[string]*machineState),
	}
}

// ApplyResults takes fetch results from the bridge and updates the fleet state.
// Failed connections are reflected as machines with Health=offline,
// retaining their last known agents.
func (s *Store) ApplyResults(results []bridge.FetchResult) bool {
	now := time.Now().UTC().Format(time.RFC3339)
	seen := make(map[string]bool)

	for _, r := range results {
		seen[r.ConnName] = true
		ms, exists := s.machines[r.ConnName]
		if !exists {
			ms = &machineState{
				name:  r.ConnName,
				abbr:  r.ConnAbbr,
				color: r.ConnColor,
			}
			s.machines[r.ConnName] = ms
			s.order = append(s.order, r.ConnName)
		}

		if r.Err != nil {
			// Connection failed — mark offline, keep last agents
			ms.health = "offline"
			ms.lastError = r.Err.Error()
			if len(ms.lastError) > 120 {
				ms.lastError = ms.lastError[:120]
			}
			continue
		}

		// Connection succeeded — update agents
		ms.health = "online"
		ms.lastError = ""
		ms.lastSeenAt = now

		var agents []protocol.AgentState
		for _, ws := range r.Workspaces {
			for _, a := range ws.Agents {
				agents = append(agents, protocol.AgentState{
					ID:          ws.ConnName + "|" + a.PaneID,
					Machine:     ws.ConnName,
					Agent:       a.Agent,
					Name:        coalesce(a.Name, a.TabLabel, a.Agent, ""),
					Status:      mapStatus(a.Status),
					Focused:     a.Focused,
					Workspace:   ws.Label,
					WorkspaceID: ws.WorkspaceID,
					TabLabel:    a.TabLabel,
					PaneID:      a.PaneID,
					UpdatedAt:   now,
				})
			}
		}
		ms.agents = agents
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Build machines and agents from state (deterministic order)
	machines := make([]protocol.MachineInfo, 0, len(s.order))
	var allAgents []protocol.AgentState
	stats := protocol.AgentStats{}

	for _, name := range s.order {
		ms, ok := s.machines[name]
		if !ok {
			continue
		}
		if !seen[ms.name] {
			continue
		}
		machines = append(machines, protocol.MachineInfo{
			Name:       ms.name,
			Abbr:       ms.abbr,
			Color:      ms.color,
			Health:     ms.health,
			LastError:  ms.lastError,
			LastSeenAt: ms.lastSeenAt,
		})
		allAgents = append(allAgents, ms.agents...)

		for _, a := range ms.agents {
			switch a.Status {
			case protocol.StatusDone:
				stats.Done++
			case protocol.StatusIdle:
				stats.Idle++
			case protocol.StatusWorking:
				stats.Working++
			case protocol.StatusBlocked:
				stats.Blocked++
			default:
				stats.Unknown++
			}
		}
	}

	s.seq++
	snap := &protocol.FleetSnapshot{
		Version:   protocol.SchemaVersion,
		Seq:       s.seq,
		UpdatedAt: now,
		Machines:  machines,
		Agents:    allAgents,
		Stats:     stats,
	}

	changed := !snapshotEqual(s.snapshot, snap)
	s.snapshot = snap
	return changed
}

// ApplyRaw is the old API kept for backward compat with tests.
func (s *Store) ApplyRaw(raw []bridge.RawWorkspace) bool {
	// Preserve raw order when building FetchResults.
	byConn := make(map[string]*bridge.FetchResult)
	var order []string
	for _, ws := range raw {
		if _, ok := byConn[ws.ConnName]; !ok {
			byConn[ws.ConnName] = &bridge.FetchResult{
				ConnName:  ws.ConnName,
				ConnAbbr:  ws.ConnAbbr,
				ConnColor: ws.ConnAbbrColor,
			}
			order = append(order, ws.ConnName)
		}
		byConn[ws.ConnName].Workspaces = append(byConn[ws.ConnName].Workspaces, ws)
	}
	results := make([]bridge.FetchResult, 0, len(order))
	for _, name := range order {
		results = append(results, *byConn[name])
	}
	return s.ApplyResults(results)
}

// Snapshot returns a copy of the current fleet snapshot.
func (s *Store) Snapshot() protocol.FleetSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copySnapshot(s.snapshot)
}

func copySnapshot(src *protocol.FleetSnapshot) protocol.FleetSnapshot {
	dst := *src
	dst.Machines = make([]protocol.MachineInfo, len(src.Machines))
	copy(dst.Machines, src.Machines)
	dst.Agents = make([]protocol.AgentState, len(src.Agents))
	copy(dst.Agents, src.Agents)
	return dst
}

// ─── helpers ────────────────────────────────────────────────

func mapStatus(raw string) protocol.AgentStatus {
	switch raw {
	case "done":
		return protocol.StatusDone
	case "idle":
		return protocol.StatusIdle
	case "working":
		return protocol.StatusWorking
	case "blocked":
		return protocol.StatusBlocked
	default:
		return protocol.StatusUnknown
	}
}

func coalesce(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

func snapshotEqual(a, b *protocol.FleetSnapshot) bool {
	if a == nil || b == nil {
		return false
	}
	if len(a.Agents) != len(b.Agents) {
		return false
	}
	if len(a.Machines) != len(b.Machines) {
		return false
	}
	for i := range a.Machines {
		if a.Machines[i].Name != b.Machines[i].Name ||
			a.Machines[i].Abbr != b.Machines[i].Abbr ||
			a.Machines[i].Color != b.Machines[i].Color ||
			a.Machines[i].Health != b.Machines[i].Health ||
			a.Machines[i].LastError != b.Machines[i].LastError {
			return false
		}
		// LastSeenAt and config-only fields are excluded from comparison
	}
	if a.Stats != b.Stats {
		return false
	}
	for i := range a.Agents {
		if a.Agents[i].ID != b.Agents[i].ID ||
			a.Agents[i].Machine != b.Agents[i].Machine ||
			a.Agents[i].Agent != b.Agents[i].Agent ||
			a.Agents[i].Name != b.Agents[i].Name ||
			a.Agents[i].Status != b.Agents[i].Status ||
			a.Agents[i].Focused != b.Agents[i].Focused ||
			a.Agents[i].Workspace != b.Agents[i].Workspace ||
			a.Agents[i].WorkspaceID != b.Agents[i].WorkspaceID ||
			a.Agents[i].TabLabel != b.Agents[i].TabLabel ||
			a.Agents[i].PaneID != b.Agents[i].PaneID {
			return false
		}
	}
	return true
}
