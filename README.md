# igent

A simple, extensible AI agent in Go with persistent context and intelligent memory management.

## Features

- **Multiple LLM Providers**: OpenAI, Z.AI, GLM-5, and any OpenAI-compatible API
- **Tool Support**: Built-in CLI tools (date, ls, ps, curl, etc.) that the LLM can call
- **Persistent Storage**: JSON-based storage for portability
- **Context Optimization**: Automatic summarization and sliding window to keep context relevant
- **Memory System**: Store and retrieve important facts, preferences, and context
- **Skill System**: Extensible capabilities with pattern matching
- **Interactive REPL**: Built-in interactive mode with slash commands

## Installation

```bash
go install github.com/igm/igent/cmd/igent@latest
```

## Quick Start

```bash
# Initialize configuration
igent config init

# Interactive mode
igent

# Single message
igent "What is the capital of France?"

# Use specific conversation
igent -C work-chat "Continue our discussion"
```

## Configuration

Configuration is stored in `~/.igent/config.yaml`:

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
  summarize_when: 30    # Trigger summarization threshold

agent:
  name: igent
  system_prompt: "You are a helpful AI assistant."
```

### Environment Variables

- `IGENT_API_KEY` or `OPENAI_API_KEY`: API key
- `IGENT_CONFIG`: Custom config file path

## Usage

### CLI Commands

```bash
# Interactive chat
igent

# Single query
igent "Your question here"

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
> /tools                # List available tools
> /clear                # Clear screen
> /exit                 # Exit
```

## Tool System

The agent has built-in tools that the LLM can automatically call to perform actions:

### Available Tools

| Tool | Description |
|------|-------------|
| `date` | Get current date/time with optional format |
| `ls` | List directory contents |
| `cat` | Read file contents (limited to 1000 lines) |
| `pwd` | Get current working directory |
| `ps` | List running processes |
| `curl` | Make HTTP requests |
| `which` | Find command location |
| `echo` | Echo text (for testing) |
| `env` | List environment variables |
| `head` | Read first N lines of a file |
| `tail` | Read last N lines of a file |
| `df` | Show disk space usage |
| `uname` | Get system information |

### How Tools Work

1. When you ask a question that requires real-time data, the LLM requests a tool call
2. The agent executes the tool locally
3. Results are fed back to the LLM for processing
4. The LLM provides a final response based on tool results

Example:
```
> What's the current date and time?
[LLM calls date tool]
The current date and time is Monday, February 17, 2026 at 2:30 PM UTC.

> List files in the current directory
[LLM calls ls tool]
Here are the files in /Users/you/project:
- README.md (1.2K)
- main.go (3.4K)
- go.mod (0.5K)
...
```

## Supported Providers

### OpenAI
```yaml
provider:
  type: openai
  base_url: https://api.openai.com/v1
  model: gpt-4o-mini
```

### Z.AI / GLM
```yaml
provider:
  type: zhipu
  base_url: https://open.bigmodel.cn/api/paas/v4
  model: glm-4-flash
```

### Custom OpenAI-Compatible
```yaml
provider:
  type: openai
  base_url: https://your-api.com/v1
  model: your-model
```

## Architecture

```
igent/
├── cmd/igent/           # CLI entry point
├── internal/
│   ├── agent/           # Core agent logic & tool orchestration
│   ├── config/          # Configuration management
│   ├── llm/             # LLM provider abstraction
│   ├── memory/          # Context & memory optimization
│   ├── skills/          # Skill system
│   ├── storage/         # Persistence layer
│   └── tools/           # Tool registry & execution
```

## Context Optimization

The agent uses a multi-layer approach to keep context relevant:

1. **Sliding Window**: Keeps most recent messages within token budget
2. **Summarization**: When message count exceeds threshold, older messages are summarized
3. **Memory Extraction**: Important facts are extracted from summarized content
4. **Relevance Matching**: Memories are retrieved based on keyword matching and relevance scores

## Development

```bash
# Build
go build -o igent ./cmd/igent

# Test
go test ./...

# Install
go install ./cmd/igent
```

## License

MIT
