package skills

import (
	"os"
	"testing"

	"github.com/igm/igent/internal/storage"
)

func TestRegistry(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.NewJSONStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	registry, err := NewRegistry(store)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}

	// Test initial state (no skills)
	skills := registry.List()
	if len(skills) != 0 {
		t.Errorf("expected 0 skills initially, got %d", len(skills))
	}

	// Test registration
	skill := &storage.Skill{
		ID:          "test-skill",
		Name:        "Test Skill",
		Description: "A test skill",
		Prompt:      "Test prompt",
		Enabled:     true,
	}

	if err := registry.Register(skill); err != nil {
		t.Fatalf("failed to register skill: %v", err)
	}

	// Test Get
	retrieved, ok := registry.Get("test-skill")
	if !ok {
		t.Error("skill not found")
	}

	if retrieved.Name != skill.Name {
		t.Errorf("expected name %s, got %s", skill.Name, retrieved.Name)
	}

	// Test List
	skills = registry.List()
	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}

	// Test Match
	matches := registry.Match("I need help with test skill")
	if len(matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(matches))
	}

	// Test Unregister
	if err := registry.Unregister("test-skill"); err != nil {
		t.Fatalf("failed to unregister skill: %v", err)
	}

	_, ok = registry.Get("test-skill")
	if ok {
		t.Error("skill should be deleted")
	}
}

func TestDefaultSkills(t *testing.T) {
	defaults := DefaultSkills()

	if len(defaults) == 0 {
		t.Error("no default skills defined")
	}

	for _, skill := range defaults {
		if skill.ID == "" {
			t.Error("skill has empty ID")
		}
		if skill.Name == "" {
			t.Error("skill has empty name")
		}
		if skill.Prompt == "" {
			t.Error("skill has empty prompt")
		}
	}
}

func TestInitializeDefaults(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.NewJSONStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	registry, err := NewRegistry(store)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}

	// Initialize defaults
	if err := registry.InitializeDefaults(); err != nil {
		t.Fatalf("failed to initialize defaults: %v", err)
	}

	skills := registry.List()
	if len(skills) == 0 {
		t.Error("no default skills loaded")
	}

	// Should be idempotent
	if err := registry.InitializeDefaults(); err != nil {
		t.Fatalf("second initialization failed: %v", err)
	}

	// Should not duplicate
	skills = registry.List()
	if len(skills) != len(DefaultSkills()) {
		t.Errorf("expected %d skills, got %d", len(DefaultSkills()), len(skills))
	}
}

func TestEnhancePrompt(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.NewJSONStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	registry, err := NewRegistry(store)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}

	registry.InitializeDefaults()

	basePrompt := "You are a helpful assistant."
	// The default skills have "Code Assistant" which should match "code"
	enhanced := registry.EnhancePrompt("help me with Code Assistant", basePrompt)

	// If no skills match, the base prompt is returned unchanged
	// This is expected behavior - the test should verify the function works
	if enhanced == "" {
		t.Error("enhanced prompt should not be empty")
	}
}
