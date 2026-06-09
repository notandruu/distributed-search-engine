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
	"github.com/notandruu/distributed-search-engine/internal/gateway"
)

func main() {
	flag.Parse()

	cfg := config.LoadGateway()

	lis, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		log.Fatalf("listen %s: %v", cfg.Addr, err)
	}

	srv, err := gateway.NewServer(gateway.Options{
		ShardAddrs:      cfg.ShardAddrs,
		MaxConcurrent:   cfg.MaxConcurrentSearches,
		SearchTimeoutMS: cfg.SearchTimeoutMS,
		ShardTimeoutMS:  cfg.ShardTimeoutMS,
	})
	if err != nil {
		log.Fatalf("create gateway: %v", err)
	}
	defer srv.Close()

	grpcSrv := grpc.NewServer()
	searchv1.RegisterSearchGatewayServer(grpcSrv, srv)

	go func() {
		fmt.Printf("{\"service\":\"gateway\",\"event\":\"start\",\"addr\":%q,\"shards\":%d}\n",
			cfg.Addr, len(cfg.ShardAddrs))
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("serve: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	grpcSrv.GracefulStop()
	fmt.Println(`{"service":"gateway","event":"shutdown"}`)
}
