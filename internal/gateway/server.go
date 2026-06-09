package gateway

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	searchv1 "github.com/notandruu/distributed-search-engine/gen/search/v1"
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

	shards              []*shardClient
	maxConcurrent       int64
	activeConcurrent    atomic.Int64
	searchTimeoutMS     int
	shardTimeoutMS      int

	mu sync.RWMutex // protects shards slice after init
}

type Options struct {
	ShardAddrs      []string
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
		maxConcurrent:   int64(opts.MaxConcurrent),
		searchTimeoutMS: opts.SearchTimeoutMS,
		shardTimeoutMS:  opts.ShardTimeoutMS,
	}

	for i, addr := range opts.ShardAddrs {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, err
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

// Close releases gRPC connections.
func (s *Server) Close() {
	for _, sc := range s.shards {
		sc.conn.Close()
	}
}

func (s *Server) Search(ctx context.Context, req *searchv1.SearchRequest) (*searchv1.SearchResponse, error) {
	if req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query must not be empty")
	}
	if req.TopK <= 0 {
		req.TopK = 10
	}

	// Backpressure: reject if at capacity.
	cur := s.activeConcurrent.Add(1)
	defer s.activeConcurrent.Add(-1)
	if cur > s.maxConcurrent {
		return nil, status.Error(codes.ResourceExhausted, "gateway at capacity")
	}

	// Apply global search timeout.
	searchCtx, cancel := context.WithTimeout(ctx, time.Duration(s.searchTimeoutMS)*time.Millisecond)
	defer cancel()

	// Phase 4 will add Redis cache-aside here.

	results, failedShards := s.fanout(searchCtx, req)

	return &searchv1.SearchResponse{
		Results:        results,
		PartialFailure: len(failedShards) > 0,
		FailedShards:   failedShards,
		Stats: &searchv1.SearchStats{
			TookMs:          0,
			CacheHit:        false,
			ShardsQueried:   int32(len(s.shards)),
			ShardsSucceeded: int32(len(s.shards) - len(failedShards)),
		},
	}, nil
}

type shardResult struct {
	results []*searchv1.SearchResult
	shardID int
	err     error
}

// fanout sends SearchShard to all shards concurrently and merges top-K.
func (s *Server) fanout(ctx context.Context, req *searchv1.SearchRequest) ([]*searchv1.SearchResult, []string) {
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

	var allResults []*searchv1.SearchResult
	var failedShards []string

	for range s.shards {
		r := <-resultCh
		if r.err != nil {
			failedShards = append(failedShards, s.shards[r.shardID].addr)
			continue
		}
		allResults = append(allResults, r.results...)
	}

	// Phase 4 will replace this with a proper min-heap merge.
	return topKMerge(allResults, int(req.TopK)), failedShards
}

// topKMerge returns the top-K results by score descending.
// Uses a simple sort for the Phase 1 skeleton; Phase 4 will use a heap.
func topKMerge(results []*searchv1.SearchResult, k int) []*searchv1.SearchResult {
	// Insertion sort — fine for small k, replaced in Phase 4.
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].Score > results[j-1].Score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
	if len(results) > k {
		return results[:k]
	}
	return results
}

func (s *Server) Health(ctx context.Context, req *searchv1.HealthRequest) (*searchv1.HealthResponse, error) {
	return &searchv1.HealthResponse{Status: "SERVING"}, nil
}

func (s *Server) Stats(ctx context.Context, req *searchv1.StatsRequest) (*searchv1.StatsResponse, error) {
	// Aggregate stats from all shards.
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
