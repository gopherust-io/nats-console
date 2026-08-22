// Package jetstream serves the JetStream bounded context: streams, consumers,
// messages, DLQ retries, incident capsules, replay, and the account overview.
package jetstream

import (
	"context"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
)

type Handler struct {
	*apikit.Core

	overview *snapshot.ConnzBroker
}

func NewHandler(svc *app.Services, cfg config.Config, hub *snapshot.Hub) *Handler {
	h := &Handler{Core: apikit.NewCore(svc, cfg, hub)}
	if svc != nil && svc.JetStream != nil {
		h.overview = snapshot.NewConnzBroker(func(ctx context.Context, clusterID string) ([]byte, error) {
			return fetchAccountOverviewJSON(ctx, svc, hub, clusterID)
		}, snapshot.DefaultAccountOverviewInterval)
	}
	return h
}

// AccountInfo godoc
//
// @Summary Account Info
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.AccountInfoEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/account [get]
func (h *Handler) AccountInfo(ctx *fasthttp.RequestCtx) {
	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.AccountInfo(c)
		if err != nil {
			return nil, 0, err
		}
		return domain.AccountInfoFromNATS(info), fasthttp.StatusOK, nil
	})
}

func (h *Handler) slowConsumerThresholds() domain.SlowConsumerThresholds {
	return domain.SlowConsumerThresholds{
		PendingThreshold: h.Cfg.SlowConsumer.PendingThreshold,
		LagThreshold:     h.Cfg.SlowConsumer.LagThreshold,
		AckPendingRatio:  h.Cfg.SlowConsumer.AckPendingRatio,
	}
}
