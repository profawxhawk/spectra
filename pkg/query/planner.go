package query

import (
	"context"

	"github.com/spectra-ai/spectra/pkg/index"
	"github.com/spectra-ai/spectra/pkg/memindex"
	"github.com/spectra-ai/spectra/pkg/meta"
	"github.com/spectra-ai/spectra/pkg/model"
	"github.com/spectra-ai/spectra/pkg/segment"
	"github.com/spectra-ai/spectra/pkg/wal"
)

// Source describes where spans can be fetched from.
type Source int

const (
	SourceMemIndex Source = iota
	SourceWAL
	SourceSegment
	SourceIndex
)

// Plan describes how to execute a query across the 4 data layers.
type Plan struct {
	Query   model.QueryRequest
	Sources []Source
	// If trace_id filter exists, direct lookup is possible.
	TraceID string
}

// Planner creates execution plans for queries.
type Planner struct {
	memIdx    *memindex.MemIndex
	walReader *wal.Reader
	metaStore meta.MetaStore
	segReader *segment.Reader
	indexer   *index.Indexer
}

// NewPlanner creates a query planner.
func NewPlanner(memIdx *memindex.MemIndex, walReader *wal.Reader, metaStore meta.MetaStore, segReader *segment.Reader, indexer *index.Indexer) *Planner {
	return &Planner{
		memIdx:    memIdx,
		walReader: walReader,
		metaStore: metaStore,
		segReader: segReader,
		indexer:   indexer,
	}
}

// CreatePlan analyzes a query and produces an execution plan.
func (p *Planner) CreatePlan(q model.QueryRequest) *Plan {
	plan := &Plan{Query: q}

	// Check for direct trace_id lookup
	for _, f := range q.Filters {
		if f.Field == "trace_id" && f.Operator == model.OpEq {
			plan.TraceID = f.Value.(string)
		}
	}

	if plan.TraceID != "" {
		// For trace lookups, check memindex first, then meta, then WAL scan
		plan.Sources = []Source{SourceMemIndex, SourceSegment, SourceWAL}
	} else if q.Search != "" {
		// For full-text search, use the index first, fall back to WAL scan
		plan.Sources = []Source{SourceIndex, SourceSegment, SourceWAL}
	} else {
		// General filter: scan all layers
		plan.Sources = []Source{SourceIndex, SourceSegment, SourceWAL}
	}

	return plan
}

// Execute runs a plan and returns results, merging from all data layers.
func (p *Planner) Execute(ctx context.Context, plan *Plan) (*model.QueryResult, error) {
	seen := make(map[string]bool) // span_id dedup
	var allSpans []model.Span

	// Direct trace lookup path
	if plan.TraceID != "" {
		spans, err := p.lookupTrace(ctx, plan.TraceID)
		if err == nil {
			for _, s := range spans {
				if !seen[s.SpanID] && matchesAllFilters(&s, plan.Query.Filters) && matchesSearch(&s, plan.Query.Search) {
					seen[s.SpanID] = true
					allSpans = append(allSpans, s)
				}
			}
		}
	} else {
		// Search/filter path

		// First: scan indexed data via the indexer (search + filter)
		if p.indexer != nil {
			searchTerm := plan.Query.Search
			if searchTerm == "" {
				// For filter-only queries, do a broad search
				searchTerm = "*"
			}
			results, err := p.indexer.Search(ctx, plan.Query.Search, 0)
			if err == nil && len(results) > 0 {
				// Look up full spans from segments via meta store
				for _, r := range results {
					if seen[r.SpanID] {
						continue
					}
					if p.metaStore != nil {
						loc, err := p.metaStore.LookupTrace(ctx, r.TraceID)
						if err == nil {
							traceSpans, err := p.segReader.ReadTrace(ctx, loc.S3Key, r.TraceID)
							if err == nil {
								for _, s := range traceSpans {
									if !seen[s.SpanID] && matchesAllFilters(s, plan.Query.Filters) && matchesSearch(s, plan.Query.Search) {
										seen[s.SpanID] = true
										allSpans = append(allSpans, *s)
									}
								}
							}
						}
					}
				}
			}
		}

		// Second: scan unindexed segments
		if p.metaStore != nil {
			unindexed, err := p.metaStore.ListUnindexedSegments(ctx)
			if err == nil {
				for _, segMeta := range unindexed {
					spans, err := p.segReader.ReadAllSpans(ctx, segMeta.S3Key)
					if err != nil {
						continue
					}
					for _, s := range spans {
						if !seen[s.SpanID] && matchesAllFilters(s, plan.Query.Filters) && matchesSearch(s, plan.Query.Search) {
							seen[s.SpanID] = true
							allSpans = append(allSpans, *s)
						}
					}
				}
			}
		}
	}

	// Sort
	sortSpans(allSpans, plan.Query.OrderBy, plan.Query.Ascending)

	// Paginate
	total := len(allSpans)
	start := plan.Query.Offset
	if start > total {
		start = total
	}
	end := start + plan.Query.Limit
	if end > total {
		end = total
	}

	return &model.QueryResult{
		Spans:      allSpans[start:end],
		TotalCount: total,
		HasMore:    end < total,
	}, nil
}

// lookupTrace finds spans for a trace across all layers.
func (p *Planner) lookupTrace(ctx context.Context, traceID string) ([]model.Span, error) {
	// Layer 1: Check memindex for WAL locations
	if p.memIdx != nil {
		if locs, ok := p.memIdx.Lookup(traceID); ok {
			var spans []model.Span
			for _, loc := range locs {
				if loc.WALKey != "" {
					span, err := p.walReader.ReadSpan(ctx, loc.WALKey, "")
					if err == nil {
						spans = append(spans, *span)
					}
				}
			}
			if len(spans) > 0 {
				return spans, nil
			}
		}
	}

	// Layer 2: Check metadata store for segment location
	if p.metaStore != nil {
		loc, err := p.metaStore.LookupTrace(ctx, traceID)
		if err == nil {
			segSpans, err := p.segReader.ReadTrace(ctx, loc.S3Key, traceID)
			if err == nil {
				spans := make([]model.Span, len(segSpans))
				for i, s := range segSpans {
					spans[i] = *s
				}
				return spans, nil
			}
		}
	}

	// Layer 3: Scan WAL files
	if p.walReader != nil {
		walFiles, err := p.walReader.ListAllWALFiles(ctx)
		if err == nil {
			var spans []model.Span
			for _, wf := range walFiles {
				entries, err := p.walReader.ReadFile(ctx, wf)
				if err != nil {
					continue
				}
				for _, e := range entries {
					if e.TraceID == traceID {
						var span model.Span
						if err := decodeMsgpack(e.Payload, &span); err == nil {
							spans = append(spans, span)
						}
					}
				}
			}
			if len(spans) > 0 {
				return spans, nil
			}
		}
	}

	return nil, meta.ErrTraceNotFound
}
