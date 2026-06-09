package shard

import (
	"context"
	"fmt"
	"sync"
	"testing"

	searchv1 "github.com/notandruu/distributed-search-engine/gen/search/v1"
)

func TestHealth(t *testing.T) {
	srv := NewServer(0)
	resp, err := srv.Health(context.Background(), &searchv1.HealthRequest{})
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if resp.Status != "SERVING" {
		t.Fatalf("expected SERVING, got %q", resp.Status)
	}
}

func TestStats_empty(t *testing.T) {
	srv := NewServer(0)
	resp, err := srv.Stats(context.Background(), &searchv1.StatsRequest{})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if resp.IndexedDocs != 0 {
		t.Errorf("expected 0 indexed docs, got %d", resp.IndexedDocs)
	}
}

func TestIngest_basic(t *testing.T) {
	srv := NewServer(0)
	resp, err := srv.Ingest(context.Background(), &searchv1.IngestRequest{
		Documents: []*searchv1.Document{
			{Id: "doc-1", Title: "Distributed Systems", Body: "consensus replication raft paxos"},
			{Id: "doc-2", Title: "Inverted Index", Body: "posting list full text search"},
		},
	})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if resp.Accepted != 2 {
		t.Errorf("expected 2 accepted, got %d", resp.Accepted)
	}

	stats, _ := srv.Stats(context.Background(), &searchv1.StatsRequest{})
	if stats.IndexedDocs != 2 {
		t.Errorf("expected 2 indexed docs, got %d", stats.IndexedDocs)
	}
	if stats.UniqueTerms == 0 {
		t.Error("expected >0 unique terms")
	}
}

func TestIngest_rejectsEmptyID(t *testing.T) {
	srv := NewServer(0)
	resp, err := srv.Ingest(context.Background(), &searchv1.IngestRequest{
		Documents: []*searchv1.Document{
			{Id: "", Title: "bad doc", Body: "no id"},
		},
	})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if resp.Rejected != 1 {
		t.Errorf("expected 1 rejected, got %d", resp.Rejected)
	}
}

func TestSearchShard_emptyQuery(t *testing.T) {
	srv := NewServer(0)
	_, err := srv.SearchShard(context.Background(), &searchv1.ShardSearchRequest{Query: ""})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestSearchShard_noResults(t *testing.T) {
	srv := NewServer(0)
	resp, err := srv.SearchShard(context.Background(), &searchv1.ShardSearchRequest{
		Query: "nonexistenttoken12345",
		TopK:  10,
	})
	if err != nil {
		t.Fatalf("SearchShard: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(resp.Results))
	}
}

func TestSearchShard_returnsResults(t *testing.T) {
	srv := NewServer(0)
	_, _ = srv.Ingest(context.Background(), &searchv1.IngestRequest{
		Documents: []*searchv1.Document{
			{Id: "doc-1", Title: "Raft Consensus", Body: "distributed consensus replication leader election"},
			{Id: "doc-2", Title: "Paxos", Body: "consensus protocol distributed agreement quorum"},
			{Id: "doc-3", Title: "Storage", Body: "disk storage filesystem blocks"},
		},
	})

	resp, err := srv.SearchShard(context.Background(), &searchv1.ShardSearchRequest{
		Query: "distributed consensus",
		TopK:  5,
	})
	if err != nil {
		t.Fatalf("SearchShard: %v", err)
	}
	if len(resp.Results) == 0 {
		t.Fatal("expected results for 'distributed consensus'")
	}
	// Results should be sorted descending by score.
	for i := 1; i < len(resp.Results); i++ {
		if resp.Results[i].Score > resp.Results[i-1].Score {
			t.Errorf("results not sorted: results[%d].Score=%.4f > results[%d].Score=%.4f",
				i, resp.Results[i].Score, i-1, resp.Results[i-1].Score)
		}
	}
	// doc-3 should NOT appear (no matching terms).
	for _, r := range resp.Results {
		if r.DocId == "doc-3" {
			t.Error("doc-3 should not match 'distributed consensus'")
		}
	}
}

func TestSearchShard_topKRespected(t *testing.T) {
	srv := NewServer(0)
	docs := make([]*searchv1.Document, 20)
	for i := range docs {
		docs[i] = &searchv1.Document{
			Id:    fmt.Sprintf("doc-%d", i),
			Title: "common topic",
			Body:  "common token shared across all docs",
		}
	}
	_, _ = srv.Ingest(context.Background(), &searchv1.IngestRequest{Documents: docs})

	resp, _ := srv.SearchShard(context.Background(), &searchv1.ShardSearchRequest{
		Query: "common",
		TopK:  5,
	})
	if len(resp.Results) > 5 {
		t.Errorf("expected <=5 results, got %d", len(resp.Results))
	}
}

func TestConcurrentIngestAndSearch(t *testing.T) {
	srv := NewServer(0)

	const goroutines = 20
	var wg sync.WaitGroup

	// 10 concurrent ingesters
	for i := 0; i < goroutines/2; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, _ = srv.Ingest(context.Background(), &searchv1.IngestRequest{
				Documents: []*searchv1.Document{
					{Id: fmt.Sprintf("doc-%d", n), Title: "test", Body: "concurrent ingestion"},
				},
			})
		}(i)
	}

	// 10 concurrent searchers
	for i := 0; i < goroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = srv.SearchShard(context.Background(), &searchv1.ShardSearchRequest{
				Query: "concurrent",
				TopK:  5,
			})
		}()
	}

	wg.Wait()
}
