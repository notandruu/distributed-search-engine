package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/notandruu/distributed-search-engine/internal/ingest"
)

var (
	inputFlag     = flag.String("input", "", "path to JSONL corpus file (required)")
	gatewayFlag   = flag.String("gateway", "", "gateway address (unused; route directly to shards)")
	shardAddrsFlag = flag.String("shard-addrs", "localhost:50052", "comma-separated shard gRPC addresses")
	workersFlag   = flag.Int("workers", 8, "number of ingestion workers per shard")
	batchSizeFlag = flag.Int("batch-size", 256, "documents per gRPC ingest call")
)

func main() {
	flag.Parse()

	if *inputFlag == "" {
		fmt.Fprintln(os.Stderr, "error: --input is required")
		flag.Usage()
		os.Exit(1)
	}

	shardAddrs := strings.Split(*shardAddrsFlag, ",")
	for i := range shardAddrs {
		shardAddrs[i] = strings.TrimSpace(shardAddrs[i])
	}

	fmt.Printf(`{"event":"ingest_start","input":%q,"shards":%d,"workers":%d,"batch":%d}`+"\n",
		*inputFlag, len(shardAddrs), *workersFlag, *batchSizeFlag)

	_ = gatewayFlag // gateway flag kept for CLI compatibility

	w, err := ingest.NewWorker(ingest.Config{
		ShardAddrs: shardAddrs,
		Workers:    *workersFlag,
		BatchSize:  *batchSizeFlag,
	})
	if err != nil {
		log.Fatalf("create worker: %v", err)
	}
	defer w.Close()

	if err := w.IngestFile(context.Background(), *inputFlag); err != nil {
		log.Fatalf("ingest: %v", err)
	}

	accepted, rejected := w.Stats()
	fmt.Printf(`{"event":"ingest_done","accepted":%d,"rejected":%d}`+"\n", accepted, rejected)
}
