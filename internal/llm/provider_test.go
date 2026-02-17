package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMessage(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello, world!",
	}

	if msg.Role != "user" {
		t.Errorf("expected role 'user', got %s", msg.Role)
	}

	if msg.Content != "Hello, world!" {
		t.Errorf("expected content 'Hello, world!', got %s", msg.Content)
	}
}

func TestHasToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		response *Response
		expected bool
	}{
		{
			name:     "empty response",
			response: &Response{},
			expected: false,
		},
		{
			name: "response with content only",
			response: &Response{
				Content: "Hello!",
			},
			expected: false,
		},
		{
			name: "response with empty tool calls",
			response: &Response{
				Content:   "",
				ToolCalls: []ToolCall{},
			},
			expected: false,
		},
		{
			name: "response with tool calls",
			response: &Response{
				Content: "",
				ToolCalls: []ToolCall{
					{ID: "call-1", Type: "function", Function: &ToolCallFunction{Name: "test"}},
				},
			},
			expected: true,
		},
		{
			name: "response with multiple tool calls",
			response: &Response{
				ToolCalls: []ToolCall{
					{ID: "call-1", Type: "function", Function: &ToolCallFunction{Name: "test1"}},
					{ID: "call-2", Type: "function", Function: &ToolCallFunction{Name: "test2"}},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.response.HasToolCalls(); got != tt.expected {
				t.Errorf("HasToolCalls() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProviderRegistration(t *testing.T) {
	// Test that openai provider is registered
	factory, ok := providers["openai"]
	if !ok {
		t.Error("openai provider not registered")
	}

	// Test creating provider
	cfg := ProviderConfig{
		Type:    "openai",
		BaseURL: "https://api.example.com/v1",
		APIKey:  "test-key",
		Model:   "test-model",
	}

	provider, err := factory(cfg)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	if provider == nil {
		t.Error("provider is nil")
	}
}

func TestProviderUnknown(t *testing.T) {
	cfg := ProviderConfig{
		Type:   "unknown",
		APIKey: "test-key",
	}

	_, err := New(cfg)
	if err == nil {
		t.Error("expected error for unknown provider type")
	}
}

func TestOpenAITokenCount(t *testing.T) {
	cfg := ProviderConfig{
		Type:    "openai",
		APIKey:  "test-key",
		BaseURL: "https://api.example.com/v1",
		Model:   "test-model",
	}

	provider, err := NewOpenAIProvider(cfg)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	messages := []Message{
		{Role: "user", Content: "This is a test message"},
		{Role: "assistant", Content: "This is a response"},
	}

	count := provider.CountTokens(messages)
	if count <= 0 {
		t.Errorf("expected positive token count, got %d", count)
	}
}

func TestComplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := openAIResponse{
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openAIMessage `json:"message"`
				Delta        openAIMessage `json:"delta"`
				FinishReason string        `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: openAIMessage{
						Role:    "assistant",
						Content: "Hello! How can I help you?",
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := ProviderConfig{
		Type:    "openai",
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "test-model",
	}

	provider, err := NewOpenAIProvider(cfg)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	resp, err := provider.Complete(context.Background(), messages)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if resp.Content != "Hello! How can I help you?" {
		t.Errorf("unexpected content: %s", resp.Content)
	}
	if resp.TokensUsed != 15 {
		t.Errorf("expected 15 tokens, got %d", resp.TokensUsed)
	}
}

func TestCompleteWithOptions_ToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		// Verify tools were sent
		if len(req.Tools) == 0 {
			t.Error("expected tools in request")
		}

		resp := openAIResponse{
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openAIMessage `json:"message"`
				Delta        openAIMessage `json:"delta"`
				FinishReason string        `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: openAIMessage{
						Role: "assistant",
						ToolCalls: []openAIToolCall{
							{
								ID:   "call-123",
								Type: "function",
								Function: openAIToolCallFunction{
									Name:      "get_weather",
									Arguments: `{"location": "San Francisco"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     20,
				CompletionTokens: 10,
				TotalTokens:      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := ProviderConfig{
		Type:    "openai",
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "test-model",
	}

	provider, err := NewOpenAIProvider(cfg)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	messages := []Message{
		{Role: "user", Content: "What's the weather?"},
	}

	opts := &CompleteOptions{
		Tools: []ToolDefinition{
			{
				Type: "function",
				Function: &ToolFunctionDef{
					Name:        "get_weather",
					Description: "Get weather",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{"type": "string"},
						},
					},
				},
			},
		},
	}

	resp, err := provider.CompleteWithOptions(context.Background(), messages, opts)
	if err != nil {
		t.Fatalf("CompleteWithOptions() error = %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}

	tc := resp.ToolCalls[0]
	if tc.ID != "call-123" {
		t.Errorf("expected ID 'call-123', got %s", tc.ID)
	}
	if tc.Function.Name != "get_weather" {
		t.Errorf("expected function name 'get_weather', got %s", tc.Function.Name)
	}
	if tc.Function.Arguments != `{"location": "San Francisco"}` {
		t.Errorf("unexpected arguments: %s", tc.Function.Arguments)
	}
	if !resp.HasToolCalls() {
		t.Error("HasToolCalls() should return true")
	}
}

func TestCompleteWithOptions_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			Error: &openAIError{
				Message: "Invalid API key",
				Type:    "invalid_request_error",
				Code:    "invalid_api_key",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := ProviderConfig{
		Type:    "openai",
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "test-model",
	}

	provider, err := NewOpenAIProvider(cfg)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	_, err = provider.Complete(context.Background(), []Message{{Role: "user", Content: "test"}})
	if err == nil {
		t.Error("expected error for API error response")
	}
	if !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCompleteWithOptions_NoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openAIMessage `json:"message"`
				Delta        openAIMessage `json:"delta"`
				FinishReason string        `json:"finish_reason"`
			}{},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := ProviderConfig{
		Type:    "openai",
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "test-model",
	}

	provider, err := NewOpenAIProvider(cfg)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	_, err = provider.Complete(context.Background(), []Message{{Role: "user", Content: "test"}})
	if err == nil {
		t.Error("expected error for empty choices")
	}
}

func TestStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		// Send SSE events
		events := []string{
			`data: {"choices":[{"delta":{"content":"Hello"},"index":0}]}`,
			`data: {"choices":[{"delta":{"content":" world"},"index":0}]}`,
			`data: {"choices":[{"delta":{"content":"!"},"index":0}]}`,
			`data: [DONE]`,
		}

		for _, event := range events {
			w.Write([]byte(event + "\n\n"))
		}
	}))
	defer server.Close()

	cfg := ProviderConfig{
		Type:    "openai",
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "test-model",
	}

	provider, err := NewOpenAIProvider(cfg)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	var chunks []string
	err = provider.Stream(context.Background(), []Message{{Role: "user", Content: "Hi"}}, func(chunk string) {
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	if len(chunks) != 3 {
		t.Errorf("expected 3 chunks, got %d", len(chunks))
	}
	expected := "Hello world!"
	if strings.Join(chunks, "") != expected {
		t.Errorf("expected '%s', got '%s'", expected, strings.Join(chunks, ""))
	}
}

func TestStream_WithToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		// Send SSE events with tool calls in message
		events := []string{
			`data: {"choices":[{"delta":{"role":"assistant","tool_calls":[{"id":"call-1","type":"function","function":{"name":"test","arguments":"{}"}}]},"index":0}]}`,
			`data: [DONE]`,
		}

		for _, event := range events {
			w.Write([]byte(event + "\n\n"))
		}
	}))
	defer server.Close()

	cfg := ProviderConfig{
		Type:    "openai",
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "test-model",
	}

	provider, err := NewOpenAIProvider(cfg)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Stream with tool calls in the message history
	messages := []Message{
		{Role: "user", Content: "test"},
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []ToolCall{
				{ID: "call-1", Type: "function", Function: &ToolCallFunction{Name: "test", Arguments: "{}"}},
			},
		},
		{Role: "tool", ToolCallID: "call-1", Name: "test", Content: "result"},
	}

	var chunks []string
	err = provider.Stream(context.Background(), messages, func(chunk string) {
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	// Stream completed without error
	// Note: tool calls in delta don't produce content chunks
	if len(chunks) != 0 {
		t.Logf("chunks received: %v", chunks)
	}
}

func TestOpenAIError_StringFormat(t *testing.T) {
	// Test string format error
	err := &openAIError{Raw: "something went wrong"}
	if err.Error() != "something went wrong" {
		t.Errorf("expected 'something went wrong', got %s", err.Error())
	}

	// Test object format error
	err2 := &openAIError{Message: "invalid request", Type: "validation_error"}
	if err2.Error() != "invalid request" {
		t.Errorf("expected 'invalid request', got %s", err2.Error())
	}
}

func TestOpenAIError_UnmarshalJSON(t *testing.T) {
	// Test unmarshaling string error
	var err1 openAIError
	if err := json.Unmarshal([]byte(`"rate limit exceeded"`), &err1); err != nil {
		t.Fatalf("failed to unmarshal string error: %v", err)
	}
	if err1.Raw != "rate limit exceeded" {
		t.Errorf("expected Raw 'rate limit exceeded', got %s", err1.Raw)
	}

	// Test unmarshaling object error
	var err2 openAIError
	if err := json.Unmarshal([]byte(`{"message":"bad request","type":"error","code":"400"}`), &err2); err != nil {
		t.Fatalf("failed to unmarshal object error: %v", err)
	}
	if err2.Message != "bad request" {
		t.Errorf("expected Message 'bad request', got %s", err2.Message)
	}
	if err2.Type != "error" {
		t.Errorf("expected Type 'error', got %s", err2.Type)
	}
}

func TestNewOpenAIProvider_MissingAPIKey(t *testing.T) {
	cfg := ProviderConfig{
		Type:    "openai",
		BaseURL: "https://api.example.com/v1",
		Model:   "test-model",
	}

	_, err := NewOpenAIProvider(cfg)
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestNewOpenAIProvider_DefaultBaseURL(t *testing.T) {
	cfg := ProviderConfig{
		Type:   "openai",
		APIKey: "test-key",
		Model:  "test-model",
	}

	provider, err := NewOpenAIProvider(cfg)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	p := provider.(*OpenAIProvider)
	if p.baseURL != "https://api.openai.com/v1" {
		t.Errorf("expected default baseURL, got %s", p.baseURL)
	}
}

func TestCompleteWithOptions_ToolCallWithNilFunction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openAIMessage `json:"message"`
				Delta        openAIMessage `json:"delta"`
				FinishReason string        `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: openAIMessage{
						Role: "assistant",
						ToolCalls: []openAIToolCall{
							{
								ID:       "call-123",
								Type:     "function",
								Function: openAIToolCallFunction{Name: "test", Arguments: "{}"},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				TotalTokens: 10,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := ProviderConfig{
		Type:    "openai",
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "test-model",
	}

	provider, err := NewOpenAIProvider(cfg)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	resp, err := provider.CompleteWithOptions(context.Background(), []Message{{Role: "user", Content: "test"}}, nil)
	if err != nil {
		t.Fatalf("CompleteWithOptions() error = %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
}
