package api

import (
	"context"
	"fmt"
	"strings"

	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/valyala/fasthttp"
)

var allowedMonitoringEndpoints = map[string]struct{}{
	"varz":      {},
	"jsz":       {},
	"routez":    {},
	"gatewayz":  {},
	"leafz":     {},
	"connz":     {},
	"healthz":   {},
	"subsz":     {},
	"accountz":  {},
	"accstatz":  {},
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

func (h *Handler) Supercluster(ctx *fasthttp.RequestCtx) {
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		overview, err := natsclient.BuildSuperclusterOverview(c, client)
		if err != nil {
			return nil, fasthttp.StatusBadGateway, err
		}
		return overview, fasthttp.StatusOK, nil
	})
}
