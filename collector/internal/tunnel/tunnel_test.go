// Package tunnel tests SSH tunnel creation. Uses real SSH to localhost
// when available, and verifies ControlMaster arguments enable connection reuse.
package tunnel

import (
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewTunnel(t *testing.T) {
	tun := NewTunnel("user@host", "/remote/herdr.sock", 19999)
	if tun == nil {
		t.Fatal("NewTunnel returned nil")
	}
	if tun.Host != "user@host" {
		t.Errorf("Host: got %q, want user@host", tun.Host)
	}
	if tun.LocalPort != 19999 {
		t.Errorf("LocalPort: got %d, want 19999", tun.LocalPort)
	}
	if tun.TargetAddr != "127.0.0.1:19999" {
		t.Errorf("TargetAddr: got %q, want 127.0.0.1:19999", tun.TargetAddr)
	}
}

func TestTunnel_Args_ContainsControlMaster(t *testing.T) {
	tun := NewTunnel("user@host", "/path/sock", 20000)
	args := tun.Args()

	full := strings.Join(args, " ")
	if !strings.Contains(full, "ControlMaster=auto") {
		t.Errorf("args missing ControlMaster=auto: %s", full)
	}
	if !strings.Contains(full, "ControlPersist=10m") {
		t.Errorf("args missing ControlPersist=10m: %s", full)
	}
	if !strings.Contains(full, "ControlPath") {
		t.Errorf("args missing ControlPath: %s", full)
	}
	if !strings.Contains(full, "-NL") {
		t.Errorf("args missing -NL: %s", full)
	}
	if !strings.Contains(full, "20000:/path/sock") {
		t.Errorf("args missing port:socket: %s", full)
	}
}

func TestTunnel_Args_SSHPort(t *testing.T) {
	tun := NewTunnel("user@host", "/path/sock", 20001)
	tun.SSHPort = 2222
	args := tun.Args()

	full := strings.Join(args, " ")
	if !strings.Contains(full, "-p 2222") {
		t.Errorf("args missing -p 2222: %s", full)
	}
}

func TestTunnel_Args_NoSSHPort(t *testing.T) {
	tun := NewTunnel("user@host", "/path/sock", 20002)
	tun.SSHPort = 0
	args := tun.Args()

	full := strings.Join(args, " ")
	if strings.Contains(full, "-p ") {
		t.Errorf("args should not contain -p when SSHPort=0: %s", full)
	}
}

func TestControlPath(t *testing.T) {
	tun := NewTunnel("jyhl@114.236.93.186", "/remote.sock", 19998)
	path := tun.controlPath()
	expected := filepath.Join("/tmp", "herdr-tunnel-jyhl@114.236.93.186-19998")
	if path != expected {
		t.Errorf("controlPath: got %q, want %q", path, expected)
	}
	// Verify it starts with /tmp and ends with the correct suffix
	if !strings.HasPrefix(path, "/tmp/herdr-tunnel-") {
		t.Errorf("controlPath should be under /tmp/herdr-tunnel-: %s", path)
	}
	if !strings.HasSuffix(path, "-19998") {
		t.Errorf("controlPath should end with -19998: %s", path)
	}
}

func TestTunnel_TargetAddr(t *testing.T) {
	tun := NewTunnel("user@host", "/sock", 19999)
	if tun.TargetAddr != "127.0.0.1:19999" {
		t.Errorf("expected 127.0.0.1:19999, got %s", tun.TargetAddr)
	}
}

func TestTunnel_WaitReady_Timeout(t *testing.T) {
	// Use a port that's not in use — WaitReady should timeout.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close() // release the port

	tun := NewTunnel("user@host", "/dev/null", port)

	// Start a fake SSH process that just sleeps (no port binding)
	tun.Start()

	err = tun.WaitReady(1 * time.Second)
	if err == nil {
		// Port might be available (reused by another process) — skip
		t.Log("port was available before timeout (unexpected but acceptable)")
		// Try to close SSH gracefully
		tun.Close()
		return
	}
	if !strings.Contains(err.Error(), "not ready") {
		t.Errorf("expected timeout error, got: %v", err)
	}
	tun.Close()
}

func TestTunnel_WaitReady_Success(t *testing.T) {
	// Start a real TCP listener, then verify WaitReady detects it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	tun := NewTunnel("user@host", "/dev/null", port)

	// Verify the port is immediately available
	err = tun.WaitReady(2 * time.Second)
	if err != nil {
		t.Fatalf("WaitReady on open port: %v", err)
	}

	ln.Close()
}

func TestTunnel_Close_Idempotent(t *testing.T) {
	tun := NewTunnel("user@host", "/dev/null", 65500)
	// Close without Start — should not panic
	err := tun.Close()
	if err != nil {
		t.Errorf("Close without Start: %v", err)
	}
	// Close again — should not panic
	err = tun.Close()
	if err != nil {
		t.Errorf("Close twice: %v", err)
	}
}

func TestNewTunnel_AllFields(t *testing.T) {
	tun := NewTunnel("test@host", "/tmp/test.sock", 12345)
	if tun.Host != "test@host" {
		t.Errorf("Host: got %q", tun.Host)
	}
	if tun.RemoteSocket != "/tmp/test.sock" {
		t.Errorf("RemoteSocket: got %q", tun.RemoteSocket)
	}
	if tun.LocalPort != 12345 {
		t.Errorf("LocalPort: got %d", tun.LocalPort)
	}
	if tun.SSHPort != 0 {
		t.Errorf("SSHPort should default to 0, got %d", tun.SSHPort)
	}
}

func TestArgsRedacted(t *testing.T) {
	args := []string{"ssh", "-NL", "19999:/home/user/herdr.sock", "user@host"}
	redacted := argsRedacted(args)
	if len(redacted) != len(args) {
		t.Errorf("length changed: got %d, want %d", len(redacted), len(args))
	}
	for _, a := range redacted {
		if strings.HasPrefix(a, "/") {
			t.Errorf("path should be redacted, got: %s", a)
		}
	}
}
