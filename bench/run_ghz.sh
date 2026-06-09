#!/usr/bin/env bash
set -euo pipefail

GATEWAY="${GATEWAY_ADDR:-0.0.0.0:50051}"
CONCURRENCY="${CONCURRENCY:-64}"
REQUESTS="${REQUESTS:-100000}"
PROTO="proto/search/v1/search.proto"
OUTPUT="bench/ghz_results.json"

echo "==> Running ghz load test"
echo "    gateway:     ${GATEWAY}"
echo "    concurrency: ${CONCURRENCY}"
echo "    requests:    ${REQUESTS}"

ghz \
  --insecure \
  --proto "${PROTO}" \
  --call search.v1.SearchGateway/Search \
  -d '{"query":"distributed systems consensus replication fault tolerance","top_k":10,"explain":false}' \
  -c "${CONCURRENCY}" \
  -n "${REQUESTS}" \
  "${GATEWAY}" \
  --format json \
  --output "${OUTPUT}"

echo "==> Results written to ${OUTPUT}"
