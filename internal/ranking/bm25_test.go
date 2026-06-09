package ranking

import (
	"math"
	"testing"

	"github.com/notandruu/distributed-search-engine/internal/index"
)

func buildIndex(docs []struct{ id string; tokens []string }) *index.InvertedIndex {
	idx := index.New()
	for _, d := range docs {
		idx.Add(d.id, d.id, "", "", d.tokens)
	}
	return idx
}

func TestBM25_singleTermSingleDoc(t *testing.T) {
	idx := buildIndex([]struct{ id string; tokens []string }{
		{"doc-1", []string{"hello", "world"}},
	})
	scorer := Default()
	results := scorer.ScoreQuery(idx, []string{"hello"}, 10)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Score <= 0 {
		t.Errorf("expected positive score, got %.4f", results[0].Score)
	}
}

func TestBM25_rankOrder(t *testing.T) {
	// doc-1 contains "foo" once, doc-2 contains "foo" three times.
	// doc-2 should rank higher.
	idx := buildIndex([]struct{ id string; tokens []string }{
		{"doc-1", []string{"foo", "bar", "baz", "qux", "quux"}},
		{"doc-2", []string{"foo", "foo", "foo", "bar", "baz"}},
	})
	scorer := Default()
	results := scorer.ScoreQuery(idx, []string{"foo"}, 10)

	if len(results) < 2 {
		t.Fatalf("expected >=2 results, got %d", len(results))
	}
	// Results are sorted descending — doc-2 should be first.
	if results[0].Score <= results[1].Score {
		t.Errorf("expected results[0].score > results[1].score: %.4f vs %.4f",
			results[0].Score, results[1].Score)
	}
}

func TestBM25_missingTerm(t *testing.T) {
	idx := buildIndex([]struct{ id string; tokens []string }{
		{"doc-1", []string{"hello"}},
	})
	scorer := Default()
	results := scorer.ScoreQuery(idx, []string{"zzz_nonexistent"}, 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results for missing term, got %d", len(results))
	}
}

func TestBM25_emptyIndex(t *testing.T) {
	idx := index.New()
	scorer := Default()
	results := scorer.ScoreQuery(idx, []string{"hello"}, 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty index, got %d", len(results))
	}
}

func TestBM25_topKLimit(t *testing.T) {
	docs := make([]struct{ id string; tokens []string }, 20)
	for i := range docs {
		docs[i] = struct{ id string; tokens []string }{
			id:     "doc",
			tokens: []string{"common"},
		}
	}
	idx := buildIndex(docs)
	scorer := Default()
	results := scorer.ScoreQuery(idx, []string{"common"}, 5)

	if len(results) != 5 {
		t.Errorf("expected 5 results (top-K=5), got %d", len(results))
	}
}

func TestBM25_deterministic(t *testing.T) {
	idx := buildIndex([]struct{ id string; tokens []string }{
		{"doc-1", []string{"distributed", "systems", "consensus"}},
		{"doc-2", []string{"distributed", "computing", "raft"}},
		{"doc-3", []string{"consensus", "paxos", "leader", "election"}},
	})
	scorer := Default()

	r1 := scorer.ScoreQuery(idx, []string{"distributed", "consensus"}, 10)
	r2 := scorer.ScoreQuery(idx, []string{"distributed", "consensus"}, 10)

	if len(r1) != len(r2) {
		t.Fatalf("non-deterministic result count: %d vs %d", len(r1), len(r2))
	}
	for i := range r1 {
		if math.Abs(r1[i].Score-r2[i].Score) > 1e-9 {
			t.Errorf("result[%d] score differs: %.10f vs %.10f", i, r1[i].Score, r2[i].Score)
		}
	}
}
