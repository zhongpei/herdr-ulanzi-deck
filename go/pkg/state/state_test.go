package state

import (
	"testing"
	"time"

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

	// Space filter uses Label (workspace name), not wsID
	agents := m.GetFilteredAgents("local", "main-proj")
	if len(agents) != 3 {
		t.Fatalf("expected 3 agents for 'main-proj', got %d", len(agents))
	}
	for _, a := range agents {
		if a.WsLabel != "main-proj" {
			t.Errorf("expected WsLabel 'main-proj', got '%s'", a.WsLabel)
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

// ─── Duration tracking ──────────────────────────────────────

func TestFormatDuration_Values(t *testing.T) {
	tests := []struct {
		minutes int
		want    string
	}{
		{0, "0m"},
		{1, "1m"},
		{45, "45m"},
		{59, "59m"},
		{60, "1h00m"},
		{90, "1h30m"},
		{1439, "23h59m"},
		{1440, "1d0h"},
		{1500, "1d1h"},
		{3000, "2d2h"},
	}
	for _, tt := range tests {
		d := time.Duration(tt.minutes) * time.Minute
		got := formatDuration(d)
		if got != tt.want {
			t.Errorf("formatDuration(%dm) = %q, want %q", tt.minutes, got, tt.want)
		}
	}
}

func TestFormatAgentDuration_Shows0mImmediately(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())

	// Agent p1 is in workspace ws-1 which belongs to "local" machine
	dur := m.FormatAgentDuration("local", "p1")
	if dur != "0m" {
		t.Errorf("fresh agent should show 0m, got %s", dur)
	}

	// Unknown agent returns "0m" too
	dur = m.FormatAgentDuration("nowhere", "nope")
	if dur != "0m" {
		t.Errorf("unknown agent should show 0m, got %s", dur)
	}
}

func TestFormatAgentDuration_Accumulates(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())

	// Manually wind the clock forward for agent p1
	key := "local|p1"
	m.statusSince[key] = time.Now().Add(-5 * time.Minute)

	dur := m.FormatAgentDuration("local", "p1")
	if dur != "5m" {
		t.Errorf("expected 5m, got %s", dur)
	}

	// Winding to 1 hour
	m.statusSince[key] = time.Now().Add(-90 * time.Minute)
	dur = m.FormatAgentDuration("local", "p1")
	if dur != "1h30m" {
		t.Errorf("expected 1h30m, got %s", dur)
	}

	// Winding to 2 days
	m.statusSince[key] = time.Now().Add(-50 * time.Hour)
	dur = m.FormatAgentDuration("local", "p1")
	if dur != "2d2h" {
		t.Errorf("expected 2d2h, got %s", dur)
	}

	// Agent p2 in different machine (dev-server)
	key2 := "dev-server|p6"
	m.statusSince[key2] = time.Now().Add(-10 * time.Minute)
	dur = m.FormatAgentDuration("dev-server", "p6")
	if dur != "10m" {
		t.Errorf("expected 10m, got %s", dur)
	}
}

func TestAgentDuration_StatusChangeResetsTimer(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())

	// Wind p1's timer forward
	key := "local|p1"
	m.statusSince[key] = time.Now().Add(-30 * time.Minute)
	if m.FormatAgentDuration("local", "p1") != "30m" {
		t.Fatal("sanity check: expected 30m")
	}

	// Simulate a status change: p1 goes from working → done
	data := buildTestData()
	for i := range data {
		if data[i].ConnName == "local" {
			for j := range data[i].Agents {
				if data[i].Agents[j].PaneID == "p1" {
					data[i].Agents[j].AgentStatus = types.StatusDone
					break
				}
			}
		}
	}
	m.Init(data)

	dur := m.FormatAgentDuration("local", "p1")
	if dur != "0m" {
		t.Errorf("expected 0m after status change, got %s", dur)
	}

	// Unchanged agent should keep its timer
	dur = m.FormatAgentDuration("local", "p2") // still blocked
	if dur != "0m" {
		t.Errorf("expected 0m for unchanged agent p2, got %s", dur)
	}
}

func TestAgentDuration_StaleAgentRemoved(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())

	// Wind p1's timer
	key := "local|p1"
	m.statusSince[key] = time.Now().Add(-10 * time.Minute)
	if m.FormatAgentDuration("local", "p1") != "10m" {
		t.Fatal("sanity check: expected 10m")
	}

	// Simulate p1 being removed (agent no longer in any workspace)
	data := buildTestData()
	for i := range data {
		if data[i].ConnName == "local" {
			for j := range data[i].Agents {
				if data[i].Agents[j].PaneID == "p1" {
					data[i].Agents = append(data[i].Agents[:j], data[i].Agents[j+1:]...)
					break
				}
			}
		}
	}
	m.Init(data)

	// p1's timer should be gone, FormatAgentDuration returns "0m" (not-found)
	dur := m.FormatAgentDuration("local", "p1")
	if dur != "0m" {
		t.Errorf("expected 0m for removed agent, got %s", dur)
	}
}

func TestAgentDuration_CrossMachineKeys(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())

	// Same pane ID on different machines should have independent timers
	// ws-1 (local) has no pane called "dup", ws-3 (dev-server) has no "dup" either
	// So this tests that keys don't collide
	keyLocal := "local|dup"
	keyDev := "dev-server|dup"
	m.statusSince[keyLocal] = time.Now().Add(-5 * time.Minute)
	m.statusSince[keyDev] = time.Now().Add(-10 * time.Minute)

	if m.FormatAgentDuration("local", "dup") != "5m" {
		t.Errorf("expected 5m for local|dup")
	}
	if m.FormatAgentDuration("dev-server", "dup") != "10m" {
		t.Errorf("expected 10m for dev-server|dup")
	}
}

// ─── GetAllSpaces ──────────────────────────────────────────

func TestGetAllSpaces(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())

	spaces := m.GetAllSpaces()
	if len(spaces) != 3 {
		t.Fatalf("expected 3 unique spaces, got %d", len(spaces))
	}
	// Order should match first occurrence in unified
	if spaces[0].Label != "main-proj" {
		t.Errorf("first space: expected label 'main-proj', got '%s'", spaces[0].Label)
	}
	if spaces[1].Label != "web-app" {
		t.Errorf("second space: expected label 'web-app', got '%s'", spaces[1].Label)
	}
	if spaces[2].Label != "backend" {
		t.Errorf("third space: expected label 'backend', got '%s'", spaces[2].Label)
	}
}

func TestGetAllSpaces_Dedup(t *testing.T) {
	m := NewManager()
	// Two workspaces with same label on different machines (different wsID)
	data := []types.UnifiedWorkspace{
		{ConnName: "local", ConnAbbr: "LCL", WorkspaceID: "uuid-a", Label: "shared"},
		{ConnName: "remote", ConnAbbr: "REM", WorkspaceID: "uuid-b", Label: "shared"},
		{ConnName: "remote", ConnAbbr: "REM", WorkspaceID: "uuid-c", Label: "other"},
	}
	m.Init(data)

	spaces := m.GetAllSpaces()
	if len(spaces) != 2 {
		t.Fatalf("expected 2 unique spaces (dedup by label), got %d", len(spaces))
	}
	// First occurrence of label "shared" (local) should be kept
	if spaces[0].Label != "shared" {
		t.Errorf("expected first space label 'shared', got '%s'", spaces[0].Label)
	}
}

// ─── Global Space Filter ───────────────────────────────────

func TestGetFilteredAgents_GlobalSpaceFilter_IgnoresMachine(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())

	// Space filter by label ignores machine arg
	agents := m.GetFilteredAgents("local", "main-proj")
	// main-proj only exists on local machine, so same 3 agents
	if len(agents) != 3 {
		t.Fatalf("expected 3 agents for 'main-proj', got %d", len(agents))
	}
	for _, a := range agents {
		if a.WsLabel != "main-proj" {
			t.Errorf("expected WsLabel 'main-proj', got '%s'", a.WsLabel)
		}
	}
}

// buildTestDataWithSharedWS returns data where the same label exists on multiple machines.
func buildTestDataWithSharedWS() []types.UnifiedWorkspace {
	return []types.UnifiedWorkspace{
		{
			ConnName: "local", ConnAbbr: "LCL", ConnAbbrColor: "#4ADE80",
			WorkspaceID: "ws-shared", Label: "shared-proj",
			Agents: []types.AgentInfo{
				{Agent: "pi", Name: "local-a", AgentStatus: types.StatusWorking, PaneID: "p1", WorkspaceID: "ws-shared"},
				{Agent: "cursor", Name: "local-b", AgentStatus: types.StatusBlocked, PaneID: "p2", WorkspaceID: "ws-shared"},
			},
		},
		{
			ConnName: "dev-server", ConnAbbr: "DEV", ConnAbbrColor: "#60A5FA",
			WorkspaceID: "ws-shared", Label: "shared-proj",
			Agents: []types.AgentInfo{
				{Agent: "devin", Name: "remote-a", AgentStatus: types.StatusDone, PaneID: "p3", WorkspaceID: "ws-shared"},
			},
		},
		{
			ConnName: "dev-server", ConnAbbr: "DEV", ConnAbbrColor: "#60A5FA",
			WorkspaceID: "ws-other", Label: "other",
			Agents: []types.AgentInfo{
				{Agent: "gemini", Name: "other-a", AgentStatus: types.StatusIdle, PaneID: "p4", WorkspaceID: "ws-other"},
			},
		},
	}
}

func TestGetFilteredAgents_GlobalSpaceFilter_AcrossMachines(t *testing.T) {
	m := NewManager()
	m.Init(buildTestDataWithSharedWS())

	// Space filter by label returns agents from ALL machines with that label
	agents := m.GetFilteredAgents("", "shared-proj")
	if len(agents) != 3 {
		t.Fatalf("expected 3 agents from ws-shared across both machines, got %d", len(agents))
	}
	// Verify both machines represented
	seenLocal := false
	seenDev := false
	for _, a := range agents {
		if a.ConnName == "local" {
			seenLocal = true
		}
		if a.ConnName == "dev-server" {
			seenDev = true
		}
	}
	if !seenLocal {
		t.Error("expected agent from local machine")
	}
	if !seenDev {
		t.Error("expected agent from dev-server machine")
	}
}

func TestGetFilteredAgents_GlobalSpaceFilter_WithMachineArgIgnored(t *testing.T) {
	m := NewManager()
	m.Init(buildTestDataWithSharedWS())

	// When space filter is active, the machine argument is ignored
	// Both of these should return the same 3 agents from shared-proj
	agentsWithMachine := m.GetFilteredAgents("local", "shared-proj")
	agentsWithout := m.GetFilteredAgents("", "shared-proj")
	if len(agentsWithMachine) != len(agentsWithout) {
		t.Errorf("machine arg should be ignored: got %d vs %d", len(agentsWithMachine), len(agentsWithout))
	}
}

// ─── K11 Mode Status Filter ────────────────────────────────

func TestK11Toggle_FiltersIdleAndUnknown(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())
	m.SetK11Toggle(true)
	m.ToggleK11Filter()

	agents := m.GetFilteredAgents("", "")
	// buildTestData stats: blocked=2, done=2, working=3, idle=2, unknown=1
	// filtered removes idle(2) + unknown(1) → 7 remaining
	if len(agents) != 7 {
		t.Fatalf("expected 7 agents (without idle+unknown), got %d", len(agents))
	}
	for _, a := range agents {
		if a.AgentStatus == types.StatusIdle || a.AgentStatus == types.StatusUnknown {
			t.Errorf("K11 filtered should remove %s, got agent %s", a.AgentStatus, a.PaneID)
		}
	}
}

func TestK11Toggle_Filters_WithMachineFilter(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())
	m.SetK11Toggle(true)
	m.ToggleK11Filter()

	// local machine has: ws-1 (1 working, 1 blocked, 1 idle) + ws-2 (1 done, 1 working)
	// idle filtered out → 4 remaining
	agents := m.GetFilteredAgents("local", "")
	if len(agents) != 4 {
		t.Fatalf("expected 4 local agents (without idle), got %d", len(agents))
	}
	for _, a := range agents {
		if a.ConnName != "local" {
			t.Errorf("expected local only, got %s", a.ConnName)
		}
		if a.AgentStatus == types.StatusIdle {
			t.Errorf("K11 filtered should remove idle, got %s", a.PaneID)
		}
	}
}

func TestK11Toggle_Filters_WithSpaceFilter(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())
	m.SetK11Toggle(true)
	m.ToggleK11Filter()

	// main-proj has: 1 working, 1 blocked, 1 idle
	// idle filtered out → 2 remaining
	agents := m.GetFilteredAgents("", "main-proj")
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents in ws-1 (without idle), got %d", len(agents))
	}
	for _, a := range agents {
		if a.AgentStatus == types.StatusIdle {
			t.Errorf("K11 filtered should remove idle, got %s", a.PaneID)
		}
	}
}

func TestK11Toggle_DefaultNotFiltered(t *testing.T) {
	m := NewManager()
	m.Init(buildTestData())
	// toggle enabled but not activated → no filtering
	m.SetK11Toggle(true)

	agents := m.GetFilteredAgents("", "")
	if len(agents) != 10 {
		t.Fatalf("expected all 10 agents (toggle not activated), got %d", len(agents))
	}
}

func TestK11Toggle_ToggleOnOff(t *testing.T) {
	m := NewManager()
	m.SetK11Toggle(true)

	if m.IsK11Filtered() {
		t.Error("expected not filtered initially")
	}
	m.ToggleK11Filter()
	if !m.IsK11Filtered() {
		t.Error("expected filtered after toggle")
	}
	m.ToggleK11Filter()
	if m.IsK11Filtered() {
		t.Error("expected not filtered after second toggle")
	}
}

func TestK11Toggle_DisabledByDefault(t *testing.T) {
	m := NewManager()
	// k11Toggle defaults to false — no filtering unless ToggleK11Filter is called
	if m.IsK11Filtered() {
		t.Error("expected not filtered initially")
	}
	// calling ToggleK11Filter still works mechanically regardless of k11Toggle
	m.ToggleK11Filter()
	if !m.IsK11Filtered() {
		t.Error("expected filtered after ToggleK11Filter")
	}
	// The k11Toggle flag only controls whether the UI calls ToggleK11Filter
}
