package assistant

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const ArchitectureExportADRSystemPrompt = `You are the NATS Consol Architecture Generator.

SCOPE:
- Polish Architecture Decision Records (ADRs) using ONLY the inventory JSON and draft ADR texts provided.
- Do not invent streams, consumers, or subjects that are not present.
- Do not discuss credentials, payloads, or database contents.

STYLE:
- Return plain Markdown ADRs only.
- Keep headings: Context, Decision, Consequences.
- Be concise and operator-focused.

OUTPUT FORMAT (mandatory):
Return exactly two ADR documents separated by a line containing only:
---ADR_SPLIT---
First document is adr/0001-jetstream-topology.md content.
Second document is adr/0002-subject-boundaries.md content.`

// PolishArchitectureADRs asks Gemini to refine deterministic ADR drafts.
func (s *Service) PolishArchitectureADRs(ctx context.Context, inv domain.ArchitectureInventory, drafts map[string]string) (map[string]string, error) {
	if s == nil {
		return nil, ErrNotEnabled
	}
	payload, err := json.Marshal(struct {
		Drafts    map[string]string            `json:"drafts"`
		Inventory domain.ArchitectureInventory `json:"inventory"`
	}{Inventory: inv, Drafts: drafts})
	if err != nil {
		return nil, newAssistantError(CodeContext, "Could not encode architecture inventory.", true, 0)
	}
	user := "Inventory and draft ADRs JSON:\n" + string(payload) +
		"\n\nPolish both ADRs. Separate them with ---ADR_SPLIT---."
	reply, err := s.llm.Chat(ctx, ArchitectureExportADRSystemPrompt, []Message{
		{Role: "user", Content: user},
	})
	if err != nil {
		return nil, err
	}
	reply = SanitizeReply(reply)
	parts := strings.Split(reply, "---ADR_SPLIT---")
	out := map[string]string{}
	if len(parts) >= 1 {
		if t := strings.TrimSpace(parts[0]); !commonstrings.IsEmpty(t) {
			out["adr/0001-jetstream-topology.md"] = t + "\n"
		}
	}
	if len(parts) >= 2 {
		if t := strings.TrimSpace(parts[1]); !commonstrings.IsEmpty(t) {
			out["adr/0002-subject-boundaries.md"] = t + "\n"
		}
	}
	if len(out) == 0 {
		return drafts, nil
	}
	return out, nil
}
