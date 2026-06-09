package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	searchv1 "github.com/notandruu/distributed-search-engine/gen/search/v1"
)

// Client wraps a Redis connection for query caching.
type Client struct {
	rdb          *redis.Client
	ttl          time.Duration
	indexVersion string
}

// NewClient connects to Redis at addr and returns a Client.
func NewClient(addr, indexVersion string, ttl time.Duration) *Client {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	return &Client{
		rdb:          rdb,
		ttl:          ttl,
		indexVersion: indexVersion,
	}
}

// Close releases the Redis connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Ping checks connectivity.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Get returns the cached SearchResponse for query+topK, or nil if not found.
func (c *Client) Get(ctx context.Context, query string, topK int32) (*searchv1.SearchResponse, error) {
	key := c.cacheKey(query, topK)
	val, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var resp searchv1.SearchResponse
	if err := unmarshalResponse(val, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Set caches the SearchResponse for query+topK. Does not cache partial failures.
func (c *Client) Set(ctx context.Context, query string, topK int32, resp *searchv1.SearchResponse) error {
	if resp.PartialFailure {
		return nil
	}
	key := c.cacheKey(query, topK)
	b, err := marshalResponse(resp)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, b, c.ttl).Err()
}

func (c *Client) cacheKey(query string, topK int32) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(query))))
	return fmt.Sprintf("search:v1:%s:%d:%x", c.indexVersion, topK, h)
}

// marshalResponse serializes a SearchResponse to JSON for caching.
// We use JSON because proto binary is not stable for cache keys.
func marshalResponse(resp *searchv1.SearchResponse) ([]byte, error) {
	type cachedResult struct {
		DocID   string  `json:"doc_id"`
		Title   string  `json:"title"`
		Snippet string  `json:"snippet"`
		Score   float64 `json:"score"`
		ShardID int32   `json:"shard_id"`
	}
	type cachedStats struct {
		TookMs          int64 `json:"took_ms"`
		CacheHit        bool  `json:"cache_hit"`
		ShardsQueried   int32 `json:"shards_queried"`
		ShardsSucceeded int32 `json:"shards_succeeded"`
		TotalDocs       int64 `json:"total_docs"`
	}
	type cachedResp struct {
		Results []cachedResult `json:"results"`
		Stats   *cachedStats   `json:"stats,omitempty"`
	}

	cr := cachedResp{Results: make([]cachedResult, 0, len(resp.Results))}
	for _, r := range resp.Results {
		cr.Results = append(cr.Results, cachedResult{
			DocID:   r.DocId,
			Title:   r.Title,
			Snippet: r.Snippet,
			Score:   r.Score,
			ShardID: r.ShardId,
		})
	}
	if resp.Stats != nil {
		cr.Stats = &cachedStats{
			TookMs:          resp.Stats.TookMs,
			CacheHit:        true,
			ShardsQueried:   resp.Stats.ShardsQueried,
			ShardsSucceeded: resp.Stats.ShardsSucceeded,
			TotalDocs:       resp.Stats.TotalDocs,
		}
	}
	return json.Marshal(cr)
}

func unmarshalResponse(b []byte, resp *searchv1.SearchResponse) error {
	type cachedResult struct {
		DocID   string  `json:"doc_id"`
		Title   string  `json:"title"`
		Snippet string  `json:"snippet"`
		Score   float64 `json:"score"`
		ShardID int32   `json:"shard_id"`
	}
	type cachedStats struct {
		TookMs          int64 `json:"took_ms"`
		CacheHit        bool  `json:"cache_hit"`
		ShardsQueried   int32 `json:"shards_queried"`
		ShardsSucceeded int32 `json:"shards_succeeded"`
		TotalDocs       int64 `json:"total_docs"`
	}
	type cachedResp struct {
		Results []cachedResult `json:"results"`
		Stats   *cachedStats   `json:"stats,omitempty"`
	}

	var cr cachedResp
	if err := json.Unmarshal(b, &cr); err != nil {
		return err
	}

	resp.Results = make([]*searchv1.SearchResult, 0, len(cr.Results))
	for _, r := range cr.Results {
		resp.Results = append(resp.Results, &searchv1.SearchResult{
			DocId:   r.DocID,
			Title:   r.Title,
			Snippet: r.Snippet,
			Score:   r.Score,
			ShardId: r.ShardID,
		})
	}
	if cr.Stats != nil {
		resp.Stats = &searchv1.SearchStats{
			TookMs:          cr.Stats.TookMs,
			CacheHit:        true,
			ShardsQueried:   cr.Stats.ShardsQueried,
			ShardsSucceeded: cr.Stats.ShardsSucceeded,
			TotalDocs:       cr.Stats.TotalDocs,
		}
	}
	return nil
}
