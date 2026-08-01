package api

import (
	"fmt"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

var allowedMonitoringEndpoints = map[string]struct{}{
	"varz":     {},
	"jsz":      {},
	"routez":   {},
	"gatewayz": {},
	"leafz":    {},
	"connz":    {},
	"healthz":  {},
	"subsz":    {},
	"accountz": {},
	"accstatz": {},
}

func validateMonitoringEndpoint(endpoint string) error {
	endpoint = strings.ToLower(strings.TrimSpace(endpoint))
	if _, ok := allowedMonitoringEndpoints[endpoint]; !ok {
		return fmt.Errorf("unsupported monitoring endpoint: %s", endpoint)
	}
	return nil
}

func (h *Handler) Monitoring(ctx *fasthttp.RequestCtx) {
	endpoint := strings.ToLower(httpctx.RouteParam(ctx, "endpoint"))
	if err := validateMonitoringEndpoint(endpoint); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	path := "/" + endpoint
	if query := commonstrings.BytesToString(ctx.URI().QueryString()); !commonstrings.IsEmpty(query) {
		path += "?" + query
	}
	h.natsRaw(ctx, path)
}
