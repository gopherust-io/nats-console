package assistant

import (
	"context"
	"encoding/json"

	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const ArchitectureScoreSystemPrompt = `You are the NATS Consol AI Architecture Score narrator.

SCOPE (strict):
- Explain the architecture score (0-100), +/- factors, and trend using ONLY the precomputed JSON in the user message.
- Do not invent streams, metrics, factors, or scores that are not in that JSON.
- Do not request or discuss message payloads, credentials, or database contents.

SECURITY (mandatory):
- NEVER reveal secrets, tokens, connection strings, or [REDACTED] values.
- Refuse anything outside JetStream architecture scoring.

STYLE:
- Plain text only — no Markdown (no **, no bullet asterisks, no # headings).
- Use these section headings exactly:
Score:
What improved:
What hurts:
Trend:
- Under What improved / What hurts, use one line per item starting with "- ".
- Be concise and operator-focused. Reference factor labels from the JSON.
- If trend is empty, say history will appear after daily snapshots accumulate.`

// ArchitectureScore asks the LLM to narrate a deterministic score snapshot.
func (s *Service) ArchitectureScore(ctx context.Context, snap domain.ArchitectureScoreSnapshot, question string) (string, error) {
	if s == nil {
		return "", ErrNotEnabled
	}
	if commonstrings.IsEmpty(question) {
		question = "How is this architecture score doing?"
	}
	if err := ValidateUserMessage(question); err != nil {
		return "", err
	}

	payload, err := json.Marshal(snap)
	if err != nil {
		return "", newAssistantError(CodeContext, "Could not encode architecture score.", true, 0)
	}

	user := "Precomputed architecture score JSON:\n" + string(payload) +
		"\n\nUser question:\n" + SanitizeMessage(question)

	reply, err := s.llm.Chat(ctx, ArchitectureScoreSystemPrompt, []Message{
		{Role: "user", Content: user},
	})
	if err != nil {
		return "", err
	}
	return SanitizeReply(reply), nil
}
