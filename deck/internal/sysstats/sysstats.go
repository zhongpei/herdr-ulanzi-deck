// Package sysstats collects system CPU and memory statistics.
// Uses gopsutil internally; works on macOS and Linux.
package sysstats

import (
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// Stats holds CPU and memory usage percentages.
type Stats struct {
	CPUPercent    float64 // 0-100, instantaneous since last Collect call
	MemoryPercent float64 // 0-100, instantaneous
}

// Collector reads system stats and computes CPU delta on each call.
// Instant/non-blocking — reads /proc or sysctl directly, no sleep.
type Collector struct {
	prev *cpu.TimesStat
}

// New creates a Collector. The first Collect() returns zero CPU (baseline).
func New() *Collector {
	return &Collector{}
}

// Collect reads current system stats: CPU% (delta from previous call)
// and memory%. First call after New() returns 0 CPU (baseline).
func (c *Collector) Collect() (*Stats, error) {
	memStats, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	times, err := cpu.Times(false)
	if err != nil {
		return nil, err
	}

	cpuPct := 0.0
	if c.prev != nil {
		prev := *c.prev
		curr := times[0]

		totalDelta := (curr.User + curr.System + curr.Idle + curr.Iowait +
			curr.Irq + curr.Softirq + curr.Steal + curr.Guest + curr.GuestNice) -
			(prev.User + prev.System + prev.Idle + prev.Iowait +
				prev.Irq + prev.Softirq + prev.Steal + prev.Guest + prev.GuestNice)
		idleDelta := curr.Idle - prev.Idle

		if totalDelta > 0 {
			cpuPct = (1.0 - idleDelta/totalDelta) * 100.0
		}
	}

	// Save current times for next delta
	cp := times[0]
	c.prev = &cp

	return &Stats{
		CPUPercent:    cpuPct,
		MemoryPercent: memStats.UsedPercent,
	}, nil
}
