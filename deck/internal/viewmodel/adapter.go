// Package viewmodel converts displaymodel.Model into 14 hardware key commands
// for the Ulanzi D200X renderer. It is a thin adapter — all filtering,
// sorting, and navigation semantics live in displaymodel.
package viewmodel

import (
	"github.com/herdr-deck/herdrdeck/displaymodel"
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
	keys = append(keys, KeyCommand{
		Stats: &StatsData{
			KeyID:         "stats",
			Type:          "stats",
			Stats:         m.Stats.AgentStats,
			CPUPercent:    m.Stats.CPUPercent,
			MemoryPercent: m.Stats.MemoryPercent,
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
