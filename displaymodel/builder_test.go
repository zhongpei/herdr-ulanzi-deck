package displaymodel

import (
	"testing"

	"github.com/herdr-deck/herdrdeck/protocol"
)

// ─── Test helpers ───────────────────────────────────────────

func testSnapshot() *protocol.FleetSnapshot {
	return &protocol.FleetSnapshot{
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
	}
}

// largeSnapshot returns a snapshot with >10 agents to test truncation.
func largeSnapshot() *protocol.FleetSnapshot {
	agents := make([]protocol.AgentState, 15)
	for i := 0; i < 15; i++ {
		agents[i] = protocol.AgentState{
			ID: "local|p" + itoa(i), Machine: "local", Agent: "pi",
			Status: protocol.StatusIdle, PaneID: "p" + itoa(i),
			Workspace: "ws", WorkspaceID: "ws",
		}
	}
	return &protocol.FleetSnapshot{
		Version:  1,
		Machines: []protocol.MachineInfo{{Name: "local", Abbr: "LCL", Color: "#4ADE80"}},
		Agents:   agents,
		Stats:    protocol.AgentStats{Idle: 15},
	}
}

func TestNewBuilder_InAllMode(t *testing.T) {
	b := NewBuilder()
	s := b.State()
	if s.Mode != ModeAll {
		t.Errorf("expected ModeAll, got %v", s.Mode)
	}
	if s.SelectedMachine != "" {
		t.Errorf("expected empty SelectedMachine, got %q", s.SelectedMachine)
	}
	if s.SelectedSpace != "" {
		t.Errorf("expected empty SelectedSpace, got %q", s.SelectedSpace)
	}
	if s.ActiveOnly {
		t.Error("expected ActiveOnly=false")
	}
}

func TestSetAll(t *testing.T) {
	b := NewBuilder()
	b.NextMachine(testSnapshot())
	b.NextSpace(testSnapshot())
	b.SetAll()

	s := b.State()
	if s.Mode != ModeAll {
		t.Errorf("expected ModeAll, got %v", s.Mode)
	}
	if s.SelectedMachine != "" {
		t.Errorf("expected empty SelectedMachine, got %q", s.SelectedMachine)
	}
	if s.SelectedSpace != "" {
		t.Errorf("expected empty SelectedSpace, got %q", s.SelectedSpace)
	}
}

func TestToggleActiveOnly(t *testing.T) {
	b := NewBuilder()
	if b.State().ActiveOnly {
		t.Error("expected default ActiveOnly=false")
	}

	b.ToggleActiveOnly()
	if !b.State().ActiveOnly {
		t.Error("expected ActiveOnly=true after toggle")
	}

	b.ToggleActiveOnly()
	if b.State().ActiveOnly {
		t.Error("expected ActiveOnly=false after second toggle")
	}
}

func TestSetActiveOnly(t *testing.T) {
	b := NewBuilder()
	b.SetActiveOnly(true)
	if !b.State().ActiveOnly {
		t.Error("expected ActiveOnly=true")
	}
	b.SetActiveOnly(false)
	if b.State().ActiveOnly {
		t.Error("expected ActiveOnly=false")
	}
}

// ─── NextMachine ────────────────────────────────────────────

func TestNextMachine_FromAll_SelectsFirst(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	b.NextMachine(snap)

	s := b.State()
	if s.Mode != ModeMachine {
		t.Errorf("expected ModeMachine, got %v", s.Mode)
	}
	// First machine in list is "local"
	if s.SelectedMachine != "local" {
		t.Errorf("expected 'local', got %q", s.SelectedMachine)
	}
	if s.SelectedSpace != "" {
		t.Errorf("expected empty SelectedSpace, got %q", s.SelectedSpace)
	}
}

func TestNextMachine_Cycle(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()

	b.NextMachine(snap) // → local
	b.NextMachine(snap) // → dev-server
	if b.State().SelectedMachine != "dev-server" {
		t.Errorf("expected 'dev-server', got %q", b.State().SelectedMachine)
	}

	b.NextMachine(snap) // wraps → local
	if b.State().SelectedMachine != "local" {
		t.Errorf("expected wrap to 'local', got %q", b.State().SelectedMachine)
	}
}

func TestNextMachine_ClearsSpace(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()

	b.NextSpace(snap)   // → main-proj, Mode=Space
	b.NextMachine(snap) // → local, Mode=Machine

	if b.State().SelectedSpace != "" {
		t.Error("NextMachine should clear SelectedSpace")
	}
	if b.State().Mode != ModeMachine {
		t.Error("NextMachine should set Mode=Machine")
	}
}

func TestNextMachine_NilSnapshot(t *testing.T) {
	b := NewBuilder()
	// Must not panic
	b.NextMachine(nil)
	if b.State().Mode != ModeAll {
		t.Error("nil snapshot should not change mode")
	}
}

func TestNextMachine_EmptyMachines(t *testing.T) {
	b := NewBuilder()
	snap := &protocol.FleetSnapshot{}
	b.NextMachine(snap)
	if b.State().Mode != ModeAll {
		t.Error("empty machines should not change mode")
	}
}

func TestNextMachine_InvalidMachineName(t *testing.T) {
	b := NewBuilder()
	// Push to machine mode first
	b.NextMachine(testSnapshot()) // → local
	b.NextMachine(testSnapshot()) // → dev-server

	// Manually set invalid machine name (simulates stale state)
	b.state.SelectedMachine = "nonexistent"
	snap := testSnapshot()
	b.NextMachine(snap) // should reset to first machine
	if b.State().SelectedMachine != "local" {
		t.Errorf("invalid machine should reset to first, got %q", b.State().SelectedMachine)
	}
}

// ─── NextSpace ──────────────────────────────────────────────

func TestNextSpace_FromAll_SelectsFirst(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	b.NextSpace(snap)

	s := b.State()
	if s.Mode != ModeSpace {
		t.Errorf("expected ModeSpace, got %v", s.Mode)
	}
	// First space is "main-proj" (first agent's workspace, in order)
	if s.SelectedSpace != "main-proj" {
		t.Errorf("expected 'main-proj', got %q", s.SelectedSpace)
	}
	if s.SelectedMachine != "" {
		t.Errorf("expected empty SelectedMachine, got %q", s.SelectedMachine)
	}
}

func TestNextSpace_Cycle(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()

	b.NextSpace(snap) // → main-proj
	b.NextSpace(snap) // → web-app
	if b.State().SelectedSpace != "web-app" {
		t.Errorf("expected 'web-app', got %q", b.State().SelectedSpace)
	}

	b.NextSpace(snap) // → backend
	b.NextSpace(snap) // wraps → main-proj
	if b.State().SelectedSpace != "main-proj" {
		t.Errorf("expected wrap to 'main-proj', got %q", b.State().SelectedSpace)
	}
}

func TestNextSpace_ClearsMachine(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()

	b.NextMachine(snap) // Mode=Machine
	b.NextSpace(snap)   // Mode=Space

	if b.State().SelectedMachine != "" {
		t.Error("NextSpace should clear SelectedMachine")
	}
	if b.State().Mode != ModeSpace {
		t.Error("NextSpace should set Mode=Space")
	}
}

func TestNextSpace_NilSnapshot(t *testing.T) {
	b := NewBuilder()
	b.NextSpace(nil)
	if b.State().Mode != ModeAll {
		t.Error("nil snapshot should not change mode")
	}
}

func TestNextSpace_EmptyAgents(t *testing.T) {
	b := NewBuilder()
	snap := &protocol.FleetSnapshot{}
	b.NextSpace(snap)
	if b.State().Mode != ModeAll {
		t.Error("empty agents should not change mode")
	}
}

func TestNextSpace_StaleLabel(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()

	// Select a space
	b.NextSpace(snap) // → main-proj

	// Simulate: snapshot changes, "main-proj" no longer exists
	smallSnap := &protocol.FleetSnapshot{
		Agents: []protocol.AgentState{
			{Machine: "local", Agent: "pi", Workspace: "new-proj", PaneID: "p1"},
		},
	}
	b.NextSpace(smallSnap)
	// Should reset to first available space = "new-proj"
	if b.State().SelectedSpace != "new-proj" {
		t.Errorf("stale space should reset, got %q", b.State().SelectedSpace)
	}
}

// ─── K12/K13 Independence ───────────────────────────────────

func TestK12AndK13_Independent(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()

	// K12 from ALL → machine mode
	b.NextMachine(snap)
	if b.State().Mode != ModeMachine || b.State().SelectedMachine == "" {
		t.Fatalf("expected Machine mode with machine set")
	}

	// K13 from machine → space mode, clears machine
	b.NextSpace(snap)
	if b.State().Mode != ModeSpace || b.State().SelectedMachine != "" {
		t.Fatalf("expected Space mode with empty machine")
	}

	// K12 from space → machine mode, clears space
	b.NextMachine(snap)
	if b.State().Mode != ModeMachine || b.State().SelectedSpace != "" {
		t.Fatalf("expected Machine mode with empty space")
	}
}

// ─── Build: empty / edge ────────────────────────────────────

func TestBuild_NilSnapshot(t *testing.T) {
	b := NewBuilder()
	// Must not panic
	m := b.Build(nil, LocalStats{}, nil)
	if len(m.Agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(m.Agents))
	}
}

func TestBuild_EmptySnapshot(t *testing.T) {
	b := NewBuilder()
	m := b.Build(&protocol.FleetSnapshot{}, LocalStats{}, nil)
	if len(m.Agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(m.Agents))
	}
	if m.NavAll.Label != "ALL" {
		t.Errorf("expected 'ALL', got %q", m.NavAll.Label)
	}
	if m.NavAll.Active != true {
		t.Error("ALL mode should have Active=true")
	}
	if m.NavMachine.CurrentAbbr != "-" {
		t.Errorf("expected '-', got %q", m.NavMachine.CurrentAbbr)
	}
	if m.NavSpace.CurrentLabel != "-" {
		t.Errorf("expected '-', got %q", m.NavSpace.CurrentLabel)
	}
	if m.NavSpace.NextLabel != "-" {
		t.Errorf("expected '-', got %q", m.NavSpace.NextLabel)
	}
}

// ─── Build: agents ──────────────────────────────────────────

func TestBuild_AgentCount(t *testing.T) {
	b := NewBuilder()
	m := b.Build(testSnapshot(), LocalStats{}, nil)
	if len(m.Agents) != 5 {
		t.Errorf("expected 5 agents, got %d", len(m.Agents))
	}
}

func TestBuild_AgentEnrichment(t *testing.T) {
	b := NewBuilder()
	m := b.Build(testSnapshot(), LocalStats{}, nil)

	// First agent: should have machine abbr/color enriched
	a := m.Agents[0]
	if a.ConnName == "" {
		t.Error("agent should have ConnName")
	}
	if a.ConnAbbr == "" {
		t.Error("agent should have ConnAbbr")
	}
	if a.ConnAbbrColor == "" {
		t.Error("agent should have ConnAbbrColor")
	}
}

func TestBuild_AgentSortOrder(t *testing.T) {
	b := NewBuilder()
	m := b.Build(testSnapshot(), LocalStats{}, nil)

	// Expected sorted order (by StatusPriority, then ConnName):
	// blocked: p5 (devin, dev-server), p2 (cursor, local)
	//   tiebreaker: "dev-server" < "local" alphabetically
	// done:    p4 (claude, local)
	// working: p1 (pi, local)
	// idle:    p3 (pi, local)

	expect := []struct {
		paneID string
		status protocol.AgentStatus
	}{
		{"p5", protocol.StatusBlocked}, // devin, dev-server
		{"p2", protocol.StatusBlocked}, // cursor, local
		{"p4", protocol.StatusDone},    // claude, local
		{"p1", protocol.StatusWorking}, // pi, local
		{"p3", protocol.StatusIdle},    // pi, local
	}

	if len(m.Agents) != len(expect) {
		t.Fatalf("expected %d agents, got %d", len(expect), len(m.Agents))
	}
	for i, exp := range expect {
		if m.Agents[i].PaneID != exp.paneID {
			t.Errorf("agent[%d]: expected PaneID %q, got %q (status=%s)", i, exp.paneID, m.Agents[i].PaneID, m.Agents[i].Status)
		}
		if m.Agents[i].Status != exp.status {
			t.Errorf("agent[%d]: expected status %s, got %s", i, exp.status, m.Agents[i].Status)
		}
	}
}

func TestBuild_AgentCoalesceName(t *testing.T) {
	// Agent with empty Name should fall back to Agent type
	snap := &protocol.FleetSnapshot{
		Machines: []protocol.MachineInfo{{Name: "local", Abbr: "LCL", Color: "#4ADE80"}},
		Agents: []protocol.AgentState{
			{ID: "local|p1", Machine: "local", Agent: "pi", Name: "", PaneID: "p1", Workspace: "ws"},
		},
	}
	b := NewBuilder()
	m := b.Build(snap, LocalStats{}, nil)
	if len(m.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(m.Agents))
	}
	if m.Agents[0].Name != "pi" {
		t.Errorf("expected Name fallback to 'pi', got %q", m.Agents[0].Name)
	}
}

// ─── Build: machine filter ──────────────────────────────────

func TestBuild_MachineFilter(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	b.NextMachine(snap) // → local (4 agents)

	m := b.Build(snap, LocalStats{}, nil)
	if len(m.Agents) != 4 {
		t.Fatalf("local: expected 4 agents, got %d", len(m.Agents))
	}
	for _, a := range m.Agents {
		if a.ConnName != "local" {
			t.Errorf("local filter: got agent from %q", a.ConnName)
		}
	}
}

func TestBuild_MachineFilter_DevServer(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()

	b.NextMachine(snap) // → local
	b.NextMachine(snap) // → dev-server (1 agent)

	m := b.Build(snap, LocalStats{}, nil)
	if len(m.Agents) != 1 {
		t.Fatalf("dev-server: expected 1 agent, got %d", len(m.Agents))
	}
	if m.Agents[0].ConnName != "dev-server" {
		t.Errorf("expected dev-server, got %q", m.Agents[0].ConnName)
	}
}

// ─── Build: space filter ────────────────────────────────────

func TestBuild_SpaceFilter(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	b.NextSpace(snap) // → main-proj (3 agents)

	m := b.Build(snap, LocalStats{}, nil)
	if len(m.Agents) != 3 {
		t.Fatalf("main-proj: expected 3 agents, got %d", len(m.Agents))
	}
	for _, a := range m.Agents {
		if a.WsLabel != "main-proj" {
			t.Errorf("main-proj filter: got agent with WsLabel %q", a.WsLabel)
		}
	}
}

func TestBuild_SpaceFilter_Backend(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()

	// cycle to backend (third space): main-proj → web-app → backend
	b.NextSpace(snap)
	b.NextSpace(snap)
	b.NextSpace(snap)

	m := b.Build(snap, LocalStats{}, nil)
	if len(m.Agents) != 1 {
		t.Fatalf("backend: expected 1 agent, got %d", len(m.Agents))
	}
	if m.Agents[0].WsLabel != "backend" {
		t.Errorf("expected backend, got %q", m.Agents[0].WsLabel)
	}
	if m.Agents[0].ConnName != "dev-server" {
		t.Errorf("expected dev-server, got %q", m.Agents[0].ConnName)
	}
}

// ─── Build: ActiveOnly filter ───────────────────────────────

func TestBuild_ActiveOnly(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	b.SetActiveOnly(true)

	m := b.Build(snap, LocalStats{}, nil)

	// 5 total: 1 idle, 1 unknown (none), 2 blocked, 1 done, 1 working
	// ActiveOnly keeps: blocked(2) + done(1) + working(1) = 4
	if len(m.Agents) != 4 {
		t.Fatalf("ActiveOnly: expected 4 agents, got %d", len(m.Agents))
	}
	for _, a := range m.Agents {
		if a.Status != protocol.StatusBlocked &&
			a.Status != protocol.StatusWorking &&
			a.Status != protocol.StatusDone {
			t.Errorf("ActiveOnly: unexpected status %s", a.Status)
		}
	}
}

func TestBuild_ActiveOnly_WithMachineFilter(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	b.NextMachine(snap) // → local (4 agents: blocked, working, done, idle)
	b.SetActiveOnly(true)

	m := b.Build(snap, LocalStats{}, nil)

	// local agents: p2(blocked), p1(working), p4(done), p3(idle)
	// ActiveOnly: p2 + p1 + p4 = 3
	if len(m.Agents) != 3 {
		t.Fatalf("local + ActiveOnly: expected 3 agents, got %d", len(m.Agents))
	}
	for _, a := range m.Agents {
		if a.ConnName != "local" {
			t.Errorf("expected local agent, got %q", a.ConnName)
		}
		if a.Status == protocol.StatusIdle {
			t.Error("ActiveOnly should filter idle")
		}
	}
}

func TestBuild_ActiveOnly_WithSpaceFilter(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	b.NextSpace(snap) // → main-proj (3 agents: working, blocked, idle)
	b.SetActiveOnly(true)

	m := b.Build(snap, LocalStats{}, nil)

	// main-proj: p1(working), p2(blocked), p3(idle)
	// ActiveOnly: p1 + p2 = 2
	if len(m.Agents) != 2 {
		t.Fatalf("main-proj + ActiveOnly: expected 2 agents, got %d", len(m.Agents))
	}
	for _, a := range m.Agents {
		if a.WsLabel != "main-proj" {
			t.Errorf("expected main-proj agent, got %q", a.WsLabel)
		}
	}
}

// ─── Build: truncation ──────────────────────────────────────

func TestBuild_TruncateTo10(t *testing.T) {
	b := NewBuilder()
	snap := largeSnapshot()
	m := b.Build(snap, LocalStats{}, nil)
	if len(m.Agents) != 10 {
		t.Errorf("expected 10 agents (truncated), got %d", len(m.Agents))
	}
}

// ─── Build: NavAll (K11) ────────────────────────────────────

func TestBuild_NavAll_AllMode(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	m := b.Build(snap, LocalStats{}, nil)

	if m.NavAll.Label != "ALL" {
		t.Errorf("ALL mode: expected 'ALL', got %q", m.NavAll.Label)
	}
	if !m.NavAll.Active {
		t.Error("ALL mode: Active should be true")
	}
	if m.NavAll.Filtered {
		t.Error("default: Filtered should be false")
	}
}

func TestBuild_NavAll_ACTMode(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	b.SetActiveOnly(true)
	m := b.Build(snap, LocalStats{}, nil)

	if m.NavAll.Label != "ACT" {
		t.Errorf("ActiveOnly: expected 'ACT', got %q", m.NavAll.Label)
	}
	if !m.NavAll.Active {
		t.Error("ALL mode (even with ActiveOnly): Active should be true")
	}
	if !m.NavAll.Filtered {
		t.Error("ActiveOnly: Filtered should be true")
	}
}

func TestBuild_NavAll_MachineMode_Inactive(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	b.NextMachine(snap)
	m := b.Build(snap, LocalStats{}, nil)

	if m.NavAll.Active {
		t.Error("Machine mode: K11 should be inactive")
	}
}

func TestBuild_NavAll_SpaceMode_Inactive(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	b.NextSpace(snap)
	m := b.Build(snap, LocalStats{}, nil)

	if m.NavAll.Active {
		t.Error("Space mode: K11 should be inactive")
	}
}

// ─── Build: NavMachine (K12) ────────────────────────────────

func TestBuild_NavMachine_AllMode(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	m := b.Build(snap, LocalStats{}, nil)

	if m.NavMachine.Active {
		t.Error("ALL mode: K12 should be inactive")
	}
	if m.NavMachine.CurrentAbbr != "-" {
		t.Errorf("ALL mode: expected '-', got %q", m.NavMachine.CurrentAbbr)
	}
	if m.NavMachine.NextAbbr != "LCL" {
		t.Errorf("ALL mode: expected 'LCL' as first machine next, got %q", m.NavMachine.NextAbbr)
	}
}

func TestBuild_NavMachine_FirstMachine(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	b.NextMachine(snap) // → local
	m := b.Build(snap, LocalStats{}, nil)

	if !m.NavMachine.Active {
		t.Error("Machine mode: K12 should be active")
	}
	if m.NavMachine.CurrentAbbr != "LCL" {
		t.Errorf("expected 'LCL', got %q", m.NavMachine.CurrentAbbr)
	}
	if m.NavMachine.CurrentColor != "#4ADE80" {
		t.Errorf("expected '#4ADE80', got %q", m.NavMachine.CurrentColor)
	}
	if m.NavMachine.NextAbbr != "DEV" {
		t.Errorf("expected 'DEV', got %q", m.NavMachine.NextAbbr)
	}
}

func TestBuild_NavMachine_SecondMachine(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	b.NextMachine(snap) // → local
	b.NextMachine(snap) // → dev-server
	m := b.Build(snap, LocalStats{}, nil)

	if !m.NavMachine.Active {
		t.Error("Machine mode: K12 should be active")
	}
	if m.NavMachine.CurrentAbbr != "DEV" {
		t.Errorf("expected 'DEV', got %q", m.NavMachine.CurrentAbbr)
	}
	if m.NavMachine.CurrentColor != "#60A5FA" {
		t.Errorf("expected '#60A5FA', got %q", m.NavMachine.CurrentColor)
	}
	if m.NavMachine.NextAbbr != "LCL" {
		t.Errorf("expected wrap to 'LCL', got %q", m.NavMachine.NextAbbr)
	}
}

func TestBuild_NavMachine_SpaceMode_Inactive(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	b.NextSpace(snap)
	m := b.Build(snap, LocalStats{}, nil)

	if m.NavMachine.Active {
		t.Error("Space mode: K12 should be inactive")
	}
}

// ─── Build: NavSpace (K13) ──────────────────────────────────

func TestBuild_NavSpace_AllMode(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	m := b.Build(snap, LocalStats{}, nil)

	if m.NavSpace.Active {
		t.Error("ALL mode: K13 should be inactive")
	}
	if m.NavSpace.CurrentLabel != "-" {
		t.Errorf("ALL mode: expected '-', got %q", m.NavSpace.CurrentLabel)
	}
	if m.NavSpace.NextLabel != "main-proj" {
		t.Errorf("ALL mode: expected 'main-proj' as next, got %q", m.NavSpace.NextLabel)
	}
	if m.NavSpace.Count != 3 {
		t.Errorf("expected 3 spaces, got %d", m.NavSpace.Count)
	}
}

func TestBuild_NavSpace_FirstSpace(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	b.NextSpace(snap) // → main-proj
	m := b.Build(snap, LocalStats{}, nil)

	if !m.NavSpace.Active {
		t.Error("Space mode: K13 should be active")
	}
	if m.NavSpace.CurrentLabel != "main-proj" {
		t.Errorf("expected 'main-proj', got %q", m.NavSpace.CurrentLabel)
	}
	if m.NavSpace.NextLabel != "web-app" {
		t.Errorf("expected 'web-app', got %q", m.NavSpace.NextLabel)
	}
}

func TestBuild_NavSpace_SecondSpace(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	b.NextSpace(snap) // → main-proj
	b.NextSpace(snap) // → web-app
	m := b.Build(snap, LocalStats{}, nil)

	if m.NavSpace.CurrentLabel != "web-app" {
		t.Errorf("expected 'web-app', got %q", m.NavSpace.CurrentLabel)
	}
	if m.NavSpace.NextLabel != "backend" {
		t.Errorf("expected 'backend', got %q", m.NavSpace.NextLabel)
	}
}

func TestBuild_NavSpace_MachineMode_Inactive(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	b.NextMachine(snap)
	m := b.Build(snap, LocalStats{}, nil)

	if m.NavSpace.Active {
		t.Error("Machine mode: K13 should be inactive")
	}
}

// ─── Build: Stats (K14) ─────────────────────────────────────

func TestBuild_Stats(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	m := b.Build(snap, LocalStats{}, nil)

	if m.Stats.AgentStats.Blocked != 2 {
		t.Errorf("blocked: got %d, want 2", m.Stats.AgentStats.Blocked)
	}
	if m.Stats.AgentStats.Working != 1 {
		t.Errorf("working: got %d, want 1", m.Stats.AgentStats.Working)
	}
	if m.Stats.AgentStats.Done != 1 {
		t.Errorf("done: got %d, want 1", m.Stats.AgentStats.Done)
	}
	if m.Stats.AgentStats.Idle != 1 {
		t.Errorf("idle: got %d, want 1", m.Stats.AgentStats.Idle)
	}
	if m.Stats.AgentStats.Unknown != 0 {
		t.Errorf("unknown: got %d, want 0", m.Stats.AgentStats.Unknown)
	}
}

func TestBuild_Stats_CPU_MEM(t *testing.T) {
	b := NewBuilder()
	local := LocalStats{CPUPercent: 48.3, MemoryPercent: 62.7}
	m := b.Build(testSnapshot(), local, nil)

	if m.Stats.CPUPercent != 48.3 {
		t.Errorf("CPU: got %.1f, want 48.3", m.Stats.CPUPercent)
	}
	if m.Stats.MemoryPercent != 62.7 {
		t.Errorf("MEM: got %.1f, want 62.7", m.Stats.MemoryPercent)
	}
}

func TestBuild_Stats_DefaultZero(t *testing.T) {
	b := NewBuilder()
	m := b.Build(testSnapshot(), LocalStats{}, nil)
	if m.Stats.CPUPercent != 0 {
		t.Errorf("expected 0 CPU, got %.1f", m.Stats.CPUPercent)
	}
	if m.Stats.MemoryPercent != 0 {
		t.Errorf("expected 0 MEM, got %.1f", m.Stats.MemoryPercent)
	}
}

// ─── Build: durations ───────────────────────────────────────

func TestBuild_Durations(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	durations := map[string]string{
		"local|p1":      "5m",
		"local|p2":      "30m",
		"dev-server|p5": "1h02m",
	}
	m := b.Build(snap, LocalStats{}, durations)

	// p1 should have duration "5m"
	found := false
	for _, a := range m.Agents {
		if a.PaneID == "p1" {
			if a.StatusDuration != "5m" {
				t.Errorf("p1: expected '5m', got %q", a.StatusDuration)
			}
			found = true
		}
	}
	if !found {
		t.Error("p1 not found in agents")
	}

	// p2: "30m"
	for _, a := range m.Agents {
		if a.PaneID == "p2" && a.StatusDuration != "30m" {
			t.Errorf("p2: expected '30m', got %q", a.StatusDuration)
		}
	}

	// p5 (dev-server): "1h02m"
	for _, a := range m.Agents {
		if a.PaneID == "p5" && a.StatusDuration != "1h02m" {
			t.Errorf("p5: expected '1h02m', got %q", a.StatusDuration)
		}
	}
}

func TestBuild_Durations_Missing(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	durations := map[string]string{
		"nonexistent|p99": "10m",
	}
	m := b.Build(snap, LocalStats{}, durations)

	// All existing agents should have empty duration (not in map)
	for _, a := range m.Agents {
		if a.StatusDuration != "" {
			t.Errorf("%s: expected empty duration, got %q", a.PaneID, a.StatusDuration)
		}
	}
}

func TestBuild_Durations_NilMap(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()
	m := b.Build(snap, LocalStats{}, nil)

	for _, a := range m.Agents {
		if a.StatusDuration != "" {
			t.Errorf("%s: expected empty duration for nil map, got %q", a.PaneID, a.StatusDuration)
		}
	}
}

// ─── Build: space deduplication ─────────────────────────────

func TestBuild_UniqueSpaces(t *testing.T) {
	b := NewBuilder()
	// Snapshot with duplicate workspace labels across machines
	snap := &protocol.FleetSnapshot{
		Version:  1,
		Machines: []protocol.MachineInfo{{Name: "m1", Abbr: "M1", Color: "#000"}},
		Agents: []protocol.AgentState{
			{Machine: "m1", Agent: "pi", Workspace: "shared-proj", PaneID: "p1"},
			{Machine: "m1", Agent: "pi", Workspace: "shared-proj", PaneID: "p2"},
		},
		Stats: protocol.AgentStats{Idle: 2},
	}

	// Empty workspace labels should not create space entries
	m := b.Build(snap, LocalStats{}, nil)
	if m.NavSpace.Count != 1 {
		t.Errorf("expected 1 unique space, got %d", m.NavSpace.Count)
	}
	if m.NavSpace.NextLabel != "shared-proj" {
		t.Errorf("expected 'shared-proj', got %q", m.NavSpace.NextLabel)
	}
}

func TestBuild_SkipEmptyWorkspace(t *testing.T) {
	b := NewBuilder()
	snap := &protocol.FleetSnapshot{
		Version:  1,
		Machines: []protocol.MachineInfo{{Name: "m1", Abbr: "M1", Color: "#000"}},
		Agents: []protocol.AgentState{
			{Machine: "m1", Agent: "pi", Workspace: "", PaneID: "p1"},
			{Machine: "m1", Agent: "pi", Workspace: "", PaneID: "p2"},
		},
	}
	m := b.Build(snap, LocalStats{}, nil)
	if m.NavSpace.Count != 0 {
		t.Errorf("expected 0 spaces when all workspaces are empty, got %d", m.NavSpace.Count)
	}
}

// ─── Build: mode reflects in model ──────────────────────────

func TestBuild_ModeReflected(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()

	// ALL mode
	m := b.Build(snap, LocalStats{}, nil)
	if m.Mode != ModeAll {
		t.Errorf("expected ModeAll, got %v", m.Mode)
	}

	// Machine mode
	b.NextMachine(snap)
	m = b.Build(snap, LocalStats{}, nil)
	if m.Mode != ModeMachine {
		t.Errorf("expected ModeMachine, got %v", m.Mode)
	}

	// Space mode
	b.NextSpace(snap)
	m = b.Build(snap, LocalStats{}, nil)
	if m.Mode != ModeSpace {
		t.Errorf("expected ModeSpace, got %v", m.Mode)
	}
}

// ─── Coverage: edge branches ───────────────────────────────

func TestBuild_StaleSpaceLabel(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()

	// Directly set a space label that doesn't exist in snapshot
	b.SetState(ViewState{Mode: ModeSpace, SelectedSpace: "nonexistent"})
	m := b.Build(snap, LocalStats{}, nil)

	// Should gracefully degrade: space shows "-"
	if m.NavSpace.CurrentLabel != "-" {
		t.Errorf("stale space: expected '-', got %q", m.NavSpace.CurrentLabel)
	}
	// First real space should still show as NextLabel
	if m.NavSpace.NextLabel == "" {
		t.Error("stale space: NextLabel should not be empty")
	}
	// Active reflects the Mode, not whether the space is found
	if !m.NavSpace.Active {
		t.Error("stale space: Active should be true when Mode=ModeSpace")
	}
}

func TestBuild_AgentEmptyAllFields(t *testing.T) {
	b := NewBuilder()

	// Agent with empty Name, TabLabel, and Agent (all coalesce inputs empty)
	snap := &protocol.FleetSnapshot{
		Machines: []protocol.MachineInfo{{Name: "m1", Abbr: "M1", Color: "#000"}},
		Agents: []protocol.AgentState{
			{Machine: "m1", Agent: "", Name: "", Workspace: "ws", PaneID: "p1"},
		},
	}
	m := b.Build(snap, LocalStats{}, nil)
	if len(m.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(m.Agents))
	}
	// coalesce returns "" when all inputs empty
	if m.Agents[0].Name != "" {
		t.Errorf("expected empty name fallback, got %q", m.Agents[0].Name)
	}
}

func TestNextSpace_FromStaleLabel(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()

	// Directly set a nonexistent space label
	b.SetState(ViewState{Mode: ModeSpace, SelectedSpace: "nonexistent"})
	// NextSpace should reset to first real space
	b.NextSpace(snap)

	if b.State().SelectedSpace != "main-proj" {
		t.Errorf("stale label reset: expected 'main-proj', got %q", b.State().SelectedSpace)
	}
	if b.State().Mode != ModeSpace {
		t.Error("should stay in space mode")
	}
}

// ─── State round-trip ───────────────────────────────────────

func TestSetState_RoundTrip(t *testing.T) {
	b := NewBuilder()
	snap := testSnapshot()

	// Build up some state
	b.NextMachine(snap)
	b.NextMachine(snap) // dev-server
	b.SetActiveOnly(true)

	// Save and restore
	saved := b.State()
	b2 := NewBuilder()
	b2.SetState(saved)

	if b2.State().Mode != ModeMachine {
		t.Errorf("expected ModeMachine, got %v", b2.State().Mode)
	}
	if b2.State().SelectedMachine != "dev-server" {
		t.Errorf("expected 'dev-server', got %q", b2.State().SelectedMachine)
	}
	if !b2.State().ActiveOnly {
		t.Error("expected ActiveOnly=true")
	}

	// Build from restored state should produce same model
	m := b.Build(snap, LocalStats{}, nil)
	m2 := b2.Build(snap, LocalStats{}, nil)
	if len(m.Agents) != len(m2.Agents) {
		t.Errorf("agent count mismatch: %d vs %d", len(m.Agents), len(m2.Agents))
	}
}

// ─── itoa helper ────────────────────────────────────────────

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
