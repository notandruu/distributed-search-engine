package ranking

import (
	"testing"
)

func TestTopK_ordering(t *testing.T) {
	scores := map[uint64]float64{
		1: 1.0,
		2: 4.0,
		3: 2.0,
		4: 3.0,
	}
	results := topK(scores, 3)

	if len(results) != 3 {
		t.Fatalf("expected 3, got %d", len(results))
	}
	// Should be sorted descending: 4.0, 3.0, 2.0
	if results[0].Score != 4.0 {
		t.Errorf("results[0].Score = %.2f, want 4.0", results[0].Score)
	}
	if results[1].Score != 3.0 {
		t.Errorf("results[1].Score = %.2f, want 3.0", results[1].Score)
	}
	if results[2].Score != 2.0 {
		t.Errorf("results[2].Score = %.2f, want 2.0", results[2].Score)
	}
}

func TestTopK_fewerThanK(t *testing.T) {
	scores := map[uint64]float64{1: 1.0, 2: 2.0}
	results := topK(scores, 10)
	if len(results) != 2 {
		t.Errorf("expected 2, got %d", len(results))
	}
}

func TestTopK_empty(t *testing.T) {
	results := topK(map[uint64]float64{}, 10)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestTopK_zeroK(t *testing.T) {
	scores := map[uint64]float64{1: 1.0}
	results := topK(scores, 0)
	if len(results) != 0 {
		t.Errorf("expected 0 for k=0, got %d", len(results))
	}
}

func TestTopK_excludesZeroScores(t *testing.T) {
	scores := map[uint64]float64{1: 0.0, 2: 2.0}
	results := topK(scores, 10)
	if len(results) != 1 {
		t.Errorf("expected 1 (zero-score doc excluded), got %d", len(results))
	}
	if results[0].Score != 2.0 {
		t.Errorf("expected score 2.0, got %.2f", results[0].Score)
	}
}

func TestMergeTopK(t *testing.T) {
	shard0 := []ScoredDoc{{DocID: 1, Score: 5.0}, {DocID: 2, Score: 3.0}}
	shard1 := []ScoredDoc{{DocID: 3, Score: 4.0}, {DocID: 4, Score: 1.0}}

	merged := MergeTopK([][]ScoredDoc{shard0, shard1}, 3)
	if len(merged) != 3 {
		t.Fatalf("expected 3, got %d", len(merged))
	}
	if merged[0].DocID != 1 {
		t.Errorf("merged[0] should be doc 1 (score 5.0), got doc %d", merged[0].DocID)
	}
}
