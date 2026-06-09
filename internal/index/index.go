package index

// Posting represents one document's term frequency entry in a posting list.
type Posting struct {
	DocID    uint64
	TermFreq uint32
}

// DocumentMeta stores per-document metadata for result rendering.
type DocumentMeta struct {
	ExternalID  string
	Title       string
	URL         string
	Length      int
	BodyPreview string
}

// InvertedIndex is the core shard data structure.
// Not safe for concurrent writes — callers must use an RWMutex.
type InvertedIndex struct {
	// Postings maps term → sorted posting list (sorted by DocID).
	Postings map[string][]Posting

	// DocMeta maps internal numeric DocID → document metadata.
	DocMeta map[uint64]DocumentMeta

	// DocLengths maps DocID → total token count.
	DocLengths map[uint64]int

	// DocFreq maps term → number of documents containing the term.
	DocFreq map[string]int

	// TotalDocs is the number of indexed documents.
	TotalDocs int

	// TotalDocLength is the sum of all document lengths (for avgDocLen).
	TotalDocLength int64

	nextID uint64
}

// New returns an initialized empty InvertedIndex.
func New() *InvertedIndex {
	return &InvertedIndex{
		Postings:   make(map[string][]Posting),
		DocMeta:    make(map[uint64]DocumentMeta),
		DocLengths: make(map[uint64]int),
		DocFreq:    make(map[string]int),
	}
}

// Add indexes a document. tokens is the pre-tokenized body+title.
func (idx *InvertedIndex) Add(externalID, title, url, bodyPreview string, tokens []string) {
	docID := idx.nextID
	idx.nextID++

	// Build term frequency map for this document.
	tf := make(map[string]uint32, len(tokens))
	for _, t := range tokens {
		tf[t]++
	}

	// Store metadata.
	idx.DocMeta[docID] = DocumentMeta{
		ExternalID:  externalID,
		Title:       title,
		URL:         url,
		Length:      len(tokens),
		BodyPreview: bodyPreview,
	}
	idx.DocLengths[docID] = len(tokens)
	idx.TotalDocs++
	idx.TotalDocLength += int64(len(tokens))

	// Update posting lists.
	for term, freq := range tf {
		if _, seen := idx.DocFreq[term]; !seen {
			idx.DocFreq[term]++
		} else {
			idx.DocFreq[term]++
		}
		idx.Postings[term] = append(idx.Postings[term], Posting{DocID: docID, TermFreq: freq})
	}
}

// AvgDocLength returns the average document length across all indexed docs.
func (idx *InvertedIndex) AvgDocLength() float64 {
	if idx.TotalDocs == 0 {
		return 0
	}
	return float64(idx.TotalDocLength) / float64(idx.TotalDocs)
}

// UniqueTerms returns the number of unique indexed terms.
func (idx *InvertedIndex) UniqueTerms() int {
	return len(idx.Postings)
}

// TotalPostings returns the total number of posting entries across all terms.
func (idx *InvertedIndex) TotalPostings() int64 {
	var n int64
	for _, pl := range idx.Postings {
		n += int64(len(pl))
	}
	return n
}
