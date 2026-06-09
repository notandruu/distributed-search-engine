package gateway

import (
	"context"
	"testing"

	searchv1 "github.com/notandruu/distributed-search-engine/gen/search/v1"
)

func TestHealth(t *testing.T) {
	srv, err := NewServer(Options{ShardAddrs: nil})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.Health(context.Background(), &searchv1.HealthRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "SERVING" {
		t.Errorf("got %q, want SERVING", resp.Status)
	}
}

func TestSearch_emptyQuery(t *testing.T) {
	srv, err := NewServer(Options{ShardAddrs: nil})
	if err != nil {
		t.Fatal(err)
	}
	_, err = srv.Search(context.Background(), &searchv1.SearchRequest{Query: ""})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestSearch_noShards(t *testing.T) {
	srv, err := NewServer(Options{ShardAddrs: nil})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.Search(context.Background(), &searchv1.SearchRequest{Query: "test", TopK: 5})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if resp.PartialFailure {
		t.Error("expected no partial failure with 0 shards")
	}
	if len(resp.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(resp.Results))
	}
}

func TestTopKMerge(t *testing.T) {
	results := []*searchv1.SearchResult{
		{DocId: "a", Score: 1.0},
		{DocId: "b", Score: 3.0},
		{DocId: "c", Score: 2.0},
		{DocId: "d", Score: 4.0},
	}
	top2 := topKMerge(results, 2)
	if len(top2) != 2 {
		t.Fatalf("expected 2 results, got %d", len(top2))
	}
	if top2[0].DocId != "d" || top2[1].DocId != "b" {
		t.Errorf("unexpected order: %v %v", top2[0].DocId, top2[1].DocId)
	}
}

func TestBackpressure(t *testing.T) {
	srv, err := NewServer(Options{
		ShardAddrs:    nil,
		MaxConcurrent: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Simulate 2 in-flight (counter = 2 > max 1).
	srv.activeConcurrent.Add(2)
	_, err = srv.Search(context.Background(), &searchv1.SearchRequest{Query: "test"})
	if err == nil {
		t.Fatal("expected ResourceExhausted error")
	}
}
