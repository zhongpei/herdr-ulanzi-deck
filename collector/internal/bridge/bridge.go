// Package bridge merges herdr data from multiple connections.
// FetchAll fetches from all connections concurrently with per-connection
// timeout, so a single bad connection doesn't block the global refresh.
package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/herdr-deck/herdrdeck/collector/internal/herdrclient"
	"github.com/rs/zerolog/log"
)

// ConnRef bundles connection metadata with a herdr client.
type ConnRef struct {
	Name   string
	Abbr   string
	Color  string
	Client *herdrclient.Client
}

// RawWorkspace is a workspace with agents enriched with connection metadata.
type RawWorkspace struct {
	ConnName      string
	ConnAbbr      string
	ConnAbbrColor string
	WorkspaceID   string
	Label         string
	Number        int
	AgentStatus   string
	TabCount      int
	PaneCount     int
	Agents        []RawAgent
}

// RawAgent is an agent record with connection metadata.
type RawAgent struct {
	PaneID      string
	WorkspaceID string
	TabID       string
	Agent       string
	Name        string
	Status      string
	Focused     bool
	TabLabel    string
}

// FetchResult holds the result of fetching a single connection.
type FetchResult struct {
	ConnName   string
	ConnAbbr   string
	ConnColor  string
	Workspaces []RawWorkspace
	Err        error
}

// Bridge manages a pool of herdr connections.
type Bridge struct {
	connections []ConnRef
}

// NewBridge creates an empty bridge.
func NewBridge() *Bridge {
	return &Bridge{}
}

// AddConnection registers a herdr connection with metadata.
func (b *Bridge) AddConnection(name, abbr, color string, client *herdrclient.Client) {
	b.connections = append(b.connections, ConnRef{
		Name: name, Abbr: abbr, Color: color, Client: client,
	})
}

// Connections returns a copy of the connection list. Used by the collector
// to track machine metadata even when a connection fails.
func (b *Bridge) Connections() []ConnRef {
	out := make([]ConnRef, len(b.connections))
	copy(out, b.connections)
	return out
}

// FetchAll fetches ALL connections concurrently with a 5s timeout per
// connection. Returns one FetchResult per connection — failed connections
// carry the error instead of being silently dropped.
func (b *Bridge) FetchAll() []FetchResult {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := make(chan FetchResult, len(b.connections))
	for _, conn := range b.connections {
		go func(c ConnRef) {
			result := FetchResult{
				ConnName:  c.Name,
				ConnAbbr:  c.Abbr,
				ConnColor: c.Color,
			}
			ws, agents, err := fetchConn(c, ctx)
			if err != nil {
				result.Err = err
			} else {
				result.Workspaces = merge(c, ws, agents)
			}
			ch <- result
		}(conn)
	}

	results := make([]FetchResult, 0, len(b.connections))
	for i := 0; i < len(b.connections); i++ {
		select {
		case r := <-ch:
			results = append(results, r)
		case <-ctx.Done():
			// Timeout — add failure results for remaining connections
			for _, conn := range b.connections {
				if !containsResult(results, conn.Name) {
					results = append(results, FetchResult{
						ConnName: conn.Name,
						ConnAbbr: conn.Abbr,
						ConnColor: conn.Color,
						Err:      ctx.Err(),
					})
				}
			}
			return results
		}
	}
	return results
}

// FocusAgent sends an agent.focus command to the matching connection.
func (b *Bridge) FocusAgent(connName, paneID string) {
	for _, conn := range b.connections {
		if conn.Name == connName {
			_, err := conn.Client.Request("agent.focus", map[string]any{
				"target": paneID,
			})
			if err != nil {
				log.Error().Err(err).Str("conn", connName).Str("pane", paneID).Msg("focus failed")
			}
			return
		}
	}
}

// ─── helpers ────────────────────────────────────────────────

func containsResult(results []FetchResult, name string) bool {
	for _, r := range results {
		if r.ConnName == name {
			return true
		}
	}
	return false
}

func merge(c ConnRef, ws []wsInfo, agents []RawAgent) []RawWorkspace {
	agentMap := make(map[string][]RawAgent)
	for _, a := range agents {
		agentMap[a.WorkspaceID] = append(agentMap[a.WorkspaceID], a)
	}
	var workspaces []RawWorkspace
	for _, w := range ws {
		label := w.Label
		if label == "" {
			label = fmt.Sprintf("ws-%d", w.Number)
		}
		workspaces = append(workspaces, RawWorkspace{
			ConnName:      c.Name,
			ConnAbbr:      c.Abbr,
			ConnAbbrColor: c.Color,
			WorkspaceID:   w.WorkspaceID,
			Label:         label,
			Number:        w.Number,
			AgentStatus:   w.AgentStatus,
			TabCount:      w.TabCount,
			PaneCount:     w.PaneCount,
			Agents:        agentMap[w.WorkspaceID],
		})
	}
	return workspaces
}

// ─── internal fetch ────────────────────────────────────────

type wsInfo struct {
	WorkspaceID string `json:"workspace_id"`
	Label       string `json:"label"`
	Number      int    `json:"number"`
	Focused     bool   `json:"focused"`
	AgentStatus string `json:"agent_status"`
	TabCount    int    `json:"tab_count"`
	ActiveTabID string `json:"active_tab_id"`
	PaneCount   int    `json:"pane_count"`
}

type agInfo struct {
	PaneID      string            `json:"pane_id"`
	TerminalID  string            `json:"terminal_id"`
	WorkspaceID string            `json:"workspace_id"`
	TabID       string            `json:"tab_id"`
	Agent       string            `json:"agent"`
	Name        string            `json:"name"`
	AgentStatus string            `json:"agent_status"`
	StateLabels map[string]string `json:"state_labels"`
	Focused     bool              `json:"focused"`
	Revision    int               `json:"revision"`
}

func fetchConn(conn ConnRef, ctx context.Context) ([]wsInfo, []RawAgent, error) {
	type listCall struct {
		name string
		fn   func() ([]byte, error)
	}

	calls := []struct {
		name string
		fn   func() (json.RawMessage, error)
	}{
		{"workspace.list", conn.Client.ListWorkspaces},
		{"agent.list", conn.Client.ListAgents},
		{"tab.list", conn.Client.ListTabs},
	}

	var wsObj struct {
		Workspaces []wsInfo `json:"workspaces"`
	}
	var agObj struct {
		Agents []agInfo `json:"agents"`
	}
	var tabObj struct {
		Tabs []struct {
			TabID string `json:"tab_id"`
			Label string `json:"label"`
		} `json:"tabs"`
	}

	for _, call := range calls {
		select {
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("%s: %w", call.name, ctx.Err())
		default:
		}

		data, err := call.fn()
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", call.name, err)
		}

		switch call.name {
		case "workspace.list":
			if err := json.Unmarshal(data, &wsObj); err != nil {
				return nil, nil, fmt.Errorf("parse workspaces: %w", err)
			}
		case "agent.list":
			if err := json.Unmarshal(data, &agObj); err != nil {
				return nil, nil, fmt.Errorf("parse agents: %w", err)
			}
		case "tab.list":
			if err := json.Unmarshal(data, &tabObj); err != nil {
				return nil, nil, fmt.Errorf("parse tabs: %w", err)
			}
		}
	}

	tabLabels := make(map[string]string, len(tabObj.Tabs))
	for _, t := range tabObj.Tabs {
		tabLabels[t.TabID] = t.Label
	}

	rawAgents := make([]RawAgent, len(agObj.Agents))
	for i, a := range agObj.Agents {
		rawAgents[i] = RawAgent{
			PaneID:      a.PaneID,
			WorkspaceID: a.WorkspaceID,
			TabID:       a.TabID,
			Agent:       a.Agent,
			Name:        a.Name,
			Status:      a.AgentStatus,
			Focused:     a.Focused,
			TabLabel:    tabLabels[a.TabID],
		}
	}

	return wsObj.Workspaces, rawAgents, nil
}
