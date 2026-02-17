package llm

import (
	"context"
)

// ZhipuProvider is a specialized provider for Z.AI/GLM models
// Currently uses OpenAI-compatible mode, but can be extended for native API
type ZhipuProvider struct {
	*OpenAIProvider
}

// NewZhipuProvider creates a Z.AI specific provider
func NewZhipuProvider(cfg ProviderConfig) (Provider, error) {
	// Z.AI uses OpenAI-compatible endpoints
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://open.bigmodel.cn/api/paas/v4"
	}

	openai, err := NewOpenAIProvider(cfg)
	if err != nil {
		return nil, err
	}

	return &ZhipuProvider{
		OpenAIProvider: openai.(*OpenAIProvider),
	}, nil
}

func init() {
	Register("glm", func(cfg ProviderConfig) (Provider, error) {
		return NewZhipuProvider(cfg)
	})
}

// Complete overrides to add Z.AI specific handling if needed
func (p *ZhipuProvider) Complete(ctx context.Context, messages []Message) (*Response, error) {
	// Add any Z.AI specific logic here
	return p.OpenAIProvider.Complete(ctx, messages)
}

// Stream overrides to add Z.AI specific handling if needed
func (p *ZhipuProvider) Stream(ctx context.Context, messages []Message, onChunk func(string)) error {
	return p.OpenAIProvider.Stream(ctx, messages, onChunk)
}
