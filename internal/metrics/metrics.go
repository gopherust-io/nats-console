package metrics

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/gopherust-io/tel"
)

var (
	initOnce sync.Once

	httpRequests         *tel.FastCounter
	httpDuration         *tel.FastHistogram
	wsActive             *tel.FastGauge
	natsActive           *tel.FastGauge
	natsDialErrors       *tel.FastCounter
	natsReconnects       *tel.FastCounter
	natsDialLatency      *tel.FastHistogram
	natsExecutorErrors   *tel.FastCounter
	natsMonitoringErrors *tel.FastCounter
	snapSuccess          *tel.FastCounter
	snapErrors           *tel.FastCounter
	wsFramesDrop         *tel.FastCounter
	snapHubHit           *tel.FastCounter
	snapHubMiss          *tel.FastCounter
	viewCacheHit         *tel.FastCounter
	viewCacheMiss        *tel.FastCounter
	liveMuxFanout        *tel.FastCounter
)

// Common HTTP status codes as static strings to avoid strconv on the hot path.
var httpStatusStrings = [600]string{
	200: "200",
	201: "201",
	204: "204",
	301: "301",
	302: "302",
	304: "304",
	400: "400",
	401: "401",
	403: "403",
	404: "404",
	409: "409",
	429: "429",
	500: "500",
	502: "502",
	503: "503",
}

func ensure() {
	initOnce.Do(func() {
		r := tel.Global().Registry()
		httpRequests, _ = r.Counter("nats_consol_http_requests_total")
		httpDuration, _ = r.Histogram("nats_consol_http_request_duration_seconds")
		wsActive, _ = r.Gauge("nats_consol_ws_connections_active")
		natsActive, _ = r.Gauge("nats_consol_nats_connections_active")
		natsDialErrors, _ = r.Counter("nats_consol_nats_dial_errors_total")
		natsReconnects, _ = r.Counter("nats_consol_nats_reconnects_total")
		natsDialLatency, _ = r.Histogram("nats_consol_nats_dial_duration_seconds")
		natsExecutorErrors, _ = r.Counter("nats_consol_nats_executor_errors_total")
		natsMonitoringErrors, _ = r.Counter("nats_consol_nats_monitoring_proxy_errors_total")
		snapSuccess, _ = r.Counter("nats_consol_metrics_snapshot_success_total")
		snapErrors, _ = r.Counter("nats_consol_metrics_snapshot_errors_total")
		wsFramesDrop, _ = r.Counter("nats_consol_live_ws_frames_dropped_total")
		snapHubHit, _ = r.Counter("nats_consol_snapshot_hub_hit_total")
		snapHubMiss, _ = r.Counter("nats_consol_snapshot_hub_miss_total")
		viewCacheHit, _ = r.Counter("nats_consol_view_cache_hit_total")
		viewCacheMiss, _ = r.Counter("nats_consol_view_cache_miss_total")
		liveMuxFanout, _ = r.Counter("nats_consol_live_mux_fanout_total")
	})
}

func ObserveHTTP(method, path string, status int, duration time.Duration) {
	ensure()
	ctx := context.Background()
	statusLabel := statusLabel(status)
	if httpRequests != nil {
		httpRequests.AddWith(ctx, 1, joinSubject(method, path, statusLabel))
	}
	if httpDuration != nil {
		httpDuration.RecordWith(ctx, duration.Seconds(), joinSubject(method, path))
	}
}

func statusLabel(status int) string {
	if status >= 0 && status < len(httpStatusStrings) {
		if s := httpStatusStrings[status]; !commonstrings.IsEmpty(s) {
			return s
		}
	}
	return strconv.Itoa(status)
}

func joinSubject(parts ...string) string {
	n := len(parts) - 1
	for _, p := range parts {
		n += len(p)
	}
	var b strings.Builder
	b.Grow(n)
	for i, p := range parts {
		if i > 0 {
			b.WriteByte('|')
		}
		b.WriteString(p)
	}
	return b.String()
}

var wsCount atomic.Int64

func IncWS() {
	ensure()
	n := wsCount.Add(1)
	if wsActive != nil {
		wsActive.Record(context.Background(), n)
	}
}

func DecWS() {
	ensure()
	for {
		cur := wsCount.Load()
		if cur <= 0 {
			if wsActive != nil {
				wsActive.Record(context.Background(), 0)
			}
			return
		}
		if wsCount.CompareAndSwap(cur, cur-1) {
			if wsActive != nil {
				wsActive.Record(context.Background(), cur-1)
			}
			return
		}
	}
}

func SetNATSConnectionsActive(count int) {
	ensure()
	if natsActive != nil {
		natsActive.Record(context.Background(), int64(count))
	}
}

func IncNATSDialError(clusterID string) {
	ensure()
	if natsDialErrors != nil {
		natsDialErrors.AddWith(context.Background(), 1, clusterID)
	}
}

func IncNATSReconnect(clusterID string) {
	ensure()
	if natsReconnects != nil {
		natsReconnects.AddWith(context.Background(), 1, clusterID)
	}
}

func ObserveNATSDialLatency(clusterID string, duration time.Duration) {
	ensure()
	if natsDialLatency != nil && duration >= 0 {
		natsDialLatency.RecordWith(context.Background(), duration.Seconds(), clusterID)
	}
}

func IncNATSExecutorError(clusterID string) {
	ensure()
	if natsExecutorErrors != nil {
		natsExecutorErrors.AddWith(context.Background(), 1, clusterID)
	}
}

func IncNATSMonitoringProxyError(clusterID string) {
	ensure()
	if natsMonitoringErrors != nil {
		natsMonitoringErrors.AddWith(context.Background(), 1, clusterID)
	}
}

func IncSnapshotSuccess(clusterID string) {
	ensure()
	if snapSuccess != nil {
		snapSuccess.AddWith(context.Background(), 1, clusterID)
	}
}

func IncSnapshotErrors(clusterID string) {
	ensure()
	if snapErrors != nil {
		snapErrors.AddWith(context.Background(), 1, clusterID)
	}
}

func IncLiveWSFramesDropped() {
	ensure()
	if wsFramesDrop != nil {
		wsFramesDrop.Add(context.Background(), 1)
	}
}

func IncSnapshotHubHit(kind string) {
	ensure()
	if snapHubHit != nil {
		snapHubHit.AddWith(context.Background(), 1, kind)
	}
}

func IncSnapshotHubMiss(path string) {
	ensure()
	if snapHubMiss != nil {
		snapHubMiss.AddWith(context.Background(), 1, path)
	}
}

func IncViewCacheHit() {
	ensure()
	if viewCacheHit != nil {
		viewCacheHit.Add(context.Background(), 1)
	}
}

func IncViewCacheMiss() {
	ensure()
	if viewCacheMiss != nil {
		viewCacheMiss.Add(context.Background(), 1)
	}
}

func IncLiveMuxFanout(n int) {
	ensure()
	if liveMuxFanout != nil && n > 0 {
		liveMuxFanout.Add(context.Background(), int64(n))
	}
}
