package wal

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"go.uber.org/zap"

	"github.com/spectra-ai/spectra/pkg/model"
	"github.com/spectra-ai/spectra/pkg/storage"
	"github.com/vmihailenco/msgpack/v5"
)

// ErrSpanNotFound is returned when a span is not found in a WAL file.
var ErrSpanNotFound = errors.New("wal: span not found")

// Reader reads and replays WAL files from object storage.
type Reader struct {
	store  storage.ObjectStore
	logger *zap.Logger
}

// NewReader creates a new WAL reader.
func NewReader(store storage.ObjectStore, logger *zap.Logger) *Reader {
	return &Reader{store: store, logger: logger}
}

// ListWALFiles returns all WAL file keys for a specific node, sorted lexicographically.
func (r *Reader) ListWALFiles(ctx context.Context, nodeID string) ([]string, error) {
	prefix := fmt.Sprintf("wal/%s/", nodeID)
	keys, err := r.store.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("wal: list files for node %s: %w", nodeID, err)
	}
	sort.Strings(keys)
	return keys, nil
}

// ListAllWALFiles returns all WAL file keys across all nodes, sorted.
func (r *Reader) ListAllWALFiles(ctx context.Context) ([]string, error) {
	keys, err := r.store.List(ctx, "wal/")
	if err != nil {
		return nil, fmt.Errorf("wal: list all files: %w", err)
	}
	sort.Strings(keys)
	return keys, nil
}

// ReadFile reads and decodes all entries from a WAL file.
func (r *Reader) ReadFile(ctx context.Context, key string) ([]*Entry, error) {
	data, err := r.store.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("wal: read file %s: %w", key, err)
	}
	entries, err := DecodeEntries(data)
	if err != nil {
		return nil, fmt.Errorf("wal: decode file %s: %w", key, err)
	}
	return entries, nil
}

// ReadSpan reads a specific span from a WAL file by span ID.
func (r *Reader) ReadSpan(ctx context.Context, key string, spanID string) (*model.Span, error) {
	entries, err := r.ReadFile(ctx, key)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.SpanID == spanID {
			var span model.Span
			if err := msgpack.Unmarshal(entry.Payload, &span); err != nil {
				return nil, fmt.Errorf("wal: decode span %s: %w", spanID, err)
			}
			return &span, nil
		}
	}

	return nil, ErrSpanNotFound
}
