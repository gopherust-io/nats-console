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

	WSConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "nats_consol_ws_connections_active",
		Help: "Active WebSocket connections.",
	})

	NATSConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "nats_consol_nats_connections_active",
		Help: "Active cached NATS client connections.",
	})

	NATSDialErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nats_consol_nats_dial_errors_total",
		Help: "Total NATS dial errors by cluster.",
	}, []string{"cluster"})

	NATSReconnectsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nats_consol_nats_reconnects_total",
		Help: "Total NATS client reconnects by cluster.",
	}, []string{"cluster"})
)

func ObserveHTTP(method, path string, status int, duration time.Duration) {
	statusStr := strconv.Itoa(status)
	HTTPRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
	HTTPRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

func IncWS() {
	WSConnectionsActive.Inc()
}

func DecWS() {
	WSConnectionsActive.Dec()
}

func SetNATSConnectionsActive(count int) {
	NATSConnectionsActive.Set(float64(count))
}

func IncNATSDialError(clusterID string) {
	NATSDialErrorsTotal.WithLabelValues(clusterID).Inc()
}

func IncNATSReconnect(clusterID string) {
	NATSReconnectsTotal.WithLabelValues(clusterID).Inc()
}
