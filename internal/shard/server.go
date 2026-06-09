package shard

import (
	"context"
	"fmt"
	"sync"
	"time"
	"unicode/utf8"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	searchv1 "github.com/notandruu/distributed-search-engine/gen/search/v1"
	"github.com/notandruu/distributed-search-engine/internal/index"
	"github.com/notandruu/distributed-search-engine/internal/ranking"
	"github.com/notandruu/distributed-search-engine/internal/tokenizer"
)

const bodyPreviewMaxBytes = 200

// Server implements the ShardService gRPC interface.
type Server struct {
	searchv1.UnimplementedShardServiceServer

	mu      sync.RWMutex
	idx     *index.InvertedIndex
	scorer  ranking.BM25
	shardID int32
}

func NewServer(shardID int32) *Server {
	return &Server{
		idx:     index.New(),
		scorer:  ranking.Default(),
		shardID: shardID,
	}
}

func (s *Server) Ingest(ctx context.Context, req *searchv1.IngestRequest) (*searchv1.IngestResponse, error) {
	if len(req.Documents) == 0 {
		return &searchv1.IngestResponse{}, nil
	}

	var accepted, rejected int64

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, doc := range req.Documents {
		if doc.Id == "" {
			rejected++
			continue
		}
		tokens := tokenizeDoc(doc.Title, doc.Body)
		preview := bodyPreview(doc.Body)
		s.idx.Add(doc.Id, doc.Title, doc.Url, preview, tokens)
		accepted++
	}

	return &searchv1.IngestResponse{
		Accepted: accepted,
		Rejected: rejected,
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

	queryTerms := tokenizer.Tokenize(req.Query)
	if len(queryTerms) == 0 {
		return &searchv1.ShardSearchResponse{TookMs: time.Since(start).Milliseconds()}, nil
	}

	s.mu.RLock()
	scored := s.scorer.ScoreQuery(s.idx, queryTerms, int(req.TopK))
	results := make([]*searchv1.SearchResult, 0, len(scored))
	for _, sd := range scored {
		meta, ok := s.idx.DocMeta[sd.DocID]
		if !ok {
			continue
		}
		results = append(results, &searchv1.SearchResult{
			DocId:   meta.ExternalID,
			Title:   meta.Title,
			Snippet: meta.BodyPreview,
			Score:   sd.Score,
			ShardId: s.shardID,
		})
	}
	s.mu.RUnlock()

	return &searchv1.ShardSearchResponse{
		Results: results,
		TookMs:  time.Since(start).Milliseconds(),
	}, nil
}

func (s *Server) Health(ctx context.Context, req *searchv1.HealthRequest) (*searchv1.HealthResponse, error) {
	return &searchv1.HealthResponse{Status: "SERVING"}, nil
}

func (s *Server) Stats(ctx context.Context, req *searchv1.StatsRequest) (*searchv1.StatsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &searchv1.StatsResponse{
		IndexedDocs:  int64(s.idx.TotalDocs),
		UniqueTerms:  int64(s.idx.UniqueTerms()),
		Postings:     s.idx.TotalPostings(),
		AvgDocLength: s.idx.AvgDocLength(),
	}, nil
}

func (s *Server) String() string {
	return fmt.Sprintf("ShardServer(id=%d)", s.shardID)
}

// tokenizeDoc combines title and body into one token stream.
func tokenizeDoc(title, body string) []string {
	combined := title + " " + body
	return tokenizer.Tokenize(combined)
}

// bodyPreview returns a UTF-8 safe prefix of the body for display.
func bodyPreview(body string) string {
	if len(body) <= bodyPreviewMaxBytes {
		return body
	}
	for i := bodyPreviewMaxBytes; i > 0; i-- {
		if utf8.RuneStart(body[i]) {
			return body[:i] + "…"
		}
	}
	return body[:bodyPreviewMaxBytes]
}
