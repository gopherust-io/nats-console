package jetstream

import (
	"bufio"
	"context"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/domain"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type accountOverviewEvent struct {
	Account      domain.AccountInfo           `json:"account"`
	RequestReply *domain.RequestReplySnapshot `json:"requestReply,omitempty"`
	Varz         *accountVarzLite             `json:"varz,omitempty"`
}

type accountVarzLite struct {
	Connections int   `json:"connections"`
	InMsgs      int64 `json:"in_msgs"`
	InBytes     int64 `json:"in_bytes"`
}

// AccountOverviewEventsSSE godoc
//
// @Summary Account Overview Events SSE
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Produce text/event-stream
// @Success 200 {object} api.AccountInfoEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/account/events [get]
func (h *Handler) AccountOverviewEventsSSE(ctx *fasthttp.RequestCtx) {
	if h.overview == nil {
		ctx.SetStatusCode(fasthttp.StatusServiceUnavailable)
		ctx.SetBodyString("account overview broker unavailable")
		return
	}
	clusterID := apikit.ClusterID(ctx)
	if strings.IsEmpty(clusterID) {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetBodyString("cluster id required")
		return
	}

	updates, latest, unsub := h.overview.Subscribe(clusterID)

	ctx.SetContentType("text/event-stream")
	ctx.Response.Header.Set("Cache-Control", "no-cache")
	ctx.Response.Header.Set("Connection", "keep-alive")
	apikit.RefreshSSEWriteDeadline(ctx)
	ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
		defer unsub()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		if len(latest) > 0 {
			if err := writeAccountOverviewSSE(w, latest); err != nil {
				return
			}
		}

		for {
			select {
			case payload, ok := <-updates:
				if !ok {
					return
				}
				apikit.RefreshSSEWriteDeadline(ctx)
				if err := writeAccountOverviewSSE(w, payload); err != nil {
					return
				}
			case <-ticker.C:
				apikit.RefreshSSEWriteDeadline(ctx)
				if _, err := w.WriteString(": ping\n\n"); err != nil {
					return
				}
				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	})
}

func writeAccountOverviewSSE(w *bufio.Writer, payload []byte) error {
	return apikit.WriteSSEEvent(w, "account-overview", payload)
}

func fetchAccountOverviewJSON(ctx context.Context, svc *app.Services, hub *snapshot.Hub, clusterID string) ([]byte, error) {
	client, err := svc.JetStream.GetExecutor(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	info, err := client.AccountInfo(ctx)
	if err != nil {
		return nil, err
	}

	out := accountOverviewEvent{Account: domain.AccountInfoFromNATS(info)}

	if raw, varzErr := client.Monitoring(ctx, "/varz"); varzErr == nil {
		var lite accountVarzLite
		if err := serializer.Unmarshal(raw, &lite); err == nil {
			out.Varz = &lite
		}
	}

	if connz, connzErr := client.Monitoring(ctx, natsclient.RequestReplyConnzPath); connzErr == nil {
		var probes []domain.RequestReplyProbeResult
		if hub != nil {
			if overlay, _, ok := hub.ProbeResultsOverlay(clusterID); ok {
				probes = overlay
			}
		}
		rr := natsclient.BuildRequestReplySnapshot(connz, probes)
		rr.CapturedAt = time.Now().UTC()
		out.RequestReply = &rr
	}

	return serializer.Marshal(out)
}
