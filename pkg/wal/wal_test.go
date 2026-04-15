package wal_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/spectra-ai/spectra/pkg/model"
	"github.com/spectra-ai/spectra/pkg/storage"
	"github.com/spectra-ai/spectra/pkg/wal"
)

func newTestStore(t *testing.T) storage.ObjectStore {
	t.Helper()
	s, err := storage.NewFS(t.TempDir())
	require.NoError(t, err)
	return s
}

func TestEntryRoundTrip(t *testing.T) {
	original := &wal.Entry{
		TxnID:     42,
		TraceID:   "trace-1",
		SpanID:    "span-1",
		Timestamp: time.Now().UTC().Truncate(time.Millisecond),
		Payload:   []byte("test-payload"),
	}

	data, err := wal.EncodeEntry(original)
	require.NoError(t, err)

	decoded, err := wal.DecodeEntry(data)
	require.NoError(t, err)

	assert.Equal(t, original.TxnID, decoded.TxnID)
	assert.Equal(t, original.TraceID, decoded.TraceID)
	assert.Equal(t, original.SpanID, decoded.SpanID)
	assert.Equal(t, original.Payload, decoded.Payload)
	assert.True(t, original.Timestamp.Equal(decoded.Timestamp))
}

func TestEntriesBatchRoundTrip(t *testing.T) {
	entries := []*wal.Entry{
		{TxnID: 1, TraceID: "t1", SpanID: "s1", Timestamp: time.Now().UTC().Truncate(time.Millisecond), Payload: []byte("p1")},
		{TxnID: 2, TraceID: "t2", SpanID: "s2", Timestamp: time.Now().UTC().Truncate(time.Millisecond), Payload: []byte("p2")},
		{TxnID: 3, TraceID: "t1", SpanID: "s3", Timestamp: time.Now().UTC().Truncate(time.Millisecond), Payload: []byte("p3")},
	}

	data, err := wal.EncodeEntries(entries)
	require.NoError(t, err)

	decoded, err := wal.DecodeEntries(data)
	require.NoError(t, err)
	require.Len(t, decoded, 3)

	for i, e := range decoded {
		assert.Equal(t, entries[i].TxnID, e.TxnID)
		assert.Equal(t, entries[i].TraceID, e.TraceID)
		assert.Equal(t, entries[i].SpanID, e.SpanID)
	}
}

func TestWriterAppendAndFlush(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	logger := zap.NewNop()

	w := wal.NewWriter(store, "node-1", time.Hour, 100, logger)

	spans := []*model.Span{
		model.NewSpan("trace-1", "llm_call", model.SpanKindLLM),
		model.NewSpan("trace-1", "tool_call", model.SpanKindTool),
		model.NewSpan("trace-2", "agent_run", model.SpanKindAgent),
	}

	for _, s := range spans {
		_, err := w.Append(ctx, s)
		require.NoError(t, err)
	}

	require.NoError(t, w.Flush(ctx))

	reader := wal.NewReader(store, logger)
	files, err := reader.ListWALFiles(ctx, "node-1")
	require.NoError(t, err)
	require.Len(t, files, 1)

	entries, err := reader.ReadFile(ctx, files[0])
	require.NoError(t, err)
	assert.Len(t, entries, 3)
	assert.Equal(t, "trace-1", entries[0].TraceID)
	assert.Equal(t, "trace-2", entries[2].TraceID)
}

func TestWriterAutoFlush(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	logger := zap.NewNop()

	w := wal.NewWriter(store, "node-1", time.Hour, 2, logger) // maxBatchSize=2

	_, err := w.Append(ctx, model.NewSpan("t1", "s1", model.SpanKindLLM))
	require.NoError(t, err)
	_, err = w.Append(ctx, model.NewSpan("t2", "s2", model.SpanKindTool))
	require.NoError(t, err)

	// Auto-flush should have triggered
	reader := wal.NewReader(store, logger)
	files, err := reader.ListAllWALFiles(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(files), 1)
}

func TestReaderListAndRead(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	logger := zap.NewNop()

	w := wal.NewWriter(store, "node-1", time.Hour, 100, logger)

	_, err := w.Append(ctx, model.NewSpan("t1", "s1", model.SpanKindLLM))
	require.NoError(t, err)
	require.NoError(t, w.Flush(ctx))

	time.Sleep(10 * time.Millisecond) // ensure different WAL file name

	_, err = w.Append(ctx, model.NewSpan("t2", "s2", model.SpanKindTool))
	require.NoError(t, err)
	require.NoError(t, w.Flush(ctx))

	reader := wal.NewReader(store, logger)
	files, err := reader.ListWALFiles(ctx, "node-1")
	require.NoError(t, err)
	assert.Len(t, files, 2)

	entries1, err := reader.ReadFile(ctx, files[0])
	require.NoError(t, err)
	assert.Len(t, entries1, 1)

	entries2, err := reader.ReadFile(ctx, files[1])
	require.NoError(t, err)
	assert.Len(t, entries2, 1)
}

func TestReaderReadSpan(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	logger := zap.NewNop()

	span1 := model.NewSpan("t1", "llm_call", model.SpanKindLLM)
	span1.Input = "What is 2+2?"
	span1.Output = "4"
	span2 := model.NewSpan("t1", "tool_call", model.SpanKindTool)
	span3 := model.NewSpan("t2", "agent_run", model.SpanKindAgent)

	w := wal.NewWriter(store, "node-1", time.Hour, 100, logger)
	_, err := w.Append(ctx, span1)
	require.NoError(t, err)
	_, err = w.Append(ctx, span2)
	require.NoError(t, err)
	_, err = w.Append(ctx, span3)
	require.NoError(t, err)
	require.NoError(t, w.Flush(ctx))

	reader := wal.NewReader(store, logger)
	files, err := reader.ListWALFiles(ctx, "node-1")
	require.NoError(t, err)

	// Read specific span
	found, err := reader.ReadSpan(ctx, files[0], span1.SpanID)
	require.NoError(t, err)
	assert.Equal(t, span1.SpanID, found.SpanID)
	assert.Equal(t, "What is 2+2?", found.Input)
	assert.Equal(t, "4", found.Output)

	// Read non-existent span
	_, err = reader.ReadSpan(ctx, files[0], "nonexistent")
	assert.ErrorIs(t, err, wal.ErrSpanNotFound)
}
