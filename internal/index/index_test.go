package index

import (
	"testing"
)

func TestAdd_singleDoc(t *testing.T) {
	idx := New()
	idx.Add("doc-1", "Hello World", "", "", []string{"hello", "world", "hello"})

	if idx.TotalDocs != 1 {
		t.Errorf("TotalDocs: got %d, want 1", idx.TotalDocs)
	}
	if idx.TotalDocLength != 3 {
		t.Errorf("TotalDocLength: got %d, want 3", idx.TotalDocLength)
	}

	pl := idx.Postings["hello"]
	if len(pl) != 1 {
		t.Fatalf("expected 1 posting for 'hello', got %d", len(pl))
	}
	if pl[0].TermFreq != 2 {
		t.Errorf("TermFreq for 'hello': got %d, want 2", pl[0].TermFreq)
	}
}

func TestAdd_multiDoc(t *testing.T) {
	idx := New()
	idx.Add("doc-1", "a", "", "", []string{"foo", "bar"})
	idx.Add("doc-2", "b", "", "", []string{"foo", "baz"})

	if idx.TotalDocs != 2 {
		t.Errorf("TotalDocs: got %d, want 2", idx.TotalDocs)
	}
	if idx.DocFreq["foo"] != 2 {
		t.Errorf("DocFreq[foo]: got %d, want 2", idx.DocFreq["foo"])
	}
	if idx.DocFreq["bar"] != 1 {
		t.Errorf("DocFreq[bar]: got %d, want 1", idx.DocFreq["bar"])
	}
}

func TestAvgDocLength(t *testing.T) {
	idx := New()
	if idx.AvgDocLength() != 0 {
		t.Error("expected 0 avg doc length for empty index")
	}

	idx.Add("doc-1", "", "", "", []string{"a", "b", "c"})        // len=3
	idx.Add("doc-2", "", "", "", []string{"a", "b", "c", "d", "e"}) // len=5

	want := 4.0
	got := idx.AvgDocLength()
	if got != want {
		t.Errorf("AvgDocLength: got %.2f, want %.2f", got, want)
	}
}

func TestUniqueTerms(t *testing.T) {
	idx := New()
	idx.Add("doc-1", "", "", "", []string{"a", "b", "a"})
	idx.Add("doc-2", "", "", "", []string{"b", "c"})

	if idx.UniqueTerms() != 3 {
		t.Errorf("UniqueTerms: got %d, want 3", idx.UniqueTerms())
	}
}

func TestTotalPostings(t *testing.T) {
	idx := New()
	idx.Add("doc-1", "", "", "", []string{"a", "b"})
	idx.Add("doc-2", "", "", "", []string{"a", "c"})

	// "a" → 2 postings, "b" → 1 posting, "c" → 1 posting = 4 total
	if idx.TotalPostings() != 4 {
		t.Errorf("TotalPostings: got %d, want 4", idx.TotalPostings())
	}
}

func TestDocMeta_stored(t *testing.T) {
	idx := New()
	idx.Add("external-1", "My Title", "http://example.com", "preview text", []string{"hello"})

	meta, ok := idx.DocMeta[0]
	if !ok {
		t.Fatal("expected doc meta for docID 0")
	}
	if meta.ExternalID != "external-1" {
		t.Errorf("ExternalID: got %q, want external-1", meta.ExternalID)
	}
	if meta.Title != "My Title" {
		t.Errorf("Title: got %q, want My Title", meta.Title)
	}
}
