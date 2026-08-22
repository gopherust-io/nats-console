package insights

import (
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/assistant"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// ArchitectureRefactorDemo godoc
//
// @Summary Architecture Refactor Demo
// @Tags Docs
// @Produce json
// @Success 200 {object} api.ArchitectureRefactorEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Router /api/v1/architecture-refactor/demo [get]
func (h *Handler) ArchitectureRefactorDemo(ctx *fasthttp.RequestCtx) {
	httpstatus.WriteData(ctx, fasthttp.StatusOK, domain.DemoArchitectureRefactorPlan())
}

// ArchitectureRefactor godoc
//
// @Summary Architecture Refactor
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.ArchitectureRefactorEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/architecture-refactor [get]
func (h *Handler) ArchitectureRefactor(ctx *fasthttp.RequestCtx) {
	plan, err := h.loadArchitectureRefactor(ctx)
	if err != nil {
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, plan)
}

type architectureRefactorAskRequest struct {
	Message string `json:"message"`
}

type architectureRefactorAskResponse struct {
	Reply string                          `json:"reply"`
	Plan  domain.ArchitectureRefactorPlan `json:"plan"`
}

// ArchitectureRefactorAsk godoc
//
// @Summary Architecture Refactor Ask
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
// @Router /api/v1/clusters/{clusterId}/architecture-refactor/ask [post]
func (h *Handler) ArchitectureRefactorAsk(ctx *fasthttp.RequestCtx) {
	if h.Svc == nil || h.Svc.Assistant == nil || !h.Svc.Assistant.Enabled() {
		apikit.WriteAssistantError(ctx, assistant.WrapError(assistant.ErrNotEnabled))
		return
	}
	plan, err := h.loadArchitectureRefactor(ctx)
	if err != nil {
		return
	}
	var req architectureRefactorAskRequest
	if len(ctx.PostBody()) > 0 {
		if uerr := serializer.Unmarshal(ctx.PostBody(), &req); uerr != nil {
			apikit.WriteAssistantError(ctx, assistant.WrapError(uerr))
			return
		}
	}
	reply, aerr := h.Svc.Assistant.ArchitectureRefactor(httpctx.FromRequest(ctx), plan, req.Message)
	if aerr != nil {
		apikit.WriteAssistantError(ctx, assistant.WrapError(aerr))
		return
	}
	plan.Narrative = reply
	httpstatus.WriteData(ctx, fasthttp.StatusOK, architectureRefactorAskResponse{Reply: reply, Plan: plan})
}

func (h *Handler) loadArchitectureRefactor(ctx *fasthttp.RequestCtx) (domain.ArchitectureRefactorPlan, error) {
	demo := strings.BytesToString(ctx.QueryArgs().Peek("demo")) == "1"
	seed := domain.ArchitectureRefactorSeed{
		Kind:    strings.BytesToString(ctx.QueryArgs().Peek("kind")),
		Stream:  strings.BytesToString(ctx.QueryArgs().Peek("stream")),
		Subject: strings.BytesToString(ctx.QueryArgs().Peek("subject")),
	}
	if demo {
		plan := domain.DemoArchitectureRefactorPlan()
		if plan.Seed == nil && (!strings.IsEmpty(seed.Kind) || !strings.IsEmpty(seed.Stream) || !strings.IsEmpty(seed.Subject)) {
			plan.Seed = &seed
		}
		return plan, nil
	}

	inv, err := h.loadArchitectureInventory(ctx)
	if err != nil {
		return domain.ArchitectureRefactorPlan{}, err
	}
	return domain.BuildArchitectureRefactorPlan(inv, seed), nil
}
