package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry       *prometheus.Registry
	syncRuns       *prometheus.CounterVec // total syncs
	syncDuration   prometheus.Histogram   // time to sync
	dnsRecords     *prometheus.GaugeVec   // known dns records
	dnsRequests    *prometheus.CounterVec // dns provider requests
	caddyRequests  *prometheus.CounterVec // caddy requests
	badgerRequests *prometheus.CounterVec // badgerdb requests
}

// Public interface for metrics operations
func (m *Metrics) IncSyncRun(success bool) {
	status := boolToResult(success)
	m.syncRuns.WithLabelValues(status).Inc()
}

func (m *Metrics) SetSyncDuration(duration time.Duration) {
	m.syncDuration.Observe(duration.Seconds())
}

func (m *Metrics) SetDNSRecords(count int, operation, zone, recordType string, managed bool) {
	if !isValidOperation(operation) || !isValidRecordType(recordType) || zone == "" {
		return
	}
	status := boolToManaged(managed)
	m.dnsRecords.WithLabelValues(operation, zone, recordType, status).Set(float64(count))
}

func (m *Metrics) IncDNSRequest(operation, zone, recordType string, success bool) {
	if !isValidOperation(operation) || !isValidRecordType(recordType) || zone == "" {
		return
	}
	status := boolToResult(success)
	m.dnsRequests.WithLabelValues(operation, zone, recordType, status).Inc()
}

func (m *Metrics) IncCaddyRequest(success bool) {
	status := boolToResult(success)
	m.caddyRequests.WithLabelValues(status).Inc()
}

func (m *Metrics) IncBadgerRequest(success bool) {
	status := boolToResult(success)
	m.badgerRequests.WithLabelValues(status).Inc()
}

// Validation helpers
func boolToResult(b bool) string {
	if b {
		return "success"
	}
	return "failure"
}

func boolToManaged(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func isValidOperation(op string) bool {
	switch op {
	case "create", "update", "delete":
		return true
	}
	return false
}

func isValidRecordType(rt string) bool {
	switch rt {
	case "A", "CNAME", "TXT":
		return true
	}
	return false
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
		}, []string{"status"}),

		syncDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "sync_duration_milliseconds",
			Help:      "Duration of synchronization runs in milliseconds",
			Buckets:   prometheus.DefBuckets,
		}),

		dnsRecords: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "dns_records_current",
			Help:      "Current known DNS records",
		}, []string{"operation", "zone", "type", "managed"}),

		dnsRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "dns_requests_total",
			Help:      "Total DNS provider requests",
		}, []string{"operation", "zone", "record_type", "status"}),

		caddyRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "caddy_requests_total",
			Help:      "Total caddy requests",
		}, []string{"status"}),

		badgerRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "badgerdb_requests_total",
			Help:      "Total badgerdb requests",
		}, []string{"status"}),
	}

	registry.MustRegister(
		m.syncRuns,
		m.syncDuration,
		m.dnsRecords,
		m.dnsRequests,
		m.caddyRequests,
		m.badgerRequests,
	)
	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
