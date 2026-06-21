package protocol

// MachineInfo carries connection identity metadata for a single machine
// that collector is pulling state from.
type MachineInfo struct {
	Name       string `json:"name"`        // Internal identifier
	Abbr       string `json:"abbr"`        // Abbreviation shown on K12
	Color      string `json:"color"`       // Machine color for K12 background
	Health     string `json:"health"`      // "online", "offline"
	LastError  string `json:"last_error,omitempty"`
	LastSeenAt string `json:"last_seen_at,omitempty"`
}

// AgentState is a flat, machine-enriched agent record sent from collector to
// display processes. It replaces the old UnifiedWorkspace nested tree.
//
// ID is the stable unique key: "machineName|paneID".
type AgentState struct {
	ID          string      `json:"id"`          // "machineName|paneID"
	Machine     string      `json:"machine"`     // connection name (matches MachineInfo.Name)
	Agent       string      `json:"agent"`       // agent type (pi, claude, cursor, ...)
	Name        string      `json:"name"`        // alias / pane label / tab label
	Status      AgentStatus `json:"status"`
	Focused     bool        `json:"focused"`
	Workspace   string      `json:"workspace"`   // workspace label (display name)
	WorkspaceID string      `json:"workspace_id"`
	TabLabel    string      `json:"tab_label,omitempty"`
	PaneID      string      `json:"pane_id"`
	UpdatedAt   string      `json:"updated_at"` // RFC3339 timestamp
}

// AgentStats holds cross-workspace agent state tallies (for K14).
type AgentStats struct {
	Done    int `json:"done"`
	Idle    int `json:"idle"`
	Working int `json:"working"`
	Blocked int `json:"blocked"`
	Unknown int `json:"unknown"`
}

// FleetSnapshot is the complete fleet state published by collector on NATS.
// It contains all machines, all agents, and aggregated stats.
type FleetSnapshot struct {
	Version   int           `json:"version"`    // SchemaVersion
	Seq       uint64        `json:"seq"`        // monotonic sequence number
	UpdatedAt string        `json:"updated_at"` // RFC3339
	Machines  []MachineInfo `json:"machines"`   // all connected machines
	Agents    []AgentState  `json:"agents"`     // all agents across all machines
	Stats     AgentStats    `json:"stats"`
}
