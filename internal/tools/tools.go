package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/igm/igent/internal/logger"
)

// Tool represents a tool that can be called by the LLM
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Executor    func(args map[string]interface{}) (string, error)
}

// ToolCall represents a tool call request from the LLM
type ToolCall struct {
	ID      string                 `json:"id"`
	Name    string                 `json:"name"`
	Args    map[string]interface{} `json:"args"`
	RawArgs string                 `json:"-"` // Original JSON string
}

// ToolResult represents the result of executing a tool
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
}

// Registry manages available tools
type Registry struct {
	tools map[string]*Tool
	log   *slog.Logger
}

// NewRegistry creates a new tool registry with default tools
func NewRegistry() *Registry {
	r := &Registry{
		tools: make(map[string]*Tool),
		log:   logger.L().With("component", "tools"),
	}
	r.registerDefaults()
	return r
}

// Register adds a tool to the registry
func (r *Registry) Register(tool *Tool) {
	r.tools[tool.Name] = tool
	r.log.Debug("tool registered", "name", tool.Name)
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) (*Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools
func (r *Registry) List() []*Tool {
	tools := make([]*Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

// ToOpenAIFormat converts tools to OpenAI function format
func (r *Registry) ToOpenAIFormat() []map[string]interface{} {
	tools := make([]map[string]interface{}, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			},
		})
	}
	return tools
}

// Execute runs a tool with the given arguments
func (r *Registry) Execute(ctx context.Context, call *ToolCall) *ToolResult {
	r.log.Info("executing tool", "name", call.Name, "id", call.ID)

	tool, ok := r.tools[call.Name]
	if !ok {
		return &ToolResult{
			ToolCallID: call.ID,
			Name:       call.Name,
			Error:      fmt.Sprintf("unknown tool: %s", call.Name),
		}
	}

	output, err := tool.Executor(call.Args)
	if err != nil {
		r.log.Error("tool execution failed", "name", call.Name, "error", err)
		return &ToolResult{
			ToolCallID: call.ID,
			Name:       call.Name,
			Error:      err.Error(),
		}
	}

	r.log.Debug("tool executed successfully", "name", call.Name, "output_length", len(output))
	return &ToolResult{
		ToolCallID: call.ID,
		Name:       call.Name,
		Output:     output,
	}
}

// registerDefaults adds the default CLI tools
func (r *Registry) registerDefaults() {
	// date - Get current date/time
	r.Register(&Tool{
		Name:        "date",
		Description: "Get the current date and time. Use format string to customize output (Go time format).",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"format": map[string]interface{}{
					"type":        "string",
					"description": "Optional Go time format string (e.g., '2006-01-02', '15:04:05')",
				},
			},
		},
		Executor: func(args map[string]interface{}) (string, error) {
			format := time.RFC1123
			if f, ok := args["format"].(string); ok && f != "" {
				format = f
			}
			return time.Now().Format(format), nil
		},
	})

	// ls - List directory contents
	r.Register(&Tool{
		Name:        "ls",
		Description: "List files and directories in a given path. Returns detailed listing with permissions, size, and modification time.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Directory path to list (default: current directory)",
				},
				"long": map[string]interface{}{
					"type":        "boolean",
					"description": "Use long format with details (default: true)",
				},
				"all": map[string]interface{}{
					"type":        "boolean",
					"description": "Show hidden files (default: false)",
				},
			},
		},
		Executor: func(args map[string]interface{}) (string, error) {
			path := "."
			if p, ok := args["path"].(string); ok && p != "" {
				path = p
			}

			cmdArgs := []string{}
			if getBool(args, "long", true) {
				cmdArgs = append(cmdArgs, "-l")
			}
			if getBool(args, "all", false) {
				cmdArgs = append(cmdArgs, "-a")
			}
			cmdArgs = append(cmdArgs, path)

			return runCommand("ls", cmdArgs...)
		},
	})

	// cat - Read file contents
	r.Register(&Tool{
		Name:        "cat",
		Description: "Read and return the contents of a file. Limited to first 1000 lines for safety.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to read",
				},
			},
			"required": []string{"path"},
		},
		Executor: func(args map[string]interface{}) (string, error) {
			path, ok := args["path"].(string)
			if !ok || path == "" {
				return "", fmt.Errorf("path is required")
			}

			// Read file with line limit for safety
			data, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}

			content := string(data)
			lines := strings.Split(content, "\n")
			if len(lines) > 1000 {
				return strings.Join(lines[:1000], "\n") + "\n... (truncated, file has more lines)", nil
			}
			return content, nil
		},
	})

	// pwd - Print working directory
	r.Register(&Tool{
		Name:        "pwd",
		Description: "Get the current working directory.",
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Executor: func(args map[string]interface{}) (string, error) {
			return os.Getwd()
		},
	})

	// ps - List processes
	r.Register(&Tool{
		Name:        "ps",
		Description: "List running processes. Shows process ID, CPU usage, memory usage, and command.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"all": map[string]interface{}{
					"type":        "boolean",
					"description": "Show all processes, not just user's (default: false)",
				},
			},
		},
		Executor: func(args map[string]interface{}) (string, error) {
			// Use ps command with custom format
			cmdArgs := []string{"-o", "pid,pcpu,pmem,comm"}
			if getBool(args, "all", false) {
				cmdArgs = []string{"-e", "-o", "pid,pcpu,pmem,comm"}
			}
			return runCommand("ps", cmdArgs...)
		},
	})

	// curl - Make HTTP requests
	r.Register(&Tool{
		Name:        "curl",
		Description: "Make HTTP requests to URLs. Supports GET, POST, and other methods. Returns response body and status.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "The URL to request",
				},
				"method": map[string]interface{}{
					"type":        "string",
					"description": "HTTP method (GET, POST, PUT, DELETE, etc.)",
					"enum":        []string{"GET", "POST", "PUT", "DELETE", "HEAD", "PATCH"},
				},
				"headers": map[string]interface{}{
					"type":        "object",
					"description": "HTTP headers as key-value pairs",
				},
				"data": map[string]interface{}{
					"type":        "string",
					"description": "Request body data (for POST, PUT, PATCH)",
				},
				"timeout": map[string]interface{}{
					"type":        "integer",
					"description": "Request timeout in seconds (default: 30)",
				},
			},
			"required": []string{"url"},
		},
		Executor: func(args map[string]interface{}) (string, error) {
			url, ok := args["url"].(string)
			if !ok || url == "" {
				return "", fmt.Errorf("url is required")
			}

			cmdArgs := []string{"-s", "-i"} // Silent but include headers

			// Method
			if method, ok := args["method"].(string); ok && method != "" {
				cmdArgs = append(cmdArgs, "-X", strings.ToUpper(method))
			}

			// Headers
			if headers, ok := args["headers"].(map[string]interface{}); ok {
				for k, v := range headers {
					if vs, ok := v.(string); ok {
						cmdArgs = append(cmdArgs, "-H", fmt.Sprintf("%s: %s", k, vs))
					}
				}
			}

			// Body data
			if data, ok := args["data"].(string); ok && data != "" {
				cmdArgs = append(cmdArgs, "-d", data)
			}

			// Timeout
			timeout := 30
			if t, ok := args["timeout"].(float64); ok {
				timeout = int(t)
			}
			cmdArgs = append(cmdArgs, "--max-time", fmt.Sprintf("%d", timeout))

			cmdArgs = append(cmdArgs, url)

			return runCommand("curl", cmdArgs...)
		},
	})

	// which - Find command location
	r.Register(&Tool{
		Name:        "which",
		Description: "Find the full path to a command.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The command to find",
				},
			},
			"required": []string{"command"},
		},
		Executor: func(args map[string]interface{}) (string, error) {
			cmd, ok := args["command"].(string)
			if !ok || cmd == "" {
				return "", fmt.Errorf("command is required")
			}
			return runCommand("which", cmd)
		},
	})

	// echo - Echo text (useful for testing)
	r.Register(&Tool{
		Name:        "echo",
		Description: "Echo the input text. Useful for testing tool functionality.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{
					"type":        "string",
					"description": "Text to echo",
				},
			},
			"required": []string{"text"},
		},
		Executor: func(args map[string]interface{}) (string, error) {
			text, ok := args["text"].(string)
			if !ok {
				return "", fmt.Errorf("text is required")
			}
			return text, nil
		},
	})

	// env - List environment variables
	r.Register(&Tool{
		Name:        "env",
		Description: "List environment variables. Can optionally filter by name pattern.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"filter": map[string]interface{}{
					"type":        "string",
					"description": "Optional filter pattern (substring match)",
				},
			},
		},
		Executor: func(args map[string]interface{}) (string, error) {
			filter, _ := args["filter"].(string)
			var result []string
			for _, env := range os.Environ() {
				if filter == "" || strings.Contains(strings.ToLower(env), strings.ToLower(filter)) {
					result = append(result, env)
				}
			}
			return strings.Join(result, "\n"), nil
		},
	})

	// head - Read first lines of file
	r.Register(&Tool{
		Name:        "head",
		Description: "Read the first N lines of a file.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file",
				},
				"lines": map[string]interface{}{
					"type":        "integer",
					"description": "Number of lines to read (default: 10, max: 100)",
				},
			},
			"required": []string{"path"},
		},
		Executor: func(args map[string]interface{}) (string, error) {
			path, ok := args["path"].(string)
			if !ok || path == "" {
				return "", fmt.Errorf("path is required")
			}

			lines := 10
			if l, ok := args["lines"].(float64); ok && l > 0 {
				lines = int(l)
				if lines > 100 {
					lines = 100
				}
			}

			return runCommand("head", "-n", fmt.Sprintf("%d", lines), path)
		},
	})

	// tail - Read last lines of file
	r.Register(&Tool{
		Name:        "tail",
		Description: "Read the last N lines of a file.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file",
				},
				"lines": map[string]interface{}{
					"type":        "integer",
					"description": "Number of lines to read (default: 10, max: 100)",
				},
			},
			"required": []string{"path"},
		},
		Executor: func(args map[string]interface{}) (string, error) {
			path, ok := args["path"].(string)
			if !ok || path == "" {
				return "", fmt.Errorf("path is required")
			}

			lines := 10
			if l, ok := args["lines"].(float64); ok && l > 0 {
				lines = int(l)
				if lines > 100 {
					lines = 100
				}
			}

			return runCommand("tail", "-n", fmt.Sprintf("%d", lines), path)
		},
	})

	// df - Disk free space
	r.Register(&Tool{
		Name:        "df",
		Description: "Show disk space usage for file systems.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"human": map[string]interface{}{
					"type":        "boolean",
					"description": "Show sizes in human readable format (default: true)",
				},
			},
		},
		Executor: func(args map[string]interface{}) (string, error) {
			cmdArgs := []string{}
			if getBool(args, "human", true) {
				cmdArgs = append(cmdArgs, "-h")
			}
			return runCommand("df", cmdArgs...)
		},
	})

	// uname - System information
	r.Register(&Tool{
		Name:        "uname",
		Description: "Get system information (OS, kernel version, etc.).",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"all": map[string]interface{}{
					"type":        "boolean",
					"description": "Show all information (default: true)",
				},
			},
		},
		Executor: func(args map[string]interface{}) (string, error) {
			if getBool(args, "all", true) {
				return runCommand("uname", "-a")
			}
			return runCommand("uname")
		},
	})

	// shell - Execute shell commands with pipes and redirections
	r.Register(&Tool{
		Name:        "shell",
		Description: "Execute a shell command. Supports pipes (|), redirections (>), and other shell features. Use this for complex commands that need shell processing.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The shell command to execute (supports pipes, redirections, etc.)",
				},
				"timeout": map[string]interface{}{
					"type":        "integer",
					"description": "Timeout in seconds (default: 30, max: 120)",
				},
			},
			"required": []string{"command"},
		},
		Executor: func(args map[string]interface{}) (string, error) {
			command, ok := args["command"].(string)
			if !ok || command == "" {
				return "", fmt.Errorf("command is required")
			}

			timeout := 30
			if t, ok := args["timeout"].(float64); ok && t > 0 {
				timeout = int(t)
				if timeout > 120 {
					timeout = 120
				}
			}

			// Use sh -c for Unix-like systems
			shell := "/bin/sh"
			if _, err := os.Stat("/bin/sh"); os.IsNotExist(err) {
				// Fallback for non-Unix systems
				shell = "sh"
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, shell, "-c", command)
			cmd.Env = os.Environ()

			output, err := cmd.CombinedOutput()
			if ctx.Err() == context.DeadlineExceeded {
				return "", fmt.Errorf("command timed out after %d seconds", timeout)
			}
			if err != nil {
				return string(output), fmt.Errorf("command failed: %w", err)
			}

			result := strings.TrimSpace(string(output))
			if len(result) > 15000 {
				result = result[:15000] + "\n... (output truncated)"
			}

			return result, nil
		},
	})
}

// ParseToolCall parses a tool call from LLM response
func ParseToolCall(id, name, argsJSON string) (*ToolCall, error) {
	call := &ToolCall{
		ID:      id,
		Name:    name,
		RawArgs: argsJSON,
		Args:    make(map[string]interface{}),
	}

	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &call.Args); err != nil {
			return nil, fmt.Errorf("parsing tool arguments: %w", err)
		}
	}

	return call, nil
}

// runCommand safely executes a shell command
func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	result := strings.TrimSpace(string(output))
	if len(result) > 10000 {
		result = result[:10000] + "\n... (output truncated)"
	}

	return result, nil
}

// getBool safely gets a boolean from args with default
func getBool(args map[string]interface{}, key string, def bool) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return def
}
