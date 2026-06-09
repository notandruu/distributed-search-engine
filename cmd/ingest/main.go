package main

import (
	"flag"
	"fmt"
)

var (
	input     = flag.String("input", "", "path to JSONL corpus file")
	gateway   = flag.String("gateway", "localhost:50051", "gateway gRPC address")
	workers   = flag.Int("workers", 8, "number of ingestion workers")
	batchSize = flag.Int("batch-size", 256, "documents per ingestion batch")
)

func main() {
	flag.Parse()

	if *input == "" {
		fmt.Println("error: --input is required")
		return
	}

	fmt.Printf("ingest: input=%s gateway=%s workers=%d batch=%d\n",
		*input, *gateway, *workers, *batchSize)
}
