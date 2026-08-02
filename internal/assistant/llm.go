package assistant

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/httpclient"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLM interface {
	Chat(ctx context.Context, system string, messages []Message) (string, error)
}

type gemini struct {
	client    *fasthttp.Client
	apiKey    string
	model     string
	apiBase   string
	maxTokens int
	timeout   time.Duration
}

func NewLLM(cfg config.Config) (LLM, error) {
	if commonstrings.IsEmpty(cfg.AI.APIKey) {
		return nil, errors.New("AI_API_KEY is required when AI_ENABLED=true")
	}
	timeout := cfg.AI.RequestTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	model := cfg.AI.Model
	if commonstrings.IsEmpty(model) {
		model = "gemini-2.5-flash"
	}
	apiBase := strings.TrimSuffix(cfg.AI.GeminiAPIBase, "/")
	if commonstrings.IsEmpty(apiBase) {
		apiBase = "https://generativelanguage.googleapis.com/v1beta"
	}
	maxTokens := cfg.AI.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	return &gemini{
		client:    httpclient.NewClient(cfg, timeout),
		apiKey:    cfg.AI.APIKey,
		model:     model,
		maxTokens: maxTokens,
		apiBase:   apiBase,
		timeout:   timeout,
	}, nil
}

func (g *gemini) Chat(ctx context.Context, system string, messages []Message) (string, error) {
	contents := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		role := "user"
		if m.Role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]any{
			"role":  role,
			"parts": []map[string]string{{"text": m.Content}},
		})
	}
	maxTokens := g.maxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	body, _ := sonic.Marshal(map[string]any{
		"systemInstruction": map[string]any{
			"parts": []map[string]string{{"text": system}},
		},
		"contents": contents,
		"generationConfig": map[string]any{
			"maxOutputTokens": maxTokens,
			"temperature":     0.2,
		},
		"safetySettings": []map[string]string{
			{"category": "HARM_CATEGORY_HARASSMENT", "threshold": "BLOCK_MEDIUM_AND_ABOVE"},
			{"category": "HARM_CATEGORY_HATE_SPEECH", "threshold": "BLOCK_MEDIUM_AND_ABOVE"},
			{"category": "HARM_CATEGORY_SEXUALLY_EXPLICIT", "threshold": "BLOCK_MEDIUM_AND_ABOVE"},
			{"category": "HARM_CATEGORY_DANGEROUS_CONTENT", "threshold": "BLOCK_MEDIUM_AND_ABOVE"},
		},
	})
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", g.apiBase, g.model, g.apiKey)

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI(url)
	req.Header.SetContentType("application/json")
	req.SetBody(body)

	timeout := g.timeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return "", newAssistantError(CodeTimeout, "Gemini request timed out. Try again or increase AI_REQUEST_TIMEOUT.", true, 0)
		}
		if remaining < timeout || timeout <= 0 {
			timeout = remaining
		}
	}
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", newAssistantError(CodeTimeout, "Gemini request timed out. Try again or increase AI_REQUEST_TIMEOUT.", true, 0)
		}
		return "", WrapError(err)
	}

	var err error
	if timeout > 0 {
		err = g.client.DoTimeout(req, resp, timeout)
	} else {
		err = g.client.Do(req, resp)
	}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, fasthttp.ErrTimeout) || isTimeoutError(err) {
			return "", newAssistantError(CodeTimeout, "Gemini request timed out. Try again or increase AI_REQUEST_TIMEOUT.", true, 0)
		}
		var netErr net.Error
		if errors.As(err, &netErr) && !netErr.Timeout() {
			return "", newAssistantError(CodeUnavailable, "Could not reach Gemini API. Check your network connection.", true, 0)
		}
		return "", WrapError(err)
	}

	raw := append([]byte(nil), resp.Body()...)
	status := resp.StatusCode()
	if status >= 400 {
		return "", mapGeminiHTTPError(g.model, status, raw)
	}

	var parsed struct {
		PromptFeedback *struct {
			BlockReason string `json:"blockReason"`
		} `json:"promptFeedback"`
		Candidates []struct {
			FinishReason string `json:"finishReason"`
			Content      struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := sonic.Unmarshal(raw, &parsed); err != nil {
		return "", newProviderError("Gemini returned an unreadable response.", true, 0)
	}
	if parsed.PromptFeedback != nil && !commonstrings.IsEmpty(parsed.PromptFeedback.BlockReason) {
		return "", newAssistantError(CodeBlocked, "Request blocked by content safety filters.", false, 0)
	}
	if len(parsed.Candidates) == 0 {
		return "", newProviderError("Gemini returned an empty response. Try again.", true, 0)
	}
	candidate := parsed.Candidates[0]
	if isGeminiSafetyFinish(candidate.FinishReason) {
		return "", newAssistantError(CodeBlocked, "Response blocked by content safety filters.", false, 0)
	}
	if len(candidate.Content.Parts) == 0 || commonstrings.IsEmpty(strings.TrimSpace(candidate.Content.Parts[0].Text)) {
		return "", newProviderError("Gemini returned an empty response. Try again.", true, 0)
	}
	return candidate.Content.Parts[0].Text, nil
}

func isGeminiSafetyFinish(reason string) bool {
	switch strings.ToUpper(strings.TrimSpace(reason)) {
	case "SAFETY", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII":
		return true
	default:
		return false
	}
}

func mapGeminiHTTPError(model string, status int, raw []byte) *Error {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Status  string `json:"status"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	if err := sonic.Unmarshal(raw, &envelope); err != nil || commonstrings.IsEmpty(envelope.Error.Message) {
		body := strings.TrimSpace(commonstrings.BytesToString(raw))
		if len(body) > 200 {
			body = body[:200] + "…"
		}
		switch status {
		case fasthttp.StatusUnauthorized, fasthttp.StatusForbidden:
			return newAuthError()
		case fasthttp.StatusTooManyRequests:
			return newRateLimitError(0)
		default:
			return newProviderError(fmt.Sprintf("Gemini API error (HTTP %d): %s", status, body), status >= 500, status)
		}
	}

	msg := strings.TrimSpace(envelope.Error.Message)
	retryAfter := parseRetryAfterSeconds(msg)

	switch {
	case strings.Contains(msg, "limit: 0"):
		return newQuotaError(model)
	case envelope.Error.Code == 429 || status == fasthttp.StatusTooManyRequests:
		return newRateLimitError(retryAfter)
	case envelope.Error.Status == "UNAUTHENTICATED", envelope.Error.Status == "PERMISSION_DENIED":
		return newAuthError()
	case strings.Contains(strings.ToLower(msg), "api key not valid"):
		return newAuthError()
	default:
		firstLine := msg
		if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
			firstLine = firstLine[:idx]
		}
		return newProviderError(firstLine, status >= 500, status)
	}
}
