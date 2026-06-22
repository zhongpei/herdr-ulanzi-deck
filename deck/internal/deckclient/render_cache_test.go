package deckclient

import (
	"testing"
)

func TestImageCache_GetByKey_Empty(t *testing.T) {
	c := NewImageCache()

	_, ok := c.GetByKey("0_2")
	if ok {
		t.Error("empty cache: GetByKey should return false")
	}

	_, ok = c.GetByKey("1_2")
	if ok {
		t.Error("empty cache: GetByKey for another key should return false")
	}

	_, ok = c.GetByKey("3_2")
	if ok {
		t.Error("empty cache: GetByKey for stats key should return false")
	}
}

func TestImageCache_GetByKey_Hit(t *testing.T) {
	c := NewImageCache()

	c.PutByKey("0_2", 42, "data:image/png;base64,abc")

	got, ok := c.GetByKey("0_2")
	if !ok {
		t.Fatal("GetByKey after Put should return true")
	}
	if got.SVGHash != 42 {
		t.Errorf("SVGHash = %d, want 42", got.SVGHash)
	}
	if got.DataURI != "data:image/png;base64,abc" {
		t.Errorf("DataURI = %q, want data:image/png;base64,abc", got.DataURI)
	}
}

func TestImageCache_GetByKey_MissDiffHash(t *testing.T) {
	c := NewImageCache()

	c.PutByKey("0_2", 42, "data:image/png;base64,abc")

	// Same key, different hash → should still "hit" (key exists), just hash mismatch
	got, ok := c.GetByKey("0_2")
	if !ok {
		t.Fatal("key exists, GetByKey should return true even if hash differs")
	}
	if got.SVGHash != 42 {
		t.Errorf("SVGHash = %d, want 42", got.SVGHash)
	}
}

func TestImageCache_GetByKey_MissDiffKey(t *testing.T) {
	c := NewImageCache()

	c.PutByKey("0_2", 42, "data:image/png;base64,abc")

	_, ok := c.GetByKey("1_2")
	if ok {
		t.Error("different key should return false")
	}
}

func TestImageCache_ClearByKey(t *testing.T) {
	c := NewImageCache()

	c.PutByKey("0_2", 42, "data:image/png;base64,abc")
	c.ClearByKey("0_2")

	_, ok := c.GetByKey("0_2")
	if ok {
		t.Error("ClearByKey should remove entry")
	}
}

func TestImageCache_LRU_Basic(t *testing.T) {
	c := NewImageCache()

	c.PutLRU(42, "data:image/png;base64,abc")

	got, ok := c.GetLRU(42)
	if !ok {
		t.Fatal("PutLRU then GetLRU should return true")
	}
	if got != "data:image/png;base64,abc" {
		t.Errorf("GetLRU = %q, want data:image/png;base64,abc", got)
	}
}

func TestImageCache_LRU_Miss(t *testing.T) {
	c := NewImageCache()

	_, ok := c.GetLRU(99)
	if ok {
		t.Error("empty LRU: GetLRU should return false")
	}
}

func TestImageCache_LRU_UpdateExisting(t *testing.T) {
	c := NewImageCache()

	c.PutLRU(42, "data:image/png;base64,old")
	c.PutLRU(42, "data:image/png;base64,new")

	got, ok := c.GetLRU(42)
	if !ok {
		t.Fatal("GetLRU after update should return true")
	}
	if got != "data:image/png;base64,new" {
		t.Errorf("after update, GetLRU = %q, want data:image/png;base64,new", got)
	}
}

func TestImageCache_LRU_ReuseAcrossKeys(t *testing.T) {
	c := NewImageCache()

	svgHash := uint64(42)
	dataURI := "data:image/png;base64,shared"

	// key1 renders first → goes into LRU
	c.PutLRU(svgHash, dataURI)

	// key2 checks LRU with same hash → hit
	got, ok := c.GetLRU(svgHash)
	if !ok {
		t.Fatal("same SVG hash should hit LRU from another key")
	}
	if got != dataURI {
		t.Errorf("LRU reuse: got %q, want %q", got, dataURI)
	}
}

func TestImageCache_LRU_Eviction(t *testing.T) {
	// Cap = 64, insert 65 unique entries → oldest evicted
	c := NewImageCache()

	for i := uint64(0); i < 65; i++ {
		c.PutLRU(i, "data:image/png;base64,entry")
	}

	// Entry 0 should be evicted
	_, ok := c.GetLRU(0)
	if ok {
		t.Error("LRU: entry 0 should have been evicted (oldest, cap=64, 65 inserted)")
	}

	// Entry 64 should exist (most recent)
	_, ok = c.GetLRU(64)
	if !ok {
		t.Error("LRU: entry 64 should exist (most recent)")
	}
}

func TestImageCache_LRU_GetPromotes(t *testing.T) {
	c := NewImageCache()

	// Insert 65 entries: cap=64, so 0 is evicted, 1-64 remain
	for i := uint64(0); i < 65; i++ {
		c.PutLRU(i, "data:image/png;base64,e")
	}

	// Entry 0 is gone
	// Access entry 1 → promotes it to MRU
	c.GetLRU(1)

	// Insert one more → should evict 2, not 1 (since 1 was promoted)
	c.PutLRU(100, "data:image/png;base64,new")

	_, ok := c.GetLRU(1)
	if !ok {
		t.Error("LRU: entry 1 should survive after Get promotes it")
	}

	_, ok = c.GetLRU(2)
	if ok {
		t.Error("LRU: entry 2 should be evicted (oldest non-promoted)")
	}
}

func TestImageCache_LRU_ConcurrentSafe(t *testing.T) {
	c := NewImageCache()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			c.PutLRU(uint64(i), "data:image/png;base64,a")
			c.GetLRU(uint64(i))
		}
		done <- struct{}{}
	}()
	go func() {
		for i := 0; i < 100; i++ {
			c.GetLRU(uint64(i))
			c.PutByKey("0_2", uint64(i), "data:image/png;base64,b")
			c.GetByKey("0_2")
		}
		done <- struct{}{}
	}()

	<-done
	<-done
}

func TestImageCache_Reset(t *testing.T) {
	c := NewImageCache()

	c.PutByKey("0_2", 42, "data:image/png;base64,abc")
	c.PutLRU(99, "data:image/png;base64,lru")
	c.Reset()

	_, ok := c.GetByKey("0_2")
	if ok {
		t.Error("after Reset, GetByKey should return false")
	}

	_, lruOK := c.GetLRU(99)
	if lruOK {
		t.Error("after Reset, LRU should be empty")
	}
}
