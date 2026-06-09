package gateway

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	searchv1 "github.com/notandruu/distributed-search-engine/gen/search/v1"
	"github.com/notandruu/distributed-search-engine/internal/cache"
)

// shardClient wraps a gRPC client for one shard.
type shardClient struct {
	id   int
	addr string
	conn *grpc.ClientConn
	stub searchv1.ShardServiceClient
}

// Server implements the SearchGateway gRPC interface.
type Server struct {
	searchv1.UnimplementedSearchGatewayServer

	shards          []*shardClient
	cache           *cache.Client
	maxConcurrent   int64
	activeSem       atomic.Int64
	searchTimeoutMS int
	shardTimeoutMS  int

	mu sync.RWMutex // guards shards after construction
}

// Options configures a new Server.
type Options struct {
	ShardAddrs      []string
	Cache           *cache.Client
	MaxConcurrent   int
	SearchTimeoutMS int
	ShardTimeoutMS  int
}

func NewServer(opts Options) (*Server, error) {
	if opts.MaxConcurrent <= 0 {
		opts.MaxConcurrent = 256
	}
	if opts.SearchTimeoutMS <= 0 {
		opts.SearchTimeoutMS = 100
	}
	if opts.ShardTimeoutMS <= 0 {
		opts.ShardTimeoutMS = 75
	}

	srv := &Server{
		cache:           opts.Cache,
		maxConcurrent:   int64(opts.MaxConcurrent),
		searchTimeoutMS: opts.SearchTimeoutMS,
		shardTimeoutMS:  opts.ShardTimeoutMS,
	}

	for i, addr := range opts.ShardAddrs {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, fmt.Errorf("dial shard %d at %s: %w", i, addr, err)
		}
		srv.shards = append(srv.shards, &shardClient{
			id:   i,
			addr: addr,
			conn: conn,
			stub: searchv1.NewShardServiceClient(conn),
		})
	}

	return srv, nil
}

// Close releases all gRPC connections and the cache client.
func (s *Server) Close() {
	for _, sc := range s.shards {
		sc.conn.Close()
	}
	if s.cache != nil {
		s.cache.Close()
	}
}

func (s *Server) Search(ctx context.Context, req *searchv1.SearchRequest) (*searchv1.SearchResponse, error) {
	if req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query must not be empty")
	}
	if req.TopK <= 0 {
		req.TopK = 10
	}

	// Backpressure: reject when at capacity.
	cur := s.activeSem.Add(1)
	defer s.activeSem.Add(-1)
	if cur > s.maxConcurrent {
		return nil, status.Error(codes.ResourceExhausted, "gateway at capacity")
	}

	start := time.Now()

	// Apply global search timeout.
	searchCtx, cancel := context.WithTimeout(ctx, time.Duration(s.searchTimeoutMS)*time.Millisecond)
	defer cancel()

	// Cache-aside: check Redis first.
	if s.cache != nil {
		if cached, err := s.cache.Get(searchCtx, req.Query, req.TopK); err == nil && cached != nil {
			cached.Stats.TookMs = time.Since(start).Milliseconds()
			cached.Stats.CacheHit = true
			return cached, nil
		}
	}

	// Fan out to all shards.
	results, failedShards := s.fanout(searchCtx, req)

	took := time.Since(start).Milliseconds()
	resp := &searchv1.SearchResponse{
		Results:        results,
		PartialFailure: len(failedShards) > 0,
		FailedShards:   failedShards,
		Stats: &searchv1.SearchStats{
			TookMs:          took,
			CacheHit:        false,
			ShardsQueried:   int32(len(s.shards)),
			ShardsSucceeded: int32(len(s.shards) - len(failedShards)),
		},
	}

	// Populate cache on success (non-partial, background).
	if s.cache != nil && !resp.PartialFailure {
		cacheCtx, ccancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer ccancel()
		_ = s.cache.Set(cacheCtx, req.Query, req.TopK, resp)
	}

	return resp, nil
}

type shardResult struct {
	results []*searchv1.SearchResult
	shardID int
	err     error
}

// fanout sends SearchShard to all shards concurrently and returns globally merged top-K results.
func (s *Server) fanout(ctx context.Context, req *searchv1.SearchRequest) ([]*searchv1.SearchResult, []string) {
	if len(s.shards) == 0 {
		return nil, nil
	}

	resultCh := make(chan shardResult, len(s.shards))

	for _, sc := range s.shards {
		go func(sc *shardClient) {
			shardCtx, cancel := context.WithTimeout(ctx, time.Duration(s.shardTimeoutMS)*time.Millisecond)
			defer cancel()

			resp, err := sc.stub.SearchShard(shardCtx, &searchv1.ShardSearchRequest{
				Query:   req.Query,
				TopK:    req.TopK,
				Explain: req.Explain,
			})
			if err != nil {
				resultCh <- shardResult{shardID: sc.id, err: err}
				return
			}
			resultCh <- shardResult{results: resp.Results, shardID: sc.id}
		}(sc)
	}

	// Collect all shard results.
	allResults := make([]*searchv1.SearchResult, 0, int(req.TopK)*len(s.shards))
	var failedShards []string

	for range s.shards {
		r := <-resultCh
		if r.err != nil {
			failedShards = append(failedShards, s.shards[r.shardID].addr)
			continue
		}
		allResults = append(allResults, r.results...)
	}

	// Merge into global top-K using a min-heap.
	merged := topKMergeProto(allResults, int(req.TopK))
	return merged, failedShards
}

// resultMinHeap is a min-heap of SearchResult pointers ordered by score ascending.
// The root always holds the lowest score — we pop it when a higher score arrives.
type resultMinHeap []*searchv1.SearchResult

func (h resultMinHeap) Len() int            { return len(h) }
func (h resultMinHeap) Less(i, j int) bool  { return h[i].Score < h[j].Score }
func (h resultMinHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *resultMinHeap) Push(x any)         { *h = append(*h, x.(*searchv1.SearchResult)) }
func (h *resultMinHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// topKMergeProto selects the top-K SearchResults by score descending using a min-heap.
func topKMergeProto(results []*searchv1.SearchResult, k int) []*searchv1.SearchResult {
	if k <= 0 || len(results) == 0 {
		return nil
	}

	h := &resultMinHeap{}
	heap.Init(h)

	for _, r := range results {
		if h.Len() < k {
			heap.Push(h, r)
		} else if r.Score > (*h)[0].Score {
			heap.Pop(h)
			heap.Push(h, r)
		}
	}

	// Drain heap into result slice (heap is min-heap, so drain gives ascending order).
	out := make([]*searchv1.SearchResult, h.Len())
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = heap.Pop(h).(*searchv1.SearchResult)
	}
	return out
}

// topKMerge is used by tests without proto types.
func topKMerge(results []*searchv1.SearchResult, k int) []*searchv1.SearchResult {
	return topKMergeProto(results, k)
}

func (s *Server) Health(ctx context.Context, req *searchv1.HealthRequest) (*searchv1.HealthResponse, error) {
	return &searchv1.HealthResponse{Status: "SERVING"}, nil
}

func (s *Server) Stats(ctx context.Context, req *searchv1.StatsRequest) (*searchv1.StatsResponse, error) {
	var totalDocs, totalTerms, totalPostings int64
	var totalDocLen float64

	for _, sc := range s.shards {
		statsCtx, cancel := context.WithTimeout(ctx, time.Duration(s.shardTimeoutMS)*time.Millisecond)
		resp, err := sc.stub.Stats(statsCtx, &searchv1.StatsRequest{})
		cancel()
		if err != nil {
			continue
		}
		totalDocs += resp.IndexedDocs
		totalTerms += resp.UniqueTerms
		totalPostings += resp.Postings
		totalDocLen += resp.AvgDocLength * float64(resp.IndexedDocs)
	}

	var avgDocLen float64
	if totalDocs > 0 {
		avgDocLen = totalDocLen / float64(totalDocs)
	}

	return &searchv1.StatsResponse{
		IndexedDocs:  totalDocs,
		UniqueTerms:  totalTerms,
		Postings:     totalPostings,
		AvgDocLength: avgDocLen,
	}, nil
}
