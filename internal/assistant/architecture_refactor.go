package assistant

import (
	"context"
	"encoding/json"

	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const ArchitectureRefactorSystemPrompt = `You are the NATS Consol AI Architecture Refactoring assistant.

SCOPE:
- Help reduce coupling using ONLY the precomputed refactor plan JSON (before/after graphs and migration steps).
- Do not invent streams, consumers, or subjects absent from that JSON.
- Refuse anything outside JetStream architecture refactoring.

STYLE:
- Plain text only — no Markdown asterisks or # headings.
- Use these section headings exactly:
Verdict:
Before:
After:
Migration steps:
- Under Migration steps, number steps 1. 2. 3. …
- Be concise and operator-focused. Emphasize dual-publish then cutover.

` + SecurityAndConductRules

// ArchitectureRefactor narrates a deterministic coupling-reduction plan.
func (s *Service) ArchitectureRefactor(ctx context.Context, plan domain.ArchitectureRefactorPlan, question string) (string, error) {
	if s == nil {
		return "", ErrNotEnabled
	}
	if commonstrings.IsEmpty(question) {
		question = "Reduce coupling."
	}
	if err := ValidateUserMessage(question); err != nil {
		return "", err
	}
	payload, err := json.Marshal(plan)
	if err != nil {
		return "", newAssistantError(CodeContext, "Could not encode refactor plan.", true, 0)
	}
	user := "Precomputed refactor plan JSON:\n" + commonstrings.BytesToString(payload) +
		"\n\nUser question:\n" + SanitizeMessage(question)
	reply, err := s.llm.Chat(ctx, ArchitectureRefactorSystemPrompt, []Message{
		{Role: "user", Content: user},
	})
	if err != nil {
		return "", err
	}
	return SanitizeReply(reply), nil
}
