package subscriber

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/herdr-deck/herdrdeck/protocol"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// startEmbeddedNATS runs a NATS server on a random port for testing.
func startEmbeddedNATS(t *testing.T) (string, func()) {
	t.Helper()

	opts := &server.Options{
		Host:   "127.0.0.1",
		Port:   -1, // random port
		NoLog:  true,
		NoSigs: true,
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("new nats server: %v", err)
	}
	ns.Start()

	if !ns.ReadyForConnections(5 * time.Second) {
		ns.Shutdown()
		t.Fatal("nats server not ready")
	}

	url := ns.ClientURL()

	cleanup := func() {
		ns.Shutdown()
		ns.WaitForShutdown()
	}

	return url, cleanup
}

func TestSubscriber_NewAndClose(t *testing.T) {
	url, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	s, err := New(url)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Should connect and create channels
	if s.snapshotCh == nil {
		t.Error("snapshotCh should not be nil")
	}
	if s.heartbeatCh == nil {
		t.Error("heartbeatCh should not be nil")
	}

	s.Close()
}

func TestSubscriber_ReceivesSnapshot(t *testing.T) {
	url, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	s, err := New(url)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	// Publish a real snapshot via raw NATS
	pubNC, _ := nats.Connect(url)
	defer pubNC.Close()

	snap := protocol.FleetSnapshot{
		Version:   protocol.SchemaVersion,
		Seq:       42,
		UpdatedAt: "2026-06-21T10:00:00Z",
		Machines: []protocol.MachineInfo{
			{Name: "local", Abbr: "LCL", Color: "#4ADE80"},
		},
		Agents: []protocol.AgentState{
			{
				ID: "local|p1", Machine: "local", Agent: "pi", Name: "test",
				Status: protocol.StatusWorking, Focused: true,
				Workspace: "main-proj", WorkspaceID: "ws-1",
				PaneID: "p1", UpdatedAt: "2026-06-21T10:00:00Z",
			},
		},
		Stats: protocol.AgentStats{Working: 1},
	}

	data, _ := json.Marshal(snap)
	if err := pubNC.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if err := pubNC.Publish(protocol.SubjectSnapshot, data); err != nil {
		t.Fatalf("publish: %v", err)
	}
	pubNC.Flush()

	// Receive via subscriber
	select {
	case received := <-s.SnapshotCh():
		if received.Seq != 42 {
			t.Errorf("seq: got %d, want 42", received.Seq)
		}
		if len(received.Agents) != 1 {
			t.Fatalf("agents: got %d, want 1", len(received.Agents))
		}
		a := received.Agents[0]
		if a.Agent != "pi" {
			t.Errorf("agent: got %q, want pi", a.Agent)
		}
		if a.Status != protocol.StatusWorking {
			t.Errorf("status: got %q, want working", a.Status)
		}
		if !a.Focused {
			t.Error("focused should be true")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for snapshot")
	}
}

func TestSubscriber_ReceivesHeartbeat(t *testing.T) {
	url, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	s, err := New(url)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	pubNC, _ := nats.Connect(url)
	defer pubNC.Close()

	hb := map[string]string{"ts": time.Now().UTC().Format(time.RFC3339)}
	data, _ := json.Marshal(hb)
	pubNC.Publish(protocol.SubjectHeartbeat, data)
	pubNC.Flush()

	select {
	case <-s.HeartbeatCh():
		// received
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for heartbeat")
	}
}

func TestSubscriber_ConnectionFail(t *testing.T) {
	// Invalid URL format fails immediately (no retry)
	_, err := New("://invalid")
	if err == nil {
		t.Error("expected error for invalid NATS URL")
	}
}

func TestSubscriber_SnapshotChannelNonBlocking(t *testing.T) {
	url, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	s, err := New(url)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	pubNC, _ := nats.Connect(url)
	defer pubNC.Close()

	// Publish many snapshots faster than they're consumed
	snap := protocol.FleetSnapshot{Version: 1, Seq: 1}
	for i := 0; i < 10; i++ {
		data, _ := json.Marshal(snap)
		pubNC.Publish(protocol.SubjectSnapshot, data)
	}
	pubNC.Flush()

	// Should not block — at least one must arrive
	timeout := time.After(3 * time.Second)
	received := false
	for !received {
		select {
		case <-s.SnapshotCh():
			received = true
		case <-timeout:
			t.Fatal("timeout — no snapshot received after 10 publishes")
		}
	}
}
