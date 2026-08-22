package insights

import (
	"fmt"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// ArchitectureExportDemo godoc
//
// @Summary Architecture Export Demo
// @Tags Docs
// @Produce json
// @Success 200 {object} api.ArchitectureInventoryEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Router /api/v1/architecture-export/demo [get]
func (h *Handler) ArchitectureExportDemo(ctx *fasthttp.RequestCtx) {
	inv := domain.DemoArchitectureInventory()
	bundle := domain.GenerateArchitectureExport(inv, nil)
	raw, err := domain.ZipArchitectureExport(bundle)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	ctx.Response.Header.Set("Content-Type", "application/zip")
	ctx.Response.Header.Set("Content-Disposition", "attachment; filename=nats-consol-architecture-demo.zip")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBody(raw)
}

// ArchitectureExport godoc
//
// @Summary Architecture Export
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.ArchitectureInventoryEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/architecture-export [get]
// @Router /api/v1/clusters/{clusterId}/architecture-export [post]
func (h *Handler) ArchitectureExport(ctx *fasthttp.RequestCtx) {
	demo := strings.BytesToString(ctx.QueryArgs().Peek("demo")) == "1"
	wantAI := strings.BytesToString(ctx.QueryArgs().Peek("ai")) == "1"

	var inv domain.ArchitectureInventory
	if demo {
		inv = domain.DemoArchitectureInventory()
	} else {
		loaded, err := h.loadArchitectureInventory(ctx)
		if err != nil {
			return
		}
		inv = loaded
	}

	draftBundle := domain.GenerateArchitectureExport(inv, nil)
	adrDrafts := map[string]string{}
	for _, f := range draftBundle.Files {
		if f.Path == "adr/0001-jetstream-topology.md" || f.Path == "adr/0002-subject-boundaries.md" {
			adrDrafts[f.Path] = f.Content
		}
	}

	overrides := map[string]string{}
	if wantAI && h.Svc != nil && h.Svc.Assistant != nil && h.Svc.Assistant.Enabled() {
		polished, err := h.Svc.Assistant.PolishArchitectureADRs(httpctx.FromRequest(ctx), inv, adrDrafts)
		if err == nil {
			overrides = polished
		}
		// On AI failure, still return deterministic zip.
	}

	bundle := domain.GenerateArchitectureExport(inv, overrides)
	raw, err := domain.ZipArchitectureExport(bundle)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}

	name := inv.ClusterName
	if demo {
		name = "demo"
	}
	filename := fmt.Sprintf("nats-consol-architecture-%s.zip", sanitizeExportFilename(name))
	ctx.Response.Header.Set("Content-Type", "application/zip")
	ctx.Response.Header.Set("Content-Disposition", "attachment; filename="+filename)
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBody(raw)
}

func (h *Handler) loadArchitectureInventory(ctx *fasthttp.RequestCtx) (domain.ArchitectureInventory, error) {
	clusterID := apikit.ClusterID(ctx)
	fresh := strings.BytesToString(ctx.QueryArgs().Peek("fresh")) == "1"

	var raw []byte
	var capturedAt time.Time
	if !fresh && h.Hub != nil {
		if data, at, ok := h.Hub.MonitoringPayload(clusterID, snapshot.TopologyJSZPath); ok {
			raw = data
			capturedAt = at
		}
	}
	if raw == nil {
		c := httpctx.FromRequest(ctx)
		client, err := h.Svc.JetStream.GetExecutor(c, clusterID)
		if err != nil {
			apikit.WriteAPIError(ctx, err)
			return domain.ArchitectureInventory{}, err
		}
		data, err := client.Monitoring(c, snapshot.TopologyJSZPath)
		if err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
			return domain.ArchitectureInventory{}, err
		}
		raw = data
		capturedAt = time.Now().UTC()
		if int64(len(raw)) > h.Cfg.MaxMonitoringBodyBytes {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, apikit.ErrMonitoringTooLarge)
			return domain.ArchitectureInventory{}, apikit.ErrMonitoringTooLarge
		}
	}

	projected := projectJSZForTopology(raw)
	inputs, err := architectureReviewInputsFromJSZ(projected)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return domain.ArchitectureInventory{}, err
	}
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}
	return domain.BuildArchitectureInventory(clusterID, capturedAt, inputs), nil
}

func sanitizeExportFilename(name string) string {
	if strings.IsEmpty(name) {
		return "cluster"
	}
	b := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b = append(b, r)
		default:
			b = append(b, '_')
		}
	}
	out := string(b)
	if strings.IsEmpty(out) {
		return "cluster"
	}
	return out
}
