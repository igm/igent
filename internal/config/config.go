package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/igm/igent/internal/logger"
	"github.com/spf13/viper"
)

// Config holds all configuration for the agent
type Config struct {
	Provider ProviderConfig `mapstructure:"provider"`
	Storage  StorageConfig  `mapstructure:"storage"`
	Context  ContextConfig  `mapstructure:"context"`
	Agent    AgentConfig    `mapstructure:"agent"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

// ProviderConfig holds LLM provider settings
type ProviderConfig struct {
	Type    string `mapstructure:"type"` // openai, zhipu, anthropic
	BaseURL string `mapstructure:"base_url"`
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`
}

// StorageConfig holds storage settings
type StorageConfig struct {
	WorkDir string `mapstructure:"work_dir"`
}

// ContextConfig holds context management settings
type ContextConfig struct {
	MaxMessages   int `mapstructure:"max_messages"`   // Max messages before summarization
	MaxTokens     int `mapstructure:"max_tokens"`     // Approximate max context tokens
	SummarizeWhen int `mapstructure:"summarize_when"` // Trigger summarization at this count
}

// AgentConfig holds general agent settings
type AgentConfig struct {
	SystemPrompt string `mapstructure:"system_prompt"`
	Name         string `mapstructure:"name"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string `mapstructure:"level"`  // debug, info, warn, error
	Format string `mapstructure:"format"` // text, json
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	workDir := filepath.Join(home, ".igent")

	return &Config{
		Provider: ProviderConfig{
			Type:    "openai",
			BaseURL: "https://api.openai.com/v1",
			Model:   "gpt-4o-mini",
		},
		Storage: StorageConfig{
			WorkDir: workDir,
		},
		Context: ContextConfig{
			MaxMessages:   50,
			MaxTokens:     4000,
			SummarizeWhen: 30,
		},
		Agent: AgentConfig{
			Name:         "igent",
			SystemPrompt: "You are a helpful AI assistant. Be concise and accurate.",
		},
		Logging: LoggingConfig{
			Level:  string(logger.LevelInfo),
			Format: string(logger.FormatText),
		},
	}
}

// Load reads configuration from file and environment
func Load(cfgFile string) (*Config, error) {
	cfg := DefaultConfig()

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		// Check multiple locations
		v.AddConfigPath(".")
		v.AddConfigPath(cfg.Storage.WorkDir)
		v.AddConfigPath("/etc/igent")
	}

	// Set defaults
	v.SetDefault("provider.type", cfg.Provider.Type)
	v.SetDefault("provider.base_url", cfg.Provider.BaseURL)
	v.SetDefault("provider.model", cfg.Provider.Model)
	v.SetDefault("storage.work_dir", cfg.Storage.WorkDir)
	v.SetDefault("context.max_messages", cfg.Context.MaxMessages)
	v.SetDefault("context.max_tokens", cfg.Context.MaxTokens)
	v.SetDefault("context.summarize_when", cfg.Context.SummarizeWhen)
	v.SetDefault("agent.name", cfg.Agent.Name)
	v.SetDefault("agent.system_prompt", cfg.Agent.SystemPrompt)
	v.SetDefault("logging.level", cfg.Logging.Level)
	v.SetDefault("logging.format", cfg.Logging.Format)

	// Environment variable overrides
	v.SetEnvPrefix("IGENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config: %w", err)
		}
		// Config file not found, use defaults
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Explicitly check for API key in environment (Viper nested env binding is unreliable)
	if cfg.Provider.APIKey == "" {
		if key := os.Getenv("IGENT_PROVIDER_API_KEY"); key != "" {
			cfg.Provider.APIKey = key
		} else if key := os.Getenv("IGENT_API_KEY"); key != "" {
			cfg.Provider.APIKey = key
		} else if key := os.Getenv("OPENAI_API_KEY"); key != "" {
			cfg.Provider.APIKey = key
		}
	}

	return cfg, nil
}

// EnsureWorkDir creates the working directory if it doesn't exist
func (c *Config) EnsureWorkDir() error {
	return os.MkdirAll(c.Storage.WorkDir, 0755)
}

// ConfigPath returns the path to config file
func (c *Config) ConfigPath() string {
	return filepath.Join(c.Storage.WorkDir, "config.yaml")
}

// Save writes the current config to file
func (c *Config) Save() error {
	if err := c.EnsureWorkDir(); err != nil {
		return err
	}

	// Use a map with explicit keys to preserve snake_case
	configMap := map[string]interface{}{
		"provider": map[string]interface{}{
			"type":     c.Provider.Type,
			"base_url": c.Provider.BaseURL,
			"api_key":  c.Provider.APIKey,
			"model":    c.Provider.Model,
		},
		"storage": map[string]interface{}{
			"work_dir": c.Storage.WorkDir,
		},
		"context": map[string]interface{}{
			"max_messages":   c.Context.MaxMessages,
			"max_tokens":     c.Context.MaxTokens,
			"summarize_when": c.Context.SummarizeWhen,
		},
		"agent": map[string]interface{}{
			"name":          c.Agent.Name,
			"system_prompt": c.Agent.SystemPrompt,
		},
		"logging": map[string]interface{}{
			"level":  c.Logging.Level,
			"format": c.Logging.Format,
		},
	}

	v := viper.New()
	v.SetConfigFile(c.ConfigPath())
	for key, value := range configMap {
		v.Set(key, value)
	}

	return v.WriteConfig()
}
