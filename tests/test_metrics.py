"""Tests for MRR, DCG, and NDCG@10 metric implementations in scripts/eval.py."""

import sys
import os
import math

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from scripts.eval import (
    reciprocal_rank,
    dcg,
    ideal_dcg,
    ndcg,
    mean_reciprocal_rank,
    mean_ndcg,
)


class TestReciprocalRank:
    def test_first_result_relevant(self):
        rr = reciprocal_rank(["doc-1", "doc-2", "doc-3"], {"doc-1"})
        assert rr == 1.0

    def test_second_result_relevant(self):
        rr = reciprocal_rank(["doc-1", "doc-2", "doc-3"], {"doc-2"})
        assert abs(rr - 0.5) < 1e-9

    def test_third_result_relevant(self):
        rr = reciprocal_rank(["doc-1", "doc-2", "doc-3"], {"doc-3"})
        assert abs(rr - 1 / 3) < 1e-9

    def test_no_relevant(self):
        rr = reciprocal_rank(["doc-1", "doc-2"], {"doc-99"})
        assert rr == 0.0

    def test_empty_results(self):
        rr = reciprocal_rank([], {"doc-1"})
        assert rr == 0.0

    def test_empty_relevant(self):
        rr = reciprocal_rank(["doc-1"], set())
        assert rr == 0.0

    def test_multiple_relevant_uses_first(self):
        # Both doc-2 and doc-3 are relevant; RR should be 1/2 (first hit at rank 2)
        rr = reciprocal_rank(["doc-1", "doc-2", "doc-3"], {"doc-2", "doc-3"})
        assert abs(rr - 0.5) < 1e-9


class TestDCG:
    def test_perfect_ranking(self):
        grades = {"a": 3, "b": 2, "c": 1}
        results = ["a", "b", "c"]
        score = dcg(results, grades, k=3)
        expected = (2**3 - 1) / math.log2(2) + (2**2 - 1) / math.log2(3) + (2**1 - 1) / math.log2(4)
        assert abs(score - expected) < 1e-9

    def test_no_relevant(self):
        grades = {}
        results = ["a", "b", "c"]
        assert dcg(results, grades, k=3) == 0.0

    def test_truncated_at_k(self):
        grades = {"a": 3, "b": 2, "c": 1}
        score_k1 = dcg(["a", "b", "c"], grades, k=1)
        expected = (2**3 - 1) / math.log2(2)
        assert abs(score_k1 - expected) < 1e-9


class TestIdealDCG:
    def test_sorted_descending(self):
        grades = {"a": 1, "b": 3, "c": 2}
        # Ideal order: b(3), c(2), a(1)
        idcg = ideal_dcg(grades, k=3)
        expected = (2**3 - 1) / math.log2(2) + (2**2 - 1) / math.log2(3) + (2**1 - 1) / math.log2(4)
        assert abs(idcg - expected) < 1e-9

    def test_empty_grades(self):
        assert ideal_dcg({}, k=10) == 0.0


class TestNDCG:
    def test_perfect_ranking_gives_1(self):
        grades = {"a": 3, "b": 2, "c": 1}
        results = ["a", "b", "c"]
        score = ndcg(results, grades, k=3)
        assert abs(score - 1.0) < 1e-9

    def test_reversed_ranking_less_than_1(self):
        grades = {"a": 3, "b": 2, "c": 1}
        results = ["c", "b", "a"]  # worst order
        score = ndcg(results, grades, k=3)
        assert score < 1.0
        assert score > 0.0

    def test_no_relevant_gives_0(self):
        grades = {}
        results = ["a", "b", "c"]
        assert ndcg(results, grades, k=10) == 0.0

    def test_empty_results_gives_0(self):
        grades = {"a": 3}
        assert ndcg([], grades, k=10) == 0.0

    def test_ndcg_k10_standard(self):
        # Standard TREC-style test: one relevant doc at rank 5
        grades = {"doc-5": 1}
        results = [f"doc-{i}" for i in range(1, 11)]
        score = ndcg(results, grades, k=10)
        expected_dcg = (2**1 - 1) / math.log2(6)  # rank 5
        expected_idcg = (2**1 - 1) / math.log2(2)  # ideal: rank 1
        assert abs(score - expected_dcg / expected_idcg) < 1e-9


class TestMRR:
    def test_basic(self):
        rr_values = [1.0, 0.5, 1 / 3]
        mrr = mean_reciprocal_rank(rr_values)
        assert abs(mrr - (1.0 + 0.5 + 1 / 3) / 3) < 1e-9

    def test_empty(self):
        assert mean_reciprocal_rank([]) == 0.0

    def test_all_zero(self):
        assert mean_reciprocal_rank([0.0, 0.0, 0.0]) == 0.0


class TestMeanNDCG:
    def test_basic(self):
        values = [0.8, 0.6, 1.0]
        result = mean_ndcg(values)
        assert abs(result - 0.8) < 1e-9

    def test_empty(self):
        assert mean_ndcg([]) == 0.0
