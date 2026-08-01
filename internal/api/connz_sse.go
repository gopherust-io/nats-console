package api

import (
	"bufio"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// ConnzEventsSSE pushes live connz payloads while the client stays connected.
// Scraping is demand-driven via ConnzBroker (starts on first subscriber).
func (h *Handler) ConnzEventsSSE(ctx *fasthttp.RequestCtx) {
	if h.connz == nil {
		ctx.SetStatusCode(fasthttp.StatusServiceUnavailable)
		ctx.SetBodyString("connz broker unavailable")
		return
	}
	clusterID := clusterID(ctx)
	if strings.IsEmpty(clusterID) {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetBodyString("cluster id required")
		return
	}

	updates, latest, unsub := h.connz.Subscribe(clusterID)

	ctx.SetContentType("text/event-stream")
	ctx.Response.Header.Set("Cache-Control", "no-cache")
	ctx.Response.Header.Set("Connection", "keep-alive")
	// unsub must run inside the stream writer: SetBodyStreamWriter returns
	// immediately after starting the writer goroutine.
	ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
		defer unsub()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		if len(latest) > 0 {
			if err := writeConnzSSE(w, latest); err != nil {
				return
			}
		}

		for {
			select {
			case payload, ok := <-updates:
				if !ok {
					return
				}
				if err := writeConnzSSE(w, payload); err != nil {
					return
				}
			case <-ticker.C:
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

func writeConnzSSE(w *bufio.Writer, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	if _, err := w.WriteString("event: connz\ndata: "); err != nil {
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
