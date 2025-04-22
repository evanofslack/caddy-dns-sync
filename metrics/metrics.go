package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry       *prometheus.Registry
	syncRuns       *prometheus.CounterVec // total syncs
	syncDuration   prometheus.Histogram   // time to sync
	dnsOperations  *prometheus.CounterVec // dns operations
	dnsRequests    *prometheus.CounterVec // dns provider requests
	caddyEntries   *prometheus.GaugeVec   // known caddy entries
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

func (m *Metrics) IncDNSOperation(operation, zone, recordType string) {
	if !isValidOperation(operation) || !isValidRecordType(recordType) || zone == "" {
		return
	}
	m.dnsOperations.WithLabelValues(operation, zone, recordType).Inc()
}

func (m *Metrics) IncDNSRequest(operation, zone string, success bool) {
	if !isValidOperation(operation) || zone == "" {
		return
	}
	status := boolToResult(success)
	m.dnsRequests.WithLabelValues(operation, zone, status).Inc()
}

func (m *Metrics) SetCaddyEntries(count int, rp bool) {
	rpstr := boolToStr(rp)
	m.caddyEntries.WithLabelValues(rpstr).Set(float64(count))
}

func (m *Metrics) IncCaddyRequest(success bool, code int) {
	status := boolToResult(success)
	scode := strconv.Itoa(code)
	m.caddyRequests.WithLabelValues(status, scode).Inc()
}

func (m *Metrics) IncBadgerRequest(operation string, success bool) {
	if !isValidOperation(operation) {
		return
	}
	status := boolToResult(success)
	m.badgerRequests.WithLabelValues(operation, status).Inc()
}

// Validation helpers
func boolToResult(b bool) string {
	if b {
		return "success"
	}
	return "failure"
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func isValidOperation(op string) bool {
	switch op {
	case "create", "read", "update", "delete", "skip":
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

func New(register bool) *Metrics {
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

		dnsOperations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "dns_operations_total",
			Help:      "Total DNS operations managed by app",
		}, []string{"operation", "zone", "type"}),

		dnsRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "dns_requests_total",
			Help:      "Total DNS provider requests",
		}, []string{"operation", "zone", "status"}),

		caddyEntries: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "caddy_entries_current",
			Help:      "Current known caddy entries",
		}, []string{"reverse_proxy"}),

		caddyRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "caddy_requests_total",
			Help:      "Total caddy requests",
		}, []string{"status", "code"}),

		badgerRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "badgerdb_requests_total",
			Help:      "Total badgerdb requests",
		}, []string{"operation", "status"}),
	}

	if register {
		registry.MustRegister(
			m.syncRuns,
			m.syncDuration,
			m.dnsOperations,
			m.dnsRequests,
			m.caddyEntries,
			m.caddyRequests,
			m.badgerRequests,
		)
	}
	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
