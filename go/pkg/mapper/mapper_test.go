package mapper

import (
	"testing"

	"github.com/herdr-deck/herdrdeck/pkg/state"
	"github.com/herdr-deck/herdrdeck/pkg/types"
)

// buildTestManager creates a mapper with known test data.
func buildTestManager() *Mapper {
	sm := state.NewManager()
	sm.Init([]types.UnifiedWorkspace{
		{
			ConnName: "local", ConnAbbr: "LCL", ConnAbbrColor: "#4ADE80",
			WorkspaceID: "ws-1", Label: "main-proj",
			Agents: []types.AgentInfo{
				{Agent: "pi", Name: "review", AgentStatus: types.StatusWorking, PaneID: "p1", WorkspaceID: "ws-1"},
				{Agent: "cursor", AgentStatus: types.StatusBlocked, PaneID: "p2", WorkspaceID: "ws-1"},
				{Agent: "pi", Name: "idle", AgentStatus: types.StatusIdle, PaneID: "p3", WorkspaceID: "ws-1"},
			},
		},
		{
			ConnName: "local", ConnAbbr: "LCL", ConnAbbrColor: "#4ADE80",
			WorkspaceID: "ws-2", Label: "web-app",
			Agents: []types.AgentInfo{
				{Agent: "claude", Name: "api-done", AgentStatus: types.StatusDone, PaneID: "p4", WorkspaceID: "ws-2"},
			},
		},
		{
			ConnName: "dev-server", ConnAbbr: "DEV", ConnAbbrColor: "#60A5FA",
			WorkspaceID: "ws-3", Label: "backend",
			Agents: []types.AgentInfo{
				{Agent: "devin", Name: "test-fail", AgentStatus: types.StatusBlocked, PaneID: "p5", WorkspaceID: "ws-3"},
			},
		},
	})
	return New(sm)
}

func TestNew_InAllMode(t *testing.T) {
	m := buildTestManager()
	if m.Mode != ModeAll {
		t.Errorf("expected ModeAll, got %v", m.Mode)
	}
}

func TestSetAll(t *testing.T) {
	m := buildTestManager()
	m.NextMachine() // go to machine mode
	m.SetAll()
	if m.Mode != ModeAll {
		t.Errorf("expected ModeAll after SetAll, got %v", m.Mode)
	}
	if m.ConnName != "" {
		t.Errorf("expected empty ConnName, got %s", m.ConnName)
	}
	if m.WsID != "" {
		t.Errorf("expected empty WsID, got %s", m.WsID)
	}
}

func TestNextMachine_FromAll(t *testing.T) {
	m := buildTestManager()
	m.NextMachine()
	if m.Mode != ModeMachine {
		t.Errorf("expected ModeMachine, got %v", m.Mode)
	}
	if m.ConnName != "local" {
		t.Errorf("expected first machine 'local', got %s", m.ConnName)
	}
}

func TestNextMachine_Cycle(t *testing.T) {
	m := buildTestManager()
	m.NextMachine() // → local
	m.NextMachine() // → dev-server
	if m.ConnName != "dev-server" {
		t.Errorf("expected 'dev-server', got %s", m.ConnName)
	}
}

func TestNextMachine_Wraps(t *testing.T) {
	m := buildTestManager()
	m.NextMachine() // → local
	m.NextMachine() // → dev-server
	m.NextMachine() // wraps → local
	if m.ConnName != "local" {
		t.Errorf("expected wrap to 'local', got %s", m.ConnName)
	}
}

func TestNextMachine_ClearsSpace(t *testing.T) {
	m := buildTestManager()
	m.NextMachine() // → local
	m.NextSpace()   // → ws-1
	m.NextMachine() // → dev-server, should clear space
	if m.Mode != ModeMachine {
		t.Errorf("expected ModeMachine, got %v", m.Mode)
	}
	if m.WsID != "" {
		t.Errorf("expected WsID cleared, got %s", m.WsID)
	}
}

func TestNextMachine_Empty(t *testing.T) {
	sm := state.NewManager()
	sm.Init(nil)
	m := New(sm)
	m.NextMachine() // no-op
	if m.Mode != ModeAll {
		t.Errorf("expected ModeAll on empty state, got %v", m.Mode)
	}
}

func TestNextSpace_FromMachine(t *testing.T) {
	m := buildTestManager()
	m.NextMachine() // → local
	m.NextSpace()
	if m.Mode != ModeSpace {
		t.Errorf("expected ModeSpace, got %v", m.Mode)
	}
	if m.WsID != "ws-1" {
		t.Errorf("expected first space 'ws-1', got %s", m.WsID)
	}
}

func TestNextSpace_Cycle(t *testing.T) {
	m := buildTestManager()
	m.NextMachine() // → local
	m.NextSpace()   // → ws-1
	m.NextSpace()   // → ws-2
	if m.WsID != "ws-2" {
		t.Errorf("expected 'ws-2', got %s", m.WsID)
	}
}

func TestNextSpace_Wraps(t *testing.T) {
	m := buildTestManager()
	m.NextMachine() // → local (2 spaces)
	m.NextSpace()   // → ws-1
	m.NextSpace()   // → ws-2
	m.NextSpace()   // wraps → ws-1
	if m.WsID != "ws-1" {
		t.Errorf("expected wrap to 'ws-1', got %s", m.WsID)
	}
}

func TestNextSpace_AllMode(t *testing.T) {
	m := buildTestManager()
	m.NextSpace() // no-op in ALL mode
	if m.Mode != ModeAll {
		t.Errorf("expected ModeAll unchanged, got %v", m.Mode)
	}
}

func TestRenderAll_Has14Keys(t *testing.T) {
	m := buildTestManager()
	keys := m.RenderAll()
	if len(keys) != 14 {
		t.Fatalf("expected 14 key commands, got %d", len(keys))
	}
}

func TestRenderAll_First10AreAgentsOrEmpty(t *testing.T) {
	m := buildTestManager()
	keys := m.RenderAll()
	for i := 0; i < 10; i++ {
		if keys[i].Agent == nil && keys[i].Empty == nil {
			t.Errorf("key[%d] type=%s: expected agent or empty", i, keys[i].Type())
		}
	}
}

func TestRenderAll_NavButtonsPresent(t *testing.T) {
	m := buildTestManager()
	keys := m.RenderAll()
	// K11 = index 10
	if keys[10].NavAll == nil {
		t.Error("K11 (index 10): expected NavAll")
	}
	// K12 = index 11
	if keys[11].NavMac == nil {
		t.Error("K12 (index 11): expected NavMac")
	}
	// K13 = index 12
	if keys[12].NavSpc == nil {
		t.Error("K13 (index 12): expected NavSpc")
	}
	// K14 = index 13
	if keys[13].Stats == nil {
		t.Error("K14 (index 13): expected Stats")
	}
}

func TestRenderAll_FilteredCount(t *testing.T) {
	m := buildTestManager()
	m.NextMachine() // → local — 4 agents if no space filter
	keys := m.RenderAll()
	agentCount := 0
	for i := 0; i < 10; i++ {
		if keys[i].Agent != nil {
			agentCount++
		}
	}
	if agentCount != 4 {
		t.Errorf("expected 4 agents for local, got %d", agentCount)
	}
}

func TestRenderAll_SpaceFilterCount(t *testing.T) {
	m := buildTestManager()
	m.NextMachine() // → local
	m.NextSpace()   // → ws-1 (3 agents)
	keys := m.RenderAll()
	agentCount := 0
	for i := 0; i < 10; i++ {
		if keys[i].Agent != nil {
			agentCount++
		}
	}
	if agentCount != 3 {
		t.Errorf("expected 3 agents for ws-1, got %d", agentCount)
	}
}

func TestRenderAll_AgentData(t *testing.T) {
	m := buildTestManager()
	keys := m.RenderAll()
	// First agent in ALL mode should be highest priority (blocked)
	// tie-breaking by connName: "dev-server" < "local" alphabetically → devin first
	first := keys[0].Agent
	if first == nil {
		t.Fatal("expected non-nil first agent")
	}
	if first.Status != "blocked" {
		t.Errorf("expected status 'blocked', got '%s'", first.Status)
	}
	if first.ConnName != "dev-server" {
		t.Errorf("expected first from 'dev-server' (alphabetical order), got '%s'", first.ConnName)
	}
}
