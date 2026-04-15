package storage_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"sort"
	"testing"

	"github.com/spectra-ai/spectra/pkg/storage"
)

// ---------------------------------------------------------------------------
// Shared conformance suite
// ---------------------------------------------------------------------------

// runObjectStoreTests exercises every ObjectStore method. It is called once
// per backend so that all implementations stay in lock-step.
func runObjectStoreTests(t *testing.T, store storage.ObjectStore) {
	t.Helper()
	ctx := context.Background()

	// ---- Put / Get round-trip ----
	t.Run("PutGet", func(t *testing.T) {
		data := []byte("hello, spectra")
		if err := store.Put(ctx, "a/b/c.txt", data); err != nil {
			t.Fatalf("Put: %v", err)
		}
		got, err := store.Get(ctx, "a/b/c.txt")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !bytes.Equal(got, data) {
			t.Fatalf("Get = %q, want %q", got, data)
		}
	})

	// ---- PutReader / Get round-trip ----
	t.Run("PutReaderGet", func(t *testing.T) {
		data := []byte("streamed content")
		if err := store.PutReader(ctx, "stream.bin", bytes.NewReader(data)); err != nil {
			t.Fatalf("PutReader: %v", err)
		}
		got, err := store.Get(ctx, "stream.bin")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !bytes.Equal(got, data) {
			t.Fatalf("Get = %q, want %q", got, data)
		}
	})

	// ---- Get missing key → ErrNotFound ----
	t.Run("GetNotFound", func(t *testing.T) {
		_, err := store.Get(ctx, "does/not/exist")
		if !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("Get non-existent key: got %v, want ErrNotFound", err)
		}
	})

	// ---- Exists ----
	t.Run("Exists", func(t *testing.T) {
		if err := store.Put(ctx, "exists-test", []byte("1")); err != nil {
			t.Fatalf("Put: %v", err)
		}
		ok, err := store.Exists(ctx, "exists-test")
		if err != nil {
			t.Fatalf("Exists: %v", err)
		}
		if !ok {
			t.Fatal("Exists = false, want true")
		}

		ok, err = store.Exists(ctx, "missing-key")
		if err != nil {
			t.Fatalf("Exists: %v", err)
		}
		if ok {
			t.Fatal("Exists = true for missing key, want false")
		}
	})

	// ---- Delete ----
	t.Run("Delete", func(t *testing.T) {
		if err := store.Put(ctx, "delete-me", []byte("bye")); err != nil {
			t.Fatalf("Put: %v", err)
		}
		if err := store.Delete(ctx, "delete-me"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		ok, err := store.Exists(ctx, "delete-me")
		if err != nil {
			t.Fatalf("Exists after delete: %v", err)
		}
		if ok {
			t.Fatal("key still exists after Delete")
		}
	})

	// ---- Delete non-existent key is not an error ----
	t.Run("DeleteMissing", func(t *testing.T) {
		if err := store.Delete(ctx, "never-existed"); err != nil {
			t.Fatalf("Delete missing key: %v", err)
		}
	})

	// ---- List with prefix ----
	t.Run("List", func(t *testing.T) {
		for _, key := range []string{"list/x", "list/y", "list/sub/z", "other/q"} {
			if err := store.Put(ctx, key, []byte("v")); err != nil {
				t.Fatalf("Put %s: %v", key, err)
			}
		}
		got, err := store.List(ctx, "list/")
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		sort.Strings(got)
		want := []string{"list/sub/z", "list/x", "list/y"}
		if len(got) != len(want) {
			t.Fatalf("List = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("List[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	// ---- Put overwrites ----
	t.Run("Overwrite", func(t *testing.T) {
		if err := store.Put(ctx, "overwrite", []byte("first")); err != nil {
			t.Fatalf("Put: %v", err)
		}
		if err := store.Put(ctx, "overwrite", []byte("second")); err != nil {
			t.Fatalf("Put overwrite: %v", err)
		}
		got, err := store.Get(ctx, "overwrite")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if string(got) != "second" {
			t.Fatalf("Get after overwrite = %q, want %q", got, "second")
		}
	})
}

// ---------------------------------------------------------------------------
// Filesystem backend
// ---------------------------------------------------------------------------

func TestFSStore(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewFS(dir)
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}
	runObjectStoreTests(t, store)
}

// ---------------------------------------------------------------------------
// S3 backend (opt-in via SPECTRA_TEST_S3=1)
// ---------------------------------------------------------------------------

func TestS3Store(t *testing.T) {
	if os.Getenv("SPECTRA_TEST_S3") != "1" {
		t.Skip("set SPECTRA_TEST_S3=1 to run S3 tests (requires MinIO or real S3)")
	}

	bucket := os.Getenv("SPECTRA_TEST_S3_BUCKET")
	if bucket == "" {
		bucket = "spectra-test"
	}
	endpoint := os.Getenv("SPECTRA_TEST_S3_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:9000"
	}

	ctx := context.Background()

	var opts []storage.S3Option
	opts = append(opts, storage.WithEndpoint(endpoint))
	opts = append(opts, storage.WithPathStyle(true))

	store, err := storage.NewS3(ctx, bucket, opts...)
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}

	runObjectStoreTests(t, store)
}
