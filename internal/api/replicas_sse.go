package api

import (
	"bufio"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// ReplicasEventsSSE pushes projected replica snapshots while the Replicas page is open.
// Scraping is demand-driven via the replicas ConnzBroker (starts on first subscriber).
func (h *Handler) ReplicasEventsSSE(ctx *fasthttp.RequestCtx) {
	if h.replicas == nil {
		ctx.SetStatusCode(fasthttp.StatusServiceUnavailable)
		ctx.SetBodyString("replicas broker unavailable")
		return
	}
	clusterID := clusterID(ctx)
	if strings.IsEmpty(clusterID) {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetBodyString("cluster id required")
		return
	}

	updates, latest, unsub := h.replicas.Subscribe(clusterID)

	ctx.SetContentType("text/event-stream")
	ctx.Response.Header.Set("Cache-Control", "no-cache")
	ctx.Response.Header.Set("Connection", "keep-alive")
	refreshSSEWriteDeadline(ctx)
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
				refreshSSEWriteDeadline(ctx)
				if err := writeReplicasSSE(w, payload); err != nil {
					return
				}
			case <-ticker.C:
				refreshSSEWriteDeadline(ctx)
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
	if len(payload) == 0 {
		return nil
	}
	if _, err := w.WriteString("event: replicas\ndata: "); err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	if _, err := w.WriteString("\n\n"); err != nil {
		return err
	}
	return w.Flush()
}
