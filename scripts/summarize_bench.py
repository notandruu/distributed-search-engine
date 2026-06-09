#!/usr/bin/env python3
"""Parse ghz JSON output and write a markdown benchmark summary."""

import argparse
import json
import platform
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path


def get_hardware() -> str:
    try:
        cpu = platform.processor() or platform.machine()
        ram_gb = "?"
        try:
            out = subprocess.check_output(["sysctl", "-n", "hw.memsize"], stderr=subprocess.DEVNULL)
            ram_gb = f"{int(out.strip()) // (1024 ** 3)} GB"
        except Exception:
            pass
        return f"{platform.system()} {platform.release()}, {cpu}, RAM: {ram_gb}"
    except Exception:
        return "unknown"


def ns_to_ms(ns: int) -> float:
    return ns / 1_000_000


def main():
    parser = argparse.ArgumentParser(description="Summarize ghz benchmark results")
    parser.add_argument("--input", default="bench/ghz_results.json", help="ghz JSON output")
    parser.add_argument("--out", default="bench/BENCHMARK_SUMMARY.md", help="Markdown output")
    parser.add_argument("--corpus-size", default="?", help="e.g. 1M or 100K")
    parser.add_argument("--num-shards", type=int, default=4)
    args = parser.parse_args()

    with open(args.input) as f:
        data = json.load(f)

    # ghz JSON structure
    total = data.get("count", 0)
    ok_count = data.get("okCount", 0)
    fail_count = data.get("failCount", 0)
    duration_ns = data.get("duration", 0)  # nanoseconds
    rps = data.get("rps", 0.0)
    average_ns = data.get("average", 0)
    fastest_ns = data.get("fastest", 0)
    slowest_ns = data.get("slowest", 0)

    # Latency percentiles
    latency_dist = {item["percentage"]: item["latency"] for item in data.get("latencyDistribution", [])}
    p50_ns = latency_dist.get(50, 0)
    p95_ns = latency_dist.get(95, 0)
    p99_ns = latency_dist.get(99, 0)

    error_rate = (fail_count / total * 100) if total > 0 else 0.0
    duration_s = duration_ns / 1e9

    now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC")
    hardware = get_hardware()

    md = f"""# Benchmark Summary

*Generated: {now}*

## Environment

| | |
|---|---|
| Hardware | {hardware} |
| Corpus size | {args.corpus_size} |
| Shards | {args.num_shards} |
| Test duration | {duration_s:.1f}s |

## Results

| Metric | Value |
|--------|-------|
| Total requests | {total:,} |
| QPS | {rps:.1f} |
| OK | {ok_count:,} ({100-error_rate:.2f}%) |
| Errors | {fail_count:,} ({error_rate:.2f}%) |
| Mean latency | {ns_to_ms(average_ns):.2f} ms |
| p50 | {ns_to_ms(p50_ns):.2f} ms |
| p95 | {ns_to_ms(p95_ns):.2f} ms |
| p99 | {ns_to_ms(p99_ns):.2f} ms |
| Fastest | {ns_to_ms(fastest_ns):.2f} ms |
| Slowest | {ns_to_ms(slowest_ns):.2f} ms |

## How to reproduce

```bash
make compose-up
make index-{args.corpus_size.lower().replace(' ', '')}
make bench
make bench-summary
```

## Raw output

See `bench/ghz_results.json` for full percentile histogram.
"""

    out_path = Path(args.out)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    with open(out_path, "w") as f:
        f.write(md)

    print(f"Summary written to {out_path}")
    print(f"QPS={rps:.1f}  p95={ns_to_ms(p95_ns):.2f}ms  p99={ns_to_ms(p99_ns):.2f}ms")


if __name__ == "__main__":
    main()
