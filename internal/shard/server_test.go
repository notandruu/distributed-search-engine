package shard

import (
	"context"
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
			{Id: "doc-1", Title: "Hello", Body: "world"},
			{Id: "doc-2", Title: "Foo",   Body: "bar"},
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
}

func TestSearchShard_empty_query(t *testing.T) {
	srv := NewServer(0)
	_, err := srv.SearchShard(context.Background(), &searchv1.ShardSearchRequest{Query: ""})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestSearchShard_no_docs(t *testing.T) {
	srv := NewServer(0)
	resp, err := srv.SearchShard(context.Background(), &searchv1.ShardSearchRequest{
		Query: "test",
		TopK:  10,
	})
	if err != nil {
		t.Fatalf("SearchShard: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Errorf("expected empty results, got %d", len(resp.Results))
	}
}
