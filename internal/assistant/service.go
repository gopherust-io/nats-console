package assistant

import (
	"context"

	"github.com/gopherust-io/nats-consol/internal/config"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/store"
)

type ChatRequest struct {
	Page    PageContext `json:"page"`
	Message string      `json:"message"`
	History []Message   `json:"history"`
}

type ChatResponse struct {
	Reply string `json:"reply"`
}

type Service struct {
	llm     LLM
	context *ContextBuilder
	cfg     config.Config
}

func NewService(cfg config.Config, st *store.Store, nats *natsclient.Manager) (*Service, error) {
	if !cfg.AIEnabled {
		return nil, nil
	}
	llm, err := NewLLM(cfg)
	if err != nil {
		return nil, err
	}
	return &Service{
		cfg:     cfg,
		llm:     llm,
		context: NewContextBuilder(st, nats, cfg.AIContextCacheTTL),
	}, nil
}

func (s *Service) Enabled() bool {
	return s != nil
}

func (s *Service) Provider() string {
	if s == nil {
		return ""
	}
	return "gemini"
}

func (s *Service) Model() string {
	if s == nil {
		return ""
	}
	if s.cfg.AIModel != "" {
		return s.cfg.AIModel
	}
	return "gemini-2.5-flash"
}

func (s *Service) Chat(ctx context.Context, clusterID string, req ChatRequest) (ChatResponse, error) {
	if s == nil {
		return ChatResponse{}, ErrNotEnabled
	}
	if err := ValidateUserMessage(req.Message); err != nil {
		return ChatResponse{}, err
	}

	clusterCtx, err := s.context.Build(ctx, clusterID, req.Page)
	if err != nil {
		return ChatResponse{}, newAssistantError(CodeContext, "Could not load cluster context. Check NATS connectivity.", true, 0)
	}
	contextBlock, err := FormatContextBlock(clusterCtx)
	if err != nil {
		return ChatResponse{}, newAssistantError(CodeContext, "Could not prepare cluster context for the assistant.", true, 0)
	}

	history := trimHistory(SanitizeHistory(req.History), 12)
	messages := make([]Message, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, Message{
		Role:    "user",
		Content: contextBlock + "\n\nUser question:\n" + SanitizeMessage(req.Message),
	})

	reply, err := s.llm.Chat(ctx, SystemPrompt, messages)
	if err != nil {
		return ChatResponse{}, err
	}
	return ChatResponse{Reply: SanitizeReply(reply)}, nil
}

func trimHistory(history []Message, maxTurns int) []Message {
	if len(history) <= maxTurns {
		return history
	}
	return history[len(history)-maxTurns:]
}
