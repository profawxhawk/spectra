package meta

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrTraceNotFound is returned when a trace has no segment mapping.
var ErrTraceNotFound = errors.New("spectra: trace not found")

// SegmentMeta holds metadata about a compacted segment.
type SegmentMeta struct {
	SegmentID string    `json:"segment_id"`
	S3Key     string    `json:"s3_key"`
	MinTime   time.Time `json:"min_time"`
	MaxTime   time.Time `json:"max_time"`
	SpanCount int       `json:"span_count"`
	SizeBytes int64     `json:"size_bytes"`
	Indexed   bool      `json:"indexed"`
	CreatedAt time.Time `json:"created_at"`
}

// TraceMapping maps a trace ID to its offset within a segment.
type TraceMapping struct {
	TraceID string `json:"trace_id"`
	Offset  int    `json:"offset"`
}

// TraceLocation describes where a trace's spans can be found.
type TraceLocation struct {
	SegmentID string `json:"segment_id"`
	S3Key     string `json:"s3_key"`
	Offset    int    `json:"offset"`
}

// WALFileMeta tracks a WAL file's processing state.
type WALFileMeta struct {
	Key       string    `json:"key"`
	NodeID    string    `json:"node_id"`
	CreatedAt time.Time `json:"created_at"`
	Processed bool      `json:"processed"`
}

// MetaStore manages segment metadata and trace-to-segment mappings.
type MetaStore interface {
	Migrate(ctx context.Context) error
	RegisterSegment(ctx context.Context, seg SegmentMeta) error
	MapTraceToSegment(ctx context.Context, traceID, segmentID string, offset int) error
	BatchMapTraces(ctx context.Context, segmentID string, mappings []TraceMapping) error
	LookupTrace(ctx context.Context, traceID string) (*TraceLocation, error)
	ListSegments(ctx context.Context, from, to time.Time) ([]SegmentMeta, error)
	ListUnindexedSegments(ctx context.Context) ([]SegmentMeta, error)
	ListUnprocessedWALFiles(ctx context.Context) ([]WALFileMeta, error)
	RegisterWALFile(ctx context.Context, meta WALFileMeta) error
	MarkWALProcessed(ctx context.Context, key string) error
	MarkSegmentIndexed(ctx context.Context, segmentID string) error
	Close()
}

// PostgresStore implements MetaStore backed by PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new PostgresStore with a connection pool.
func NewPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("spectra meta: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("spectra meta: ping: %w", err)
	}
	return &PostgresStore{pool: pool}, nil
}

// Migrate runs embedded SQL migrations.
func (s *PostgresStore) Migrate(ctx context.Context) error {
	data, err := MigrationFS.ReadFile("migrations/001_initial.sql")
	if err != nil {
		return fmt.Errorf("spectra meta: read migration: %w", err)
	}
	if _, err := s.pool.Exec(ctx, string(data)); err != nil {
		return fmt.Errorf("spectra meta: run migration: %w", err)
	}
	return nil
}

// RegisterSegment inserts a new segment record.
func (s *PostgresStore) RegisterSegment(ctx context.Context, seg SegmentMeta) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO segments (segment_id, s3_key, min_time, max_time, span_count, size_bytes, indexed, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (segment_id) DO NOTHING`,
		seg.SegmentID, seg.S3Key, seg.MinTime, seg.MaxTime, seg.SpanCount, seg.SizeBytes, seg.Indexed, seg.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("spectra meta: register segment: %w", err)
	}
	return nil
}

// MapTraceToSegment records that a trace is in a segment at the given offset.
func (s *PostgresStore) MapTraceToSegment(ctx context.Context, traceID, segmentID string, offset int) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO trace_segments (trace_id, segment_id, offset_pos)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (trace_id, segment_id) DO UPDATE SET offset_pos = $3`,
		traceID, segmentID, offset,
	)
	if err != nil {
		return fmt.Errorf("spectra meta: map trace: %w", err)
	}
	return nil
}

// BatchMapTraces maps multiple traces to a segment in a single transaction.
func (s *PostgresStore) BatchMapTraces(ctx context.Context, segmentID string, mappings []TraceMapping) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("spectra meta: begin batch: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, m := range mappings {
		if _, err := tx.Exec(ctx,
			`INSERT INTO trace_segments (trace_id, segment_id, offset_pos)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (trace_id, segment_id) DO UPDATE SET offset_pos = $3`,
			m.TraceID, segmentID, m.Offset,
		); err != nil {
			return fmt.Errorf("spectra meta: batch map trace %s: %w", m.TraceID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("spectra meta: commit batch: %w", err)
	}
	return nil
}

// LookupTrace finds which segment contains a trace.
func (s *PostgresStore) LookupTrace(ctx context.Context, traceID string) (*TraceLocation, error) {
	var loc TraceLocation
	err := s.pool.QueryRow(ctx,
		`SELECT ts.segment_id, s.s3_key, ts.offset_pos
		 FROM trace_segments ts
		 JOIN segments s ON s.segment_id = ts.segment_id
		 WHERE ts.trace_id = $1
		 LIMIT 1`,
		traceID,
	).Scan(&loc.SegmentID, &loc.S3Key, &loc.Offset)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTraceNotFound
		}
		return nil, fmt.Errorf("spectra meta: lookup trace: %w", err)
	}
	return &loc, nil
}

// ListSegments returns segments overlapping the given time range.
func (s *PostgresStore) ListSegments(ctx context.Context, from, to time.Time) ([]SegmentMeta, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT segment_id, s3_key, min_time, max_time, span_count, size_bytes, indexed, created_at
		 FROM segments
		 WHERE min_time <= $2 AND max_time >= $1
		 ORDER BY min_time`,
		from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("spectra meta: list segments: %w", err)
	}
	defer rows.Close()

	var result []SegmentMeta
	for rows.Next() {
		var seg SegmentMeta
		if err := rows.Scan(&seg.SegmentID, &seg.S3Key, &seg.MinTime, &seg.MaxTime,
			&seg.SpanCount, &seg.SizeBytes, &seg.Indexed, &seg.CreatedAt); err != nil {
			return nil, fmt.Errorf("spectra meta: scan segment: %w", err)
		}
		result = append(result, seg)
	}
	return result, rows.Err()
}

// ListUnindexedSegments returns segments not yet fully indexed.
func (s *PostgresStore) ListUnindexedSegments(ctx context.Context) ([]SegmentMeta, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT segment_id, s3_key, min_time, max_time, span_count, size_bytes, indexed, created_at
		 FROM segments
		 WHERE indexed = FALSE
		 ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("spectra meta: list unindexed: %w", err)
	}
	defer rows.Close()

	var result []SegmentMeta
	for rows.Next() {
		var seg SegmentMeta
		if err := rows.Scan(&seg.SegmentID, &seg.S3Key, &seg.MinTime, &seg.MaxTime,
			&seg.SpanCount, &seg.SizeBytes, &seg.Indexed, &seg.CreatedAt); err != nil {
			return nil, fmt.Errorf("spectra meta: scan unindexed: %w", err)
		}
		result = append(result, seg)
	}
	return result, rows.Err()
}

// ListUnprocessedWALFiles returns WAL files not yet compacted.
func (s *PostgresStore) ListUnprocessedWALFiles(ctx context.Context) ([]WALFileMeta, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT key, node_id, created_at, processed
		 FROM wal_files
		 WHERE processed = FALSE
		 ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("spectra meta: list unprocessed WAL: %w", err)
	}
	defer rows.Close()

	var result []WALFileMeta
	for rows.Next() {
		var w WALFileMeta
		if err := rows.Scan(&w.Key, &w.NodeID, &w.CreatedAt, &w.Processed); err != nil {
			return nil, fmt.Errorf("spectra meta: scan WAL file: %w", err)
		}
		result = append(result, w)
	}
	return result, rows.Err()
}

// RegisterWALFile records a new WAL file.
func (s *PostgresStore) RegisterWALFile(ctx context.Context, meta WALFileMeta) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO wal_files (key, node_id, created_at, processed)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (key) DO NOTHING`,
		meta.Key, meta.NodeID, meta.CreatedAt, meta.Processed,
	)
	if err != nil {
		return fmt.Errorf("spectra meta: register WAL file: %w", err)
	}
	return nil
}

// MarkWALProcessed marks a WAL file as compacted.
func (s *PostgresStore) MarkWALProcessed(ctx context.Context, key string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE wal_files SET processed = TRUE WHERE key = $1`,
		key,
	)
	if err != nil {
		return fmt.Errorf("spectra meta: mark WAL processed: %w", err)
	}
	return nil
}

// MarkSegmentIndexed marks a segment as fully indexed.
func (s *PostgresStore) MarkSegmentIndexed(ctx context.Context, segmentID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE segments SET indexed = TRUE WHERE segment_id = $1`,
		segmentID,
	)
	if err != nil {
		return fmt.Errorf("spectra meta: mark indexed: %w", err)
	}
	return nil
}

// Close closes the connection pool.
func (s *PostgresStore) Close() {
	s.pool.Close()
}
