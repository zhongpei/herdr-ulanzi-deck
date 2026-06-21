// Package fleet maintains the canonical fleet state inside the collector.
//
// Responsibilities:
//   - Accept raw workspace data from bridge
//   - Convert to protocol.FleetSnapshot
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
	seq      uint64
}

// NewStore creates an empty fleet store.
func NewStore() *Store {
	return &Store{
		snapshot: &protocol.FleetSnapshot{
			Version: protocol.SchemaVersion,
		},
	}
}

// ApplyRaw takes fresh raw data from the bridge and updates the fleet state.
// Returns true if the snapshot changed.
func (s *Store) ApplyRaw(raw []bridge.RawWorkspace) bool {
	now := time.Now().UTC().Format(time.RFC3339)

	machineSet := make(map[string]protocol.MachineInfo)
	var agents []protocol.AgentState
	stats := protocol.AgentStats{}

	for _, ws := range raw {
		machineSet[ws.ConnName] = protocol.MachineInfo{
			Name:  ws.ConnName,
			Abbr:  ws.ConnAbbr,
			Color: ws.ConnAbbrColor,
		}

		for _, a := range ws.Agents {
			status := mapStatus(a.Status)
			agent := protocol.AgentState{
				ID:          ws.ConnName + "|" + a.PaneID,
				Machine:     ws.ConnName,
				Agent:       a.Agent,
				Name:        coalesce(a.Name, a.TabLabel, a.Agent, ""),
				Status:      status,
				Focused:     a.Focused,
				Workspace:   ws.Label,
				WorkspaceID: ws.WorkspaceID,
				TabLabel:    a.TabLabel,
				PaneID:      a.PaneID,
				UpdatedAt:   now,
			}
			agents = append(agents, agent)

			switch status {
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

	// Build ordered machine list
	machines := make([]protocol.MachineInfo, 0, len(machineSet))
	for _, ws := range raw {
		name := ws.ConnName
		if m, ok := machineSet[name]; ok {
			machines = append(machines, m)
			delete(machineSet, name)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	snap := &protocol.FleetSnapshot{
		Version:   protocol.SchemaVersion,
		Seq:       s.seq,
		UpdatedAt: now,
		Machines:  machines,
		Agents:    agents,
		Stats:     stats,
	}

	changed := !snapshotEqual(s.snapshot, snap)
	s.snapshot = snap
	return changed
}

// Snapshot returns a copy of the current fleet snapshot.
// Safe for the caller to use concurrently with ApplyRaw.
func (s *Store) Snapshot() protocol.FleetSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copySnapshot(s.snapshot)
}

// copySnapshot deep-copies a FleetSnapshot so callers don't
// share the internal slice headers.
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

// snapshotEqual returns true if two snapshots have the same agent state.
// Ignores Seq, UpdatedAt (top-level), and per-agent UpdatedAt
// because they are set by the collector on every cycle and do not
// represent a semantic state change.
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
	if a.Stats != b.Stats {
		return false
	}
	for i := range a.Agents {
		// Compare all fields except UpdatedAt
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
