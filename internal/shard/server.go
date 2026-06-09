package shard

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	searchv1 "github.com/notandruu/distributed-search-engine/gen/search/v1"
)

// Server implements the ShardService gRPC interface.
type Server struct {
	searchv1.UnimplementedShardServiceServer

	mu      sync.RWMutex
	shardID int32

	// Index state — will be replaced by internal/index in Phase 2.
	indexedDocs  int64
	uniqueTerms  int64
	postings     int64
	totalDocLen  int64
}

func NewServer(shardID int32) *Server {
	return &Server{shardID: shardID}
}

func (s *Server) Ingest(ctx context.Context, req *searchv1.IngestRequest) (*searchv1.IngestResponse, error) {
	if len(req.Documents) == 0 {
		return &searchv1.IngestResponse{}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Phase 3 will wire in the real index. For now, just count docs.
	s.indexedDocs += int64(len(req.Documents))

	return &searchv1.IngestResponse{
		Accepted: int64(len(req.Documents)),
		Rejected: 0,
	}, nil
}

func (s *Server) SearchShard(ctx context.Context, req *searchv1.ShardSearchRequest) (*searchv1.ShardSearchResponse, error) {
	if req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query must not be empty")
	}
	if req.TopK <= 0 {
		req.TopK = 10
	}

	start := time.Now()

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Phase 3 will run BM25. For now, return empty results.
	return &searchv1.ShardSearchResponse{
		Results: nil,
		TookMs:  time.Since(start).Milliseconds(),
	}, nil
}

func (s *Server) Health(ctx context.Context, req *searchv1.HealthRequest) (*searchv1.HealthResponse, error) {
	return &searchv1.HealthResponse{Status: "SERVING"}, nil
}

func (s *Server) Stats(ctx context.Context, req *searchv1.StatsRequest) (*searchv1.StatsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var avgDocLen float64
	if s.indexedDocs > 0 {
		avgDocLen = float64(s.totalDocLen) / float64(s.indexedDocs)
	}

	return &searchv1.StatsResponse{
		IndexedDocs:   s.indexedDocs,
		UniqueTerms:   s.uniqueTerms,
		Postings:      s.postings,
		AvgDocLength:  avgDocLen,
	}, nil
}

func (s *Server) String() string {
	return fmt.Sprintf("ShardServer(id=%d)", s.shardID)
}
