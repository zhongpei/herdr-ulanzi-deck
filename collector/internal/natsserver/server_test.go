package natsserver

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestServer_NewAndShutdown(t *testing.T) {
	s, err := New(Options{Host: "127.0.0.1", Port: -1, Debug: false})
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
	s, err := New(Options{Host: "127.0.0.1", Port: -1, Debug: false})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	s.Shutdown()
	// Second shutdown should not panic
	s.Shutdown()
}

func TestServer_URL(t *testing.T) {
	s, err := New(Options{Host: "127.0.0.1", Port: -1, Debug: false})
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
	s, err := New(Options{Host: "127.0.0.1", Port: -1, Debug: false})
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

func TestServer_CustomPort(t *testing.T) {
	// Find an available port by binding to 0
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	freePort := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	s, err := New(Options{Host: "127.0.0.1", Port: freePort, Debug: false})
	if err != nil {
		t.Fatalf("New with Port %d: %v", freePort, err)
	}
	defer s.Shutdown()

	// Verify URL contains the specified port
	if !strings.Contains(s.URL(), fmt.Sprintf(":%d", freePort)) {
		t.Errorf("URL %q does not contain port %d", s.URL(), freePort)
	}

	// Verify connectivity with that URL
	nc, err := nats.Connect(s.URL(), nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect on custom port %d: %v", freePort, err)
	}
	nc.Close()
}

func TestServer_DefaultPort(t *testing.T) {
	// Port=0 should use the default port 4222 (if available)
	// Skip if port 4222 is already in use
	s, err := New(Options{Host: "127.0.0.1", Port: 0, Debug: false})
	if err != nil {
		t.Skipf("default port 4222 not available: %v", err)
	}
	defer s.Shutdown()

	if !strings.Contains(s.URL(), ":4222") {
		t.Errorf("default port: expected :4222 in URL, got %q", s.URL())
	}
}
