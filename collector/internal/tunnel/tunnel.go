// Package tunnel provides SSH tunnel support for remote herdr connections.
//
// Uses SSH ControlMaster so repeated connections to the same host re-use
// the existing SSH transport instead of creating duplicate processes.
package tunnel

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
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

// Args returns the SSH command arguments. Exported for testing.
func (t *Tunnel) Args() []string {
	args := []string{
		"-NL",
		fmt.Sprintf("%d:%s", t.LocalPort, t.RemoteSocket),
	}
	// ControlMaster: share SSH transport across repeated connections.
	// Prevents duplicate SSH processes when the same host/port pair is started again.
	args = append(args,
		"-o", "ControlMaster=auto",
		"-o", fmt.Sprintf("ControlPath=%s", t.controlPath()),
		"-o", "ControlPersist=10m",
	)
	if t.SSHPort > 0 {
		args = append(args, "-p", fmt.Sprintf("%d", t.SSHPort))
	}
	args = append(args, t.Host)
	return args
}

// controlPath returns the ControlMaster socket path, unique per host+port combination.
func (t *Tunnel) controlPath() string {
	key := fmt.Sprintf("herdr-tunnel-%s-%d", t.Host, t.LocalPort)
	return filepath.Join("/tmp", key)
}

// Start spawns the SSH port-forwarding process.
// If an SSH connection to the same host already exists (via ControlMaster socket),
// SSH silently reuses it instead of starting a new process.
func (t *Tunnel) Start() error {
	cmd := exec.Command("ssh", t.Args()...)

	// Capture SSH stderr for diagnostics
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("ssh stderr pipe: %w", err)
	}

	t.cmd = cmd
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ssh start: %w", err)
	}

	// Log SSH stderr lines as they arrive (non-blocking)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if line != "" {
				log.Error().
					Str("conn", t.Host).
					Int("port", t.LocalPort).
					Str("ssh_stderr", line).
					Msg("SSH tunnel error")
			}
		}
		if err := scanner.Err(); err != nil {
			log.Error().Err(err).Str("conn", t.Host).Msg("SSH stderr read error")
		}
	}()

	log.Debug().
		Int("pid", cmd.Process.Pid).
		Strs("args", argsRedacted(t.Args())).
		Msg("SSH tunnel process started")
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
			// Check if SSH process died
			if t.cmd != nil && t.cmd.Process != nil {
				if err := t.cmd.Process.Signal(syscall.Signal(0)); err != nil {
					log.Error().
						Str("conn", t.Host).
						Int("port", t.LocalPort).
						Err(err).
						Msg("SSH process was dead before timeout")
				} else {
					log.Error().
						Str("conn", t.Host).
						Int("port", t.LocalPort).
						Msg("SSH process still alive, port not accepting after timeout")
				}
			}
			return fmt.Errorf("tunnel port %d not ready within %v", t.LocalPort, timeout)
		case <-t.cancel:
			return fmt.Errorf("tunnel cancelled")
		case <-tick.C:
			// Check if SSH process died before port is ready
			if t.cmd != nil && t.cmd.Process != nil {
				if err := t.cmd.Process.Signal(syscall.Signal(0)); err != nil {
					log.Error().
						Str("conn", t.Host).
						Int("port", t.LocalPort).
						Err(err).
						Msg("SSH process died during WaitReady")
					return fmt.Errorf("ssh process died: %w", err)
				}
			}
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
	default:
		close(t.cancel)
	}
	if t.cmd == nil || t.cmd.Process == nil {
		return nil
	}
	log.Debug().Str("conn", t.Host).Int("port", t.LocalPort).Msg("shutting down SSH tunnel")

	// Try to clean up the ControlMaster socket
	ctrlPath := t.controlPath()
	if st, err := os.Stat(ctrlPath); err == nil && !st.IsDir() {
		os.Remove(ctrlPath)
	}

	if err := t.cmd.Process.Signal(syscall.SIGINT); err == nil {
		done := make(chan error, 1)
		go func() { done <- t.cmd.Wait() }()
		select {
		case <-done:
			log.Debug().Str("conn", t.Host).Msg("SSH tunnel stopped gracefully")
			return nil
		case <-time.After(1 * time.Second):
			log.Warn().Str("conn", t.Host).Msg("SSH tunnel force-killed")
			return t.cmd.Process.Kill()
		}
	}
	return t.cmd.Process.Kill()
}

// argsRedacted returns args suitable for logging (redacts socket paths).
func argsRedacted(args []string) []string {
	out := make([]string, len(args))
	copy(out, args)
	for i, a := range out {
		if len(a) > 0 && a[0] == '/' {
			out[i] = "<path>"
		}
	}
	return out
}
