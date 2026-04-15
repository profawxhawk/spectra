package query_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
	"go.uber.org/zap"

	"github.com/spectra-ai/spectra/pkg/index"
	"github.com/spectra-ai/spectra/pkg/memindex"
	"github.com/spectra-ai/spectra/pkg/meta"
	"github.com/spectra-ai/spectra/pkg/model"
	"github.com/spectra-ai/spectra/pkg/query"
	"github.com/spectra-ai/spectra/pkg/segment"
	"github.com/spectra-ai/spectra/pkg/storage"
	"github.com/spectra-ai/spectra/pkg/wal"
)

func setupTestEnv(t *testing.T) (storage.ObjectStore, *wal.Reader, *segment.Reader, *index.Indexer, *memindex.MemIndex, meta.MetaStore) {
	t.Helper()
	store, err := storage.NewFS(t.TempDir())
	require.NoError(t, err)
	logger := zap.NewNop()

	walReader := wal.NewReader(store, logger)
	segReader := segment.NewReader(store, logger)
	memIdx := memindex.New(10 * time.Minute)
	metaStore := newQueryTestMeta()
	indexer := index.NewIndexer(metaStore, segReader, store, time.Hour, logger)

	return store, walReader, segReader, indexer, memIdx, metaStore
}

func ingestAndCompact(t *testing.T, store storage.ObjectStore, metaStore meta.MetaStore, spans []*model.Span) {
	t.Helper()
	logger := zap.NewNop()

	now := time.Now().UTC().Truncate(time.Millisecond)
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

	s3Key := "segments/" + seg.ID + ".seg"
	require.NoError(t, store.Put(context.Background(), s3Key, data))

	require.NoError(t, metaStore.RegisterSegment(context.Background(), meta.SegmentMeta{
		SegmentID: seg.ID, S3Key: s3Key, MinTime: now, MaxTime: now.Add(time.Minute), SpanCount: len(spans), CreatedAt: now,
	}))

	offsets := segment.TraceOffsets(seg)
	require.NoError(t, metaStore.BatchMapTraces(context.Background(), seg.ID, offsets))
}

func TestPlannerTraceIDLookup(t *testing.T) {
	store, walReader, segReader, indexer, memIdx, metaStore := setupTestEnv(t)

	spans := []*model.Span{
		{SpanID: "s1", TraceID: "trace-abc", Name: "llm", Kind: model.SpanKindLLM, StartTime: time.Now().UTC().Truncate(time.Millisecond), Input: "hello", Output: "world"},
		{SpanID: "s2", TraceID: "trace-abc", Name: "tool", Kind: model.SpanKindTool, StartTime: time.Now().UTC().Truncate(time.Millisecond), Input: "search", Output: "found"},
		{SpanID: "s3", TraceID: "trace-def", Name: "chain", Kind: model.SpanKindChain, StartTime: time.Now().UTC().Truncate(time.Millisecond), Input: "x", Output: "y"},
	}

	ingestAndCompact(t, store, metaStore, spans)

	planner := query.NewPlanner(memIdx, walReader, metaStore, segReader, indexer)
	q := model.QueryRequest{
		Filters: []model.Filter{{Field: "trace_id", Operator: model.OpEq, Value: "trace-abc"}},
		Limit:   50,
	}

	plan := planner.CreatePlan(q)
	assert.Equal(t, "trace-abc", plan.TraceID)

	result, err := planner.Execute(context.Background(), plan)
	require.NoError(t, err)
	assert.Len(t, result.Spans, 2)
	for _, s := range result.Spans {
		assert.Equal(t, "trace-abc", s.TraceID)
	}
}

func TestPlannerFilterByField(t *testing.T) {
	store, walReader, segReader, indexer, memIdx, metaStore := setupTestEnv(t)

	spans := []*model.Span{
		{SpanID: "s1", TraceID: "t1", Name: "llm_call", Kind: model.SpanKindLLM, StartTime: time.Now().UTC().Truncate(time.Millisecond), Status: model.SpanStatusOK},
		{SpanID: "s2", TraceID: "t1", Name: "tool_call", Kind: model.SpanKindTool, StartTime: time.Now().UTC().Truncate(time.Millisecond), Status: model.SpanStatusError},
		{SpanID: "s3", TraceID: "t2", Name: "llm_call", Kind: model.SpanKindLLM, StartTime: time.Now().UTC().Truncate(time.Millisecond), Status: model.SpanStatusOK},
	}

	ingestAndCompact(t, store, metaStore, spans)

	// Index the segments
	require.NoError(t, indexer.IndexOnce(context.Background()))

	planner := query.NewPlanner(memIdx, walReader, metaStore, segReader, indexer)

	// Search all, filter by kind=llm
	q := model.QueryRequest{
		Filters: []model.Filter{{Field: "kind", Operator: model.OpEq, Value: "llm"}},
		Search:  "llm_call", // need search or filter
		Limit:   50,
	}

	plan := planner.CreatePlan(q)
	result, err := planner.Execute(context.Background(), plan)
	require.NoError(t, err)
	assert.Len(t, result.Spans, 2) // s1 and s3 are both llm_call with kind=llm
}

func TestPlannerPagination(t *testing.T) {
	store, walReader, segReader, indexer, memIdx, metaStore := setupTestEnv(t)

	var spans []*model.Span
	for i := 0; i < 10; i++ {
		spans = append(spans, &model.Span{
			SpanID: "s" + string(rune('0'+i)), TraceID: "t1", Name: "call",
			Kind: model.SpanKindLLM, StartTime: time.Now().UTC().Truncate(time.Millisecond),
			Input: "test input", Output: "test output",
		})
	}

	ingestAndCompact(t, store, metaStore, spans)
	require.NoError(t, indexer.IndexOnce(context.Background()))

	planner := query.NewPlanner(memIdx, walReader, metaStore, segReader, indexer)

	q := model.QueryRequest{
		Search: "test input",
		Limit:  3,
		Offset: 0,
	}

	plan := planner.CreatePlan(q)
	result, err := planner.Execute(context.Background(), plan)
	require.NoError(t, err)
	assert.Len(t, result.Spans, 3)
	assert.Equal(t, 10, result.TotalCount)
	assert.True(t, result.HasMore)
}

// Minimal in-memory MetaStore for query tests
type queryTestMeta struct {
	segments map[string]meta.SegmentMeta
	traces   map[string]meta.TraceLocation
}

func newQueryTestMeta() *queryTestMeta {
	return &queryTestMeta{segments: make(map[string]meta.SegmentMeta), traces: make(map[string]meta.TraceLocation)}
}

func (m *queryTestMeta) Migrate(_ context.Context) error { return nil }
func (m *queryTestMeta) RegisterSegment(_ context.Context, s meta.SegmentMeta) error {
	m.segments[s.SegmentID] = s; return nil
}
func (m *queryTestMeta) MapTraceToSegment(_ context.Context, tid, sid string, off int) error {
	seg := m.segments[sid]; m.traces[tid] = meta.TraceLocation{SegmentID: sid, S3Key: seg.S3Key, Offset: off}; return nil
}
func (m *queryTestMeta) BatchMapTraces(ctx context.Context, sid string, ms []meta.TraceMapping) error {
	for _, mp := range ms { m.MapTraceToSegment(ctx, mp.TraceID, sid, mp.Offset) }; return nil
}
func (m *queryTestMeta) LookupTrace(_ context.Context, tid string) (*meta.TraceLocation, error) {
	l, ok := m.traces[tid]; if !ok { return nil, meta.ErrTraceNotFound }; return &l, nil
}
func (m *queryTestMeta) ListSegments(_ context.Context, _, _ time.Time) ([]meta.SegmentMeta, error) { return nil, nil }
func (m *queryTestMeta) ListUnindexedSegments(_ context.Context) ([]meta.SegmentMeta, error) {
	var r []meta.SegmentMeta; for _, s := range m.segments { if !s.Indexed { r = append(r, s) } }; return r, nil
}
func (m *queryTestMeta) ListUnprocessedWALFiles(_ context.Context) ([]meta.WALFileMeta, error) { return nil, nil }
func (m *queryTestMeta) RegisterWALFile(_ context.Context, _ meta.WALFileMeta) error { return nil }
func (m *queryTestMeta) MarkWALProcessed(_ context.Context, _ string) error { return nil }
func (m *queryTestMeta) MarkSegmentIndexed(_ context.Context, sid string) error {
	s := m.segments[sid]; s.Indexed = true; m.segments[sid] = s; return nil
}
func (m *queryTestMeta) Close() {}
