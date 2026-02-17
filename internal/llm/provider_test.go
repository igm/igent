package llm

import (
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
