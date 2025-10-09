package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Collector holds all Prometheus metrics for the enterprise system
type Collector struct {
	// Tenant metrics
	TenantsActive         prometheus.Gauge
	TenantsLoaded         prometheus.Counter
	TenantsUnloaded       prometheus.Counter
	TenantsEvicted        prometheus.Counter
	TenantLoadDuration    prometheus.Histogram
	TenantUnloadDuration  prometheus.Histogram

	// HTTP request metrics
	HTTPRequestsTotal     *prometheus.CounterVec
	HTTPRequestDuration   *prometheus.HistogramVec
	HTTPRequestsInFlight  prometheus.Gauge

	// Litestream metrics
	LitestreamActive      prometheus.Gauge
	LitestreamLag         *prometheus.GaugeVec
	LitestreamSyncs       *prometheus.CounterVec
	LitestreamErrors      *prometheus.CounterVec

	// Control plane metrics
	RaftLeader            prometheus.Gauge
	NodesActive           prometheus.Gauge
	PlacementDecisions    *prometheus.CounterVec

	// Gateway metrics
	ProxyCacheSize        prometheus.Gauge
	ProxyCacheHits        prometheus.Counter
	ProxyCacheMisses      prometheus.Counter
	ProxyErrors           *prometheus.CounterVec

	// System metrics
	 CacheUtilization      prometheus.Gauge
	RequestCount          prometheus.Counter
}

// NewCollector creates a new metrics collector with all Prometheus metrics
func NewCollector(subsystem string) *Collector {
	return &Collector{
		// Tenant metrics
		TenantsActive: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "tenants_active",
			Help:      "Number of currently active (loaded) tenants",
		}),
		TenantsLoaded: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "tenants_loaded_total",
			Help:      "Total number of tenants loaded",
		}),
		TenantsUnloaded: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "tenants_unloaded_total",
			Help:      "Total number of tenants unloaded",
		}),
		TenantsEvicted: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "tenants_evicted_total",
			Help:      "Total number of tenants evicted due to inactivity or capacity",
		}),
		TenantLoadDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "tenant_load_duration_seconds",
			Help:      "Time taken to load a tenant",
			Buckets:   prometheus.DefBuckets,
		}),
		TenantUnloadDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "tenant_unload_duration_seconds",
			Help:      "Time taken to unload a tenant",
			Buckets:   prometheus.DefBuckets,
		}),

		// HTTP request metrics
		HTTPRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		}, []string{"method", "path", "status"}),
		HTTPRequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{"method", "path"}),
		HTTPRequestsInFlight: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "http_requests_in_flight",
			Help:      "Number of HTTP requests currently being processed",
		}),

		// Litestream metrics
		LitestreamActive: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "litestream_active_replicas",
			Help:      "Number of active Litestream replications",
		}),
		LitestreamLag: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "litestream_lag_seconds",
			Help:      "Replication lag in seconds",
		}, []string{"tenant_id", "database"}),
		LitestreamSyncs: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "litestream_syncs_total",
			Help:      "Total number of Litestream syncs",
		}, []string{"tenant_id", "database", "status"}),
		LitestreamErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "litestream_errors_total",
			Help:      "Total number of Litestream errors",
		}, []string{"tenant_id", "database", "error_type"}),

		// Control plane metrics
		RaftLeader: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "raft_is_leader",
			Help:      "Whether this node is the Raft leader (1 = leader, 0 = follower)",
		}),
		NodesActive: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "nodes_active",
			Help:      "Number of active tenant nodes",
		}),
		PlacementDecisions: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "placement_decisions_total",
			Help:      "Total number of placement decisions made",
		}, []string{"strategy", "result"}),

		// Gateway metrics
		ProxyCacheSize: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "proxy_cache_size",
			Help:      "Number of cached reverse proxies",
		}),
		ProxyCacheHits: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "proxy_cache_hits_total",
			Help:      "Total number of proxy cache hits",
		}),
		ProxyCacheMisses: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "proxy_cache_misses_total",
			Help:      "Total number of proxy cache misses",
		}),
		ProxyErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "proxy_errors_total",
			Help:      "Total number of proxy errors",
		}, []string{"error_type"}),

		// System metrics
		CacheUtilization: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "cache_utilization_percent",
			Help:      "Cache utilization as a percentage",
		}),
		RequestCount: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "pocketbase_enterprise",
			Subsystem: subsystem,
			Name:      "requests_total",
			Help:      "Total number of requests processed",
		}),
	}
}
