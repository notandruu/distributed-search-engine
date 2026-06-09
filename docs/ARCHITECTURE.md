# Architecture

## Service Overview

### Query Gateway
- Accepts external gRPC search requests
- Normalizes query text
- Checks Redis cache (cache-aside)
- Fans out to all shard services concurrently
- Enforces per-shard deadline (default 75ms)
- Applies global request timeout (default 100ms)
- Merges shard-local top-K results
- Returns globally ranked results with SearchStats
- Emits OTel spans and Prometheus metrics

### Shard Service
- Holds one document partition
- Maintains an in-memory inverted index
- Accepts `Ingest` requests (batched documents)
- Accepts `SearchShard` requests (BM25 query)
- Returns shard-local top-K results
- Protected with RWMutex for concurrent access

### Ingestion CLI
- Reads JSONL documents from stdin or file
- Routes each document: `hash(doc_id) % num_shards`
- Batches documents and sends via gRPC to each shard
- Uses bounded worker pool (configurable concurrency)
- Applies backpressure if shard queues are full

## Sharding Strategy

Documents are assigned to shards by `crc32(doc_id) % num_shards`.

- **Pro**: simple routing, no coordination, easy rebalancing (re-ingest)
- **Con**: fanout on every query (all shards queried for every search)
- **Alternative not chosen**: term sharding (reduces fanout but complicates writes)

## BM25 Parameters

```
k1 = 1.2
b  = 0.75
```

Each shard computes IDF using only its local document set. This is an approximation but avoids cross-shard coordination during query time.

## Cache Strategy

Cache key: `search:v1:{index_version}:{k}:{sha256(normalized_query)}`

- TTL: 5–15 minutes (configurable)
- Do not cache partial-failure responses
- Do not cache empty result sets

## Timeout Model

```
Client ──────────────────────────────────── 100ms total ──────────┐
       │                                                           │
       └→ Gateway ──── 75ms per-shard deadline ──→ Shard 0        │
                   ├── 75ms per-shard deadline ──→ Shard 1        │
                   ├── 75ms per-shard deadline ──→ Shard 2        │
                   └── 75ms per-shard deadline ──→ Shard 3        │
                                                                   │
       ◄────────────────── merge + respond ──────────────────────-┘
```

If a shard misses its deadline, the gateway excludes it from the merge, sets `partial_failure=true`, and returns with the failed shard IDs listed.

## Backpressure

Gateway: bounded semaphore of size `MAX_CONCURRENT_SEARCHES` (default 256).
When saturated: return gRPC `ResourceExhausted` immediately.

Shard: RWMutex protects index. Write lock only during ingest flush; read lock during search.

Ingestion CLI: bounded channel `(workers * 2)` buffered. If channel full, pause reading corpus.
