// Package viewmodel converts displaymodel.Model into 14 hardware key commands
// for the Ulanzi D200X renderer. It is a thin adapter — all filtering,
// sorting, and navigation semantics live in displaymodel.
package viewmodel

import (
	"github.com/herdr-deck/herdrdeck/displaymodel"
	"github.com/herdr-deck/herdrdeck/protocol"
)

// Adapt converts a displaymodel.Model into 14 KeyCommand values (K1-K14).
// K1-K10 are agent keys (or empty slots), K11=NavAll, K12=NavMachine,
// K13=NavSpace, K14=Stats.
func Adapt(m displaymodel.Model) []KeyCommand {
	keys := make([]KeyCommand, 0, 14)

	// K1-K10: agents
	for i, a := range m.Agents {
		keys = append(keys, KeyCommand{
			Agent: &AgentKeyData{
				KeyID:          "agent_" + itoa(i),
				Type:           "agent",
				AgentType:      a.Agent,
				Alias:          a.Name,
				Status:         string(a.Status),
				Focused:        a.Focused,
				PaneID:         a.PaneID,
				ConnName:       a.ConnName,
				ConnAbbr:       a.ConnAbbr,
				ConnAbbrColor:  a.ConnAbbrColor,
				WsLabel:        a.WsLabel,
				StatusDuration: a.StatusDuration,
			},
		})
	}
	// Fill remaining K1-K10 slots with empty
	for i := len(m.Agents); i < 10; i++ {
		keys = append(keys, KeyCommand{
			Empty: &EmptyKeyData{
				KeyID: "empty_" + itoa(i),
				Type:  "empty",
			},
		})
	}

	// Compute per-machine agent counts for K11 display
	counts := make(map[string]int)
	for _, a := range m.Agents {
		counts[a.ConnName]++
	}

	// K11: NavAll
	keys = append(keys, KeyCommand{
		NavAll: &NavAllData{
			KeyID:         "nav_all",
			Type:          "navAll",
			Label:         m.NavAll.Label,
			Active:        m.NavAll.Active,
			Filtered:      m.NavAll.Filtered,
			CPUPercent:    m.Stats.CPUPercent,
			MemoryPercent: m.Stats.MemoryPercent,
			Machines:      m.Machines,
			AgentCounts:   counts,
		},
	})

	// K12: NavMachine
	keys = append(keys, KeyCommand{
		NavMac: &NavMachineData{
			KeyID:        "nav_machine",
			Type:         "navMachine",
			CurrentAbbr:  m.NavMachine.CurrentAbbr,
			CurrentColor: m.NavMachine.CurrentColor,
			NextAbbr:     m.NavMachine.NextAbbr,
			Active:       m.NavMachine.Active,
		},
	})

	// K13: NavSpace
	keys = append(keys, KeyCommand{
		NavSpc: &NavSpaceData{
			KeyID:        "nav_space",
			Type:         "navSpace",
			Count:        m.NavSpace.Count,
			CurrentLabel: m.NavSpace.CurrentLabel,
			NextLabel:    m.NavSpace.NextLabel,
			Active:       m.NavSpace.Active,
		},
	})
	// K14: Stats
	// Compute per-space, per-machine, per-status breakdown
	type statusKey struct{ space, machine, status string }
	statCount := make(map[statusKey]int)
	spaceTotal := make(map[string]int)
	spaceMachineSet := make(map[string]map[string]bool)

	for _, a := range m.Agents {
		key := statusKey{a.WsLabel, a.ConnName, string(a.Status)}
		statCount[key]++
		spaceTotal[a.WsLabel]++
		if spaceMachineSet[a.WsLabel] == nil {
			spaceMachineSet[a.WsLabel] = make(map[string]bool)
		}
		spaceMachineSet[a.WsLabel][a.ConnName] = true
	}

	// Build machine color map
	machineColor := make(map[string]string)
	machineAbbr := make(map[string]string)
	for _, mac := range m.Machines {
		machineColor[mac.Name] = mac.Color
		machineAbbr[mac.Name] = mac.Abbr
	}

	// Collect unique spaces in display order (preserve agent order)
	var spaceOrder []string
	seen := make(map[string]bool)
	for _, a := range m.Agents {
		if !seen[a.WsLabel] {
			seen[a.WsLabel] = true
			spaceOrder = append(spaceOrder, a.WsLabel)
		}
	}

	var spaces []SpaceStats
	for _, label := range spaceOrder {
		// Collect machines for this space in order
		var machines []MachineStats
		for _, mac := range m.Machines {
			if !spaceMachineSet[label][mac.Name] {
				continue
			}
			machineTotal := 0
			statusMap := make(map[string]int)
			for _, st := range []protocol.AgentStatus{
				protocol.StatusDone, protocol.StatusIdle,
				protocol.StatusWorking, protocol.StatusBlocked,
				protocol.StatusUnknown,
			} {
				cnt := statCount[statusKey{label, mac.Name, string(st)}]
				if cnt > 0 {
					statusMap[string(st)] = cnt
					machineTotal += cnt
				}
			}
			machines = append(machines, MachineStats{
				Abbr:  mac.Abbr,
				Color: mac.Color,
				Total: machineTotal,
				Stats: statusMap,
			})
		}
		spaces = append(spaces, SpaceStats{
			Label:    label,
			Machines: machines,
			Total:    spaceTotal[label],
		})
	}

	// Filter: only keep spaces with active agents (blocked/done/working)
	var activeSpaces []SpaceStats
	for _, sp := range spaces {
		hasActive := false
		for _, mac := range sp.Machines {
			for st := range mac.Stats {
				if st == "blocked" || st == "done" || st == "working" {
					hasActive = true
					break
				}
			}
			if hasActive {
				break
			}
		}
		if hasActive {
			activeSpaces = append(activeSpaces, sp)
		}
	}
	spaces = activeSpaces

	keys = append(keys, KeyCommand{
		Stats: &StatsData{
			KeyID:         "stats",
			Type:          "stats",
			Stats:         m.Stats.AgentStats,
			CPUPercent:    m.Stats.CPUPercent,
			MemoryPercent: m.Stats.MemoryPercent,
			Spaces:        spaces,
		},
	})

	return keys
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
