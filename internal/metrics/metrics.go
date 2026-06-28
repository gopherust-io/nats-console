package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nats_consol_http_requests_total",
		Help: "Total HTTP requests processed.",
	}, []string{"method", "path", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "nats_consol_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	NATSOperationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nats_consol_nats_operations_total",
		Help: "Total NATS operations.",
	}, []string{"cluster", "operation", "result"})

	WSConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "nats_consol_ws_connections_active",
		Help: "Active WebSocket connections.",
	})
)

func ObserveHTTP(method, path string, status int, duration time.Duration) {
	statusStr := strconv.Itoa(status)
	HTTPRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
	HTTPRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

func ObserveNATS(cluster, operation, result string) {
	NATSOperationsTotal.WithLabelValues(cluster, operation, result).Inc()
}

func IncWS() {
	WSConnectionsActive.Inc()
}

func DecWS() {
	WSConnectionsActive.Dec()
}
