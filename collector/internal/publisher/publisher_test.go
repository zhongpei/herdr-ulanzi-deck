package publisher

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/herdr-deck/herdrdeck/protocol"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func startNATS(t *testing.T) (string, func()) {
	t.Helper()
	opts := &server.Options{
		Host:   "127.0.0.1",
		Port:   -1,
		NoLog:  true,
		NoSigs: true,
	}
	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("nats server: %v", err)
	}
	ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		ns.Shutdown()
		t.Fatal("nats not ready")
	}
	return ns.ClientURL(), func() { ns.Shutdown(); ns.WaitForShutdown() }
}

func TestPublisher_New(t *testing.T) {
	url, cleanup := startNATS(t)
	defer cleanup()

	p, err := New(url)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil Publisher")
	}
	p.Close()
}

func TestPublisher_PublishSnapshot(t *testing.T) {
	url, cleanup := startNATS(t)
	defer cleanup()

	p, err := New(url)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	// Subscribe on another connection to verify receipt
	subNC, _ := nats.Connect(url)
	defer subNC.Close()

	ch := make(chan *protocol.FleetSnapshot, 1)
	subNC.Subscribe(protocol.SubjectSnapshot, func(msg *nats.Msg) {
		var snap protocol.FleetSnapshot
		if json.Unmarshal(msg.Data, &snap) == nil {
			ch <- &snap
		}
	})
	subNC.Flush()

	snap := &protocol.FleetSnapshot{
		Version:   1,
		Seq:       99,
		UpdatedAt: "2026-06-21T10:00:00Z",
		Machines:  []protocol.MachineInfo{{Name: "t", Abbr: "T", Color: "#000"}},
		Agents: []protocol.AgentState{
			{ID: "t|p1", Machine: "t", Agent: "pi", Name: "task", Status: protocol.StatusWorking, Workspace: "ws", PaneID: "p1"},
		},
		Stats: protocol.AgentStats{Working: 1},
	}

	if err := p.PublishSnapshot(snap); err != nil {
		t.Fatalf("PublishSnapshot: %v", err)
	}

	select {
	case received := <-ch:
		if received.Seq != 99 {
			t.Errorf("seq: got %d, want 99", received.Seq)
		}
		if len(received.Agents) != 1 {
			t.Errorf("agents: got %d, want 1", len(received.Agents))
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
}

func TestPublisher_PublishHeartbeat(t *testing.T) {
	url, cleanup := startNATS(t)
	defer cleanup()

	p, err := New(url)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	subNC, _ := nats.Connect(url)
	defer subNC.Close()

	ch := make(chan bool, 1)
	subNC.Subscribe(protocol.SubjectHeartbeat, func(msg *nats.Msg) {
		var hb map[string]string
		if json.Unmarshal(msg.Data, &hb) == nil {
			if _, ok := hb["ts"]; ok {
				ch <- true
			}
		}
	})
	subNC.Flush()

	if err := p.PublishHeartbeat(); err != nil {
		t.Fatalf("PublishHeartbeat: %v", err)
	}

	select {
	case <-ch:
		// heartbeat received with ts field
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
}

func TestPublisher_Close(t *testing.T) {
	url, cleanup := startNATS(t)
	defer cleanup()

	p, _ := New(url)
	p.Close()
	// Second close should not panic
	p.Close()
}
