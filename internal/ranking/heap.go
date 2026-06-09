package ranking

import "container/heap"

// ScoredDoc is a document with its BM25 score.
type ScoredDoc struct {
	DocID uint64
	Score float64
}

// minHeap is a min-heap of ScoredDocs (lowest score at root).
// Used to efficiently maintain a top-K set.
type minHeap []ScoredDoc

func (h minHeap) Len() int           { return len(h) }
func (h minHeap) Less(i, j int) bool { return h[i].Score < h[j].Score }
func (h minHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *minHeap) Push(x any) {
	*h = append(*h, x.(ScoredDoc))
}

func (h *minHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// topK returns the top-k ScoredDocs by score descending.
func topK(scores map[uint64]float64, k int) []ScoredDoc {
	if k <= 0 || len(scores) == 0 {
		return nil
	}

	h := &minHeap{}
	heap.Init(h)

	for docID, score := range scores {
		if score <= 0 {
			continue
		}
		if h.Len() < k {
			heap.Push(h, ScoredDoc{DocID: docID, Score: score})
		} else if score > (*h)[0].Score {
			heap.Pop(h)
			heap.Push(h, ScoredDoc{DocID: docID, Score: score})
		}
	}

	// Drain heap into slice, then reverse for descending order.
	result := make([]ScoredDoc, h.Len())
	for i := len(result) - 1; i >= 0; i-- {
		result[i] = heap.Pop(h).(ScoredDoc)
	}
	return result
}

// MergeTopK merges multiple top-K result slices into a global top-K.
// Each input slice must already be sorted descending by score.
func MergeTopK(shardResults [][]ScoredDoc, k int) []ScoredDoc {
	// Flatten and re-run topK on the combined set.
	combined := make(map[uint64]float64)
	for _, results := range shardResults {
		for _, r := range results {
			// Use max score in case of duplicate doc IDs across shards (shouldn't happen
			// with document sharding, but defensive).
			if existing, ok := combined[r.DocID]; !ok || r.Score > existing {
				combined[r.DocID] = r.Score
			}
		}
	}
	return topK(combined, k)
}
