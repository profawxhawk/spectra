<p align="center">
  <h1 align="center">Spectra</h1>
  <p align="center">
    A production-grade database optimized for AI observability workloads.
    <br />
    High-throughput trace ingestion, tiered storage, and full-text search across LLM prompts, agent outputs, and tool calls.
  </p>
  <p align="center">
    <a href="https://github.com/profawxhawk/spectra/actions"><img src="https://github.com/profawxhawk/spectra/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
    <a href="https://goreportcard.com/report/github.com/profawxhawk/spectra"><img src="https://goreportcard.com/badge/github.com/profawxhawk/spectra" alt="Go Report Card"></a>
    <a href="https://opensource.org/licenses/Apache-2.0"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License"></a>
    <a href="https://pkg.go.dev/github.com/profawxhawk/spectra"><img src="https://pkg.go.dev/badge/github.com/profawxhawk/spectra.svg" alt="Go Reference"></a>
    <a href="https://go.dev"><img src="https://img.shields.io/badge/Go-1.22+-00ADD8.svg" alt="Go Version"></a>
  </p>
</p>

---

## Why Spectra?

Existing observability databases (Jaeger, Tempo) are built for traditional distributed tracing — small, structured spans. AI workloads are different:

- **Massive payloads** — LLM prompts and responses can be 100KB+ per span
- **Semi-structured data** — tool call arguments, agent reasoning chains, retrieval contexts
- **Full-text search** — "find all traces where the model mentioned X"
- **Immediate consistency** — data must be queryable the instant it's ingested, not after a pipeline delay

Spectra is purpose-built for these workloads.

## Architecture

```mermaid
graph TD
    A[Ingest API] --> B[WAL Writer]
    B --> C[(Object Storage<br/>WAL Files)]
    B --> D[MemIndex<br/>ID → Location]
    B --> E[(Redis<br/>Txn IDs)]

    C --> F[Compactor<br/>WAL → Segments]
    F --> G[(Object Storage<br/>Segments)]
    F --> H[(Postgres<br/>Segment Map)]

    G --> I[Indexer<br/>Segments → Search]
    I --> J[Search Index<br/>Bloom Filters]

    K[Query API] --> L[Planner]
    L --> D
    L --> C
    L --> G
    L --> J
    L --> M[Stream Results]
```

### Data Flow

Every trace passes through 4 stages. **Data is queryable at every stage** — reads never wait for compaction or indexing.

```mermaid
graph LR
    A["1. MemIndex<br/><small>In-memory | &lt;1ms</small>"] --> B["2. WAL<br/><small>Object storage | ~5ms</small>"]
    B --> C["3. Segment<br/><small>Object storage | ~50ms</small>"]
    C --> D["4. Full Index<br/><small>Bloom filters | ~100ms</small>"]

    style A fill:#2d6a4f,color:#fff
    style B fill:#40916c,color:#fff
    style C fill:#52b788,color:#fff
    style D fill:#74c69d,color:#000
```

| Stage | Storage | Latency | Queryable? |
|-------|---------|---------|------------|
| **MemIndex** | In-memory (IDs + timestamps) | <1ms | Yes — instant lookup by trace ID |
| **WAL** | Object storage (S3/filesystem) | ~5ms | Yes — scan WAL files |
| **Segment** | Object storage (co-located by trace) | ~50ms | Yes — read single file per trace |
| **Full Index** | Local disk (bloom filters on S3) | ~100ms | Yes — full-text search |

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

# Or use filesystem-only mode (no Postgres/Redis needed)
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
        "metrics": {
          "latency_ms": 1500,
          "prompt_tokens": 50,
          "completion_tokens": 200
        }
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
  -d '{
    "filters": [{"field": "kind", "operator": "eq", "value": "llm"}],
    "search": "error",
    "limit": 100
  }'
```

### Run Tests

```bash
make test          # all tests with -race detector
make build         # compile binary
make lint          # golangci-lint
```

## Web UI

Spectra includes a built-in web interface for exploring traces, searching spans, and monitoring system health.

### Start the Frontend

```bash
cd web
npm install
npm run dev
```

The UI is available at [http://localhost:5173](http://localhost:5173) and proxies API requests to the backend on `:8080`.

### Features

- **Dashboard** — Stats cards, span volume chart, error rates, latency percentiles (P50/P90/P95/P99), and span kind breakdown
- **Explorer** — Sortable data table with faceted filtering by kind and status, full-text search, pagination, and span detail panel
- **Trace Detail** — Waterfall timeline and span tree with parent-child hierarchy, token counts, and input/output viewer
- **Live Tail** — Real-time span streaming with pause/resume, filtering, and 500-span rolling buffer
- **Search** — Full-text search across span inputs, outputs, and names with result cards

### Tech Stack

React, TypeScript, Vite, TailwindCSS, shadcn/ui, TanStack Query, Recharts, React Router

## Project Structure

```
spectra/
├── cmd/spectra/main.go          # CLI entrypoint (cobra)
├── web/                         # React frontend (Vite + TypeScript)
│   ├── src/
│   │   ├── components/          # UI components (dashboard, explorer, trace, live)
│   │   ├── hooks/               # React Query data hooks
│   │   ├── pages/               # Route pages
│   │   ├── lib/                 # API client, utilities
│   │   └── types/               # TypeScript type definitions
│   └── package.json
├── pkg/
│   ├── config/                  # YAML configuration
│   ├── model/                   # Trace, Span, Query types
│   ├── storage/                 # ObjectStore interface (FS + S3)
│   ├── wal/                     # Write-ahead log
│   ├── memindex/                # In-memory trace → location index
│   ├── meta/                    # Postgres metadata store + migrations
│   ├── segment/                 # Segment builder, reader, compactor
│   ├── index/                   # Full-text indexer + bloom filters
│   ├── query/                   # Query planner (merges 4 data layers)
│   ├── server/                  # HTTP REST API
│   └── engine/                  # Top-level orchestrator
├── internal/testutil/           # End-to-end integration tests
├── docker-compose.yaml          # Postgres + Redis + MinIO
├── Dockerfile                   # Multi-stage production build
└── Makefile                     # build, test, lint, run
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

## HA / Distributed Setup

Spectra nodes are **stateless** — all durable state lives in S3 + Postgres + Redis.

```mermaid
graph TD
    LB[Load Balancer] --> N1[Spectra<br/>ingest]
    LB --> N2[Spectra<br/>ingest]
    LB --> N3[Spectra<br/>query]

    BG1[Spectra<br/>compact + index] --> Shared
    N1 --> Shared
    N2 --> Shared
    N3 --> Shared

    subgraph Shared["Shared Infrastructure"]
        S3[(S3)]
        PG[(Postgres)]
        RD[(Redis)]
    end
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

## Query Engine

The query planner merges results across all 4 data layers automatically:

```mermaid
graph TD
    Q[Query] --> P[Planner]
    P --> Plan

    subgraph Plan["Execution Plan"]
        MI[MemIndex]
        WAL[WAL Files]
        SEG[Segments]
        IDX[Search Index]
    end

    Plan --> E[Executor<br/>merge + dedup + filter + sort + paginate]
    E --> R["QueryResult<br/>{spans, total_count, has_more}"]
```

- **Trace ID lookup**: MemIndex → Meta/Segment → WAL scan
- **Full-text search**: Index → Unindexed segments → WAL scan

## Dependencies

| Library | Purpose |
|---------|---------|
| [`spf13/cobra`](https://github.com/spf13/cobra) | CLI framework |
| [`aws/aws-sdk-go-v2`](https://github.com/aws/aws-sdk-go-v2) | S3 object storage |
| [`jackc/pgx/v5`](https://github.com/jackc/pgx) | PostgreSQL driver |
| [`redis/go-redis/v9`](https://github.com/redis/go-redis) | Redis client |
| [`vmihailenco/msgpack/v5`](https://github.com/vmihailenco/msgpack) | Binary serialization |
| [`cespare/xxhash/v2`](https://github.com/cespare/xxhash) | Fast hashing (bloom filters) |
| [`uber-go/zap`](https://github.com/uber-go/zap) | Structured logging |
| [`stretchr/testify`](https://github.com/stretchr/testify) | Test assertions |

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Please make sure tests pass before submitting:

```bash
make test
make lint
```

## License

Distributed under the Apache License 2.0. See [`LICENSE`](LICENSE) for details.
