// Package store holds the latest FleetSnapshot, ViewState, and collector health.
// It is the single source of truth for the panel's display state.
package store

import (
	"sync"

	"github.com/herdr-deck/herdrdeck/displaymodel"
	"github.com/herdr-deck/herdrdeck/protocol"
)

// ConnectionHealth tracks collector connectivity.
type ConnectionHealth int

const (
	HealthConnected ConnectionHealth = iota
	HealthOffline
)

// Store holds the latest fleet state and view preferences.
// All field access is protected by a mutex.
type Store struct {
	mu sync.RWMutex

	snapshot       *protocol.FleetSnapshot
	viewState      displaymodel.ViewState
	health         ConnectionHealth
	dirty          bool
	hiddenMachines map[string]bool // empty = all shown
}

// New creates a Store with default ViewState (ModeAll).
func New() *Store {
	return &Store{
		viewState:      displaymodel.ViewState{Mode: displaymodel.ModeAll},
		health:         HealthOffline,
		dirty:          true,
		hiddenMachines: make(map[string]bool),
	}
}

// ApplySnapshot replaces the fleet state and marks the store dirty.
func (s *Store) ApplySnapshot(snap *protocol.FleetSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = snap
	s.health = HealthConnected
	s.dirty = true
}

// Snapshot returns the latest snapshot, or nil.
func (s *Store) Snapshot() *protocol.FleetSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

// ViewState returns the current view state.
func (s *Store) ViewState() displaymodel.ViewState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.viewState
}

// SetViewState replaces the view state and marks dirty.
func (s *Store) SetViewState(vs displaymodel.ViewState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.viewState = vs
	s.dirty = true
}

// MarkHeartbeat updates health to connected when a heartbeat arrives.
func (s *Store) MarkHeartbeat() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.health = HealthConnected
}

// MarkOffline sets health to offline.
func (s *Store) MarkOffline() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.health = HealthOffline
}

// Health returns the current connection health.
func (s *Store) Health() ConnectionHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.health
}

// ToggleMachine toggles whether a machine's agents are hidden.
// Default (empty set) = all machines visible.
func (s *Store) ToggleMachine(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hiddenMachines[name] {
		delete(s.hiddenMachines, name)
	} else {
		s.hiddenMachines[name] = true
	}
	s.dirty = true
}

// IsMachineHidden checks if a machine is currently hidden.
func (s *Store) IsMachineHidden(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hiddenMachines[name]
}

// HiddenMachines returns a copy of the hidden machines set.
func (s *Store) HiddenMachines() map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]bool, len(s.hiddenMachines))
	for k, v := range s.hiddenMachines {
		result[k] = v
	}
	return result
}

// MarkDirty flags the store for UI refresh.
func (s *Store) MarkDirty() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirty = true
}

// IsDirty reports whether a UI refresh is needed and clears the flag.
func (s *Store) IsDirty() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dirty {
		s.dirty = false
		return true
	}
	return false
}
