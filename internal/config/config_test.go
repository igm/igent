package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Provider.Type != "openai" {
		t.Errorf("expected default provider type 'openai', got %s", cfg.Provider.Type)
	}

	if cfg.Context.MaxMessages <= 0 {
		t.Error("max messages should be positive")
	}

	if cfg.Context.MaxTokens <= 0 {
		t.Error("max tokens should be positive")
	}
}

func TestEnsureWorkDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &Config{
		Storage: StorageConfig{
			WorkDir: filepath.Join(tmpDir, "work"),
		},
	}

	if err := cfg.EnsureWorkDir(); err != nil {
		t.Fatalf("failed to ensure work dir: %v", err)
	}

	if _, err := os.Stat(cfg.Storage.WorkDir); os.IsNotExist(err) {
		t.Error("work directory not created")
	}
}

func TestLoadWithoutConfigFile(t *testing.T) {
	// Should load config (either from file or defaults)
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg == nil {
		t.Error("config is nil")
	}

	// Verify essential fields are set
	if cfg.Provider.Type == "" {
		t.Error("provider type should not be empty")
	}

	if cfg.Storage.WorkDir == "" {
		t.Error("work dir should not be empty")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "igent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &Config{
		Provider: ProviderConfig{
			Type:    "zhipu",
			BaseURL: "https://api.example.com",
			Model:   "test-model",
		},
		Storage: StorageConfig{
			WorkDir: tmpDir,
		},
		Context: ContextConfig{
			MaxMessages:   20,
			MaxTokens:     2000,
			SummarizeWhen: 15,
		},
		Agent: AgentConfig{
			Name:         "test-agent",
			SystemPrompt: "Test prompt",
		},
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loaded, err := Load(cfg.ConfigPath())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.Provider.Type != cfg.Provider.Type {
		t.Errorf("expected provider type %s, got %s", cfg.Provider.Type, loaded.Provider.Type)
	}

	if loaded.Agent.Name != cfg.Agent.Name {
		t.Errorf("expected agent name %s, got %s", cfg.Agent.Name, loaded.Agent.Name)
	}
}
