SHELL := /bin/bash
MODULE  := github.com/notandruu/distributed-search-engine
BINARY_DIR := bin
GO      := go
PROTOC  := protoc
BUF     := buf

# ── Protobuf ──────────────────────────────────────────────────────────────────
.PHONY: proto
proto:
	@echo "==> Generating protobuf code"
	@mkdir -p gen/search/v1
	$(PROTOC) \
		--go_out=gen/search/v1 --go_opt=paths=import \
		--go_opt=Mproto/search/v1/search.proto=github.com/notandruu/distributed-search-engine/gen/search/v1 \
		--go-grpc_out=gen/search/v1 --go-grpc_opt=paths=import \
		--go-grpc_opt=Mproto/search/v1/search.proto=github.com/notandruu/distributed-search-engine/gen/search/v1 \
		proto/search/v1/search.proto
	@mv gen/search/v1/github.com/notandruu/distributed-search-engine/gen/search/v1/* gen/search/v1/ 2>/dev/null || true
	@rm -rf gen/search/v1/github.com 2>/dev/null || true
	@echo "==> Done"

# ── Build ─────────────────────────────────────────────────────────────────────
.PHONY: build
build:
	@echo "==> Building binaries"
	@mkdir -p $(BINARY_DIR)
	$(GO) build -o $(BINARY_DIR)/gateway ./cmd/gateway
	$(GO) build -o $(BINARY_DIR)/shard   ./cmd/shard
	$(GO) build -o $(BINARY_DIR)/ingest  ./cmd/ingest

# ── Test ──────────────────────────────────────────────────────────────────────
.PHONY: test
test:
	@echo "==> Running tests"
	$(GO) test ./...

.PHONY: test-race
test-race:
	@echo "==> Running tests with race detector"
	$(GO) test -race ./internal/...

# ── Lint ──────────────────────────────────────────────────────────────────────
.PHONY: lint
lint:
	@echo "==> Running linter"
	golangci-lint run ./...

# ── Docker ────────────────────────────────────────────────────────────────────
.PHONY: docker-build
docker-build:
	@echo "==> Building Docker images"
	docker build -f deploy/Dockerfile.gateway -t dse-gateway:latest .
	docker build -f deploy/Dockerfile.shard   -t dse-shard:latest   .
	docker build -f deploy/Dockerfile.ingest  -t dse-ingest:latest  .

.PHONY: compose-up
compose-up:
	@echo "==> Starting Docker Compose cluster"
	docker compose -f deploy/docker-compose.yml up -d

.PHONY: compose-down
compose-down:
	@echo "==> Stopping Docker Compose cluster"
	docker compose -f deploy/docker-compose.yml down -v

.PHONY: compose-logs
compose-logs:
	docker compose -f deploy/docker-compose.yml logs -f

# ── Data & Corpus ─────────────────────────────────────────────────────────────
.PHONY: generate-corpus
generate-corpus:
	@echo "==> Generating 1M document corpus"
	python3 scripts/generate_corpus.py \
		--docs 1000000 \
		--vocab-size 50000 \
		--avg-terms 120 \
		--seed 42 \
		--out data/corpus_1m.jsonl

.PHONY: generate-corpus-100k
generate-corpus-100k:
	@echo "==> Generating 100K document corpus"
	python3 scripts/generate_corpus.py \
		--docs 100000 \
		--vocab-size 50000 \
		--avg-terms 120 \
		--seed 42 \
		--out data/corpus_100k.jsonl

.PHONY: index-100k
index-100k: generate-corpus-100k
	@echo "==> Indexing 100K documents"
	$(BINARY_DIR)/ingest \
		--input data/corpus_100k.jsonl \
		--gateway localhost:50051 \
		--workers 8 \
		--batch-size 256

.PHONY: index-1m
index-1m: generate-corpus
	@echo "==> Indexing 1M documents"
	$(BINARY_DIR)/ingest \
		--input data/corpus_1m.jsonl \
		--gateway localhost:50051 \
		--workers 16 \
		--batch-size 256

# ── Evaluation ────────────────────────────────────────────────────────────────
.PHONY: eval
eval:
	@echo "==> Running relevance evaluation"
	python3 scripts/eval.py \
		--gateway localhost:50051 \
		--queries eval/queries.jsonl \
		--qrels eval/qrels.tsv \
		--out bench/eval_results.json \
		--summary bench/eval_summary.md

# ── Benchmarking ──────────────────────────────────────────────────────────────
.PHONY: bench
bench:
	@echo "==> Running ghz load test"
	bash bench/run_ghz.sh

.PHONY: bench-summary
bench-summary:
	@echo "==> Summarizing benchmark results"
	python3 scripts/summarize_bench.py \
		--input bench/ghz_results.json \
		--out bench/BENCHMARK_SUMMARY.md

# ── Kubernetes ────────────────────────────────────────────────────────────────
.PHONY: k8s-up
k8s-up:
	@echo "==> Deploying to local Kubernetes cluster"
	kubectl apply -f deploy/k8s/

.PHONY: k8s-down
k8s-down:
	@echo "==> Tearing down Kubernetes deployment"
	kubectl delete -f deploy/k8s/

.PHONY: k8s-forward
k8s-forward:
	@echo "==> Port-forwarding gateway service"
	kubectl port-forward -n distributed-search svc/gateway 50051:50051

# ── Utilities ─────────────────────────────────────────────────────────────────
.PHONY: clean
clean:
	@echo "==> Cleaning build artifacts"
	rm -rf $(BINARY_DIR) data/

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*"}; {printf "  %-20s\n", $$1}'
