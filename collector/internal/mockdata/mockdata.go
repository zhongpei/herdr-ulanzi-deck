// Package mockdata provides a source of FetchResult slices that cycle through
// pre-defined phases on a timer. Used for testing herdr-collector without a
// real herdr daemon. Pass --mock-data <path> to the collector to activate.
//
// The JSON file is an array of phase objects. Each phase contains one or more
// connection entries (machines), each with workspaces and agents. The source
// advances to the next phase every PerPhaseDuration (default 5s) and wraps
// around after the last phase.
package mockdata

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/herdr-deck/herdrdeck/collector/internal/bridge"
)

// ─── DTO types (JSON-tagged) ─────────────────────────────────────────────

// PhaseDTO is one "scene" in the mock data timeline.
type PhaseDTO struct {
	Name        string         `json:"name"`
	Connections []ConnectionDTO `json:"connections"`
}

// ConnectionDTO represents one machine/herdr connection.
type ConnectionDTO struct {
	Name       string         `json:"name"`
	Abbr       string         `json:"abbr"`
	Color      string         `json:"color"`
	Workspaces []WorkspaceDTO `json:"workspaces"`
}

// WorkspaceDTO is a single workspace with its agents.
type WorkspaceDTO struct {
	ID     string      `json:"id"`
	Label  string      `json:"label"`
	Number int         `json:"number"`
	Agents []AgentDTO  `json:"agents"`
}

// AgentDTO represents one agent/pane.
type AgentDTO struct {
	PaneID   string `json:"pane_id"`
	TabID    string `json:"tab_id,omitempty"`
	Agent    string `json:"agent"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Focused  bool   `json:"focused"`
	TabLabel string `json:"tab_label,omitempty"`
}

// ─── Source ──────────────────────────────────────────────────────────────

// DefaultDuration is the time each phase is active before advancing.
const DefaultDuration = 5 * time.Second

// Source cycles through pre-defined phases on a timer, producing
// bridge.FetchResult slices that look like real herdr data.
type Source struct {
	phases       []PhaseDTO
	perPhase     time.Duration
	startTime    time.Time
}

// Load reads a JSON mock data file and returns a new Source.
// The file must be a JSON array of PhaseDTO.
func Load(path string, perPhase time.Duration) (*Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("mockdata: read %s: %w", path, err)
	}
	var phases []PhaseDTO
	if err := json.Unmarshal(data, &phases); err != nil {
		return nil, fmt.Errorf("mockdata: parse %s: %w", path, err)
	}
	if len(phases) == 0 {
		return nil, fmt.Errorf("mockdata: %s has zero phases", path)
	}
	if perPhase <= 0 {
		perPhase = DefaultDuration
	}
	return &Source{
		phases:    phases,
		perPhase:  perPhase,
		startTime: time.Now(),
	}, nil
}

// currentPhase returns the active phase index based on elapsed time.
func (s *Source) currentPhase() int {
	elapsed := time.Since(s.startTime)
	idx := int(elapsed / s.perPhase)
	return idx % len(s.phases)
}

// FetchAll returns the bridge.FetchResult slice for the currently active phase.
// It implements the same signature as bridge.Bridge.FetchAll.
func (s *Source) FetchAll() []bridge.FetchResult {
	phase := s.phases[s.currentPhase()]
	results := make([]bridge.FetchResult, 0, len(phase.Connections))
	for _, c := range phase.Connections {
		workspaces := make([]bridge.RawWorkspace, 0, len(c.Workspaces))
		for _, ws := range c.Workspaces {
			agents := make([]bridge.RawAgent, 0, len(ws.Agents))
			for _, a := range ws.Agents {
				agents = append(agents, bridge.RawAgent{
					PaneID:      a.PaneID,
					WorkspaceID: ws.ID,
					TabID:       a.TabID,
					Agent:       a.Agent,
					Name:        a.Name,
					Status:      a.Status,
					Focused:     a.Focused,
					TabLabel:    a.TabLabel,
				})
			}
			// Compute agent status / tab/ pane counts for the workspace
			tabCount := len(agents)
			paneCount := len(agents)
			agentStatus := aggregateStatus(agents)
			if ws.Label == "" {
			}
			workspaces = append(workspaces, bridge.RawWorkspace{
				ConnName:      c.Name,
				ConnAbbr:      c.Abbr,
				ConnAbbrColor: c.Color,
				WorkspaceID:   ws.ID,
				Label:         ws.Label,
				Number:        ws.Number,
				AgentStatus:   agentStatus,
				TabCount:      tabCount,
				PaneCount:     paneCount,
				Agents:        agents,
			})
		}
		results = append(results, bridge.FetchResult{
			ConnName:   c.Name,
			ConnAbbr:   c.Abbr,
			ConnColor:  c.Color,
			Workspaces: workspaces,
		})
	}
	return results
}

// ─── helpers ─────────────────────────────────────────────────────────────

// aggregateStatus picks the "most interesting" status among agents
// for use as RawWorkspace.AgentStatus. Priority: blocked > working > done > idle > unknown.
func aggregateStatus(agents []bridge.RawAgent) string {
	priority := map[string]int{
		"blocked": 0,
		"working": 1,
		"done":    2,
		"idle":    3,
		"unknown": 4,
	}
	best := "unknown"
	bestP := 99
	for _, a := range agents {
		if p, ok := priority[a.Status]; ok && p < bestP {
			best = a.Status
			bestP = p
		}
	}
	return best
}

// PhaseCount returns the number of loaded phases.
func (s *Source) PhaseCount() int {
	return len(s.phases)
}
