package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FSStore is an ObjectStore backed by the local filesystem.
// Keys are mapped to file paths relative to basePath.
type FSStore struct {
	basePath string
}

// NewFS returns a new FSStore rooted at basePath.
// The directory is created (along with parents) if it does not exist.
func NewFS(basePath string) (*FSStore, error) {
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, err
	}
	return &FSStore{basePath: basePath}, nil
}

func (s *FSStore) path(key string) string {
	return filepath.Join(s.basePath, filepath.FromSlash(key))
}

// Put writes data to key, creating intermediate directories as needed.
func (s *FSStore) Put(_ context.Context, key string, data []byte) error {
	p := s.path(key)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// PutReader streams the contents of r to key.
func (s *FSStore) PutReader(_ context.Context, key string, r io.Reader) error {
	p := s.path(key)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			f.Close()
		}
	}()
	if _, err := io.Copy(f, r); err != nil {
		return err
	}
	closed = true
	return f.Close()
}

// Get returns the contents of key or ErrNotFound.
func (s *FSStore) Get(_ context.Context, key string) ([]byte, error) {
	data, err := os.ReadFile(s.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return data, err
}

// Delete removes the file at key. Missing keys are silently ignored.
func (s *FSStore) Delete(_ context.Context, key string) error {
	err := os.Remove(s.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// List returns all keys under prefix, using forward-slash separators
// regardless of the host OS.
func (s *FSStore) List(_ context.Context, prefix string) ([]string, error) {
	var keys []string
	root := s.basePath
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		// Normalise to forward slashes so keys are OS-independent.
		key := filepath.ToSlash(rel)
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
		return nil
	})
	return keys, err
}

// Exists reports whether key is present on disk.
func (s *FSStore) Exists(_ context.Context, key string) (bool, error) {
	_, err := os.Stat(s.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
