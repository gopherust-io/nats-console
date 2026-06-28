package assistant

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

var (
	sensitiveKeyPattern = regexp.MustCompile(`(?i)(password|passwd|secret|token|authorization|auth_token|api[_-]?key|credential|creds|private[_-]?key|encryption[_-]?key|session[_-]?secret|cookie|password_hash|oidc_sub|database_url|client_secret|access_token|refresh_token|bearer|jwt|nkey|seed)`)
	exfiltrationPattern = regexp.MustCompile(`(?i)(show|give|tell|reveal|dump|export|what\s+is|print|display|list).{0,40}(password|passwd|secret|api[_-]?key|token|credential|database|db\s|users?\s+table|encryption|session|admin\s+password|connection\s+string|postgres)`)
)

var valueSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9\-._~+/]+=*\b`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9]{10,}\b`),
	regexp.MustCompile(`\bAIza[0-9A-Za-z\-_]{20,}\b`),
	regexp.MustCompile(`(?i)\beyJ[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_]+\b`),
	regexp.MustCompile(`(?i)(postgres|mysql|mongodb)://[^\s"']+`),
	regexp.MustCompile(`(?i)nats://[^\s"']*:[^\s"']*@`),
}

const redacted = "[REDACTED]"

func SanitizeContext(ctx map[string]any) map[string]any {
	return redactValue(ctx).(map[string]any)
}

func SanitizeMessage(content string) string {
	out := content
	for _, pattern := range valueSecretPatterns {
		out = pattern.ReplaceAllString(out, redacted)
	}
	return out
}

func SanitizeHistory(history []Message) []Message {
	if len(history) == 0 {
		return history
	}
	out := make([]Message, len(history))
	for i, msg := range history {
		out[i] = Message{
			Role:    msg.Role,
			Content: SanitizeMessage(msg.Content),
		}
	}
	return out
}

func SanitizeReply(reply string) string {
	return SanitizeMessage(strings.TrimSpace(reply))
}

func ValidateUserMessage(msg string) error {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return errors.New("message is required")
	}
	if len(msg) > 8000 {
		return errors.New("message too long (max 8000 characters)")
	}
	if requestsSensitiveData(msg) {
		return errors.New("the assistant cannot access or reveal secrets, passwords, credentials, or internal database data")
	}
	return nil
}

func redactValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, item := range val {
			if isSensitiveKey(k) {
				out[k] = redacted
				continue
			}
			out[k] = redactValue(item)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = redactValue(item)
		}
		return out
	case string:
		return redactString(val)
	default:
		return v
	}
}

func isSensitiveKey(key string) bool {
	return sensitiveKeyPattern.MatchString(key)
}

func redactString(s string) string {
	if s == "" {
		return s
	}
	out := s
	if u, err := url.Parse(out); err == nil && u.User != nil {
		u.User = url.UserPassword(redacted, redacted)
		out = u.String()
	}
	for _, pattern := range valueSecretPatterns {
		out = pattern.ReplaceAllString(out, redacted)
	}
	if looksLikeSecretValue(out) {
		return redacted
	}
	return out
}

func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	u.User = url.UserPassword(redacted, redacted)
	return u.String()
}

func looksLikeSecretValue(s string) bool {
	lower := strings.ToLower(s)
	if len(s) >= 32 && strings.Contains(lower, "secret") {
		return true
	}
	for _, pattern := range valueSecretPatterns {
		if pattern.MatchString(s) {
			return true
		}
	}
	return false
}

func requestsSensitiveData(message string) bool {
	return exfiltrationPattern.MatchString(strings.TrimSpace(message))
}
