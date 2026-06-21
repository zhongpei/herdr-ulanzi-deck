package natsserver

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestServer_NewAndShutdown(t *testing.T) {
	s, err := New("", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if s.URL() == "" {
		t.Error("URL should not be empty")
	}

	// Verify we can connect
	nc, err := nats.Connect(s.URL(), nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	nc.Close()

	s.Shutdown()
}

func TestServer_ShutdownTwice(t *testing.T) {
	s, err := New("", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	s.Shutdown()
	// Second shutdown should not panic
	s.Shutdown()
}

func TestServer_URL(t *testing.T) {
	s, err := New("", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Shutdown()

	url := s.URL()
	if url == "" {
		t.Error("URL should return non-empty string")
	}
}

func TestServer_PublishSubscribe(t *testing.T) {
	s, err := New("", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Shutdown()

	// Connect two clients
	pubNC, _ := nats.Connect(s.URL())
	defer pubNC.Close()

	subNC, _ := nats.Connect(s.URL())
	defer subNC.Close()

	ch := make(chan string, 1)
	subNC.Subscribe("test.subject", func(msg *nats.Msg) {
		ch <- string(msg.Data)
	})
	subNC.Flush()

	// Publish
	pubNC.Publish("test.subject", []byte("hello"))
	pubNC.Flush()

	select {
	case got := <-ch:
		if got != "hello" {
			t.Errorf("got %q, want hello", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}
