package agent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/igm/igent/internal/config"
	"github.com/igm/igent/internal/llm"
	"github.com/igm/igent/internal/memory"
	"github.com/igm/igent/internal/skills"
	"github.com/igm/igent/internal/storage"
)

// Agent represents the AI agent
type Agent struct {
	config         *config.Config
	provider       llm.Provider
	store          *storage.JSONStore
	memory         *memory.Manager
	skills         *skills.Registry
	conversationID string
}

// New creates a new agent instance
func New(cfg *config.Config) (*Agent, error) {
	// Ensure working directory exists
	if err := cfg.EnsureWorkDir(); err != nil {
		return nil, fmt.Errorf("creating work directory: %w", err)
	}

	// Initialize storage
	store, err := storage.NewJSONStore(cfg.Storage.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("initializing storage: %w", err)
	}

	// Initialize LLM provider
	provider, err := llm.New(llm.ProviderConfig{
		Type:    cfg.Provider.Type,
		BaseURL: cfg.Provider.BaseURL,
		APIKey:  cfg.Provider.APIKey,
		Model:   cfg.Provider.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("initializing provider: %w", err)
	}

	// Initialize memory manager
	memMgr := memory.NewManager(store, provider,
		cfg.Context.MaxMessages,
		cfg.Context.MaxTokens,
		cfg.Context.SummarizeWhen,
	)

	// Initialize skill registry
	skillRegistry, err := skills.NewRegistry(store)
	if err != nil {
		return nil, fmt.Errorf("initializing skills: %w", err)
	}

	// Initialize default skills
	if err := skillRegistry.InitializeDefaults(); err != nil {
		return nil, fmt.Errorf("loading default skills: %w", err)
	}

	return &Agent{
		config:   cfg,
		provider: provider,
		store:    store,
		memory:   memMgr,
		skills:   skillRegistry,
	}, nil
}

// SetConversation sets or creates a conversation
func (a *Agent) SetConversation(id string) error {
	if id == "" {
		id = "default"
	}

	a.conversationID = id

	// Check if conversation exists, create if not
	_, err := a.store.LoadConversation(id)
	if err == storage.ErrNotFound {
		conv := &storage.Conversation{
			ID:        id,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Messages:  []llm.Message{},
		}
		return a.store.SaveConversation(conv)
	}

	return err
}

// buildSystemPrompt constructs the system prompt with dynamic information
func (a *Agent) buildSystemPrompt() string {
	now := time.Now()
	dateTime := now.Format("Monday, January 2, 2006 at 3:04 PM MST")

	prompt := a.config.Agent.SystemPrompt
	prompt += fmt.Sprintf("\n\nCurrent date and time: %s", dateTime)

	return prompt
}

// Chat sends a message and returns the response
func (a *Agent) Chat(ctx context.Context, userInput string) (string, error) {
	return a.ChatStream(ctx, userInput, nil)
}

// ChatStream sends a message and streams the response
func (a *Agent) ChatStream(ctx context.Context, userInput string, onChunk func(string)) (string, error) {
	// Load current conversation
	conv, err := a.store.LoadConversation(a.conversationID)
	if err != nil {
		return "", fmt.Errorf("loading conversation: %w", err)
	}

	// Build context with memory optimization
	messages, err := a.memory.BuildContext(conv, userInput)
	if err != nil {
		return "", fmt.Errorf("building context: %w", err)
	}

	// Build system prompt with current date/time
	systemPrompt := a.buildSystemPrompt()
	systemPrompt = a.skills.EnhancePrompt(userInput, systemPrompt)

	fullMessages := []llm.Message{{Role: "system", Content: systemPrompt}}
	fullMessages = append(fullMessages, messages...)

	// Get response from LLM
	var response string

	if onChunk != nil {
		// Streaming mode
		var fullResponse strings.Builder
		err = a.provider.Stream(ctx, fullMessages, func(chunk string) {
			fullResponse.WriteString(chunk)
			onChunk(chunk)
		})
		response = fullResponse.String()
	} else {
		// Non-streaming mode
		resp, err := a.provider.Complete(ctx, fullMessages)
		if err != nil {
			return "", fmt.Errorf("LLM completion: %w", err)
		}
		response = resp.Content
	}

	if err != nil {
		return "", fmt.Errorf("LLM error: %w", err)
	}

	// Save messages to conversation
	conv.Messages = append(conv.Messages,
		llm.Message{Role: "user", Content: userInput},
		llm.Message{Role: "assistant", Content: response},
	)

	if err := a.store.SaveConversation(conv); err != nil {
		return "", fmt.Errorf("saving conversation: %w", err)
	}

	return response, nil
}

// ListConversations returns all conversation IDs
func (a *Agent) ListConversations() ([]string, error) {
	return a.store.ListConversations()
}

// DeleteConversation removes a conversation
func (a *Agent) DeleteConversation(id string) error {
	return a.store.DeleteConversation(id)
}

// AddMemory adds a new memory
func (a *Agent) AddMemory(content, memType string) error {
	return a.memory.AddMemory(content, memType)
}

// ListMemories returns all memories
func (a *Agent) ListMemories() ([]*storage.MemoryItem, error) {
	return a.store.LoadMemories()
}

// DeleteMemory removes a memory
func (a *Agent) DeleteMemory(id string) error {
	return a.store.DeleteMemory(id)
}

// ListSkills returns all skills
func (a *Agent) ListSkills() []*storage.Skill {
	return a.skills.List()
}

// RegisterSkill adds a new skill
func (a *Agent) RegisterSkill(skill *storage.Skill) error {
	return a.skills.Register(skill)
}

// UnregisterSkill removes a skill
func (a *Agent) UnregisterSkill(id string) error {
	return a.skills.Unregister(id)
}

// Interactive starts an interactive REPL session
func (a *Agent) Interactive(ctx context.Context) error {
	fmt.Printf("%s ready. Type your message (Ctrl+C to exit).\n", a.config.Agent.Name)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nGoodbye!")
		os.Exit(0)
	}()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Handle special commands
		if strings.HasPrefix(input, "/") {
			a.handleCommand(ctx, input)
			continue
		}

		// Send to LLM and stream response
		fmt.Print("\n")
		_, err := a.ChatStream(ctx, input, func(chunk string) {
			fmt.Print(chunk)
		})
		if err != nil {
			fmt.Printf("\nError: %v\n", err)
			continue
		}
		fmt.Print("\n\n")
	}

	return scanner.Err()
}

// handleCommand processes slash commands
func (a *Agent) handleCommand(ctx context.Context, input string) {
	parts := strings.Fields(input)
	cmd := parts[0]

	switch cmd {
	case "/help":
		fmt.Println(`Commands:
  /help          - Show this help
  /new [name]    - Start a new conversation
  /list          - List conversations
  /switch <id>   - Switch to a conversation
  /delete <id>   - Delete a conversation
  /memory        - List memories
  /memory add <type> <content> - Add memory
  /skills        - List skills
  /clear         - Clear screen
  /exit          - Exit`)

	case "/new":
		name := "default"
		if len(parts) > 1 {
			name = parts[1]
		}
		if err := a.SetConversation(name); err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("Started new conversation: %s\n", name)
		}

	case "/list":
		convs, err := a.ListConversations()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			break
		}
		fmt.Println("Conversations:")
		for _, c := range convs {
			marker := ""
			if c == a.conversationID {
				marker = " *"
			}
			fmt.Printf("  %s%s\n", c, marker)
		}

	case "/switch":
		if len(parts) < 2 {
			fmt.Println("Usage: /switch <conversation-id>")
			break
		}
		if err := a.SetConversation(parts[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("Switched to: %s\n", parts[1])
		}

	case "/delete":
		if len(parts) < 2 {
			fmt.Println("Usage: /delete <conversation-id>")
			break
		}
		if err := a.DeleteConversation(parts[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("Deleted: %s\n", parts[1])
		}

	case "/memory":
		if len(parts) > 1 && parts[1] == "add" {
			if len(parts) < 4 {
				fmt.Println("Usage: /memory add <type> <content>")
				break
			}
			memType := parts[2]
			content := strings.Join(parts[3:], " ")
			if err := a.AddMemory(content, memType); err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Println("Memory added")
			}
			break
		}
		memories, err := a.ListMemories()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			break
		}
		fmt.Println("Memories:")
		for _, m := range memories {
			fmt.Printf("  [%s] %s\n", m.Type, m.Content)
		}

	case "/skills":
		skills := a.ListSkills()
		fmt.Println("Skills:")
		for _, s := range skills {
			fmt.Printf("  %s: %s\n", s.Name, s.Description)
		}

	case "/clear":
		fmt.Print("\033[2J\033[H")

	case "/exit":
		fmt.Println("Goodbye!")
		os.Exit(0)

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
	}
}
