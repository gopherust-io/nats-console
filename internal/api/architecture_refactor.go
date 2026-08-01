package api

import (
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/assistant"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// ArchitectureRefactorDemo returns a sample coupling-reduction plan (no cluster required).
func (h *Handler) ArchitectureRefactorDemo(ctx *fasthttp.RequestCtx) {
	httpstatus.WriteData(ctx, fasthttp.StatusOK, domain.DemoArchitectureRefactorPlan())
}

// ArchitectureRefactor returns a deterministic coupling-reduction plan from topology jsz.
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

// ArchitectureRefactorAsk uses Gemini to narrate a precomputed refactor plan.
func (h *Handler) ArchitectureRefactorAsk(ctx *fasthttp.RequestCtx) {
	if h.svc == nil || h.svc.Assistant == nil || !h.svc.Assistant.Enabled() {
		writeAssistantError(ctx, assistant.WrapError(assistant.ErrNotEnabled))
		return
	}
	plan, err := h.loadArchitectureRefactor(ctx)
	if err != nil {
		return
	}
	var req architectureRefactorAskRequest
	if len(ctx.PostBody()) > 0 {
		if uerr := serializer.Unmarshal(ctx.PostBody(), &req); uerr != nil {
			writeAssistantError(ctx, assistant.WrapError(uerr))
			return
		}
	}
	reply, aerr := h.svc.Assistant.ArchitectureRefactor(httpctx.FromRequest(ctx), plan, req.Message)
	if aerr != nil {
		writeAssistantError(ctx, assistant.WrapError(aerr))
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
