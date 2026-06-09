# API Reference

## SearchGateway

### Search

```
rpc Search(SearchRequest) returns (SearchResponse)
```

**Request**
```json
{
  "query": "distributed consensus replication",
  "top_k": 10,
  "explain": false
}
```

**Response**
```json
{
  "results": [
    {
      "doc_id": "doc-12",
      "title": "Raft: In Search of an Understandable Consensus Algorithm",
      "snippet": "...distributed consensus replication protocol...",
      "score": 4.872,
      "shard_id": 2
    }
  ],
  "stats": {
    "took_ms": 18,
    "cache_hit": false,
    "shards_queried": 4,
    "shards_succeeded": 4,
    "total_docs": 1000000
  },
  "partial_failure": false,
  "failed_shards": []
}
```

### Health

```
rpc Health(HealthRequest) returns (HealthResponse)
```

Returns `{"status": "SERVING"}` when healthy.

### Stats

```
rpc Stats(StatsRequest) returns (StatsResponse)
```

Returns aggregate stats across all shards.

## ShardService

### Ingest

```
rpc Ingest(IngestRequest) returns (IngestResponse)
```

**Request**
```json
{
  "documents": [
    {"id": "doc-1", "title": "...", "body": "...", "url": "optional"}
  ]
}
```

### SearchShard

```
rpc SearchShard(ShardSearchRequest) returns (ShardSearchResponse)
```

Used internally by the gateway. Not exposed externally.

## gRPC Examples

```bash
# Search
grpcurl -plaintext \
  -d '{"query":"distributed systems","top_k":5}' \
  localhost:50051 search.v1.SearchGateway/Search

# Health
grpcurl -plaintext localhost:50051 search.v1.SearchGateway/Health

# Stats
grpcurl -plaintext localhost:50051 search.v1.SearchGateway/Stats
```
