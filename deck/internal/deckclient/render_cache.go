package deckclient

import (
	"sync"
)

// CachedImage holds the rendered PNG data URI and its SVG hash fingerprint.
type CachedImage struct {
	SVGHash uint64
	DataURI string
}

// ImageCache provides two-level caching for SetKeyImage:
//  1. latestByKey — each physical key's last rendered result
//  2. LRU        — small hash→dataURI cache for cross-key reuse
//
// The LRU has a fixed capacity (default 64). Oldest entries are evicted
// when the cache is full.
type ImageCache struct {
	mu          sync.RWMutex
	latestByKey map[string]CachedImage
	lru         *lruCache
}

// NewImageCache creates an ImageCache with default LRU capacity (64).
func NewImageCache() *ImageCache {
	return &ImageCache{
		latestByKey: make(map[string]CachedImage),
		lru:         newLRUCache(64),
	}
}

// GetByKey returns the cached result for a physical key.
// Returns false if the key has no cached entry.
func (ic *ImageCache) GetByKey(key string) (CachedImage, bool) {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	v, ok := ic.latestByKey[key]
	return v, ok
}

// PutByKey stores a rendered result for a physical key.
func (ic *ImageCache) PutByKey(key string, hash uint64, dataURI string) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	ic.latestByKey[key] = CachedImage{SVGHash: hash, DataURI: dataURI}
}

// GetLRU returns a cached PNG dataURI by SVG hash.
func (ic *ImageCache) GetLRU(hash uint64) (string, bool) {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	return ic.lru.get(hash)
}

// PutLRU stores a PNG dataURI by SVG hash in the LRU cache.
func (ic *ImageCache) PutLRU(hash uint64, dataURI string) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	ic.lru.put(hash, dataURI)
}

// ClearByKey removes the cached entry for a single physical key.
func (ic *ImageCache) ClearByKey(key string) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	delete(ic.latestByKey, key)
}

// Reset clears all cached entries (both by-key and LRU).
func (ic *ImageCache) Reset() {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	clear(ic.latestByKey)
	ic.lru = newLRUCache(64)
}

// lruCache is a simple LRU with a fixed capacity.
// Thread-safe: uses its own mutex.
// Internal — used by ImageCache only.
type lruCache struct {
	mu    sync.Mutex
	cap   int
	items map[uint64]string
	order []uint64 // FIFO-ordered keys for eviction
}

func newLRUCache(cap int) *lruCache {
	return &lruCache{
		cap:   cap,
		items: make(map[uint64]string, cap),
		order: make([]uint64, 0, cap),
	}
}

func (l *lruCache) get(hash uint64) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	v, ok := l.items[hash]
	if ok {
		l.promoteLocked(hash)
	}
	return v, ok
}

func (l *lruCache) put(hash uint64, dataURI string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, exists := l.items[hash]; exists {
		l.items[hash] = dataURI
		l.promoteLocked(hash)
		return
	}
	if len(l.items) >= l.cap {
		l.evictLocked()
	}
	l.items[hash] = dataURI
	l.order = append(l.order, hash)
}

func (l *lruCache) promoteLocked(hash uint64) {
	for i, h := range l.order {
		if h == hash {
			l.order = append(l.order[:i], l.order[i+1:]...)
			l.order = append(l.order, hash)
			return
		}
	}
}

func (l *lruCache) evictLocked() {
	if len(l.order) == 0 {
		return
	}
	oldest := l.order[0]
	delete(l.items, oldest)
	l.order = l.order[1:]
}
