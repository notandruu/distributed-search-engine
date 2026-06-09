package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	searchv1 "github.com/notandruu/distributed-search-engine/gen/search/v1"
	"github.com/notandruu/distributed-search-engine/internal/cache"
	"github.com/notandruu/distributed-search-engine/internal/config"
	"github.com/notandruu/distributed-search-engine/internal/gateway"
)

func main() {
	flag.Parse()

	cfg := config.LoadGateway()

	// Connect to Redis cache (optional — degrade gracefully if unavailable).
	var cacheClient *cache.Client
	c := cache.NewClient(
		cfg.RedisAddr,
		cfg.IndexVersion,
		time.Duration(cfg.CacheTTLSeconds)*time.Second,
	)
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer pingCancel()
	if err := c.Ping(pingCtx); err != nil {
		log.Printf(`{"service":"gateway","event":"redis_unavailable","addr":%q,"err":%q}`+"\n", cfg.RedisAddr, err)
	} else {
		cacheClient = c
	}

	// Build gateway server.
	srv, err := gateway.NewServer(gateway.Options{
		ShardAddrs:      cfg.ShardAddrs,
		Cache:           cacheClient,
		MaxConcurrent:   cfg.MaxConcurrentSearches,
		SearchTimeoutMS: cfg.SearchTimeoutMS,
		ShardTimeoutMS:  cfg.ShardTimeoutMS,
	})
	if err != nil {
		log.Fatalf("create gateway: %v", err)
	}
	defer srv.Close()

	lis, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		log.Fatalf("listen %s: %v", cfg.Addr, err)
	}

	grpcSrv := grpc.NewServer()
	searchv1.RegisterSearchGatewayServer(grpcSrv, srv)

	go func() {
		fmt.Printf(`{"service":"gateway","event":"start","addr":%q,"shards":%d,"redis":%q}`+"\n",
			cfg.Addr, len(cfg.ShardAddrs), cfg.RedisAddr)
		if serveErr := grpcSrv.Serve(lis); serveErr != nil {
			log.Fatalf("serve: %v", serveErr)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	grpcSrv.GracefulStop()
	fmt.Println(`{"service":"gateway","event":"shutdown"}`)
}
