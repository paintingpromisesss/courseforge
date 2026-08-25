package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/paintingpromisesss/courseforge/internal/domain"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/ai"
	"go.uber.org/zap"
)

type aiConfigRepository interface {
	Load(ctx context.Context) (*domain.AIConfig, error)
	Save(ctx context.Context, cfg *domain.AIConfig) error
}

type AIService struct {
	repo   aiConfigRepository
	logger *zap.Logger

	mu     sync.RWMutex
	client *ai.AIClient
}

func NewAIService(repo aiConfigRepository, logger *zap.Logger) *AIService {
	return &AIService{
		repo:   repo,
		logger: logger,
	}
}

// Init loads the config from repo and initializes the AIClient if enabled.
func (s *AIService) Init(ctx context.Context) error {
	cfg, err := s.repo.Load(ctx)
	if err != nil {
		s.logger.Error("failed to load ai config on init", zap.Error(err))
		return fmt.Errorf("init ai service: %w", err)
	}

	s.updateClient(cfg)
	return nil
}

func (s *AIService) GetConfig(ctx context.Context) (*domain.AIConfig, error) {
	cfg, err := s.repo.Load(ctx)
	if err != nil {
		s.logger.Error("failed to load ai config", zap.Error(err))
		return nil, fmt.Errorf("get ai config: %w", err)
	}
	return cfg, nil
}

func (s *AIService) SaveConfig(ctx context.Context, cfg *domain.AIConfig) error {
	if err := s.repo.Save(ctx, cfg); err != nil {
		s.logger.Error("failed to save ai config", zap.Error(err))
		return fmt.Errorf("save ai config: %w", err)
	}

	s.updateClient(cfg)
	return nil
}

func (s *AIService) updateClient(cfg *domain.AIConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cfg == nil || !cfg.Enabled {
		s.client = nil
		return
	}

	s.client = ai.NewClient(ai.ClientConfig{
		Provider:     cfg.Provider,
		BaseURL:      cfg.BaseURL,
		APIKey:       cfg.APIKey,
		Model:        cfg.Model,
		SystemPrompt: cfg.SystemPrompt,
	})
}

func (s *AIService) Client() *ai.AIClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}

func (s *AIService) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client != nil
}

func (s *AIService) ChatStream(ctx context.Context, messages []domain.Message, fn func(chunk string)) error {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("ai service is disabled")
	}

	return client.ChatStream(ctx, messages, fn)
}

func (s *AIService) FetchAvailableModels(ctx context.Context, provider domain.Provider, baseURL, apiKey string) ([]string, error) {
	switch provider {
	case domain.ProviderAnthropic:
		return ai.FetchAnthropicModels(ctx, baseURL, apiKey)
	case domain.ProviderOpenAI:
		return ai.FetchOpenAIAvailableModels(ctx, baseURL, apiKey)
	default:
		return ai.FetchOpenAIAvailableModels(ctx, baseURL, apiKey)
	}
}
