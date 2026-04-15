CREATE TABLE IF NOT EXISTS segments (
    segment_id TEXT PRIMARY KEY,
    s3_key TEXT NOT NULL,
    min_time TIMESTAMPTZ NOT NULL,
    max_time TIMESTAMPTZ NOT NULL,
    span_count INTEGER NOT NULL DEFAULT 0,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    indexed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_segments_time ON segments(min_time, max_time);
CREATE INDEX IF NOT EXISTS idx_segments_unindexed ON segments(indexed) WHERE indexed = FALSE;

CREATE TABLE IF NOT EXISTS trace_segments (
    trace_id TEXT NOT NULL,
    segment_id TEXT NOT NULL REFERENCES segments(segment_id),
    offset_pos INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (trace_id, segment_id)
);

CREATE INDEX IF NOT EXISTS idx_trace_segments_trace ON trace_segments(trace_id);

CREATE TABLE IF NOT EXISTS wal_files (
    key TEXT PRIMARY KEY,
    node_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_wal_unprocessed ON wal_files(processed) WHERE processed = FALSE;
