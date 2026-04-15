package segment

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/spectra-ai/spectra/pkg/meta"
	"github.com/spectra-ai/spectra/pkg/storage"
	"github.com/spectra-ai/spectra/pkg/wal"
)

// Compactor is a background worker that compacts WAL files into segments.
type Compactor struct {
	store     storage.ObjectStore
	meta      meta.MetaStore
	walReader *wal.Reader
	builder   *Builder
	interval  time.Duration
	logger    *zap.Logger
}

// NewCompactor creates a new compactor.
func NewCompactor(store storage.ObjectStore, metaStore meta.MetaStore, walReader *wal.Reader, builder *Builder, interval time.Duration, logger *zap.Logger) *Compactor {
	return &Compactor{
		store:     store,
		meta:      metaStore,
		walReader: walReader,
		builder:   builder,
		interval:  interval,
		logger:    logger,
	}
}

// Start runs the compaction loop in the current goroutine until ctx is cancelled.
func (c *Compactor) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.CompactOnce(ctx); err != nil {
				c.logger.Error("compaction failed", zap.Error(err))
			}
		}
	}
}

// CompactOnce performs a single compaction cycle.
func (c *Compactor) CompactOnce(ctx context.Context) error {
	// 1. List unprocessed WAL files
	walFiles, err := c.meta.ListUnprocessedWALFiles(ctx)
	if err != nil {
		return fmt.Errorf("compact: list WAL files: %w", err)
	}
	if len(walFiles) == 0 {
		return nil
	}

	c.logger.Info("compacting WAL files", zap.Int("count", len(walFiles)))

	// 2. Read all entries from WAL files
	var allEntries []*wal.Entry
	var processedKeys []string

	for _, wf := range walFiles {
		entries, err := c.walReader.ReadFile(ctx, wf.Key)
		if err != nil {
			c.logger.Warn("failed to read WAL file, skipping", zap.String("key", wf.Key), zap.Error(err))
			continue
		}
		allEntries = append(allEntries, entries...)
		processedKeys = append(processedKeys, wf.Key)
	}

	if len(allEntries) == 0 {
		return nil
	}

	// 3. Build segment
	seg, err := c.builder.Build(allEntries)
	if err != nil {
		return fmt.Errorf("compact: build segment: %w", err)
	}

	// 4. Encode and write to object storage
	data, err := c.builder.Encode(seg)
	if err != nil {
		return fmt.Errorf("compact: encode segment: %w", err)
	}

	s3Key := fmt.Sprintf("segments/%s.seg", seg.ID)
	if err := c.store.Put(ctx, s3Key, data); err != nil {
		return fmt.Errorf("compact: write segment: %w", err)
	}

	// 5. Register segment in metadata store
	segMeta := meta.SegmentMeta{
		SegmentID: seg.ID,
		S3Key:     s3Key,
		MinTime:   seg.MinTime,
		MaxTime:   seg.MaxTime,
		SpanCount: seg.SpanCount,
		SizeBytes: int64(len(data)),
		Indexed:   false,
		CreatedAt: time.Now().UTC(),
	}
	if err := c.meta.RegisterSegment(ctx, segMeta); err != nil {
		return fmt.Errorf("compact: register segment: %w", err)
	}

	// 6. Batch map traces
	mappings := TraceOffsets(seg)
	if err := c.meta.BatchMapTraces(ctx, seg.ID, mappings); err != nil {
		return fmt.Errorf("compact: map traces: %w", err)
	}

	// 7. Mark WAL files as processed
	for _, key := range processedKeys {
		if err := c.meta.MarkWALProcessed(ctx, key); err != nil {
			c.logger.Warn("failed to mark WAL processed", zap.String("key", key), zap.Error(err))
		}
	}

	c.logger.Info("compaction complete",
		zap.String("segment_id", seg.ID),
		zap.Int("spans", seg.SpanCount),
		zap.Int("traces", len(seg.Traces)),
		zap.Int("wal_files", len(processedKeys)),
	)

	return nil
}
