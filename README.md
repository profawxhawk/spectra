# Spectra - AI Trace Storage Engine

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8.svg)](https://go.dev)

A production-grade database optimized for AI observability workloads. Spectra handles massive semi-structured traces (LLM prompts, agent outputs, tool calls) with a tiered storage architecture designed for high write throughput, immediate read consistency, and full-text search across terabytes of trace data.

## Architecture

```
                         ┌───────────────┐
                         │    Redis      │
                         │  (txn IDs)    │
                         └──────┬────────┘
                                │
  Ingest API ──→ WAL Writer ────┼──→ WAL files (Object Storage)
                    │           │
                    ▼           │
            ┌──────────────┐   │     Background Workers:
            │  MemIndex     │   │
            │ (ID→location) │   │     ┌─────────────────────┐
            └──────────────┘   ├────→│  Compactor           │
                               │     │  WAL → Segments      │
                               │     └─────────┬───────────┘
                               │               ▼
                               │     ┌─────────────────────┐
                               ├────→│  Indexer             │
                               │     │  Segments → Search   │
                               │     └─────────────────────┘
                               │
                         ┌─────┴─────────┐
                         │   Postgres     │
                         │ (segment map)  │
                         └───────────────┘

  Query API ──→ Planner ──→ Merge(MemIndex + WAL + Segments + Index)
                              ──→ Stream results
```

### Data Flow (Lifecycle of a Trace)

| Stage | Storage | Latency | Queryable? |
|-------|---------|---------|------------|
| 1. **MemIndex** | In-memory (IDs + timestamps) | <1ms | Yes — instant lookup by trace ID |
| 2. **WAL** | Object storage (S3/filesystem) | ~5ms | Yes — scan WAL files |
| 3. **Segment** | Object storage (co-located by trace) | ~50ms | Yes — read single file per trace |
| 4. **Full Index** | Local disk (bloom filters on S3) | ~100ms | Yes — full-text search |

**Key insight**: Data is queryable at every stage. Reads never wait for compaction or indexing.

## Quick Start

### Prerequisites

- Go 1.22+
- Docker & Docker Compose (for Postgres + Redis)

### Run Locally

```bash
# Start infrastructure
docker-compose up -d

# Build and run
make build
./bin/spectra serve

# Or use filesystem-only mode (no Postgres/Redis needed for basic usage)
./bin/spectra serve --config spectra.yaml
```

### Ingest Spans

```bash
curl -X POST http://localhost:8080/v1/spans \
  -H "Content-Type: application/json" \
  -d '{
    "spans": [
      {
        "span_id": "s1",
        "trace_id": "trace-001",
        "name": "gpt4_call",
        "kind": "llm",
        "start_time": "2024-01-15T10:30:00Z",
        "input": "What is machine learning?",
        "output": "ML is a branch of AI...",
        "metrics": {"latency_ms": 1500, "prompt_tokens": 50, "completion_tokens": 200}
      }
    ]
  }'
```

### Query

```bash
# Get a trace by ID
curl http://localhost:8080/v1/traces/trace-001

# Full-text search
curl -X POST http://localhost:8080/v1/search \
  -d '{"query": "machine learning", "limit": 50}'

# Structured query with filters
curl -X POST http://localhost:8080/v1/query \
  -d '{"filters": [{"field": "kind", "operator": "eq", "value": "llm"}], "search": "error", "limit": 100}'
```

### Run Tests

```bash
make test          # 59 tests, all with -race
make build         # compile binary
make lint          # golangci-lint (install separately)
```

## Project Structure

```
spectra/
├── cmd/spectra/main.go          # CLI entrypoint (cobra)
├── pkg/
│   ├── config/                  # YAML configuration
│   ├── model/                   # Trace, Span, Query types
│   ├── storage/                 # ObjectStore interface (FS + S3)
│   ├── wal/                     # Write-ahead log (entry, writer, reader)
│   ├── memindex/                # In-memory trace→location index
│   ├── meta/                    # Postgres metadata store + migrations
│   ├── segment/                 # Segment builder, reader, compactor
│   ├── index/                   # Full-text indexer + bloom filters
│   ├── query/                   # Query planner (merges 4 data layers)
│   ├── server/                  # HTTP REST API
│   └── engine/                  # Top-level orchestrator
├── internal/testutil/           # End-to-end integration tests
├── proto/spectra/v1/            # Protobuf definitions (future gRPC)
├── docker-compose.yaml          # Postgres + Redis + MinIO
├── Dockerfile                   # Multi-stage production build
└── Makefile                     # build, test, lint, run, docker
```

## Package Details

### Storage Layer (`pkg/storage`)

The `ObjectStore` interface abstracts object storage:

```go
type ObjectStore interface {
    Put(ctx context.Context, key string, data []byte) error
    PutReader(ctx context.Context, key string, r io.Reader) error
    Get(ctx context.Context, key string) ([]byte, error)
    Delete(ctx context.Context, key string) error
    List(ctx context.Context, prefix string) ([]string, error)
    Exists(ctx context.Context, key string) (bool, error)
}
```

Two implementations:
- **`FSStore`** — filesystem-backed, great for local dev (`storage.NewFS("/tmp/spectra")`)
- **`S3Store`** — AWS S3 / MinIO / any S3-compatible store

### WAL (`pkg/wal`)

Write-ahead log for durable, high-throughput ingestion:
- **Writer**: Buffers spans, auto-flushes at `maxBatchSize` or `flushInterval`
- **Reader**: Scans WAL files, reads individual spans
- **Entry format**: msgpack-encoded `{TxnID, TraceID, SpanID, Timestamp, Payload}`
- **Key format**: `wal/{nodeID}/{timestamp_ns}_{txnID}.wal`

### Segments (`pkg/segment`)

Compacted, trace-colocated storage:
- **Builder**: Groups WAL entries by trace ID, sorts by timestamp within each trace
- **Reader**: Fetches segments from object storage, extracts spans per trace
- **Compactor**: Background goroutine that reads unprocessed WAL files, builds segments, registers in metadata store

### Index (`pkg/index`)

Full-text search and fast existence checks:
- **Indexer**: Background worker that reads unindexed segments, indexes spans in memory, builds bloom filters
- **Search**: Full-text search across `input`, `output`, `name` fields; field-level search by `trace_id`, `kind`, `status`
- **Bloom filters**: Per-segment probabilistic filter using xxhash double-hashing for fast "does this segment contain trace X?"

### Query Engine (`pkg/query`)

Merges results from all 4 data layers:

```
Query → Planner → Plan{sources: [MemIndex, WAL, Segment, Index]}
                      ↓
                  Executor → merge + dedup + filter + sort + paginate
                      ↓
                  QueryResult{spans, total_count, has_more}
```

For `trace_id` lookups: MemIndex → Meta/Segment → WAL scan
For full-text search: Index → Unindexed segments → WAL scan

### Metadata Store (`pkg/meta`)

Postgres-backed metadata tracking:
- **Segment manifest**: which segments exist, their time ranges, indexing status
- **Trace mapping**: which segment contains each trace (trace_id → segment_id + offset)
- **WAL tracking**: which WAL files have been compacted

## HA / Distributed Setup

Spectra nodes are **stateless** — all durable state lives in S3 + Postgres + Redis:

```
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │ Spectra  │ │ Spectra  │ │ Spectra  │
        │ (ingest) │ │ (ingest) │ │ (query)  │
        └────┬─────┘ └────┬─────┘ └────┬─────┘
             └─────────────┴─────────────┘
                    Shared: S3 + Postgres + Redis
```

### Node Roles

Configure in `spectra.yaml`:

```yaml
node:
  roles: ["ingest", "compact", "index", "query"]  # all roles (dev)
  # roles: ["ingest"]           # ingest-only (prod)
  # roles: ["compact", "index"] # background worker (prod)
  # roles: ["query"]            # query-only (prod)
```

| Role | Scaling | Leader Election |
|------|---------|-----------------|
| `ingest` | Horizontal (behind LB) | None needed |
| `compact` | 1-2 nodes | Redis `SETNX` with TTL |
| `index` | 1-2 nodes | Redis `SETNX` with TTL |
| `query` | Horizontal (behind LB) | None needed |

### Failure Modes

| Failure | Impact | Recovery |
|---------|--------|----------|
| Ingest node dies | WAL files already on S3 | LB routes to healthy nodes |
| Compactor dies | WAL files accumulate, reads still work | Another node acquires lock after 30s |
| Indexer dies | Segments unindexed, reads still work | Another node acquires lock |
| Query node dies | No state lost | LB routes to healthy nodes |
| Postgres down | No new registrations | Reconnect on recovery |
| S3 down | Full outage | Wait for S3 recovery |

## Configuration

Default `spectra.yaml`:

```yaml
node:
  node_id: ""                      # auto-generated if empty
  roles: [ingest, compact, index, query]
  grpc_addr: ":6666"
  http_addr: ":8080"

storage:
  backend: "fs"                    # "fs" or "s3"
  base_path: "/tmp/spectra/data"
  s3_bucket: ""
  s3_region: "us-east-1"
  s3_endpoint: ""                  # for MinIO

meta:
  dsn: "postgres://spectra:spectra@localhost:5432/spectra?sslmode=disable"

redis:
  addr: "localhost:6379"

wal:
  flush_interval: 1s
  max_batch_size: 1000

segment:
  target_size: 52428800            # 50MB
  max_spans: 100000
  compact_interval: 5s

index:
  index_interval: 5s
  base_path: "/tmp/spectra/index"
```

## Data Model

### Span

```go
type Span struct {
    SpanID     string                 // unique span identifier
    TraceID    string                 // trace this span belongs to
    ParentID   string                 // parent span (optional)
    Name       string                 // e.g., "gpt4_call", "tool_search"
    Kind       SpanKind               // llm, tool, agent, retriever, chain, generic
    StartTime  time.Time
    EndTime    time.Time
    Status     SpanStatus             // ok, error
    Input      string                 // LLM prompt, tool input
    Output     string                 // LLM response, tool output
    Metadata   map[string]string      // structured key-value pairs
    Attributes map[string]interface{} // semi-structured data
    Events     []Event                // timestamped annotations
    Metrics    SpanMetrics            // latency, tokens, cost
}
```

### SpanKind Values

| Kind | Use Case |
|------|----------|
| `llm` | LLM API calls (GPT, Claude, etc.) |
| `tool` | Tool/function calls |
| `agent` | Agent orchestration steps |
| `retriever` | RAG retrieval operations |
| `chain` | Chain/pipeline steps |
| `generic` | Everything else |

## Dependencies

| Library | Purpose |
|---------|---------|
| `spf13/cobra` | CLI framework |
| `aws/aws-sdk-go-v2` | S3 object storage |
| `jackc/pgx/v5` | PostgreSQL driver |
| `redis/go-redis/v9` | Redis client |
| `vmihailenco/msgpack/v5` | Binary serialization (WAL + segments) |
| `cespare/xxhash/v2` | Fast hashing (bloom filters) |
| `uber-go/zap` | Structured logging |
| `stretchr/testify` | Test assertions |

## Test Coverage

```
59 tests across 9 packages:
  pkg/model     — 9 tests  (Span validation, msgpack round-trip, query validation)
  pkg/storage   — 9 tests  (FS put/get/delete/list/exists, concurrent, S3 skip)
  pkg/meta      — 7 tests  (Segment CRUD, trace mapping, WAL lifecycle)
  pkg/wal       — 6 tests  (Entry codec, writer flush, reader, span lookup)
  pkg/memindex  — 5 tests  (Add/lookup, eviction, concurrent access)
  pkg/segment   — 6 tests  (Build, encode/decode, offsets, reader, compactor)
  pkg/index     — 5 tests  (Bloom filter, false positive rate, search, trace search)
  pkg/query     — 3 tests  (Trace lookup, field filter, pagination)
  internal/     — 4 tests  (Full pipeline e2e, late update, HTTP, concurrent ingest)
  Total: 54 PASS (all with -race detector)
```

## License

Apache License 2.0 — see [LICENSE](LICENSE).
