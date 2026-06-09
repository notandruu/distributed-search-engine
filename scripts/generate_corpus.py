#!/usr/bin/env python3
"""Generate a synthetic JSONL corpus for benchmarking."""

import argparse
import json
import random
import sys
from pathlib import Path


def build_vocab(size: int, rng: random.Random) -> list[str]:
    """Generate a vocabulary of pseudo-random English-ish words."""
    consonants = "bcdfghjklmnpqrstvwxyz"
    vowels = "aeiou"
    words = set()
    while len(words) < size:
        length = rng.randint(3, 10)
        word = []
        for i in range(length):
            if i % 2 == 0:
                word.append(rng.choice(consonants))
            else:
                word.append(rng.choice(vowels))
        words.add("".join(word))
    return sorted(words)


def generate_doc(doc_id: str, vocab: list[str], avg_terms: int, rng: random.Random) -> dict:
    """Generate one document with a title and body."""
    # Title: 4-8 words
    title_len = rng.randint(4, 8)
    title_words = rng.choices(vocab, k=title_len)
    title = " ".join(w.capitalize() for w in title_words)

    # Body: normally distributed around avg_terms
    body_len = max(10, int(rng.gauss(avg_terms, avg_terms * 0.3)))
    body_words = rng.choices(vocab, k=body_len)
    body = " ".join(body_words)

    return {"id": doc_id, "title": title, "body": body, "url": ""}


def main():
    parser = argparse.ArgumentParser(description="Generate synthetic JSONL corpus")
    parser.add_argument("--docs", type=int, default=100_000, help="Number of documents")
    parser.add_argument("--vocab-size", type=int, default=50_000, help="Vocabulary size")
    parser.add_argument("--avg-terms", type=int, default=120, help="Average body term count")
    parser.add_argument("--seed", type=int, default=42, help="Random seed for reproducibility")
    parser.add_argument("--out", type=str, default="data/corpus.jsonl", help="Output path")
    args = parser.parse_args()

    rng = random.Random(args.seed)

    out_path = Path(args.out)
    out_path.parent.mkdir(parents=True, exist_ok=True)

    print(f"Building vocabulary ({args.vocab_size} words)...", file=sys.stderr)
    vocab = build_vocab(args.vocab_size, rng)

    print(f"Generating {args.docs:,} documents → {out_path}", file=sys.stderr)
    written = 0
    with open(out_path, "w") as f:
        for i in range(args.docs):
            doc_id = f"doc-{i:07d}"
            doc = generate_doc(doc_id, vocab, args.avg_terms, rng)
            f.write(json.dumps(doc) + "\n")
            written += 1
            if written % 100_000 == 0:
                print(f"  {written:,} / {args.docs:,}", file=sys.stderr)

    print(f"Done. Wrote {written:,} documents to {out_path}", file=sys.stderr)


if __name__ == "__main__":
    main()
