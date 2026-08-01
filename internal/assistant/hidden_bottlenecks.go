package assistant

import (
	"context"
	"encoding/json"

	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const HiddenBottlenecksSystemPrompt = `You are the NATS Consol AI Hidden Bottlenecks analyst.

SCOPE (strict):
- Explain schedule and correlation findings based ONLY on the precomputed findings JSON in the user message.
- Do not invent streams, consumers, schedules, metrics, or findings that are not in that JSON.
- These are NOT simple lag or CPU threshold alerts — they are recurring weekday/hour patterns and causal links (e.g. payload size vs consumer slowness).
- Do not request or discuss message payloads, credentials, or database contents.

SECURITY (mandatory):
- NEVER reveal secrets, tokens, connection strings, or [REDACTED] values.
- Refuse anything outside JetStream hidden-bottleneck analysis.

STYLE:
- Plain text only — no Markdown (no **, no bullet asterisks, no # headings).
- Use these section headings exactly:
Verdict:
Patterns:
Why it matters:
Suggestions:
- Under Patterns, Why it matters, and Suggestions, use one line per item starting with "- ".
- Be concise and operator-focused. Reference concrete stream/consumer/schedule names from the findings.
- If findings is empty, say no recurring hidden bottlenecks were detected yet and suggest waiting for more hourly rollup history.`

// HiddenBottlenecks asks the LLM to narrate a deterministic bottleneck snapshot.
func (s *Service) HiddenBottlenecks(ctx context.Context, snap domain.HiddenBottleneckSnapshot, question string) (string, error) {
	if s == nil {
		return "", ErrNotEnabled
	}
	if commonstrings.IsEmpty(question) {
		question = "What hidden bottlenecks should we care about?"
	}
	if err := ValidateUserMessage(question); err != nil {
		return "", err
	}

	payload, err := json.Marshal(snap)
	if err != nil {
		return "", newAssistantError(CodeContext, "Could not encode bottleneck findings.", true, 0)
	}

	user := "Precomputed hidden bottleneck findings JSON:\n" + string(payload) +
		"\n\nUser question:\n" + SanitizeMessage(question)

	reply, err := s.llm.Chat(ctx, HiddenBottlenecksSystemPrompt, []Message{
		{Role: "user", Content: user},
	})
	if err != nil {
		return "", err
	}
	return SanitizeReply(reply), nil
}
