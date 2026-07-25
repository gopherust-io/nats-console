package metrics

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/gopherust-io/tel"
)

var (
	initOnce sync.Once

	httpRequests   *tel.FastCounter
	httpDuration   *tel.FastHistogram
	wsActive       *tel.FastGauge
	natsActive     *tel.FastGauge
	natsDialErrors *tel.FastCounter
	natsReconnects *tel.FastCounter
	snapSuccess    *tel.FastCounter
	snapErrors     *tel.FastCounter
	wsFramesDrop   *tel.FastCounter
)

func ensure() {
	initOnce.Do(func() {
		r := tel.Global().Registry()
		httpRequests, _ = r.Counter("nats_consol_http_requests_total")
		httpDuration, _ = r.Histogram("nats_consol_http_request_duration_seconds")
		wsActive, _ = r.Gauge("nats_consol_ws_connections_active")
		natsActive, _ = r.Gauge("nats_consol_nats_connections_active")
		natsDialErrors, _ = r.Counter("nats_consol_nats_dial_errors_total")
		natsReconnects, _ = r.Counter("nats_consol_nats_reconnects_total")
		snapSuccess, _ = r.Counter("nats_consol_metrics_snapshot_success_total")
		snapErrors, _ = r.Counter("nats_consol_metrics_snapshot_errors_total")
		wsFramesDrop, _ = r.Counter("nats_consol_live_ws_frames_dropped_total")
	})
}

func ObserveHTTP(method, path string, status int, duration time.Duration) {
	ensure()
	ctx := context.Background()
	subject := method + "|" + path + "|" + strconv.Itoa(status)
	if httpRequests != nil {
		httpRequests.AddWith(ctx, 1, subject)
	}
	if httpDuration != nil {
		httpDuration.RecordWith(ctx, duration.Seconds(), method+"|"+path)
	}
}

var wsCount int64

func IncWS() {
	ensure()
	wsCount++
	if wsActive != nil {
		wsActive.Record(context.Background(), wsCount)
	}
}

func DecWS() {
	ensure()
	if wsCount > 0 {
		wsCount--
	}
	if wsActive != nil {
		wsActive.Record(context.Background(), wsCount)
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
