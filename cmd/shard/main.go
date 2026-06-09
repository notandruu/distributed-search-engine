package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	searchv1 "github.com/notandruu/distributed-search-engine/gen/search/v1"
	"github.com/notandruu/distributed-search-engine/internal/config"
	"github.com/notandruu/distributed-search-engine/internal/shard"
)

var (
	addrFlag    = flag.String("addr", "", "gRPC listen address (overrides SHARD_ADDR env)")
	shardIDFlag = flag.Int("shard-id", -1, "shard identifier (overrides SHARD_ID env)")
)

func main() {
	flag.Parse()

	cfg := config.LoadShard()
	if *addrFlag != "" {
		cfg.Addr = *addrFlag
	}
	if *shardIDFlag >= 0 {
		cfg.ShardID = *shardIDFlag
	}

	lis, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		log.Fatalf("listen %s: %v", cfg.Addr, err)
	}

	srv := shard.NewServer(int32(cfg.ShardID))

	grpcSrv := grpc.NewServer()
	searchv1.RegisterShardServiceServer(grpcSrv, srv)

	go func() {
		fmt.Printf("{\"service\":\"shard\",\"shard_id\":%d,\"event\":\"start\",\"addr\":%q}\n",
			cfg.ShardID, cfg.Addr)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("serve: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	grpcSrv.GracefulStop()
	fmt.Printf("{\"service\":\"shard\",\"shard_id\":%d,\"event\":\"shutdown\"}\n", cfg.ShardID)
}
