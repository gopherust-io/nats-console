package assistant

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/config"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLM interface {
	Chat(ctx context.Context, system string, messages []Message) (string, error)
}

type gemini struct {
	client    *http.Client
	apiKey    string
	model     string
	apiBase   string
	maxTokens int
}

func NewLLM(cfg config.Config) (LLM, error) {
	if cfg.AIAPIKey == "" {
		return nil, errors.New("AI_API_KEY is required when AI_ENABLED=true")
	}
	timeout := cfg.AIRequestTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	model := cfg.AIModel
	if model == "" {
		model = "gemini-2.5-flash"
	}
	apiBase := strings.TrimSuffix(cfg.AIGeminiAPIBase, "/")
	if apiBase == "" {
		apiBase = "https://generativelanguage.googleapis.com/v1beta"
	}
	maxTokens := cfg.AIMaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	return &gemini{
		client:    &http.Client{Timeout: timeout},
		apiKey:    cfg.AIAPIKey,
		model:     model,
		maxTokens: maxTokens,
		apiBase:   apiBase,
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
	})
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", g.apiBase, g.model, g.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", newProviderError("Could not create Gemini request.", false, 0)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeoutError(err) {
			return "", newAssistantError(CodeTimeout, "Gemini request timed out. Try again or increase AI_REQUEST_TIMEOUT.", true, 0)
		}
		var netErr net.Error
		if errors.As(err, &netErr) && !netErr.Timeout() {
			return "", newAssistantError(CodeUnavailable, "Could not reach Gemini API. Check your network connection.", true, 0)
		}
		return "", WrapError(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", mapGeminiHTTPError(g.model, resp.StatusCode, raw)
	}

	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := sonic.Unmarshal(raw, &parsed); err != nil {
		return "", newProviderError("Gemini returned an unreadable response.", true, 0)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", newProviderError("Gemini returned an empty response. Try again.", true, 0)
	}
	return parsed.Candidates[0].Content.Parts[0].Text, nil
}

func mapGeminiHTTPError(model string, status int, raw []byte) *Error {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Status  string `json:"status"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	if err := sonic.Unmarshal(raw, &envelope); err != nil || envelope.Error.Message == "" {
		body := strings.TrimSpace(string(raw))
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
