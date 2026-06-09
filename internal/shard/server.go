package shard

import (
	"context"
	"fmt"
	"sync"
	"time"
	"unicode/utf8"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	searchv1 "github.com/notandruu/distributed-search-engine/gen/search/v1"
	"github.com/notandruu/distributed-search-engine/internal/index"
	"github.com/notandruu/distributed-search-engine/internal/observability"
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
	metrics *observability.ShardMetrics
	tracer  trace.Tracer
	shardID int32
}

func NewServer(shardID int32, metrics *observability.ShardMetrics) *Server {
	return &Server{
		idx:     index.New(),
		scorer:  ranking.Default(),
		metrics: metrics,
		tracer:  observability.Tracer(),
		shardID: shardID,
	}
}

func (s *Server) Ingest(ctx context.Context, req *searchv1.IngestRequest) (*searchv1.IngestResponse, error) {
	if len(req.Documents) == 0 {
		return &searchv1.IngestResponse{}, nil
	}

	ctx, span := s.tracer.Start(ctx, "shard.Ingest",
		trace.WithAttributes(
			attribute.Int("shard.id", int(s.shardID)),
			attribute.Int("docs.count", len(req.Documents)),
		),
	)
	defer span.End()

	var accepted, rejected int64

	s.mu.Lock()
	for _, doc := range req.Documents {
		if doc.Id == "" {
			rejected++
			continue
		}
		_, tokenSpan := s.tracer.Start(ctx, "shard.Tokenize")
		tokens := tokenizeDoc(doc.Title, doc.Body)
		tokenSpan.End()

		preview := bodyPreview(doc.Body)
		s.idx.Add(doc.Id, doc.Title, doc.Url, preview, tokens)
		accepted++
	}
	uniqueTerms := int64(s.idx.UniqueTerms())
	totalPostings := s.idx.TotalPostings()
	s.mu.Unlock()

	span.SetAttributes(attribute.Int64("docs.accepted", accepted))

	if s.metrics != nil {
		s.metrics.IngestBatches.Inc()
		s.metrics.IngestDocuments.Add(float64(accepted))
		s.metrics.IngestErrors.Add(float64(rejected))
		s.metrics.DocsIndexed.Add(float64(accepted))
		s.metrics.UniqueTermsGauge.Set(float64(uniqueTerms))
		s.metrics.PostingsGauge.Set(float64(totalPostings))
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
	ctx, span := s.tracer.Start(ctx, "shard.SearchShard",
		trace.WithAttributes(
			attribute.Int("shard.id", int(s.shardID)),
			attribute.String("query", req.Query),
			attribute.Int("top_k", int(req.TopK)),
		),
	)
	defer span.End()

	_, tokenSpan := s.tracer.Start(ctx, "shard.Tokenize")
	queryTerms := tokenizer.Tokenize(req.Query)
	tokenSpan.End()

	if len(queryTerms) == 0 {
		return &searchv1.ShardSearchResponse{TookMs: time.Since(start).Milliseconds()}, nil
	}

	s.mu.RLock()
	_, scoreSpan := s.tracer.Start(ctx, "shard.BM25Score",
		trace.WithAttributes(attribute.Int("terms.count", len(queryTerms))),
	)
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
	scoreSpan.SetAttributes(attribute.Int("results.count", len(results)))
	scoreSpan.End()
	s.mu.RUnlock()

	took := time.Since(start)
	span.SetAttributes(attribute.Int("results.count", len(results)))

	if s.metrics != nil {
		s.metrics.SearchDuration.WithLabelValues().Observe(took.Seconds())
	}

	return &searchv1.ShardSearchResponse{
		Results: results,
		TookMs:  took.Milliseconds(),
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

func tokenizeDoc(title, body string) []string {
	return tokenizer.Tokenize(title + " " + body)
}

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
