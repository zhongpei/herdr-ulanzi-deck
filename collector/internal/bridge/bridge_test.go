package bridge

import (
	"net"
	"testing"

	"github.com/herdr-deck/herdrdeck/collector/internal/herdrclient"
)

// startMockHerdr starts a TCP listener that responds with canned herdr JSON,
// and returns a herdrclient.Client connected to it.
func startMockHerdr(t *testing.T, responseJSON string) (*herdrclient.Client, func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	addr := ln.Addr().String()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// Read one line (the request)
			buf := make([]byte, 4096)
			n, _ := conn.Read(buf)
			_ = n
			// Write response (JSON line)
			conn.Write([]byte(responseJSON + "\n"))
			conn.Close()
		}
	}()

	client := herdrclient.New(addr)
	cleanup := func() {
		ln.Close()
	}

	return client, cleanup
}

func TestBridge_FetchAll_SingleConnection(t *testing.T) {
	// Canned response containing workspaces, agents, and tabs
	response := `{"id":"resp1","result":{"workspaces":[{"workspace_id":"ws-1","label":"main-proj","number":1}],"agents":[{"pane_id":"p1","workspace_id":"ws-1","agent":"pi","name":"review","agent_status":"working","focused":true}],"tabs":[{"tab_id":"t1","label":"main"}]}}`

	client, cleanup := startMockHerdr(t, response)
	defer cleanup()

	b := NewBridge()
	b.AddConnection("local", "LCL", "#4ADE80", client)

	raw := b.FetchAll()
	if len(raw) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(raw))
	}

	ws := raw[0]
	if ws.ConnName != "local" {
		t.Errorf("ConnName: got %s, want local", ws.ConnName)
	}
	if ws.ConnAbbr != "LCL" {
		t.Errorf("ConnAbbr: got %s, want LCL", ws.ConnAbbr)
	}
	if ws.Label != "main-proj" {
		t.Errorf("Label: got %s, want main-proj", ws.Label)
	}

	if len(ws.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(ws.Agents))
	}
	a := ws.Agents[0]
	if a.Agent != "pi" {
		t.Errorf("Agent: got %s, want pi", a.Agent)
	}
	if a.Name != "review" {
		t.Errorf("Name: got %s, want review", a.Name)
	}
	if a.Status != "working" {
		t.Errorf("Status: got %s, want working", a.Status)
	}
	if !a.Focused {
		t.Error("Focused should be true")
	}
}

func TestBridge_FetchAll_MultipleConnections(t *testing.T) {
	// Two different herdr responses
	resp1 := `{"id":"r1","result":{"workspaces":[{"workspace_id":"ws-local","label":"web","number":1}],"agents":[{"pane_id":"p1","workspace_id":"ws-local","agent":"pi","name":"task1","agent_status":"working","focused":true}],"tabs":[{"tab_id":"t1","label":"main"}]}}`
	resp2 := `{"id":"r2","result":{"workspaces":[{"workspace_id":"ws-remote","label":"backend","number":1}],"agents":[{"pane_id":"p2","workspace_id":"ws-remote","agent":"devin","name":"task2","agent_status":"blocked","focused":false}],"tabs":[{"tab_id":"t2","label":"api"}]}}`

	c1, cl1 := startMockHerdr(t, resp1)
	defer cl1()
	c2, cl2 := startMockHerdr(t, resp2)
	defer cl2()

	b := NewBridge()
	b.AddConnection("local", "LCL", "#4ADE80", c1)
	b.AddConnection("dev-server", "DEV", "#60A5FA", c2)

	raw := b.FetchAll()
	if len(raw) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(raw))
	}

	// Check connection metadata is propagated
	if raw[0].ConnName != "local" && raw[0].ConnName != "dev-server" {
		t.Errorf("unexpected ConnName: %s", raw[0].ConnName)
	}

	// Count total agents
	totalAgents := 0
	for _, ws := range raw {
		totalAgents += len(ws.Agents)
	}
	if totalAgents != 2 {
		t.Errorf("total agents: got %d, want 2", totalAgents)
	}
}

func TestBridge_FetchAll_MissingLabel(t *testing.T) {
	// Workspace without label → should use "ws-N"
	response := `{"id":"r","result":{"workspaces":[{"workspace_id":"ws-5","number":5}],"agents":[],"tabs":[]}}`

	client, cleanup := startMockHerdr(t, response)
	defer cleanup()

	b := NewBridge()
	b.AddConnection("test", "TST", "#000", client)

	raw := b.FetchAll()
	if len(raw) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(raw))
	}
	if raw[0].Label != "ws-5" {
		t.Errorf("default label: got %s, want ws-5", raw[0].Label)
	}
}

func TestBridge_FetchAll_TabEnrichment(t *testing.T) {
	response := `{"id":"r","result":{"workspaces":[{"workspace_id":"ws-1","label":"test","number":1}],"agents":[{"pane_id":"p1","workspace_id":"ws-1","agent":"pi","name":"task","agent_status":"working","focused":false,"tab_id":"tab1"}],"tabs":[{"tab_id":"tab1","label":"my-tab"}]}}`

	client, cleanup := startMockHerdr(t, response)
	defer cleanup()

	b := NewBridge()
	b.AddConnection("local", "LCL", "#4ADE80", client)

	raw := b.FetchAll()
	if len(raw) != 1 || len(raw[0].Agents) != 1 {
		t.Fatal("expected 1 agent")
	}

	a := raw[0].Agents[0]
	if a.TabLabel != "my-tab" {
		t.Errorf("TabLabel: got %s, want my-tab", a.TabLabel)
	}
}

func TestBridge_FetchAll_FailedConnection(t *testing.T) {
	// Simulate a connection that will fail (port is likely closed)
	b := NewBridge()
	// Connect to a port that should be refused
	b.AddConnection("bad", "BAD", "#000", herdrclient.New("127.0.0.1:1"))

	raw := b.FetchAll()
	if len(raw) != 0 {
		t.Errorf("failed connection should return empty, got %d workspaces", len(raw))
	}
}

func TestBridge_NewBridge(t *testing.T) {
	b := NewBridge()
	if b == nil {
		t.Fatal("NewBridge should not return nil")
	}
	if len(b.connections) != 0 {
		t.Errorf("new bridge should have 0 connections, got %d", len(b.connections))
	}
}

func TestBridge_FocusAgent(t *testing.T) {
	// FocusAgent should not panic even if connection doesn't exist
	b := NewBridge()
	b.FocusAgent("nonexistent", "p1")
	// Just verify no panic
}

func TestRawAgent_AllFields(t *testing.T) {
	a := RawAgent{
		PaneID:      "p1",
		WorkspaceID: "ws-1",
		TabID:       "t1",
		Agent:       "pi",
		Name:        "review",
		Status:      "working",
		Focused:     true,
		TabLabel:    "main",
	}

	if a.PaneID != "p1" || a.Agent != "pi" || !a.Focused {
		t.Error("RawAgent field mismatch")
	}
}

func TestRawWorkspace_AllFields(t *testing.T) {
	ws := RawWorkspace{
		ConnName:      "local",
		ConnAbbr:      "LCL",
		ConnAbbrColor: "#4ADE80",
		WorkspaceID:   "ws-1",
		Label:         "main-proj",
		Number:        1,
	}

	if ws.ConnName != "local" || ws.Label != "main-proj" {
		t.Error("RawWorkspace field mismatch")
	}
}

func TestFetchConn_ParseError(t *testing.T) {
	// Server returns invalid JSON
	response := `not json at all`

	client, cleanup := startMockHerdr(t, response)
	defer cleanup()

	b := NewBridge()
	b.AddConnection("bad", "BAD", "#000", client)

	raw := b.FetchAll()
	// Should not crash, should return empty
	if len(raw) != 0 {
		t.Errorf("parse error: expected 0 workspaces, got %d", len(raw))
	}
}

func TestFetchConn_EmptyResponse(t *testing.T) {
	// Server returns empty string (no content)
	response := ``

	client, cleanup := startMockHerdr(t, response)
	defer cleanup()

	b := NewBridge()
	b.AddConnection("empty", "EMP", "#000", client)

	raw := b.FetchAll()
	if len(raw) != 0 {
		t.Errorf("empty response: expected 0 workspaces, got %d", len(raw))
	}
}

func TestBridge_FocusAgent_Success(t *testing.T) {
	// Mock herdr that responds to agent.focus (workspaces/agents/tabs also needed
	// because each Request() gets the same mock response)
	resp := `{"id":"resp","result":{"status":"ok"}}`
	client, cleanup := startMockHerdr(t, resp)
	defer cleanup()

	b := NewBridge()
	b.AddConnection("local", "LCL", "#4ADE80", client)

	// FocusAgent sends agent.focus request. Mock responds with {"status":"ok"}
	// which parse as result:{"status":"ok"}. The bridge doesn't check the response,
	// just logs errors. This test verifies no panic and the request path works.
	b.FocusAgent("local", "p1")
}

func TestBridge_FocusAgent_UnknownConnection(t *testing.T) {
	b := NewBridge()
	b.FocusAgent("nonexistent", "p99")
	// Should silently return
}
