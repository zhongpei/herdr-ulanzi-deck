package viewmodel

import (
	"testing"

	"github.com/herdr-deck/herdrdeck/deck/internal/fleet"
	"github.com/herdr-deck/herdrdeck/protocol"
)

// buildTestFleet creates a fleet.Manager populated with real data.
func buildTestFleet() *fleet.Manager {
	fm := fleet.NewManager()
	fm.ApplySnapshot(&protocol.FleetSnapshot{
		Version:   protocol.SchemaVersion,
		Seq:       1,
		UpdatedAt: "2026-06-21T10:00:00Z",
		Machines: []protocol.MachineInfo{
			{Name: "local", Abbr: "LCL", Color: "#4ADE80"},
			{Name: "dev-server", Abbr: "DEV", Color: "#60A5FA"},
		},
		Agents: []protocol.AgentState{
			{ID: "local|p1", Machine: "local", Agent: "pi", Name: "review", Status: protocol.StatusWorking, Focused: true, Workspace: "main-proj", WorkspaceID: "ws-1", PaneID: "p1"},
			{ID: "local|p2", Machine: "local", Agent: "cursor", Status: protocol.StatusBlocked, Workspace: "main-proj", WorkspaceID: "ws-1", PaneID: "p2"},
			{ID: "local|p3", Machine: "local", Agent: "pi", Name: "idle", Status: protocol.StatusIdle, Workspace: "main-proj", WorkspaceID: "ws-1", PaneID: "p3"},
			{ID: "local|p4", Machine: "local", Agent: "claude", Name: "api-done", Status: protocol.StatusDone, Workspace: "web-app", WorkspaceID: "ws-2", PaneID: "p4"},
			{ID: "dev-server|p5", Machine: "dev-server", Agent: "devin", Status: protocol.StatusBlocked, Workspace: "backend", WorkspaceID: "ws-3", PaneID: "p5"},
		},
		Stats: protocol.AgentStats{Done: 1, Idle: 1, Working: 1, Blocked: 2},
	})
	return fm
}

func TestNewBuilder_InAllMode(t *testing.T) {
	fm := buildTestFleet()
	b := NewBuilder(fm)
	if b.Mode != ModeAll {
		t.Errorf("expected ModeAll, got %v", b.Mode)
	}
}

func TestBuilder_SetAll(t *testing.T) {
	fm := buildTestFleet()
	b := NewBuilder(fm)
	b.NextMachine()
	b.SetAll()
	if b.Mode != ModeAll {
		t.Errorf("expected ModeAll, got %v", b.Mode)
	}
	if b.ConnName != "" {
		t.Errorf("expected empty ConnName, got %s", b.ConnName)
	}
	if b.WsLabel != "" {
		t.Errorf("expected empty WsLabel, got %s", b.WsLabel)
	}
}

func TestBuilder_NextMachine_FromAll(t *testing.T) {
	fm := buildTestFleet()
	b := NewBuilder(fm)
	b.NextMachine()
	if b.Mode != ModeMachine {
		t.Errorf("expected ModeMachine, got %v", b.Mode)
	}
	// First machine should be "dev-server" (alphabetical? No — "local" is first in machines list)
	if b.ConnName != "local" {
		t.Errorf("expected 'local', got %s", b.ConnName)
	}
	if b.WsLabel != "" {
		t.Errorf("expected empty WsLabel after NextMachine, got %s", b.WsLabel)
	}
}

func TestBuilder_NextMachine_Cycle(t *testing.T) {
	fm := buildTestFleet()
	b := NewBuilder(fm)
	b.NextMachine() // → local
	b.NextMachine() // → dev-server
	if b.ConnName != "dev-server" {
		t.Errorf("expected 'dev-server', got %s", b.ConnName)
	}
	b.NextMachine() // wraps → local
	if b.ConnName != "local" {
		t.Errorf("expected wrap to 'local', got %s", b.ConnName)
	}
}

func TestBuilder_NextMachine_ClearsSpace(t *testing.T) {
	fm := buildTestFleet()
	b := NewBuilder(fm)
	b.NextMachine() // → local
	if b.WsLabel != "" {
		t.Error("WsLabel should be cleared on NextMachine")
	}
}

func TestBuilder_NextSpace_FromMachine(t *testing.T) {
	fm := buildTestFleet()
	b := NewBuilder(fm)
	b.NextMachine() // → local
	b.NextSpace()
	if b.Mode != ModeSpace {
		t.Errorf("expected ModeSpace, got %v", b.Mode)
	}
	// First space label
	if b.WsLabel != "main-proj" {
		t.Errorf("expected 'main-proj', got '%s'", b.WsLabel)
	}
	if b.ConnName != "" {
		t.Errorf("ConnName should be empty in space mode, got '%s'", b.ConnName)
	}
}

func TestBuilder_NextSpace_Cycle(t *testing.T) {
	fm := buildTestFleet()
	b := NewBuilder(fm)
	b.NextSpace() // → main-proj
	b.NextSpace() // → web-app
	if b.WsLabel != "web-app" {
		t.Errorf("expected 'web-app', got '%s'", b.WsLabel)
	}
	b.NextSpace() // → backend
	b.NextSpace() // wraps → main-proj
	if b.WsLabel != "main-proj" {
		t.Errorf("expected wrap to 'main-proj', got '%s'", b.WsLabel)
	}
}

func TestBuilder_Build_Has14Keys(t *testing.T) {
	fm := buildTestFleet()
	b := NewBuilder(fm)
	keys := b.Build()
	if len(keys) != 14 {
		t.Fatalf("expected 14 key commands, got %d", len(keys))
	}
}

func TestBuilder_Build_First10AreAgentsOrEmpty(t *testing.T) {
	fm := buildTestFleet()
	b := NewBuilder(fm)
	keys := b.Build()
	for i := 0; i < 10; i++ {
		if keys[i].Agent == nil && keys[i].Empty == nil {
			t.Errorf("key[%d]: expected agent or empty, got type=%s", i, keys[i].Type())
		}
	}
}

func TestBuilder_Build_NavButtonsPresent(t *testing.T) {
	fm := buildTestFleet()
	b := NewBuilder(fm)
	keys := b.Build()

	if keys[10].NavAll == nil {
		t.Error("K11 (index 10): expected NavAll")
	}
	if keys[11].NavMac == nil {
		t.Error("K12 (index 11): expected NavMac")
	}
	if keys[12].NavSpc == nil {
		t.Error("K13 (index 12): expected NavSpc")
	}
	if keys[13].Stats == nil {
		t.Error("K14 (index 13): expected Stats")
	}
}

func TestBuilder_Build_AgentCount(t *testing.T) {
	fm := buildTestFleet()
	b := NewBuilder(fm)
	keys := b.Build()
	count := 0
	for i := 0; i < 10; i++ {
		if keys[i].Agent != nil {
			count++
		}
	}
	// 5 agents total
	if count != 5 {
		t.Errorf("expected 5 agents, got %d", count)
	}
}

func TestBuilder_Build_MachineFilterCount(t *testing.T) {
	fm := buildTestFleet()
	b := NewBuilder(fm)
	b.NextMachine() // → local: 4 agents
	keys := b.Build()
	count := 0
	for i := 0; i < 10; i++ {
		if keys[i].Agent != nil {
			count++
		}
	}
	if count != 4 {
		t.Errorf("local filter: expected 4 agents, got %d", count)
	}
}

func TestBuilder_Build_SpaceFilterCount(t *testing.T) {
	fm := buildTestFleet()
	b := NewBuilder(fm)
	b.NextSpace() // → main-proj: 3 agents
	keys := b.Build()
	count := 0
	for i := 0; i < 10; i++ {
		if keys[i].Agent != nil {
			count++
		}
	}
	if count != 3 {
		t.Errorf("main-proj filter: expected 3 agents, got %d", count)
	}
}

func TestBuilder_Build_AgentFields(t *testing.T) {
	fm := buildTestFleet()
	b := NewBuilder(fm)
	keys := b.Build()

	// First agent should be blocked (highest priority)
	a := keys[0].Agent
	if a == nil {
		t.Fatal("expected non-nil first agent")
	}
	if a.Status != "blocked" {
		t.Errorf("status: got %s, want blocked", a.Status)
	}
	if a.AgentType != "devin" && a.AgentType != "cursor" {
		t.Logf("first agent type: %s (blocked status)", a.AgentType)
	}
}

func TestBuilder_Build_K11Label_All(t *testing.T) {
	fm := buildTestFleet()
	b := NewBuilder(fm)
	keys := b.Build()
	if keys[10].NavAll.Label != "ALL" {
		t.Errorf("K11 label: got %s, want ALL", keys[10].NavAll.Label)
	}
	if keys[10].NavAll.Filtered {
		t.Error("K11 should not be filtered by default")
	}
}

func TestBuilder_Build_K11Label_ACT(t *testing.T) {
	fm := buildTestFleet()
	b := NewBuilder(fm)
	b.K11Filtered = true
	keys := b.Build()
	if keys[10].NavAll.Label != "ACT" {
		t.Errorf("K11 label: got %s, want ACT", keys[10].NavAll.Label)
	}
	if !keys[10].NavAll.Filtered {
		t.Error("K11 should be filtered when K11Filtered=true")
	}
}

func TestBuilder_Build_K14Stats(t *testing.T) {
	fm := buildTestFleet()
	b := NewBuilder(fm)
	keys := b.Build()

	s := keys[13].Stats
	if s == nil {
		t.Fatal("K14 missing")
	}
	if s.Stats.Blocked != 2 {
		t.Errorf("blocked: got %d, want 2", s.Stats.Blocked)
	}
	if s.Stats.Working != 1 {
		t.Errorf("working: got %d, want 1", s.Stats.Working)
	}
	if s.Stats.Done != 1 {
		t.Errorf("done: got %d, want 1", s.Stats.Done)
	}
}

func TestKeyCommand_Type(t *testing.T) {
	kc := KeyCommand{Agent: &AgentKeyData{Type: "agent"}}
	if kc.Type() != "agent" {
		t.Errorf("Type(): got %s, want agent", kc.Type())
	}

	kc = KeyCommand{Empty: &EmptyKeyData{Type: "empty"}}
	if kc.Type() != "empty" {
		t.Errorf("Type(): got %s, want empty", kc.Type())
	}

	kc = KeyCommand{}
	if kc.Type() != "empty" {
		t.Errorf("Type() on empty KeyCommand: got %s, want empty", kc.Type())
	}
}

func TestBuilder_Build_EmptyKeys(t *testing.T) {
	fm := fleet.NewManager()
	// Empty fleet
	b := NewBuilder(fm)
	keys := b.Build()

	if len(keys) != 14 {
		t.Fatalf("expected 14 keys even with empty fleet, got %d", len(keys))
	}
	for i := 0; i < 10; i++ {
		if keys[i].Empty == nil {
			t.Errorf("key[%d]: expected empty, got %s", i, keys[i].Type())
		}
	}
	// K11-K14 should exist even with no data
	if keys[10].NavAll == nil {
		t.Error("K11 should exist with empty fleet")
	}
	if keys[11].NavMac == nil {
		t.Error("K12 should exist with empty fleet")
	}
	if keys[12].NavSpc == nil {
		t.Error("K13 should exist with empty fleet")
	}
	if keys[13].Stats == nil {
		t.Error("K14 should exist with empty fleet")
	}
}

func TestBuilder_K12AndK13_Independent(t *testing.T) {
	fm := buildTestFleet()
	b := NewBuilder(fm)

	// K12 from ALL → machine mode
	b.NextMachine()
	if b.Mode != ModeMachine || b.ConnName == "" {
		t.Fatalf("expected Machine mode with ConnName set")
	}
	// K13 from machine → space mode, clears ConnName
	b.NextSpace()
	if b.Mode != ModeSpace || b.ConnName != "" {
		t.Fatalf("expected Space mode with empty ConnName")
	}
	// K12 from space → machine mode, clears WsLabel
	b.NextMachine()
	if b.Mode != ModeMachine || b.WsLabel != "" {
		t.Fatalf("expected Machine mode with empty WsLabel")
	}
}

func TestBuilder_itoa(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{99, "99"},
		{100, "100"},
		{-1, "-1"},
	}
	for _, tt := range tests {
		got := itoa(tt.n)
		if got != tt.want {
			t.Errorf("itoa(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
