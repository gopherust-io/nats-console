package assistant

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const ChaosStorySystemPrompt = `You are the NATS Consol Chaos Story Generator.

SCOPE (strict):
- Invent a realistic multi-act JetStream / event-driven disaster story.
- Use ONLY stream, consumer, and subject names from the provided inventory seed JSON.
- You MAY invent failure modes, timing (e.g. Black Friday), and narrative beats.
- Do NOT invent inventory names that are not in the seed.
- Keep descriptions professional — no graphic, sexual, or abusive language.

OUTPUT (mandatory):
- Return ONLY a single JSON object (no markdown fences, no commentary) with this shape:
{
  "title": string,
  "setting": string,
  "severity": "info"|"warn"|"critical",
  "summary": string,
  "acts": [
    {
      "title": string,
      "description": string,
      "kind": "cluster_down"|"quorum_loss"|"schema_mismatch"|"consumer_deploy"|"traffic_spike"|"partition"|"recovery",
      "targets": [string],
      "durationSec": number
    }
  ],
  "blastRadius": [string],
  "recoveryHints": [string]
}
- Include 4–6 acts that escalate then recover.
- Prefer concrete operator-focused descriptions.

` + SecurityAndConductRules

// GenerateChaosStory invents a typed disaster narrative from inventory seed names.
func (s *Service) GenerateChaosStory(ctx context.Context, seed domain.ChaosStorySeed, hint string) (domain.ChaosStory, error) {
	if s == nil {
		return domain.ChaosStory{}, ErrNotEnabled
	}
	if commonstrings.IsEmpty(hint) {
		hint = "Invent a realistic multi-failure chaos story for peak traffic."
	}
	if err := ValidateUserMessage(hint); err != nil {
		return domain.ChaosStory{}, err
	}

	seedJSON, err := json.Marshal(seed)
	if err != nil {
		return domain.ChaosStory{}, newAssistantError(CodeContext, "Could not encode chaos story seed.", true, 0)
	}

	user := "Inventory seed JSON (use only these names):\n" + commonstrings.BytesToString(seedJSON) +
		"\n\nOperator hint:\n" + SanitizeMessage(hint) +
		"\n\nReturn only the chaos story JSON object."

	reply, err := s.llm.Chat(ctx, ChaosStorySystemPrompt, []Message{
		{Role: "user", Content: user},
	})
	if err != nil {
		return domain.ChaosStory{}, err
	}

	story, err := ParseChaosStoryJSON(SanitizeReply(reply))
	if err != nil {
		return domain.ChaosStory{}, err
	}
	story = domain.NormalizeChaosStory(story, domain.ChaosStorySourceAI)
	story = domain.FilterChaosStoryTargets(story, seed)
	story.Demo = false
	return story, nil
}

// ParseChaosStoryJSON extracts and unmarshals a ChaosStory from model output.
func ParseChaosStoryJSON(raw string) (domain.ChaosStory, error) {
	candidate := extractJSONObject(raw)
	if commonstrings.IsEmpty(candidate) {
		return domain.ChaosStory{}, newAssistantError(CodeBlocked, "model did not return valid chaos story JSON", true, 0)
	}
	var story domain.ChaosStory
	if err := json.Unmarshal(commonstrings.StringToBytes(candidate), &story); err != nil {
		return domain.ChaosStory{}, newAssistantError(CodeBlocked, "model did not return valid chaos story JSON", true, 0)
	}
	if commonstrings.IsEmpty(story.Title) || len(story.Acts) == 0 {
		return domain.ChaosStory{}, newAssistantError(CodeBlocked, "chaos story missing title or acts", true, 0)
	}
	return story, nil
}

func extractJSONObject(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```JSON")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return ""
	}
	candidate := raw[start : end+1]
	var v any
	if err := json.Unmarshal(commonstrings.StringToBytes(candidate), &v); err != nil {
		return ""
	}
	if _, ok := v.(map[string]any); !ok {
		return ""
	}
	return candidate
}
