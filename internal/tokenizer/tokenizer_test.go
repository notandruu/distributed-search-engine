package tokenizer

import (
	"reflect"
	"testing"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "single word",
			input: "hello",
			want:  []string{"hello"},
		},
		{
			name:  "uppercase lowercased",
			input: "Hello World",
			want:  []string{"hello", "world"},
		},
		{
			name:  "punctuation split",
			input: "Google's distributed systems!",
			want:  []string{"google", "s", "distributed", "systems"},
		},
		{
			name:  "numbers included",
			input: "Go 1.22 release",
			want:  []string{"go", "1", "22", "release"},
		},
		{
			name:  "multiple spaces",
			input: "foo   bar",
			want:  []string{"foo", "bar"},
		},
		{
			name:  "leading trailing punctuation",
			input: "...hello...",
			want:  []string{"hello"},
		},
		{
			name:  "hyphenated",
			input: "well-known",
			want:  []string{"well", "known"},
		},
		{
			name:  "only punctuation",
			input: "!@#$%",
			want:  []string{},
		},
		{
			name:  "unicode letters",
			input: "café résumé",
			want:  []string{"café", "résumé"},
		},
		{
			name:  "mixed case complex",
			input: "BM25 ranking algorithm",
			want:  []string{"bm25", "ranking", "algorithm"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Tokenize(tc.input)
			// Treat nil and empty slice as equivalent.
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Tokenize(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
