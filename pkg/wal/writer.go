package wal

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/spectra-ai/spectra/pkg/model"
	"github.com/spectra-ai/spectra/pkg/storage"
	"github.com/vmihailenco/msgpack/v5"
)

// Writer buffers spans and periodically flushes them to object storage as WAL files.
type Writer struct {
	store         storage.ObjectStore
	nodeID        string
	flushInterval time.Duration
	maxBatchSize  int
	txnCounter    atomic.Uint64
	mu            sync.Mutex
	buffer        []*Entry
	logger        *zap.Logger
	done          chan struct{}
}

// NewWriter creates a new WAL writer.
func NewWriter(store storage.ObjectStore, nodeID string, flushInterval time.Duration, maxBatchSize int, logger *zap.Logger) *Writer {
	return &Writer{
		store:         store,
		nodeID:        nodeID,
		flushInterval: flushInterval,
		maxBatchSize:  maxBatchSize,
		buffer:        make([]*Entry, 0, maxBatchSize),
		logger:        logger,
		done:          make(chan struct{}),
	}
}

// Append adds a span to the WAL buffer. Auto-flushes when buffer reaches maxBatchSize.
func (w *Writer) Append(ctx context.Context, span *model.Span) (*Entry, error) {
	payload, err := msgpack.Marshal(span)
	if err != nil {
		return nil, fmt.Errorf("wal: encode span: %w", err)
	}

	entry := &Entry{
		TxnID:     w.txnCounter.Add(1),
		TraceID:   span.TraceID,
		SpanID:    span.SpanID,
		Timestamp: span.StartTime,
		Payload:   payload,
	}

	var shouldFlush bool
	w.mu.Lock()
	w.buffer = append(w.buffer, entry)
	shouldFlush = len(w.buffer) >= w.maxBatchSize
	w.mu.Unlock()

	if shouldFlush {
		if err := w.Flush(ctx); err != nil {
			return entry, fmt.Errorf("wal: auto-flush: %w", err)
		}
	}

	return entry, nil
}

// Flush writes all buffered entries to object storage.
func (w *Writer) Flush(ctx context.Context) error {
	w.mu.Lock()
	if len(w.buffer) == 0 {
		w.mu.Unlock()
		return nil
	}
	batch := w.buffer
	w.buffer = make([]*Entry, 0, w.maxBatchSize)
	w.mu.Unlock()

	data, err := EncodeEntries(batch)
	if err != nil {
		return fmt.Errorf("wal: encode batch: %w", err)
	}

	key := fmt.Sprintf("wal/%s/%d_%d.wal", w.nodeID, time.Now().UnixNano(), batch[len(batch)-1].TxnID)

	if err := w.store.Put(ctx, key, data); err != nil {
		return fmt.Errorf("wal: write %s: %w", key, err)
	}

	w.logger.Debug("flushed WAL", zap.String("key", key), zap.Int("entries", len(batch)))
	return nil
}

// Start begins periodic flushing in a background goroutine.
func (w *Writer) Start(ctx context.Context) {
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()
	defer close(w.done)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.Flush(ctx); err != nil {
				w.logger.Error("periodic flush failed", zap.Error(err))
			}
		}
	}
}

// Stop performs a final flush.
func (w *Writer) Stop(ctx context.Context) error {
	return w.Flush(ctx)
}
