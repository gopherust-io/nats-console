package insights

import (
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// Zombies godoc
//
// @Summary Zombies
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.ZombieEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/zombies [get]
func (h *Handler) Zombies(ctx *fasthttp.RequestCtx) {
	clusterID := apikit.ClusterID(ctx)
	fresh := strings.BytesToString(ctx.QueryArgs().Peek("fresh")) == "1"

	c := httpctx.FromRequest(ctx)
	raw, capturedAt, err := h.Svc.Monitoring.FetchJSZ(c, clusterID, fresh)
	if err != nil {
		apikit.WriteJSZFetchError(ctx, err)
		return
	}

	streams, err := h.Svc.Monitoring.ZombieStreamsFromJSZ(raw)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	snap := domain.AnalyzeZombies(streams)
	if !capturedAt.IsZero() {
		snap.CapturedAt = capturedAt
	} else {
		snap.CapturedAt = time.Now().UTC()
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, snap)
}
