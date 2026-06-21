package sysstats

import "testing"

func TestNewCollector(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
}

func TestCollect_ReturnsValues(t *testing.T) {
	c := New()
	// First call: CPU is 0 (no baseline), memory should be > 0
	s, err := c.Collect()
	if err != nil {
		t.Fatalf("first Collect: %v", err)
	}
	if s.CPUPercent != 0 {
		t.Logf("first call CPU = %.1f (expected 0, no baseline)", s.CPUPercent)
	}
	if s.MemoryPercent <= 0 || s.MemoryPercent > 100 {
		t.Errorf("memory percent out of range: %.1f", s.MemoryPercent)
	}
	t.Logf("first call: CPU=%.1f MEM=%.1f", s.CPUPercent, s.MemoryPercent)

	// Second call: should have real CPU value (> 0 or at least readable)
	s2, err := c.Collect()
	if err != nil {
		t.Fatalf("second Collect: %v", err)
	}
	t.Logf("second call: CPU=%.1f MEM=%.1f", s2.CPUPercent, s2.MemoryPercent)
	if s2.CPUPercent < 0 {
		t.Errorf("CPU percent negative: %.1f", s2.CPUPercent)
	}
	if s2.MemoryPercent <= 0 || s2.MemoryPercent > 100 {
		t.Errorf("memory percent out of range: %.1f", s2.MemoryPercent)
	}
}
