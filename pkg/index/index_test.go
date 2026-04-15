package index_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
	"go.uber.org/zap"

	"github.com/spectra-ai/spectra/pkg/index"
	"github.com/spectra-ai/spectra/pkg/meta"
	"github.com/spectra-ai/spectra/pkg/model"
	"github.com/spectra-ai/spectra/pkg/segment"
	"github.com/spectra-ai/spectra/pkg/storage"
	"github.com/spectra-ai/spectra/pkg/wal"
)

func TestBloomFilterAddContains(t *testing.T) {
	bf := index.NewBloomFilter(100, 0.01)

	for i := 0; i < 100; i++ {
		bf.Add(fmt.Sprintf("key-%d", i))
	}

	// All inserted keys should be found
	for i := 0; i < 100; i++ {
		assert.True(t, bf.Contains(fmt.Sprintf("key-%d", i)), "key-%d should be found", i)
	}

	// Check false positive rate on non-inserted keys
	falsePositives := 0
	testCount := 10000
	for i := 1000; i < 1000+testCount; i++ {
		if bf.Contains(fmt.Sprintf("nonexistent-%d", i)) {
			falsePositives++
		}
	}

	fpRate := float64(falsePositives) / float64(testCount)
	assert.Less(t, fpRate, 0.05, "false positive rate should be under 5%%")
}

func TestBloomFilterEncodeDecode(t *testing.T) {
	bf := index.NewBloomFilter(50, 0.01)

	keys := []string{"trace-abc", "trace-def", "trace-ghi", "span-123", "span-456"}
	for _, k := range keys {
		bf.Add(k)
	}

	data, err := bf.Encode()
	require.NoError(t, err)

	decoded, err := index.DecodeBloomFilter(data)
	require.NoError(t, err)

	for _, k := range keys {
		assert.True(t, decoded.Contains(k), "%s should be found after decode", k)
	}
}

func TestBloomFilterFalsePositiveRate(t *testing.T) {
	n := 1000
	expectedFPRate := 0.01
	bf := index.NewBloomFilter(n, expectedFPRate)

	for i := 0; i < n; i++ {
		bf.Add(fmt.Sprintf("item-%d", i))
	}

	falsePositives := 0
	testCount := 10000
	for i := n; i < n+testCount; i++ {
		if bf.Contains(fmt.Sprintf("item-%d", i)) {
			falsePositives++
		}
	}

	actualFPRate := float64(falsePositives) / float64(testCount)
	// Allow 3x the expected rate as tolerance
	assert.Less(t, actualFPRate, expectedFPRate*3, "FP rate %.4f should be near %.4f", actualFPRate, expectedFPRate)
}

func TestIndexAndSearch(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewFS(t.TempDir())
	require.NoError(t, err)
	logger := zap.NewNop()

	// Create spans and WAL entries
	now := time.Now().UTC().Truncate(time.Millisecond)
	spans := []*model.Span{
		{SpanID: "s1", TraceID: "t1", Name: "llm_call", Kind: model.SpanKindLLM, StartTime: now, Input: "What is machine learning?", Output: "ML is a subset of AI"},
		{SpanID: "s2", TraceID: "t1", Name: "tool_use", Kind: model.SpanKindTool, StartTime: now.Add(time.Second), Input: "search database", Output: "found 10 results"},
		{SpanID: "s3", TraceID: "t2", Name: "retriever", Kind: model.SpanKindRetriever, StartTime: now.Add(2 * time.Second), Input: "error timeout", Output: "connection refused"},
	}

	var entries []*wal.Entry
	for _, sp := range spans {
		payload, _ := msgpack.Marshal(sp)
		entries = append(entries, &wal.Entry{TxnID: 1, TraceID: sp.TraceID, SpanID: sp.SpanID, Timestamp: sp.StartTime, Payload: payload})
	}

	// Build and store segment
	builder := segment.NewBuilder(50*1024*1024, 100000, logger)
	seg, err := builder.Build(entries)
	require.NoError(t, err)
	data, err := builder.Encode(seg)
	require.NoError(t, err)
	s3Key := fmt.Sprintf("segments/%s.seg", seg.ID)
	require.NoError(t, store.Put(ctx, s3Key, data))

	// Create in-memory meta and register segment
	metaStore := &testMeta{
		segments: map[string]meta.SegmentMeta{
			seg.ID: {SegmentID: seg.ID, S3Key: s3Key, MinTime: now, MaxTime: now.Add(2 * time.Second), SpanCount: 3, CreatedAt: now},
		},
	}

	// Index segment
	segReader := segment.NewReader(store, logger)
	indexer := index.NewIndexer(metaStore, segReader, store, time.Hour, logger)
	require.NoError(t, indexer.IndexOnce(ctx))

	// Search for text in input/output
	results, err := indexer.Search(ctx, "machine learning", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "s1", results[0].SpanID)

	// Search for error
	results, err = indexer.Search(ctx, "error timeout", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "s3", results[0].SpanID)
}

func TestSearchByTraceID(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewFS(t.TempDir())
	require.NoError(t, err)
	logger := zap.NewNop()

	now := time.Now().UTC().Truncate(time.Millisecond)
	spans := []*model.Span{
		{SpanID: "s1", TraceID: "t1", Name: "call1", Kind: model.SpanKindLLM, StartTime: now, Input: "a", Output: "b"},
		{SpanID: "s2", TraceID: "t1", Name: "call2", Kind: model.SpanKindLLM, StartTime: now.Add(time.Second), Input: "c", Output: "d"},
		{SpanID: "s3", TraceID: "t2", Name: "call3", Kind: model.SpanKindTool, StartTime: now.Add(2 * time.Second), Input: "e", Output: "f"},
	}

	var entries []*wal.Entry
	for _, sp := range spans {
		payload, _ := msgpack.Marshal(sp)
		entries = append(entries, &wal.Entry{TxnID: 1, TraceID: sp.TraceID, SpanID: sp.SpanID, Timestamp: sp.StartTime, Payload: payload})
	}

	builder := segment.NewBuilder(50*1024*1024, 100000, logger)
	seg, err := builder.Build(entries)
	require.NoError(t, err)
	data, err := builder.Encode(seg)
	require.NoError(t, err)
	s3Key := fmt.Sprintf("segments/%s.seg", seg.ID)
	require.NoError(t, store.Put(ctx, s3Key, data))

	metaStore := &testMeta{
		segments: map[string]meta.SegmentMeta{
			seg.ID: {SegmentID: seg.ID, S3Key: s3Key, MinTime: now, MaxTime: now.Add(2 * time.Second), SpanCount: 3, CreatedAt: now},
		},
	}

	segReader := segment.NewReader(store, logger)
	indexer := index.NewIndexer(metaStore, segReader, store, time.Hour, logger)
	require.NoError(t, indexer.IndexOnce(ctx))

	results, err := indexer.SearchByTraceID(ctx, "t1", 10)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

// Minimal test MetaStore
type testMeta struct {
	segments map[string]meta.SegmentMeta
}

func (m *testMeta) Migrate(_ context.Context) error                                             { return nil }
func (m *testMeta) RegisterSegment(_ context.Context, s meta.SegmentMeta) error                  { return nil }
func (m *testMeta) MapTraceToSegment(_ context.Context, _, _ string, _ int) error                { return nil }
func (m *testMeta) BatchMapTraces(_ context.Context, _ string, _ []meta.TraceMapping) error      { return nil }
func (m *testMeta) LookupTrace(_ context.Context, _ string) (*meta.TraceLocation, error)         { return nil, meta.ErrTraceNotFound }
func (m *testMeta) ListSegments(_ context.Context, _, _ time.Time) ([]meta.SegmentMeta, error)   { return nil, nil }
func (m *testMeta) ListUnprocessedWALFiles(_ context.Context) ([]meta.WALFileMeta, error)        { return nil, nil }
func (m *testMeta) RegisterWALFile(_ context.Context, _ meta.WALFileMeta) error                  { return nil }
func (m *testMeta) MarkWALProcessed(_ context.Context, _ string) error                           { return nil }
func (m *testMeta) Close()                                                                       {}

func (m *testMeta) ListUnindexedSegments(_ context.Context) ([]meta.SegmentMeta, error) {
	var result []meta.SegmentMeta
	for _, s := range m.segments {
		if !s.Indexed {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *testMeta) MarkSegmentIndexed(_ context.Context, segmentID string) error {
	s, ok := m.segments[segmentID]
	if ok {
		s.Indexed = true
		m.segments[segmentID] = s
	}
	return nil
}
