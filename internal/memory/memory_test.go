package memory

import (
	"context"
	"os"
	"testing"

	"github.com/igm/igent/internal/llm"
	"github.com/igm/igent/internal/storage"
)

// mockProvider implements llm.Provider for testing
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

func TestBuildContext(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.NewJSONStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	provider := &mockProvider{response: "test response"}
	mgr := NewManager(store, provider, 10, 1000, 5)

	conv := &storage.Conversation{
		ID:       "test",
		Messages: []llm.Message{},
	}

	context, err := mgr.BuildContext(conv, "Hello")
	if err != nil {
		t.Fatalf("failed to build context: %v", err)
	}

	if len(context) == 0 {
		t.Error("context should not be empty")
	}

	// Should include the user message
	lastMsg := context[len(context)-1]
	if lastMsg.Role != "user" || lastMsg.Content != "Hello" {
		t.Error("last message should be user's input")
	}
}

func TestGetRecentMessages(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.NewJSONStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	provider := &mockProvider{}
	mgr := NewManager(store, provider, 5, 1000, 10)

	// Create messages exceeding max
	messages := []llm.Message{}
	for i := 0; i < 10; i++ {
		messages = append(messages, llm.Message{
			Role:    "user",
			Content: "Message " + string(rune('0'+i)),
		})
	}

	recent := mgr.getRecentMessages(messages, "New message")

	// Should respect max messages limit
	if len(recent) > 6 { // 5 history + 1 new
		t.Errorf("expected at most 6 messages, got %d", len(recent))
	}

	// Should include new message at end
	if recent[len(recent)-1].Content != "New message" {
		t.Error("last message should be new user message")
	}
}

func TestAddMemory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.NewJSONStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	provider := &mockProvider{}
	mgr := NewManager(store, provider, 10, 1000, 5)

	if err := mgr.AddMemory("User prefers dark mode", "preference"); err != nil {
		t.Fatalf("failed to add memory: %v", err)
	}

	memories, err := store.LoadMemories()
	if err != nil {
		t.Fatalf("failed to load memories: %v", err)
	}

	if len(memories) != 1 {
		t.Errorf("expected 1 memory, got %d", len(memories))
	}

	if memories[0].Content != "User prefers dark mode" {
		t.Errorf("unexpected memory content: %s", memories[0].Content)
	}
}

func TestGetRelevantMemories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.NewJSONStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Add some memories
	store.SaveMemory(&storage.MemoryItem{
		ID:        "1",
		Content:   "User likes Go programming",
		Type:      "preference",
		Relevance: 0.8,
	})
	store.SaveMemory(&storage.MemoryItem{
		ID:        "2",
		Content:   "User works at Acme Corp",
		Type:      "fact",
		Relevance: 0.5,
	})

	provider := &mockProvider{}
	mgr := NewManager(store, provider, 10, 1000, 5)

	// Query related to programming
	memories, err := mgr.getRelevantMemories("help me with programming")
	if err != nil {
		t.Fatalf("failed to get relevant memories: %v", err)
	}

	// Should match the programming preference
	if len(memories) == 0 {
		t.Error("expected at least one relevant memory")
	}
}
