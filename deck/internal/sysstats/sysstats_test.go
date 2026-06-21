package sysstats

import (
	"testing"
	"time"
)

func TestNewCollector(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	// Allow background goroutine to start
	time.Sleep(10 * time.Millisecond)
}

func TestCollect_MemoryWorks(t *testing.T) {
	c := New()
	s, err := c.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if s.MemoryPercent <= 0 || s.MemoryPercent > 100 {
		t.Errorf("memory out of range: %.1f", s.MemoryPercent)
	}
	// CPU is 0 until background goroutine completes first sample (~3s)
	if s.CPUPercent < 0 || s.CPUPercent > 100 {
		t.Errorf("CPU out of range: %.1f", s.CPUPercent)
	}
	t.Logf("CPU=%.1f MEM=%.1f", s.CPUPercent, s.MemoryPercent)
}
