# CLAUDE.md

## Project

This repository is a distributed full-text search engine for a Google SWE internship resume project.

The goal is to implement a real distributed information-retrieval system in Go with:
- sharded inverted indexes
- BM25 ranking
- concurrent ingestion workers
- gRPC query service
- Redis query caching
- shard fanout
- request timeouts
- backpressure
- OpenTelemetry traces
- Prometheus-compatible metrics
- Docker Compose
- Kubernetes manifests
- Python evaluation scripts for MRR and NDCG@10
- ghz load tests for p95/p99 latency

## Non-negotiable rules

Do not fake benchmark numbers.

Do not use Elasticsearch, OpenSearch, Lucene, Solr, Meilisearch, Typesense, Tantivy, or any external search engine.

Implement tokenizer, inverted index, BM25 ranking, and top-K heap logic from scratch.

Use Go for production services.

Use Python only for scripts, corpus generation, evaluation, and benchmark summarization.

Every implementation phase must include tests.

When changing protobufs, update generated Go code and any clients.

When adding behavior, update README or docs.

Prefer simple, readable, well-tested code over clever optimizations.

## Build commands

```bash
make proto
make build
make test
make test-race
make docker-build
make compose-up
make compose-down
make generate-corpus
make index-100k
make index-1m
make bench
make eval
```

## Code style

Go:
- use context-aware functions
- return explicit errors
- avoid global mutable state unless guarded
- add table-driven tests
- keep packages small
- use structured logging
- use bounded channels/semaphores for backpressure
- propagate trace context through gRPC

Python:
- use argparse or typer
- deterministic random seeds
- write JSON and markdown outputs
- include small unit tests for metric calculations

## Observability requirements

Required spans:
- gateway.Search
- gateway.RedisGet
- gateway.RedisSet
- gateway.Fanout
- gateway.MergeTopK
- shard.SearchShard
- shard.Tokenize
- shard.BM25Score
- shard.Ingest

Required metrics:
- search_requests_total
- search_request_duration_seconds
- search_cache_hits_total
- search_cache_misses_total
- search_partial_failures_total
- shard_search_duration_seconds
- shard_docs_indexed_total
- shard_unique_terms
- shard_postings_total
- ingest_queue_depth
- backpressure_rejections_total

## Resume alignment

The final repo must truthfully support this resume entry:

**Distributed Search Engine | Go, Python, gRPC, Redis, Docker, Kubernetes, OpenTelemetry**

- Built a distributed information-retrieval system with sharded inverted indexes, BM25 ranking, concurrent document ingestion workers, and a gRPC query service for low-latency full-text search.
- Implemented Redis query caching, shard fanout, request timeouts, and backpressure to sustain 1M+ indexed documents with sub-80ms p95 query latency under concurrent load tests.
- Added relevance evaluation with MRR and NDCG@10, plus OpenTelemetry traces and service metrics to profile ranking, indexing, cache hit rate, and p95/p99 latency bottlenecks.
