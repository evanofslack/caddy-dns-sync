package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry *prometheus.Registry

	// Synchronization metrics
	syncRuns       *prometheus.CounterVec
	syncDuration prometheus.Histogram

	// DNS operation metrics
	dnsChanges        *prometheus.CounterVec
	dnsProviderErrors *prometheus.CounterVec

	// State management metrics
	stateChanges *prometheus.CounterVec
	stateSizeBytes    prometheus.Gauge

	// Caddy interaction metrics
	caddyFetches      *prometheus.CounterVec
	caddyDomainsDiscovered prometheus.Gauge

	// Reconciliation metrics
	pendingChanges        prometheus.Gauge
	reconciliationLatency prometheus.Histogram
}

func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()
	namespace := "caddy_dns_sync"

	m := &Metrics{
		registry: registry,

		syncRuns: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "sync_runs_total",
			Help:      "Total number of synchronization runs",
		}, []string{"status", "dry_run"}),

		syncDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "sync_duration_milliseconds",
			Help:      "Duration of synchronization runs in milliseconds",
			Buckets:   prometheus.DefBuckets,
		}),

		dnsChanges: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "dns_changes_total",
			Help:      "Total DNS record changes by operation type",
		}, []string{"operation", "zone", "record_type", "status"}),

		dnsProviderErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "dns_provider_errors_total",
			Help:      "Total DNS provider errors by error type",
		}, []string{"error_type"}),

		stateChanges: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "state_changes_total",
			Help:      "Total state persistence operations",
		}, []string{"operation", "status"}),

		stateSizeBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "state_size_bytes",
			Help:      "Size of persisted state in bytes",
		}),

		caddyFetches: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "caddy_config_fetches_total",
			Help:      "Total Caddy config fetch attempts",
		}, []string{"status"}),

		caddyDomainsDiscovered: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "caddy_domains_discovered",
			Help:      "Number of domains discovered in Caddy config",
		}),

		pendingChanges: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "pending_changes",
			Help:      "Number of unprocessed configuration changes",
		}),

		reconciliationLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "reconciliation_latency_seconds",
			Help:      "Time between config change and DNS update",
			Buckets:   []float64{1, 5, 10, 30, 60, 120, 300},
		}),
	}

	registry.MustRegister(
		m.syncRuns,
		m.syncDuration,
		m.dnsChanges,
		m.dnsProviderErrors,
		m.stateChanges,
		m.stateSizeBytes,
		m.caddyFetches,
		m.caddyDomainsDiscovered,
		m.pendingChanges,
		m.reconciliationLatency,
	)

	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
