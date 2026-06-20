package herdr

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

// ConnConfig describes one herdr connection (local or SSH).
type ConnConfig struct {
	Name         string `json:"name"`
	Abbr         string `json:"abbr"`
	Color        string `json:"color"`
	Type         string `json:"type"` // "local" or "ssh"
	Host         string `json:"host,omitempty"`
	RemoteSocket string `json:"remoteSocket,omitempty"`
	LocalPort    int    `json:"localPort,omitempty"`
	SSHPort      int    `json:"sshPort,omitempty"`
}

// AppConfig holds the full set of configured connections.
type AppConfig struct {
	Connections []ConnConfig `json:"connections"`
}

// defaultConfig is used when no config file exists.
var defaultConfig = AppConfig{
	Connections: []ConnConfig{
		{Name: "local", Abbr: "LCL", Color: "#4ADE80", Type: "local"},
	},
}

// configDir returns ~/.config/herdr-deck/
func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "herdr-deck")
}

// LoadConfig reads connections.json from ~/.config/herdr-deck/.
// Creates a default config if the file doesn't exist.
func LoadConfig() (*AppConfig, error) {
	dir := configDir()
	configPath := filepath.Join(dir, "connections.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config
			if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
				return &defaultConfig, fmt.Errorf("mkdir: %w", mkErr)
			}
			jsonData, _ := json.MarshalIndent(defaultConfig, "", "\t")
			if wErr := os.WriteFile(configPath, jsonData, 0644); wErr != nil {
				return &defaultConfig, fmt.Errorf("write default: %w", wErr)
			}
			log.Info().Str("path", configPath).Msg("created default config")
			return &defaultConfig, nil
		}
		return &defaultConfig, fmt.Errorf("read: %w", err)
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &defaultConfig, fmt.Errorf("parse: %w", err)
	}
	return &cfg, nil
}

// FindLocalSocket checks common paths for the herdr Unix socket.
// Returns the first socket found, or empty string.
func FindLocalSocket() string {
	home, _ := os.UserHomeDir()
	candidates := []string{
		os.Getenv("HERDR_SOCKET_PATH"),
		filepath.Join(home, ".config", "herdr", "herdr.sock"),
		filepath.Join(home, ".local", "share", "herdr", "herdr.sock"),
		"/tmp/herdr.sock",
		filepath.Join(home, ".local", "state", "herdr", "herdr.sock"),
	}

	for _, p := range candidates {
		if p == "" {
			continue
		}
		if fi, err := os.Stat(p); err == nil {
			if fi.Mode()&os.ModeSocket != 0 || fi.Mode().IsRegular() {
				log.Debug().Str("path", p).Msg("found herdr socket")
				return p
			}
		}
	}
	return ""
}

// ConnectAndFetch connects to all configured herdr instances and
// returns the merged UnifiedWorkspace list.
// Falls back to nil if no connections succeed.
func ConnectAndFetch(cfg *AppConfig) []byte {
	// TODO: return proper typed data - for now using json.RawMessage path
	// through the bridge
	return nil
}
