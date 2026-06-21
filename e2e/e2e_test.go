// Package e2e contains end-to-end tests for the herdr-agentview three-process
// architecture. Uses testcontainers-go for the NATS message bus and tests
// the protocol-level data flow (JSON roundtrip, heartbeat, snapshot semantics).
//
// Run with: go test -v -count=1 ./...
package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/herdr-deck/herdrdeck/protocol"

	"github.com/nats-io/nats.go"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// startNATS starts a NATS container and returns the client URL + cleanup.
func startNATS(t *testing.T) (string, func()) {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "nats:2.10-alpine",
		ExposedPorts: []string{"4222/tcp"},
		Cmd:          []string{"-js"},
		WaitingFor:   wait.ForLog("Server is ready"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start NATS container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("failed to get NATS host: %v", err)
	}

	port, err := container.MappedPort(ctx, "4222")
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("failed to get NATS port: %v", err)
	}

	url := "nats://" + host + ":" + port.Port()

	cleanup := func() {
		container.Terminate(ctx)
	}

	return url, cleanup
}

// waitForNATS polls until a NATS connection succeeds.
func waitForNATS(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		nc, err := nats.Connect(url, nats.Timeout(1*time.Second))
		if err == nil {
			nc.Close()
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("NATS not ready at %s within %v", url, timeout)
}

// buildTestSnapshot creates a known FleetSnapshot for testing.
func buildTestSnapshot() protocol.FleetSnapshot {
	return protocol.FleetSnapshot{
		Version:   protocol.SchemaVersion,
		Seq:       1,
		UpdatedAt: "2026-06-21T10:00:00Z",
		Machines: []protocol.MachineInfo{
			{Name: "local", Abbr: "LCL", Color: "#4ADE80"},
			{Name: "dev-server", Abbr: "DEV", Color: "#60A5FA"},
		},
		Agents: []protocol.AgentState{
			{
				ID: "local|p1", Machine: "local", Agent: "pi", Name: "review",
				Status: protocol.StatusWorking, Focused: true,
				Workspace: "main-proj", WorkspaceID: "ws-1",
				PaneID: "p1", UpdatedAt: "2026-06-21T10:00:00Z",
			},
			{
				ID: "local|p2", Machine: "local", Agent: "cursor",
				Status: protocol.StatusBlocked,
				Workspace: "main-proj", WorkspaceID: "ws-1",
				PaneID: "p2", UpdatedAt: "2026-06-21T10:00:00Z",
			},
			{
				ID: "local|p3", Machine: "local", Agent: "pi", Name: "idle",
				Status: protocol.StatusIdle,
				Workspace: "main-proj", WorkspaceID: "ws-1",
				PaneID: "p3", UpdatedAt: "2026-06-21T10:00:00Z",
			},
			{
				ID: "dev-server|p4", Machine: "dev-server", Agent: "devin", Name: "test-fail",
				Status: protocol.StatusBlocked,
				Workspace: "backend", WorkspaceID: "ws-2",
				PaneID: "p4", UpdatedAt: "2026-06-21T10:00:00Z",
			},
		},
		Stats: protocol.AgentStats{
			Blocked: 2, Working: 1, Idle: 1,
		},
	}
}

// TestE2E_SnapshotRoundtrip verifies FleetSnapshot JSON roundtrip over NATS.
func TestE2E_SnapshotRoundtrip(t *testing.T) {
	natsURL, cleanup := startNATS(t)
	defer cleanup()
	waitForNATS(t, natsURL, 10*time.Second)

	original := buildTestSnapshot()

	// Connect publisher
	pubNC, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("publisher connect: %v", err)
	}
	defer pubNC.Close()

	// Connect subscriber (with buffered channel)
	subNC, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("subscriber connect: %v", err)
	}
	defer subNC.Close()

	snapshotCh := make(chan protocol.FleetSnapshot, 1)
	sub, err := subNC.Subscribe(protocol.SubjectSnapshot, func(msg *nats.Msg) {
		var snap protocol.FleetSnapshot
		if err := json.Unmarshal(msg.Data, &snap); err != nil {
			return
		}
		select {
		case snapshotCh <- snap:
		default:
		}
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()
	if err := subNC.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// Publish
	data, _ := json.Marshal(original)
	if err := pubNC.Publish(protocol.SubjectSnapshot, data); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Receive
	select {
	case received := <-snapshotCh:
		if received.Seq != original.Seq {
			t.Errorf("seq: got %d, want %d", received.Seq, original.Seq)
		}
		if received.Version != protocol.SchemaVersion {
			t.Errorf("version: got %d, want %d", received.Version, protocol.SchemaVersion)
		}
		if len(received.Agents) != len(original.Agents) {
			t.Fatalf("agents: got %d, want %d", len(received.Agents), len(original.Agents))
		}
		if len(received.Machines) != len(original.Machines) {
			t.Fatalf("machines: got %d, want %d", len(received.Machines), len(original.Machines))
		}
		if received.Stats != original.Stats {
			t.Errorf("stats mismatch: got %+v, want %+v", received.Stats, original.Stats)
		}

		// Verify agent fields
		for i, a := range received.Agents {
			o := original.Agents[i]
			if a.Agent != o.Agent {
				t.Errorf("agent[%d].Agent: got %q, want %q", i, a.Agent, o.Agent)
			}
			if a.Status != o.Status {
				t.Errorf("agent[%d].Status: got %q, want %q", i, a.Status, o.Status)
			}
			if a.Machine != o.Machine {
				t.Errorf("agent[%d].Machine: got %q, want %q", i, a.Machine, o.Machine)
			}
		}

	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for FleetSnapshot")
	}
}

// TestE2E_Heartbeat verifies heartbeat message flow over NATS.
func TestE2E_Heartbeat(t *testing.T) {
	natsURL, cleanup := startNATS(t)
	defer cleanup()
	waitForNATS(t, natsURL, 10*time.Second)

	pubNC, _ := nats.Connect(natsURL)
	defer pubNC.Close()

	subNC, _ := nats.Connect(natsURL)
	defer subNC.Close()

	heartbeatCh := make(chan time.Time, 1)
	if _, err := subNC.Subscribe(protocol.SubjectHeartbeat, func(msg *nats.Msg) {
		select {
		case heartbeatCh <- time.Now():
		default:
		}
	}); err != nil {
		t.Fatalf("subscribe heartbeat: %v", err)
	}
	if err := subNC.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// Publish heartbeat
	hb := map[string]string{"ts": time.Now().UTC().Format(time.RFC3339)}
	data, _ := json.Marshal(hb)
	pubNC.Publish(protocol.SubjectHeartbeat, data)

	select {
	case <-heartbeatCh:
		// heartbeat received
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for heartbeat")
	}

	// Verify subject constants match what NATS uses
	if protocol.SubjectHeartbeat != "herdr.v1.collector.heartbeat" {
		t.Errorf("unexpected SubjectHeartbeat: %q", protocol.SubjectHeartbeat)
	}
	if protocol.SubjectSnapshot != "herdr.v1.snapshot.full" {
		t.Errorf("unexpected SubjectSnapshot: %q", protocol.SubjectSnapshot)
	}
}

// TestE2E_MultipleSnapshots verifies sequential snapshot handling.
func TestE2E_MultipleSnapshots(t *testing.T) {
	natsURL, cleanup := startNATS(t)
	defer cleanup()
	waitForNATS(t, natsURL, 10*time.Second)

	pubNC, _ := nats.Connect(natsURL)
	defer pubNC.Close()

	subNC, _ := nats.Connect(natsURL)
	defer subNC.Close()

	snapshotCh := make(chan protocol.FleetSnapshot, 2)
	if _, err := subNC.Subscribe(protocol.SubjectSnapshot, func(msg *nats.Msg) {
		var snap protocol.FleetSnapshot
		if json.Unmarshal(msg.Data, &snap) == nil {
			snapshotCh <- snap
		}
	}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := subNC.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// Publish seq 1
	snap1 := buildTestSnapshot()
	snap1.Seq = 1
	data1, _ := json.Marshal(snap1)
	pubNC.Publish(protocol.SubjectSnapshot, data1)

	// Publish seq 2 (modified)
	snap2 := buildTestSnapshot()
	snap2.Seq = 2
	snap2.Agents = snap2.Agents[:2] // only 2 agents
	snap2.Stats = protocol.AgentStats{Blocked: 1, Working: 1}
	data2, _ := json.Marshal(snap2)
	pubNC.Publish(protocol.SubjectSnapshot, data2)

	// Receive both in order
	for i, want := range []struct {
		seq     uint64
		agents  int
		blocked int
	}{
		{1, 4, 2},
		{2, 2, 1},
	} {
		select {
		case got := <-snapshotCh:
			if got.Seq != want.seq {
				t.Errorf("snapshot[%d].seq: got %d, want %d", i, got.Seq, want.seq)
			}
			if len(got.Agents) != want.agents {
				t.Errorf("snapshot[%d].agents: got %d, want %d", i, len(got.Agents), want.agents)
			}
			if got.Stats.Blocked != want.blocked {
				t.Errorf("snapshot[%d].blocked: got %d, want %d", i, got.Stats.Blocked, want.blocked)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for snapshot %d", i)
		}
	}
}

// TestE2E_EmptySnapshot verifies empty snapshot handling.
func TestE2E_EmptySnapshot(t *testing.T) {
	natsURL, cleanup := startNATS(t)
	defer cleanup()
	waitForNATS(t, natsURL, 10*time.Second)

	pubNC, _ := nats.Connect(natsURL)
	defer pubNC.Close()

	subNC, _ := nats.Connect(natsURL)
	defer subNC.Close()

	snapshotCh := make(chan protocol.FleetSnapshot, 1)
	if _, err := subNC.Subscribe(protocol.SubjectSnapshot, func(msg *nats.Msg) {
		var snap protocol.FleetSnapshot
		if json.Unmarshal(msg.Data, &snap) == nil {
			snapshotCh <- snap
		}
	}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	// Ensure subscription is registered on the server before publishing.
	if err := subNC.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	empty := protocol.FleetSnapshot{Version: protocol.SchemaVersion, Seq: 0}
	data, _ := json.Marshal(empty)
	pubNC.Publish(protocol.SubjectSnapshot, data)

	select {
	case got := <-snapshotCh:
		if len(got.Agents) != 0 {
			t.Errorf("empty snapshot: expected 0 agents, got %d", len(got.Agents))
		}
		if got.Stats != (protocol.AgentStats{}) {
			t.Errorf("empty snapshot: expected zero stats, got %+v", got.Stats)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for empty snapshot")
	}
}
