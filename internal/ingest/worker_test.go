package ingest

import (
	"testing"
)

func TestRouteDoc_deterministic(t *testing.T) {
	// Same doc ID must always route to the same shard.
	for i := 0; i < 100; i++ {
		got := routeDoc("doc-12345", 4)
		if got != routeDoc("doc-12345", 4) {
			t.Fatalf("routeDoc not deterministic on iteration %d", i)
		}
	}
}

func TestRouteDoc_boundedByShards(t *testing.T) {
	numShards := 4
	for _, docID := range []string{"a", "doc-1", "zzzz", "hello-world", "123"} {
		shard := routeDoc(docID, numShards)
		if shard < 0 || shard >= numShards {
			t.Errorf("routeDoc(%q, %d) = %d, out of range", docID, numShards, shard)
		}
	}
}

func TestRouteDoc_distribution(t *testing.T) {
	// With 4 shards and enough docs, all shards should receive some docs.
	numShards := 4
	counts := make([]int, numShards)
	for i := 0; i < 1000; i++ {
		id := "doc-" + string(rune('a'+i%26)) + "-" + string(rune('0'+i%10))
		counts[routeDoc(id, numShards)]++
	}
	for i, c := range counts {
		if c == 0 {
			t.Errorf("shard %d received 0 documents out of 1000 — poor distribution", i)
		}
	}
}
