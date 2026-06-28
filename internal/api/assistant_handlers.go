package api

import (
	"errors"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/assistant"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
)

type AssistantHandler struct {
	svc *assistant.Service
}

func NewAssistantHandler(svc *assistant.Service) *AssistantHandler {
	return &AssistantHandler{svc: svc}
}

func (h *AssistantHandler) Config(ctx *fasthttp.RequestCtx) {
	if h.svc == nil || !h.svc.Enabled() {
		serializer.WriteJSON(ctx, fasthttp.StatusOK, AssistantConfigResponse{
			AIEnabled: false,
		})
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, AssistantConfigResponse{
		AIEnabled:  true,
		AIProvider: h.svc.Provider(),
		AIModel:    h.svc.Model(),
	})
}

func (h *AssistantHandler) Chat(ctx *fasthttp.RequestCtx) {
	if h.svc == nil || !h.svc.Enabled() {
		writeAssistantError(ctx, assistant.WrapError(assistant.ErrNotEnabled))
		return
	}

	var req assistant.ChatRequest
	if err := parseJSONBody(ctx, &req); err != nil {
		writeAssistantError(ctx, assistant.WrapError(err))
		return
	}

	resp, err := h.svc.Chat(requestContext(ctx), clusterID(ctx), req)
	if err != nil {
		writeAssistantError(ctx, assistant.WrapError(err))
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, resp)
}

func writeAssistantError(ctx *fasthttp.RequestCtx, err *assistant.Error) {
	if err == nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, errors.New("assistant request failed"))
		return
	}
	payload := AssistantErrorResponse{
		Error:     err.Message,
		Code:      err.Code,
		Retryable: err.Retryable,
	}
	if err.RetryAfter > 0 {
		payload.RetryAfterSeconds = err.RetryAfter
	}
	serializer.WriteJSON(ctx, err.HTTPStatus(), payload)
}
