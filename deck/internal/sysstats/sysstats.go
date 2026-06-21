// Package sysstats collects system CPU and memory statistics.
// CPU is collected by a background goroutine using cpu.Percent which
// samples over a real time window (requires CGO for cross-platform support).
package sysstats

import (
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// Stats holds CPU and memory usage percentages.
type Stats struct {
	CPUPercent    float64 // 0-100, background-updated every 3s
	MemoryPercent float64 // 0-100, instantaneous
}

// Collector reads system stats. CPU runs in background goroutine.
type Collector struct {
	cpu atomic.Uint32 // centipercent ×100 for atomic storage
}

// New starts background CPU collector.
func New() *Collector {
	c := &Collector{}
	go c.run()
	return c
}

func (c *Collector) run() {
	for {
		// cpu.Percent with a real interval uses a different code path
		// than cpu.Times and works when CGO is enabled.
		pcts, _ := cpu.Percent(3*time.Second, false)
		if len(pcts) > 0 {
			c.cpu.Store(uint32(pcts[0] * 100))
		}
	}
}

// Collect returns latest stats. First ~3s CPU=0 (renders as "--").
func (c *Collector) Collect() (*Stats, error) {
	m, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	return &Stats{
		CPUPercent:    float64(c.cpu.Load()) / 100.0,
		MemoryPercent: m.UsedPercent,
	}, nil
}
