package agent

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/chzyer/readline"
	"github.com/igm/igent/internal/config"
	"github.com/igm/igent/internal/llm"
	"github.com/igm/igent/internal/logger"
	"github.com/igm/igent/internal/memory"
	"github.com/igm/igent/internal/skills"
	"github.com/igm/igent/internal/storage"
	"github.com/igm/igent/internal/tools"
)

// ErrToolDenied is returned when user denies tool execution
var ErrToolDenied = fmt.Errorf("tool execution denied by user")

// ToolConfirmationFunc is called before executing a tool to get user confirmation.
// Returns true to allow execution, false to deny.
type ToolConfirmationFunc func(call *tools.ToolCall) bool

// Agent represents the AI agent
type Agent struct {
	config         *config.Config
	provider       llm.Provider
	store          *storage.JSONStore
	memory         *memory.Manager
	skills         *skills.Registry
	tools          *tools.Registry
	conversationID string
	log            *slog.Logger

	// onToolConfirm is called before each tool execution for user confirmation
	onToolConfirm ToolConfirmationFunc
}

// New creates a new agent instance
func New(cfg *config.Config) (*Agent, error) {
	log := logger.L().With("component", "agent")

	// Initialize logger with config
	logger.Init(logger.Config{
		Level:  logger.Level(cfg.Logging.Level),
		Format: logger.Format(cfg.Logging.Format),
	}, nil)
	log = logger.L().With("component", "agent")

	log.Debug("initializing agent", "name", cfg.Agent.Name)

	// Ensure working directory exists
	if err := cfg.EnsureWorkDir(); err != nil {
		return nil, fmt.Errorf("creating work directory: %w", err)
	}
	log.Debug("work directory ensured", "path", cfg.Storage.WorkDir)

	// Initialize storage
	store, err := storage.NewJSONStore(cfg.Storage.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("initializing storage: %w", err)
	}
	log.Debug("storage initialized")

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
	log.Info("LLM provider initialized", "type", cfg.Provider.Type, "model", cfg.Provider.Model)

	// Initialize memory manager
	memMgr := memory.NewManager(store, provider,
		cfg.Context.MaxMessages,
		cfg.Context.MaxTokens,
		cfg.Context.SummarizeWhen,
	)
	log.Debug("memory manager initialized",
		"max_messages", cfg.Context.MaxMessages,
		"max_tokens", cfg.Context.MaxTokens,
		"summarize_when", cfg.Context.SummarizeWhen,
	)

	// Initialize skill registry
	skillRegistry, err := skills.NewRegistry(store)
	if err != nil {
		return nil, fmt.Errorf("initializing skills: %w", err)
	}
	log.Debug("skill registry initialized")

	// Initialize default skills
	if err := skillRegistry.InitializeDefaults(); err != nil {
		return nil, fmt.Errorf("loading default skills: %w", err)
	}

	// Initialize tools registry
	toolRegistry := tools.NewRegistry()
	log.Debug("tools registry initialized", "tool_count", len(toolRegistry.List()))

	log.Info("agent ready", "name", cfg.Agent.Name)

	return &Agent{
		config:   cfg,
		provider: provider,
		store:    store,
		memory:   memMgr,
		skills:   skillRegistry,
		tools:    toolRegistry,
		log:      log,
	}, nil
}

// SetToolConfirmation sets the callback function for tool confirmation
func (a *Agent) SetToolConfirmation(fn ToolConfirmationFunc) {
	a.onToolConfirm = fn
}

// FormatToolCall formats a tool call for display, showing the exact command/payload
func FormatToolCall(call *tools.ToolCall) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("\n\033[1;33m━━━ Tool Call ━━━\033[0m\n"))
	sb.WriteString(fmt.Sprintf("\033[1;36mTool:\033[0m %s\n", call.Name))

	// Format arguments nicely
	if len(call.Args) > 0 {
		sb.WriteString("\033[1;36mPayload:\033[0m\n")
		for key, val := range call.Args {
			sb.WriteString(fmt.Sprintf("  %s: %v\n", key, val))
		}
	}

	// For shell tool, show the actual command prominently
	if call.Name == "shell" {
		if cmd, ok := call.Args["command"].(string); ok {
			sb.WriteString(fmt.Sprintf("\n\033[1;32m▶ Executing:\033[0m %s\n", cmd))
		}
	}

	return sb.String()
}

// DefaultToolConfirmation is the default confirmation function for interactive mode
func DefaultToolConfirmation(call *tools.ToolCall) bool {
	fmt.Print(FormatToolCall(call))
	fmt.Print("\033[1;33mAllow execution? [y/N]: \033[0m")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
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
		a.log.Info("creating new conversation", "id", id)
		conv := &storage.Conversation{
			ID:        id,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Messages:  []llm.Message{},
		}
		if err := a.store.SaveConversation(conv); err != nil {
			return err
		}
		a.log.Debug("conversation created", "id", id)
		return nil
	}

	if err != nil {
		return err
	}

	a.log.Debug("conversation loaded", "id", id)
	return nil
}

// buildSystemPrompt constructs the system prompt with dynamic information
func (a *Agent) buildSystemPrompt() string {
	now := time.Now()
	dateTime := now.Format("Monday, January 2, 2006 at 3:04 PM MST")

	prompt := a.config.Agent.SystemPrompt
	prompt += fmt.Sprintf("\n\nCurrent date and time: %s", dateTime)

	a.log.Debug("system prompt built", "datetime", dateTime)

	return prompt
}

// Chat sends a message and returns the response
func (a *Agent) Chat(ctx context.Context, userInput string) (string, error) {
	return a.ChatStream(ctx, userInput, nil)
}

// ChatStream sends a message and streams the response
func (a *Agent) ChatStream(ctx context.Context, userInput string, onChunk func(string)) (string, error) {
	a.log.Debug("chat request started", "input_length", len(userInput))

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
	a.log.Debug("context built", "message_count", len(messages))

	// Build system prompt with current date/time
	systemPrompt := a.buildSystemPrompt()
	systemPrompt = a.skills.EnhancePrompt(userInput, systemPrompt)
	a.log.Debug("prompt enhanced with skills")

	fullMessages := []llm.Message{{Role: "system", Content: systemPrompt}}
	fullMessages = append(fullMessages, messages...)

	// Add user message
	fullMessages = append(fullMessages, llm.Message{Role: "user", Content: userInput})

	// Build tool definitions
	toolDefs := a.buildToolDefinitions()
	a.log.Debug("tools prepared", "tool_count", len(toolDefs))

	// Agentic loop: keep calling LLM until we get a text response
	maxIterations := 10
	iteration := 0
	var response string
	var toolCallsMade []llm.ToolCall

	startTime := time.Now()

	for iteration < maxIterations {
		iteration++
		a.log.Debug("agent loop iteration", "iteration", iteration)

		// Get response from LLM with tools
		opts := &llm.CompleteOptions{Tools: toolDefs}
		resp, err := a.provider.CompleteWithOptions(ctx, fullMessages, opts)
		if err != nil {
			return "", fmt.Errorf("LLM completion: %w", err)
		}

		// If no tool calls, we have our final response
		if !resp.HasToolCalls() {
			response = resp.Content
			break
		}

		// Handle tool calls
		a.log.Info("processing tool calls", "count", len(resp.ToolCalls))
		toolCallsMade = resp.ToolCalls

		// Add assistant message with tool calls to conversation
		fullMessages = append(fullMessages, llm.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute each tool and add result to messages
		for _, tc := range resp.ToolCalls {
			if tc.Function == nil {
				continue
			}

			// Parse tool call
			call, err := tools.ParseToolCall(tc.ID, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				a.log.Error("failed to parse tool call", "error", err)
				fullMessages = append(fullMessages, llm.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
					Content:    fmt.Sprintf("Error parsing tool arguments: %v", err),
				})
				continue
			}

			// Request confirmation before execution
			if a.onToolConfirm != nil {
				if !a.onToolConfirm(call) {
					// User denied execution - stop and return to input
					return "", ErrToolDenied
				}
			}

			// Execute tool
			result := a.tools.Execute(ctx, call)

			// Format result for LLM
			var resultContent string
			if result.Error != "" {
				resultContent = fmt.Sprintf("Error: %s", result.Error)
			} else {
				resultContent = result.Output
			}

			a.log.Info("tool executed",
				"tool", call.Name,
				"success", result.Error == "",
				"output_length", len(resultContent),
			)

			// Add tool result to messages
			fullMessages = append(fullMessages, llm.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    resultContent,
			})
		}
	}

	if iteration >= maxIterations {
		return "", fmt.Errorf("max tool iterations reached (%d)", maxIterations)
	}

	duration := time.Since(startTime)
	a.log.Info("chat completed",
		"response_length", len(response),
		"iterations", iteration,
		"tool_calls", len(toolCallsMade),
		"duration_ms", duration.Milliseconds(),
	)

	// Stream the response if callback provided
	if onChunk != nil && response != "" {
		// For streaming, we already have the full response, so send it in chunks
		// In a real implementation, we might want to chunk this more naturally
		onChunk(response)
	}

	// Save messages to conversation
	// Note: We save the simplified version (user + assistant) for conversation history
	// The tool call details are kept in the session but simplified for storage
	conv.Messages = append(conv.Messages,
		llm.Message{Role: "user", Content: userInput},
		llm.Message{Role: "assistant", Content: response},
	)

	if err := a.store.SaveConversation(conv); err != nil {
		return "", fmt.Errorf("saving conversation: %w", err)
	}
	a.log.Debug("conversation saved", "total_messages", len(conv.Messages))

	return response, nil
}

// buildToolDefinitions converts tool registry to LLM tool definitions
func (a *Agent) buildToolDefinitions() []llm.ToolDefinition {
	toolList := a.tools.List()
	defs := make([]llm.ToolDefinition, len(toolList))

	for i, t := range toolList {
		defs[i] = llm.ToolDefinition{
			Type: "function",
			Function: &llm.ToolFunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		}
	}

	return defs
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
	a.log.Info("starting interactive session", "conversation", a.conversationID)

	// Set up default tool confirmation
	a.SetToolConfirmation(DefaultToolConfirmation)

	fmt.Printf("%s ready. Type your message (Ctrl+C or /exit to exit).\n", a.config.Agent.Name)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nGoodbye!")
		os.Exit(0)
	}()

	// Initialize readline with history support
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "> ",
		HistoryFile:     "/tmp/.igent_history",
		AutoComplete:    nil,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return fmt.Errorf("initializing readline: %w", err)
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			// Handle Ctrl+D (EOF) or Ctrl+C
			break
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		// Handle special commands
		if strings.HasPrefix(input, "/") {
			a.handleCommand(ctx, input, rl)
			continue
		}

		// Send to LLM and stream response
		fmt.Print("\n")
		_, err = a.ChatStream(ctx, input, func(chunk string) {
			fmt.Print(chunk)
		})
		if err != nil {
			if err == ErrToolDenied {
				// Tool denied - just return to prompt
				fmt.Print("\n\n")
				continue
			}
			fmt.Printf("\nError: %v\n", err)
			continue
		}
		fmt.Print("\n\n")
	}

	fmt.Println("Goodbye!")
	return nil
}

// handleCommand processes slash commands
func (a *Agent) handleCommand(ctx context.Context, input string, rl *readline.Instance) {
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
  /tools         - List available tools
  /clear         - Clear screen
  /exit          - Exit

Navigation:
  UP/DOWN arrows - Navigate through message history`)

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

	case "/tools":
		tools := a.tools.List()
		fmt.Println("Available Tools:")
		for _, t := range tools {
			fmt.Printf("  %s: %s\n", t.Name, t.Description)
		}

	case "/clear":
		fmt.Print("\033[2J\033[H")

	case "/exit":
		rl.Close()
		fmt.Println("Goodbye!")
		os.Exit(0)

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
	}
}
