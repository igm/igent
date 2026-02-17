package agent

import (
	"context"
	"os"
	"testing"

	"github.com/igm/igent/internal/config"
	"github.com/igm/igent/internal/llm"
	"github.com/igm/igent/internal/storage"
)

// mockProvider for testing
type mockProvider struct {
	response string
}

func (m *mockProvider) Complete(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	return &llm.Response{Content: m.response}, nil
}

func (m *mockProvider) Stream(ctx context.Context, messages []llm.Message, onChunk func(string)) error {
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
