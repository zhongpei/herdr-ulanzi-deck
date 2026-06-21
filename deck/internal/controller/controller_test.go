package controller

import (
	"testing"

	"github.com/herdr-deck/herdrdeck/deck/internal/fleet"
	"github.com/herdr-deck/herdrdeck/deck/internal/viewmodel"
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
	bm := viewmodel.NewBuilder(fm)
	c := NewController(fm, bm)

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
	bm := viewmodel.NewBuilder(fm)
	c := NewController(fm, bm)

	snap := c.Capture()
	if snap.Mode != viewmodel.ModeAll {
		t.Errorf("mode: got %v, want ModeAll", snap.Mode)
	}
	if snap.Stats.Working != 1 || snap.Stats.Blocked != 1 {
		t.Errorf("stats: got %+v", snap.Stats)
	}
}

func TestController_HashChanges_OnStateChange(t *testing.T) {
	fm := buildControllerFleet()
	bm := viewmodel.NewBuilder(fm)
	c := NewController(fm, bm)

	// Capture initial hash
	snap1 := c.Capture()
	hash1 := snap1.Hash()

	// Change viewmodel mode
	bm.NextMachine()
	snap2 := c.Capture()
	hash2 := snap2.Hash()

	if hash1 == hash2 {
		t.Error("hash should change when filter mode changes")
	}
	if !snap2.ChangedSince(hash1) {
		t.Error("ChangedSince should return true")
	}
}

func TestController_HashSame_OnUnchangedState(t *testing.T) {
	fm := buildControllerFleet()
	bm := viewmodel.NewBuilder(fm)
	c := NewController(fm, bm)

	snap1 := c.Capture()
	hash1 := snap1.Hash()

	// No state change → same hash
	snap2 := c.Capture()
	hash2 := snap2.Hash()

	if hash1 != hash2 {
		t.Errorf("hash should be same for unchanged state: %q vs %q", hash1, hash2)
	}
	if snap2.ChangedSince(hash1) {
		t.Error("ChangedSince should return false")
	}
}

func TestController_HashChanges_OnAgentStatusChange(t *testing.T) {
	fm := buildControllerFleet()
	bm := viewmodel.NewBuilder(fm)
	c := NewController(fm, bm)

	snap1 := c.Capture()
	hash1 := snap1.Hash()

	// Change agent status
	fm.ApplySnapshot(&protocol.FleetSnapshot{
		Version: 1, Seq: 2,
		Machines: []protocol.MachineInfo{{Name: "local", Abbr: "LCL", Color: "#4ADE80"}},
		Agents: []protocol.AgentState{
			{ID: "local|p1", Machine: "local", Agent: "pi", Name: "review", Status: protocol.StatusDone, Focused: true, Workspace: "main-proj", WorkspaceID: "ws-1", PaneID: "p1"},
			{ID: "local|p2", Machine: "local", Agent: "cursor", Status: protocol.StatusBlocked, Workspace: "main-proj", WorkspaceID: "ws-1", PaneID: "p2"},
		},
		Stats: protocol.AgentStats{Done: 1, Blocked: 1},
	})
	c.MarkDirty()

	snap2 := c.Capture()
	hash2 := snap2.Hash()

	if hash1 == hash2 {
		t.Error("hash should change when agent status changes")
	}
}

func TestController_HashChanges_OnK11Toggle(t *testing.T) {
	fm := buildControllerFleet()
	bm := viewmodel.NewBuilder(fm)
	c := NewController(fm, bm)

	snap1 := c.Capture()
	hash1 := snap1.Hash()

	// Toggle K11
	fm.SetK11Toggle(true)
	fm.ToggleK11Filter()
	bm.K11Filtered = true

	snap2 := c.Capture()
	hash2 := snap2.Hash()

	if hash1 == hash2 {
		t.Error("hash should change when K11 toggle changes")
	}
}

func TestController_Capture_FilterModeReflected(t *testing.T) {
	fm := buildControllerFleet()
	bm := viewmodel.NewBuilder(fm)
	c := NewController(fm, bm)

	// ALL mode
	snap := c.Capture()
	if snap.ConnName != "" {
		t.Error("ALL mode: ConnName should be empty")
	}

	// Machine mode
	bm.NextMachine()
	snap = c.Capture()
	if snap.Mode != viewmodel.ModeMachine {
		t.Errorf("expected ModeMachine, got %v", snap.Mode)
	}
	if snap.ConnName == "" {
		t.Error("Machine mode: ConnName should be set")
	}

	// Space mode
	bm.NextSpace()
	snap = c.Capture()
	if snap.Mode != viewmodel.ModeSpace {
		t.Errorf("expected ModeSpace, got %v", snap.Mode)
	}
	if snap.WsLabel == "" {
		t.Error("Space mode: WsLabel should be set")
	}
}

func TestController_EmptyFleet(t *testing.T) {
	fm := fleet.NewManager()
	bm := viewmodel.NewBuilder(fm)
	c := NewController(fm, bm)

	snap := c.Capture()
	if snap.Stats != (protocol.AgentStats{}) {
		t.Errorf("empty fleet: expected zero stats, got %+v", snap.Stats)
	}

	hash := snap.Hash()
	if hash == "" {
		t.Error("hash should not be empty even for empty fleet")
	}
}
