"""Tests for corpus generation determinism and output format."""

import json
import sys
import os
import random
import tempfile
from pathlib import Path

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from scripts.generate_corpus import build_vocab, generate_doc


class TestVocab:
    def test_deterministic(self):
        rng1 = random.Random(42)
        rng2 = random.Random(42)
        v1 = build_vocab(100, rng1)
        v2 = build_vocab(100, rng2)
        assert v1 == v2

    def test_size(self):
        vocab = build_vocab(500, random.Random(0))
        assert len(vocab) == 500

    def test_unique(self):
        vocab = build_vocab(200, random.Random(1))
        assert len(set(vocab)) == len(vocab)

    def test_all_lowercase(self):
        vocab = build_vocab(100, random.Random(99))
        for word in vocab:
            assert word == word.lower(), f"word {word!r} is not lowercase"


class TestGenerateDoc:
    def test_has_required_fields(self):
        vocab = build_vocab(100, random.Random(42))
        rng = random.Random(42)
        doc = generate_doc("doc-001", vocab, avg_terms=50, rng=rng)
        assert "id" in doc
        assert "title" in doc
        assert "body" in doc
        assert "url" in doc

    def test_id_preserved(self):
        vocab = build_vocab(50, random.Random(1))
        rng = random.Random(1)
        doc = generate_doc("my-custom-id", vocab, avg_terms=20, rng=rng)
        assert doc["id"] == "my-custom-id"

    def test_deterministic(self):
        vocab = build_vocab(100, random.Random(42))
        doc1 = generate_doc("doc-1", vocab, avg_terms=50, rng=random.Random(7))
        doc2 = generate_doc("doc-1", vocab, avg_terms=50, rng=random.Random(7))
        assert doc1 == doc2

    def test_body_has_words(self):
        vocab = build_vocab(100, random.Random(3))
        rng = random.Random(3)
        doc = generate_doc("doc-x", vocab, avg_terms=50, rng=rng)
        words = doc["body"].split()
        assert len(words) >= 10, f"expected >= 10 body words, got {len(words)}"


class TestCorpusFile:
    def test_write_and_read(self):
        """Generate a small corpus file and verify all docs are valid JSONL."""
        import subprocess
        with tempfile.TemporaryDirectory() as tmpdir:
            out = Path(tmpdir) / "corpus.jsonl"
            result = subprocess.run(
                [sys.executable, "scripts/generate_corpus.py",
                 "--docs", "100",
                 "--vocab-size", "500",
                 "--avg-terms", "20",
                 "--seed", "42",
                 "--out", str(out)],
                capture_output=True,
                text=True,
            )
            assert result.returncode == 0, f"generate_corpus.py failed: {result.stderr}"
            assert out.exists()

            lines = out.read_text().strip().split("\n")
            assert len(lines) == 100

            for i, line in enumerate(lines):
                doc = json.loads(line)
                assert doc["id"] == f"doc-{i:07d}"
                assert "title" in doc
                assert "body" in doc
