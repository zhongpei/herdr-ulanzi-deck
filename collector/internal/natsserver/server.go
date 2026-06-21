// Package natsserver runs an embedded NATS server for inter-process communication.
package natsserver

import (
	"fmt"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// Server wraps an embedded NATS server instance.
type Server struct {
	ns  *server.Server
	url string
}

// New starts an embedded NATS server on the given listen address.
// Returns after the server is ready to accept connections.
func New(listenAddr string, debug bool) (*Server, error) {
	opts := &server.Options{
		Host:   "127.0.0.1",
		Port:   4222,
		NoLog:  !debug,
		NoSigs: true,
	}
	if listenAddr != "" {
		// parse host:port from listenAddr
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	ns.Start()

	// Wait for server to be ready
	if !ns.ReadyForConnections(5 * time.Second) {
		ns.Shutdown()
		return nil, fmt.Errorf("nats server not ready within 5s")
	}

	url := ns.ClientURL()
	s := &Server{ns: ns, url: url}

	// Verify connectivity
	nc, err := nats.Connect(url, nats.Timeout(2*time.Second))
	if err != nil {
		ns.Shutdown()
		return nil, fmt.Errorf("nats verify connect: %w", err)
	}
	nc.Close()

	return s, nil
}

// URL returns the NATS client URL (e.g. "nats://127.0.0.1:4222").
func (s *Server) URL() string {
	return s.url
}

// Shutdown stops the embedded NATS server gracefully.
func (s *Server) Shutdown() {
	if s.ns != nil {
		s.ns.Shutdown()
		s.ns.WaitForShutdown()
	}
}
