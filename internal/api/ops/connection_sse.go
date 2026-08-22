package ops

import (
	"bufio"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// ConnectionEventsSSE godoc
//
// @Summary Connection Events SSE
// @Tags API
// @Param clusterId path string true "clusterId"
// @Produce text/event-stream
// @Success 200 {object} api.ConnectionStatusEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/connection/events [get]
func (h *Handler) ConnectionEventsSSE(ctx *fasthttp.RequestCtx) {
	if h.Svc == nil || h.Svc.JetStream == nil {
		ctx.SetStatusCode(fasthttp.StatusServiceUnavailable)
		ctx.SetBodyString("connection status unavailable")
		return
	}
	clusterID := apikit.ClusterID(ctx)
	if strings.IsEmpty(clusterID) {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetBodyString("cluster id required")
		return
	}

	updates, latest, unsub := h.Svc.JetStream.SubscribeConnectionStatus(clusterID)

	ctx.SetContentType("text/event-stream")
	ctx.Response.Header.Set("Cache-Control", "no-cache")
	ctx.Response.Header.Set("Connection", "keep-alive")
	apikit.RefreshSSEWriteDeadline(ctx)
	ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
		defer unsub()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		if err := writeConnectionSSE(w, latest); err != nil {
			return
		}

		for {
			select {
			case status, ok := <-updates:
				if !ok {
					return
				}
				apikit.RefreshSSEWriteDeadline(ctx)
				if err := writeConnectionSSE(w, status); err != nil {
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

func writeConnectionSSE(w *bufio.Writer, status domain.NATSConnectionStatus) error {
	payload, err := serializer.Marshal(status)
	if err != nil {
		return err
	}
	return apikit.WriteSSEEvent(w, "connection", payload)
}
