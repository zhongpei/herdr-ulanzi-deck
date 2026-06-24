// Package fleet manages the display-side agent state, consuming FleetSnapshot
// from NATS. It provides filtered views, stats, and duration tracking.
//
// Mirrors the original state.Manager but consumes protocol.FleetSnapshot
// instead of []types.UnifiedWorkspace.
package fleet

import (
	"fmt"
	"time"

	"github.com/herdr-deck/herdrdeck/protocol"
)
// Manager holds the fleet state, duration tracking, health, and system stats.
// Filtering and navigation semantics are handled by displaymodel.Builder.
//
// SINGLE-GOROUTINE: Manager is only accessed from the main event-loop
// goroutine (deck's runMain select loop). Do not call methods concurrently
// without adding a mutex.
type Manager struct {
	machines      []protocol.MachineInfo
	agents        []protocol.AgentState
	cpuPercent    float64
	memoryPercent float64
	statusSince   map[string]time.Time // "machine|paneID" → when current status started

	// Health tracking
	lastHeartbeat time.Time
	health        ConnectionHealth
}

// ConnectionHealth tracks collector connectivity.
type ConnectionHealth int

const (
	HealthConnected ConnectionHealth = iota
	HealthOffline
)

// NewManager creates an empty fleet manager.
func NewManager() *Manager {
	return &Manager{
		statusSince: make(map[string]time.Time),
	}
}

// ApplySnapshot replaces the fleet state with a new snapshot from the collector.
func (m *Manager) ApplySnapshot(snap *protocol.FleetSnapshot) {
	now := time.Now()
	old := m.buildAgentStatusMap()

	for _, a := range snap.Agents {
		key := a.ID // "machine|paneID"
		oldStatus, exists := old[key]
		if !exists || string(oldStatus) != string(a.Status) {
			m.statusSince[key] = now
		}
		delete(old, key)
	}

	// Remove stale entries
	for key := range old {
		delete(m.statusSince, key)
	}

	m.machines = snap.Machines
	m.agents = snap.Agents
	m.markAlive()
}

// ─── Converters (for backward compatibility with old enriched AgentInfo) ──

// AgentInfo is the internal enriched agent type used by the fleet manager
// for filtering, sorting, and building viewmodel commands.
// It is NOT the protocol.AgentState — it adds machine abbreviation/color
// that were previously in UnifiedWorkspace.
type AgentInfo struct {
	PaneID        string
	Agent         string
	Name          string
	Status        protocol.AgentStatus
	Focused       bool
	ConnName      string
	ConnAbbr      string
	ConnAbbrColor string
	WsLabel       string
	WsID          string
	TabLabel      string
}

// buildEnrichedAgents converts flat AgentState + MachineInfo into enriched
// AgentInfo list with machine abbreviation/color resolved.
func (m *Manager) buildEnrichedAgents() []AgentInfo {
	machineMap := make(map[string]protocol.MachineInfo, len(m.machines))
	for _, mac := range m.machines {
		machineMap[mac.Name] = mac
	}

	agents := make([]AgentInfo, len(m.agents))
	for i, a := range m.agents {
		mac := machineMap[a.Machine]
		agents[i] = AgentInfo{
			PaneID:        a.PaneID,
			Agent:         a.Agent,
			Name:          a.Name,
			Status:        a.Status,
			Focused:       a.Focused,
			ConnName:      a.Machine,
			ConnAbbr:      mac.Abbr,
			ConnAbbrColor: mac.Color,
			WsLabel:       a.Workspace,
			WsID:          a.WorkspaceID,
			TabLabel:      a.TabLabel,
		}
	}
	return agents
}

// GetAllAgents returns all enriched agents.
func (m *Manager) GetAllAgents() []AgentInfo {
	return m.buildEnrichedAgents()
}

// GetMachines returns unique machine references in connection order.
func (m *Manager) GetMachines() []protocol.MachineInfo {
	return m.machines
}

// SpaceRef identifies a workspace by label (for K13 display).
// Label is the canonical key for cross-machine matching.
type SpaceRef struct {
	WsID  string `json:"ws_id,omitempty"`
	Label string `json:"label"`
}

// GetAllSpaces returns unique workspace labels across ALL machines.
func (m *Manager) GetAllSpaces() []SpaceRef {
	var spaces []SpaceRef
	seen := make(map[string]bool)
	for _, a := range m.agents {
		if seen[a.Workspace] {
			continue
		}
		seen[a.Workspace] = true
		spaces = append(spaces, SpaceRef{
			WsID:  a.WorkspaceID,
			Label: a.Workspace,
		})
	}
	return spaces
}

// ComputeStats returns agent state tallies.
func (m *Manager) ComputeStats() protocol.AgentStats {
	var stats protocol.AgentStats
	for _, a := range m.agents {
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
	return stats
}

// ─── Duration tracking ─────────────────────────────────────

// FormatAgentDuration returns a human-readable duration string.
func (m *Manager) FormatAgentDuration(machine, paneID string) string {
	key := machine + "|" + paneID
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

// ─── System stats ──────────────────────────────────────────

func (m *Manager) SetSysStats(cpu, mem float64) {
	m.cpuPercent = cpu
	m.memoryPercent = mem
}

func (m *Manager) GetSysStats() (cpu, mem float64) {
	return m.cpuPercent, m.memoryPercent
}

// ─── Health ────────────────────────────────────────────────

func (m *Manager) markAlive() {
	m.lastHeartbeat = time.Now()
	m.health = HealthConnected
}

// MarkHeartbeat updates the last heartbeat timestamp.
func (m *Manager) MarkHeartbeat(t time.Time) {
	m.lastHeartbeat = t
	m.health = HealthConnected
}

// CheckHealth returns the current connection health.
// If no heartbeat for 5s, marks offline.
func (m *Manager) CheckHealth() ConnectionHealth {
	if time.Since(m.lastHeartbeat) > 5*time.Second {
		m.health = HealthOffline
	}
	return m.health
}

func (m *Manager) Health() ConnectionHealth { return m.health }

// ─── internal ──────────────────────────────────────────────

func (m *Manager) buildAgentStatusMap() map[string]protocol.AgentStatus {
	result := make(map[string]protocol.AgentStatus)
	for _, a := range m.agents {
		result[a.ID] = a.Status
	}
	return result
}

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
