package controller

import (
	"testing"

	"github.com/herdr-deck/herdrdeck/deck/internal/fleet"
	"github.com/herdr-deck/herdrdeck/displaymodel"
	"github.com/herdr-deck/herdrdeck/protocol"
)

// buildControllerFleet returns a fleet manager with sample data for controller tests.
func buildControllerFleet() *fleet.Manager {
	fm := fleet.NewManager()
	fm.ApplySnapshot(&protocol.FleetSnapshot{
		Version:   1,
		Seq:       1,
		UpdatedAt: "2026-06-21T10:00:00Z",
		Machines: []protocol.MachineInfo{
			{Name: "local", Abbr: "LCL", Color: "#4ADE80"},
		},
		Agents: []protocol.AgentState{
			{ID: "local|p1", Machine: "local", Agent: "pi", Name: "review", Status: protocol.StatusWorking, Focused: true, Workspace: "main-proj", WorkspaceID: "ws-1", PaneID: "p1"},
			{ID: "local|p2", Machine: "local", Agent: "cursor", Status: protocol.StatusBlocked, Workspace: "main-proj", WorkspaceID: "ws-1", PaneID: "p2"},
		},
		Stats: protocol.AgentStats{Working: 1, Blocked: 1},
	})
	return fm
}

func TestController_DirtyFlag(t *testing.T) {
	fm := buildControllerFleet()
	bld := displaymodel.NewBuilder()
	c := NewController(fm, bld, true)

	if c.IsDirty() {
		t.Error("should start clean")
	}

	c.MarkDirty()
	if !c.IsDirty() {
		t.Error("should be dirty after MarkDirty")
	}

	c.MarkClean()
	if c.IsDirty() {
		t.Error("should be clean after MarkClean")
	}
}

func TestController_Capture(t *testing.T) {
	fm := buildControllerFleet()
	bld := displaymodel.NewBuilder()
	c := NewController(fm, bld, true)

	// Apply snapshot so Capture has data
	snap := &protocol.FleetSnapshot{
		Version:   1,
		Seq:       1,
		UpdatedAt: "2026-06-21T10:00:00Z",
		Machines:  []protocol.MachineInfo{{Name: "local", Abbr: "LCL", Color: "#4ADE80"}},
		Agents: []protocol.AgentState{
			{ID: "local|p1", Machine: "local", Agent: "pi", Name: "review", Status: protocol.StatusWorking, Focused: true, Workspace: "main-proj", WorkspaceID: "ws-1", PaneID: "p1"},
			{ID: "local|p2", Machine: "local", Agent: "cursor", Status: protocol.StatusBlocked, Workspace: "main-proj", WorkspaceID: "ws-1", PaneID: "p2"},
		},
		Stats: protocol.AgentStats{Working: 1, Blocked: 1},
	}
	c.ApplySnapshot(snap)

	capState := c.Capture()
	if capState.Model.Mode != displaymodel.ModeAll {
		t.Errorf("mode: got %v, want ModeAll", capState.Model.Mode)
	}
	if capState.Model.Stats.AgentStats.Working != 1 || capState.Model.Stats.AgentStats.Blocked != 1 {
		t.Errorf("stats: got %+v", capState.Model.Stats.AgentStats)
	}
}

func TestController_HashChanges_OnStateChange(t *testing.T) {
	fm := buildControllerFleet()
	bld := displaymodel.NewBuilder()
	c := NewController(fm, bld, true)

	snap := &protocol.FleetSnapshot{
		Version: 1, Seq: 1,
		Machines: []protocol.MachineInfo{
			{Name: "local", Abbr: "LCL", Color: "#4ADE80"},
			{Name: "dev-server", Abbr: "DEV", Color: "#60A5FA"},
		},
		Agents: []protocol.AgentState{
			{ID: "local|p1", Machine: "local", Agent: "pi", Name: "review", Status: protocol.StatusWorking, Focused: true, Workspace: "main-proj", PaneID: "p1"},
			{ID: "local|p2", Machine: "local", Agent: "cursor", Status: protocol.StatusBlocked, Workspace: "main-proj", PaneID: "p2"},
		},
		Stats: protocol.AgentStats{Working: 1, Blocked: 1},
	}
	c.ApplySnapshot(snap)

	// Capture initial hash
	cap1 := c.Capture()
	hash1 := cap1.Hash()

	// Change mode via K12
	c.OnK12()
	cap2 := c.Capture()
	hash2 := cap2.Hash()

	if hash1 == hash2 {
		t.Error("hash should change when filter mode changes")
	}
	if !cap2.ChangedSince(hash1) {
		t.Error("ChangedSince should return true")
	}
}

func TestController_HashSame_OnUnchangedState(t *testing.T) {
	fm := buildControllerFleet()
	bld := displaymodel.NewBuilder()
	c := NewController(fm, bld, true)

	snap := &protocol.FleetSnapshot{
		Version: 1, Seq: 1,
		Machines: []protocol.MachineInfo{
			{Name: "local", Abbr: "LCL", Color: "#4ADE80"},
		},
		Agents: []protocol.AgentState{
			{ID: "local|p1", Machine: "local", Agent: "pi", Status: protocol.StatusWorking, Workspace: "main-proj", PaneID: "p1"},
		},
		Stats: protocol.AgentStats{Working: 1},
	}
	c.ApplySnapshot(snap)

	cap1 := c.Capture()
	hash1 := cap1.Hash()

	// No state change → same hash
	cap2 := c.Capture()
	hash2 := cap2.Hash()

	if hash1 != hash2 {
		t.Errorf("hash should be same for unchanged state: %q vs %q", hash1, hash2)
	}
	if cap2.ChangedSince(hash1) {
		t.Error("ChangedSince should return false")
	}
}

func TestController_HashChanges_OnAgentStatusChange(t *testing.T) {
	fm := buildControllerFleet()
	bld := displaymodel.NewBuilder()
	c := NewController(fm, bld, true)

	initialSnap := &protocol.FleetSnapshot{
		Version: 1, Seq: 1,
		Machines: []protocol.MachineInfo{{Name: "local", Abbr: "LCL", Color: "#4ADE80"}},
		Agents: []protocol.AgentState{
			{ID: "local|p1", Machine: "local", Agent: "pi", Name: "review", Status: protocol.StatusWorking, Focused: true, Workspace: "main-proj", PaneID: "p1"},
			{ID: "local|p2", Machine: "local", Agent: "cursor", Status: protocol.StatusBlocked, Workspace: "main-proj", PaneID: "p2"},
		},
		Stats: protocol.AgentStats{Working: 1, Blocked: 1},
	}
	c.ApplySnapshot(initialSnap)
	cap1 := c.Capture()
	hash1 := cap1.Hash()

	// Apply new snapshot with changed status
	changedSnap := &protocol.FleetSnapshot{
		Version: 1, Seq: 2,
		Machines: []protocol.MachineInfo{{Name: "local", Abbr: "LCL", Color: "#4ADE80"}},
		Agents: []protocol.AgentState{
			{ID: "local|p1", Machine: "local", Agent: "pi", Name: "review", Status: protocol.StatusDone, Focused: true, Workspace: "main-proj", PaneID: "p1"},
			{ID: "local|p2", Machine: "local", Agent: "cursor", Status: protocol.StatusBlocked, Workspace: "main-proj", PaneID: "p2"},
		},
		Stats: protocol.AgentStats{Done: 1, Blocked: 1},
	}
	fm.ApplySnapshot(changedSnap)
	c.ApplySnapshot(changedSnap)
	c.MarkDirty()

	cap2 := c.Capture()
	hash2 := cap2.Hash()

	if hash1 == hash2 {
		t.Error("hash should change when agent status changes")
	}
}

func TestController_HashChanges_OnK11Toggle(t *testing.T) {
	fm := buildControllerFleet()
	bld := displaymodel.NewBuilder()
	c := NewController(fm, bld, true)

	snap := &protocol.FleetSnapshot{
		Version: 1, Seq: 1,
		Machines: []protocol.MachineInfo{{Name: "local", Abbr: "LCL", Color: "#4ADE80"}},
		Agents: []protocol.AgentState{
			{ID: "local|p1", Machine: "local", Agent: "pi", Status: protocol.StatusWorking, Workspace: "main-proj", PaneID: "p1"},
		},
		Stats: protocol.AgentStats{Working: 1},
	}
	c.ApplySnapshot(snap)

	cap1 := c.Capture()
	hash1 := cap1.Hash()

	// Toggle via K11
	c.OnK11()

	cap2 := c.Capture()
	hash2 := cap2.Hash()

	if hash1 == hash2 {
		t.Error("hash should change when K11 toggle changes")
	}
}

func TestController_Capture_FilterModeReflected(t *testing.T) {
	fm := buildControllerFleet()
	bld := displaymodel.NewBuilder()
	c := NewController(fm, bld, true)

	snap := &protocol.FleetSnapshot{
		Version: 1, Seq: 1,
		Machines: []protocol.MachineInfo{
			{Name: "local", Abbr: "LCL", Color: "#4ADE80"},
			{Name: "dev-server", Abbr: "DEV", Color: "#60A5FA"},
		},
		Agents: []protocol.AgentState{
			{ID: "local|p1", Machine: "local", Agent: "pi", Status: protocol.StatusWorking, Workspace: "main-proj", PaneID: "p1"},
			{ID: "dev-server|p2", Machine: "dev-server", Agent: "devin", Status: protocol.StatusBlocked, Workspace: "backend", PaneID: "p2"},
		},
		Stats: protocol.AgentStats{Working: 1, Blocked: 1},
	}
	c.ApplySnapshot(snap)

	// ALL mode
	capState := c.Capture()
	if capState.Model.Mode != displaymodel.ModeAll {
		t.Errorf("expected ModeAll, got %v", capState.Model.Mode)
	}

	// Machine mode
	c.OnK12()
	capState = c.Capture()
	if capState.Model.Mode != displaymodel.ModeMachine {
		t.Errorf("expected ModeMachine, got %v", capState.Model.Mode)
	}

	// Space mode
	c.OnK13()
	capState = c.Capture()
	if capState.Model.Mode != displaymodel.ModeSpace {
		t.Errorf("expected ModeSpace, got %v", capState.Model.Mode)
	}
}

func TestController_LastModel(t *testing.T) {
	fm := buildControllerFleet()
	bld := displaymodel.NewBuilder()
	c := NewController(fm, bld, true)

	// No model before first Capture
	if c.LastModel() != nil {
		t.Error("LastModel should be nil before first Capture")
	}

	snap := &protocol.FleetSnapshot{
		Version: 1, Seq: 1,
		Machines: []protocol.MachineInfo{{Name: "local", Abbr: "LCL", Color: "#4ADE80"}},
		Agents: []protocol.AgentState{
			{ID: "local|p1", Machine: "local", Agent: "pi", Status: protocol.StatusWorking, Workspace: "main-proj", PaneID: "p1"},
		},
		Stats: protocol.AgentStats{Working: 1},
	}
	c.ApplySnapshot(snap)

	_ = c.Capture()
	m := c.LastModel()
	if m == nil {
		t.Fatal("LastModel should be non-nil after Capture")
	}
	if len(m.Agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(m.Agents))
	}
}

func TestController_EmptyFleet(t *testing.T) {
	fm := fleet.NewManager()
	bld := displaymodel.NewBuilder()
	c := NewController(fm, bld, true)

	// No snapshot applied → Capture returns empty model
	capState := c.Capture()
	if capState.Model.Stats.AgentStats != (protocol.AgentStats{}) {
		t.Errorf("empty fleet: expected zero stats, got %+v", capState.Model.Stats.AgentStats)
	}
	if capState.Hash() != "" {
		t.Error("hash should be empty string when no snapshot applied")
	}

	// Apply empty snapshot
	c.ApplySnapshot(&protocol.FleetSnapshot{})
	capState = c.Capture()
	if len(capState.Model.Agents) != 0 {
		t.Errorf("empty snapshot: expected 0 agents, got %d", len(capState.Model.Agents))
	}
}
