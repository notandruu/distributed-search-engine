package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// GatewayMetrics holds all Prometheus metrics for the gateway service.
type GatewayMetrics struct {
	SearchRequests       *prometheus.CounterVec
	SearchDuration       *prometheus.HistogramVec
	CacheHits            prometheus.Counter
	CacheMisses          prometheus.Counter
	CacheSetErrors       prometheus.Counter
	PartialFailures      prometheus.Counter
	BackpressureRejected prometheus.Counter
}

// NewGatewayMetrics registers and returns gateway Prometheus metrics.
func NewGatewayMetrics(reg prometheus.Registerer) *GatewayMetrics {
	m := &GatewayMetrics{
		SearchRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "search_requests_total",
			Help: "Total search requests handled by the gateway.",
		}, []string{"status"}),

		SearchDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "search_request_duration_seconds",
			Help:    "Gateway search request latency.",
			Buckets: prometheus.DefBuckets,
		}, []string{"cache_hit"}),

		CacheHits: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "search_cache_hits_total",
			Help: "Total Redis cache hits.",
		}),

		CacheMisses: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "search_cache_misses_total",
			Help: "Total Redis cache misses.",
		}),

		CacheSetErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "search_cache_set_errors_total",
			Help: "Total Redis cache set errors.",
		}),

		PartialFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "search_partial_failures_total",
			Help: "Total search requests that returned partial results due to shard failure.",
		}),

		BackpressureRejected: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "backpressure_rejections_total",
			Help: "Total requests rejected due to backpressure.",
		}),
	}

	reg.MustRegister(
		m.SearchRequests,
		m.SearchDuration,
		m.CacheHits,
		m.CacheMisses,
		m.CacheSetErrors,
		m.PartialFailures,
		m.BackpressureRejected,
	)

	return m
}

// ShardMetrics holds all Prometheus metrics for a shard service.
type ShardMetrics struct {
	SearchDuration  *prometheus.HistogramVec
	DocsIndexed     prometheus.Counter
	UniqueTermsGauge prometheus.Gauge
	PostingsGauge   prometheus.Gauge
	IngestBatches   prometheus.Counter
	IngestDocuments prometheus.Counter
	IngestErrors    prometheus.Counter
}

// NewShardMetrics registers and returns shard Prometheus metrics.
func NewShardMetrics(reg prometheus.Registerer, shardID int) *ShardMetrics {
	labels := prometheus.Labels{"shard_id": itoa(shardID)}

	m := &ShardMetrics{
		SearchDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "shard_search_duration_seconds",
			Help:        "Shard-local search latency.",
			Buckets:     prometheus.DefBuckets,
			ConstLabels: labels,
		}, []string{}),

		DocsIndexed: prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "shard_docs_indexed_total",
			Help:        "Total documents indexed by this shard.",
			ConstLabels: labels,
		}),

		UniqueTermsGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "shard_unique_terms",
			Help:        "Current number of unique indexed terms in this shard.",
			ConstLabels: labels,
		}),

		PostingsGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "shard_postings_total",
			Help:        "Current total posting list entries in this shard.",
			ConstLabels: labels,
		}),

		IngestBatches: prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "ingest_batches_total",
			Help:        "Total ingest batches processed.",
			ConstLabels: labels,
		}),

		IngestDocuments: prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "ingest_documents_total",
			Help:        "Total documents successfully ingested.",
			ConstLabels: labels,
		}),

		IngestErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "ingest_errors_total",
			Help:        "Total documents rejected during ingest.",
			ConstLabels: labels,
		}),
	}

	reg.MustRegister(
		m.SearchDuration,
		m.DocsIndexed,
		m.UniqueTermsGauge,
		m.PostingsGauge,
		m.IngestBatches,
		m.IngestDocuments,
		m.IngestErrors,
	)

	return m
}

// ServeMetrics starts an HTTP server exposing Prometheus metrics at addr/metrics.
func ServeMetrics(addr string, reg prometheus.Gatherer) *http.Server {
	srv := &http.Server{Addr: addr, Handler: MetricsHandler(reg)}
	go srv.ListenAndServe()
	return srv
}

// MetricsHandler returns an http.Handler that serves Prometheus metrics.
func MetricsHandler(reg prometheus.Gatherer) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))
	return mux
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
