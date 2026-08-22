package insights

import (
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/assistant"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type chaosStoryGenerateRequest struct {
	Hint string `json:"hint"`
}

type chaosStoryGenerateResponse struct {
	Story domain.ChaosStory     `json:"story"`
	Seed  domain.ChaosStorySeed `json:"seed"`
}

type chaosStoryGetResponse struct {
	Story *domain.ChaosStory    `json:"story,omitempty"`
	Seed  domain.ChaosStorySeed `json:"seed"`
}

// ChaosStoryDemo godoc
//
// @Summary Chaos Story Demo
// @Tags Docs
// @Produce json
// @Success 200 {object} api.ChaosStoryEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Router /api/v1/chaos-story/demo [get]
func (h *Handler) ChaosStoryDemo(ctx *fasthttp.RequestCtx) {
	story := domain.DemoChaosStory()
	httpstatus.WriteData(ctx, fasthttp.StatusOK, chaosStoryGetResponse{
		Story: &story,
		Seed:  domain.DemoChaosStorySeed(),
	})
}

// ChaosStory godoc
//
// @Summary Chaos Story
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.ChaosStorySeedEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/chaos-story [get]
func (h *Handler) ChaosStory(ctx *fasthttp.RequestCtx) {
	demo := commonstrings.BytesToString(ctx.QueryArgs().Peek("demo")) == "1"
	if demo {
		story := domain.DemoChaosStory()
		httpstatus.WriteData(ctx, fasthttp.StatusOK, chaosStoryGetResponse{
			Story: &story,
			Seed:  domain.DemoChaosStorySeed(),
		})
		return
	}

	seed, err := h.loadChaosStorySeed(ctx)
	if err != nil {
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, chaosStoryGetResponse{Seed: seed})
}

// ChaosStoryGenerate godoc
//
// @Summary Chaos Story Generate
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.ChaosStoryEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/chaos-story/generate [post]
func (h *Handler) ChaosStoryGenerate(ctx *fasthttp.RequestCtx) {
	if h.Svc == nil || h.Svc.Assistant == nil || !h.Svc.Assistant.Enabled() {
		apikit.WriteAssistantError(ctx, assistant.WrapError(assistant.ErrNotEnabled))
		return
	}

	seed, err := h.loadChaosStorySeed(ctx)
	if err != nil {
		return
	}
	if len(seed.Streams) == 0 && len(seed.Consumers) == 0 && len(seed.Subjects) == 0 {
		seed = domain.DemoChaosStorySeed()
	}

	var req chaosStoryGenerateRequest
	if len(ctx.PostBody()) > 0 {
		if uerr := serializer.Unmarshal(ctx.PostBody(), &req); uerr != nil {
			apikit.WriteAssistantError(ctx, assistant.WrapError(uerr))
			return
		}
	}

	story, aerr := h.Svc.Assistant.GenerateChaosStory(
		httpctx.FromRequest(ctx),
		seed,
		req.Hint,
	)
	if aerr != nil {
		apikit.WriteAssistantError(ctx, assistant.WrapError(aerr))
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, chaosStoryGenerateResponse{
		Story: story,
		Seed:  seed,
	})
}

func (h *Handler) loadChaosStorySeed(ctx *fasthttp.RequestCtx) (domain.ChaosStorySeed, error) {
	clusterID := apikit.ClusterID(ctx)
	fresh := commonstrings.BytesToString(ctx.QueryArgs().Peek("fresh")) == "1"

	c := httpctx.FromRequest(ctx)
	raw, _, err := h.Svc.Monitoring.FetchJSZ(c, clusterID, fresh)
	if err != nil {
		apikit.WriteJSZFetchError(ctx, err)
		return domain.ChaosStorySeed{}, err
	}

	inputs, err := h.Svc.Monitoring.ChaosStoryInputsFromJSZ(raw)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return domain.ChaosStorySeed{}, err
	}
	return domain.BuildChaosStorySeed(inputs), nil
}
