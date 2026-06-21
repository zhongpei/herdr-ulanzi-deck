// Package profile manages UlanziDeck profile creation for herdr-deck.
// Mirrors src/profile-manager.js
//
// Creates a dedicated D200X profile with all 14 keys assigned to our action,
// so the plugin can render them all via WebSocket state commands.
package profile

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

const (
	PluginUUID  = "com.ulanzi.herdr.agentview"
	ActionUUID  = "com.ulanzi.herdr.agentview.monitor"
	ProfileName = "Herdr Deck"
)

// D200X key positions in col_row format (all 14 visible keys)
var D200XKeys = []string{
	"0_0", "1_0", "2_0", "3_0", "4_0", // row 0
	"0_1", "1_1", "2_1", "3_1", "4_1", // row 1
	"0_2", "1_2", "2_2", "3_2", // row 2 (3_2 = large)
}

// profilesDir returns the UlanziDeck profiles directory.
func profilesDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Ulanzi", "UlanziDeck", "ProfilesV2")
}

// Manager handles UlanziDeck profile lifecycle.
type Manager struct {
	profileDir string
}

// New creates a ProfileManager.
func New() *Manager {
	return &Manager{}
}

// pageManifest builds the controller actions for a single page (14 keys + encoders).
func buildPageManifest() map[string]any {
	keypadActions := make(map[string]any)
	for _, key := range D200XKeys {
		keypadActions[key] = map[string]any{
			"Action":      ActionUUID,
			"ActionID":    newUUID(),
			"ActionParam": map[string]any{},
			"LinkedTitle": false,
			"Name":        "Agent",
			"Plugin": map[string]any{
				"Name":    "Herdr Agent View",
				"UUID":    PluginUUID,
				"Version": "0.1.0",
			},
			"State": 0,
			"ViewParam": []map[string]any{
				{"Icon": "", "IconRel": ""},
			},
		}
	}
	return map[string]any{
		"Controllers": []map[string]any{
			{"Actions": map[string]any{}, "Type": "Encoder"},
			{"Actions": keypadActions, "Type": "Keypad"},
		},
	}
}

// FindOurProfile locates an existing herdr-deck profile.
func (m *Manager) FindOurProfile() string {
	dir := profilesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() || !isProfileDir(entry.Name()) {
			continue
		}
		manifestPath := filepath.Join(dir, entry.Name(), "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var manifest struct {
			Name string `json:"Name"`
		}
		if json.Unmarshal(data, &manifest) != nil {
			continue
		}
		if manifest.Name == ProfileName {
			m.profileDir = filepath.Join(dir, entry.Name())
			return m.profileDir
		}
	}
	return ""
}

// CreateProfile creates a new herdr-deck profile with 4 pages.
func (m *Manager) CreateProfile(deviceUUID string) string {
	profileUUID := newUUID()
	dir := filepath.Join(profilesDir(), profileUUID+".ulanziProfile")
	m.profileDir = dir

	if err := os.MkdirAll(filepath.Join(dir, "Profiles"), 0755); err != nil {
		log.Error().Err(err).Msg("mkdir failed for profile")
		return ""
	}

	// Create 4 pages
	pageUUIDs := make([]string, 4)
	for i := 0; i < 4; i++ {
		puid := newUUID()
		pageUUIDs[i] = puid
		pageDir := filepath.Join(dir, "Profiles", puid)
		os.MkdirAll(filepath.Join(pageDir, "Files"), 0755)
		os.MkdirAll(filepath.Join(pageDir, "Images"), 0755)

		manifest := buildPageManifest()
		manifestData, _ := json.MarshalIndent(manifest, "", "\t")
		os.WriteFile(filepath.Join(pageDir, "manifest.json"), manifestData, 0644)
	}

	// Profile manifest
	profileManifest := map[string]any{
		"Device": map[string]any{
			"Model": "D200X",
			"UUID":  deviceUUID,
		},
		"Icon": "icon_default_profile.png",
		"Name": ProfileName,
		"Pages": map[string]any{
			"Current": pageUUIDs[0],
			"Pages":   pageUUIDs,
		},
		"Version": 2,
	}
	manifestData, _ := json.MarshalIndent(profileManifest, "", "\t")
	os.WriteFile(filepath.Join(dir, "manifest.json"), manifestData, 0644)
	os.WriteFile(filepath.Join(dir, "icon_default_profile.png"), []byte{}, 0644)

	log.Info().Int("pages", len(pageUUIDs)).Msg("profile created") // nolint: zerolog-msg-format
	return dir
}

// GetKeyActionMap reads the key→actionid mapping from the first page.
func (m *Manager) GetKeyActionMap() map[string]string {
	if m.profileDir == "" {
		return nil
	}

	pagesDir := filepath.Join(m.profileDir, "Profiles")
	entries, err := os.ReadDir(pagesDir)
	if err != nil {
		return nil
	}

	// Find first page directory
	var firstPage string
	for _, e := range entries {
		if e.IsDir() {
			firstPage = e.Name()
			break
		}
	}
	if firstPage == "" {
		return nil
	}

	manifestPath := filepath.Join(pagesDir, firstPage, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil
	}

	var manifest struct {
		Controllers []struct {
			Type    string         `json:"Type"`
			Actions map[string]any `json:"Actions"`
		} `json:"Controllers"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil
	}

	for _, c := range manifest.Controllers {
		if c.Type == "Keypad" {
			// Extract key→ActionID maps
			// Need to parse the key→obj structure
			// Each value is {"ActionID": "..."}
			result := make(map[string]string)
			for key, val := range c.Actions {
				if obj, ok := val.(map[string]any); ok {
					if aid, ok := obj["ActionID"].(string); ok {
						result[key] = aid
					}
				}
			}
			return result
		}
	}
	return nil
}

// ActivateProfile writes the current profile name to Ulanzi's settings.json
// so the deck switches to our profile for the D200X device.
func (m *Manager) ActivateProfile(deviceUUID string) {
	if m.profileDir == "" {
		return
	}

	home, _ := os.UserHomeDir()
	settingPath := filepath.Join(home, "Library", "Application Support", "Ulanzi", "UlanziDeck", "config", "setting.json")
	data, err := os.ReadFile(settingPath)
	if err != nil {
		log.Warn().Err(err).Str("path", settingPath).Msg("setting.json not found")
		return
	}

	var setting map[string]any
	if err := json.Unmarshal(data, &setting); err != nil {
		log.Error().Err(err).Msg("parse setting.json failed")
		return
	}

	// Set CurrentProfile by name
	setting["CurrentProfile"] = ProfileName

	// Update device-specific settings
	if devices, ok := setting["Devices"].([]any); ok {
		for i, dev := range devices {
			if devMap, ok := dev.(map[string]any); ok {
				devType, _ := devMap["DeviceType"].(string)
				devUUID, _ := devMap["CurrentDevice"].(string)
				if devType == "D200X" || devUUID == deviceUUID {
					devices[i].(map[string]any)["CurrentProfile"] = ProfileName
				}
			}
		}
	}

	updated, _ := json.MarshalIndent(setting, "", "\t")
	if err := os.WriteFile(settingPath, updated, 0644); err != nil {
		log.Error().Err(err).Str("path", settingPath).Msg("failed to write setting.json")
		return
	}
	log.Info().Str("device", deviceUUID).Msg("profile activated")
}

// RemoveOldProfiles deletes ALL existing Herdr Deck profiles.
// This prevents duplicates from accumulating.
func (m *Manager) RemoveOldProfiles() {
	dir := profilesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !isProfileDir(entry.Name()) {
			continue
		}
		manifestPath := filepath.Join(dir, entry.Name(), "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var manifest struct {
			Name string `json:"Name"`
		}
		if json.Unmarshal(data, &manifest) != nil {
			continue
		}
		if manifest.Name == ProfileName {
			fullPath := filepath.Join(dir, entry.Name())
			os.RemoveAll(fullPath)
			log.Info().Str("path", entry.Name()).Msg("removed old duplicate profile")
		}
	}
}

// Ensure finds or creates a SINGLE herdr-deck profile.
// Always removes old duplicates first, then creates one fresh.
func (m *Manager) Ensure(deviceUUID string) string {
	m.RemoveOldProfiles()
	return m.CreateProfile(deviceUUID)
}

func isProfileDir(name string) bool {
	return len(name) > len(".ulanziProfile") && name[len(name)-len(".ulanziProfile"):] == ".ulanziProfile"
}

// newUUID generates a random UUID v4 string using crypto/rand.
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	// Set version 4
	b[6] = (b[6] & 0x0f) | 0x40
	// Set variant bits
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// nolint: unused
var _ = fmt.Sprintf
