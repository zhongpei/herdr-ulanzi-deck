package protocol

import (
	"encoding/json"
	"testing"
)

func TestAgentStatusPriority(t *testing.T) {
	if StatusPriority[StatusBlocked] >= StatusPriority[StatusDone] {
		t.Error("blocked should have higher priority than done")
	}
	if StatusPriority[StatusDone] >= StatusPriority[StatusWorking] {
		t.Error("done should have higher priority than working")
	}
	if StatusPriority[StatusWorking] >= StatusPriority[StatusIdle] {
		t.Error("working should have higher priority than idle")
	}
	if StatusPriority[StatusIdle] >= StatusPriority[StatusUnknown] {
		t.Error("idle should have higher priority than unknown")
	}
}

func TestFleetSnapshot_JSON(t *testing.T) {
	snap := FleetSnapshot{
		Version:   1,
		Seq:       42,
		UpdatedAt: "2026-06-21T10:00:00Z",
		Machines: []MachineInfo{
			{Name: "local", Abbr: "LCL", Color: "#4ADE80"},
		},
		Agents: []AgentState{
			{
				ID:        "local|p1",
				Machine:   "local",
				Agent:     "pi",
				Name:      "review",
				Status:    StatusWorking,
				Focused:   true,
				Workspace: "main-proj",
				PaneID:    "p1",
				UpdatedAt: "2026-06-21T10:00:00Z",
			},
		},
		Stats: AgentStats{Done: 1, Working: 1},
	}

	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}

	var decoded FleetSnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Version != 1 {
		t.Errorf("version: got %d, want 1", decoded.Version)
	}
	if decoded.Seq != 42 {
		t.Errorf("seq: got %d, want 42", decoded.Seq)
	}
	if len(decoded.Agents) != 1 {
		t.Fatalf("agents: got %d, want 1", len(decoded.Agents))
	}

	a := decoded.Agents[0]
	if a.Agent != "pi" {
		t.Errorf("agent: got %q, want pi", a.Agent)
	}
	if a.Status != StatusWorking {
		t.Errorf("status: got %q, want working", a.Status)
	}
	if !a.Focused {
		t.Error("focused should be true")
	}
}

func TestFleetSnapshot_Empty(t *testing.T) {
	snap := FleetSnapshot{Version: 1}
	data, _ := json.Marshal(snap)
	var decoded FleetSnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(decoded.Agents))
	}
}

func TestSubjectConstants(t *testing.T) {
	if SubjectSnapshot != "herdr.v1.snapshot.full" {
		t.Errorf("unexpected SubjectSnapshot: %q", SubjectSnapshot)
	}
	if SubjectHeartbeat != "herdr.v1.collector.heartbeat" {
		t.Errorf("unexpected SubjectHeartbeat: %q", SubjectHeartbeat)
	}
}
