package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/igm/igent/internal/config"
	"github.com/igm/igent/internal/llm"
	"github.com/igm/igent/internal/storage"
)

// mockProvider for testing
type mockProvider struct {
	response    string
	toolCalls   []llm.ToolCall
	callCount   int
	responses   []string // for multiple responses
	streamError error
}

func (m *mockProvider) Complete(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	return &llm.Response{Content: m.response}, nil
}

func (m *mockProvider) CompleteWithOptions(ctx context.Context, messages []llm.Message, opts *llm.CompleteOptions) (*llm.Response, error) {
	m.callCount++
	// First call returns tool calls, subsequent calls return text
	if m.toolCalls != nil && m.callCount == 1 {
		return &llm.Response{ToolCalls: m.toolCalls}, nil
	}
	if len(m.responses) > 0 {
		idx := m.callCount - 1
		if idx < len(m.responses) {
			return &llm.Response{Content: m.responses[idx]}, nil
		}
	}
	return &llm.Response{Content: m.response}, nil
}

func (m *mockProvider) Stream(ctx context.Context, messages []llm.Message, onChunk func(string)) error {
	if m.streamError != nil {
		return m.streamError
	}
	onChunk(m.response)
	return nil
}

func (m *mockProvider) CountTokens(messages []llm.Message) int {
	return len(messages) * 10
}

func TestNewAgent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Provider: config.ProviderConfig{
			Type:    "openai",
			APIKey:  "test-key",
			BaseURL: "https://api.example.com/v1",
			Model:   "test-model",
		},
		Storage: config.StorageConfig{
			WorkDir: tmpDir,
		},
		Context: config.ContextConfig{
			MaxMessages:   10,
			MaxTokens:     1000,
			SummarizeWhen: 5,
		},
		Agent: config.AgentConfig{
			Name:         "test-agent",
			SystemPrompt: "Test prompt",
		},
	}

	ag, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	if ag == nil {
		t.Error("agent is nil")
	}
}

func TestSetConversation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Provider: config.ProviderConfig{
			Type:    "openai",
			APIKey:  "test-key",
			BaseURL: "https://api.example.com/v1",
			Model:   "test-model",
		},
		Storage: config.StorageConfig{
			WorkDir: tmpDir,
		},
		Context: config.ContextConfig{
			MaxMessages:   10,
			MaxTokens:     1000,
			SummarizeWhen: 5,
		},
		Agent: config.AgentConfig{
			Name:         "test-agent",
			SystemPrompt: "Test prompt",
		},
	}

	ag, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Create new conversation
	if err := ag.SetConversation("test-conv"); err != nil {
		t.Fatalf("failed to set conversation: %v", err)
	}

	// Load conversation
	store, _ := storage.NewJSONStore(tmpDir)
	conv, err := store.LoadConversation("test-conv")
	if err != nil {
		t.Fatalf("failed to load conversation: %v", err)
	}

	if conv.ID != "test-conv" {
		t.Errorf("expected ID 'test-conv', got %s", conv.ID)
	}

	// Switch to existing conversation
	if err := ag.SetConversation("test-conv"); err != nil {
		t.Fatalf("failed to switch conversation: %v", err)
	}
}

func TestListConversations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Provider: config.ProviderConfig{
			Type:    "openai",
			APIKey:  "test-key",
			BaseURL: "https://api.example.com/v1",
			Model:   "test-model",
		},
		Storage: config.StorageConfig{
			WorkDir: tmpDir,
		},
		Context: config.ContextConfig{
			MaxMessages:   10,
			MaxTokens:     1000,
			SummarizeWhen: 5,
		},
		Agent: config.AgentConfig{
			Name:         "test-agent",
			SystemPrompt: "Test prompt",
		},
	}

	ag, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Create multiple conversations
	ag.SetConversation("conv1")
	ag.SetConversation("conv2")

	convs, err := ag.ListConversations()
	if err != nil {
		t.Fatalf("failed to list conversations: %v", err)
	}

	if len(convs) < 2 {
		t.Errorf("expected at least 2 conversations, got %d", len(convs))
	}
}

func TestMemoryOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Provider: config.ProviderConfig{
			Type:    "openai",
			APIKey:  "test-key",
			BaseURL: "https://api.example.com/v1",
			Model:   "test-model",
		},
		Storage: config.StorageConfig{
			WorkDir: tmpDir,
		},
		Context: config.ContextConfig{
			MaxMessages:   10,
			MaxTokens:     1000,
			SummarizeWhen: 5,
		},
		Agent: config.AgentConfig{
			Name:         "test-agent",
			SystemPrompt: "Test prompt",
		},
	}

	ag, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Add memory
	if err := ag.AddMemory("Test memory", "fact"); err != nil {
		t.Fatalf("failed to add memory: %v", err)
	}

	// List memories
	memories, err := ag.ListMemories()
	if err != nil {
		t.Fatalf("failed to list memories: %v", err)
	}

	if len(memories) != 1 {
		t.Errorf("expected 1 memory, got %d", len(memories))
	}

	// Delete memory
	if err := ag.DeleteMemory(memories[0].ID); err != nil {
		t.Fatalf("failed to delete memory: %v", err)
	}

	memories, _ = ag.ListMemories()
	if len(memories) != 0 {
		t.Errorf("expected 0 memories after delete, got %d", len(memories))
	}
}

func TestSkillOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Provider: config.ProviderConfig{
			Type:    "openai",
			APIKey:  "test-key",
			BaseURL: "https://api.example.com/v1",
			Model:   "test-model",
		},
		Storage: config.StorageConfig{
			WorkDir: tmpDir,
		},
		Context: config.ContextConfig{
			MaxMessages:   10,
			MaxTokens:     1000,
			SummarizeWhen: 5,
		},
		Agent: config.AgentConfig{
			Name:         "test-agent",
			SystemPrompt: "Test prompt",
		},
	}

	ag, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// List skills (should have defaults)
	skills := ag.ListSkills()
	if len(skills) == 0 {
		t.Error("expected default skills to be loaded")
	}

	// Register new skill
	newSkill := &storage.Skill{
		ID:          "test-skill",
		Name:        "Test Skill",
		Description: "A test skill",
		Prompt:      "Test prompt",
		Enabled:     true,
	}

	if err := ag.RegisterSkill(newSkill); err != nil {
		t.Fatalf("failed to register skill: %v", err)
	}

	// Unregister skill
	if err := ag.UnregisterSkill("test-skill"); err != nil {
		t.Fatalf("failed to unregister skill: %v", err)
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Provider: config.ProviderConfig{
			Type:    "openai",
			APIKey:  "test-key",
			BaseURL: "https://api.example.com/v1",
			Model:   "test-model",
		},
		Storage: config.StorageConfig{
			WorkDir: tmpDir,
		},
		Context: config.ContextConfig{
			MaxMessages:   10,
			MaxTokens:     1000,
			SummarizeWhen: 5,
		},
		Agent: config.AgentConfig{
			Name:         "test-agent",
			SystemPrompt: "You are a helpful assistant.",
		},
	}

	ag, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	prompt := ag.buildSystemPrompt()

	if !strings.Contains(prompt, "You are a helpful assistant.") {
		t.Error("system prompt should contain base prompt")
	}
	if !strings.Contains(prompt, "Current date and time:") {
		t.Error("system prompt should contain current date and time")
	}
	// Verify it contains a valid date format
	if !strings.Contains(prompt, time.Now().Format("2006")) {
		t.Error("system prompt should contain current year")
	}
}

func TestBuildToolDefinitions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Provider: config.ProviderConfig{
			Type:    "openai",
			APIKey:  "test-key",
			BaseURL: "https://api.example.com/v1",
			Model:   "test-model",
		},
		Storage: config.StorageConfig{
			WorkDir: tmpDir,
		},
		Context: config.ContextConfig{
			MaxMessages:   10,
			MaxTokens:     1000,
			SummarizeWhen: 5,
		},
		Agent: config.AgentConfig{
			Name:         "test-agent",
			SystemPrompt: "Test prompt",
		},
	}

	ag, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	defs := ag.buildToolDefinitions()

	if len(defs) == 0 {
		t.Error("expected tool definitions to be non-empty")
	}

	// Verify structure of tool definitions
	for _, def := range defs {
		if def.Type != "function" {
			t.Errorf("expected type 'function', got %s", def.Type)
		}
		if def.Function == nil {
			t.Error("function definition should not be nil")
			continue
		}
		if def.Function.Name == "" {
			t.Error("function name should not be empty")
		}
		if def.Function.Description == "" {
			t.Error("function description should not be empty")
		}
		if def.Function.Parameters == nil {
			t.Error("function parameters should not be nil")
		}
	}
}

func TestChat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Provider: config.ProviderConfig{
			Type:    "openai",
			APIKey:  "test-key",
			BaseURL: "https://api.example.com/v1",
			Model:   "test-model",
		},
		Storage: config.StorageConfig{
			WorkDir: tmpDir,
		},
		Context: config.ContextConfig{
			MaxMessages:   10,
			MaxTokens:     1000,
			SummarizeWhen: 5,
		},
		Agent: config.AgentConfig{
			Name:         "test-agent",
			SystemPrompt: "Test prompt",
		},
	}

	ag, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Replace provider with mock
	ag.provider = &mockProvider{response: "Hello! How can I help you?"}

	// Set conversation
	if err := ag.SetConversation("test-chat"); err != nil {
		t.Fatalf("failed to set conversation: %v", err)
	}

	// Test Chat
	resp, err := ag.Chat(context.Background(), "Hello")
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp != "Hello! How can I help you?" {
		t.Errorf("unexpected response: %s", resp)
	}

	// Verify conversation was saved
	store, _ := storage.NewJSONStore(tmpDir)
	conv, err := store.LoadConversation("test-chat")
	if err != nil {
		t.Fatalf("failed to load conversation: %v", err)
	}

	if len(conv.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(conv.Messages))
	}
	if conv.Messages[0].Role != "user" {
		t.Error("first message should be from user")
	}
	if conv.Messages[1].Role != "assistant" {
		t.Error("second message should be from assistant")
	}
}

func TestChatStream(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Provider: config.ProviderConfig{
			Type:    "openai",
			APIKey:  "test-key",
			BaseURL: "https://api.example.com/v1",
			Model:   "test-model",
		},
		Storage: config.StorageConfig{
			WorkDir: tmpDir,
		},
		Context: config.ContextConfig{
			MaxMessages:   10,
			MaxTokens:     1000,
			SummarizeWhen: 5,
		},
		Agent: config.AgentConfig{
			Name:         "test-agent",
			SystemPrompt: "Test prompt",
		},
	}

	ag, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Replace provider with mock
	ag.provider = &mockProvider{response: "Streaming response"}

	// Set conversation
	if err := ag.SetConversation("test-stream"); err != nil {
		t.Fatalf("failed to set conversation: %v", err)
	}

	var chunks []string
	resp, err := ag.ChatStream(context.Background(), "Hello", func(chunk string) {
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatalf("ChatStream() error = %v", err)
	}

	if resp != "Streaming response" {
		t.Errorf("unexpected response: %s", resp)
	}

	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}
}

func TestChatStream_WithToolCalls(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Provider: config.ProviderConfig{
			Type:    "openai",
			APIKey:  "test-key",
			BaseURL: "https://api.example.com/v1",
			Model:   "test-model",
		},
		Storage: config.StorageConfig{
			WorkDir: tmpDir,
		},
		Context: config.ContextConfig{
			MaxMessages:   10,
			MaxTokens:     1000,
			SummarizeWhen: 5,
		},
		Agent: config.AgentConfig{
			Name:         "test-agent",
			SystemPrompt: "Test prompt",
		},
	}

	ag, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Replace provider with mock that returns tool calls first
	ag.provider = &mockProvider{
		toolCalls: []llm.ToolCall{
			{
				ID:   "call-1",
				Type: "function",
				Function: &llm.ToolCallFunction{
					Name:      "echo",
					Arguments: `{"text": "test"}`,
				},
			},
		},
		response: "Tool executed successfully!",
	}

	// Set conversation
	if err := ag.SetConversation("test-tools"); err != nil {
		t.Fatalf("failed to set conversation: %v", err)
	}

	resp, err := ag.ChatStream(context.Background(), "Use the echo tool", nil)
	if err != nil {
		t.Fatalf("ChatStream() error = %v", err)
	}

	if resp != "Tool executed successfully!" {
		t.Errorf("unexpected response: %s", resp)
	}
}

func TestChatStream_MaxIterations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Provider: config.ProviderConfig{
			Type:    "openai",
			APIKey:  "test-key",
			BaseURL: "https://api.example.com/v1",
			Model:   "test-model",
		},
		Storage: config.StorageConfig{
			WorkDir: tmpDir,
		},
		Context: config.ContextConfig{
			MaxMessages:   10,
			MaxTokens:     1000,
			SummarizeWhen: 5,
		},
		Agent: config.AgentConfig{
			Name:         "test-agent",
			SystemPrompt: "Test prompt",
		},
	}

	ag, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Replace provider with mock that ALWAYS returns tool calls (infinite loop)
	ag.provider = &mockProviderAlwaysToolCalls{}

	// Set conversation
	if err := ag.SetConversation("test-max-iter"); err != nil {
		t.Fatalf("failed to set conversation: %v", err)
	}

	_, err = ag.ChatStream(context.Background(), "Loop forever", nil)
	if err == nil {
		t.Error("expected error for max iterations reached")
	}
	if !strings.Contains(err.Error(), "max tool iterations") {
		t.Errorf("unexpected error: %v", err)
	}
}

// mockProviderAlwaysToolCalls always returns tool calls, never a final response
type mockProviderAlwaysToolCalls struct{}

func (m *mockProviderAlwaysToolCalls) Complete(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	return m.CompleteWithOptions(ctx, messages, nil)
}

func (m *mockProviderAlwaysToolCalls) CompleteWithOptions(ctx context.Context, messages []llm.Message, opts *llm.CompleteOptions) (*llm.Response, error) {
	return &llm.Response{
		ToolCalls: []llm.ToolCall{
			{
				ID:   "call-loop",
				Type: "function",
				Function: &llm.ToolCallFunction{
					Name:      "echo",
					Arguments: `{"text": "loop"}`,
				},
			},
		},
	}, nil
}

func (m *mockProviderAlwaysToolCalls) Stream(ctx context.Context, messages []llm.Message, onChunk func(string)) error {
	onChunk("streamed")
	return nil
}

func (m *mockProviderAlwaysToolCalls) CountTokens(messages []llm.Message) int {
	return len(messages) * 10
}

func TestDeleteConversation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Provider: config.ProviderConfig{
			Type:    "openai",
			APIKey:  "test-key",
			BaseURL: "https://api.example.com/v1",
			Model:   "test-model",
		},
		Storage: config.StorageConfig{
			WorkDir: tmpDir,
		},
		Context: config.ContextConfig{
			MaxMessages:   10,
			MaxTokens:     1000,
			SummarizeWhen: 5,
		},
		Agent: config.AgentConfig{
			Name:         "test-agent",
			SystemPrompt: "Test prompt",
		},
	}

	ag, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Create conversation
	if err := ag.SetConversation("to-delete"); err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	// Delete it
	if err := ag.DeleteConversation("to-delete"); err != nil {
		t.Fatalf("DeleteConversation() error = %v", err)
	}

	// Verify it's gone
	store, _ := storage.NewJSONStore(tmpDir)
	_, err = store.LoadConversation("to-delete")
	if err == nil {
		t.Error("expected error loading deleted conversation")
	}
}

func TestSetConversation_EmptyID(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Provider: config.ProviderConfig{
			Type:    "openai",
			APIKey:  "test-key",
			BaseURL: "https://api.example.com/v1",
			Model:   "test-model",
		},
		Storage: config.StorageConfig{
			WorkDir: tmpDir,
		},
		Context: config.ContextConfig{
			MaxMessages:   10,
			MaxTokens:     1000,
			SummarizeWhen: 5,
		},
		Agent: config.AgentConfig{
			Name:         "test-agent",
			SystemPrompt: "Test prompt",
		},
	}

	ag, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Empty ID should use "default"
	if err := ag.SetConversation(""); err != nil {
		t.Fatalf("SetConversation('') error = %v", err)
	}

	if ag.conversationID != "default" {
		t.Errorf("expected conversationID 'default', got %s", ag.conversationID)
	}
}

// mockProviderWithCustomBehavior allows customizing responses
type mockProviderWithCustomBehavior struct {
	responses      []*llm.Response
	responseIndex  int
	completeError  error
	completeCalled int
}

func (m *mockProviderWithCustomBehavior) Complete(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	return m.CompleteWithOptions(ctx, messages, nil)
}

func (m *mockProviderWithCustomBehavior) CompleteWithOptions(ctx context.Context, messages []llm.Message, opts *llm.CompleteOptions) (*llm.Response, error) {
	m.completeCalled++
	if m.completeError != nil {
		return nil, m.completeError
	}
	if m.responseIndex < len(m.responses) {
		resp := m.responses[m.responseIndex]
		m.responseIndex++
		return resp, nil
	}
	return &llm.Response{Content: fmt.Sprintf("response-%d", m.responseIndex)}, nil
}

func (m *mockProviderWithCustomBehavior) Stream(ctx context.Context, messages []llm.Message, onChunk func(string)) error {
	onChunk("streamed response")
	return nil
}

func (m *mockProviderWithCustomBehavior) CountTokens(messages []llm.Message) int {
	return len(messages) * 10
}

func TestChat_CompleteError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Provider: config.ProviderConfig{
			Type:    "openai",
			APIKey:  "test-key",
			BaseURL: "https://api.example.com/v1",
			Model:   "test-model",
		},
		Storage: config.StorageConfig{
			WorkDir: tmpDir,
		},
		Context: config.ContextConfig{
			MaxMessages:   10,
			MaxTokens:     1000,
			SummarizeWhen: 5,
		},
		Agent: config.AgentConfig{
			Name:         "test-agent",
			SystemPrompt: "Test prompt",
		},
	}

	ag, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Replace provider with mock that returns error
	ag.provider = &mockProviderWithCustomBehavior{
		completeError: fmt.Errorf("API error"),
	}

	if err := ag.SetConversation("test-error"); err != nil {
		t.Fatalf("failed to set conversation: %v", err)
	}

	_, err = ag.Chat(context.Background(), "Hello")
	if err == nil {
		t.Error("expected error from Chat")
	}
	if !strings.Contains(err.Error(), "API error") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestChat_MultipleToolCalls(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Provider: config.ProviderConfig{
			Type:    "openai",
			APIKey:  "test-key",
			BaseURL: "https://api.example.com/v1",
			Model:   "test-model",
		},
		Storage: config.StorageConfig{
			WorkDir: tmpDir,
		},
		Context: config.ContextConfig{
			MaxMessages:   10,
			MaxTokens:     1000,
			SummarizeWhen: 5,
		},
		Agent: config.AgentConfig{
			Name:         "test-agent",
			SystemPrompt: "Test prompt",
		},
	}

	ag, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Provider that returns multiple tool calls then a response
	ag.provider = &mockProviderWithCustomBehavior{
		responses: []*llm.Response{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call-1",
						Type: "function",
						Function: &llm.ToolCallFunction{
							Name:      "echo",
							Arguments: `{"text": "first"}`,
						},
					},
					{
						ID:   "call-2",
						Type: "function",
						Function: &llm.ToolCallFunction{
							Name:      "echo",
							Arguments: `{"text": "second"}`,
						},
					},
				},
			},
			{
				Content: "All tools executed!",
			},
		},
	}

	if err := ag.SetConversation("test-multi-tools"); err != nil {
		t.Fatalf("failed to set conversation: %v", err)
	}

	resp, err := ag.Chat(context.Background(), "Use multiple tools")
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp != "All tools executed!" {
		t.Errorf("unexpected response: %s", resp)
	}
}

func TestChat_ToolCallWithNilFunction(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Provider: config.ProviderConfig{
			Type:    "openai",
			APIKey:  "test-key",
			BaseURL: "https://api.example.com/v1",
			Model:   "test-model",
		},
		Storage: config.StorageConfig{
			WorkDir: tmpDir,
		},
		Context: config.ContextConfig{
			MaxMessages:   10,
			MaxTokens:     1000,
			SummarizeWhen: 5,
		},
		Agent: config.AgentConfig{
			Name:         "test-agent",
			SystemPrompt: "Test prompt",
		},
	}

	ag, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Provider that returns tool call with nil function (edge case)
	ag.provider = &mockProviderWithCustomBehavior{
		responses: []*llm.Response{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:       "call-nil",
						Type:     "function",
						Function: nil, // nil function should be skipped
					},
					{
						ID:   "call-valid",
						Type: "function",
						Function: &llm.ToolCallFunction{
							Name:      "echo",
							Arguments: `{"text": "test"}`,
						},
					},
				},
			},
			{
				Content: "Done!",
			},
		},
	}

	if err := ag.SetConversation("test-nil-function"); err != nil {
		t.Fatalf("failed to set conversation: %v", err)
	}

	resp, err := ag.Chat(context.Background(), "Test nil function")
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp != "Done!" {
		t.Errorf("unexpected response: %s", resp)
	}
}
