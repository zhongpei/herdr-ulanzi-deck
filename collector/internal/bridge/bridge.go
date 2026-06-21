// Package bridge merges herdr data from multiple connections into a
// unified raw workspace list. The fleet store converts this to FleetSnapshot.
package bridge

import (
	"encoding/json"
	"fmt"

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
// Internal type: fleet.Store converts this to protocol.FleetSnapshot.
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
	Status      string // raw status string from herdr
	Focused     bool
	TabLabel    string
}

// Bridge manages a pool of herdr connections and merges their data.
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

// FetchAll queries all connections and returns merged raw workspace data.
func (b *Bridge) FetchAll() []RawWorkspace {
	var all []RawWorkspace
	for _, conn := range b.connections {
		workspaces, agents, err := fetchConn(conn)
		if err != nil {
			log.Error().Err(err).Str("conn", conn.Name).Msg("fetch failed for connection")
			continue
		}
		agentMap := make(map[string][]RawAgent)
		for _, a := range agents {
			wid := a.WorkspaceID
			agentMap[wid] = append(agentMap[wid], a)
		}
		for _, ws := range workspaces {
			label := ws.Label
			if label == "" {
				label = fmt.Sprintf("ws-%d", ws.Number)
			}
			all = append(all, RawWorkspace{
				ConnName:      conn.Name,
				ConnAbbr:      conn.Abbr,
				ConnAbbrColor: conn.Color,
				WorkspaceID:   ws.WorkspaceID,
				Label:         label,
				Number:        ws.Number,
				AgentStatus:   ws.AgentStatus,
				TabCount:      ws.TabCount,
				PaneCount:     ws.PaneCount,
				Agents:        agentMap[ws.WorkspaceID],
			})
		}
	}
	return all
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

// ─── internal fetch ────────────────────────────────────────

// wsInfo mirrors the JSON structure from herdr workspace.list.
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

// agInfo mirrors the JSON structure from herdr agent.list.
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

func fetchConn(conn ConnRef) ([]wsInfo, []RawAgent, error) {
	wsRaw, err := conn.Client.ListWorkspaces()
	if err != nil {
		return nil, nil, fmt.Errorf("workspace.list: %w", err)
	}
	agRaw, err := conn.Client.ListAgents()
	if err != nil {
		return nil, nil, fmt.Errorf("agent.list: %w", err)
	}
	tabRaw, err := conn.Client.ListTabs()
	if err != nil {
		return nil, nil, fmt.Errorf("tab.list: %w", err)
	}

	var wsObj struct {
		Workspaces []wsInfo `json:"workspaces"`
	}
	if err := json.Unmarshal(wsRaw, &wsObj); err != nil {
		return nil, nil, fmt.Errorf("parse workspaces: %w", err)
	}

	var agObj struct {
		Agents []agInfo `json:"agents"`
	}
	if err := json.Unmarshal(agRaw, &agObj); err != nil {
		return nil, nil, fmt.Errorf("parse agents: %w", err)
	}

	var tabObj struct {
		Tabs []struct {
			TabID string `json:"tab_id"`
			Label string `json:"label"`
		} `json:"tabs"`
	}
	if err := json.Unmarshal(tabRaw, &tabObj); err != nil {
		return nil, nil, fmt.Errorf("parse tabs: %w", err)
	}
	tabLabels := make(map[string]string, len(tabObj.Tabs))
	for _, t := range tabObj.Tabs {
		tabLabels[t.TabID] = t.Label
	}

	// Convert to RawAgent and enrich with tab labels
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
