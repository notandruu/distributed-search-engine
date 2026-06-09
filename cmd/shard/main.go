package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

var (
	addr    = flag.String("addr", ":50052", "gRPC listen address")
	shardID = flag.Int("shard-id", 0, "shard identifier")
)

func main() {
	flag.Parse()

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	fmt.Printf("shard %d listening on %s\n", *shardID, *addr)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	lis.Close()
}
