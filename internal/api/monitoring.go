package api

import (
	"fmt"
	"strings"

	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/valyala/fasthttp"
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
	endpoint := strings.ToLower(routeParam(ctx, "endpoint"))
	if err := validateMonitoringEndpoint(endpoint); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	path := "/" + endpoint
	if query := string(ctx.URI().QueryString()); query != "" {
		path += "?" + query
	}
	h.natsRaw(ctx, path)
}
