package insights

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
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
	*apikit.Core
}

// NewEventCatalogHandler wires JetStream discovery + Postgres enrichments.
func NewEventCatalogHandler(svc *app.Services, cfg config.Config, hub *snapshot.Hub) *EventCatalogHandler {
	return &EventCatalogHandler{Core: apikit.NewCore(svc, cfg, hub)}
}

// List godoc
//
// @Summary List
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.EventCatalogEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/event-catalog [get]
func (h *EventCatalogHandler) List(ctx *fasthttp.RequestCtx) {
	clusterID := apikit.ClusterID(ctx)
	fresh := commonstrings.BytesToString(ctx.QueryArgs().Peek("fresh")) == "1"

	c := httpctx.FromRequest(ctx)
	raw, capturedAt, err := h.Svc.Monitoring.FetchJSZ(c, clusterID, fresh)
	if err != nil {
		apikit.WriteJSZFetchError(ctx, err)
		return
	}

	live, err := h.Svc.Monitoring.EventCatalogLiveFromJSZ(raw)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	docs, err := h.Svc.EventCatalog.ListDocs(httpctx.FromRequest(ctx), clusterID)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
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

// Upsert godoc
//
// @Summary Upsert
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Param subject path string true "subject"
// @Produce json
// @Success 200 {object} api.EventCatalogDocEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/event-catalog/{subject} [put]
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
	row, err := h.Svc.EventCatalog.Upsert(c, apikit.ClusterID(ctx), subject, user.ID, req)
	if err != nil {
		if isCatalogValidationError(err) {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
		apikit.WriteAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, row)
}

// Delete godoc
//
// @Summary Delete
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Param subject path string true "subject"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/event-catalog/{subject} [delete]
func (h *EventCatalogHandler) Delete(ctx *fasthttp.RequestCtx) {
	subject, err := catalogSubjectParam(ctx)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	err = h.Svc.EventCatalog.Delete(httpctx.FromRequest(ctx), apikit.ClusterID(ctx), subject)
	if errors.Is(err, domain.ErrEventCatalogEntryNotFound) {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		if isCatalogValidationError(err) {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
		apikit.WriteAPIError(ctx, err)
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
