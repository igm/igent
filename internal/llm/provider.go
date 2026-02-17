package llm

import (
	"context"
	"fmt"
)

// ToolCall represents a tool call in a message
type ToolCall struct {
	ID       string            `json:"id"`
	Type     string            `json:"type,omitempty"` // usually "function"
	Function *ToolCallFunction `json:"function,omitempty"`
}

// ToolCallFunction contains the function details of a tool call
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// Message represents a conversation message
type Message struct {
	Role       string     `json:"role"`                   // system, user, assistant, tool
	Content    string     `json:"content"`                // Can be empty for tool calls
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // For assistant messages requesting tools
	ToolCallID string     `json:"tool_call_id,omitempty"` // For tool response messages
	Name       string     `json:"name,omitempty"`         // Tool name for tool role messages
}

// Response represents the LLM response
type Response struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	TokensUsed   int        `json:"tokens_used"`
	FinishReason string     `json:"finish_reason"`
}

// HasToolCalls returns true if the response contains tool calls
func (r *Response) HasToolCalls() bool {
	return len(r.ToolCalls) > 0
}

// ToolDefinition represents a tool definition for the LLM
type ToolDefinition struct {
	Type     string           `json:"type"` // "function"
	Function *ToolFunctionDef `json:"function"`
}

// ToolFunctionDef defines a function tool
type ToolFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// CompleteOptions holds optional parameters for completion
type CompleteOptions struct {
	Tools []ToolDefinition `json:"tools,omitempty"`
}

// Provider defines the interface for LLM providers
type Provider interface {
	// Complete sends messages to the LLM and returns the response
	Complete(ctx context.Context, messages []Message) (*Response, error)

	// CompleteWithOptions sends messages with additional options (like tools)
	CompleteWithOptions(ctx context.Context, messages []Message, opts *CompleteOptions) (*Response, error)

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
