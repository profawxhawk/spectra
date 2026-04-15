// Package storage defines the ObjectStore interface and provides
// implementations backed by the local filesystem and Amazon S3.
package storage

import (
	"context"
	"errors"
	"io"
)

// ErrNotFound is returned when a requested object does not exist.
var ErrNotFound = errors.New("storage: object not found")

// ObjectStore is the abstraction used by every Spectra component that
// needs to persist or retrieve opaque blobs (segments, WAL files, etc.).
type ObjectStore interface {
	// Put writes data to the given key, overwriting any previous value.
	Put(ctx context.Context, key string, data []byte) error

	// PutReader streams the contents of r to the given key.
	PutReader(ctx context.Context, key string, r io.Reader) error

	// Get returns the contents stored under key.
	// Returns ErrNotFound if the key does not exist.
	Get(ctx context.Context, key string) ([]byte, error)

	// Delete removes the object stored under key.
	// Deleting a non-existent key is not an error.
	Delete(ctx context.Context, key string) error

	// List returns all keys that share the given prefix.
	// An empty prefix returns every key in the store.
	List(ctx context.Context, prefix string) ([]string, error)

	// Exists reports whether key is present in the store.
	Exists(ctx context.Context, key string) (bool, error)
}
