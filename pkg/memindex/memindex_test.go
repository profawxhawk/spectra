package memindex_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/spectra-ai/spectra/pkg/memindex"
)

func TestAddAndLookup(t *testing.T) {
	idx := memindex.New(10 * time.Minute)

	idx.Add("trace-1", memindex.Location{WALKey: "wal/n1/1.wal", Timestamp: time.Now()})
	idx.Add("trace-1", memindex.Location{WALKey: "wal/n1/2.wal", Timestamp: time.Now()})
	idx.Add("trace-2", memindex.Location{SegmentID: "seg-1", Offset: 0, Timestamp: time.Now()})

	locs, ok := idx.Lookup("trace-1")
	require.True(t, ok)
	assert.Len(t, locs, 2)
	assert.Equal(t, "wal/n1/1.wal", locs[0].WALKey)

	locs2, ok := idx.Lookup("trace-2")
	require.True(t, ok)
	assert.Len(t, locs2, 1)
	assert.Equal(t, "seg-1", locs2[0].SegmentID)

	assert.Equal(t, 2, idx.Len())
}

func TestLookupMiss(t *testing.T) {
	idx := memindex.New(10 * time.Minute)

	_, ok := idx.Lookup("nonexistent")
	assert.False(t, ok)
}

func TestRemove(t *testing.T) {
	idx := memindex.New(10 * time.Minute)

	idx.Add("trace-1", memindex.Location{WALKey: "wal/1.wal", Timestamp: time.Now()})
	assert.Equal(t, 1, idx.Len())

	idx.Remove("trace-1")
	assert.Equal(t, 0, idx.Len())

	_, ok := idx.Lookup("trace-1")
	assert.False(t, ok)
}

func TestEviction(t *testing.T) {
	idx := memindex.New(50 * time.Millisecond)

	idx.Add("old-trace", memindex.Location{WALKey: "wal/old.wal", Timestamp: time.Now()})
	assert.Equal(t, 1, idx.Len())

	time.Sleep(100 * time.Millisecond)

	// Add a fresh one
	idx.Add("new-trace", memindex.Location{WALKey: "wal/new.wal", Timestamp: time.Now()})

	idx.Evict()

	assert.Equal(t, 1, idx.Len())
	_, ok := idx.Lookup("old-trace")
	assert.False(t, ok)
	_, ok = idx.Lookup("new-trace")
	assert.True(t, ok)
}

func TestConcurrentAccess(t *testing.T) {
	idx := memindex.New(10 * time.Minute)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			traceID := "trace-concurrent"
			idx.Add(traceID, memindex.Location{
				WALKey:    "wal/concurrent.wal",
				Timestamp: time.Now(),
			})
			idx.Lookup(traceID)
			idx.Len()
		}(i)
	}

	wg.Wait()

	locs, ok := idx.Lookup("trace-concurrent")
	require.True(t, ok)
	assert.Len(t, locs, 10)
}
