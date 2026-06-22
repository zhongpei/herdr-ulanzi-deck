package displaymodel

import (
	"sort"

	"github.com/herdr-deck/herdrdeck/protocol"
)

// Builder holds the current ViewState and produces display models from
// fleet snapshots. It is the single source of truth for navigation and
// filter semantics shared between deck and pet.
type Builder struct {
	state ViewState
}

// NewBuilder creates a builder in ModeAll with no filters.
func NewBuilder() *Builder {
	return &Builder{
		state: ViewState{
			Mode: ModeAll,
		},
	}
}

// State returns a copy of the current view state.
func (b *Builder) State() ViewState { return b.state }

// SetState replaces the entire view state.
func (b *Builder) SetState(s ViewState) { b.state = s }

// SetAll resets navigation to ModeAll and clears all filters.
func (b *Builder) SetAll() {
	b.state.Mode = ModeAll
	b.state.SelectedMachine = ""
	b.state.SelectedSpace = ""
}

// ToggleActiveOnly inverts the ActiveOnly filter flag.
func (b *Builder) ToggleActiveOnly() {
	b.state.ActiveOnly = !b.state.ActiveOnly
}

// SetActiveOnly explicitly sets the ActiveOnly filter flag.
func (b *Builder) SetActiveOnly(v bool) {
	b.state.ActiveOnly = v
}

// NextMachine advances to the next machine in the cycle or selects the first
// machine if none is selected. Clears SelectedSpace and sets Mode to ModeMachine.
func (b *Builder) NextMachine(snap *protocol.FleetSnapshot) {
	if snap == nil || len(snap.Machines) == 0 {
		return
	}
	machines := snap.Machines
	if b.state.SelectedMachine == "" {
		b.state.SelectedMachine = machines[0].Name
	} else {
		idx := findMachineIndex(machines, b.state.SelectedMachine)
		if idx < 0 {
			b.state.SelectedMachine = machines[0].Name
		} else {
			b.state.SelectedMachine = machines[(idx+1)%len(machines)].Name
		}
	}
	b.state.Mode = ModeMachine
	b.state.SelectedSpace = ""
}

// NextSpace advances to the next workspace in the cycle or selects the first
// space if none is selected. Clears SelectedMachine and sets Mode to ModeSpace.
func (b *Builder) NextSpace(snap *protocol.FleetSnapshot) {
	if snap == nil {
		return
	}
	spaces := extractUniqueSpaces(snap.Agents)
	if len(spaces) == 0 {
		return
	}
	if b.state.SelectedSpace != "" {
		found := false
		for _, s := range spaces {
			if s == b.state.SelectedSpace {
				found = true
				break
			}
		}
		if !found {
			b.state.SelectedSpace = ""
		}
	}
	if b.state.SelectedSpace == "" {
		b.state.SelectedSpace = spaces[0]
	} else {
		idx := findSpaceIndex(spaces, b.state.SelectedSpace)
		if idx < 0 {
			b.state.SelectedSpace = spaces[0]
		} else {
			b.state.SelectedSpace = spaces[(idx+1)%len(spaces)]
		}
	}
	b.state.Mode = ModeSpace
	b.state.SelectedMachine = ""
}

// Build produces the complete display Model from a fleet snapshot, local
// stats, and a duration map. The duration map key is "machineName|paneID".
//
// Build is purely functional with respect to the snapshot: the same inputs
// always produce the same output (modulo the builder's ViewState).
func (b *Builder) Build(snap *protocol.FleetSnapshot, local LocalStats, durations map[string]string) Model {
	if snap == nil {
		snap = &protocol.FleetSnapshot{}
	}
	if durations == nil {
		durations = make(map[string]string)
	}

	// Enrich agents with machine abbr/color
	machineMap := makeMachineMap(snap.Machines)
	agents := enrichAgents(snap.Agents, machineMap)

	// Apply filters (machine, space, active-only)
	agents = b.filterAgents(agents)

	// Sort by status priority (blocked > done > working > idle > unknown)
	sortAgents(agents)

	// Truncate to 10
	if len(agents) > 10 {
		agents = agents[:10]
	}

	// Apply durations
	for i := range agents {
		key := agents[i].ConnName + "|" + agents[i].PaneID
		if d, ok := durations[key]; ok {
			agents[i].StatusDuration = d
		}
	}

	// Cycle position helpers
	machines := snap.Machines
	machineIdx := -1
	if b.state.SelectedMachine != "" {
		machineIdx = findMachineIndex(machines, b.state.SelectedMachine)
	}

	spaces := extractUniqueSpaces(snap.Agents)
	spaceIdx := -1
	if b.state.SelectedSpace != "" {
		spaceIdx = findSpaceIndex(spaces, b.state.SelectedSpace)
	}

	// K11 label
	k11Label := "ALL"
	if b.state.ActiveOnly {
		k11Label = "ACT"
	}

	// K12 machine data
	curMachine := machineCurrent(machines, machineIdx)
	nextMachine := machineNext(machines, machineIdx)
	currentAbbr := "-"
	currentColor := "#888"
	nextAbbr := "-"
	if curMachine != nil {
		currentAbbr = curMachine.Abbr
		currentColor = curMachine.Color
	}
	if nextMachine != nil {
		nextAbbr = nextMachine.Abbr
	}

	// K13 space data
	currentSpaceLabel := "-"
	nextSpaceLabel := "-"
	if spaceIdx >= 0 && spaceIdx < len(spaces) {
		currentSpaceLabel = spaces[spaceIdx]
	}
	if n := spaceNextIdx(spaces, spaceIdx); n >= 0 && n < len(spaces) {
		nextSpaceLabel = spaces[n]
	}

	return Model{
		Mode:   b.state.Mode,
		Agents: agents,
		NavAll: NavAll{
			Label:    k11Label,
			Active:   b.state.Mode == ModeAll,
			Filtered: b.state.ActiveOnly,
		},
		NavMachine: NavMachine{
			CurrentAbbr:  currentAbbr,
			CurrentColor: currentColor,
			NextAbbr:     nextAbbr,
			Active:       b.state.Mode == ModeMachine,
		},
		NavSpace: NavSpace{
			Count:        len(spaces),
			CurrentLabel: currentSpaceLabel,
			NextLabel:    nextSpaceLabel,
			Active:       b.state.Mode == ModeSpace,
		},
		Stats: Stats{
			AgentStats:    snap.Stats,
			CPUPercent:    local.CPUPercent,
			MemoryPercent: local.MemoryPercent,
		},
		Machines: snap.Machines,
	}
}

// ─── Filtering ──────────────────────────────────────────────

func (b *Builder) filterAgents(agents []AgentCell) []AgentCell {
	if len(agents) == 0 {
		return agents
	}

	result := agents

	// Machine filter
	if b.state.SelectedMachine != "" && b.state.SelectedSpace == "" {
		var filtered []AgentCell
		for _, a := range result {
			if a.ConnName == b.state.SelectedMachine {
				filtered = append(filtered, a)
			}
		}
		result = filtered
	}

	// Space filter (global, by label)
	if b.state.SelectedSpace != "" {
		var filtered []AgentCell
		for _, a := range result {
			if a.WsLabel == b.state.SelectedSpace {
				filtered = append(filtered, a)
			}
		}
		result = filtered
	}

	// Active-only filter (blocked / working / done)
	if b.state.ActiveOnly {
		var filtered []AgentCell
		for _, a := range result {
			if a.Status == protocol.StatusBlocked ||
				a.Status == protocol.StatusWorking ||
				a.Status == protocol.StatusDone {
				filtered = append(filtered, a)
			}
		}
		result = filtered
	}

	return result
}

// ─── Helpers ────────────────────────────────────────────────

func findMachineIndex(machines []protocol.MachineInfo, name string) int {
	for i, m := range machines {
		if m.Name == name {
			return i
		}
	}
	return -1
}

func findSpaceIndex(spaces []string, label string) int {
	for i, s := range spaces {
		if s == label {
			return i
		}
	}
	return -1
}

func machineCurrent(machines []protocol.MachineInfo, idx int) *protocol.MachineInfo {
	if idx >= 0 && idx < len(machines) {
		return &machines[idx]
	}
	return nil
}

func machineNext(machines []protocol.MachineInfo, idx int) *protocol.MachineInfo {
	if len(machines) == 0 {
		return nil
	}
	if idx < 0 || idx >= len(machines) {
		// No selection → point to first machine as "next"
		return &machines[0]
	}
	return &machines[(idx+1)%len(machines)]
}

func spaceNextIdx(spaces []string, idx int) int {
	if len(spaces) == 0 {
		return -1
	}
	if idx < 0 || idx >= len(spaces) {
		return 0
	}
	return (idx + 1) % len(spaces)
}

func makeMachineMap(machines []protocol.MachineInfo) map[string]protocol.MachineInfo {
	m := make(map[string]protocol.MachineInfo, len(machines))
	for _, mac := range machines {
		m[mac.Name] = mac
	}
	return m
}

func enrichAgents(agents []protocol.AgentState, machineMap map[string]protocol.MachineInfo) []AgentCell {
	cells := make([]AgentCell, len(agents))
	for i, a := range agents {
		mac := machineMap[a.Machine]
		cells[i] = AgentCell{
			PaneID:        a.PaneID,
			Agent:         a.Agent,
			Name:          coalesce(a.Name, a.TabLabel, a.Agent, ""),
			Status:        a.Status,
			Focused:       a.Focused,
			ConnName:      a.Machine,
			ConnAbbr:      mac.Abbr,
			ConnAbbrColor: mac.Color,
			WsLabel:       a.Workspace,
		}
	}
	return cells
}

func extractUniqueSpaces(agents []protocol.AgentState) []string {
	var spaces []string
	seen := make(map[string]bool)
	for _, a := range agents {
		if a.Workspace == "" || seen[a.Workspace] {
			continue
		}
		seen[a.Workspace] = true
		spaces = append(spaces, a.Workspace)
	}
	return spaces
}

func sortAgents(agents []AgentCell) {
	sort.Slice(agents, func(i, j int) bool {
		pi := protocol.StatusPriority[agents[i].Status]
		pj := protocol.StatusPriority[agents[j].Status]
		if pi != pj {
			return pi < pj
		}
		return agents[i].ConnName < agents[j].ConnName
	})
}

func coalesce(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
