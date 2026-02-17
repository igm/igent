# igent - AI Agent with Persistent Context

A simple, extensible AI agent implemented in Go that maintains conversation history and memory across sessions with intelligent context optimization.

## Architecture Overview

```
igent/
├── cmd/igent/           # CLI entry point
├── internal/
│   ├── agent/           # Core agent logic
│   ├── config/          # Configuration management
│   ├── llm/             # LLM provider abstraction
│   ├── memory/          # Context & memory optimization
│   ├── skills/          # Skill system
│   └── storage/         # Persistence layer
└── CLAUDE.md           # This file
```

## Key Components

### 1. LLM Provider (`internal/llm/`)
- **Interface**: `Provider` with `Complete`, `Stream`, `CountTokens` methods
- **Implementations**: OpenAI-compatible (works with OpenAI, Z.AI, GLM, etc.)
- **Registry pattern**: Add new providers via `Register(name, factory)`

### 2. Storage (`internal/storage/`)
- **JSON-based persistence** for portability
- **Three data types**:
  - `Conversation`: Message history with summaries
  - `MemoryItem`: Persistent facts/preferences with relevance scores
  - `Skill`: Extensible agent capabilities

### 3. Memory Manager (`internal/memory/`)
- **Context window optimization**:
  - Sliding window for recent messages
  - Automatic summarization when threshold reached
  - Memory extraction from old conversations
- **Relevance scoring** for memory retrieval

### 4. Skills (`internal/skills/`)
- **Dynamic skill loading** from storage
- **Pattern matching** for skill activation
- **Prompt enhancement** with skill context

## Configuration

Location: `~/.igent/config.yaml` (or `IGENT_CONFIG`)

```yaml
provider:
  type: openai          # openai, zhipu, glm
  base_url: https://api.openai.com/v1
  api_key: ${IGENT_API_KEY}
  model: gpt-4o-mini

storage:
  work_dir: ~/.igent

context:
  max_messages: 50      # Max messages in context
  max_tokens: 4000      # Token budget
  summarize_when: 30    # Trigger summarization

agent:
  name: igent
  system_prompt: "You are a helpful AI assistant."
```

## Data Structures

### Conversation
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

### Memory Item
```json
{
  "id": "abc123",
  "content": "User prefers Go programming",
  "type": "preference",  // fact, preference, context
  "created_at": "2024-01-15T10:00:00Z",
  "relevance": 0.9
}
```

### Skill
```json
{
  "id": "code",
  "name": "Code Assistant",
  "description": "Helps with coding tasks",
  "prompt": "When discussing code...",
  "enabled": true
}
```

## Usage Patterns

### CLI Commands
```bash
# Interactive mode
igent

# Single message
igent "What is the capital of France?"

# Specify conversation
igent -C work-chat "Continue our discussion"

# Stream response (default)
igent -s "Tell me a story"

# Non-streaming
igent --stream=false "Quick answer"
```

### Management Commands
```bash
# Configuration
igent config init       # Initialize config
igent config show       # Show current config

# Conversations
igent list              # List all conversations
igent -C new-chat       # Start new conversation

# Memory
igent memory list                    # Show memories
igent memory add preference "..."    # Add memory
igent memory delete <id>             # Remove memory

# Skills
igent skill list        # List skills
```

### Interactive Commands
```
> /help                 # Show commands
> /new work             # New conversation
> /list                 # List conversations
> /switch work          # Switch conversation
> /memory               # Show memories
> /memory add fact "..." # Add memory
> /skills               # List skills
> /clear                # Clear screen
> /exit                 # Exit
```

## Adding a New LLM Provider

1. Implement `llm.Provider` interface
2. Register in `init()`:
```go
func init() {
    llm.Register("myprovider", func(cfg llm.ProviderConfig) (llm.Provider, error) {
        return &MyProvider{...}, nil
    })
}
```

## Context Optimization Strategy

1. **Token Budget**: Reserve tokens for system prompt + response
2. **Sliding Window**: Keep most recent messages within budget
3. **Summarization**: When message count > `summarize_when`:
   - Keep last N messages
   - Summarize older messages via LLM
   - Extract important facts as memories
4. **Memory Retrieval**: Keyword matching with relevance boosting

## Environment Variables

- `IGENT_API_KEY` or `OPENAI_API_KEY`: API key
- `IGENT_CONFIG`: Custom config file path

## Testing

```bash
go test ./...
go test -v ./internal/memory/...
```

## Build & Install

```bash
go build -o igent ./cmd/igent
go install ./cmd/igent
```
