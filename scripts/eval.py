#!/usr/bin/env python3
"""
Evaluate search relevance using MRR and NDCG@10.

Calls the gRPC gateway and compares results against a qrels file.

Usage:
    python3 scripts/eval.py \
        --gateway localhost:50051 \
        --queries eval/queries.jsonl \
        --qrels eval/qrels.tsv \
        --out bench/eval_results.json \
        --summary bench/eval_summary.md
"""

import argparse
import json
import math
import sys
from pathlib import Path

import grpc

# Generated proto stubs — import path matches go_package option.
# Install with: pip install grpcio grpcio-tools
# Then regenerate with: python -m grpc_tools.protoc ...
# For local testing without stubs, we use a simple HTTP fallback.

_HAVE_STUBS = False
try:
    import gen.search.v1.search_pb2 as pb2
    import gen.search.v1.search_pb2_grpc as pb2_grpc
    _HAVE_STUBS = True
except ImportError:
    pass


# ── Metrics ────────────────────────────────────────────────────────────────────

def reciprocal_rank(results: list[str], relevant: set[str]) -> float:
    """Compute reciprocal rank: 1/rank of first relevant result, 0 if none found."""
    for rank, doc_id in enumerate(results, start=1):
        if doc_id in relevant:
            return 1.0 / rank
    return 0.0


def dcg(results: list[str], grades: dict[str, int], k: int) -> float:
    """Compute Discounted Cumulative Gain at k."""
    score = 0.0
    for rank, doc_id in enumerate(results[:k], start=1):
        rel = grades.get(doc_id, 0)
        score += (2 ** rel - 1) / math.log2(rank + 1)
    return score


def ideal_dcg(grades: dict[str, int], k: int) -> float:
    """Compute Ideal DCG at k (perfect ranking of relevant docs)."""
    sorted_rels = sorted(grades.values(), reverse=True)[:k]
    score = 0.0
    for rank, rel in enumerate(sorted_rels, start=1):
        score += (2 ** rel - 1) / math.log2(rank + 1)
    return score


def ndcg(results: list[str], grades: dict[str, int], k: int) -> float:
    """Compute Normalized DCG at k."""
    idcg = ideal_dcg(grades, k)
    if idcg == 0:
        return 0.0
    return dcg(results, grades, k) / idcg


def mean_reciprocal_rank(per_query_rr: list[float]) -> float:
    if not per_query_rr:
        return 0.0
    return sum(per_query_rr) / len(per_query_rr)


def mean_ndcg(per_query_ndcg: list[float]) -> float:
    if not per_query_ndcg:
        return 0.0
    return sum(per_query_ndcg) / len(per_query_ndcg)


# ── Data loading ───────────────────────────────────────────────────────────────

def load_queries(path: str) -> list[dict]:
    queries = []
    with open(path) as f:
        for line in f:
            line = line.strip()
            if line:
                queries.append(json.loads(line))
    return queries


def load_qrels(path: str) -> dict[str, dict[str, int]]:
    """Load qrels TSV: query_id<TAB>doc_id<TAB>grade"""
    qrels: dict[str, dict[str, int]] = {}
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            parts = line.split("\t")
            if len(parts) < 3:
                continue
            qid, doc_id, grade = parts[0], parts[1], int(parts[2])
            qrels.setdefault(qid, {})[doc_id] = grade
    return qrels


# ── Search client ─────────────────────────────────────────────────────────────

def search_grpc(channel, query: str, top_k: int = 10) -> list[str]:
    """Issue a gRPC Search and return ordered doc_ids."""
    if not _HAVE_STUBS:
        raise RuntimeError(
            "gRPC stubs not available. Generate with:\n"
            "  pip install grpcio-tools\n"
            "  python -m grpc_tools.protoc -I. --python_out=. --grpc_python_out=. "
            "proto/search/v1/search.proto"
        )
    stub = pb2_grpc.SearchGatewayStub(channel)
    req = pb2.SearchRequest(query=query, top_k=top_k, explain=False)
    resp = stub.Search(req)
    return [r.doc_id for r in resp.results]


# ── Main ───────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(description="Evaluate search relevance (MRR, NDCG@10)")
    parser.add_argument("--gateway", default="localhost:50051", help="Gateway gRPC address")
    parser.add_argument("--queries", default="eval/queries.jsonl", help="Queries JSONL file")
    parser.add_argument("--qrels", default="eval/qrels.tsv", help="Relevance judgments TSV")
    parser.add_argument("--out", default="bench/eval_results.json", help="JSON output path")
    parser.add_argument("--summary", default="bench/eval_summary.md", help="Markdown summary path")
    parser.add_argument("--top-k", type=int, default=10, help="Results per query")
    parser.add_argument("--dry-run", action="store_true",
                        help="Skip gRPC calls, compute metrics on dummy results")
    args = parser.parse_args()

    queries = load_queries(args.queries)
    qrels = load_qrels(args.qrels)

    print(f"Loaded {len(queries)} queries, {sum(len(v) for v in qrels.values())} qrels",
          file=sys.stderr)

    results_per_query = {}

    if args.dry_run:
        print("Dry run: using empty results for all queries", file=sys.stderr)
        for q in queries:
            results_per_query[q["query_id"]] = []
    else:
        channel = grpc.insecure_channel(args.gateway)
        for q in queries:
            qid = q["query_id"]
            query_text = q["query"]
            try:
                doc_ids = search_grpc(channel, query_text, top_k=args.top_k)
                results_per_query[qid] = doc_ids
                print(f"  {qid}: {len(doc_ids)} results", file=sys.stderr)
            except Exception as e:
                print(f"  {qid}: ERROR {e}", file=sys.stderr)
                results_per_query[qid] = []
        channel.close()

    # Compute per-query metrics.
    per_query = []
    rr_values = []
    ndcg_values = []

    for q in queries:
        qid = q["query_id"]
        query_text = q["query"]
        grades = qrels.get(qid, {})
        relevant = set(grades.keys())
        result_ids = results_per_query.get(qid, [])

        rr = reciprocal_rank(result_ids, relevant)
        nd = ndcg(result_ids, grades, k=10)

        rr_values.append(rr)
        ndcg_values.append(nd)

        per_query.append({
            "query_id": qid,
            "query": query_text,
            "results": result_ids,
            "rr": round(rr, 4),
            "ndcg_10": round(nd, 4),
            "num_relevant": len(relevant),
        })

    mrr = mean_reciprocal_rank(rr_values)
    mndcg = mean_ndcg(ndcg_values)

    summary = {
        "gateway": args.gateway,
        "num_queries": len(queries),
        "top_k": args.top_k,
        "MRR": round(mrr, 4),
        "NDCG@10": round(mndcg, 4),
        "per_query": per_query,
    }

    # Write JSON results.
    out_path = Path(args.out)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    with open(out_path, "w") as f:
        json.dump(summary, f, indent=2)
    print(f"Results written to {out_path}", file=sys.stderr)

    # Write markdown summary.
    md_path = Path(args.summary)
    md_path.parent.mkdir(parents=True, exist_ok=True)
    with open(md_path, "w") as f:
        f.write("# Relevance Evaluation\n\n")
        f.write(f"| Metric | Score |\n|--------|-------|\n")
        f.write(f"| MRR | {mrr:.4f} |\n")
        f.write(f"| NDCG@10 | {mndcg:.4f} |\n\n")
        f.write(f"*{len(queries)} queries evaluated against gateway at `{args.gateway}`*\n\n")
        f.write("## Per-query results\n\n")
        f.write("| Query ID | Query | RR | NDCG@10 | Relevant docs |\n")
        f.write("|----------|-------|----|---------|---------------|\n")
        for pq in per_query:
            f.write(f"| {pq['query_id']} | {pq['query'][:40]} | {pq['rr']:.4f} | {pq['ndcg_10']:.4f} | {pq['num_relevant']} |\n")
    print(f"Summary written to {md_path}", file=sys.stderr)

    print(f"\nMRR={mrr:.4f}  NDCG@10={mndcg:.4f}")


if __name__ == "__main__":
    main()
