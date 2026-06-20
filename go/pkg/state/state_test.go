package state

import (
	"testing"

	"github.com/herdr-deck/herdrdeck/pkg/types"
)

// buildTestData returns a controlled workspace set for deterministic tests.
func buildTestData() []types.UnifiedWorkspace {
	return []types.UnifiedWorkspace{
		{
			ConnName:      "local",
			ConnAbbr:      "LCL",
			ConnAbbrColor: "#4ADE80",
			WorkspaceID:   "ws-1",
			Label:         "main-proj",
			Agents: []types.AgentInfo{
				{Agent: "pi", Name: "review", AgentStatus: types.StatusWorking, Focused: true, PaneID: "p1", WorkspaceID: "ws-1"},
				{Agent: "cursor", Name: "fix-bug", AgentStatus: types.StatusBlocked, PaneID: "p2", WorkspaceID: "ws-1"},
				{Agent: "pi", Name: "idle", AgentStatus: types.StatusIdle, PaneID: "p3", WorkspaceID: "ws-1"},
			},
		},
		{
			ConnName:      "local",
			ConnAbbr:      "LCL",
			ConnAbbrColor: "#4ADE80",
			WorkspaceID:   "ws-2",
			Label:         "web-app",
			Agents: []types.AgentInfo{
				{Agent: "claude", Name: "api-done", AgentStatus: types.StatusDone, PaneID: "p4", WorkspaceID: "ws-2"},
				{Agent: "pi", Name: "feat-auth", AgentStatus: types.StatusWorking, PaneID: "p5", WorkspaceID: "ws-2"},
			},
		},
		{
			ConnName:      "dev-server",
			ConnAbbr:      "DEV",
			ConnAbbrColor: "#60A5FA",
			WorkspaceID:   "ws-3",
			Label:         "backend",
			Agents: []types.AgentInfo{
				{Agent: "gemini", Name: "waiting", AgentStatus: types.StatusIdle, PaneID: "p6", WorkspaceID: "ws-3"},
				{Agent: "copilot", Name: "deploy", AgentStatus: types.StatusWorking, PaneID: "p7", WorkspaceID: "ws-3"},
				{Agent: "devin", Name: "test-fail", AgentStatus: types.StatusBlocked, PaneID: "p8", WorkspaceID: "ws-3"},
				{Agent: "cursor", Name: "done", AgentStatus: types.StatusDone, PaneID: "p9", WorkspaceID: "ws-3"},
				{Agent: "cline", Name: "unknown-act", AgentStatus: types.StatusUnknown, PaneID: "p10", WorkspaceID: "ws-3"},
			},
		},
	}
}

func TestInitAndGetAllAgents(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())

	agents := m.GetAllAgents()
	if len(agents) != 10 {
		t.Fatalf("expected 10 agents, got %d", len(agents))
	}

	// Check enrichment
	for _, a := range agents {
		if a.ConnName == "" {
			t.Errorf("agent %s missing ConnName", a.PaneID)
		}
		if a.ConnAbbr == "" {
			t.Errorf("agent %s missing ConnAbbr", a.PaneID)
		}
	}
}

func TestGetFilteredAgents_NoFilter(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())

	agents := m.GetFilteredAgents("", "")
	if len(agents) > 10 {
		t.Fatalf("expected ≤10 agents, got %d", len(agents))
	}

	// Sort order: blocked first, then done, working, idle, unknown
	prevPriority := -1
	for _, a := range agents {
		p := types.StatusPriority[a.AgentStatus]
		if p < prevPriority {
			t.Errorf("sort violation: %s (%d) after priority %d", a.PaneID, p, prevPriority)
		}
		prevPriority = p
	}
}

func TestGetFilteredAgents_MachineFilter(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())

	agents := m.GetFilteredAgents("local", "")
	if len(agents) != 5 {
		t.Fatalf("expected 5 local agents, got %d", len(agents))
	}
	for _, a := range agents {
		if a.ConnName != "local" {
			t.Errorf("expected local only, got %s", a.ConnName)
		}
	}
}

func TestGetFilteredAgents_SpaceFilter(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())

	agents := m.GetFilteredAgents("local", "ws-1")
	if len(agents) != 3 {
		t.Fatalf("expected 3 agents in ws-1, got %d", len(agents))
	}
	for _, a := range agents {
		if a.WorkspaceID != "ws-1" {
			t.Errorf("expected ws-1 only, got %s", a.WorkspaceID)
		}
	}
}

func TestGetFilteredAgents_UnknownMachine(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())

	agents := m.GetFilteredAgents("nonexistent", "")
	if len(agents) != 0 {
		t.Errorf("expected 0 agents for unknown machine, got %d", len(agents))
	}
}

func TestGetMachines(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())

	machines := m.GetMachines()
	if len(machines) != 2 {
		t.Fatalf("expected 2 machines, got %d", len(machines))
	}
	if machines[0].ConnName != "local" {
		t.Errorf("expected first machine 'local', got %s", machines[0].ConnName)
	}
	if machines[1].ConnName != "dev-server" {
		t.Errorf("expected second machine 'dev-server', got %s", machines[1].ConnName)
	}
}

func TestGetMachines_Dedup(t *testing.T) {
	m := NewManager()
	// Three workspaces, two unique machines
	data := []types.UnifiedWorkspace{
		{ConnName: "a", ConnAbbr: "A", ConnAbbrColor: "#111"},
		{ConnName: "a", ConnAbbr: "A", ConnAbbrColor: "#111"},
		{ConnName: "b", ConnAbbr: "B", ConnAbbrColor: "#222"},
	}
	m.Init(data)

	machines := m.GetMachines()
	if len(machines) != 2 {
		t.Fatalf("expected 2 unique machines, got %d", len(machines))
	}
}

func TestGetSpaces(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())

	spaces := m.GetSpaces("local")
	if len(spaces) != 2 {
		t.Fatalf("expected 2 spaces for local, got %d", len(spaces))
	}
	if spaces[0].Label != "main-proj" {
		t.Errorf("expected first space 'main-proj', got %s", spaces[0].Label)
	}
}

func TestGetSpaces_NoMatch(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())

	spaces := m.GetSpaces("nowhere")
	if len(spaces) != 0 {
		t.Errorf("expected 0 spaces, got %d", len(spaces))
	}
}

func TestComputeStats(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())

	stats := m.ComputeStats()
	// local  ws-1: 1 working, 1 blocked, 1 idle
	// local  ws-2: 1 done, 1 working
	// dev    ws-3: 1 idle, 1 working, 1 blocked, 1 done, 1 unknown
	expected := types.AgentStats{Done: 2, Idle: 2, Working: 3, Blocked: 2, Unknown: 1}
	if stats != expected {
		t.Errorf("stats mismatch:\n  got:  %+v\n  want: %+v", stats, expected)
	}
}

func TestComputeStats_Empty(t *testing.T) {
	m := NewManager()
	m.Init(nil)
	stats := m.ComputeStats()
	zero := types.AgentStats{}
	if stats != zero {
		t.Errorf("expected zero stats for empty manager, got %+v", stats)
	}
}

func TestOnChange(t *testing.T) {
	m := NewManager()
	called := false
	m.OnChange(func(event string, data any) {
		called = true
		if event != "stateChanged" {
			t.Errorf("expected event 'stateChanged', got %s", event)
		}
	})
	m.Init(buildTestData())
	if !called {
		t.Error("listener was not called after Init")
	}
}

func TestFilteredSort_BlockedFirst(t *testing.T) {
	m := NewManager()
	// Agents in reverse priority order to test sort
	data := []types.UnifiedWorkspace{
		{
			ConnName:    "x",
			ConnAbbr:    "X",
			WorkspaceID: "ws-1",
			Label:       "x",
			Agents: []types.AgentInfo{
				{Agent: "a", AgentStatus: types.StatusIdle, PaneID: "idle"},
				{Agent: "b", AgentStatus: types.StatusBlocked, PaneID: "blocked"},
				{Agent: "c", AgentStatus: types.StatusDone, PaneID: "done"},
				{Agent: "d", AgentStatus: types.StatusWorking, PaneID: "working"},
				{Agent: "e", AgentStatus: types.StatusUnknown, PaneID: "unknown"},
			},
		},
	}
	m.Init(data)

	agents := m.GetFilteredAgents("", "")
	expectedOrder := []string{"blocked", "done", "working", "idle", "unknown"}
	for i, a := range agents {
		if a.PaneID != expectedOrder[i] {
			t.Errorf("position %d: expected %s, got %s", i, expectedOrder[i], a.PaneID)
		}
	}
}

func TestTruncateTo10(t *testing.T) {
	m := NewManager()
	// 12 agents in one workspace
	agents := make([]types.AgentInfo, 12)
	for i := 0; i < 12; i++ {
		agents[i] = types.AgentInfo{
			Agent:       "a",
			AgentStatus: types.StatusIdle,
			PaneID:      "p" + string(rune('a'+i)),
		}
	}
	data := []types.UnifiedWorkspace{
		{ConnName: "x", ConnAbbr: "X", ConnAbbrColor: "#000", WorkspaceID: "ws-1", Label: "x", Agents: agents},
	}
	m.Init(data)

	result := m.GetFilteredAgents("", "")
	if len(result) > 10 {
		t.Errorf("expected max 10 agents, got %d", len(result))
	}
}
