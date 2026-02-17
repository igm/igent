package llm

import (
	"context"
	"fmt"
)

// Message represents a conversation message
type Message struct {
	Role    string `json:"role"` // system, user, assistant
	Content string `json:"content"`
}

// Response represents the LLM response
type Response struct {
	Content      string
	TokensUsed   int
	FinishReason string
}

// Provider defines the interface for LLM providers
type Provider interface {
	// Complete sends messages to the LLM and returns the response
	Complete(ctx context.Context, messages []Message) (*Response, error)

	// Stream sends messages and streams the response
	Stream(ctx context.Context, messages []Message, onChunk func(string)) error

	// CountTokens estimates token count for messages
	CountTokens(messages []Message) int
}

// ProviderFactory creates a provider based on type
type ProviderFactory func(cfg ProviderConfig) (Provider, error)

// ProviderConfig holds provider-specific configuration
type ProviderConfig struct {
	Type    string
	BaseURL string
	APIKey  string
	Model   string
}

var providers = make(map[string]ProviderFactory)

// Register adds a new provider factory
func Register(name string, factory ProviderFactory) {
	providers[name] = factory
}

// New creates a provider from configuration
func New(cfg ProviderConfig) (Provider, error) {
	factory, ok := providers[cfg.Type]
	if !ok {
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}
	return factory(cfg)
}
