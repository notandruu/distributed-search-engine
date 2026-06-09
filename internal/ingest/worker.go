package ingest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"os"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	searchv1 "github.com/notandruu/distributed-search-engine/gen/search/v1"
)

// RawDoc is the JSONL document format.
type RawDoc struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
}

// Config holds ingestion worker options.
type Config struct {
	ShardAddrs []string
	Workers    int
	BatchSize  int
}

// Worker manages concurrent document ingestion across shards.
type Worker struct {
	shards    []searchv1.ShardServiceClient
	conns     []*grpc.ClientConn
	workers   int
	batchSize int

	accepted atomic.Int64
	rejected atomic.Int64
}

// NewWorker connects to all shard addresses and returns a Worker.
func NewWorker(cfg Config) (*Worker, error) {
	if cfg.Workers <= 0 {
		cfg.Workers = 8
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 256
	}

	w := &Worker{
		workers:   cfg.Workers,
		batchSize: cfg.BatchSize,
	}

	for _, addr := range cfg.ShardAddrs {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			w.Close()
			return nil, fmt.Errorf("dial shard %s: %w", addr, err)
		}
		w.conns = append(w.conns, conn)
		w.shards = append(w.shards, searchv1.NewShardServiceClient(conn))
	}

	return w, nil
}

// Close releases gRPC connections.
func (w *Worker) Close() {
	for _, conn := range w.conns {
		conn.Close()
	}
}

// IngestFile reads a JSONL file and distributes documents to shards.
func (w *Worker) IngestFile(ctx context.Context, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return w.IngestReader(ctx, f)
}

// IngestReader reads JSONL from r and distributes documents to shards.
func (w *Worker) IngestReader(ctx context.Context, r io.Reader) error {
	numShards := len(w.shards)
	if numShards == 0 {
		return fmt.Errorf("no shard clients configured")
	}

	// One channel per shard, buffered to 2x worker count.
	queues := make([]chan *searchv1.Document, numShards)
	for i := range queues {
		queues[i] = make(chan *searchv1.Document, w.workers*2*w.batchSize)
	}

	// Start worker goroutines — one per shard, handling batching and RPCs.
	var wg sync.WaitGroup
	for shardIdx := range w.shards {
		wg.Add(1)
		go func(si int) {
			defer wg.Done()
			w.shardWorker(ctx, si, queues[si])
		}(shardIdx)
	}

	// Read documents and route to shard queues.
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		var doc RawDoc
		if err := json.Unmarshal(scanner.Bytes(), &doc); err != nil {
			log.Printf(`{"event":"parse_error","line":%d,"err":%q}`, lineNum, err)
			w.rejected.Add(1)
			continue
		}
		if doc.ID == "" {
			w.rejected.Add(1)
			continue
		}

		shardIdx := routeDoc(doc.ID, numShards)
		queues[shardIdx] <- &searchv1.Document{
			Id:    doc.ID,
			Title: doc.Title,
			Body:  doc.Body,
			Url:   doc.URL,
		}
	}

	// Close queues to signal workers to flush and exit.
	for _, q := range queues {
		close(q)
	}
	wg.Wait()

	return scanner.Err()
}

// shardWorker drains its queue, batches documents, and sends to the shard.
func (w *Worker) shardWorker(ctx context.Context, shardIdx int, queue <-chan *searchv1.Document) {
	batch := make([]*searchv1.Document, 0, w.batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		resp, err := w.shards[shardIdx].Ingest(ctx, &searchv1.IngestRequest{Documents: batch})
		if err != nil {
			log.Printf(`{"event":"ingest_error","shard":%d,"docs":%d,"err":%q}`, shardIdx, len(batch), err)
			w.rejected.Add(int64(len(batch)))
		} else {
			w.accepted.Add(resp.Accepted)
			w.rejected.Add(resp.Rejected)
		}
		batch = batch[:0]
	}

	for doc := range queue {
		batch = append(batch, doc)
		if len(batch) >= w.batchSize {
			flush()
		}
	}
	flush() // final partial batch
}

// routeDoc assigns a document to a shard by crc32 hash of its ID.
func routeDoc(docID string, numShards int) int {
	return int(crc32.ChecksumIEEE([]byte(docID))) % numShards
}

// Stats returns ingestion totals.
func (w *Worker) Stats() (accepted, rejected int64) {
	return w.accepted.Load(), w.rejected.Load()
}
