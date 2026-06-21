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
			buf := make([]byte, 4096)
			n, _ := conn.Read(buf)
			_ = n
			conn.Write([]byte(responseJSON + "\n"))
			conn.Close()
		}
	}()

	client := herdrclient.New(addr)
	cleanup := func() { ln.Close() }
	return client, cleanup
}

func TestBridge_FetchAll_SingleConnection(t *testing.T) {
	response := `{"id":"resp1","result":{"workspaces":[{"workspace_id":"ws-1","label":"main-proj","number":1}],"agents":[{"pane_id":"p1","workspace_id":"ws-1","agent":"pi","name":"review","agent_status":"working","focused":true}],"tabs":[{"tab_id":"t1","label":"main"}]}}`

	client, cleanup := startMockHerdr(t, response)
	defer cleanup()

	b := NewBridge()
	b.AddConnection("local", "LCL", "#4ADE80", client)

	results := b.FetchAll()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Err != nil {
		t.Fatalf("unexpected error: %v", r.Err)
	}
	if r.ConnName != "local" {
		t.Errorf("ConnName: got %s, want local", r.ConnName)
	}
	if r.ConnAbbr != "LCL" {
		t.Errorf("ConnAbbr: got %s, want LCL", r.ConnAbbr)
	}
	if len(r.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(r.Workspaces))
	}
	ws := r.Workspaces[0]
	if ws.Label != "main-proj" {
		t.Errorf("Label: got %s, want main-proj", ws.Label)
	}
	if len(ws.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(ws.Agents))
	}
	a := ws.Agents[0]
	if a.Agent != "pi" || a.Name != "review" || a.Status != "working" || !a.Focused {
		t.Error("agent field mismatch")
	}
}

func TestBridge_FetchAll_MultipleConnections(t *testing.T) {
	resp1 := `{"id":"r1","result":{"workspaces":[{"workspace_id":"ws-local","label":"web","number":1}],"agents":[{"pane_id":"p1","workspace_id":"ws-local","agent":"pi","name":"task1","agent_status":"working","focused":true}],"tabs":[{"tab_id":"t1","label":"main"}]}}`
	resp2 := `{"id":"r2","result":{"workspaces":[{"workspace_id":"ws-remote","label":"backend","number":1}],"agents":[{"pane_id":"p2","workspace_id":"ws-remote","agent":"devin","name":"task2","agent_status":"blocked","focused":false}],"tabs":[{"tab_id":"t2","label":"api"}]}}`

	c1, cl1 := startMockHerdr(t, resp1)
	defer cl1()
	c2, cl2 := startMockHerdr(t, resp2)
	defer cl2()

	b := NewBridge()
	b.AddConnection("local", "LCL", "#4ADE80", c1)
	b.AddConnection("dev-server", "DEV", "#60A5FA", c2)

	results := b.FetchAll()
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Count total agents across all workspaces
	totalAgents := 0
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error for %s: %v", r.ConnName, r.Err)
		}
		for _, ws := range r.Workspaces {
			totalAgents += len(ws.Agents)
		}
	}
	if totalAgents != 2 {
		t.Errorf("total agents: got %d, want 2", totalAgents)
	}
}

func TestBridge_FetchAll_MissingLabel(t *testing.T) {
	response := `{"id":"r","result":{"workspaces":[{"workspace_id":"ws-5","number":5}],"agents":[],"tabs":[]}}`

	client, cleanup := startMockHerdr(t, response)
	defer cleanup()

	b := NewBridge()
	b.AddConnection("test", "TST", "#000", client)

	results := b.FetchAll()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("unexpected error: %v", results[0].Err)
	}
	if len(results[0].Workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(results[0].Workspaces))
	}
	if results[0].Workspaces[0].Label != "ws-5" {
		t.Errorf("default label: got %s, want ws-5", results[0].Workspaces[0].Label)
	}
}

func TestBridge_FetchAll_TabEnrichment(t *testing.T) {
	response := `{"id":"r","result":{"workspaces":[{"workspace_id":"ws-1","label":"test","number":1}],"agents":[{"pane_id":"p1","workspace_id":"ws-1","agent":"pi","name":"task","agent_status":"working","focused":false,"tab_id":"tab1"}],"tabs":[{"tab_id":"tab1","label":"my-tab"}]}}`

	client, cleanup := startMockHerdr(t, response)
	defer cleanup()

	b := NewBridge()
	b.AddConnection("local", "LCL", "#4ADE80", client)

	results := b.FetchAll()
	r := results[0]
	if r.Err != nil || len(r.Workspaces) != 1 {
		t.Fatal("expected workspace")
	}
	if len(r.Workspaces[0].Agents) != 1 {
		t.Fatal("expected agent")
	}
	if r.Workspaces[0].Agents[0].TabLabel != "my-tab" {
		t.Errorf("TabLabel: got %s, want my-tab", r.Workspaces[0].Agents[0].TabLabel)
	}
}

func TestBridge_FetchAll_FailedConnection(t *testing.T) {
	b := NewBridge()
	b.AddConnection("bad", "BAD", "#000", herdrclient.New("127.0.0.1:1"))

	results := b.FetchAll()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Error("expected error for bad connection")
	}
}

func TestBridge_FocusAgent(t *testing.T) {
	b := NewBridge()
	b.FocusAgent("nonexistent", "p1") // should not panic
}

func TestBridge_FocusAgent_Success(t *testing.T) {
	resp := `{"id":"resp","result":{"status":"ok"}}`
	client, cleanup := startMockHerdr(t, resp)
	defer cleanup()

	b := NewBridge()
	b.AddConnection("local", "LCL", "#4ADE80", client)
	b.FocusAgent("local", "p1") // should not panic
}

func TestBridge_FetchAll_ConnectionsList(t *testing.T) {
	b := NewBridge()
	if len(b.Connections()) != 0 {
		t.Errorf("new bridge: expected 0 connections, got %d", len(b.Connections()))
	}
	b.AddConnection("a", "A", "#000", herdrclient.New("tcp://127.0.0.1:1"))
	b.AddConnection("b", "B", "#111", herdrclient.New("tcp://127.0.0.1:2"))
	if len(b.Connections()) != 2 {
		t.Errorf("expected 2 connections, got %d", len(b.Connections()))
	}
}

func TestBridge_FetchAll_ParseError(t *testing.T) {
	response := `not json`
	client, cleanup := startMockHerdr(t, response)
	defer cleanup()

	b := NewBridge()
	b.AddConnection("bad", "BAD", "#000", client)

	results := b.FetchAll()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Error("expected parse error")
	}
}

func TestBridge_NewBridge(t *testing.T) {
	b := NewBridge()
	if b == nil {
		t.Fatal("NewBridge returned nil")
	}
}
