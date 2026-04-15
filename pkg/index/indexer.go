package index

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/spectra-ai/spectra/pkg/meta"
	"github.com/spectra-ai/spectra/pkg/model"
	"github.com/spectra-ai/spectra/pkg/segment"
	"github.com/spectra-ai/spectra/pkg/storage"
)

// SpanResult holds a search result.
type SpanResult struct {
	SpanID    string  `json:"span_id"`
	TraceID   string  `json:"trace_id"`
	Name      string  `json:"name"`
	SegmentID string  `json:"segment_id"`
	Score     float64 `json:"score"`
}

// spanDoc is an indexed span stored in memory for search.
type spanDoc struct {
	Span      *model.Span
	SegmentID string
}

// Indexer builds and maintains searchable indexes from segments.
type Indexer struct {
	meta      meta.MetaStore
	segReader *segment.Reader
	store     storage.ObjectStore
	interval  time.Duration
	logger    *zap.Logger

	mu   sync.RWMutex
	docs []spanDoc // in-memory index (simple for now)
}

// NewIndexer creates a new indexer.
func NewIndexer(metaStore meta.MetaStore, segReader *segment.Reader, store storage.ObjectStore, interval time.Duration, logger *zap.Logger) *Indexer {
	return &Indexer{
		meta:      metaStore,
		segReader: segReader,
		store:     store,
		interval:  interval,
		logger:    logger,
		docs:      make([]spanDoc, 0),
	}
}

// Start runs the indexing loop.
func (idx *Indexer) Start(ctx context.Context) {
	ticker := time.NewTicker(idx.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := idx.IndexOnce(ctx); err != nil {
				idx.logger.Error("indexing failed", zap.Error(err))
			}
		}
	}
}

// IndexOnce indexes all unindexed segments.
func (idx *Indexer) IndexOnce(ctx context.Context) error {
	segments, err := idx.meta.ListUnindexedSegments(ctx)
	if err != nil {
		return fmt.Errorf("index: list unindexed: %w", err)
	}
	if len(segments) == 0 {
		return nil
	}

	for _, segMeta := range segments {
		if err := idx.indexSegment(ctx, segMeta); err != nil {
			idx.logger.Warn("failed to index segment", zap.String("segment_id", segMeta.SegmentID), zap.Error(err))
			continue
		}
		if err := idx.meta.MarkSegmentIndexed(ctx, segMeta.SegmentID); err != nil {
			return fmt.Errorf("index: mark indexed %s: %w", segMeta.SegmentID, err)
		}
	}

	idx.logger.Info("indexing complete", zap.Int("segments", len(segments)))
	return nil
}

func (idx *Indexer) indexSegment(ctx context.Context, segMeta meta.SegmentMeta) error {
	spans, err := idx.segReader.ReadAllSpans(ctx, segMeta.S3Key)
	if err != nil {
		return fmt.Errorf("index: read segment %s: %w", segMeta.SegmentID, err)
	}

	// Build bloom filter for this segment
	bloom := NewBloomFilter(len(spans)*2, 0.01)
	for _, span := range spans {
		bloom.Add(span.TraceID)
		bloom.Add(span.SpanID)
	}

	// Store bloom filter
	bloomData, err := bloom.Encode()
	if err != nil {
		return fmt.Errorf("index: encode bloom: %w", err)
	}
	bloomKey := fmt.Sprintf("index/%s.bloom", segMeta.SegmentID)
	if err := idx.store.Put(ctx, bloomKey, bloomData); err != nil {
		idx.logger.Warn("failed to store bloom filter", zap.Error(err))
	}

	// Add to in-memory index
	idx.mu.Lock()
	for _, span := range spans {
		idx.docs = append(idx.docs, spanDoc{Span: span, SegmentID: segMeta.SegmentID})
	}
	idx.mu.Unlock()

	return nil
}

// Search performs full-text search across indexed spans.
func (idx *Indexer) Search(_ context.Context, query string, limit int) ([]SpanResult, error) {
	if limit <= 0 {
		limit = 50
	}
	queryLower := strings.ToLower(query)

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var results []SpanResult
	for _, doc := range idx.docs {
		content := strings.ToLower(doc.Span.Input + " " + doc.Span.Output + " " + doc.Span.Name)
		if strings.Contains(content, queryLower) {
			results = append(results, SpanResult{
				SpanID:    doc.Span.SpanID,
				TraceID:   doc.Span.TraceID,
				Name:      doc.Span.Name,
				SegmentID: doc.SegmentID,
				Score:     1.0,
			})
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// SearchByTraceID finds all indexed spans for a trace.
func (idx *Indexer) SearchByTraceID(_ context.Context, traceID string, limit int) ([]SpanResult, error) {
	if limit <= 0 {
		limit = 50
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var results []SpanResult
	for _, doc := range idx.docs {
		if doc.Span.TraceID == traceID {
			results = append(results, SpanResult{
				SpanID:    doc.Span.SpanID,
				TraceID:   doc.Span.TraceID,
				Name:      doc.Span.Name,
				SegmentID: doc.SegmentID,
				Score:     1.0,
			})
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// SearchByField finds spans matching a field value.
func (idx *Indexer) SearchByField(_ context.Context, field, value string, limit int) ([]SpanResult, error) {
	if limit <= 0 {
		limit = 50
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var results []SpanResult
	for _, doc := range idx.docs {
		var fieldVal string
		switch field {
		case "name":
			fieldVal = doc.Span.Name
		case "kind":
			fieldVal = string(doc.Span.Kind)
		case "status":
			fieldVal = string(doc.Span.Status)
		case "trace_id":
			fieldVal = doc.Span.TraceID
		case "span_id":
			fieldVal = doc.Span.SpanID
		default:
			if v, ok := doc.Span.Metadata[field]; ok {
				fieldVal = v
			}
		}
		if strings.EqualFold(fieldVal, value) {
			results = append(results, SpanResult{
				SpanID:    doc.Span.SpanID,
				TraceID:   doc.Span.TraceID,
				Name:      doc.Span.Name,
				SegmentID: doc.SegmentID,
				Score:     1.0,
			})
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}
