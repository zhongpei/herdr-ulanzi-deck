// Package config persists window size between sessions.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

const (
	configDir  = "herdr-deck"
	configFile = "panel-gio.json"
)

// Size holds the saved window size and position.
type Size struct {
	Width  float32 `json:"width"`
	Height float32 `json:"height"`
	PosX   int     `json:"pos_x,omitempty"`
	PosY   int     `json:"pos_y,omitempty"`
}

// Path returns the config file path.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(dir, configDir, configFile), nil
}

// Load reads saved window size, returning defaults if unavailable.
// Never returns nil — always returns a valid Size.
func Load(defaultW, defaultH float32) *Size {
	path, err := Path()
	if err != nil {
		log.Warn().Err(err).Msg("cannot determine config path, using defaults")
		return &Size{Width: defaultW, Height: defaultH}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warn().Err(err).Str("path", path).Msg("cannot read config")
		}
		return &Size{Width: defaultW, Height: defaultH}
	}
	var s Size
	if err := json.Unmarshal(data, &s); err != nil {
		log.Warn().Err(err).Str("path", path).Msg("invalid config, using defaults")
		return &Size{Width: defaultW, Height: defaultH}
	}
	log.Debug().Float32("w", s.Width).Float32("h", s.Height).Msg("loaded window size")
	return &s
}

// SavePosition writes position to the existing config.
func SavePosition(x, y int) {
	path, err := Path()
	if err != nil {
		return
	}
	var s Size
	if data, _ := os.ReadFile(path); data != nil {
		json.Unmarshal(data, &s)
	}
	s.PosX, s.PosY = x, y
	writeConfig(path, s)
}

// Save atomically writes the window size to the config file.
func Save(w, h float32) {
	path, err := Path()
	if err != nil {
		log.Warn().Err(err).Msg("cannot save window size")
		return
	}
	writeConfig(path, Size{Width: w, Height: h})
}

func writeConfig(path string, s Size) {
	data, err := json.Marshal(s)
	if err != nil {
		log.Warn().Err(err).Msg("cannot marshal config")
		return
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Warn().Err(err).Str("dir", dir).Msg("cannot create config dir")
		return
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		log.Warn().Err(err).Str("path", tmpPath).Msg("cannot write temp config")
		return
	}
	if err := os.Rename(tmpPath, path); err != nil {
		log.Warn().Err(err).Str("tmp", tmpPath).Str("path", path).Msg("cannot atomically rename config")
		os.Remove(tmpPath)
		return
	}
	log.Debug().Interface("config", s).Msg("saved config")
}
