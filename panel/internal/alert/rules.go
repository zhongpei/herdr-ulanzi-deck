// Package alert defines alert rules that trigger the panel window to pop up
// when agents enter certain statuses.
package alert

import (
	"fmt"
	"strings"

	"github.com/herdr-deck/herdrdeck/protocol"
)

// Rule defines which agent statuses trigger a panel popup.
type Rule struct {
	WatchStatuses []string // normalized status strings, e.g. ["blocked", "done"]
}

// ParseRule parses a comma-separated status string into a Rule.
// Example: "blocked,done,working" → Rule{WatchStatuses: ["blocked","done","working"]}
func ParseRule(s string) (*Rule, error) {
	parts := strings.Split(s, ",")
	seen := make(map[string]bool)
	var statuses []string

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		// Validate against known statuses
		switch protocol.AgentStatus(p) {
		case protocol.StatusBlocked,
			protocol.StatusDone,
			protocol.StatusWorking,
			protocol.StatusIdle,
			protocol.StatusUnknown:
			// valid
		default:
			return nil, fmt.Errorf("unknown status %q in --alert-on", p)
		}
		seen[p] = true
		statuses = append(statuses, p)
	}

	if len(statuses) == 0 {
		return nil, fmt.Errorf("--alert-on must specify at least one status")
	}

	return &Rule{WatchStatuses: statuses}, nil
}

// ShouldAlert checks if a status is in the watch list.
func (r *Rule) ShouldAlert(status protocol.AgentStatus) bool {
	for _, s := range r.WatchStatuses {
		if s == string(status) {
			return true
		}
	}
	return false
}
