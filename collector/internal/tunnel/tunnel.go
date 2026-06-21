// Package tunnel provides SSH tunnel support for remote herdr connections.
//
// Usage:
//
//	tun := tunnel.New("fofo@192.168.2.60", "/home/fofo/.config/herdr/herdr.sock", 19999)
//	if err := tun.Start(); err != nil { ... }
//	defer tun.Close()
//	if err := tun.WaitReady(10 * time.Second); err != nil { ... }
//	// Use herdrclient.New("127.0.0.1:19999") to connect to the remote herdr via TCP
package tunnel

import (
	"fmt"
	"net"
	"os/exec"
	"syscall"
	"time"
)

// Tunnel manages an SSH port-forwarding process that exposes a remote herdr
// Unix socket as a local TCP port.
type Tunnel struct {
	Host         string // SSH target, e.g. "fofo@192.168.2.60"
	RemoteSocket string // remote Unix socket path
	LocalPort    int    // local TCP port to bind
	TargetAddr   string // "127.0.0.1:LocalPort"
	SSHPort      int    // SSH port override (0 = default 22)

	cmd    *exec.Cmd
	cancel chan struct{}
}

// NewTunnel creates a tunnel descriptor but does not start it.
func NewTunnel(host, remoteSocket string, localPort int) *Tunnel {
	return &Tunnel{
		Host:         host,
		RemoteSocket: remoteSocket,
		LocalPort:    localPort,
		TargetAddr:   fmt.Sprintf("127.0.0.1:%d", localPort),
		cancel:       make(chan struct{}),
	}
}

// Start spawns the SSH port-forwarding process.
// Equivalent to: ssh -NL <localPort>:<remoteSocket> <host>
func (t *Tunnel) Start() error {
	args := []string{
		"-NL",
		fmt.Sprintf("%d:%s", t.LocalPort, t.RemoteSocket),
	}
	if t.SSHPort > 0 {
		args = append(args, "-p", fmt.Sprintf("%d", t.SSHPort))
	}
	args = append(args, t.Host)
	cmd := exec.Command("ssh", args...)
	t.cmd = cmd
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ssh start: %w", err)
	}
	return nil
}

// WaitReady blocks until the local TCP port is accepting connections,
// or until the timeout expires. Call after Start().
func (t *Tunnel) WaitReady(timeout time.Duration) error {
	deadline := time.After(timeout)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-deadline:
			return fmt.Errorf("tunnel port %d not ready within %v", t.LocalPort, timeout)
		case <-t.cancel:
			return fmt.Errorf("tunnel cancelled")
		case <-tick.C:
			conn, err := net.DialTimeout("tcp", t.TargetAddr, 300*time.Millisecond)
			if err == nil {
				conn.Close()
				return nil
			}
		}
	}
}

// Close terminates the SSH forwarding process gracefully (SIGINT), then
// force-kills after 1 second if it hasn't exited.
func (t *Tunnel) Close() error {
	select {
	case <-t.cancel:
		// already closed
	default:
		close(t.cancel)
	}
	if t.cmd == nil || t.cmd.Process == nil {
		return nil
	}
	// Try graceful shutdown first
	if err := t.cmd.Process.Signal(syscall.SIGINT); err == nil {
		done := make(chan error, 1)
		go func() { done <- t.cmd.Wait() }()
		select {
		case <-done:
			return nil
		case <-time.After(1 * time.Second):
			return t.cmd.Process.Kill()
		}
	}
	return t.cmd.Process.Kill()
}
