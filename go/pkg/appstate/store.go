// Package appstate provides the central state store for herdr-deck.
//
// Architecture: events → Store → Capture → hash compare → render
//
// All state mutations go through Store methods. No code path should modify
// stateManager or mapper directly outside this package.
package appstate

import (
	"fmt"

	"github.com/herdr-deck/herdrdeck/pkg/deck"
	"github.com/herdr-deck/herdrdeck/pkg/mapper"
	"github.com/herdr-deck/herdrdeck/pkg/state"
	"github.com/herdr-deck/herdrdeck/pkg/types"
)

// Store holds the application state and provides the single source of truth
// for render decisions. It wraps state.Manager, mapper.Mapper, and references
// the deck client for actionID management.
type Store struct {
	sm     *state.Manager
	mapper *mapper.Mapper
	deck   *deck.Client // for seeding keyActions
	dirty  bool
}

// New creates a Store wrapping the given state manager and mapper.
func New(sm *state.Manager, m *mapper.Mapper) *Store {
	return &Store{
		sm:     sm,
		mapper: m,
	}
}

// SetDeckClient attaches the deck client for key action seeding.
func (s *Store) SetDeckClient(dc *deck.Client) {
	s.deck = dc
}

// SeedKeyActions seeds the key→actionID map into the deck client.
// Called after profile ensure or reconnect.
func (s *Store) SeedKeyActions(kv map[string]string) {
	if s.deck != nil {
		s.deck.SeedKeyActions(kv)
	}
}

// ─── Business methods ──────────────────────────────────────
// These mutate the store and mark it dirty for the next render tick.

func (s *Store) SetAll() {
	s.mapper.SetAll()
	s.dirty = true
}

func (s *Store) NextMachine() {
	s.mapper.NextMachine()
	s.dirty = true
}

func (s *Store) NextSpace() {
	s.mapper.NextSpace()
	s.dirty = true
}

// RefreshHerdrData replaces the unified workspace tree and marks dirty.
func (s *Store) RefreshHerdrData(unified []types.UnifiedWorkspace) {
	s.sm.Init(unified)
	s.dirty = true
}

// ─── Snapshot ──────────────────────────────────────────────

// Snapshot holds render-relevant state at a point in time.
type Snapshot struct {
	TopAgents     []types.AgentInfo
	Mode          mapper.FilterMode
	ConnName      string
	WsID          string
	Stats         types.AgentStats
	CPUPercent    float64
	MemoryPercent float64
	durationFP    string // fingerprint of per-agent status durations
	hash          string
}

// Capture reads the current state from the store and returns a Snapshot.
func (s *Store) Capture() *Snapshot {
	cpu, mem := s.sm.GetSysStats()
	snap := &Snapshot{
		TopAgents:     s.sm.GetFilteredAgents(s.mapper.ConnName, s.mapper.WsID),
		Mode:          s.mapper.Mode,
		ConnName:      s.mapper.ConnName,
		WsID:          s.mapper.WsID,
		Stats:         s.sm.ComputeStats(),
		CPUPercent:    cpu,
		MemoryPercent: mem,
	}
	// Build per-agent duration fingerprint so hash catches minute-level changes
	var durFP string
	for _, a := range snap.TopAgents {
		d := s.sm.FormatAgentDuration(a.ConnName, a.PaneID)
		durFP += a.PaneID + "=" + d + "|"
	}
	snap.durationFP = durFP
	snap.hash = snap.visualHash()
	return snap
}

// ChangedSince returns true if the snapshot differs from a previous hash.
func (s *Snapshot) ChangedSince(prevHash string) bool {
	return s.hash != prevHash
}

// Hash returns the visual hash for dedup comparison.
func (s *Snapshot) Hash() string {
	return s.hash
}

func (s *Store) IsDirty() bool {
	return s.dirty
}

func (s *Store) MarkClean() {
	s.dirty = false
}

// SetSysStats updates system CPU/memory percentages in the state manager
// and marks the store dirty so the next render tick picks them up.
func (s *Store) SetSysStats(cpu, mem float64) {
	s.sm.SetSysStats(cpu, mem)
	s.dirty = true
}

// ForceDirty marks the store dirty to trigger re-render (used after reconnect).
func (s *Store) ForceDirty() {
	s.dirty = true
}

// visualHash computes a fingerprint of all state that affects visual output.
func (s *Snapshot) visualHash() string {
	var fp string
	for _, a := range s.TopAgents {
		fp += a.PaneID + "|" + a.Agent + "|" + string(a.AgentStatus) + "|"
		if a.Focused {
			fp += "1"
		} else {
			fp += "0"
		}
		fp += "|" + a.Name + "|" + a.ConnName + "|" + a.WsLabel + "\n"
	}
	fp += "M" + itoa(int(s.Mode)) + "|" + s.ConnName + "|" + s.WsID + "\n"
	fp += "S" + itoa(s.Stats.Done) + itoa(s.Stats.Idle) +
		itoa(s.Stats.Working) + itoa(s.Stats.Blocked) + itoa(s.Stats.Unknown)
	cpuStr := fmt.Sprintf("%.1f", s.CPUPercent)
	memStr := fmt.Sprintf("%.1f", s.MemoryPercent)
	fp += "CPU" + cpuStr + "|MEM" + memStr
	fp += "|DUR|" + s.durationFP
	return fp
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
