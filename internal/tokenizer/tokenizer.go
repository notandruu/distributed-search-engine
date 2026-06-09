package tokenizer

import "unicode"

// Tokenize lowercases text and splits on non-alphanumeric rune boundaries.
// Empty tokens are dropped. Returns nil for empty input.
func Tokenize(text string) []string {
	if text == "" {
		return nil
	}

	runes := []rune(text)
	tokens := make([]string, 0, len(runes)/5+1)
	start := -1

	flush := func(end int) {
		if start >= 0 && end > start {
			tokens = append(tokens, string(runes[start:end]))
		}
		start = -1
	}

	for i, r := range runes {
		if isAlphanumeric(r) {
			lr := unicode.ToLower(r)
			runes[i] = lr
			if start < 0 {
				start = i
			}
		} else {
			flush(i)
		}
	}
	flush(len(runes))

	return tokens
}

func isAlphanumeric(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
