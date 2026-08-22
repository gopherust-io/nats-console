package api

import (
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/assistant"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
)

type AssistantHandler struct {
	svc *assistant.Service
}

func NewAssistantHandler(svc *assistant.Service) *AssistantHandler {
	return &AssistantHandler{svc: svc}
}

// Config godoc
//
// @Summary Config
// @Tags Assistant
// @Produce json
// @Success 200 {object} AssistantConfigEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Router /api/v1/assistant/config [get]
func (h *AssistantHandler) Config(ctx *fasthttp.RequestCtx) {
	if h.svc == nil || !h.svc.Enabled() {
		httpstatus.WriteJSON(ctx, fasthttp.StatusOK, AssistantConfigResponse{
			AIEnabled: false,
		})
		return
	}
	httpstatus.WriteJSON(ctx, fasthttp.StatusOK, AssistantConfigResponse{
		AIEnabled:  true,
		AIProvider: h.svc.Provider(),
		AIModel:    h.svc.Model(),
	})
}

// Chat godoc
//
// @Summary Chat
// @Tags Assistant
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} DataMetaEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/assistant/chat [post]
func (h *AssistantHandler) Chat(ctx *fasthttp.RequestCtx) {
	if h.svc == nil || !h.svc.Enabled() {
		apikit.WriteAssistantError(ctx, assistant.WrapError(assistant.ErrNotEnabled))
		return
	}

	var req assistant.ChatRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		apikit.WriteAssistantError(ctx, assistant.WrapError(err))
		return
	}

	resp, err := h.svc.Chat(httpctx.FromRequest(ctx), apikit.ClusterID(ctx), req)
	if err != nil {
		apikit.WriteAssistantError(ctx, assistant.WrapError(err))
		return
	}
	httpstatus.WriteJSON(ctx, fasthttp.StatusOK, resp)
}
