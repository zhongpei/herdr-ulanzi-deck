// Package state manages the unified workspace tree with sort + filter.
// Mirrors src/state-manager.js
package state

import (
	"fmt"
	"sort"
	"time"

	"github.com/herdr-deck/herdrdeck/pkg/types"
)

// Manager holds the unified workspace list and change listeners.
// No pagination. Agents sorted by priority: BLOCKED > DONE > WORKING > IDLE > UNKNOWN.
// K1-K10 show top 10 after filtering by machine/space.
// Also stores system CPU/memory stats for the K11 ALL button.
type Manager struct {
	unified       []types.UnifiedWorkspace
	cpuPercent    float64
	memoryPercent float64
	listeners     []func(event string, data any)
	statusSince   map[string]time.Time // "connName|paneID" → when current status started
}

// NewManager creates an empty state manager.
func NewManager() *Manager {
	return &Manager{
		unified:     nil,
		listeners:   nil,
		statusSince: make(map[string]time.Time),
	}
}

// Init replaces the unified workspace list and notifies.
// Tracks status change timestamps so FormatAgentDuration can report how long
// each agent has been in its current status.
func (m *Manager) Init(unified []types.UnifiedWorkspace) {
	now := time.Now()
	old := m.buildAgentStatusMap() // connName|paneID → old status

	for _, ws := range unified {
		for _, a := range ws.Agents {
			key := a.ConnName + "|" + a.PaneID
			oldStatus, exists := old[key]
			if !exists || string(oldStatus) != string(a.AgentStatus) {
				// New agent or status changed → reset timer
				m.statusSince[key] = now
			} // else: status unchanged → keep original start time
			delete(old, key) // marked as seen
		}
	}

	// Remove stale entries (agents that no longer exist in any workspace)
	for key := range old {
		delete(m.statusSince, key)
	}

	m.unified = unified
	m.notify("stateChanged", nil)
}

// buildAgentStatusMap returns connName|paneID → current status for all agents.
func (m *Manager) buildAgentStatusMap() map[string]types.AgentStatus {
	result := make(map[string]types.AgentStatus)
	for _, ws := range m.unified {
		for _, a := range ws.Agents {
			key := a.ConnName + "|" + a.PaneID
			result[key] = a.AgentStatus
		}
	}
	return result
}

// FormatAgentDuration returns a human-readable duration string for how long
// an agent has been in its current status. Shows "0m" for < 1 minute so the
// user can see the display is live immediately.
func (m *Manager) FormatAgentDuration(connName, paneID string) string {
	key := connName + "|" + paneID
	since, ok := m.statusSince[key]
	if !ok {
		return "0m"
	}
	d := time.Since(since)
	if d < time.Minute {
		return "0m"
	}
	return formatDuration(d)
}

// formatDuration formats a duration for display on agent keys.
// Examples: "3m", "45m", "1h02m", "2d"
func formatDuration(d time.Duration) string {
	totalMin := int(d.Minutes())
	if totalMin < 60 {
		return fmt.Sprintf("%dm", totalMin)
	}
	hours := totalMin / 60
	mins := totalMin % 60
	if hours < 24 {
		return fmt.Sprintf("%dh%02dm", hours, mins)
	}
	days := hours / 24
	hours = hours % 24
	return fmt.Sprintf("%dd%dh", days, hours)
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

// SetSysStats updates the latest system CPU/memory percentages.
func (m *Manager) SetSysStats(cpu, mem float64) {
	m.cpuPercent = cpu
	m.memoryPercent = mem
}

// GetSysStats returns the latest system CPU and memory percentages.
func (m *Manager) GetSysStats() (cpu, mem float64) {
	return m.cpuPercent, m.memoryPercent
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
