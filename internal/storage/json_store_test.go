package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/igm/igent/internal/llm"
)

func TestNewJSONStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewJSONStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	if store == nil {
		t.Error("store is nil")
	}

	// Check subdirectories exist
	for _, sub := range []string{"messages", "memory", "skills"} {
		path := filepath.Join(tmpDir, sub)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("subdirectory %s not created", sub)
		}
	}
}

func TestConversationCRUD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewJSONStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create
	conv := &Conversation{
		ID:        "test-conv",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages: []llm.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		},
	}

	if err := store.SaveConversation(conv); err != nil {
		t.Fatalf("failed to save conversation: %v", err)
	}

	// Read
	loaded, err := store.LoadConversation("test-conv")
	if err != nil {
		t.Fatalf("failed to load conversation: %v", err)
	}

	if loaded.ID != conv.ID {
		t.Errorf("expected ID %s, got %s", conv.ID, loaded.ID)
	}

	if len(loaded.Messages) != len(conv.Messages) {
		t.Errorf("expected %d messages, got %d", len(conv.Messages), len(loaded.Messages))
	}

	// List
	ids, err := store.ListConversations()
	if err != nil {
		t.Fatalf("failed to list conversations: %v", err)
	}

	if len(ids) != 1 {
		t.Errorf("expected 1 conversation, got %d", len(ids))
	}

	// Delete
	if err := store.DeleteConversation("test-conv"); err != nil {
		t.Fatalf("failed to delete conversation: %v", err)
	}

	_, err = store.LoadConversation("test-conv")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryCRUD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewJSONStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create
	mem := &MemoryItem{
		ID:        "test-mem",
		Content:   "User prefers Go programming",
		Type:      "preference",
		CreatedAt: time.Now(),
		Relevance: 0.9,
	}

	if err := store.SaveMemory(mem); err != nil {
		t.Fatalf("failed to save memory: %v", err)
	}

	// Read all
	memories, err := store.LoadMemories()
	if err != nil {
		t.Fatalf("failed to load memories: %v", err)
	}

	if len(memories) != 1 {
		t.Errorf("expected 1 memory, got %d", len(memories))
	}

	if memories[0].Content != mem.Content {
		t.Errorf("expected content %s, got %s", mem.Content, memories[0].Content)
	}

	// Delete
	if err := store.DeleteMemory("test-mem"); err != nil {
		t.Fatalf("failed to delete memory: %v", err)
	}

	memories, _ = store.LoadMemories()
	if len(memories) != 0 {
		t.Errorf("expected 0 memories after delete, got %d", len(memories))
	}
}

func TestSkillCRUD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewJSONStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create
	skill := &Skill{
		ID:          "test-skill",
		Name:        "Test Skill",
		Description: "A test skill",
		Prompt:      "This is a test prompt",
		Enabled:     true,
	}

	if err := store.SaveSkill(skill); err != nil {
		t.Fatalf("failed to save skill: %v", err)
	}

	// Read all
	skills, err := store.LoadSkills()
	if err != nil {
		t.Fatalf("failed to load skills: %v", err)
	}

	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}

	if skills[0].Name != skill.Name {
		t.Errorf("expected name %s, got %s", skill.Name, skills[0].Name)
	}

	// Delete
	if err := store.DeleteSkill("test-skill"); err != nil {
		t.Fatalf("failed to delete skill: %v", err)
	}

	skills, _ = store.LoadSkills()
	if len(skills) != 0 {
		t.Errorf("expected 0 skills after delete, got %d", len(skills))
	}
}
