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

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"

	searchv1 "github.com/notandruu/distributed-search-engine/gen/search/v1"
	"github.com/notandruu/distributed-search-engine/internal/config"
	"github.com/notandruu/distributed-search-engine/internal/observability"
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

	// OTel tracing.
	shutdown, err := observability.InitTracer(context.Background(),
		fmt.Sprintf("shard-%d", cfg.ShardID), cfg.OtelEndpoint)
	if err != nil {
		log.Printf(`{"service":"shard","event":"otel_init_failed","err":%q}`+"\n", err)
	} else {
		defer shutdown(context.Background())
	}

	// Prometheus metrics.
	reg := prometheus.NewRegistry()
	prometheus.MustRegister() // reset default registerer; use our own
	metrics := observability.NewShardMetrics(reg, cfg.ShardID)

	// Start metrics server.
	metricsSrv := &http.Server{
		Addr:    cfg.MetricsAddr,
		Handler: observability.MetricsHandler(reg),
	}
	go func() {
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf(`{"service":"shard","event":"metrics_error","err":%q}`+"\n", err)
		}
	}()

	// gRPC server.
	srv := shard.NewServer(int32(cfg.ShardID), metrics)
	grpcSrv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	searchv1.RegisterShardServiceServer(grpcSrv, srv)

	lis, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		log.Fatalf("listen %s: %v", cfg.Addr, err)
	}

	go func() {
		fmt.Printf(`{"service":"shard","shard_id":%d,"event":"start","addr":%q,"metrics":%q}`+"\n",
			cfg.ShardID, cfg.Addr, cfg.MetricsAddr)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("serve: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	grpcSrv.GracefulStop()
	metricsSrv.Shutdown(context.Background())
	fmt.Printf(`{"service":"shard","shard_id":%d,"event":"shutdown"}`+"\n", cfg.ShardID)
}
