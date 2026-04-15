package segment

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/vmihailenco/msgpack/v5"
	"go.uber.org/zap"

	"github.com/spectra-ai/spectra/pkg/meta"
	"github.com/spectra-ai/spectra/pkg/wal"
)

// Segment is a time-ordered, trace-colocated chunk of spans.
type Segment struct {
	ID        string
	MinTime   time.Time
	MaxTime   time.Time
	SpanCount int
	Traces    map[string][]*wal.Entry // traceID -> entries
}

// segmentWire is the msgpack serialization format for a segment.
type segmentWire struct {
	ID        string       `msgpack:"id"`
	MinTime   time.Time    `msgpack:"min_time"`
	MaxTime   time.Time    `msgpack:"max_time"`
	SpanCount int          `msgpack:"span_count"`
	TraceIDs  []string     `msgpack:"trace_ids"`
	Groups    [][]*wal.Entry `msgpack:"groups"`
}

// Builder creates segments from WAL entries.
type Builder struct {
	targetSize int
	maxSpans   int
	logger     *zap.Logger
}

// NewBuilder creates a new segment builder.
func NewBuilder(targetSize, maxSpans int, logger *zap.Logger) *Builder {
	return &Builder{targetSize: targetSize, maxSpans: maxSpans, logger: logger}
}

// Build groups entries by trace ID, sorts within each trace, and creates a segment.
func (b *Builder) Build(entries []*wal.Entry) (*Segment, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("segment: no entries to build from")
	}

	traces := make(map[string][]*wal.Entry)
	var minTime, maxTime time.Time

	for _, e := range entries {
		traces[e.TraceID] = append(traces[e.TraceID], e)
		if minTime.IsZero() || e.Timestamp.Before(minTime) {
			minTime = e.Timestamp
		}
		if maxTime.IsZero() || e.Timestamp.After(maxTime) {
			maxTime = e.Timestamp
		}
	}

	// Sort entries within each trace by timestamp
	for _, group := range traces {
		sort.Slice(group, func(i, j int) bool {
			return group[i].Timestamp.Before(group[j].Timestamp)
		})
	}

	return &Segment{
		ID:        uuid.New().String(),
		MinTime:   minTime,
		MaxTime:   maxTime,
		SpanCount: len(entries),
		Traces:    traces,
	}, nil
}

// Encode serializes a segment to msgpack.
func (b *Builder) Encode(seg *Segment) ([]byte, error) {
	traceIDs := make([]string, 0, len(seg.Traces))
	for id := range seg.Traces {
		traceIDs = append(traceIDs, id)
	}
	sort.Strings(traceIDs)

	groups := make([][]*wal.Entry, len(traceIDs))
	for i, id := range traceIDs {
		groups[i] = seg.Traces[id]
	}

	wire := segmentWire{
		ID:        seg.ID,
		MinTime:   seg.MinTime,
		MaxTime:   seg.MaxTime,
		SpanCount: seg.SpanCount,
		TraceIDs:  traceIDs,
		Groups:    groups,
	}

	return msgpack.Marshal(&wire)
}

// Decode deserializes a segment from msgpack.
func Decode(data []byte) (*Segment, error) {
	var wire segmentWire
	if err := msgpack.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("segment: decode: %w", err)
	}

	traces := make(map[string][]*wal.Entry, len(wire.TraceIDs))
	for i, id := range wire.TraceIDs {
		if i < len(wire.Groups) {
			traces[id] = wire.Groups[i]
		}
	}

	return &Segment{
		ID:        wire.ID,
		MinTime:   wire.MinTime,
		MaxTime:   wire.MaxTime,
		SpanCount: wire.SpanCount,
		Traces:    traces,
	}, nil
}

// TraceOffsets returns trace-to-offset mappings for the metadata store.
func TraceOffsets(seg *Segment) []meta.TraceMapping {
	traceIDs := make([]string, 0, len(seg.Traces))
	for id := range seg.Traces {
		traceIDs = append(traceIDs, id)
	}
	sort.Strings(traceIDs)

	mappings := make([]meta.TraceMapping, len(traceIDs))
	for i, id := range traceIDs {
		mappings[i] = meta.TraceMapping{TraceID: id, Offset: i}
	}
	return mappings
}
