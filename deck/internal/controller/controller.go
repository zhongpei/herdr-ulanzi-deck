// Package controller manages the deck's render event loop.
//
// It owns the dirty flag, snapshot capture, hash compare, and renderAll()
// orchestration. It replaces the old appstate.Store.
package controller

import (
	"fmt"

	"github.com/herdr-deck/herdrdeck/deck/internal/fleet"
	"github.com/herdr-deck/herdrdeck/displaymodel"
	"github.com/herdr-deck/herdrdeck/protocol"
)

// CaptureState holds the rendered display model and its fingerprint at a
// point in time. Used for hash-based render dedup.
type CaptureState struct {
	Model displaymodel.Model
	hash  string
}

// Hash returns the fingerprint for render dedup comparison.
func (s *CaptureState) Hash() string { return s.hash }

// ChangedSince returns true if this state differs from a previous hash.
func (s *CaptureState) ChangedSince(prevHash string) bool { return s.hash != prevHash }

// Controller manages the deck's render cycle.
type Controller struct {
	fleet        *fleet.Manager
	displayBld   *displaymodel.Builder
	dirty        bool
	lastSnapshot *protocol.FleetSnapshot
	lastModel    *displaymodel.Model
	k11Enabled   bool // K11Toggle from CLI flag
}

// NewController creates a controller wrapping fleet manager, displaymodel builder,
// and K11 toggle preference.
func NewController(fm *fleet.Manager, bld *displaymodel.Builder, k11Enabled bool) *Controller {
	return &Controller{
		fleet:      fm,
		displayBld: bld,
		k11Enabled: k11Enabled,
	}
}

// ApplySnapshot caches the latest snapshot for use by displaymodel builder.
func (c *Controller) ApplySnapshot(snap *protocol.FleetSnapshot) {
	c.lastSnapshot = snap
}

// OnK11 handles the K11 (ALL/ACT) key press.
// Preserves current behavior: always resets to ALL mode first, then toggles
// the ActiveOnly filter when K11Toggle is enabled.
func (c *Controller) OnK11() {
	c.displayBld.SetAll()
	if c.k11Enabled {
		c.displayBld.ToggleActiveOnly()
	}
	c.MarkDirty()
}

// OnK12 handles the K12 (machine cycle) key press.
func (c *Controller) OnK12() {
	if snap := c.lastSnapshot; snap != nil {
		c.displayBld.NextMachine(snap)
	}
	c.MarkDirty()
}

// OnK13 handles the K13 (space cycle) key press.
func (c *Controller) OnK13() {
	if snap := c.lastSnapshot; snap != nil {
		c.displayBld.NextSpace(snap)
	}
	c.MarkDirty()
}

// LastModel returns the most recently built display model, or nil.
func (c *Controller) LastModel() *displaymodel.Model {
	return c.lastModel
}

// MarkDirty flags the controller for the next render cycle.
func (c *Controller) MarkDirty() {
	c.dirty = true
}

// IsDirty returns whether a render is pending.
func (c *Controller) IsDirty() bool {
	return c.dirty
}

// MarkClean resets the dirty flag after a render cycle.
func (c *Controller) MarkClean() {
	c.dirty = false
}

// Capture reads the current fleet/viewmodel state and returns a CaptureState
// with the built display model and its hash fingerprint.
func (c *Controller) Capture() *CaptureState {
	if c.lastSnapshot == nil {
		// Before first snapshot, return empty model
		model := c.displayBld.Build(&protocol.FleetSnapshot{}, displaymodel.LocalStats{}, nil)
		return &CaptureState{Model: model, hash: ""}
	}

	// Gather durations from fleet manager
	durations := make(map[string]string, len(c.lastSnapshot.Agents))
	for _, a := range c.lastSnapshot.Agents {
		durations[a.ID] = c.fleet.FormatAgentDuration(a.Machine, a.PaneID)
	}

	// Gather local stats
	cpu, mem := c.fleet.GetSysStats()
	local := displaymodel.LocalStats{CPUPercent: cpu, MemoryPercent: mem}

	// Build display model
	model := c.displayBld.Build(c.lastSnapshot, local, durations)
	c.lastModel = &model

	// Compute hash fingerprint
	hash := visualHash(model)
	return &CaptureState{Model: model, hash: hash}
}

func visualHash(m displaymodel.Model) string {
	var fp string
	for _, a := range m.Agents {
		fp += a.PaneID + "|" + a.Agent + "|" + string(a.Status) + "|"
		if a.Focused {
			fp += "1"
		} else {
			fp += "0"
		}
		fp += "|" + a.Name + "|" + a.ConnName + "|" + a.WsLabel + "|" + a.StatusDuration + "\n"
	}
	// Pad empty slots to ensure consistent hash length
	for i := len(m.Agents); i < 10; i++ {
		fp += "empty\n"
	}
	filt := "0"
	if m.NavAll.Filtered {
		filt = "1"
	}
	fp += "M" + itoa(int(m.Mode)) + "|" + filt + "\n"
	fp += "S" + itoa(m.Stats.AgentStats.Done) + itoa(m.Stats.AgentStats.Idle) +
		itoa(m.Stats.AgentStats.Working) + itoa(m.Stats.AgentStats.Blocked) + itoa(m.Stats.AgentStats.Unknown)
	cpuStr := fmt.Sprintf("%.1f", m.Stats.CPUPercent)
	memStr := fmt.Sprintf("%.1f", m.Stats.MemoryPercent)
	fp += "CPU" + cpuStr + "|MEM" + memStr
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
