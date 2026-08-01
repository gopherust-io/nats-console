package assistant

import (
	"context"
	"testing"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type chaosStubLLM struct {
	err   error
	reply string
}

func (s *chaosStubLLM) Chat(_ context.Context, _ string, _ []Message) (string, error) {
	return s.reply, s.err
}

func TestParseChaosStoryJSON(t *testing.T) {
	raw := "```json\n{\"title\":\"T\",\"setting\":\"S\",\"severity\":\"critical\",\"summary\":\"sum\",\"acts\":[{\"title\":\"A\",\"description\":\"D\",\"kind\":\"quorum_loss\",\"targets\":[\"ORDERS\"],\"durationSec\":5}]}\n```"
	story, err := ParseChaosStoryJSON(raw)
	require.NoError(t, err)
	assert.Equal(t, "T", story.Title)
	require.Len(t, story.Acts, 1)
	assert.Equal(t, "ORDERS", story.Acts[0].Targets[0])

	_, err = ParseChaosStoryJSON("not json")
	require.Error(t, err)
}

func TestGenerateChaosStoryNilService(t *testing.T) {
	var s *Service
	_, err := s.GenerateChaosStory(t.Context(), domain.DemoChaosStorySeed(), "")
	require.ErrorIs(t, err, ErrNotEnabled)
}

func TestGenerateChaosStoryWithStub(t *testing.T) {
	llm := &chaosStubLLM{reply: `{"title":"Peak failure","setting":"Black Friday","severity":"critical","summary":"Multi-failure","acts":[{"title":"Down","description":"PAYMENTS unreachable","kind":"cluster_down","targets":["PAYMENTS","ghost"],"durationSec":4}],"blastRadius":["checkout"],"recoveryHints":["restore"]}`}
	svc := &Service{llm: llm}
	story, err := svc.GenerateChaosStory(t.Context(), domain.DemoChaosStorySeed(), "Black Friday")
	require.NoError(t, err)
	assert.Equal(t, domain.ChaosStorySourceAI, story.Source)
	assert.Equal(t, []string{"PAYMENTS"}, story.Acts[0].Targets)
	assert.NotContains(t, story.Acts[0].Targets, "ghost")
}
