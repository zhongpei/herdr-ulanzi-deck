// Package displaymodel defines the shared display state model used by
// herdr-deck and herdr-pet. It converts a protocol.FleetSnapshot into a
// generic display model with filtered agents, navigation semantics (K11-K14),
// and stats.
//
// Deck and pet each have their own adapter that projects this model onto
// their respective rendering surfaces (14 hardware keys vs. desktop overlay).
package displaymodel

import "github.com/herdr-deck/herdrdeck/protocol"

// ViewMode represents the current fleet navigation mode.
type ViewMode int

const (
	ModeAll     ViewMode = iota // Show all agents across all machines/spaces
	ModeMachine                 // Show agents on a single machine
	ModeSpace                   // Show agents in a single workspace
)

// ViewState holds the user's navigation and filter preferences.
// It drives how Build() filters and sorts agents.
type ViewState struct {
	Mode            ViewMode
	SelectedMachine string // machine name filter (empty = no machine filter)
	SelectedSpace   string // workspace label filter (empty = no space filter)
	ActiveOnly      bool   // if true, only show blocked/working/done agents
}

// LocalStats holds system resource stats from the display process itself.
type LocalStats struct {
	CPUPercent    float64
	MemoryPercent float64
}

// AgentCell is the display-oriented representation of a single agent.
// It enriches protocol.AgentState with machine abbreviation/color and
// duration string for display purposes.
type AgentCell struct {
	PaneID         string
	Agent          string
	Name           string
	Status         protocol.AgentStatus
	Focused        bool
	ConnName       string
	ConnAbbr       string
	ConnAbbrColor  string
	WsLabel        string
	StatusDuration string
}

// NavAll holds the "ALL / ACT" filter toggle display data (K11 equivalent).
type NavAll struct {
	Label    string // "ALL" or "ACT"
	Active   bool   // true when ViewMode == ModeAll
	Filtered bool   // true when ActiveOnly is enabled
}

// NavMachine holds the machine cycle display data (K12 equivalent).
type NavMachine struct {
	CurrentAbbr  string // abbreviation of currently selected machine, "-" if none
	CurrentColor string // color of currently selected machine
	NextAbbr     string // abbreviation of next machine in cycle
	Active       bool   // true when ViewMode == ModeMachine
}

// NavSpace holds the space (workspace) cycle display data (K13 equivalent).
type NavSpace struct {
	Count        int    // total number of unique workspaces across all machines
	CurrentLabel string // label of currently selected space, "-" if none
	NextLabel    string // label of next space in cycle
	Active       bool   // true when ViewMode == ModeSpace
}

// Stats holds the aggregate agent stats and system resource display data (K14 equivalent).
type Stats struct {
	AgentStats    protocol.AgentStats // tallies across all agents (unfiltered)
	CPUPercent    float64
	MemoryPercent float64
}

// Model is the complete display model produced by Builder.Build().
// It contains filtered/sorted agents for the main grid and four navigation
// panels (K11-K14 equivalents).
type Model struct {
	Mode       ViewMode
	Agents     []AgentCell // filtered, sorted, truncated to ≤10
	NavAll     NavAll
	NavMachine NavMachine
	NavSpace   NavSpace
	Stats      Stats
}
