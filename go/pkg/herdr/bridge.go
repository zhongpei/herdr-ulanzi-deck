// Package herdr provides the bridge that reads herdr data → UnifiedWorkspace format.
// Mirrors src/herdr-bridge.js
package herdr

import (
	"encoding/json"
	"fmt"

	"github.com/herdr-deck/herdrdeck/pkg/types"
	"github.com/rs/zerolog/log"
)

type ConnRef struct {
	Name   string
	Abbr   string
	Color  string
	Client *Client
}

type Bridge struct {
	connections []ConnRef
}

func NewBridge() *Bridge {
	return &Bridge{}
}

func (b *Bridge) AddConnection(name, abbr, color string, client *Client) {
	b.connections = append(b.connections, ConnRef{
		Name: name, Abbr: abbr, Color: color, Client: client,
	})
}

// FetchAll queries ALL connections and merges into UnifiedWorkspace list.
func (b *Bridge) FetchAll() []types.UnifiedWorkspace {
	var all []types.UnifiedWorkspace
	for _, conn := range b.connections {
		workspaces, agents, err := b.fetchConn(conn)
		if err != nil {
			log.Error().Err(err).Str("conn", conn.Name).Msg("fetch failed for connection")
			continue
		}
		agentMap := make(map[string][]types.AgentInfo)
		for _, a := range agents {
			wid := a.WorkspaceID
			agentMap[wid] = append(agentMap[wid], a)
		}
		for _, ws := range workspaces {
			label := ws.Label
			if label == "" {
				label = fmt.Sprintf("ws-%d", ws.Number)
			}
			all = append(all, types.UnifiedWorkspace{
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

// fetchConn queries a single connection.
// herdr returns {"result":{"workspaces":[...],"agents":[...]}}
// The client returns the "result" portion, so we need to unwrap the nested keys.
func (b *Bridge) fetchConn(conn ConnRef) ([]types.WorkspaceInfo, []types.AgentInfo, error) {
	wsRaw, err := conn.Client.ListWorkspaces()
	if err != nil {
		return nil, nil, fmt.Errorf("workspace.list: %w", err)
	}
	agRaw, err := conn.Client.ListAgents()
	if err != nil {
		return nil, nil, fmt.Errorf("agent.list: %w", err)
	}

	// Result is {"workspaces": [...]}, unwrap the array
	var wsObj struct {
		Workspaces []types.WorkspaceInfo `json:"workspaces"`
	}
	if err := json.Unmarshal(wsRaw, &wsObj); err != nil {
		return nil, nil, fmt.Errorf("parse workspaces: %w", err)
	}

	// Result is {"agents": [...]}, unwrap the array
	var agObj struct {
		Agents []types.AgentInfo `json:"agents"`
	}
	if err := json.Unmarshal(agRaw, &agObj); err != nil {
		return nil, nil, fmt.Errorf("parse agents: %w", err)
	}

	return wsObj.Workspaces, agObj.Agents, nil
}
