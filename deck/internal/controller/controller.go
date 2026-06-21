// Package controller manages the deck's render event loop.
//
// It owns the dirty flag, snapshot capture, hash compare, and renderAll()
// orchestration. It replaces the old appstate.Store.
package controller

import (
	"fmt"

	"github.com/herdr-deck/herdrdeck/deck/internal/fleet"
	"github.com/herdr-deck/herdrdeck/deck/internal/viewmodel"
	"github.com/herdr-deck/herdrdeck/protocol"
)

// State holds render-relevant state at a point in time.
type State struct {
	Mode          viewmodel.FilterMode
	ConnName      string
	WsLabel       string
	K11Filtered   bool
	Stats         protocol.AgentStats
	CPUPercent    float64
	MemoryPercent float64
	durationFP    string
	hash          string
}

// Controller manages the deck's render cycle.
type Controller struct {
	fleet   *fleet.Manager
	builder *viewmodel.Builder
	dirty   bool
}

// NewController creates a controller wrapping fleet manager and viewmodel builder.
func NewController(fm *fleet.Manager, bm *viewmodel.Builder) *Controller {
	return &Controller{
		fleet:   fm,
		builder: bm,
	}
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

// Capture reads the current fleet/viewmodel state and returns a State snapshot.
func (c *Controller) Capture() *State {
	cpu, mem := c.fleet.GetSysStats()
	agents := c.fleet.GetFilteredAgents(c.builder.ConnName, c.builder.WsLabel)

	var durFP string
	for _, a := range agents {
		d := c.fleet.FormatAgentDuration(a.ConnName, a.PaneID)
		durFP += a.PaneID + "=" + d + "|"
	}

	snap := &State{
		Mode:          c.builder.Mode,
		ConnName:      c.builder.ConnName,
		WsLabel:       c.builder.WsLabel,
		K11Filtered:   c.builder.K11Filtered,
		Stats:         c.fleet.ComputeStats(),
		CPUPercent:    cpu,
		MemoryPercent: mem,
		durationFP:    durFP,
	}
	snap.hash = visualHash(agents, snap)
	return snap
}

// ChangedSince returns true if the snapshot differs from a previous hash.
func (s *State) ChangedSince(prevHash string) bool {
	return s.hash != prevHash
}

// Hash returns the visual hash for dedup comparison.
func (s *State) Hash() string {
	return s.hash
}

func visualHash(agents []fleet.AgentInfo, s *State) string {
	var fp string
	for _, a := range agents {
		fp += a.PaneID + "|" + a.Agent + "|" + string(a.Status) + "|"
		if a.Focused {
			fp += "1"
		} else {
			fp += "0"
		}
		fp += "|" + a.Name + "|" + a.ConnName + "|" + a.WsLabel + "\n"
	}
	filt := "0"
	if s.K11Filtered {
		filt = "1"
	}
	fp += "M" + itoa(int(s.Mode)) + "|" + s.ConnName + "|" + s.WsLabel + "|" + filt + "\n"
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
