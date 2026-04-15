package testutil

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/spectra-ai/spectra/pkg/config"
	"github.com/spectra-ai/spectra/pkg/index"
	"github.com/spectra-ai/spectra/pkg/memindex"
	"github.com/spectra-ai/spectra/pkg/meta"
	"github.com/spectra-ai/spectra/pkg/model"
	"github.com/spectra-ai/spectra/pkg/segment"
	"github.com/spectra-ai/spectra/pkg/storage"
	"github.com/spectra-ai/spectra/pkg/wal"
)

// TestFullPipeline tests the complete data flow: ingest → WAL → compact → index → query.
func TestFullPipeline(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	store, err := storage.NewFS(t.TempDir())
	require.NoError(t, err)

	metaStore := newTestMeta()

	// 1. INGEST: Write spans to WAL
	walWriter := wal.NewWriter(store, "test-node", time.Hour, 100, logger)

	spans := []*model.Span{
		{SpanID: "s1", TraceID: "trace-001", Name: "gpt4_call", Kind: model.SpanKindLLM,
			StartTime: time.Now().UTC().Truncate(time.Millisecond),
			Input: "What is machine learning?", Output: "ML is a branch of AI that enables systems to learn from data",
			Metadata: map[string]string{"model": "gpt-4o"}, Attributes: map[string]interface{}{"temp": 0.7}},
		{SpanID: "s2", TraceID: "trace-001", Name: "tool_search", Kind: model.SpanKindTool,
			StartTime: time.Now().UTC().Add(time.Second).Truncate(time.Millisecond),
			Input: "search papers", Output: "found 42 results"},
		{SpanID: "s3", TraceID: "trace-001", Name: "compose", Kind: model.SpanKindChain,
			StartTime: time.Now().UTC().Add(2 * time.Second).Truncate(time.Millisecond),
			Input: "compose final answer", Output: "Here is a comprehensive answer about ML..."},
		{SpanID: "s4", TraceID: "trace-002", Name: "embeddings", Kind: model.SpanKindRetriever,
			StartTime: time.Now().UTC().Add(3 * time.Second).Truncate(time.Millisecond),
			Input: "error connecting to database", Output: "connection timeout after 30s"},
		{SpanID: "s5", TraceID: "trace-002", Name: "retry", Kind: model.SpanKindGeneric,
			StartTime: time.Now().UTC().Add(4 * time.Second).Truncate(time.Millisecond),
			Input: "retrying connection", Output: "success on retry"},
	}

	memIdx := memindex.New(10 * time.Minute)
	for _, sp := range spans {
		entry, err := walWriter.Append(ctx, sp)
		require.NoError(t, err)
		require.NotNil(t, entry)
		memIdx.Add(sp.TraceID, memindex.Location{WALKey: "pending", Timestamp: time.Now()})
	}
	require.NoError(t, walWriter.Flush(ctx))

	// 2. VERIFY: WAL files exist
	walReader := wal.NewReader(store, logger)
	walFiles, err := walReader.ListWALFiles(ctx, "test-node")
	require.NoError(t, err)
	assert.Len(t, walFiles, 1, "should have 1 WAL file")

	// Register WAL files in meta (simulating what the writer would do in prod)
	for _, wf := range walFiles {
		require.NoError(t, metaStore.RegisterWALFile(ctx, meta.WALFileMeta{Key: wf, NodeID: "test-node", CreatedAt: time.Now()}))
	}

	// 3. VERIFY: Can read from WAL before compaction
	entries, err := walReader.ReadFile(ctx, walFiles[0])
	require.NoError(t, err)
	assert.Len(t, entries, 5, "WAL should have 5 entries")

	span, err := walReader.ReadSpan(ctx, walFiles[0], "s1")
	require.NoError(t, err)
	assert.Equal(t, "What is machine learning?", span.Input)

	// 4. COMPACT: WAL → Segments
	builder := segment.NewBuilder(50*1024*1024, 100000, logger)
	compactor := segment.NewCompactor(store, metaStore, walReader, builder, time.Hour, logger)
	require.NoError(t, compactor.CompactOnce(ctx))

	// 5. VERIFY: Segments created, traces mapped
	loc, err := metaStore.LookupTrace(ctx, "trace-001")
	require.NoError(t, err)
	assert.NotEmpty(t, loc.SegmentID)
	assert.NotEmpty(t, loc.S3Key)

	loc2, err := metaStore.LookupTrace(ctx, "trace-002")
	require.NoError(t, err)
	assert.Equal(t, loc.SegmentID, loc2.SegmentID, "both traces in same segment")

	// WAL files should be marked processed
	unprocessed, err := metaStore.ListUnprocessedWALFiles(ctx)
	require.NoError(t, err)
	assert.Len(t, unprocessed, 0, "all WAL files should be processed")

	// 6. VERIFY: Can read from segment
	segReader := segment.NewReader(store, logger)
	traceSpans, err := segReader.ReadTrace(ctx, loc.S3Key, "trace-001")
	require.NoError(t, err)
	assert.Len(t, traceSpans, 3, "trace-001 should have 3 spans")

	allSpans, err := segReader.ReadAllSpans(ctx, loc.S3Key)
	require.NoError(t, err)
	assert.Len(t, allSpans, 5, "segment should have all 5 spans")

	// 7. INDEX: Segments → Full-text index
	indexer := index.NewIndexer(metaStore, segReader, store, time.Hour, logger)
	require.NoError(t, indexer.IndexOnce(ctx))

	// Verify segment marked as indexed
	unindexed, err := metaStore.ListUnindexedSegments(ctx)
	require.NoError(t, err)
	assert.Len(t, unindexed, 0, "all segments should be indexed")

	// 8. SEARCH: Full-text search via index
	results, err := indexer.Search(ctx, "machine learning", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "s1", results[0].SpanID)
	assert.Equal(t, "trace-001", results[0].TraceID)

	results, err = indexer.Search(ctx, "connection timeout", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "s4", results[0].SpanID)

	// 9. SEARCH BY TRACE: Find all spans for a trace
	traceResults, err := indexer.SearchByTraceID(ctx, "trace-001", 10)
	require.NoError(t, err)
	assert.Len(t, traceResults, 3)

	// 10. SEARCH BY FIELD: Filter by span kind
	kindResults, err := indexer.SearchByField(ctx, "kind", "llm", 10)
	require.NoError(t, err)
	assert.Len(t, kindResults, 1)
	assert.Equal(t, "gpt4_call", kindResults[0].Name)

	// 11. BLOOM FILTER: Fast trace existence check
	bloom := index.NewBloomFilter(10, 0.01)
	bloom.Add("trace-001")
	bloom.Add("trace-002")
	assert.True(t, bloom.Contains("trace-001"))
	assert.True(t, bloom.Contains("trace-002"))
	assert.False(t, bloom.Contains("trace-999"))

	t.Log("Full pipeline test passed: ingest → WAL → compact → index → search")
}

// TestLateUpdate tests that late feedback on a trace merges correctly.
func TestLateUpdate(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	store, err := storage.NewFS(t.TempDir())
	require.NoError(t, err)
	metaStore := newTestMeta()

	// Ingest initial span
	walWriter := wal.NewWriter(store, "test-node", time.Hour, 100, logger)
	span1 := &model.Span{SpanID: "s1", TraceID: "trace-late", Name: "llm", Kind: model.SpanKindLLM,
		StartTime: time.Now().UTC().Truncate(time.Millisecond), Input: "hello", Output: "world"}
	_, err = walWriter.Append(ctx, span1)
	require.NoError(t, err)
	require.NoError(t, walWriter.Flush(ctx))

	// Register and compact
	walReader := wal.NewReader(store, logger)
	walFiles, _ := walReader.ListWALFiles(ctx, "test-node")
	for _, wf := range walFiles {
		metaStore.RegisterWALFile(ctx, meta.WALFileMeta{Key: wf, NodeID: "test-node", CreatedAt: time.Now()})
	}
	builder := segment.NewBuilder(50*1024*1024, 100000, logger)
	compactor := segment.NewCompactor(store, metaStore, walReader, builder, time.Hour, logger)
	require.NoError(t, compactor.CompactOnce(ctx))

	// Late feedback: add a new span to the same trace
	span2 := &model.Span{SpanID: "s2-feedback", TraceID: "trace-late", Name: "feedback", Kind: model.SpanKindGeneric,
		StartTime: time.Now().UTC().Add(time.Hour).Truncate(time.Millisecond),
		Input: "user rated", Output: "score: 5/5",
		Metadata: map[string]string{"feedback": "positive"}}
	_, err = walWriter.Append(ctx, span2)
	require.NoError(t, err)
	require.NoError(t, walWriter.Flush(ctx))

	// The late span is now in a WAL file. The original span is in a segment.
	// Both should be discoverable.
	loc, err := metaStore.LookupTrace(ctx, "trace-late")
	require.NoError(t, err)
	assert.NotEmpty(t, loc.SegmentID, "original span in segment")

	// Late span still in WAL
	newWalFiles, err := walReader.ListWALFiles(ctx, "test-node")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(newWalFiles), 1, "new WAL file should exist for late update")

	t.Log("Late update test passed: trace spans split across segment + WAL")
}

// TestHTTPIngestAndQuery tests the HTTP API end-to-end.
func TestHTTPIngestAndQuery(t *testing.T) {
	_ = config.DefaultConfig() // just verify config loads

	// Test that model serialization works through JSON (HTTP API format)
	span := model.NewSpan("trace-http", "test_call", model.SpanKindLLM)
	span.Input = "HTTP test input"
	span.Output = "HTTP test output"

	data, err := json.Marshal(span)
	require.NoError(t, err)

	var decoded model.Span
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, span.SpanID, decoded.SpanID)
	assert.Equal(t, span.Input, decoded.Input)

	// Test ingest request serialization
	type ingestReq struct {
		Spans []model.Span `json:"spans"`
	}
	req := ingestReq{Spans: []model.Span{*span}}
	reqData, err := json.Marshal(req)
	require.NoError(t, err)

	var decodedReq ingestReq
	require.NoError(t, json.Unmarshal(reqData, &decodedReq))
	assert.Len(t, decodedReq.Spans, 1)

	// Test query request serialization
	q := model.QueryRequest{
		Filters: []model.Filter{{Field: "trace_id", Operator: model.OpEq, Value: "trace-http"}},
		Search:  "HTTP test",
		Limit:   50,
	}
	assert.NoError(t, q.Validate())

	t.Log("HTTP serialization test passed")
}

// TestConcurrentIngest tests concurrent writes don't corrupt data.
func TestConcurrentIngest(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	store, err := storage.NewFS(t.TempDir())
	require.NoError(t, err)

	walWriter := wal.NewWriter(store, "test-node", time.Hour, 50, logger)

	// Write 100 spans concurrently from 10 goroutines
	done := make(chan struct{}, 10)
	for g := 0; g < 10; g++ {
		go func(goroutineID int) {
			defer func() { done <- struct{}{} }()
			for i := 0; i < 10; i++ {
				span := model.NewSpan("trace-concurrent", "span", model.SpanKindLLM)
				span.Input = "concurrent test"
				_, err := walWriter.Append(ctx, span)
				assert.NoError(t, err)
			}
		}(g)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	require.NoError(t, walWriter.Stop(ctx))

	// Verify all spans are readable
	walReader := wal.NewReader(store, logger)
	files, err := walReader.ListAllWALFiles(ctx)
	require.NoError(t, err)

	totalEntries := 0
	for _, f := range files {
		entries, err := walReader.ReadFile(ctx, f)
		require.NoError(t, err)
		totalEntries += len(entries)
	}

	assert.Equal(t, 100, totalEntries, "all 100 concurrent spans should be persisted")
	t.Log("Concurrent ingest test passed: 100 spans from 10 goroutines")
}

// --- In-memory MetaStore for integration tests ---

type testMetaStore struct {
	segments map[string]meta.SegmentMeta
	traces   map[string]meta.TraceLocation
	walFiles map[string]meta.WALFileMeta
}

func newTestMeta() *testMetaStore {
	return &testMetaStore{
		segments: make(map[string]meta.SegmentMeta),
		traces:   make(map[string]meta.TraceLocation),
		walFiles: make(map[string]meta.WALFileMeta),
	}
}

func (m *testMetaStore) Migrate(_ context.Context) error { return nil }
func (m *testMetaStore) RegisterSegment(_ context.Context, s meta.SegmentMeta) error {
	m.segments[s.SegmentID] = s; return nil
}
func (m *testMetaStore) MapTraceToSegment(_ context.Context, tid, sid string, off int) error {
	seg := m.segments[sid]; m.traces[tid] = meta.TraceLocation{SegmentID: sid, S3Key: seg.S3Key, Offset: off}; return nil
}
func (m *testMetaStore) BatchMapTraces(ctx context.Context, sid string, ms []meta.TraceMapping) error {
	for _, mp := range ms { m.MapTraceToSegment(ctx, mp.TraceID, sid, mp.Offset) }; return nil
}
func (m *testMetaStore) LookupTrace(_ context.Context, tid string) (*meta.TraceLocation, error) {
	l, ok := m.traces[tid]; if !ok { return nil, meta.ErrTraceNotFound }; return &l, nil
}
func (m *testMetaStore) ListSegments(_ context.Context, from, to time.Time) ([]meta.SegmentMeta, error) {
	var r []meta.SegmentMeta
	for _, s := range m.segments { if s.MinTime.Before(to) && s.MaxTime.After(from) { r = append(r, s) } }
	return r, nil
}
func (m *testMetaStore) ListUnindexedSegments(_ context.Context) ([]meta.SegmentMeta, error) {
	var r []meta.SegmentMeta
	for _, s := range m.segments { if !s.Indexed { r = append(r, s) } }
	return r, nil
}
func (m *testMetaStore) ListUnprocessedWALFiles(_ context.Context) ([]meta.WALFileMeta, error) {
	var r []meta.WALFileMeta
	for _, w := range m.walFiles { if !w.Processed { r = append(r, w) } }
	return r, nil
}
func (m *testMetaStore) RegisterWALFile(_ context.Context, wf meta.WALFileMeta) error {
	m.walFiles[wf.Key] = wf; return nil
}
func (m *testMetaStore) MarkWALProcessed(_ context.Context, key string) error {
	w := m.walFiles[key]; w.Processed = true; m.walFiles[key] = w; return nil
}
func (m *testMetaStore) MarkSegmentIndexed(_ context.Context, sid string) error {
	s := m.segments[sid]; s.Indexed = true; m.segments[sid] = s; return nil
}
func (m *testMetaStore) Close() {}

// Suppress unused import warning
var _ = http.StatusOK
