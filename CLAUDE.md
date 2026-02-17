# igent - AI Agent with Persistent Context

A simple, extensible AI agent implemented in Go that maintains conversation history and memory across sessions with intelligent context optimization.

## Architecture Overview

```
igent/
├── cmd/igent/main.go        # CLI entry point (Cobra)
├── internal/
│   ├── agent/agent.go       # Core agent logic, Chat, Interactive REPL
│   ├── config/config.go     # Viper-based configuration
│   ├── llm/
│   │   ├── provider.go      # Provider interface
│   │   ├── openai.go        # OpenAI-compatible HTTP client
│   │   └── zhipu.go         # Z.AI/GLM provider wrapper
│   ├── memory/memory.go     # Context optimization, summarization
│   ├── skills/skills.go     # Skill registry with pattern matching
│   └── storage/
│       ├── storage.go       # Storage interface
│       └── json_store.go    # JSON file persistence
├── Makefile
├── go.mod
├── README.md
└── CLAUDE.md
```

## Key Components

### 1. Agent (`internal/agent/`)

The core orchestrator that:
- Loads/saves conversations
- Builds context with memory optimization
- Constructs system prompts with current date/time
- Manages streaming and non-streaming responses
- Provides interactive REPL with slash commands

**System Prompt Construction:**
```go
func (a *Agent) buildSystemPrompt() string {
    now := time.Now()
    dateTime := now.Format("Monday, January 2, 2006 at 3:04 PM MST")
    prompt := a.config.Agent.SystemPrompt
    prompt += fmt.Sprintf("\n\nCurrent date and time: %s", dateTime)
    return prompt
}
```

### 2. LLM Provider (`internal/llm/`)

- **Interface**: `Provider` with `Complete`, `Stream`, `CountTokens` methods
- **Implementations**: OpenAI-compatible (works with OpenAI, Z.AI, GLM, etc.)
- **Registry pattern**: Add new providers via `Register(name, factory)`

**Adding a new provider:**
```go
func init() {
    llm.Register("myprovider", func(cfg llm.ProviderConfig) (llm.Provider, error) {
        return &MyProvider{...}, nil
    })
}
```

### 3. Storage (`internal/storage/`)

- **JSON-based persistence** in `~/.igent/`
- **Subdirectories**: `messages/`, `memory/`, `skills/`
- **Three data types**:
  - `Conversation`: Message history with summaries
  - `MemoryItem`: Persistent facts/preferences with relevance scores
  - `Skill`: Extensible agent capabilities

### 4. Memory Manager (`internal/memory/`)

- **Context window optimization**:
  - Sliding window for recent messages (respects `max_messages`)
  - Token budget awareness (respects `max_tokens`)
  - Automatic summarization when threshold (`summarize_when`) reached
  - Memory extraction from summarized conversations
- **Relevance scoring**: Keyword matching + stored relevance for memory retrieval

### 5. Skills (`internal/skills/`)

- **Dynamic skill loading** from storage
- **Pattern matching** for skill activation
- **Prompt enhancement**: Skills inject context into system prompt
- **Default skills**: `code`, `explain`, `summarize`

## Configuration

Location: `~/.igent/config.yaml`

**Important**: Config keys must use `snake_case` (e.g., `api_key`, `base_url`).

```yaml
provider:
  type: glm                        # openai, zhipu, glm
  base_url: https://api.z.ai/api/coding/paas/v4
  api_key: your-api-key-here
  model: glm-5

storage:
  work_dir: ~/.igent

context:
  max_messages: 50                 # Max messages in context window
  max_tokens: 4000                 # Token budget for context
  summarize_when: 30               # Trigger summarization at this count

agent:
  name: igent
  system_prompt: "You are a helpful AI assistant. Be concise and accurate."
```

### Environment Variables

API key is loaded in this order:
1. `IGENT_PROVIDER_API_KEY` - Explicit provider key
2. `IGENT_API_KEY` - Generic igent key
3. `OPENAI_API_KEY` - Fallback for OpenAI compatibility
4. Config file `provider.api_key`

Other:
- `IGENT_CONFIG`: Custom config file path

## Data Structures

### Conversation (`~/.igent/messages/<id>.json`)
```json
{
  "id": "default",
  "created_at": "2024-01-15T10:00:00Z",
  "updated_at": "2024-01-15T10:30:00Z",
  "messages": [
    {"role": "user", "content": "..."},
    {"role": "assistant", "content": "..."}
  ],
  "summary": "Previous conversation about..."
}
```

### Memory Item (`~/.igent/memory/<id>.json`)
```json
{
  "id": "abc123",
  "content": "User prefers Go programming",
  "type": "preference",
  "created_at": "2024-01-15T10:00:00Z",
  "relevance": 0.9
}
```

### Skill (`~/.igent/skills/<id>.json`)
```json
{
  "id": "code",
  "name": "Code Assistant",
  "description": "Helps with coding tasks",
  "prompt": "When discussing code...",
  "enabled": true
}
```

## CLI Usage

### Commands

```bash
# Interactive mode (REPL)
igent

# Single message
igent "What is the capital of France?"

# Specify conversation
igent -C work-chat "Continue our discussion"

# Flags
igent -c /path/to/config.yaml    # Custom config
igent -C my-conversation          # Conversation ID
igent -s                          # Stream response (default)
igent --stream=false              # Non-streaming
igent -v                          # Show version
```

### Management Commands

```bash
igent config init                 # Initialize config interactively
igent config show                 # Show current config

igent list                        # List all conversations

igent memory list                 # Show all memories
igent memory add preference "..." # Add memory
igent memory delete <id>          # Remove memory

igent skill list                  # List skills
```

### Interactive REPL Commands

```
> /help                 # Show all commands
> /new [name]           # Start new conversation
> /list                 # List conversations
> /switch <id>          # Switch to conversation
> /delete <id>          # Delete conversation
> /memory               # List memories
> /memory add <type> <content>  # Add memory (type: fact/preference/context)
> /skills               # List skills
> /clear                # Clear screen
> /exit                 # Exit
```

## Build Commands

```bash
make build          # Build binary
make install        # Install to GOBIN
make test           # Run tests
make clean          # Clean build artifacts
make build-all      # Cross-compile for darwin/linux/windows
make fmt            # Format code
make lint           # Run linter
```

## Context Optimization Strategy

1. **Token Budget**: Reserve tokens for system prompt + response
2. **Sliding Window**: Keep most recent messages within budget
3. **Summarization**: When message count > `summarize_when`:
   - Keep last 10 messages
   - Summarize older messages via LLM
   - Extract important facts as memories (async)
4. **Memory Retrieval**: Keyword matching with relevance boosting

## Supported Providers

| Provider | Type | Default Base URL |
|----------|------|------------------|
| OpenAI | `openai` | `https://api.openai.com/v1` |
| Z.AI | `zhipu` | `https://open.bigmodel.cn/api/paas/v4` |
| GLM | `glm` | Same as Z.AI |
| Custom | `openai` | Your URL |

## Development

```bash
# Run tests
go test ./...

# Run specific package tests
go test -v ./internal/memory/...

# Build
go build -o igent ./cmd/igent

# Install
go install ./cmd/igent
```
