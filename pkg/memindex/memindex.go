package memindex

import (
	"context"
	"sync"
	"time"
)

// Location describes where a trace's data can be found.
type Location struct {
	WALKey    string    // WAL file key (before compaction)
	SegmentID string    // segment ID (after compaction)
	Offset    int       // offset within segment
	Timestamp time.Time // when this entry was added
}

// MemIndex is a thread-safe in-memory index mapping trace IDs to their locations.
// It provides fast lookups for recently ingested data before compaction.
type MemIndex struct {
	mu      sync.RWMutex
	entries map[string][]Location
	ttl     time.Duration
}

// New creates a new MemIndex with the given TTL for eviction.
func New(ttl time.Duration) *MemIndex {
	return &MemIndex{
		entries: make(map[string][]Location),
		ttl:     ttl,
	}
}

// Add records a location for a trace.
func (m *MemIndex) Add(traceID string, loc Location) {
	if loc.Timestamp.IsZero() {
		loc.Timestamp = time.Now()
	}
	m.mu.Lock()
	m.entries[traceID] = append(m.entries[traceID], loc)
	m.mu.Unlock()
}

// Lookup returns all known locations for a trace. Returns a copy to prevent races.
func (m *MemIndex) Lookup(traceID string) ([]Location, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	locs, ok := m.entries[traceID]
	if !ok || len(locs) == 0 {
		return nil, false
	}
	result := make([]Location, len(locs))
	copy(result, locs)
	return result, true
}

// Remove deletes all locations for a trace.
func (m *MemIndex) Remove(traceID string) {
	m.mu.Lock()
	delete(m.entries, traceID)
	m.mu.Unlock()
}

// Evict removes locations older than the configured TTL.
// If all locations for a trace are evicted, the trace is removed entirely.
func (m *MemIndex) Evict() {
	cutoff := time.Now().Add(-m.ttl)
	m.mu.Lock()
	defer m.mu.Unlock()

	for traceID, locs := range m.entries {
		kept := locs[:0]
		for _, loc := range locs {
			if loc.Timestamp.After(cutoff) {
				kept = append(kept, loc)
			}
		}
		if len(kept) == 0 {
			delete(m.entries, traceID)
		} else {
			m.entries[traceID] = kept
		}
	}
}

// Len returns the number of traces currently tracked.
func (m *MemIndex) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

// StartEviction runs periodic eviction in a background goroutine.
func (m *MemIndex) StartEviction(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.Evict()
		}
	}
}
