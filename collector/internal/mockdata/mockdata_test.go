package mockdata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/herdr-deck/herdrdeck/collector/internal/bridge"
)

func TestLoad_ValidFile(t *testing.T) {
	src, err := Load(filepath.Join("testdata", "three-phase.json"), DefaultDuration)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(src.phases) != 3 {
		t.Fatalf("expected 3 phases, got %d", len(src.phases))
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("nonexistent.json", DefaultDuration)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_ZeroDuration(t *testing.T) {
	src, err := Load(filepath.Join("testdata", "three-phase.json"), 0)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if src.perPhase != DefaultDuration {
		t.Errorf("expected default duration %v, got %v", DefaultDuration, src.perPhase)
	}
}

func TestLoad_EmptyPhases(t *testing.T) {
	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty.json")
	if err := os.WriteFile(emptyFile, []byte("[]"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(emptyFile, DefaultDuration)
	if err == nil || !strings.Contains(err.Error(), "zero phases") {
		t.Fatalf("expected 'zero phases' error, got: %v", err)
	}
}

func TestPhaseCycling(t *testing.T) {
	src, err := Load(filepath.Join("testdata", "three-phase.json"), 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Phase 0
	r0 := src.FetchAll()
	if len(r0) != 3 {
		t.Fatalf("phase 0: expected 3 connections, got %d", len(r0))
	}
	if r0[0].ConnName != "local" {
		t.Errorf("phase 0 conn 0: expected 'local', got %q", r0[0].ConnName)
	}
	// local should have 2 workspaces in phase 0
	if len(r0[0].Workspaces) != 2 {
		t.Errorf("phase 0: local expected 2 workspaces, got %d", len(r0[0].Workspaces))
	}

	// Phase 1 (wait for timer to advance)
	time.Sleep(60 * time.Millisecond)
	r1 := src.FetchAll()
	if len(r1) != 3 {
		t.Fatalf("phase 1: expected 3 connections, got %d", len(r1))
	}

	// Phase 2
	time.Sleep(60 * time.Millisecond)
	r2 := src.FetchAll()
	if len(r2) != 3 {
		t.Fatalf("phase 2: expected 3 connections, got %d", len(r2))
	}

	// Wrap around back to phase 0
	time.Sleep(60 * time.Millisecond)
	r0b := src.FetchAll()
	if len(r0b) != 3 {
		t.Fatalf("wrap: expected 3 connections, got %d", len(r0b))
	}
	if len(r0b[0].Workspaces) != 2 {
		t.Errorf("phase 0 wrap: local expected 2 workspaces, got %d", len(r0b[0].Workspaces))
	}
}

func TestFetchAll_HasAllMachines(t *testing.T) {
	src, err := Load(filepath.Join("testdata", "three-phase.json"), DefaultDuration)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	results := src.FetchAll()

	machines := make(map[string]bool)
	for _, r := range results {
		machines[r.ConnName] = true
	}
	for _, m := range []string{"local", "remote-server", "dev-vm"} {
		if !machines[m] {
			t.Errorf("missing machine %q", m)
		}
	}
}

func TestFetchAll_CoversAllStatuses(t *testing.T) {
	src, err := Load(filepath.Join("testdata", "three-phase.json"), DefaultDuration)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Collect all agent statuses across all phases
	seen := make(map[string]bool)
	for range src.phases {
		for _, r := range src.FetchAll() {
			for _, ws := range r.Workspaces {
				for _, a := range ws.Agents {
					seen[a.Status] = true
				}
			}
		}
		// Advance to next phase
		src.startTime = src.startTime.Add(-src.perPhase)
	}

	for _, s := range []string{"working", "idle", "blocked", "done", "unknown"} {
		if !seen[s] {
			t.Errorf("status %q not covered across all phases", s)
		}
	}
}

func TestFetchAll_NoErrorResults(t *testing.T) {
	src, err := Load(filepath.Join("testdata", "three-phase.json"), DefaultDuration)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	for range src.phases {
		results := src.FetchAll()
		for _, r := range results {
			if r.Err != nil {
				t.Errorf("unexpected error in result %q: %v", r.ConnName, r.Err)
			}
		}
		src.startTime = src.startTime.Add(-src.perPhase)
	}
}

func TestFetchAll_WorkspaceAgentCounts(t *testing.T) {
	src, err := Load(filepath.Join("testdata", "three-phase.json"), DefaultDuration)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	results := src.FetchAll()

	totalAgents := 0
	for _, r := range results {
		for _, ws := range r.Workspaces {
			totalAgents += len(ws.Agents)
			if len(ws.Agents) != ws.PaneCount {
				t.Errorf("workspace %q: PaneCount=%d != %d agents",
					ws.Label, ws.PaneCount, len(ws.Agents))
			}
			if len(ws.Agents) != ws.TabCount {
				t.Errorf("workspace %q: TabCount=%d != %d agents",
					ws.Label, ws.TabCount, len(ws.Agents))
			}
		}
	}
	if totalAgents == 0 {
		t.Error("no agents found in any workspace")
	}
}

func TestAggregateStatus(t *testing.T) {
	tests := []struct {
		statuses []string
		want     string
	}{
		{[]string{"working", "idle"}, "working"},
		{[]string{"idle", "done"}, "done"},
		{[]string{"blocked", "working"}, "blocked"},
		{[]string{"unknown"}, "unknown"},
		{[]string{"done", "done"}, "done"},
		{[]string{"idle", "unknown"}, "idle"},
	}
	for _, tt := range tests {
		agents := make([]bridge.RawAgent, len(tt.statuses))
		for i, s := range tt.statuses {
			agents[i] = bridge.RawAgent{Status: s}
		}
		got := aggregateStatus(agents)
		if got != tt.want {
			t.Errorf("aggregateStatus(%v) = %q, want %q", tt.statuses, got, tt.want)
		}
	}
}
