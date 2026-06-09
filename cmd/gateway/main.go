package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"

	searchv1 "github.com/notandruu/distributed-search-engine/gen/search/v1"
	"github.com/notandruu/distributed-search-engine/internal/cache"
	"github.com/notandruu/distributed-search-engine/internal/config"
	"github.com/notandruu/distributed-search-engine/internal/gateway"
	"github.com/notandruu/distributed-search-engine/internal/observability"
)

func main() {
	flag.Parse()

	cfg := config.LoadGateway()

	// OTel tracing.
	shutdown, err := observability.InitTracer(context.Background(), "gateway", cfg.OtelEndpoint)
	if err != nil {
		log.Printf(`{"service":"gateway","event":"otel_init_failed","err":%q}`+"\n", err)
	} else {
		defer shutdown(context.Background())
	}

	// Prometheus metrics.
	reg := prometheus.NewRegistry()
	metrics := observability.NewGatewayMetrics(reg)

	// Start metrics server.
	metricsSrv := &http.Server{
		Addr:    cfg.MetricsAddr,
		Handler: observability.MetricsHandler(reg),
	}
	go func() {
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf(`{"service":"gateway","event":"metrics_error","err":%q}`+"\n", err)
		}
	}()

	// Redis cache.
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
		Metrics:         metrics,
		MaxConcurrent:   cfg.MaxConcurrentSearches,
		SearchTimeoutMS: cfg.SearchTimeoutMS,
		ShardTimeoutMS:  cfg.ShardTimeoutMS,
	})
	if err != nil {
		log.Fatalf("create gateway: %v", err)
	}
	defer srv.Close()

	// gRPC server with OTel instrumentation.
	lis, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		log.Fatalf("listen %s: %v", cfg.Addr, err)
	}
	grpcSrv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	searchv1.RegisterSearchGatewayServer(grpcSrv, srv)

	go func() {
		fmt.Printf(`{"service":"gateway","event":"start","addr":%q,"shards":%d,"redis":%q,"metrics":%q}`+"\n",
			cfg.Addr, len(cfg.ShardAddrs), cfg.RedisAddr, cfg.MetricsAddr)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("serve: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	grpcSrv.GracefulStop()
	metricsSrv.Shutdown(context.Background())
	fmt.Println(`{"service":"gateway","event":"shutdown"}`)
}
