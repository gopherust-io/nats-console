package insights

import (
	"bufio"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// ReplicasEventsSSE godoc
//
// @Summary Replicas Events SSE
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Produce text/event-stream
// @Success 200 {object} api.DataMetaEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/replicas/events [get]
func (h *Handler) ReplicasEventsSSE(ctx *fasthttp.RequestCtx) {
	if h.replicas == nil {
		ctx.SetStatusCode(fasthttp.StatusServiceUnavailable)
		ctx.SetBodyString("replicas broker unavailable")
		return
	}
	clusterID := apikit.ClusterID(ctx)
	if strings.IsEmpty(clusterID) {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetBodyString("cluster id required")
		return
	}

	updates, latest, unsub := h.replicas.Subscribe(clusterID)

	ctx.SetContentType("text/event-stream")
	ctx.Response.Header.Set("Cache-Control", "no-cache")
	ctx.Response.Header.Set("Connection", "keep-alive")
	apikit.RefreshSSEWriteDeadline(ctx)
	ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
		defer unsub()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		if len(latest) > 0 {
			if err := writeReplicasSSE(w, latest); err != nil {
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
				if err := writeReplicasSSE(w, payload); err != nil {
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

func writeReplicasSSE(w *bufio.Writer, payload []byte) error {
	return apikit.WriteSSEEvent(w, "replicas", payload)
}
