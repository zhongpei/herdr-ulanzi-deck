// Package viewmodel converts fleet state into 14 key render commands.
// Replaces the old mapper package.
package viewmodel

import (
	"github.com/herdr-deck/herdrdeck/deck/internal/fleet"
	"github.com/herdr-deck/herdrdeck/protocol"
)

// FilterMode enumerates the three display modes.
type FilterMode int

const (
	ModeAll FilterMode = iota
	ModeMachine
	ModeSpace
)

// Builder holds filter state and a reference to the fleet manager.
type Builder struct {
	fleet       *fleet.Manager
	Mode        FilterMode
	ConnName    string
	WsLabel     string
	K11Toggle   bool
	K11Filtered bool
}

// NewBuilder creates a Builder in ALL mode.
func NewBuilder(fm *fleet.Manager) *Builder {
	return &Builder{
		fleet: fm,
		Mode:  ModeAll,
	}
}

func (b *Builder) SetAll() {
	b.Mode = ModeAll
	b.ConnName = ""
	b.WsLabel = ""
}

func (b *Builder) NextMachine() {
	machines := b.fleet.GetMachines()
	if len(machines) == 0 {
		return
	}
	if b.ConnName == "" {
		b.ConnName = machines[0].Name
	} else {
		idx := findMachineIndex(machines, b.ConnName)
		b.ConnName = machines[(idx+1)%len(machines)].Name
	}
	b.Mode = ModeMachine
	b.WsLabel = ""
}

func (b *Builder) NextSpace() {
	spaces := b.fleet.GetAllSpaces()
	if len(spaces) == 0 {
		return
	}
	if b.WsLabel != "" {
		found := false
		for _, s := range spaces {
			if s.Label == b.WsLabel {
				found = true
				break
			}
		}
		if !found {
			b.WsLabel = ""
		}
	}
	if b.WsLabel == "" {
		b.WsLabel = spaces[0].Label
	} else {
		idx := findSpaceIndex(spaces, b.WsLabel)
		b.WsLabel = spaces[(idx+1)%len(spaces)].Label
	}
	b.Mode = ModeSpace
	b.ConnName = ""
}

// Build produces 14 key commands (K1-K10 agents + K11-K14 nav/stats).
func (b *Builder) Build() []KeyCommand {
	agents := b.fleet.GetFilteredAgents(b.ConnName, b.WsLabel)
	machines := b.fleet.GetMachines()
	stats := b.fleet.ComputeStats()

	keys := make([]KeyCommand, 0, 14)

	for i := 0; i < 10; i++ {
		if i < len(agents) {
			a := agents[i]
			keys = append(keys, KeyCommand{
				Agent: &AgentKeyData{
					KeyID:          "agent_" + itoa(i),
					Type:           "agent",
					AgentType:      a.Agent,
					Alias:          coalesce(a.Name, a.TabLabel, a.Agent, ""),
					Status:         string(a.Status),
					Focused:        a.Focused,
					PaneID:         a.PaneID,
					ConnName:       a.ConnName,
					ConnAbbr:       a.ConnAbbr,
					ConnAbbrColor:  a.ConnAbbrColor,
					WsLabel:        a.WsLabel,
					StatusDuration: b.fleet.FormatAgentDuration(a.ConnName, a.PaneID),
				},
			})
		} else {
			keys = append(keys, KeyCommand{
				Empty: &EmptyKeyData{
					KeyID: "empty_" + itoa(i),
					Type:  "empty",
				},
			})
		}
	}

	machineIdx := -1
	if b.ConnName != "" {
		machineIdx = findMachineIndex(machines, b.ConnName)
	}
	nextMachine := machineNext(machines, machineIdx)
	curMachine := machineCurrent(machines, machineIdx)

	spaces := b.fleet.GetAllSpaces()
	spaceIdx := -1
	if b.WsLabel != "" {
		spaceIdx = findSpaceIndex(spaces, b.WsLabel)
	}
	nextSpace := spaceNext(spaces, spaceIdx)

	// K11
	k11Label := "ALL"
	if b.K11Filtered {
		k11Label = "ACT"
	}
	cpuPct, memPct := b.fleet.GetSysStats()
	keys = append(keys, KeyCommand{
		NavAll: &NavAllData{
			KeyID:         "nav_all",
			Type:          "navAll",
			Label:         k11Label,
			Active:        b.Mode == ModeAll,
			Filtered:      b.K11Filtered,
			CPUPercent:    cpuPct,
			MemoryPercent: memPct,
		},
	})

	// K12
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
	keys = append(keys, KeyCommand{
		NavMac: &NavMachineData{
			KeyID:        "nav_machine",
			Type:         "navMachine",
			CurrentAbbr:  currentAbbr,
			CurrentColor: currentColor,
			NextAbbr:     nextAbbr,
			Active:       b.Mode == ModeMachine,
		},
	})

	// K13
	currentSpace := spaceCurrent(spaces, spaceIdx)
	keys = append(keys, KeyCommand{
		NavSpc: &NavSpaceData{
			KeyID:        "nav_space",
			Type:         "navSpace",
			Count:        len(spaces),
			CurrentLabel: spaceCurrentLabel(currentSpace),
			NextLabel:    spaceNextLabel(nextSpace),
			Active:       b.Mode == ModeSpace,
		},
	})

	// K14
	keys = append(keys, KeyCommand{
		Stats: &StatsData{
			KeyID:         "stats",
			Type:          "stats",
			Stats:         stats,
			CPUPercent:    cpuPct,
			MemoryPercent: memPct,
		},
	})

	return keys
}

// ─── helpers ────────────────────────────────────────────────

func findMachineIndex(machines []protocol.MachineInfo, name string) int {
	for i, m := range machines {
		if m.Name == name {
			return i
		}
	}
	return -1
}

func findSpaceIndex(spaces []fleet.SpaceRef, label string) int {
	for i, s := range spaces {
		if s.Label == label {
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
	if idx < 0 {
		if len(machines) > 1 {
			return &machines[1]
		}
		return nil
	}
	return &machines[(idx+1)%len(machines)]
}

func spaceNext(spaces []fleet.SpaceRef, idx int) *fleet.SpaceRef {
	if len(spaces) == 0 {
		return nil
	}
	if idx < 0 {
		return &spaces[0]
	}
	return &spaces[(idx+1)%len(spaces)]
}

func spaceCurrent(spaces []fleet.SpaceRef, idx int) *fleet.SpaceRef {
	if idx >= 0 && idx < len(spaces) {
		return &spaces[idx]
	}
	return nil
}

func spaceCurrentLabel(s *fleet.SpaceRef) string {
	if s == nil {
		return "-"
	}
	return s.Label
}

func spaceNextLabel(s *fleet.SpaceRef) string {
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
