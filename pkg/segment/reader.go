package segment

import (
	"context"
	"fmt"
	"sort"

	"go.uber.org/zap"

	"github.com/spectra-ai/spectra/pkg/model"
	"github.com/spectra-ai/spectra/pkg/storage"
	"github.com/vmihailenco/msgpack/v5"
)

// Reader reads segments from object storage.
type Reader struct {
	store  storage.ObjectStore
	logger *zap.Logger
}

// NewReader creates a new segment reader.
func NewReader(store storage.ObjectStore, logger *zap.Logger) *Reader {
	return &Reader{store: store, logger: logger}
}

// ReadSegment fetches and decodes a segment from object storage.
func (r *Reader) ReadSegment(ctx context.Context, s3Key string) (*Segment, error) {
	data, err := r.store.Get(ctx, s3Key)
	if err != nil {
		return nil, fmt.Errorf("segment: read %s: %w", s3Key, err)
	}
	seg, err := Decode(data)
	if err != nil {
		return nil, fmt.Errorf("segment: decode %s: %w", s3Key, err)
	}
	return seg, nil
}

// ReadTrace reads and decodes all spans for a specific trace from a segment.
func (r *Reader) ReadTrace(ctx context.Context, s3Key string, traceID string) ([]*model.Span, error) {
	seg, err := r.ReadSegment(ctx, s3Key)
	if err != nil {
		return nil, err
	}

	entries, ok := seg.Traces[traceID]
	if !ok {
		return nil, fmt.Errorf("segment: trace %s not found in %s", traceID, s3Key)
	}

	spans := make([]*model.Span, 0, len(entries))
	for _, entry := range entries {
		var span model.Span
		if err := msgpack.Unmarshal(entry.Payload, &span); err != nil {
			return nil, fmt.Errorf("segment: decode span %s: %w", entry.SpanID, err)
		}
		spans = append(spans, &span)
	}
	return spans, nil
}

// ReadAllSpans reads all spans from a segment, sorted by timestamp.
func (r *Reader) ReadAllSpans(ctx context.Context, s3Key string) ([]*model.Span, error) {
	seg, err := r.ReadSegment(ctx, s3Key)
	if err != nil {
		return nil, err
	}

	spans := make([]*model.Span, 0, seg.SpanCount)
	for _, entries := range seg.Traces {
		for _, entry := range entries {
			var span model.Span
			if err := msgpack.Unmarshal(entry.Payload, &span); err != nil {
				r.logger.Warn("failed to decode span", zap.String("span_id", entry.SpanID), zap.Error(err))
				continue
			}
			spans = append(spans, &span)
		}
	}

	sort.Slice(spans, func(i, j int) bool {
		return spans[i].StartTime.Before(spans[j].StartTime)
	})

	return spans, nil
}
