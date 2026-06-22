// Package viewmodel defines the display-oriented types used by the renderer.
// These are deck-internal types, not part of the protocol module.
package viewmodel

import "github.com/herdr-deck/herdrdeck/protocol"

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
	StatusDuration string `json:"statusDuration"`
}

// NavAllData describes the K11 ALL/ACT button with machine status blocks.
type NavAllData struct {
	KeyID         string                    `json:"keyId"`
	Type          string                    `json:"type"`
	Label         string                    `json:"label"`
	Active        bool                      `json:"active"`
	Filtered      bool                      `json:"filtered"`
	CPUPercent    float64                   `json:"cpuPercent"`
	MemoryPercent float64                   `json:"memPercent"`
	Machines      []protocol.MachineInfo    `json:"machines,omitempty"`
	AgentCounts   map[string]int            `json:"agentCounts,omitempty"`
}
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
	KeyID        string `json:"keyId"`
	Type         string `json:"type"`
	Count        int    `json:"count"`
	CurrentLabel string `json:"currentLabel"`
	NextLabel    string `json:"nextLabel"`
	Active       bool   `json:"active"`
}
// MachineStats holds per-machine agent counts within one space.
type MachineStats struct {
	Abbr   string `json:"abbr"`
	Color  string `json:"color"`
	Total  int    `json:"total"`
	// Non-zero status counts: key=status string, value=count
	// e.g. {"done":3, "idle":1, "working":1}
	Stats map[string]int `json:"stats"`
}

// SpaceStats holds the agent breakdown for one workspace label.
type SpaceStats struct {
	Label    string         `json:"label"`
	Machines []MachineStats `json:"machines"`
	Total    int            `json:"total"`
}

// StatsData describes the K14 stats bar with CPU/MEM and space breakdown.
type StatsData struct {
	KeyID         string              `json:"keyId"`
	Type          string              `json:"type"`
	Stats         protocol.AgentStats `json:"stats"`
	CPUPercent    float64             `json:"cpuPercent"`
	MemoryPercent float64             `json:"memPercent"`
	Spaces        []SpaceStats        `json:"spaces,omitempty"`
}

// EmptyKeyData describes an unused key slot.
type EmptyKeyData struct {
	KeyID string `json:"keyId"`
	Type  string `json:"type"`
}

// KeyCommand is a union type returned by Builder.RenderAll.
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
