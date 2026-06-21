package fleet

import (
	"testing"
	"time"

	"github.com/herdr-deck/herdrdeck/protocol"
)

// buildSnapshot returns a FleetSnapshot with real-world data: 3 machines,
// 12 agents across 5 workspaces, mixed statuses.
func buildSnapshot() *protocol.FleetSnapshot {
	return &protocol.FleetSnapshot{
		Version:   protocol.SchemaVersion,
		Seq:       7,
		UpdatedAt: "2026-06-21T10:00:00Z",
		Machines: []protocol.MachineInfo{
			{Name: "local", Abbr: "LCL", Color: "#4ADE80"},
			{Name: "dev-server", Abbr: "DEV", Color: "#60A5FA"},
			{Name: "prod", Abbr: "PRD", Color: "#F87171"},
		},
		Agents: []protocol.AgentState{
			// local, main-proj
			{ID: "local|p1", Machine: "local", Agent: "pi", Name: "review", Status: protocol.StatusWorking, Focused: true, Workspace: "main-proj", WorkspaceID: "ws-1", PaneID: "p1", UpdatedAt: "2026-06-21T10:00:00Z"},
			{ID: "local|p2", Machine: "local", Agent: "cursor", Name: "fix-bug", Status: protocol.StatusBlocked, Workspace: "main-proj", WorkspaceID: "ws-1", PaneID: "p2", UpdatedAt: "2026-06-21T10:00:00Z"},
			{ID: "local|p3", Machine: "local", Agent: "pi", Name: "idle", Status: protocol.StatusIdle, Workspace: "main-proj", WorkspaceID: "ws-1", PaneID: "p3", UpdatedAt: "2026-06-21T10:00:00Z"},
			{ID: "local|p4", Machine: "local", Agent: "claude", Name: "api-done", Status: protocol.StatusDone, Workspace: "web-app", WorkspaceID: "ws-2", PaneID: "p4", UpdatedAt: "2026-06-21T10:00:00Z"},
			{ID: "local|p5", Machine: "local", Agent: "pi", Name: "feat-auth", Status: protocol.StatusWorking, Workspace: "web-app", WorkspaceID: "ws-2", PaneID: "p5", UpdatedAt: "2026-06-21T10:00:00Z"},
			// dev-server, backend
			{ID: "dev-server|p6", Machine: "dev-server", Agent: "gemini", Name: "waiting", Status: protocol.StatusIdle, Workspace: "backend", WorkspaceID: "ws-3", PaneID: "p6", UpdatedAt: "2026-06-21T10:00:00Z"},
			{ID: "dev-server|p7", Machine: "dev-server", Agent: "copilot", Name: "deploy", Status: protocol.StatusWorking, Workspace: "backend", WorkspaceID: "ws-3", PaneID: "p7", UpdatedAt: "2026-06-21T10:00:00Z"},
			{ID: "dev-server|p8", Machine: "dev-server", Agent: "devin", Name: "test-fail", Status: protocol.StatusBlocked, Workspace: "backend", WorkspaceID: "ws-3", PaneID: "p8", UpdatedAt: "2026-06-21T10:00:00Z"},
			{ID: "dev-server|p9", Machine: "dev-server", Agent: "cursor", Name: "done", Status: protocol.StatusDone, Workspace: "backend", WorkspaceID: "ws-3", PaneID: "p9", UpdatedAt: "2026-06-21T10:00:00Z"},
			{ID: "dev-server|p10", Machine: "dev-server", Agent: "cline", Name: "unknown-act", Status: protocol.StatusUnknown, Workspace: "backend", WorkspaceID: "ws-3", PaneID: "p10", UpdatedAt: "2026-06-21T10:00:00Z"},
			// prod, infra
			{ID: "prod|p11", Machine: "prod", Agent: "cline", Name: "tf-plan", Status: protocol.StatusWorking, Workspace: "infra", WorkspaceID: "ws-4", PaneID: "p11", UpdatedAt: "2026-06-21T10:00:00Z"},
			// prod, monitoring
			{ID: "prod|p12", Machine: "prod", Agent: "grok", Name: "alert-45", Status: protocol.StatusBlocked, Workspace: "monitoring", WorkspaceID: "ws-5", PaneID: "p12", UpdatedAt: "2026-06-21T10:00:00Z"},
		},
		Stats: protocol.AgentStats{Done: 2, Idle: 2, Working: 4, Blocked: 3, Unknown: 1},
	}
}

func TestManager_ApplySnapshot(t *testing.T) {
	m := NewManager()
	snap := buildSnapshot()
	m.ApplySnapshot(snap)

	// Verify agents are populated
	all := m.GetAllAgents()
	if len(all) != 12 {
		t.Fatalf("expected 12 agents, got %d", len(all))
	}

	// Verify machine metadata enriched
	for _, a := range all {
		if a.ConnName == "" {
			t.Errorf("agent %s missing ConnName", a.PaneID)
		}
		if a.ConnAbbr == "" {
			t.Errorf("agent %s missing ConnAbbr", a.PaneID)
		}
	}

	// Verify focused agent
	found := false
	for _, a := range all {
		if a.PaneID == "p1" && a.Focused {
			found = true
			break
		}
	}
	if !found {
		t.Error("agent p1 should be focused")
	}
}

func TestManager_GetFilteredAgents_NoFilter(t *testing.T) {
	m := NewManager()
	m.ApplySnapshot(buildSnapshot())

	// No filter → top 10 sorted by priority
	agents := m.GetFilteredAgents("", "")
	if len(agents) > 10 {
		t.Fatalf("max 10 agents, got %d", len(agents))
	}

	// Sort order: blocked > done > working > idle > unknown
	prev := -1
	for _, a := range agents {
		p := protocol.StatusPriority[a.Status]
		if p < prev {
			t.Errorf("sort violation: %s (prio %d) after prio %d", a.PaneID, p, prev)
		}
		prev = p
	}
}

func TestManager_GetFilteredAgents_MachineFilter(t *testing.T) {
	m := NewManager()
	m.ApplySnapshot(buildSnapshot())

	agents := m.GetFilteredAgents("local", "")
	if len(agents) != 5 {
		t.Fatalf("local: expected 5 agents, got %d", len(agents))
	}
	for _, a := range agents {
		if a.ConnName != "local" {
			t.Errorf("local filter: got agent from %s", a.ConnName)
		}
	}
}

func TestManager_GetFilteredAgents_SpaceFilter(t *testing.T) {
	m := NewManager()
	m.ApplySnapshot(buildSnapshot())

	// Space filter: backend has 5 agents on dev-server
	agents := m.GetFilteredAgents("", "backend")
	if len(agents) != 5 {
		t.Fatalf("backend: expected 5 agents, got %d", len(agents))
	}
	for _, a := range agents {
		if a.WsLabel != "backend" {
			t.Errorf("backend filter: got agent from %s", a.WsLabel)
		}
	}
}

func TestManager_GetFilteredAgents_Empty(t *testing.T) {
	m := NewManager()
	agents := m.GetFilteredAgents("", "")
	if len(agents) != 0 {
		t.Errorf("empty manager: expected 0 agents, got %d", len(agents))
	}
}

func TestManager_GetMachines(t *testing.T) {
	m := NewManager()
	m.ApplySnapshot(buildSnapshot())

	machines := m.GetMachines()
	if len(machines) != 3 {
		t.Fatalf("expected 3 machines, got %d", len(machines))
	}
	if machines[0].Name != "local" {
		t.Errorf("first machine: got %s, want local", machines[0].Name)
	}
	if machines[1].Name != "dev-server" {
		t.Errorf("second machine: got %s, want dev-server", machines[1].Name)
	}
}

func TestManager_GetAllSpaces(t *testing.T) {
	m := NewManager()
	m.ApplySnapshot(buildSnapshot())

	spaces := m.GetAllSpaces()
	// 5 unique workspaces: main-proj, web-app, backend, infra, monitoring
	if len(spaces) != 5 {
		t.Fatalf("expected 5 spaces, got %d", len(spaces))
	}
	labels := make(map[string]bool)
	for _, s := range spaces {
		labels[s.Label] = true
	}
	for _, label := range []string{"main-proj", "web-app", "backend", "infra", "monitoring"} {
		if !labels[label] {
			t.Errorf("missing space: %s", label)
		}
	}
}

func TestManager_ComputeStats(t *testing.T) {
	m := NewManager()
	m.ApplySnapshot(buildSnapshot())

	stats := m.ComputeStats()
	want := protocol.AgentStats{Done: 2, Idle: 2, Working: 4, Blocked: 3, Unknown: 1}
	if stats != want {
		t.Errorf("stats: got %+v, want %+v", stats, want)
	}
}

func TestManager_K11Toggle(t *testing.T) {
	m := NewManager()
	m.ApplySnapshot(buildSnapshot())

	// Default: not filtered
	if m.IsK11Filtered() {
		t.Error("should not be filtered by default")
	}

	m.SetK11Toggle(true)
	m.ToggleK11Filter()

	if !m.IsK11Filtered() {
		t.Error("should be filtered after toggle")
	}

	agents := m.GetFilteredAgents("", "")
	// 12 total: 2 done + 2 idle + 4 working + 3 blocked + 1 unknown
	// K11 filters idle(2) + unknown(1) → 9 remaining, truncated to 10 = 9
	if len(agents) != 9 {
		t.Fatalf("K11 filtered: expected 9 agents, got %d", len(agents))
	}
	for _, a := range agents {
		if a.Status == protocol.StatusIdle || a.Status == protocol.StatusUnknown {
			t.Errorf("K11 should filter idle/unknown, got %s with status %s", a.PaneID, a.Status)
		}
	}

	// Toggle off
	m.ToggleK11Filter()
	if m.IsK11Filtered() {
		t.Error("should not be filtered after second toggle")
	}
}

func TestManager_K11Toggle_WithMachineFilter(t *testing.T) {
	m := NewManager()
	m.ApplySnapshot(buildSnapshot())
	m.SetK11Toggle(true)
	m.ToggleK11Filter()

	// dev-server: 5 agents (1 idle, 1 done, 1 working, 1 blocked, 1 unknown)
	// K11: filters idle + unknown → 3 remaining
	agents := m.GetFilteredAgents("dev-server", "")
	if len(agents) != 3 {
		t.Fatalf("dev-server + K11: expected 3 agents, got %d", len(agents))
	}
}

func TestManager_K11Toggle_WithSpaceFilter(t *testing.T) {
	m := NewManager()
	m.ApplySnapshot(buildSnapshot())
	m.SetK11Toggle(true)
	m.ToggleK11Filter()

	// main-proj: 3 agents (1 working, 1 blocked, 1 idle)
	// K11: filters idle → 2 remaining
	agents := m.GetFilteredAgents("", "main-proj")
	if len(agents) != 2 {
		t.Fatalf("main-proj + K11: expected 2 agents, got %d", len(agents))
	}
}

func TestManager_FormatAgentDuration(t *testing.T) {
	m := NewManager()
	m.ApplySnapshot(buildSnapshot())

	// Fresh agent should show 0m
	dur := m.FormatAgentDuration("local", "p1")
	if dur != "0m" {
		t.Errorf("fresh agent: got %s, want 0m", dur)
	}

	// Manually advance clock
	m.statusSince["local|p1"] = time.Now().Add(-5 * time.Minute)
	dur = m.FormatAgentDuration("local", "p1")
	if dur != "5m" {
		t.Errorf("5m ago: got %s, want 5m", dur)
	}

	m.statusSince["local|p1"] = time.Now().Add(-90 * time.Minute)
	dur = m.FormatAgentDuration("local", "p1")
	if dur != "1h30m" {
		t.Errorf("90m ago: got %s, want 1h30m", dur)
	}

	m.statusSince["local|p1"] = time.Now().Add(-1500 * time.Minute)
	dur = m.FormatAgentDuration("local", "p1")
	if dur != "1d1h" {
		t.Errorf("1500m ago: got %s, want 1d1h", dur)
	}
}

func TestManager_DurationStatusChangeResets(t *testing.T) {
	m := NewManager()
	snap := buildSnapshot()
	m.ApplySnapshot(snap)

	// Wind p1's timer
	m.statusSince["local|p1"] = time.Now().Add(-30 * time.Minute)
	if m.FormatAgentDuration("local", "p1") != "30m" {
		t.Fatal("sanity: expected 30m")
	}

	// Change p1 status from working → done (deep copy to avoid aliasing)
	modified := protocol.FleetSnapshot{
		Version:   snap.Version,
		Seq:       snap.Seq + 1,
		UpdatedAt: snap.UpdatedAt,
		Machines:  snap.Machines,
		Stats:     snap.Stats,
	}
	modified.Agents = make([]protocol.AgentState, len(snap.Agents))
	copy(modified.Agents, snap.Agents)
	for i := range modified.Agents {
		if modified.Agents[i].PaneID == "p1" {
			modified.Agents[i].Status = protocol.StatusDone
			break
		}
	}
	m.ApplySnapshot(&modified)

	dur := m.FormatAgentDuration("local", "p1")
	if dur != "0m" {
		t.Errorf("after status change: got %s, want 0m", dur)
	}
}

func TestManager_HealthTracking(t *testing.T) {
	m := NewManager()

	// Fresh manager → offline
	if h := m.CheckHealth(); h != HealthOffline {
		t.Errorf("fresh: expected HealthOffline, got %v", h)
	}

	// Heartbeat → connected
	m.MarkHeartbeat(time.Now())
	if h := m.Health(); h != HealthConnected {
		t.Errorf("after heartbeat: expected HealthConnected, got %v", h)
	}

	// Stale heartbeat → offline
	m.MarkHeartbeat(time.Now().Add(-6 * time.Second))
	if h := m.CheckHealth(); h != HealthOffline {
		t.Errorf("stale: expected HealthOffline, got %v", h)
	}
}

func TestManager_SysStats(t *testing.T) {
	m := NewManager()

	cpu, mem := m.GetSysStats()
	if cpu != 0 || mem != 0 {
		t.Errorf("default stats: got cpu=%.1f mem=%.1f, want 0", cpu, mem)
	}

	m.SetSysStats(45.5, 72.3)
	cpu, mem = m.GetSysStats()
	if cpu != 45.5 || mem != 72.3 {
		t.Errorf("after set: got cpu=%.1f mem=%.1f", cpu, mem)
	}
}

func TestManager_TruncateTo10(t *testing.T) {
	m := NewManager()

	// Build snapshot with 15 agents
	agents := make([]protocol.AgentState, 15)
	for i := 0; i < 15; i++ {
		agents[i] = protocol.AgentState{
			ID: "m|px" + string(rune('a'+i)), Machine: "m", Agent: "pi",
			Status: protocol.StatusIdle, PaneID: "px" + string(rune('a'+i)),
			Workspace: "ws", WorkspaceID: "ws",
		}
	}
	snap := &protocol.FleetSnapshot{
		Version: 1, Machines: []protocol.MachineInfo{{Name: "m", Abbr: "M", Color: "#000"}},
		Agents: agents,
	}
	m.ApplySnapshot(snap)

	result := m.GetFilteredAgents("", "")
	if len(result) != 10 {
		t.Errorf("expected 10 (truncated), got %d", len(result))
	}
}

func TestManager_DurationStaleAgentRemoved(t *testing.T) {
	m := NewManager()
	snap := buildSnapshot()
	m.ApplySnapshot(snap)

	// Wind p1's timer
	m.statusSince["local|p1"] = time.Now().Add(-10 * time.Minute)
	if m.FormatAgentDuration("local", "p1") != "10m" {
		t.Fatal("sanity: expected 10m")
	}

	// Remove p1 (deep copy to avoid mutating shared array)
	modified := protocol.FleetSnapshot{
		Version:   snap.Version,
		Seq:       snap.Seq + 1,
		UpdatedAt: snap.UpdatedAt,
		Machines:  snap.Machines,
		Stats:     snap.Stats,
	}
	modified.Agents = make([]protocol.AgentState, len(snap.Agents))
	copy(modified.Agents, snap.Agents)
	for i := range modified.Agents {
		if modified.Agents[i].PaneID == "p1" {
			modified.Agents = append(modified.Agents[:i], modified.Agents[i+1:]...)
			break
		}
	}
	m.ApplySnapshot(&modified)

	dur := m.FormatAgentDuration("local", "p1")
	if dur != "0m" {
		t.Errorf("after removal: got %s, want 0m", dur)
	}
}

func TestFormatDuration_EdgeCases(t *testing.T) {
	tests := []struct {
		minutes int
		want    string
	}{
		{0, "0m"},
		{1, "1m"},
		{30, "30m"},
		{59, "59m"},
		{60, "1h00m"},
		{61, "1h01m"},
		{90, "1h30m"},
		{120, "2h00m"},
		{1439, "23h59m"},
		{1440, "1d0h"},
		{1500, "1d1h"},
		{2880, "2d0h"},
	}
	for _, tt := range tests {
		d := time.Duration(tt.minutes) * time.Minute
		got := formatDuration(d)
		if got != tt.want {
			t.Errorf("formatDuration(%dm) = %q, want %q", tt.minutes, got, tt.want)
		}
	}
}
