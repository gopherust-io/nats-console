package api

import (
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/assistant"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
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

// ChaosStoryDemo returns the canned Black Friday chaos story (no cluster required).
func (h *Handler) ChaosStoryDemo(ctx *fasthttp.RequestCtx) {
	story := domain.DemoChaosStory()
	httpstatus.WriteData(ctx, fasthttp.StatusOK, chaosStoryGetResponse{
		Story: &story,
		Seed:  domain.DemoChaosStorySeed(),
	})
}

// ChaosStory returns demo story or live inventory seed for the Chaos Story Generator.
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

// ChaosStoryGenerate uses Gemini to invent a multi-act disaster story from live inventory.
func (h *Handler) ChaosStoryGenerate(ctx *fasthttp.RequestCtx) {
	if h.svc == nil || h.svc.Assistant == nil || !h.svc.Assistant.Enabled() {
		writeAssistantError(ctx, assistant.WrapError(assistant.ErrNotEnabled))
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
			writeAssistantError(ctx, assistant.WrapError(uerr))
			return
		}
	}

	story, aerr := h.svc.Assistant.GenerateChaosStory(
		httpctx.FromRequest(ctx),
		seed,
		req.Hint,
	)
	if aerr != nil {
		writeAssistantError(ctx, assistant.WrapError(aerr))
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, chaosStoryGenerateResponse{
		Story: story,
		Seed:  seed,
	})
}

func (h *Handler) loadChaosStorySeed(ctx *fasthttp.RequestCtx) (domain.ChaosStorySeed, error) {
	clusterID := clusterID(ctx)
	fresh := commonstrings.BytesToString(ctx.QueryArgs().Peek("fresh")) == "1"

	var raw []byte
	if !fresh && h.hub != nil {
		if data, _, ok := h.hub.MonitoringPayload(clusterID, snapshot.TopologyJSZPath); ok {
			raw = data
		}
	}
	if raw == nil {
		c := httpctx.FromRequest(ctx)
		client, err := h.svc.JetStream.GetExecutor(c, clusterID)
		if err != nil {
			writeAPIError(ctx, err)
			return domain.ChaosStorySeed{}, err
		}
		data, err := client.Monitoring(c, snapshot.TopologyJSZPath)
		if err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
			return domain.ChaosStorySeed{}, err
		}
		raw = data
		if int64(len(raw)) > h.cfg.MaxMonitoringBodyBytes {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, errMonitoringTooLarge)
			return domain.ChaosStorySeed{}, errMonitoringTooLarge
		}
	}

	projected := projectJSZForTopology(raw)
	inputs, err := chaosStoryInputsFromJSZ(projected)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return domain.ChaosStorySeed{}, err
	}
	return domain.BuildChaosStorySeed(inputs), nil
}

func chaosStoryInputsFromJSZ(raw []byte) ([]domain.ChaosStoryInventoryInput, error) {
	var payload jszTopologyPayload
	if err := serializer.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	out := make([]domain.ChaosStoryInventoryInput, 0)
	for _, acct := range payload.AccountDetails {
		for _, stream := range acct.StreamDetail {
			in := domain.ChaosStoryInventoryInput{Name: stream.Name}
			if stream.Config != nil {
				in.Subjects = append([]string(nil), stream.Config.Subjects...)
			}
			for _, c := range stream.ConsumerDetail {
				in.Consumers = append(in.Consumers, c.Name)
			}
			out = append(out, in)
		}
	}
	return out, nil
}
