package assistant

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"
)

const (
	CodeNotEnabled  = "not_enabled"
	CodeValidation  = "validation"
	CodeBlocked     = "blocked"
	CodeContext     = "context"
	CodeRateLimit   = "rate_limit"
	CodeQuota       = "quota"
	CodeAuth        = "auth"
	CodeTimeout     = "timeout"
	CodeProvider    = "provider"
	CodeUnavailable = "unavailable"
)

type Error struct {
	Code       string
	Message    string
	Retryable  bool
	RetryAfter int
	status     int
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *Error) HTTPStatus() int {
	if e.status != 0 {
		return e.status
	}
	switch e.Code {
	case CodeNotEnabled:
		return fasthttp.StatusNotFound
	case CodeValidation, CodeBlocked:
		return fasthttp.StatusBadRequest
	case CodeRateLimit, CodeQuota:
		return fasthttp.StatusTooManyRequests
	case CodeTimeout:
		return fasthttp.StatusGatewayTimeout
	case CodeUnavailable, CodeContext:
		return fasthttp.StatusServiceUnavailable
	default:
		return fasthttp.StatusBadGateway
	}
}

func newAssistantError(code, message string, retryable bool, status int) *Error {
	return &Error{
		Code:      code,
		Message:   message,
		Retryable: retryable,
		status:    status,
	}
}

func WrapError(err error) *Error {
	if err == nil {
		return nil
	}
	var aerr *Error
	if errors.As(err, &aerr) {
		return aerr
	}

	msg := strings.TrimSpace(err.Error())
	lower := strings.ToLower(msg)

	switch {
	case errors.Is(err, ErrNotEnabled):
		return newAssistantError(CodeNotEnabled, "AI assistant is not enabled. Set AI_ENABLED=true and configure AI_API_KEY.", false, 0)
	case errors.Is(err, ErrContextUnavailable):
		return newAssistantError(CodeContext, "Could not load cluster context. Check NATS connectivity.", true, 0)
	case strings.Contains(lower, "message is required"):
		return newAssistantError(CodeValidation, "Enter a message before sending.", false, 0)
	case strings.Contains(lower, "message too long"):
		return newAssistantError(CodeValidation, msg, false, 0)
	case strings.Contains(lower, "cannot access or reveal secrets"):
		return newAssistantError(CodeBlocked, "This assistant cannot access or reveal secrets, passwords, credentials, or internal database data.", false, 0)
	case isTimeoutError(err):
		return newAssistantError(CodeTimeout, "Gemini request timed out. Try again or increase AI_REQUEST_TIMEOUT.", true, 0)
	case strings.Contains(lower, "api key not valid"), strings.Contains(lower, "invalid api key"):
		return newAssistantError(CodeAuth, "Invalid Gemini API key. Check AI_API_KEY in your .env file.", false, 0)
	case strings.Contains(lower, "limit: 0"):
		return newAssistantError(
			CodeQuota,
			"This Gemini model is not available on your plan. Set AI_MODEL=gemini-2.5-flash or enable billing in Google AI Studio.",
			false,
			0,
		)
	case strings.Contains(lower, "rate limit"), strings.Contains(lower, "quota exceeded"):
		return &Error{
			Code:       CodeRateLimit,
			Message:    "Gemini rate limit reached. Wait a moment and try again.",
			Retryable:  true,
			RetryAfter: parseRetryAfterSeconds(msg),
		}
	case strings.Contains(lower, "could not reach gemini"):
		return newAssistantError(CodeUnavailable, "Could not reach Gemini API. Check your network connection.", true, 0)
	default:
		if msg == "" {
			msg = "Assistant request failed"
		}
		return newAssistantError(CodeProvider, msg, true, 0)
	}
}

func isTimeoutError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

var retryAfterPattern = regexp.MustCompile(`(?i)retry in ([0-9]+(?:\.[0-9]+)?)s`)

func parseRetryAfterSeconds(message string) int {
	match := retryAfterPattern.FindStringSubmatch(message)
	if len(match) < 2 {
		return 0
	}
	seconds, err := strconv.ParseFloat(match[1], 64)
	if err != nil || seconds <= 0 {
		return 0
	}
	return int(seconds + 0.999)
}

func newRateLimitError(retryAfter int) *Error {
	return &Error{
		Code:       CodeRateLimit,
		Message:    "Gemini rate limit reached. Wait a moment and try again.",
		Retryable:  true,
		RetryAfter: retryAfter,
		status:     fasthttp.StatusTooManyRequests,
	}
}

func newQuotaError(model string) *Error {
	return newAssistantError(
		CodeQuota,
		fmt.Sprintf("Model %q is not available on your Gemini plan. Try AI_MODEL=gemini-2.5-flash or enable billing.", model),
		false,
		fasthttp.StatusTooManyRequests,
	)
}

func newAuthError() *Error {
	return newAssistantError(CodeAuth, "Gemini rejected the API key. Verify AI_API_KEY in .env.", false, fasthttp.StatusUnauthorized)
}

func newProviderError(message string, retryable bool, status int) *Error {
	if strings.TrimSpace(message) == "" {
		message = "Gemini request failed"
	}
	return newAssistantError(CodeProvider, message, retryable, status)
}
