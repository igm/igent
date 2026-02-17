package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/igm/igent/internal/logger"
)

func init() {
	Register("openai", NewOpenAIProvider)
	Register("zhipu", NewOpenAIProvider)     // Z.AI uses OpenAI-compatible API
	Register("anthropic", NewOpenAIProvider) // Can be adapted
}

// OpenAIProvider implements Provider for OpenAI-compatible APIs
type OpenAIProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
	log     *slog.Logger
}

// NewOpenAIProvider creates a new OpenAI-compatible provider
func NewOpenAIProvider(cfg ProviderConfig) (Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &OpenAIProvider{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
		log: logger.L().With("component", "llm", "model", cfg.Model),
	}, nil
}

type openAIRequest struct {
	Model       string           `json:"model"`
	Messages    []openAIMessage  `json:"messages"`
	Stream      bool             `json:"stream,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
}

type openAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int           `json:"index"`
		Message      openAIMessage `json:"message"`
		Delta        openAIMessage `json:"delta"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// openAIMessage matches OpenAI's message format
type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	Name       string           `json:"name,omitempty"`
}

// openAIToolCall matches OpenAI's tool call format
type openAIToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function openAIToolCallFunction `json:"function"`
}

type openAIToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Complete sends a completion request
func (p *OpenAIProvider) Complete(ctx context.Context, messages []Message) (*Response, error) {
	return p.CompleteWithOptions(ctx, messages, nil)
}

// CompleteWithOptions sends a completion request with optional tools
func (p *OpenAIProvider) CompleteWithOptions(ctx context.Context, messages []Message, opts *CompleteOptions) (*Response, error) {
	startTime := time.Now()
	p.log.Debug("sending completion request", "message_count", len(messages))

	// Convert messages to OpenAI format
	openAIMessages := make([]openAIMessage, len(messages))
	for i, m := range messages {
		openAIMessages[i] = openAIMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
			Name:       m.Name,
		}
		if len(m.ToolCalls) > 0 {
			openAIMessages[i].ToolCalls = make([]openAIToolCall, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				openAIMessages[i].ToolCalls[j] = openAIToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: openAIToolCallFunction{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}
	}

	reqBody := openAIRequest{
		Model:    p.model,
		Messages: openAIMessages,
	}

	if opts != nil && len(opts.Tools) > 0 {
		reqBody.Tools = opts.Tools
		p.log.Debug("request includes tools", "tool_count", len(opts.Tools))
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		p.log.Error("request failed", "error", err)
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var result openAIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if result.Error != nil {
		p.log.Error("API error", "message", result.Error.Message, "type", result.Error.Type)
		return nil, fmt.Errorf("API error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := result.Choices[0]
	response := &Response{
		Content:      choice.Message.Content,
		TokensUsed:   result.Usage.TotalTokens,
		FinishReason: choice.FinishReason,
	}

	// Parse tool calls if present
	if len(choice.Message.ToolCalls) > 0 {
		response.ToolCalls = make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			response.ToolCalls[i] = ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: &ToolCallFunction{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
		p.log.Info("response includes tool calls", "count", len(response.ToolCalls))
	}

	duration := time.Since(startTime)
	p.log.Info("completion received",
		"tokens_used", result.Usage.TotalTokens,
		"prompt_tokens", result.Usage.PromptTokens,
		"completion_tokens", result.Usage.CompletionTokens,
		"duration_ms", duration.Milliseconds(),
		"finish_reason", choice.FinishReason,
	)

	return response, nil
}

// Stream sends a streaming completion request
func (p *OpenAIProvider) Stream(ctx context.Context, messages []Message, onChunk func(string)) error {
	startTime := time.Now()
	p.log.Debug("starting stream request", "message_count", len(messages))

	// Convert messages to OpenAI format
	openAIMessages := make([]openAIMessage, len(messages))
	for i, m := range messages {
		openAIMessages[i] = openAIMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
			Name:       m.Name,
		}
		if len(m.ToolCalls) > 0 {
			openAIMessages[i].ToolCalls = make([]openAIToolCall, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				openAIMessages[i].ToolCalls[j] = openAIToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: openAIToolCallFunction{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}
	}

	reqBody := openAIRequest{
		Model:    p.model,
		Messages: openAIMessages,
		Stream:   true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(req)
	if err != nil {
		p.log.Error("stream request failed", "error", err)
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	chunkCount := 0
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var result openAIResponse
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			continue
		}

		if len(result.Choices) > 0 && result.Choices[0].Delta.Content != "" {
			onChunk(result.Choices[0].Delta.Content)
			chunkCount++
		}
	}

	duration := time.Since(startTime)
	p.log.Info("stream completed",
		"chunks", chunkCount,
		"duration_ms", duration.Milliseconds(),
	)

	return scanner.Err()
}

// CountTokens provides a rough estimate of token count
func (p *OpenAIProvider) CountTokens(messages []Message) int {
	// Rough estimation: ~4 chars per token
	total := 0
	for _, m := range messages {
		total += len(m.Content) / 4
		total += 4 // Role overhead
	}
	return total
}
