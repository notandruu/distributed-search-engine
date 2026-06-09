# Distributed Search Engine

A distributed full-text search engine built from scratch in Go — sharded inverted indexes, BM25 ranking, gRPC APIs, Redis caching, and OpenTelemetry instrumentation.

[![CI](https://github.com/notandruu/distributed-search-engine/actions/workflows/ci.yml/badge.svg)](https://github.com/notandruu/distributed-search-engine/actions/workflows/ci.yml)

## Why not Elasticsearch?

This project intentionally implements tokenization, inverted indexes, BM25 ranking, and top-K merging from scratch. The goal is to demonstrate information retrieval and distributed systems fundamentals — not to configure a managed search system.

## Architecture

```
                    ┌─────────────────────────────┐
                    │        Python Tools          │
                    │ corpus gen / eval / qrels    │
                    └──────────────┬──────────────┘
                                   │ JSONL docs
                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Ingestion CLI                           │
│       concurrent workers, bounded queue, docID hash router       │
└──────────────┬──────────────────┬──────────────────┬────────────┘
               │                  │                  │
               ▼                  ▼                  ▼
       ┌──────────────┐    ┌──────────────┐   ┌──────────────┐
       │ Shard 0      │    │ Shard 1      │   │ Shard N      │
       │ Go gRPC svc  │    │ Go gRPC svc  │   │ Go gRPC svc  │
       │ inverted idx │    │ inverted idx │   │ inverted idx │
       └──────┬───────┘    └──────┬───────┘   └──────┬───────┘
              │                   │                  │
              └───────────────────┼──────────────────┘
                                  │ gRPC fanout
                                  ▼
                        ┌──────────────────┐
                        │ Query Gateway    │
                        │ Go gRPC service  │
                        │ cache + fanout   │
                        │ top-k merge      │
                        └───────┬──────────┘
                                │
              ┌─────────────────┼───────────────────┐
              ▼                 ▼                   ▼
          ┌────────┐    ┌──────────────┐     ┌──────────────┐
          │ Redis  │    │ OTel Collector│     │ Prom/Grafana │
          │ cache  │    │ traces/logs   │     │ metrics      │
          └────────┘    └──────────────┘     └──────────────┘
```

**Document sharding** — each document is routed to a shard by `hash(doc_id) % num_shards`. The gateway fans every query out to all shards concurrently and merges the top-K results globally.

## Tech Stack

| Layer | Technology |
|---|---|
| Services | Go 1.22+ |
| API | gRPC + Protocol Buffers |
| Caching | Redis |
| Observability | OpenTelemetry, Prometheus, Grafana, Tempo |
| Containers | Docker, Docker Compose |
| Orchestration | Kubernetes (kind/minikube) |
| Evaluation | Python 3.11+ |
| Load testing | ghz |

## Local Quickstart

```bash
# Build all binaries
make build

# Run tests
make test
make test-race
```

## Docker Compose Quickstart

```bash
# Start 4-shard cluster + Redis + observability stack
make compose-up

# Index 100K documents
make index-100k

# Run load test
make bench

# View traces at http://localhost:16686 (Jaeger)
# View metrics at http://localhost:3000 (Grafana)

# Tear down
make compose-down
```

## Kubernetes Quickstart

```bash
# Requires kind or minikube
make k8s-up

# Port-forward gateway
make k8s-forward

# Tear down
make k8s-down
```

## API Examples

```bash
# gRPC search via grpcurl
grpcurl -plaintext \
  -d '{"query":"distributed consensus replication","top_k":10}' \
  localhost:50051 \
  search.v1.SearchGateway/Search

# Health check
grpcurl -plaintext localhost:50051 search.v1.SearchGateway/Health

# Stats
grpcurl -plaintext localhost:50051 search.v1.SearchGateway/Stats
```

## Benchmarks

> Benchmarks are run after full indexing. See [docs/BENCHMARKS.md](docs/BENCHMARKS.md) for methodology and hardware.

| Corpus | Shards | Concurrency | QPS | p50 | p95 | p99 | Cache hit |
|--------|--------|-------------|-----|-----|-----|-----|-----------|
| 100K docs | 4 | 32 | TBD | TBD | TBD | TBD | TBD |
| 1M docs | 4 | 64 | TBD | TBD | TBD | TBD | TBD |

*Run `make bench` to populate with real numbers.*

## Relevance Evaluation

| Metric | Score |
|--------|-------|
| MRR | TBD |
| NDCG@10 | TBD |

*Run `make eval` to populate.*

## Observability

- **Traces**: Jaeger at `http://localhost:16686` — spans for gateway.Search, shard fanout, Redis get/set, BM25 scoring
- **Metrics**: Grafana at `http://localhost:3000` — QPS, cache hit ratio, p95/p99 latency, shard doc counts
- **Logs**: Structured JSON with trace_id correlation

## Design Tradeoffs

**Document sharding vs term sharding** — document sharding is simpler to implement and rebalance. Term sharding reduces fanout for selective queries but complicates document insertion and requires a more complex routing layer.

**Cache-aside** — the gateway checks Redis before fanning out. Cache keys encode query hash + top_k + index version. TTL is 5–15 minutes. Partial-failure responses are not cached to avoid returning stale degraded results.

**BM25 implementation** — k1=1.2, b=0.75 defaults. Each shard computes local IDF using its own document set. A global IDF approach would be more accurate but requires a coordination round-trip during ingestion.

**Timeout and partial failure** — per-shard deadline defaults to 75ms. If a shard misses its deadline, the gateway returns results from available shards with `partial_failure=true` and logs which shard failed.

**Backpressure** — a bounded semaphore in the gateway enforces max concurrent searches. When saturated, the gateway returns gRPC `ResourceExhausted` immediately rather than queueing indefinitely.

## Repository Layout

```
distributed-search-engine/
├── cmd/gateway/         # Gateway entry point
├── cmd/shard/           # Shard service entry point
├── cmd/ingest/          # Ingestion CLI
├── internal/
│   ├── tokenizer/       # Text tokenization
│   ├── index/           # Inverted index + posting lists
│   ├── ranking/         # BM25 scoring + top-K heap
│   ├── shard/           # Shard service logic
│   ├── gateway/         # Fanout, merge, cache
│   ├── cache/           # Redis cache-aside
│   ├── ingest/          # Worker pool + routing
│   ├── observability/   # OTel + Prometheus setup
│   └── config/          # Config structs
├── proto/search/v1/     # Protobuf definitions
├── gen/search/v1/       # Generated Go code
├── scripts/             # Python corpus gen + eval
├── eval/                # Queries and qrels
├── bench/               # ghz scripts + results
├── deploy/              # Dockerfiles + Docker Compose + k8s
└── docs/                # PRD, architecture, benchmarks
```
