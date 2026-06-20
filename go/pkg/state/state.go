// Package state manages the unified workspace tree with sort + filter.
// Mirrors src/state-manager.js
package state

import (
	"sort"

	"github.com/herdr-deck/herdrdeck/pkg/types"
)

// Manager holds the unified workspace list and change listeners.
// No pagination. Agents sorted by priority: BLOCKED > DONE > WORKING > IDLE > UNKNOWN.
// K1-K10 show top 10 after filtering by machine/space.
type Manager struct {
	unified   []types.UnifiedWorkspace
	listeners []func(event string, data any)
}

// NewManager creates an empty state manager.
func NewManager() *Manager {
	return &Manager{
		unified:   nil,
		listeners: nil,
	}
}

// Init replaces the unified workspace list and notifies.
func (m *Manager) Init(unified []types.UnifiedWorkspace) {
	m.unified = unified
	m.notify("stateChanged", nil)
}

// GetAllAgents flattens all workspaces into a single agent list with enriched metadata.
func (m *Manager) GetAllAgents() []types.AgentInfo {
	var agents []types.AgentInfo
	for _, ws := range m.unified {
		for _, a := range ws.Agents {
			a.ConnName = ws.ConnName
			a.ConnAbbr = ws.ConnAbbr
			a.ConnAbbrColor = ws.ConnAbbrColor
			a.WsLabel = ws.Label
			a.WsID = ws.WorkspaceID
			agents = append(agents, a)
		}
	}
	return agents
}

// GetFilteredAgents returns sorted, filtered, truncated (≤10) agent list.
// filterConnName and filterWsId are optional; empty means no filter.
func (m *Manager) GetFilteredAgents(filterConnName, filterWsID string) []types.AgentInfo {
	agents := m.GetAllAgents()

	// Apply machine filter
	if filterConnName != "" {
		var filtered []types.AgentInfo
		for _, a := range agents {
			if a.ConnName == filterConnName {
				filtered = append(filtered, a)
			}
		}
		agents = filtered
	}

	// Apply space filter (intersection with machine filter)
	if filterWsID != "" {
		var filtered []types.AgentInfo
		for _, a := range agents {
			if a.WsID == filterWsID {
				filtered = append(filtered, a)
			}
		}
		agents = filtered
	}

	// Sort: by status priority (lower = higher priority), then by machine name
	sort.Slice(agents, func(i, j int) bool {
		pi := types.StatusPriority[agents[i].AgentStatus]
		pj := types.StatusPriority[agents[j].AgentStatus]
		if pi != pj {
			return pi < pj
		}
		return agents[i].ConnName < agents[j].ConnName
	})

	// Truncate to K1-K10
	if len(agents) > 10 {
		agents = agents[:10]
	}
	return agents
}

// GetMachines returns unique machine references in connection order.
func (m *Manager) GetMachines() []types.MachineRef {
	var machines []types.MachineRef
	seen := make(map[string]bool)
	for _, ws := range m.unified {
		if seen[ws.ConnName] {
			continue
		}
		seen[ws.ConnName] = true
		machines = append(machines, types.MachineRef{
			ConnName:      ws.ConnName,
			ConnAbbr:      ws.ConnAbbr,
			ConnAbbrColor: ws.ConnAbbrColor,
		})
	}
	return machines
}

// GetSpaces returns unique workspace references for a given machine.
func (m *Manager) GetSpaces(connName string) []types.SpaceRef {
	var spaces []types.SpaceRef
	for _, ws := range m.unified {
		if ws.ConnName == connName {
			spaces = append(spaces, types.SpaceRef{
				WsID:  ws.WorkspaceID,
				Label: ws.Label,
			})
		}
	}
	return spaces
}

// ComputeStats returns agent state tallies across ALL workspaces.
func (m *Manager) ComputeStats() types.AgentStats {
	var stats types.AgentStats
	for _, ws := range m.unified {
		for _, a := range ws.Agents {
			switch a.AgentStatus {
			case types.StatusDone:
				stats.Done++
			case types.StatusIdle:
				stats.Idle++
			case types.StatusWorking:
				stats.Working++
			case types.StatusBlocked:
				stats.Blocked++
			default:
				stats.Unknown++
			}
		}
	}
	return stats
}

// OnChange registers a listener for state changes.
func (m *Manager) OnChange(fn func(event string, data any)) {
	m.listeners = append(m.listeners, fn)
}

func (m *Manager) notify(event string, data any) {
	for _, fn := range m.listeners {
		fn(event, data)
	}
}
