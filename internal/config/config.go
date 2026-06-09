package config

import (
	"os"
	"strconv"
	"strings"
)

type Gateway struct {
	Addr                 string
	MetricsAddr          string
	ShardAddrs           []string
	RedisAddr            string
	OtelEndpoint         string
	MaxConcurrentSearches int
	SearchTimeoutMS      int
	ShardTimeoutMS       int
	CacheTTLSeconds      int
	IndexVersion         string
}

type Shard struct {
	Addr         string
	MetricsAddr  string
	ShardID      int
	OtelEndpoint string
}

func LoadGateway() Gateway {
	shardAddrsStr := getEnv("SHARD_ADDRS", "localhost:50052")
	shards := strings.Split(shardAddrsStr, ",")
	for i := range shards {
		shards[i] = strings.TrimSpace(shards[i])
	}

	return Gateway{
		Addr:                  getEnv("GATEWAY_ADDR", ":50051"),
		MetricsAddr:           getEnv("METRICS_ADDR", ":2112"),
		ShardAddrs:            shards,
		RedisAddr:             getEnv("REDIS_ADDR", "localhost:6379"),
		OtelEndpoint:          getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		MaxConcurrentSearches: getEnvInt("MAX_CONCURRENT_SEARCHES", 256),
		SearchTimeoutMS:       getEnvInt("SEARCH_TIMEOUT_MS", 100),
		ShardTimeoutMS:        getEnvInt("SHARD_TIMEOUT_MS", 75),
		CacheTTLSeconds:       getEnvInt("CACHE_TTL_SECONDS", 300),
		IndexVersion:          getEnv("INDEX_VERSION", "1"),
	}
}

func LoadShard() Shard {
	return Shard{
		Addr:         getEnv("SHARD_ADDR", ":50052"),
		MetricsAddr:  getEnv("METRICS_ADDR", ":2113"),
		ShardID:      getEnvInt("SHARD_ID", 0),
		OtelEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
