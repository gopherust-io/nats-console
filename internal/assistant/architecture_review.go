package assistant

import (
	"context"
	"encoding/json"

	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const ArchitectureReviewSystemPrompt = `You are the NATS Consol AI Architecture Reviewer.

SCOPE (strict):
- Answer whether the event architecture is good based ONLY on the precomputed findings JSON in the user message.
- Do not invent streams, consumers, subjects, metrics, or findings that are not in that JSON.
- Refuse anything outside JetStream event architecture review.

STYLE:
- Plain text only — no Markdown (no **, no bullet asterisks, no # headings).
- Use these section headings exactly:
Verdict:
Problems:
Suggestions:
- Under Problems and Suggestions, use one line per item starting with "- ".
- Be concise and operator-focused. Reference concrete stream/subject names from the findings.
- If problems is empty, say the architecture looks healthy and suggest continuing to monitor Topology lints.

` + SecurityAndConductRules

// ArchitectureReview asks the LLM to narrate a deterministic architecture snapshot.
func (s *Service) ArchitectureReview(ctx context.Context, snap domain.EventArchitectureSnapshot, question string) (string, error) {
	if s == nil {
		return "", ErrNotEnabled
	}
	if commonstrings.IsEmpty(question) {
		question = "Is this event architecture good?"
	}
	if err := ValidateUserMessage(question); err != nil {
		return "", err
	}

	payload, err := json.Marshal(snap)
	if err != nil {
		return "", newAssistantError(CodeContext, "Could not encode architecture findings.", true, 0)
	}

	user := "Precomputed architecture findings JSON:\n" + commonstrings.BytesToString(payload) +
		"\n\nUser question:\n" + SanitizeMessage(question)

	reply, err := s.llm.Chat(ctx, ArchitectureReviewSystemPrompt, []Message{
		{Role: "user", Content: user},
	})
	if err != nil {
		return "", err
	}
	return SanitizeReply(reply), nil
}
