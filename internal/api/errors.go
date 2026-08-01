package api

import (
	"errors"

	"github.com/gopherust-io/tel"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const natsRetryAfterSeconds = 5

// writeAPIError maps domain sentinels to stable HTTP statuses/codes.
// Unknown errors are logged and sanitized to a generic 500 body.
func writeAPIError(ctx *fasthttp.RequestCtx, err error) {
	if err == nil {
		httpstatus.WriteErrorMessage(ctx, fasthttp.StatusInternalServerError, httpstatus.CodeInternal, "internal error")
		return
	}
	switch {
	case errors.Is(err, domain.ErrNotFound),
		errors.Is(err, domain.ErrAlertNotFound),
		errors.Is(err, domain.ErrAlertRuleNotFound),
		errors.Is(err, domain.ErrEventCatalogEntryNotFound):
		httpstatus.WriteErrorCode(ctx, fasthttp.StatusNotFound, httpstatus.CodeNotFound, err)
	case errors.Is(err, domain.ErrForbidden),
		errors.Is(err, domain.ErrRootProtected),
		errors.Is(err, domain.ErrCannotEscalate):
		httpstatus.WriteErrorCode(ctx, fasthttp.StatusForbidden, httpstatus.CodeForbidden, err)
	case errors.Is(err, domain.ErrInvalidInput), errors.Is(err, domain.ErrInvalidRange):
		httpstatus.WriteErrorCode(ctx, fasthttp.StatusBadRequest, httpstatus.CodeValidation, err)
	case errors.Is(err, domain.ErrConflict),
		errors.Is(err, domain.ErrRootExists),
		errors.Is(err, domain.ErrSigningGroupProtected),
		errors.Is(err, domain.ErrSigningGroupInUse):
		httpstatus.WriteErrorCode(ctx, fasthttp.StatusConflict, httpstatus.CodeConflict, err)
	default:
		tel.Error().Err(err).Str("component", "api").Msg("unhandled API error")
		httpstatus.WriteErrorMessage(ctx, fasthttp.StatusInternalServerError, httpstatus.CodeInternal, "internal error")
	}
}

func writeNATSError(ctx *fasthttp.RequestCtx, status int, err error) {
	if status == fasthttp.StatusNotFound {
		writeAPIError(ctx, domain.ErrNotFound)
		return
	}
	code := httpstatus.CodeFromStatus(status)
	msg := "NATS request failed"
	switch status {
	case fasthttp.StatusGatewayTimeout, fasthttp.StatusRequestTimeout:
		msg = "NATS request timed out"
	case fasthttp.StatusBadGateway, fasthttp.StatusServiceUnavailable:
		msg = "NATS is unavailable"
	case fasthttp.StatusBadRequest, fasthttp.StatusConflict:
		if err != nil && !strings.IsEmpty(err.Error()) {
			msg = err.Error()
		}
	default:
		if code == httpstatus.CodeValidation || code == httpstatus.CodeConflict {
			if err != nil && !strings.IsEmpty(err.Error()) {
				msg = err.Error()
			}
		}
	}
	body := httpstatus.ErrorBody{Message: msg, Code: code}
	switch status {
	case fasthttp.StatusBadGateway, fasthttp.StatusServiceUnavailable,
		fasthttp.StatusGatewayTimeout, fasthttp.StatusRequestTimeout:
		body.Retryable = true
		body.RetryAfterSeconds = natsRetryAfterSeconds
	}
	httpstatus.WriteErrorBody(ctx, status, body)
}
