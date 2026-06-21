// Package appstate provides the central state store for herdr-deck.
package deckclient

import (
	"hash/fnv"
)

// KeyHashTracker tracks per-key SVG hashes to avoid re-rendering keys whose
// visual content hasn't changed. The render loop generates SVG for all 14 keys
// (cheap fmt.Sprintf), but only sends SVG→PNG→WebSocket for keys whose hash
// changed since the last cycle.
//
// Usage: maintain one tracker across render cycles, indexed by the 0..13 loop
// order returned by mapper.RenderAll() (K1-K10 = 0..9, K11=10, K12=11,
// K13=12, K14=13).
type KeyHashTracker struct {
	hashes [14]uint64
}

// NewKeyHashTracker creates a tracker with all hashes zeroed (first render
// always passes through).
func NewKeyHashTracker() *KeyHashTracker {
	return &KeyHashTracker{}
}

// CheckAndUpdate checks whether a key's SVG hash changed since the last call.
// Returns true if the key needs re-render (first time or changed). On first
// call for an index the tracker stores the hash and returns true.
func (t *KeyHashTracker) CheckAndUpdate(idx int, svg string) bool {
	h := fnvHash(svg)
	if idx < 0 || idx >= len(t.hashes) {
		return true // out of range — let it through
	}
	if h == t.hashes[idx] {
		return false
	}
	t.hashes[idx] = h
	return true
}

// Reset clears all tracked hashes. Call after profile re-seed, reconnect,
// or any event that invalidates all key images.
func (t *KeyHashTracker) Reset() {
	t.hashes = [14]uint64{}
}

// fnvHash returns a 64-bit FNV-1a hash of a string.
func fnvHash(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// itoa converts an int to a decimal string. (Helper for tests.)
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
