package appstate

import (
	"strings"
	"testing"
)

func TestKeyHashTracker_FirstCallReturnsTrue(t *testing.T) {
	tracker := NewKeyHashTracker()
	if !tracker.CheckAndUpdate(0, "<svg>hello</svg>") {
		t.Error("first call should return true")
	}
}

func TestKeyHashTracker_SameContentReturnsFalse(t *testing.T) {
	tracker := NewKeyHashTracker()
	svg := "<svg>hello</svg>"
	tracker.CheckAndUpdate(0, svg)
	if tracker.CheckAndUpdate(0, svg) {
		t.Error("same content should return false")
	}
}

func TestKeyHashTracker_DifferentContentReturnsTrue(t *testing.T) {
	tracker := NewKeyHashTracker()
	tracker.CheckAndUpdate(0, "<svg>hello</svg>")
	if !tracker.CheckAndUpdate(0, "<svg>world</svg>") {
		t.Error("different content should return true")
	}
}

func TestKeyHashTracker_MultipleKeys(t *testing.T) {
	tracker := NewKeyHashTracker()
	// First call for all 14 keys
	for i := 0; i < 14; i++ {
		if !tracker.CheckAndUpdate(i, "key-"+itoa(i)) {
			t.Errorf("first call for key %d should return true", i)
		}
	}
	// Second call — same SVG → all false
	for i := 0; i < 14; i++ {
		if tracker.CheckAndUpdate(i, "key-"+itoa(i)) {
			t.Errorf("second call for key %d should return false", i)
		}
	}
	// Change a single key
	if !tracker.CheckAndUpdate(5, "key-5-changed") {
		t.Error("changed key 5 should return true")
	}
	// Other keys still unchanged
	for i := 0; i < 14; i++ {
		if i == 5 {
			continue
		}
		if tracker.CheckAndUpdate(i, "key-"+itoa(i)) {
			t.Errorf("unchanged key %d should return false", i)
		}
	}
}

func TestKeyHashTracker_Reset(t *testing.T) {
	tracker := NewKeyHashTracker()
	tracker.CheckAndUpdate(0, "old")
	tracker.Reset()
	if !tracker.CheckAndUpdate(0, "old") {
		t.Error("after reset, even same content should return true")
	}
}

func TestKeyHashTracker_OutOfRange(t *testing.T) {
	tracker := NewKeyHashTracker()
	if !tracker.CheckAndUpdate(99, "anything") {
		t.Error("out-of-range index should return true")
	}
}

func TestKeyHashTracker_LargeSVG(t *testing.T) {
	tracker := NewKeyHashTracker()
	svg := strings.Repeat("ABCDEFGHIJ", 1000) // 10KB
	if !tracker.CheckAndUpdate(0, svg) {
		t.Error("first call should return true")
	}
	if tracker.CheckAndUpdate(0, svg) {
		t.Error("same large SVG should return false")
	}
	// Slight change should trigger
	svg2 := svg + " "
	if !tracker.CheckAndUpdate(0, svg2) {
		t.Error("different large SVG should return true")
	}
}


