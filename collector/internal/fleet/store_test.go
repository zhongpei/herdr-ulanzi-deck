package fleet

import (
	"testing"
	"time"

	"github.com/herdr-deck/herdrdeck/collector/internal/bridge"
	"github.com/herdr-deck/herdrdeck/protocol"
)

func buildTestRaw() []bridge.RawWorkspace {
	return []bridge.RawWorkspace{
		{
			ConnName: "local", ConnAbbr: "LCL", ConnAbbrColor: "#4ADE80",
			WorkspaceID: "ws-1", Label: "main-proj",
			Agents: []bridge.RawAgent{
				{Agent: "pi", Name: "review", Status: "working", PaneID: "p1", WorkspaceID: "ws-1", Focused: true},
				{Agent: "cursor", Status: "blocked", PaneID: "p2", WorkspaceID: "ws-1"},
				{Agent: "pi", Name: "idle", Status: "idle", PaneID: "p3", WorkspaceID: "ws-1"},
			},
		},
		{
			ConnName: "dev-server", ConnAbbr: "DEV", ConnAbbrColor: "#60A5FA",
			WorkspaceID: "ws-2", Label: "backend",
			Agents: []bridge.RawAgent{
				{Agent: "gemini", Name: "done", Status: "done", PaneID: "p4", WorkspaceID: "ws-2"},
				{Agent: "devin", Name: "fail", Status: "blocked", PaneID: "p5", WorkspaceID: "ws-2"},
			},
		},
	}
}

func TestStoreApplyRaw(t *testing.T) {
	s := NewStore()
	raw := buildTestRaw()
	changed := s.ApplyRaw(raw)
	if !changed {
		t.Error("first apply should report changed")
	}

	snap := s.Snapshot()
	if snap.Version != protocol.SchemaVersion {
		t.Errorf("version: got %d, want %d", snap.Version, protocol.SchemaVersion)
	}
	if snap.Seq != 1 {
		t.Errorf("seq: got %d, want 1", snap.Seq)
	}
	if len(snap.Agents) != 5 {
		t.Fatalf("agents: got %d, want 5", len(snap.Agents))
	}
	if len(snap.Machines) != 2 {
		t.Fatalf("machines: got %d, want 2", len(snap.Machines))
	}
	if snap.Machines[0].Name != "local" {
		t.Errorf("first machine: got %s, want local", snap.Machines[0].Name)
	}
}

func TestStoreStats(t *testing.T) {
	s := NewStore()
	s.ApplyRaw(buildTestRaw())
	snap := s.Snapshot()

	// 2 blocked (p2, p5), 1 done (p4), 1 working (p1), 1 idle (p3), 0 unknown
	if snap.Stats.Blocked != 2 {
		t.Errorf("blocked: got %d, want 2", snap.Stats.Blocked)
	}
	if snap.Stats.Done != 1 {
		t.Errorf("done: got %d, want 1", snap.Stats.Done)
	}
	if snap.Stats.Working != 1 {
		t.Errorf("working: got %d, want 1", snap.Stats.Working)
	}
	if snap.Stats.Idle != 1 {
		t.Errorf("idle: got %d, want 1", snap.Stats.Idle)
	}
	if snap.Stats.Unknown != 0 {
		t.Errorf("unknown: got %d, want 0", snap.Stats.Unknown)
	}
}

func TestStoreSeqIncrements(t *testing.T) {
	s := NewStore()
	s.ApplyRaw(buildTestRaw())
	if s.Snapshot().Seq != 1 {
		t.Errorf("first seq should be 1")
	}
	s.ApplyRaw(buildTestRaw()) // same data
	if s.Snapshot().Seq != 2 {
		t.Errorf("seq should increment even with same data")
	}
}

func TestStoreUnchangedDetection(t *testing.T) {
	s := NewStore()
	raw := buildTestRaw()

	changed := s.ApplyRaw(raw)
	if !changed {
		t.Error("first apply should report changed")
	}

	// Apply same data again → should report unchanged
	changed = s.ApplyRaw(raw)
	if changed {
		t.Error("same data should report unchanged")
	}
}

func TestStoreUnchangedAcrossTime(t *testing.T) {
	// Same raw data applied with a time gap should produce changed=false
	// when only UpdatedAt differs (the bug: old code compared UpdatedAt).
	s := NewStore()
	raw := buildTestRaw()

	s.ApplyRaw(raw)

	// Simulate a time delay
	time.Sleep(2 * time.Second)

	changed := s.ApplyRaw(raw)
	if changed {
		t.Error("same data after time delay should report unchanged")
	}
}

func TestStoreAgentFields(t *testing.T) {
	s := NewStore()
	s.ApplyRaw(buildTestRaw())
	snap := s.Snapshot()

	// Find agent p1
	var a *protocol.AgentState
	for i := range snap.Agents {
		if snap.Agents[i].PaneID == "p1" {
			a = &snap.Agents[i]
			break
		}
	}
	if a == nil {
		t.Fatal("agent p1 not found")
	}
	if a.ID != "local|p1" {
		t.Errorf("ID: got %s, want local|p1", a.ID)
	}
	if a.Machine != "local" {
		t.Errorf("Machine: got %s, want local", a.Machine)
	}
	if a.Workspace != "main-proj" {
		t.Errorf("Workspace: got %s, want main-proj", a.Workspace)
	}
	if !a.Focused {
		t.Error("p1 should be focused")
	}
}

func TestStoreEmpty(t *testing.T) {
	s := NewStore()
	s.ApplyRaw(nil)
	snap := s.Snapshot()

	if len(snap.Agents) != 0 {
		t.Errorf("empty: expected 0 agents, got %d", len(snap.Agents))
	}
	if snap.Stats != (protocol.AgentStats{}) {
		t.Errorf("empty: expected zero stats, got %+v", snap.Stats)
	}
}

func TestMapStatus(t *testing.T) {
	tests := []struct {
		raw  string
		want protocol.AgentStatus
	}{
		{"done", protocol.StatusDone},
		{"idle", protocol.StatusIdle},
		{"working", protocol.StatusWorking},
		{"blocked", protocol.StatusBlocked},
		{"unknown", protocol.StatusUnknown},
		{"garbage", protocol.StatusUnknown},
		{"", protocol.StatusUnknown},
	}
	for _, tt := range tests {
		got := mapStatus(tt.raw)
		if got != tt.want {
			t.Errorf("mapStatus(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}
