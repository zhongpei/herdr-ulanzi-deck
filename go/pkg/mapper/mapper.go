// Package mapper implements ButtonMapper: state + filter → 14 key render commands.
// Mirrors src/button-mapper.js
//
// Filter modes:
//
//	ALL         → show all machines (K11 active)
//	Machine     → show one machine's agents (K12 cycles)
//	Machine+WS  → show one workspace's agents (K12 + K13 intersection)
//
// Sort: BLOCKED(0) > DONE(1) > WORKING(2) > IDLE(3) > UNKNOWN(4)
package mapper

import (
	"github.com/herdr-deck/herdrdeck/pkg/state"
	"github.com/herdr-deck/herdrdeck/pkg/types"
)

// FilterMode enumerates the three display modes.
type FilterMode int

const (
	ModeAll     FilterMode = iota // show all machines
	ModeMachine                   // show one machine
	ModeSpace                     // show one workspace (machine + space intersect)
)

// Mapper holds filter state and a reference to the state manager.
type Mapper struct {
	state       *state.Manager
	Mode        FilterMode
	ConnName    string // current machine filter
	WsLabel     string // current space label filter (not wsID — labels match across machines)
	K11Toggle   bool   // true = K11 toggles ALL↔ACTIVE
	K11Filtered bool   // current toggle state mirrored from state.Manager
}

// New creates a Mapper in ALL mode.
func New(sm *state.Manager) *Mapper {
	return &Mapper{
		state: sm,
		Mode:  ModeAll,
	}
}

// SetAll switches to ALL mode and clears all filters.
func (m *Mapper) SetAll() {
	m.Mode = ModeAll
	m.ConnName = ""
	m.WsLabel = ""
}

// NextMachine cycles to the next machine in sequence.
// From ALL → first machine. Within machine mode → next in list.
func (m *Mapper) NextMachine() {
	machines := m.state.GetMachines()
	if len(machines) == 0 {
		return
	}
	if m.ConnName == "" {
		m.ConnName = machines[0].ConnName
	} else {
		idx := findMachineIndex(machines, m.ConnName)
		m.ConnName = machines[(idx+1)%len(machines)].ConnName
	}
	m.Mode = ModeMachine
	m.WsLabel = "" // clear space filter
}

// NextSpace cycles to the next workspace (space) globally, independent of machine.
// Uses Label (workspace name) for matching, not wsID — so the same project
// on different machines is treated as one space.
func (m *Mapper) NextSpace() {
	spaces := m.state.GetAllSpaces()
	if len(spaces) == 0 {
		return
	}
	// If label is stale, reset
	if m.WsLabel != "" {
		found := false
		for _, s := range spaces {
			if s.Label == m.WsLabel {
				found = true
				break
			}
		}
		if !found {
			m.WsLabel = ""
		}
	}
	if m.WsLabel == "" {
		m.WsLabel = spaces[0].Label
	} else {
		idx := findSpaceIndex(spaces, m.WsLabel)
		m.WsLabel = spaces[(idx+1)%len(spaces)].Label
	}
	m.Mode = ModeSpace
	m.ConnName = "" // clear machine filter
}

// RenderAll produces 14 key commands (K1-K10 agents + K11-K14 nav/stats).
func (m *Mapper) RenderAll() []types.KeyCommand {
	agents := m.state.GetFilteredAgents(m.ConnName, m.WsLabel)
	machines := m.state.GetMachines()
	stats := m.state.ComputeStats()

	keys := make([]types.KeyCommand, 0, 14)

	// K1-K10: agents
	for i := 0; i < 10; i++ {
		if i < len(agents) {
			a := agents[i]
			keys = append(keys, types.KeyCommand{
				Agent: &types.AgentKeyData{
					KeyID:          "agent_" + itoa(i),
					Type:           "agent",
					AgentType:      a.Agent,
					Alias:          coalesce(a.Name, a.TabLabel, a.Agent, ""),
					Status:         string(a.AgentStatus),
					Focused:        a.Focused,
					PaneID:         a.PaneID,
					ConnName:       a.ConnName,
					ConnAbbr:       a.ConnAbbr,
					ConnAbbrColor:  a.ConnAbbrColor,
					WsLabel:        a.WsLabel,
					StatusDuration: m.state.FormatAgentDuration(a.ConnName, a.PaneID),
				},
			})
		} else {
			keys = append(keys, types.KeyCommand{
				Empty: &types.EmptyKeyData{
					KeyID: "empty_" + itoa(i),
					Type:  "empty",
				},
			})
		}
	}

	// Determine machine and space state for nav buttons
	machineIdx := -1
	if m.ConnName != "" {
		machineIdx = findMachineIndex(machines, m.ConnName)
	}
	nextMachine := machineNext(machines, machineIdx)

	spaces := m.state.GetAllSpaces()
	spaceIdx := -1
	if m.WsLabel != "" {
		spaceIdx = findSpaceIndex(spaces, m.WsLabel)
	}
	nextSpace := spaceNext(spaces, spaceIdx)

	curMachine := machineCurrent(machines, machineIdx)

	// K11 button label and filter state
	k11Filtered := m.K11Filtered
	k11Label := "ALL"
	if k11Filtered {
		k11Label = "ACT"
	}
	isMachineMode := m.Mode == ModeMachine
	cpuPct, memPct := m.state.GetSysStats()
	keys = append(keys, types.KeyCommand{
		NavAll: &types.NavAllData{
			KeyID:         "nav_all",
			Type:          "navAll",
			Label:         k11Label,
			Active:        m.Mode == ModeAll,
			Filtered:      k11Filtered,
			CPUPercent:    cpuPct,
			MemoryPercent: memPct,
		},
	})

	// K12: machine cycle (handle nil curMachine/nextMachine)
	currentAbbr := "-"
	currentColor := "#888"
	nextAbbr := "-"
	if curMachine != nil {
		currentAbbr = curMachine.ConnAbbr
		currentColor = curMachine.ConnAbbrColor
	}
	if nextMachine != nil {
		nextAbbr = nextMachine.ConnAbbr
	}
	keys = append(keys, types.KeyCommand{
		NavMac: &types.NavMachineData{
			KeyID:        "nav_machine",
			Type:         "navMachine",
			CurrentAbbr:  currentAbbr,
			CurrentColor: currentColor,
			NextAbbr:     nextAbbr,
			Active:       isMachineMode,
		},
	})

	// K13: space cycle
	keys = append(keys, types.KeyCommand{
		NavSpc: &types.NavSpaceData{
			KeyID:     "nav_space",
			Type:      "navSpace",
			Count:     len(spaces),
			NextLabel: spaceNextLabel(nextSpace),
			Active:    m.Mode == ModeSpace,
		},
	})

	// K14: stats with system CPU/memory
	keys = append(keys, types.KeyCommand{
		Stats: &types.StatsData{
			KeyID:         "stats",
			Type:          "stats",
			Stats:         stats,
			CPUPercent:    cpuPct,
			MemoryPercent: memPct,
		},
	})

	return keys
}

// ─── Helpers ────────────────────────────────────────────────

func findMachineIndex(machines []types.MachineRef, name string) int {
	for i, m := range machines {
		if m.ConnName == name {
			return i
		}
	}
	return -1
}

func findSpaceIndex(spaces []types.SpaceRef, label string) int {
	for i, s := range spaces {
		if s.Label == label {
			return i
		}
	}
	return -1
}

func machineCurrent(machines []types.MachineRef, idx int) *types.MachineRef {
	if idx >= 0 && idx < len(machines) {
		return &machines[idx]
	}
	return nil
}

func machineNext(machines []types.MachineRef, idx int) *types.MachineRef {
	if len(machines) == 0 {
		return nil
	}
	if idx < 0 {
		// ALL → first or second machine
		if len(machines) > 1 {
			return &machines[1]
		}
		return nil
	}
	return &machines[(idx+1)%len(machines)]
}

func spaceNext(spaces []types.SpaceRef, idx int) *types.SpaceRef {
	if len(spaces) == 0 {
		return nil
	}
	if idx < 0 {
		return &spaces[0]
	}
	return &spaces[(idx+1)%len(spaces)]
}

func spaceNextLabel(s *types.SpaceRef) string {
	if s == nil {
		return "-"
	}
	return s.Label
}

func coalesce(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
