package api

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// EventCatalogHandler serves the hybrid event catalog API.
// List reads JetStream monitoring (jsz) for discovery only. Upsert/Delete
// persist documentation in Postgres and do not call JetStream APIs.
type EventCatalogHandler struct {
	svc                   *app.Services
	hub                   *snapshot.Hub
	cfgMaxMonitoringBytes int64
}

// NewEventCatalogHandler wires JetStream discovery + Postgres enrichments.
func NewEventCatalogHandler(svc *app.Services, hub *snapshot.Hub, maxMonitoringBytes int64) *EventCatalogHandler {
	return &EventCatalogHandler{svc: svc, hub: hub, cfgMaxMonitoringBytes: maxMonitoringBytes}
}

// List returns the merged live + documented event catalog.
func (h *EventCatalogHandler) List(ctx *fasthttp.RequestCtx) {
	clusterID := clusterID(ctx)
	fresh := commonstrings.BytesToString(ctx.QueryArgs().Peek("fresh")) == "1"

	var raw []byte
	var capturedAt time.Time
	if !fresh && h.hub != nil {
		if data, at, ok := h.hub.MonitoringPayload(clusterID, snapshot.TopologyJSZPath); ok {
			raw = data
			capturedAt = at
		}
	}
	if raw == nil {
		c := httpctx.FromRequest(ctx)
		client, err := h.svc.JetStream.GetExecutor(c, clusterID)
		if err != nil {
			writeAPIError(ctx, err)
			return
		}
		data, err := client.Monitoring(c, snapshot.TopologyJSZPath)
		if err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
			return
		}
		raw = data
		capturedAt = time.Now().UTC()
		if h.cfgMaxMonitoringBytes > 0 && int64(len(raw)) > h.cfgMaxMonitoringBytes {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, errMonitoringTooLarge)
			return
		}
	}

	projected := projectJSZForTopology(raw)
	live, err := eventCatalogLiveFromJSZ(projected)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	docs, err := h.svc.EventCatalog.ListDocs(httpctx.FromRequest(ctx), clusterID)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}

	snap := domain.BuildEventCatalog(live, docs)
	if !capturedAt.IsZero() {
		snap.CapturedAt = capturedAt
	} else {
		snap.CapturedAt = time.Now().UTC()
	}
	if snap.Entries == nil {
		snap.Entries = []domain.EventCatalogEntry{}
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, snap)
}

// Upsert creates or updates catalog enrichment for a subject.
func (h *EventCatalogHandler) Upsert(ctx *fasthttp.RequestCtx) {
	subject, err := catalogSubjectParam(ctx)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	var req domain.EventCatalogUpsert
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := domain.ValidateEventCatalogSchema(req.Schema); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := domain.ValidateEventCatalogExample(req.Example); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	c := httpctx.FromRequest(ctx)
	user, _ := auth.UserFromContext(c)
	row, err := h.svc.EventCatalog.Upsert(c, clusterID(ctx), subject, user.ID, req)
	if err != nil {
		if isCatalogValidationError(err) {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, row)
}

// Delete removes catalog enrichment for a subject (live inventory remains).
func (h *EventCatalogHandler) Delete(ctx *fasthttp.RequestCtx) {
	subject, err := catalogSubjectParam(ctx)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	err = h.svc.EventCatalog.Delete(httpctx.FromRequest(ctx), clusterID(ctx), subject)
	if errors.Is(err, domain.ErrEventCatalogEntryNotFound) {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		if isCatalogValidationError(err) {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
		writeAPIError(ctx, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func catalogSubjectParam(ctx *fasthttp.RequestCtx) (string, error) {
	raw := httpctx.RouteParam(ctx, "subject")
	if commonstrings.IsEmpty(raw) {
		return "", errors.New("subject required")
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", errors.New("invalid subject encoding")
	}
	return domain.CanonicalEventCatalogSubject(decoded)
}

func isCatalogValidationError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "subject") ||
		strings.Contains(msg, "schema") ||
		strings.Contains(msg, "example") ||
		strings.Contains(msg, "wildcard") ||
		strings.Contains(msg, "whitespace")
}

func eventCatalogLiveFromJSZ(raw []byte) ([]domain.EventCatalogLiveStream, error) {
	var payload jszTopologyPayload
	if err := serializer.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	out := make([]domain.EventCatalogLiveStream, 0)
	for _, acct := range payload.AccountDetails {
		for _, stream := range acct.StreamDetail {
			in := domain.EventCatalogLiveStream{Name: stream.Name}
			if stream.Config != nil {
				in.Subjects = append([]string(nil), stream.Config.Subjects...)
			}
			for _, c := range stream.ConsumerDetail {
				cin := domain.EventCatalogLiveConsumer{Name: c.Name}
				if c.Config != nil {
					cin.FilterSubject = c.Config.FilterSubject
					cin.FilterSubjects = append([]string(nil), c.Config.FilterSubjects...)
					cin.DurableName = c.Config.DurableName
					if len(c.Config.Metadata) > 0 {
						cin.Metadata = cloneStringMap(c.Config.Metadata)
					}
				}
				in.Consumers = append(in.Consumers, cin)
			}
			out = append(out, in)
		}
	}
	return out, nil
}
