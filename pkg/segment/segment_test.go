package segment_test

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
	"go.uber.org/zap"

	"github.com/spectra-ai/spectra/pkg/meta"
	"github.com/spectra-ai/spectra/pkg/model"
	"github.com/spectra-ai/spectra/pkg/segment"
	"github.com/spectra-ai/spectra/pkg/storage"
	"github.com/spectra-ai/spectra/pkg/wal"
)

func makeEntry(traceID, spanID string, ts time.Time) *wal.Entry {
	span := &model.Span{SpanID: spanID, TraceID: traceID, Name: "test", Kind: model.SpanKindLLM, StartTime: ts, Input: "hello", Output: "world"}
	payload, _ := msgpack.Marshal(span)
	return &wal.Entry{TxnID: 1, TraceID: traceID, SpanID: spanID, Timestamp: ts, Payload: payload}
}

func TestBuildSegment(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	entries := []*wal.Entry{
		makeEntry("t1", "s1", now),
		makeEntry("t1", "s2", now.Add(time.Second)),
		makeEntry("t2", "s3", now.Add(2*time.Second)),
		makeEntry("t2", "s4", now.Add(3*time.Second)),
		makeEntry("t3", "s5", now.Add(4*time.Second)),
	}

	b := segment.NewBuilder(50*1024*1024, 100000, zap.NewNop())
	seg, err := b.Build(entries)
	require.NoError(t, err)

	assert.NotEmpty(t, seg.ID)
	assert.Equal(t, 5, seg.SpanCount)
	assert.Len(t, seg.Traces, 3)
	assert.Len(t, seg.Traces["t1"], 2)
	assert.Len(t, seg.Traces["t2"], 2)
	assert.Len(t, seg.Traces["t3"], 1)
	assert.True(t, seg.MinTime.Equal(now))
	assert.True(t, seg.MaxTime.Equal(now.Add(4*time.Second)))
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	entries := []*wal.Entry{
		makeEntry("t1", "s1", now),
		makeEntry("t2", "s2", now.Add(time.Second)),
	}

	b := segment.NewBuilder(50*1024*1024, 100000, zap.NewNop())
	seg, err := b.Build(entries)
	require.NoError(t, err)

	data, err := b.Encode(seg)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	decoded, err := segment.Decode(data)
	require.NoError(t, err)

	assert.Equal(t, seg.ID, decoded.ID)
	assert.Equal(t, seg.SpanCount, decoded.SpanCount)
	assert.Len(t, decoded.Traces, 2)
	assert.True(t, seg.MinTime.Equal(decoded.MinTime))
}

func TestTraceOffsets(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	entries := []*wal.Entry{
		makeEntry("trace-b", "s1", now),
		makeEntry("trace-a", "s2", now.Add(time.Second)),
		makeEntry("trace-c", "s3", now.Add(2*time.Second)),
	}

	b := segment.NewBuilder(50*1024*1024, 100000, zap.NewNop())
	seg, err := b.Build(entries)
	require.NoError(t, err)

	offsets := segment.TraceOffsets(seg)
	assert.Len(t, offsets, 3)

	// Should be sorted by trace ID
	assert.Equal(t, "trace-a", offsets[0].TraceID)
	assert.Equal(t, "trace-b", offsets[1].TraceID)
	assert.Equal(t, "trace-c", offsets[2].TraceID)
}

func TestReaderReadSegment(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewFS(t.TempDir())
	require.NoError(t, err)
	logger := zap.NewNop()

	now := time.Now().UTC().Truncate(time.Millisecond)
	entries := []*wal.Entry{
		makeEntry("t1", "s1", now),
		makeEntry("t1", "s2", now.Add(time.Second)),
	}

	b := segment.NewBuilder(50*1024*1024, 100000, logger)
	seg, err := b.Build(entries)
	require.NoError(t, err)

	data, err := b.Encode(seg)
	require.NoError(t, err)

	key := "segments/test.seg"
	require.NoError(t, store.Put(ctx, key, data))

	reader := segment.NewReader(store, logger)
	read, err := reader.ReadSegment(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, seg.ID, read.ID)
	assert.Equal(t, 2, read.SpanCount)
}

func TestReaderReadTrace(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewFS(t.TempDir())
	require.NoError(t, err)
	logger := zap.NewNop()

	now := time.Now().UTC().Truncate(time.Millisecond)
	entries := []*wal.Entry{
		makeEntry("t1", "s1", now),
		makeEntry("t1", "s2", now.Add(time.Second)),
		makeEntry("t2", "s3", now.Add(2*time.Second)),
	}

	b := segment.NewBuilder(50*1024*1024, 100000, logger)
	seg, err := b.Build(entries)
	require.NoError(t, err)

	data, err := b.Encode(seg)
	require.NoError(t, err)

	key := "segments/test.seg"
	require.NoError(t, store.Put(ctx, key, data))

	reader := segment.NewReader(store, logger)
	spans, err := reader.ReadTrace(ctx, key, "t1")
	require.NoError(t, err)
	assert.Len(t, spans, 2)
	assert.Equal(t, "s1", spans[0].SpanID)
	assert.Equal(t, "s2", spans[1].SpanID)
}

// --- In-memory MetaStore for compactor tests ---

type inMemoryMeta struct {
	mu       sync.RWMutex
	segments map[string]meta.SegmentMeta
	traces   map[string]meta.TraceLocation
	walFiles map[string]meta.WALFileMeta
}

func newInMemoryMeta() *inMemoryMeta {
	return &inMemoryMeta{
		segments: make(map[string]meta.SegmentMeta),
		traces:   make(map[string]meta.TraceLocation),
		walFiles: make(map[string]meta.WALFileMeta),
	}
}

func (m *inMemoryMeta) Migrate(_ context.Context) error { return nil }

func (m *inMemoryMeta) RegisterSegment(_ context.Context, seg meta.SegmentMeta) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.segments[seg.SegmentID] = seg
	return nil
}

func (m *inMemoryMeta) MapTraceToSegment(_ context.Context, traceID, segmentID string, offset int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	seg := m.segments[segmentID]
	m.traces[traceID] = meta.TraceLocation{SegmentID: segmentID, S3Key: seg.S3Key, Offset: offset}
	return nil
}

func (m *inMemoryMeta) BatchMapTraces(ctx context.Context, segmentID string, mappings []meta.TraceMapping) error {
	for _, mp := range mappings {
		if err := m.MapTraceToSegment(ctx, mp.TraceID, segmentID, mp.Offset); err != nil {
			return err
		}
	}
	return nil
}

func (m *inMemoryMeta) LookupTrace(_ context.Context, traceID string) (*meta.TraceLocation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	loc, ok := m.traces[traceID]
	if !ok {
		return nil, meta.ErrTraceNotFound
	}
	return &loc, nil
}

func (m *inMemoryMeta) ListSegments(_ context.Context, from, to time.Time) ([]meta.SegmentMeta, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []meta.SegmentMeta
	for _, s := range m.segments {
		if s.MinTime.Before(to) && s.MaxTime.After(from) {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *inMemoryMeta) ListUnindexedSegments(_ context.Context) ([]meta.SegmentMeta, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []meta.SegmentMeta
	for _, s := range m.segments {
		if !s.Indexed {
			result = append(result, s)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (m *inMemoryMeta) ListUnprocessedWALFiles(_ context.Context) ([]meta.WALFileMeta, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []meta.WALFileMeta
	for _, w := range m.walFiles {
		if !w.Processed {
			result = append(result, w)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (m *inMemoryMeta) RegisterWALFile(_ context.Context, wf meta.WALFileMeta) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.walFiles[wf.Key] = wf
	return nil
}

func (m *inMemoryMeta) MarkWALProcessed(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.walFiles[key]
	if !ok {
		return errors.New("WAL file not found")
	}
	w.Processed = true
	m.walFiles[key] = w
	return nil
}

func (m *inMemoryMeta) MarkSegmentIndexed(_ context.Context, segmentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.segments[segmentID]
	if !ok {
		return nil
	}
	s.Indexed = true
	m.segments[segmentID] = s
	return nil
}

func (m *inMemoryMeta) Close() {}

func TestCompactOnce(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewFS(t.TempDir())
	require.NoError(t, err)
	logger := zap.NewNop()
	metaStore := newInMemoryMeta()

	now := time.Now().UTC().Truncate(time.Millisecond)

	// Write 2 WAL files manually
	entries1 := []*wal.Entry{makeEntry("t1", "s1", now), makeEntry("t1", "s2", now.Add(time.Second))}
	entries2 := []*wal.Entry{makeEntry("t2", "s3", now.Add(2 * time.Second))}

	data1, err := wal.EncodeEntries(entries1)
	require.NoError(t, err)
	data2, err := wal.EncodeEntries(entries2)
	require.NoError(t, err)

	require.NoError(t, store.Put(ctx, "wal/node-1/001.wal", data1))
	require.NoError(t, store.Put(ctx, "wal/node-1/002.wal", data2))

	// Register WAL files in meta
	require.NoError(t, metaStore.RegisterWALFile(ctx, meta.WALFileMeta{Key: "wal/node-1/001.wal", NodeID: "node-1", CreatedAt: now}))
	require.NoError(t, metaStore.RegisterWALFile(ctx, meta.WALFileMeta{Key: "wal/node-1/002.wal", NodeID: "node-1", CreatedAt: now}))

	// Run compaction
	walReader := wal.NewReader(store, logger)
	builder := segment.NewBuilder(50*1024*1024, 100000, logger)
	compactor := segment.NewCompactor(store, metaStore, walReader, builder, time.Hour, logger)

	require.NoError(t, compactor.CompactOnce(ctx))

	// Verify: traces are mapped
	loc1, err := metaStore.LookupTrace(ctx, "t1")
	require.NoError(t, err)
	assert.NotEmpty(t, loc1.SegmentID)

	loc2, err := metaStore.LookupTrace(ctx, "t2")
	require.NoError(t, err)
	assert.Equal(t, loc1.SegmentID, loc2.SegmentID) // same segment

	// Verify: WAL files marked processed
	unprocessed, err := metaStore.ListUnprocessedWALFiles(ctx)
	require.NoError(t, err)
	assert.Len(t, unprocessed, 0)

	// Verify: segment exists in store
	exists, err := store.Exists(ctx, loc1.S3Key)
	require.NoError(t, err)
	assert.True(t, exists)

	// Verify: can read spans from segment
	segReader := segment.NewReader(store, logger)
	spans, err := segReader.ReadAllSpans(ctx, loc1.S3Key)
	require.NoError(t, err)
	assert.Len(t, spans, 3)
}
