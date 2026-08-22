package insights

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const maxMonitoringQueryLimit = 1024

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

// Per-endpoint query allowlist. Unknown keys are dropped; limit is clamped.
var monitoringQueryAllow = map[string]map[string]struct{}{
	"varz":     {},
	"routez":   {},
	"gatewayz": {},
	"leafz":    {},
	"healthz":  {"js-enabled-only": {}, "js-server-only": {}},
	"accountz": {"acc": {}},
	"accstatz": {"acc": {}},
	"subsz":    {"subs": {}, "offset": {}, "limit": {}},
	"connz": {
		"limit": {}, "offset": {}, "auth": {}, "subs": {}, "state": {},
		"cid": {}, "name": {}, "account": {},
	},
	"jsz": {
		"streams": {}, "consumers": {}, "config": {}, "accounts": {},
		"account": {}, "offset": {}, "limit": {},
	},
}

func validateMonitoringEndpoint(endpoint string) error {
	endpoint = strings.ToLower(strings.TrimSpace(endpoint))
	if _, ok := allowedMonitoringEndpoints[endpoint]; !ok {
		return fmt.Errorf("unsupported monitoring endpoint: %s", endpoint)
	}
	return nil
}

func sanitizeMonitoringQuery(endpoint string, args *fasthttp.Args) string {
	allowed, ok := monitoringQueryAllow[endpoint]
	if !ok || args == nil || args.Len() == 0 {
		return ""
	}
	values := url.Values{}
	for k, v := range args.All() {
		key := strings.ToLower(commonstrings.BytesToString(k))
		if _, ok := allowed[key]; !ok {
			continue
		}
		val := commonstrings.BytesToString(v)
		if key == "limit" {
			n, err := strconv.Atoi(val)
			if err != nil || n <= 0 {
				n = maxMonitoringQueryLimit
			}
			if n > maxMonitoringQueryLimit {
				n = maxMonitoringQueryLimit
			}
			values.Set("limit", strconv.Itoa(n))
			continue
		}
		values.Add(key, val)
	}
	return values.Encode()
}

// Monitoring godoc
//
// @Summary Monitoring
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Param endpoint path string true "endpoint"
// @Produce json
// @Success 200 {object} api.DataMetaEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/monitoring/{endpoint} [get]
func (h *Handler) Monitoring(ctx *fasthttp.RequestCtx) {
	endpoint := strings.ToLower(httpctx.RouteParam(ctx, "endpoint"))
	if err := validateMonitoringEndpoint(endpoint); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	path := "/" + endpoint
	if query := sanitizeMonitoringQuery(endpoint, ctx.URI().QueryArgs()); !commonstrings.IsEmpty(query) {
		path += "?" + query
	}
	h.Raw(ctx, path)
}

// Varz godoc
//
// @Summary Varz
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.DataMetaEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/monitoring/varz [get]
func (h *Handler) Varz(ctx *fasthttp.RequestCtx) {
	h.Raw(ctx, "/varz")
}

// Jsz godoc
//
// @Summary Jsz
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.DataMetaEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/monitoring/jsz [get]
func (h *Handler) Jsz(ctx *fasthttp.RequestCtx) {
	path := "/jsz"
	if query := sanitizeMonitoringQuery("jsz", ctx.URI().QueryArgs()); !commonstrings.IsEmpty(query) {
		path += "?" + query
	}
	h.Raw(ctx, path)
}
