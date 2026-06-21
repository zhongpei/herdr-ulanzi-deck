package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ParsesConnections(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", oldHome)

	// Create the expected config dir
	cfgDir := filepath.Join(dir, ".config", "herdr-deck")
	os.MkdirAll(cfgDir, 0755)

	// Write a real connections.json with actual data
	cfg := AppConfig{
		Connections: []ConnConfig{
			{Name: "local", Abbr: "LCL", Color: "#4ADE80", Type: "local"},
			{
				Name: "dev-server", Abbr: "DEV", Color: "#60A5FA", Type: "ssh",
				Host: "user@192.168.1.100", RemoteSocket: "/home/user/.config/herdr/herdr.sock",
				LocalPort: 19999, SSHPort: 22,
			},
		},
		K11Toggle: true,
	}
	data, err := json.MarshalIndent(cfg, "", "\t")
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "connections.json")
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if len(loaded.Connections) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(loaded.Connections))
	}

	// Verify local connection
	local := loaded.Connections[0]
	if local.Name != "local" {
		t.Errorf("local.Name: got %s, want local", local.Name)
	}
	if local.Abbr != "LCL" {
		t.Errorf("local.Abbr: got %s, want LCL", local.Abbr)
	}
	if local.Type != "local" {
		t.Errorf("local.Type: got %s, want local", local.Type)
	}

	// Verify SSH connection
	ssh := loaded.Connections[1]
	if ssh.Name != "dev-server" {
		t.Errorf("ssh.Name: got %s, want dev-server", ssh.Name)
	}
	if ssh.Type != "ssh" {
		t.Errorf("ssh.Type: got %s, want ssh", ssh.Type)
	}
	if ssh.Host != "user@192.168.1.100" {
		t.Errorf("ssh.Host: got %s", ssh.Host)
	}
	if ssh.RemoteSocket != "/home/user/.config/herdr/herdr.sock" {
		t.Errorf("ssh.RemoteSocket: got %s", ssh.RemoteSocket)
	}
	if ssh.LocalPort != 19999 {
		t.Errorf("ssh.LocalPort: got %d, want 19999", ssh.LocalPort)
	}

	// Verify K11Toggle
	if !loaded.K11Toggle {
		t.Error("K11Toggle should be true")
	}
}

func TestLoadConfig_DefaultWhenMissing(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", oldHome)

	// No config file at all → should return default
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if len(loaded.Connections) != 1 {
		t.Fatalf("expected 1 default connection, got %d", len(loaded.Connections))
	}
	if loaded.Connections[0].Name != "local" {
		t.Errorf("default conn: got %s, want local", loaded.Connections[0].Name)
	}
	if loaded.Connections[0].Abbr != "LCL" {
		t.Errorf("default conn abbr: got %s, want LCL", loaded.Connections[0].Abbr)
	}

	// Default creates the config file
	cfgDir := filepath.Join(dir, ".config", "herdr-deck")
	cfgPath := filepath.Join(cfgDir, "connections.json")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Error("default config file should have been created")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", oldHome)

	cfgDir := filepath.Join(dir, ".config", "herdr-deck")
	os.MkdirAll(cfgDir, 0755)
	cfgPath := filepath.Join(cfgDir, "connections.json")
	os.WriteFile(cfgPath, []byte("not valid json"), 0644)

	loaded, err := LoadConfig()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
	if len(loaded.Connections) != 1 {
		t.Errorf("should fall back to default: got %d connections", len(loaded.Connections))
	}
}

func TestConnConfig_AllFields(t *testing.T) {
	c := ConnConfig{
		Name:         "test-host",
		Abbr:         "TST",
		Color:        "#FF00FF",
		Type:         "ssh",
		Host:         "admin@10.0.0.1",
		RemoteSocket: "/tmp/herdr.sock",
		LocalPort:    20000,
		SSHPort:      2222,
	}

	data, _ := json.Marshal(c)
	var parsed ConnConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed != c {
		t.Errorf("roundtrip failed: got %+v, want %+v", parsed, c)
	}
}

func TestFindLocalSocket_FromEnv(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "herdr.sock")
	os.WriteFile(sockPath, []byte{}, 0644)

	t.Setenv("HERDR_SOCKET_PATH", sockPath)

	found := FindLocalSocket()
	if found != sockPath {
		t.Errorf("FindLocalSocket from env: got %q, want %q", found, sockPath)
	}
}

func TestFindLocalSocket_FromStandardPaths(t *testing.T) {
	// Override HOME so we control the search paths
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// Unset HERDR_SOCKET_PATH so it falls through to standard paths
	t.Setenv("HERDR_SOCKET_PATH", "")

	// Create socket file in expected path
	cfgDir := filepath.Join(dir, ".config", "herdr")
	os.MkdirAll(cfgDir, 0755)
	sockPath := filepath.Join(cfgDir, "herdr.sock")
	os.WriteFile(sockPath, []byte{}, 0644)

	found := FindLocalSocket()
	if found != sockPath {
		t.Errorf("FindLocalSocket from standard path: got %q, want %q", found, sockPath)
	}
}

func TestFindLocalSocket_NotFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("HERDR_SOCKET_PATH", "")

	found := FindLocalSocket()
	if found != "" {
		t.Errorf("expected empty when no socket exists, got %q", found)
	}
}
