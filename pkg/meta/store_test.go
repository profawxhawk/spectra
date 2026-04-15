package meta_test

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/spectra-ai/spectra/pkg/meta"
)

// InMemoryStore implements MetaStore for testing without Postgres.
type InMemoryStore struct {
	mu       sync.RWMutex
	segments map[string]meta.SegmentMeta
	traces   map[string]meta.TraceLocation
	walFiles map[string]meta.WALFileMeta
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		segments: make(map[string]meta.SegmentMeta),
		traces:   make(map[string]meta.TraceLocation),
		walFiles: make(map[string]meta.WALFileMeta),
	}
}

func (s *InMemoryStore) Migrate(_ context.Context) error { return nil }

func (s *InMemoryStore) RegisterSegment(_ context.Context, seg meta.SegmentMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.segments[seg.SegmentID] = seg
	return nil
}

func (s *InMemoryStore) MapTraceToSegment(_ context.Context, traceID, segmentID string, offset int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	seg, ok := s.segments[segmentID]
	if !ok {
		return errors.New("segment not found")
	}
	s.traces[traceID] = meta.TraceLocation{
		SegmentID: segmentID,
		S3Key:     seg.S3Key,
		Offset:    offset,
	}
	return nil
}

func (s *InMemoryStore) BatchMapTraces(ctx context.Context, segmentID string, mappings []meta.TraceMapping) error {
	for _, m := range mappings {
		if err := s.MapTraceToSegment(ctx, m.TraceID, segmentID, m.Offset); err != nil {
			return err
		}
	}
	return nil
}

func (s *InMemoryStore) LookupTrace(_ context.Context, traceID string) (*meta.TraceLocation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	loc, ok := s.traces[traceID]
	if !ok {
		return nil, meta.ErrTraceNotFound
	}
	return &loc, nil
}

func (s *InMemoryStore) ListSegments(_ context.Context, from, to time.Time) ([]meta.SegmentMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []meta.SegmentMeta
	for _, seg := range s.segments {
		if seg.MinTime.Before(to) && seg.MaxTime.After(from) {
			result = append(result, seg)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].MinTime.Before(result[j].MinTime) })
	return result, nil
}

func (s *InMemoryStore) ListUnindexedSegments(_ context.Context) ([]meta.SegmentMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []meta.SegmentMeta
	for _, seg := range s.segments {
		if !seg.Indexed {
			result = append(result, seg)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (s *InMemoryStore) ListUnprocessedWALFiles(_ context.Context) ([]meta.WALFileMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []meta.WALFileMeta
	for _, w := range s.walFiles {
		if !w.Processed {
			result = append(result, w)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (s *InMemoryStore) RegisterWALFile(_ context.Context, m meta.WALFileMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.walFiles[m.Key] = m
	return nil
}

func (s *InMemoryStore) MarkWALProcessed(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, ok := s.walFiles[key]
	if !ok {
		return nil
	}
	w.Processed = true
	s.walFiles[key] = w
	return nil
}

func (s *InMemoryStore) MarkSegmentIndexed(_ context.Context, segmentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	seg, ok := s.segments[segmentID]
	if !ok {
		return nil
	}
	seg.Indexed = true
	s.segments[segmentID] = seg
	return nil
}

func (s *InMemoryStore) Close() {}

// --- Tests ---

func TestRegisterAndListSegments(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()

	now := time.Now().UTC()
	seg1 := meta.SegmentMeta{SegmentID: "seg-1", S3Key: "seg/1", MinTime: now.Add(-3 * time.Hour), MaxTime: now.Add(-2 * time.Hour), SpanCount: 100, CreatedAt: now}
	seg2 := meta.SegmentMeta{SegmentID: "seg-2", S3Key: "seg/2", MinTime: now.Add(-1 * time.Hour), MaxTime: now, SpanCount: 200, CreatedAt: now}
	seg3 := meta.SegmentMeta{SegmentID: "seg-3", S3Key: "seg/3", MinTime: now.Add(1 * time.Hour), MaxTime: now.Add(2 * time.Hour), SpanCount: 50, CreatedAt: now}

	require.NoError(t, store.RegisterSegment(ctx, seg1))
	require.NoError(t, store.RegisterSegment(ctx, seg2))
	require.NoError(t, store.RegisterSegment(ctx, seg3))

	// Query for segments overlapping the last 3 hours — should return seg1 and seg2
	results, err := store.ListSegments(ctx, now.Add(-3*time.Hour), now.Add(time.Minute))
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "seg-1", results[0].SegmentID)
	assert.Equal(t, "seg-2", results[1].SegmentID)
}

func TestMapAndLookupTrace(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()

	seg := meta.SegmentMeta{SegmentID: "seg-1", S3Key: "seg/1", MinTime: time.Now(), MaxTime: time.Now(), CreatedAt: time.Now()}
	require.NoError(t, store.RegisterSegment(ctx, seg))
	require.NoError(t, store.MapTraceToSegment(ctx, "trace-abc", "seg-1", 42))

	loc, err := store.LookupTrace(ctx, "trace-abc")
	require.NoError(t, err)
	assert.Equal(t, "seg-1", loc.SegmentID)
	assert.Equal(t, "seg/1", loc.S3Key)
	assert.Equal(t, 42, loc.Offset)
}

func TestLookupTraceNotFound(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()

	_, err := store.LookupTrace(ctx, "nonexistent")
	assert.ErrorIs(t, err, meta.ErrTraceNotFound)
}

func TestBatchMapTraces(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()

	seg := meta.SegmentMeta{SegmentID: "seg-1", S3Key: "seg/1", MinTime: time.Now(), MaxTime: time.Now(), CreatedAt: time.Now()}
	require.NoError(t, store.RegisterSegment(ctx, seg))

	mappings := []meta.TraceMapping{
		{TraceID: "t1", Offset: 0},
		{TraceID: "t2", Offset: 1},
		{TraceID: "t3", Offset: 2},
		{TraceID: "t4", Offset: 3},
		{TraceID: "t5", Offset: 4},
	}
	require.NoError(t, store.BatchMapTraces(ctx, "seg-1", mappings))

	for _, m := range mappings {
		loc, err := store.LookupTrace(ctx, m.TraceID)
		require.NoError(t, err)
		assert.Equal(t, m.Offset, loc.Offset)
	}
}

func TestListUnindexedSegments(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()

	now := time.Now().UTC()
	require.NoError(t, store.RegisterSegment(ctx, meta.SegmentMeta{SegmentID: "indexed-1", S3Key: "s/1", MinTime: now, MaxTime: now, Indexed: true, CreatedAt: now}))
	require.NoError(t, store.RegisterSegment(ctx, meta.SegmentMeta{SegmentID: "unindexed-1", S3Key: "s/2", MinTime: now, MaxTime: now, Indexed: false, CreatedAt: now}))
	require.NoError(t, store.RegisterSegment(ctx, meta.SegmentMeta{SegmentID: "unindexed-2", S3Key: "s/3", MinTime: now, MaxTime: now, Indexed: false, CreatedAt: now.Add(time.Second)}))

	result, err := store.ListUnindexedSegments(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "unindexed-1", result[0].SegmentID)
	assert.Equal(t, "unindexed-2", result[1].SegmentID)
}

func TestMarkSegmentIndexed(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()

	now := time.Now()
	require.NoError(t, store.RegisterSegment(ctx, meta.SegmentMeta{SegmentID: "seg-1", S3Key: "s/1", MinTime: now, MaxTime: now, Indexed: false, CreatedAt: now}))

	require.NoError(t, store.MarkSegmentIndexed(ctx, "seg-1"))

	result, err := store.ListUnindexedSegments(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestWALFileLifecycle(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()

	now := time.Now()
	require.NoError(t, store.RegisterWALFile(ctx, meta.WALFileMeta{Key: "wal/1", NodeID: "node-1", CreatedAt: now}))
	require.NoError(t, store.RegisterWALFile(ctx, meta.WALFileMeta{Key: "wal/2", NodeID: "node-1", CreatedAt: now.Add(time.Second)}))

	unprocessed, err := store.ListUnprocessedWALFiles(ctx)
	require.NoError(t, err)
	assert.Len(t, unprocessed, 2)

	require.NoError(t, store.MarkWALProcessed(ctx, "wal/1"))

	unprocessed, err = store.ListUnprocessedWALFiles(ctx)
	require.NoError(t, err)
	assert.Len(t, unprocessed, 1)
	assert.Equal(t, "wal/2", unprocessed[0].Key)
}
