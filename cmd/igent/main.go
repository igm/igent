package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/igm/igent/internal/agent"
	"github.com/igm/igent/internal/config"
)

var (
	cfgFile     string
	convID      string
	streaming   bool
	showVersion bool

	version = "dev"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "igent [prompt]",
	Short: "AI Agent with persistent context",
	Long: `igent is an AI agent that maintains conversation history and memory
across sessions. It uses context optimization to keep conversations
relevant while staying within token limits.`,
	Args: cobra.ArbitraryArgs,
	RunE: runAgent,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is ~/.igent/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&convID, "conversation", "C", "default", "conversation ID")
	rootCmd.PersistentFlags().BoolVarP(&streaming, "stream", "s", true, "stream response")
	rootCmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "show version")

	// Subcommands
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(memoryCmd)
	rootCmd.AddCommand(skillCmd)
}

func runAgent(cmd *cobra.Command, args []string) error {
	if showVersion {
		fmt.Println("igent", version)
		return nil
	}

	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Create agent
	ag, err := agent.New(cfg)
	if err != nil {
		return fmt.Errorf("creating agent: %w", err)
	}

	// Set conversation
	if err := ag.SetConversation(convID); err != nil {
		return fmt.Errorf("setting conversation: %w", err)
	}

	ctx := context.Background()

	// Interactive mode if no prompt provided
	if len(args) == 0 {
		return ag.Interactive(ctx)
	}

	// Single message mode
	prompt := args[0]
	if len(args) > 1 {
		prompt = fmt.Sprintf("%s", args)
		for i, arg := range args {
			if i == 0 {
				prompt = arg
			} else {
				prompt += " " + arg
			}
		}
	}

	if streaming {
		_, err = ag.ChatStream(ctx, prompt, func(chunk string) {
			fmt.Print(chunk)
		})
		fmt.Println()
	} else {
		response, err := ag.Chat(ctx, prompt)
		if err != nil {
			return err
		}
		fmt.Println(response)
	}

	return err
}

// configCmd handles configuration
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.DefaultConfig()

		// Check for API key in environment
		if apiKey := os.Getenv("IGENT_API_KEY"); apiKey != "" {
			cfg.Provider.APIKey = apiKey
		} else if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
			cfg.Provider.APIKey = apiKey
		} else {
			fmt.Print("Enter API key: ")
			fmt.Scanln(&cfg.Provider.APIKey)
		}

		fmt.Print("Provider (openai/zhipu/glm) [openai]: ")
		var provider string
		fmt.Scanln(&provider)
		if provider != "" {
			cfg.Provider.Type = provider
		}

		fmt.Print("Model [gpt-4o-mini]: ")
		var model string
		fmt.Scanln(&model)
		if model != "" {
			cfg.Provider.Model = model
		}

		// Set provider defaults based on type
		switch cfg.Provider.Type {
		case "zhipu", "glm":
			if cfg.Provider.BaseURL == "" {
				cfg.Provider.BaseURL = "https://open.bigmodel.cn/api/paas/v4"
			}
			if model == "" {
				cfg.Provider.Model = "glm-4-flash"
			}
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Printf("Configuration saved to: %s\n", cfg.ConfigPath())
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return err
		}

		fmt.Printf("Provider: %s\n", cfg.Provider.Type)
		fmt.Printf("Base URL: %s\n", cfg.Provider.BaseURL)
		fmt.Printf("Model: %s\n", cfg.Provider.Model)
		fmt.Printf("Work Dir: %s\n", cfg.Storage.WorkDir)
		fmt.Printf("Max Messages: %d\n", cfg.Context.MaxMessages)
		fmt.Printf("Max Tokens: %d\n", cfg.Context.MaxTokens)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
}

// listCmd lists conversations
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List conversations",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return err
		}

		ag, err := agent.New(cfg)
		if err != nil {
			return err
		}

		convs, err := ag.ListConversations()
		if err != nil {
			return err
		}

		if len(convs) == 0 {
			fmt.Println("No conversations found")
			return nil
		}

		fmt.Println("Conversations:")
		for _, c := range convs {
			fmt.Printf("  %s\n", c)
		}
		return nil
	},
}

// memoryCmd manages memories
var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage agent memory",
}

var memoryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all memories",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return err
		}

		ag, err := agent.New(cfg)
		if err != nil {
			return err
		}

		memories, err := ag.ListMemories()
		if err != nil {
			return err
		}

		if len(memories) == 0 {
			fmt.Println("No memories found")
			return nil
		}

		fmt.Println("Memories:")
		for _, m := range memories {
			fmt.Printf("  [%s] %s (relevance: %.2f)\n", m.Type, m.Content, m.Relevance)
		}
		return nil
	},
}

var memoryAddCmd = &cobra.Command{
	Use:   "add <type> <content>",
	Short: "Add a memory",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return err
		}

		ag, err := agent.New(cfg)
		if err != nil {
			return err
		}

		memType := args[0]
		content := args[1]
		if len(args) > 2 {
			for i := 2; i < len(args); i++ {
				content += " " + args[i]
			}
		}

		if err := ag.AddMemory(content, memType); err != nil {
			return err
		}

		fmt.Println("Memory added successfully")
		return nil
	},
}

var memoryDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a memory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return err
		}

		ag, err := agent.New(cfg)
		if err != nil {
			return err
		}

		return ag.DeleteMemory(args[0])
	},
}

func init() {
	memoryCmd.AddCommand(memoryListCmd)
	memoryCmd.AddCommand(memoryAddCmd)
	memoryCmd.AddCommand(memoryDeleteCmd)
}

// skillCmd manages skills
var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage agent skills",
}

var skillListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return err
		}

		ag, err := agent.New(cfg)
		if err != nil {
			return err
		}

		skills := ag.ListSkills()
		if len(skills) == 0 {
			fmt.Println("No skills found")
			return nil
		}

		fmt.Println("Skills:")
		for _, s := range skills {
			status := "disabled"
			if s.Enabled {
				status = "enabled"
			}
			fmt.Printf("  %s (%s): %s\n", s.Name, status, s.Description)
		}
		return nil
	},
}

func init() {
	skillCmd.AddCommand(skillListCmd)
}
