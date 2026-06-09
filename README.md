# Distributed Search Engine

A distributed full-text search engine built from scratch in Go — sharded inverted indexes, BM25 ranking, concurrent document ingestion, gRPC APIs, Redis query caching, shard fanout, request timeouts, backpressure, OpenTelemetry instrumentation, Docker Compose, and Kubernetes.

[![CI](https://github.com/notandruu/distributed-search-engine/actions/workflows/ci.yml/badge.svg)](https://github.com/notandruu/distributed-search-engine/actions/workflows/ci.yml)

---

## Why not Elasticsearch?

Elasticsearch is a library call. This project implements tokenization, posting lists, BM25 ranking, and top-K heap merging from scratch — all core information-retrieval and distributed-systems concepts that managed search engines abstract away. The goal is to demonstrate the engineering, not the configuration.

---

## Architecture

```
                    ┌──────────────────────────────┐
                    │        Python Tools           │
                    │  corpus gen / eval / qrels    │
                    └──────────────┬───────────────┘
                                   │ JSONL docs
                                   ▼
┌─────────────────────────────────────────────────────────────┐
│                       Ingestion CLI (Go)                    │
│    concurrent workers · crc32 shard routing · batch RPCs   │
└──────────┬─────────────────┬─────────────────┬─────────────┘
           │                 │                 │
           ▼                 ▼                 ▼
    ┌────────────┐   ┌────────────┐   ┌────────────┐
    │  Shard 0   │   │  Shard 1   │ … │  Shard N   │
    │ gRPC / Go  │   │ gRPC / Go  │   │ gRPC / Go  │
    │ inv. index │   │ inv. index │   │ inv. index │
    └─────┬──────┘   └─────┬──────┘   └─────┬──────┘
          │                │                │
          └────────────────┼────────────────┘
                           │ gRPC fanout (concurrent)
                           ▼
                 ┌──────────────────┐
                 │  Query Gateway   │
                 │  Go gRPC service │
                 │  cache · fanout  │
                 │  top-K merge     │
                 └────────┬─────────┘
                          │
         ┌────────────────┼──────────────────┐
         ▼                ▼                  ▼
     ┌────────┐   ┌──────────────┐   ┌────────────────┐
     │ Redis  │   │ OTel Collect.│   │ Prometheus /   │
     │ cache  │   │ traces/spans │   │ Grafana / Jgr  │
     └────────┘   └──────────────┘   └────────────────┘
```

**Document sharding** — each document is routed to a shard via `crc32(doc_id) % num_shards`. Every query fans out to all shards concurrently; results are merged by a min-heap into a global top-K.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Services | Go 1.22+ |
| API | gRPC + Protocol Buffers |
| Caching | Redis 7 |
| Observability | OpenTelemetry SDK, Prometheus, Grafana, Jaeger |
| Containers | Docker, Docker Compose |
| Orchestration | Kubernetes (kind / minikube) |
| Evaluation | Python 3.9+, pytest |
| Load testing | ghz |

---

## Local Quickstart

```bash
git clone https://github.com/notandruu/distributed-search-engine
cd distributed-search-engine

# Build all binaries
make build

# Run all tests
make test

# Race detector
go test -race ./internal/...
```

---

## Docker Compose Quickstart

```bash
# Build images and start 4-shard cluster + Redis + observability
make docker-build
make compose-up

# Wait ~10s for services to be healthy, then index 100K docs
make index-100k

# Query the cluster
grpcurl -plaintext \
  -d '{"query":"distributed consensus replication","top_k":10}' \
  localhost:50051 search.v1.SearchGateway/Search

# Run load test (requires ghz: https://ghz.sh)
make bench

# View observability
# Traces:  http://localhost:16686  (Jaeger)
# Metrics: http://localhost:9090   (Prometheus)
# Grafana: http://localhost:3000   (admin/admin)

# Tear down
make compose-down
```

---

## Kubernetes Quickstart

```bash
# Create a local kind cluster
make k8s-cluster

# Build and load images into kind
make k8s-load-images

# Deploy all manifests
make k8s-up

# Port-forward the gateway
make k8s-forward   # localhost:50051

# Tear down
make k8s-down
```

---

## API Examples

```bash
# Search
grpcurl -plaintext \
  -d '{"query":"distributed systems consensus","top_k":10}' \
  localhost:50051 search.v1.SearchGateway/Search

# Health check
grpcurl -plaintext localhost:50051 search.v1.SearchGateway/Health

# Aggregate stats across shards
grpcurl -plaintext localhost:50051 search.v1.SearchGateway/Stats
```

**Response shape:**
```json
{
  "results": [
    { "doc_id": "doc-0000012", "title": "...", "snippet": "...", "score": 4.87, "shard_id": 2 }
  ],
  "stats": {
    "took_ms": 18,
    "cache_hit": false,
    "shards_queried": 4,
    "shards_succeeded": 4
  },
  "partial_failure": false
}
```

---

## Benchmarks

> Run `make compose-up && make index-1m && make bench && make bench-summary` to generate real numbers.  
> The table below is a placeholder — do not cite it until populated by `bench/BENCHMARK_SUMMARY.md`.

| Corpus | Shards | Concurrency | QPS | p50 | p95 | p99 | Cache hit |
|--------|--------|-------------|-----|-----|-----|-----|-----------|
| 100K docs | 4 | 32 | — | — | — | — | — |
| 1M docs | 4 | 64 | — | — | — | — | — |

*See [docs/BENCHMARKS.md](docs/BENCHMARKS.md) for methodology, hardware, and commands.*

---

## Relevance Evaluation

> Run `make eval` against an indexed cluster to populate.

| Metric | Score |
|--------|-------|
| MRR | — |
| NDCG@10 | — |

*See `bench/eval_summary.md` after running `make eval`.*

---

## Observability

### Traces (Jaeger — http://localhost:16686)

Spans emitted per request:

| Span | Service | Description |
|------|---------|-------------|
| `gateway.Search` | gateway | Full request lifecycle |
| `gateway.RedisGet` | gateway | Cache lookup |
| `gateway.Fanout` | gateway | Concurrent shard dispatch |
| `gateway.MergeTopK` | gateway | Heap merge of shard results |
| `gateway.RedisSet` | gateway | Cache write (background) |
| `shard.SearchShard` | shard | Per-shard BM25 search |
| `shard.Tokenize` | shard | Query tokenization |
| `shard.BM25Score` | shard | Scoring + top-K heap |
| `shard.Ingest` | shard | Document ingestion |

### Metrics (Prometheus — http://localhost:9090)

| Metric | Type | Description |
|--------|------|-------------|
| `search_requests_total` | counter | Requests by status |
| `search_request_duration_seconds` | histogram | Gateway latency |
| `search_cache_hits_total` | counter | Redis cache hits |
| `search_cache_misses_total` | counter | Redis cache misses |
| `search_partial_failures_total` | counter | Degraded responses |
| `backpressure_rejections_total` | counter | Semaphore rejections |
| `shard_search_duration_seconds` | histogram | Per-shard search latency |
| `shard_docs_indexed_total` | counter | Documents indexed |
| `shard_unique_terms` | gauge | Index vocabulary size |
| `shard_postings_total` | gauge | Total posting list entries |
| `ingest_batches_total` | counter | Batches processed |
| `ingest_documents_total` | counter | Documents accepted |

---

## Design Tradeoffs

### Document sharding vs term sharding

**Document sharding** (used here): each shard owns a disjoint set of documents. Every query fans out to all shards. Simple to route, simple to rebalance (re-ingest), but fanout cost grows with shard count.

**Term sharding**: each shard owns a subset of the vocabulary. Reduces fanout for selective queries but complicates document writes (a single doc must update multiple shards) and makes shard failure more impactful.

### Cache-aside strategy

Cache key encodes `query hash + top_k + index_version` so stale results are never served after reindexing. Partial-failure responses are not cached — a degraded result should not poison the cache for future healthy requests. TTL is configurable (default 5 minutes).

### BM25 implementation

k1=1.2, b=0.75 (standard defaults). IDF is computed per-shard using the shard's local document frequency. This is an approximation of global IDF, but avoids a cross-shard coordination round-trip. For production accuracy, a two-phase retrieval (gather term stats, then score) would improve ranking quality.

### Timeouts and partial failure

Global timeout (100ms) wraps the full search path. Per-shard deadline (75ms) is shorter to leave merge headroom. On shard timeout: include results from available shards, set `partial_failure=true`, list failed shards. This trades completeness for availability — correct for a search use case.

### Backpressure

A bounded atomic semaphore caps concurrent gateway searches (default 256). Requests that arrive when the semaphore is full get `ResourceExhausted` immediately instead of queueing — prevents latency from compounding under load. The ingestion CLI uses bounded per-shard channels with the same principle.

---

## Repository Layout

```
distributed-search-engine/
├── cmd/
│   ├── gateway/          # Gateway binary entry point
│   ├── shard/            # Shard binary entry point
│   └── ingest/           # Ingestion CLI entry point
├── internal/
│   ├── tokenizer/        # Unicode-aware lowercasing tokenizer
│   ├── index/            # Inverted index + posting lists
│   ├── ranking/          # BM25 scorer + min-heap top-K
│   ├── shard/            # Shard gRPC server (ingest + search)
│   ├── gateway/          # Gateway gRPC server (fanout + merge + cache)
│   ├── cache/            # Redis cache-aside client
│   ├── ingest/           # Worker pool + shard routing
│   ├── observability/    # OTel tracer + Prometheus metrics
│   └── config/           # Env-driven config structs
├── gen/search/v1/        # Generated protobuf Go code
├── proto/search/v1/      # Protobuf definitions
├── scripts/
│   ├── generate_corpus.py   # Deterministic JSONL corpus generator
│   ├── eval.py              # MRR + NDCG@10 evaluator
│   └── summarize_bench.py   # ghz JSON → markdown summary
├── tests/                # Python test suite (metrics, corpus)
├── eval/                 # Seed queries and qrels
├── bench/                # ghz scripts and results
├── deploy/
│   ├── docker-compose.yml
│   ├── Dockerfile.{gateway,shard,ingest}
│   ├── otel-collector-config.yaml
│   ├── prometheus.yml
│   └── k8s/              # Namespace, Deployments, StatefulSet, HPA, Services
└── docs/
    ├── ARCHITECTURE.md
    ├── BENCHMARKS.md
    └── API.md
```

---

## Resume Bullet Mapping

| Claim | Evidence in this repo |
|---|---|
| Sharded inverted indexes | `internal/index/`, `internal/shard/` — per-shard `InvertedIndex` with posting lists, doc freq, doc lengths |
| BM25 ranking | `internal/ranking/bm25.go` — from-scratch IDF × TF-norm formula, k1=1.2, b=0.75 |
| Concurrent ingestion workers | `internal/ingest/worker.go` — per-shard goroutine pools, bounded channels, crc32 routing |
| gRPC query service | `proto/search/v1/search.proto`, `gen/search/v1/`, `cmd/gateway/`, `cmd/shard/` |
| Redis query caching | `internal/cache/redis.go` — cache-aside, SHA-256 keys, partial-failure exclusion |
| Shard fanout | `internal/gateway/server.go` — concurrent goroutine fanout, per-shard context deadline |
| Request timeouts | `context.WithTimeout` in gateway (100ms global + 75ms per-shard) |
| Backpressure | `atomic.Int64` semaphore → `ResourceExhausted` when saturated |
| MRR + NDCG@10 | `scripts/eval.py` — from-scratch DCG/IDCG/NDCG, `tests/test_metrics.py` (22 tests) |
| OpenTelemetry traces | `internal/observability/otel.go` + spans in gateway + shard |
| Prometheus metrics | `internal/observability/metrics.go` + `/metrics` HTTP endpoints |
| Docker Compose | `deploy/docker-compose.yml` — 4 shards, Redis, OTel, Prometheus, Grafana, Jaeger |
| Kubernetes | `deploy/k8s/` — Deployment, StatefulSet, HPA, Services, kind config |
