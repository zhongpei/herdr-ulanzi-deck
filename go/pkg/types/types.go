// Package types defines shared data structures for herdr-deck.
// Mirrors the TypeScript interfaces in DESIGN.md and JS runtime objects.
package types

// AgentStatus represents the current agent lifecycle state.
type AgentStatus string

const (
	StatusIdle    AgentStatus = "idle"
	StatusWorking AgentStatus = "working"
	StatusBlocked AgentStatus = "blocked"
	StatusDone    AgentStatus = "done"
	StatusUnknown AgentStatus = "unknown"
)

// StatusPriority maps status to sort rank (lower = higher priority).
var StatusPriority = map[AgentStatus]int{
	StatusBlocked: 0,
	StatusDone:    1,
	StatusWorking: 2,
	StatusIdle:    3,
	StatusUnknown: 4,
}

// AgentInfo holds runtime data for a single agent pane.
type AgentInfo struct {
	PaneID       string            `json:"pane_id"`
	TerminalID   string            `json:"terminal_id"`
	WorkspaceID  string            `json:"workspace_id"`
	TabID        string            `json:"tab_id"`
	Agent        string            `json:"agent"`
	Name         string            `json:"name"`
	AgentStatus  AgentStatus       `json:"agent_status"`
	CustomStatus *string           `json:"custom_status"`
	StateLabels  map[string]string `json:"state_labels"`
	Title        *string           `json:"title"`
	DisplayAgent *string           `json:"display_agent"`
	Focused      bool              `json:"focused"`
	Revision     int               `json:"revision"`
	// Enriched at merge time (not from herdr)
	ConnName      string `json:"conn_name,omitempty"`
	ConnAbbr      string `json:"conn_abbr,omitempty"`
	ConnAbbrColor string `json:"conn_abbr_color,omitempty"`
	WsLabel       string `json:"ws_label,omitempty"`
	WsID          string `json:"ws_id,omitempty"`
	TabLabel      string `json:"tab_label,omitempty"`
}

// WorkspaceInfo holds a single workspace returned by herdr workspace.list.
type WorkspaceInfo struct {
	WorkspaceID string      `json:"workspace_id"`
	Label       string      `json:"label"`
	Number      int         `json:"number"`
	Focused     bool        `json:"focused"`
	AgentStatus AgentStatus `json:"agent_status"`
	TabCount    int         `json:"tab_count"`
	ActiveTabID string      `json:"active_tab_id"`
	PaneCount   int         `json:"pane_count"`
}

// UnifiedWorkspace is a workspace enriched with connection metadata + agents.
type UnifiedWorkspace struct {
	ConnName      string      `json:"conn_name"`
	ConnAbbr      string      `json:"conn_abbr"`
	ConnAbbrColor string      `json:"conn_abbr_color"`
	WorkspaceID   string      `json:"workspace_id"`
	Label         string      `json:"label"`
	Number        int         `json:"number"`
	AgentStatus   AgentStatus `json:"agent_status"`
	TabCount      int         `json:"tab_count"`
	PaneCount     int         `json:"pane_count"`
	Agents        []AgentInfo `json:"agents"`
}

// MachineRef identifies a connected herdr machine (for K12 display).
type MachineRef struct {
	ConnName      string `json:"conn_name"`
	ConnAbbr      string `json:"conn_abbr"`
	ConnAbbrColor string `json:"conn_abbr_color"`
}

// SpaceRef identifies a workspace within a machine (for K13 display).
type SpaceRef struct {
	WsID  string `json:"ws_id"`
	Label string `json:"label"`
}

// AgentStats holds cross-workspace agent state tallies (for K14).
type AgentStats struct {
	Done    int `json:"done"`
	Idle    int `json:"idle"`
	Working int `json:"working"`
	Blocked int `json:"blocked"`
	Unknown int `json:"unknown"`
}

// ─── Key data passed to IconRenderer ─────────────────────────

// AgentKeyData describes a single agent key (K1-K10) render command.
type AgentKeyData struct {
	KeyID          string `json:"keyId"`
	Type           string `json:"type"`
	AgentType      string `json:"agentType"`
	Alias          string `json:"alias"`
	Status         string `json:"status"`
	Focused        bool   `json:"focused"`
	PaneID         string `json:"paneId"`
	ConnName       string `json:"connName"`
	ConnAbbr       string `json:"connAbbr"`
	ConnAbbrColor  string `json:"connAbbrColor"`
	WsLabel        string `json:"wsLabel"`
	StatusDuration string `json:"statusDuration"` // formatted: "45m", "1h02m", "2d"
}

// NavAllData describes the K11 ALL button with system stats overlay.
type NavAllData struct {
	KeyID         string  `json:"keyId"`
	Type          string  `json:"type"`
	Label         string  `json:"label"`
	Active        bool    `json:"active"`
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryPercent float64 `json:"memPercent"`
}

// NavMachineData describes the K12 machine-cycle button.
type NavMachineData struct {
	KeyID        string `json:"keyId"`
	Type         string `json:"type"`
	CurrentAbbr  string `json:"currentAbbr"`
	CurrentColor string `json:"currentColor"`
	NextAbbr     string `json:"nextAbbr"`
	Active       bool   `json:"active"`
}

// NavSpaceData describes the K13 space-cycle button.
type NavSpaceData struct {
	KeyID     string `json:"keyId"`
	Type      string `json:"type"`
	Count     int    `json:"count"`
	NextLabel string `json:"nextLabel"`
	Active    bool   `json:"active"`
}

// StatsData describes the K14 stats bar with optional system CPU/memory overlay.
type StatsData struct {
	KeyID         string     `json:"keyId"`
	Type          string     `json:"type"`
	Stats         AgentStats `json:"stats"`
	CPUPercent    float64    `json:"cpuPercent"`
	MemoryPercent float64    `json:"memPercent"`
}

// EmptyKeyData describes an unused key slot.
type EmptyKeyData struct {
	KeyID string `json:"keyId"`
	Type  string `json:"type"`
}

// KeyCommand is a union type returned by ButtonMapper.RenderAll.
// Exactly one of the struct fields is non-nil.
type KeyCommand struct {
	Agent  *AgentKeyData
	NavAll *NavAllData
	NavMac *NavMachineData
	NavSpc *NavSpaceData
	Stats  *StatsData
	Empty  *EmptyKeyData
}

// Type returns the key type string for the active command.
func (k KeyCommand) Type() string {
	switch {
	case k.Agent != nil:
		return k.Agent.Type
	case k.NavAll != nil:
		return k.NavAll.Type
	case k.NavMac != nil:
		return k.NavMac.Type
	case k.NavSpc != nil:
		return k.NavSpc.Type
	case k.Stats != nil:
		return k.Stats.Type
	case k.Empty != nil:
		return k.Empty.Type
	default:
		return "empty"
	}
}
