package apikit

import (
	"errors"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/app/monitoring"
	"github.com/gopherust-io/nats-consol/internal/assistant"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
)

// WriteAssistantError renders an assistant failure with its own retry hints.
func WriteAssistantError(ctx *fasthttp.RequestCtx, err *assistant.Error) {
	if err == nil {
		httpstatus.WriteError(ctx, fasthttp.StatusInternalServerError, errors.New("assistant request failed"))
		return
	}
	body := httpstatus.ErrorBody{
		Message:   err.Message,
		Code:      err.Code,
		Retryable: err.Retryable,
	}
	if err.RetryAfter > 0 {
		body.RetryAfterSeconds = err.RetryAfter
	}
	httpstatus.WriteErrorBody(ctx, err.HTTPStatus(), body)
}

// WriteJSZFetchError maps a monitoring /jsz fetch failure onto the right status:
// oversized payloads and domain sentinels keep their meaning, everything else is
// an upstream 502.
func WriteJSZFetchError(ctx *fasthttp.RequestCtx, err error) {
	if errors.Is(err, monitoring.ErrPayloadTooLarge) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, ErrMonitoringTooLarge)
		return
	}
	if errors.Is(err, domain.ErrNotFound) ||
		errors.Is(err, domain.ErrForbidden) ||
		errors.Is(err, domain.ErrInvalidInput) {
		WriteAPIError(ctx, err)
		return
	}
	httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
}
