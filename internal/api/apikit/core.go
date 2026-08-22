// Package apikit holds the plumbing every bounded-context HTTP package in
// internal/api shares: the JetStream execution Core, error writers, pagination
// and validation helpers, and the request-scoped accessors handlers need.
package apikit

import (
	"context"
	"errors"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/metrics"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const TimeRFC3339 = "2006-01-02T15:04:05Z07:00"

var ErrMonitoringTooLarge = errors.New("monitoring payload too large")

// Core carries the services, snapshot hub, and config every cluster-scoped
// handler package needs, plus the three JetStream request shapes they build on.
type Core struct {
	Svc *app.Services
	Hub *snapshot.Hub
	Cfg config.Config
}

func NewCore(svc *app.Services, cfg config.Config, hub *snapshot.Hub) *Core {
	return &Core{Svc: svc, Cfg: cfg, Hub: hub}
}

// ClusterID returns the lowercased {clusterId} route parameter.
func ClusterID(ctx *fasthttp.RequestCtx) string {
	return strings.ToLower(httpctx.RouteParam(ctx, "clusterId"))
}

// ActorFromContext returns the authenticated principal for the request.
func ActorFromContext(ctx *fasthttp.RequestCtx) (domain.User, bool) {
	return auth.UserFromContext(httpctx.FromRequest(ctx))
}

// Action runs fn against the cluster executor and writes the returned payload.
// A DataMeta result is emitted as data+meta; anything else as a bare data body.
func (c *Core) Action(ctx *fasthttp.RequestCtx, fn func(context.Context, port.JetStreamExecutor) (any, int, error)) {
	rc := httpctx.FromRequest(ctx)
	var (
		result any
		status int
		etag   string
	)
	err := c.Svc.JetStream.WithExecutor(rc, ClusterID(ctx), func(client port.JetStreamExecutor) error {
		var actionErr error
		result, status, actionErr = fn(rc, client)
		if tagged, ok := client.(interface{ LastETag() string }); ok {
			etag = tagged.LastETag()
		}
		return actionErr
	})
	if err != nil {
		status = MapNATSErrorStatus(err, status)
		WriteNATSError(ctx, status, err)
		return
	}
	if status == 0 {
		status = fasthttp.StatusOK
	}
	if !commonstrings.IsEmpty(etag) && httpstatus.CheckIfNoneMatch(ctx, etag) {
		ctx.SetStatusCode(fasthttp.StatusNotModified)
		return
	}
	if dm, ok := result.(DataMeta); ok {
		httpstatus.WriteDataMetaWithETag(ctx, status, dm.Data, dm.Meta, etag)
		return
	}
	httpstatus.WriteDataWithETag(ctx, status, result, etag)
}

// Void runs fn against the cluster executor and answers 204 on success.
func (c *Core) Void(ctx *fasthttp.RequestCtx, fn func(context.Context, port.JetStreamExecutor) error, badStatus int) {
	rc := httpctx.FromRequest(ctx)
	err := c.Svc.JetStream.WithExecutor(rc, ClusterID(ctx), func(client port.JetStreamExecutor) error {
		return fn(rc, client)
	})
	if err != nil {
		WriteNATSError(ctx, MapNATSErrorStatus(err, badStatus), err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

// Raw proxies a NATS monitoring endpoint, preferring the snapshot hub cache
// unless the caller asked for ?fresh=1.
func (c *Core) Raw(ctx *fasthttp.RequestCtx, path string) {
	rc := httpctx.FromRequest(ctx)
	cluster := ClusterID(ctx)
	fresh := commonstrings.BytesToString(ctx.QueryArgs().Peek("fresh")) == "1"

	if !fresh && c.Hub != nil {
		if data, capturedAt, ok := c.Hub.MonitoringPayload(cluster, path); ok {
			if int64(len(data)) > c.Cfg.MaxMonitoringBodyBytes {
				httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, ErrMonitoringTooLarge)
				return
			}
			etag := `"` + capturedAt.UTC().Format("20060102T150405") + `"`
			if httpstatus.CheckIfNoneMatch(ctx, etag) {
				ctx.SetStatusCode(fasthttp.StatusNotModified)
				return
			}
			ctx.Response.Header.Set("X-Snapshot-Age", capturedAt.UTC().Format(TimeRFC3339))
			httpstatus.WriteRawJSONWithETag(ctx, data, etag)
			return
		}
		metrics.IncSnapshotHubMiss(path)
	}

	client, err := c.Svc.JetStream.GetExecutor(rc, cluster)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			WriteAPIError(ctx, err)
			return
		}
		WriteNATSError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	data, err := client.Monitoring(rc, path)
	if err != nil {
		WriteNATSError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	if int64(len(data)) > c.Cfg.MaxMonitoringBodyBytes {
		WriteNATSError(ctx, fasthttp.StatusBadGateway, ErrMonitoringTooLarge)
		return
	}
	etag := ""
	if tagged, ok := client.(interface{ LastETag() string }); ok {
		etag = tagged.LastETag()
	}
	if !commonstrings.IsEmpty(etag) && httpstatus.CheckIfNoneMatch(ctx, etag) {
		ctx.SetStatusCode(fasthttp.StatusNotModified)
		return
	}
	httpstatus.WriteRawJSONWithETag(ctx, data, etag)
}

// InvalidateSnapshot drops the cached cluster snapshot after a mutation so the
// next topology read reflects the new stream/consumer set.
func (c *Core) InvalidateSnapshot(ctx *fasthttp.RequestCtx) {
	if c.Hub == nil {
		return
	}
	c.Hub.Invalidate(ClusterID(ctx))
}

func MapNATSErrorStatus(err error, requested int) int {
	if err == nil {
		return fasthttp.StatusOK
	}
	if errors.Is(err, domain.ErrNotFound) || IsNATSNotFound(err) {
		return fasthttp.StatusNotFound
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return fasthttp.StatusGatewayTimeout
	}
	if requested == fasthttp.StatusNotFound || requested == 0 {
		return fasthttp.StatusBadGateway
	}
	return requested
}

func IsNATSNotFound(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "no stream") ||
		strings.Contains(msg, "no consumers") ||
		strings.Contains(msg, "no keys found") ||
		strings.Contains(msg, "bucket not found")
}

type missingFieldError string

func (e missingFieldError) String() string {
	return string(e)
}

func (e missingFieldError) Error() string {
	return "missing required field: " + e.String()
}

// ErrMissing reports an absent required request field.
func ErrMissing(field string) error {
	return missingFieldError(field)
}
