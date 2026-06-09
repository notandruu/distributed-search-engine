package ranking

import (
	"math"

	"github.com/notandruu/distributed-search-engine/internal/index"
)

const (
	// DefaultK1 controls term frequency saturation.
	DefaultK1 = 1.2

	// DefaultB controls document length normalization.
	DefaultB = 0.75
)

// BM25 holds the scoring parameters.
type BM25 struct {
	K1 float64
	B  float64
}

// Default returns a BM25 scorer with standard parameters.
func Default() BM25 {
	return BM25{K1: DefaultK1, B: DefaultB}
}

// ScoreQuery computes BM25 scores for all candidate documents matching queryTerms
// and returns the top-K results sorted descending by score.
func (b BM25) ScoreQuery(idx *index.InvertedIndex, queryTerms []string, k int) []ScoredDoc {
	if len(queryTerms) == 0 || idx.TotalDocs == 0 {
		return nil
	}

	avgDocLen := idx.AvgDocLength()
	N := float64(idx.TotalDocs)

	// Accumulate scores per candidate document.
	scores := make(map[uint64]float64)

	for _, term := range unique(queryTerms) {
		postings, ok := idx.Postings[term]
		if !ok {
			continue
		}

		df := float64(idx.DocFreq[term])
		idf := math.Log((N-df+0.5)/(df+0.5) + 1.0)

		for _, p := range postings {
			docLen := float64(idx.DocLengths[p.DocID])
			tf := float64(p.TermFreq)
			norm := tf * (b.K1 + 1) / (tf + b.K1*(1-b.B+b.B*docLen/avgDocLen))
			scores[p.DocID] += idf * norm
		}
	}

	return topK(scores, k)
}

// unique deduplicates a slice of strings preserving order.
func unique(terms []string) []string {
	seen := make(map[string]struct{}, len(terms))
	out := make([]string, 0, len(terms))
	for _, t := range terms {
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			out = append(out, t)
		}
	}
	return out
}
