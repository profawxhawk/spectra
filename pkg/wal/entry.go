package wal

import (
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

// Entry represents a single span write in the WAL.
type Entry struct {
	TxnID     uint64    `msgpack:"txn_id"`
	TraceID   string    `msgpack:"trace_id"`
	SpanID    string    `msgpack:"span_id"`
	Timestamp time.Time `msgpack:"timestamp"`
	Payload   []byte    `msgpack:"payload"` // msgpack-encoded Span
}

// EncodeEntry serializes a single entry to msgpack.
func EncodeEntry(e *Entry) ([]byte, error) {
	return msgpack.Marshal(e)
}

// DecodeEntry deserializes a single entry from msgpack.
func DecodeEntry(data []byte) (*Entry, error) {
	var e Entry
	if err := msgpack.Unmarshal(data, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// EncodeEntries serializes a batch of entries to msgpack.
func EncodeEntries(entries []*Entry) ([]byte, error) {
	return msgpack.Marshal(entries)
}

// DecodeEntries deserializes a batch of entries from msgpack.
func DecodeEntries(data []byte) ([]*Entry, error) {
	var entries []*Entry
	if err := msgpack.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}
