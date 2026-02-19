package tools

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/igm/igent/internal/storage"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	if registry == nil {
		t.Fatal("registry should not be nil")
	}

	tools := registry.List()
	if len(tools) == 0 {
		t.Error("registry should have default tools registered")
	}
}

func TestGetTool(t *testing.T) {
	registry := NewRegistry()

	tool, ok := registry.Get("date")
	if !ok {
		t.Error("date tool should exist")
	}
	if tool.Name != "date" {
		t.Errorf("expected tool name 'date', got %s", tool.Name)
	}

	_, ok = registry.Get("nonexistent")
	if ok {
		t.Error("nonexistent tool should not exist")
	}
}

func TestExecuteTool(t *testing.T) {
	registry := NewRegistry()

	// Test echo tool
	call := &ToolCall{
		ID:   "test-1",
		Name: "echo",
		Args: map[string]interface{}{"text": "Hello World"},
	}

	result := registry.Execute(context.Background(), call)

	if result.ToolCallID != "test-1" {
		t.Errorf("expected tool call ID 'test-1', got %s", result.ToolCallID)
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if result.Output != "Hello World" {
		t.Errorf("expected output 'Hello World', got %s", result.Output)
	}
}

func TestExecuteUnknownTool(t *testing.T) {
	registry := NewRegistry()

	call := &ToolCall{
		ID:   "test-2",
		Name: "unknown_tool",
		Args: map[string]interface{}{},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error == "" {
		t.Error("expected error for unknown tool")
	}
}

func TestParseToolCall(t *testing.T) {
	call, err := ParseToolCall("call-123", "ls", `{"path": "/tmp", "long": true}`)
	if err != nil {
		t.Fatalf("failed to parse tool call: %v", err)
	}

	if call.ID != "call-123" {
		t.Errorf("expected ID 'call-123', got %s", call.ID)
	}
	if call.Name != "ls" {
		t.Errorf("expected name 'ls', got %s", call.Name)
	}
	if call.Args["path"] != "/tmp" {
		t.Errorf("expected path '/tmp', got %v", call.Args["path"])
	}
}

func TestParseToolCallInvalidJSON(t *testing.T) {
	_, err := ParseToolCall("call-123", "ls", `{invalid json}`)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDateTool(t *testing.T) {
	registry := NewRegistry()

	call := &ToolCall{
		ID:   "test-date",
		Name: "date",
		Args: map[string]interface{}{},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
}

func TestPwdTool(t *testing.T) {
	registry := NewRegistry()

	call := &ToolCall{
		ID:   "test-pwd",
		Name: "pwd",
		Args: map[string]interface{}{},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
}

func TestToOpenAIFormat(t *testing.T) {
	registry := NewRegistry()

	tools := registry.ToOpenAIFormat()

	if len(tools) == 0 {
		t.Error("expected at least one tool in OpenAI format")
	}

	// Check that format is correct
	for _, tool := range tools {
		if tool["type"] != "function" {
			t.Errorf("expected type 'function', got %v", tool["type"])
		}
		fn, ok := tool["function"].(map[string]interface{})
		if !ok {
			t.Error("function should be a map")
			continue
		}
		if fn["name"] == "" {
			t.Error("function name should not be empty")
		}
		if fn["description"] == "" {
			t.Error("function description should not be empty")
		}
	}
}

func TestShellTool(t *testing.T) {
	registry := NewRegistry()

	call := &ToolCall{
		ID:   "test-shell",
		Name: "shell",
		Args: map[string]interface{}{"command": "echo hello"},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if result.Output != "hello" {
		t.Errorf("expected output 'hello', got %s", result.Output)
	}
}

func TestShellToolWithPipe(t *testing.T) {
	registry := NewRegistry()

	call := &ToolCall{
		ID:   "test-shell-pipe",
		Name: "shell",
		Args: map[string]interface{}{"command": "echo -e 'line1\nline2\nline3' | grep line2"},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "line2") {
		t.Errorf("expected output to contain 'line2', got %s", result.Output)
	}
}

func TestRunCommand(t *testing.T) {
	// Test successful command
	output, err := runCommand("echo", "hello", "world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "hello world" {
		t.Errorf("expected 'hello world', got %s", output)
	}
}

func TestRunCommand_WithError(t *testing.T) {
	// Test command that fails
	output, err := runCommand("ls", "/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
	// Output should still contain command output even on error
	if output == "" {
		t.Error("expected some output even on error")
	}
}

func TestRunCommand_OutputTruncation(t *testing.T) {
	// Test output truncation (output > 10000 chars)
	// Generate a large output - use a long line repeated many times
	// Each line is about 100 chars, so we need more than 100 lines to exceed 10000 chars
	output, err := runCommand("sh", "-c", "for i in $(seq 1 200); do printf '%090d\n' $i; done")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Output should be truncated if > 10000 chars
	if len(output) > 10000 {
		// Should contain truncation message
		if !strings.Contains(output, "truncated") {
			t.Errorf("large output should contain 'truncated', got length %d", len(output))
		}
		// Should be truncated to ~10000 + truncation message
		if len(output) > 10200 {
			t.Errorf("output should be truncated to ~10000, got length %d", len(output))
		}
	} else {
		// If output wasn't large enough, skip the truncation check
		t.Logf("output length %d, not large enough to test truncation", len(output))
	}
}

func TestGetBool(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		key      string
		def      bool
		expected bool
	}{
		{
			name:     "true value",
			args:     map[string]interface{}{"flag": true},
			key:      "flag",
			def:      false,
			expected: true,
		},
		{
			name:     "false value",
			args:     map[string]interface{}{"flag": false},
			key:      "flag",
			def:      true,
			expected: false,
		},
		{
			name:     "missing key with true default",
			args:     map[string]interface{}{},
			key:      "flag",
			def:      true,
			expected: true,
		},
		{
			name:     "missing key with false default",
			args:     map[string]interface{}{},
			key:      "flag",
			def:      false,
			expected: false,
		},
		{
			name:     "non-bool value",
			args:     map[string]interface{}{"flag": "true"},
			key:      "flag",
			def:      false,
			expected: false,
		},
		{
			name:     "nil args",
			args:     nil,
			key:      "flag",
			def:      true,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args == nil {
				// Test with nil map (should use default)
				result := getBool(map[string]interface{}{}, tt.key, tt.def)
				if result != tt.expected {
					t.Errorf("getBool() = %v, want %v", result, tt.expected)
				}
			} else {
				result := getBool(tt.args, tt.key, tt.def)
				if result != tt.expected {
					t.Errorf("getBool() = %v, want %v", result, tt.expected)
				}
			}
		})
	}
}

func TestLsTool(t *testing.T) {
	registry := NewRegistry()

	// Create temp dir with files
	tmpDir, err := os.MkdirTemp("", "ls-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some files
	os.WriteFile(tmpDir+"/file1.txt", []byte("test"), 0644)
	os.WriteFile(tmpDir+"/file2.txt", []byte("test"), 0644)

	call := &ToolCall{
		ID:   "test-ls",
		Name: "ls",
		Args: map[string]interface{}{"path": tmpDir},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "file1.txt") {
		t.Errorf("expected output to contain 'file1.txt', got %s", result.Output)
	}
}

func TestCatTool(t *testing.T) {
	registry := NewRegistry()

	// Create temp file
	tmpFile, err := os.CreateTemp("", "cat-test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "Hello, World!\nThis is a test file."
	os.WriteFile(tmpFile.Name(), []byte(content), 0644)

	call := &ToolCall{
		ID:   "test-cat",
		Name: "cat",
		Args: map[string]interface{}{"path": tmpFile.Name()},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "Hello, World!") {
		t.Errorf("expected output to contain 'Hello, World!', got %s", result.Output)
	}
}

func TestCatTool_MissingPath(t *testing.T) {
	registry := NewRegistry()

	call := &ToolCall{
		ID:   "test-cat-missing",
		Name: "cat",
		Args: map[string]interface{}{},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error == "" {
		t.Error("expected error for missing path")
	}
}

func TestCatTool_NonexistentFile(t *testing.T) {
	registry := NewRegistry()

	call := &ToolCall{
		ID:   "test-cat-nonexistent",
		Name: "cat",
		Args: map[string]interface{}{"path": "/nonexistent/file.txt"},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error == "" {
		t.Error("expected error for nonexistent file")
	}
}

func TestEchoTool_MissingText(t *testing.T) {
	registry := NewRegistry()

	call := &ToolCall{
		ID:   "test-echo-missing",
		Name: "echo",
		Args: map[string]interface{}{},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error == "" {
		t.Error("expected error for missing text")
	}
}

func TestHeadTool(t *testing.T) {
	registry := NewRegistry()

	// Create temp file with multiple lines
	tmpFile, err := os.CreateTemp("", "head-test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "line1\nline2\nline3\nline4\nline5\n"
	os.WriteFile(tmpFile.Name(), []byte(content), 0644)

	call := &ToolCall{
		ID:   "test-head",
		Name: "head",
		Args: map[string]interface{}{"path": tmpFile.Name(), "lines": 3.0},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "line1") {
		t.Errorf("expected output to contain 'line1', got %s", result.Output)
	}
}

func TestTailTool(t *testing.T) {
	registry := NewRegistry()

	// Create temp file with multiple lines
	tmpFile, err := os.CreateTemp("", "tail-test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "line1\nline2\nline3\nline4\nline5\n"
	os.WriteFile(tmpFile.Name(), []byte(content), 0644)

	call := &ToolCall{
		ID:   "test-tail",
		Name: "tail",
		Args: map[string]interface{}{"path": tmpFile.Name(), "lines": 2.0},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "line5") {
		t.Errorf("expected output to contain 'line5', got %s", result.Output)
	}
}

func TestEnvTool(t *testing.T) {
	registry := NewRegistry()

	call := &ToolCall{
		ID:   "test-env",
		Name: "env",
		Args: map[string]interface{}{},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("expected non-empty environment output")
	}
}

func TestEnvTool_WithFilter(t *testing.T) {
	registry := NewRegistry()

	// Set a unique env var for testing
	os.Setenv("IGENT_TEST_VAR", "test_value_123")
	defer os.Unsetenv("IGENT_TEST_VAR")

	call := &ToolCall{
		ID:   "test-env-filter",
		Name: "env",
		Args: map[string]interface{}{"filter": "IGENT_TEST"},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "IGENT_TEST_VAR") {
		t.Errorf("expected output to contain 'IGENT_TEST_VAR', got %s", result.Output)
	}
}

func TestWhichTool(t *testing.T) {
	registry := NewRegistry()

	call := &ToolCall{
		ID:   "test-which",
		Name: "which",
		Args: map[string]interface{}{"command": "ls"},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
}

func TestWhichTool_MissingCommand(t *testing.T) {
	registry := NewRegistry()

	call := &ToolCall{
		ID:   "test-which-missing",
		Name: "which",
		Args: map[string]interface{}{},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error == "" {
		t.Error("expected error for missing command")
	}
}

func TestDfTool(t *testing.T) {
	registry := NewRegistry()

	call := &ToolCall{
		ID:   "test-df",
		Name: "df",
		Args: map[string]interface{}{"human": true},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("expected non-empty disk usage output")
	}
}

func TestUnameTool(t *testing.T) {
	registry := NewRegistry()

	call := &ToolCall{
		ID:   "test-uname",
		Name: "uname",
		Args: map[string]interface{}{"all": true},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("expected non-empty uname output")
	}
}

func TestPsTool(t *testing.T) {
	registry := NewRegistry()

	call := &ToolCall{
		ID:   "test-ps",
		Name: "ps",
		Args: map[string]interface{}{"all": false},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("expected non-empty process list")
	}
}

func TestCurlTool(t *testing.T) {
	registry := NewRegistry()

	// Test with a simple HTTP request (using httpbin or similar)
	call := &ToolCall{
		ID:   "test-curl",
		Name: "curl",
		Args: map[string]interface{}{
			"url":     "https://httpbin.org/get",
			"method":  "GET",
			"timeout": 10.0,
		},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Logf("curl result error (may be network issue): %s", result.Error)
		// Don't fail on network issues
	}
}

func TestCurlTool_MissingURL(t *testing.T) {
	registry := NewRegistry()

	call := &ToolCall{
		ID:   "test-curl-missing",
		Name: "curl",
		Args: map[string]interface{}{},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error == "" {
		t.Error("expected error for missing URL")
	}
}

func TestShellTool_Timeout(t *testing.T) {
	registry := NewRegistry()

	call := &ToolCall{
		ID:   "test-shell-timeout",
		Name: "shell",
		Args: map[string]interface{}{
			"command": "sleep 5",
			"timeout": 1.0, // 1 second timeout
		},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error == "" {
		t.Error("expected timeout error")
	}
	if !strings.Contains(result.Error, "timeout") && !strings.Contains(result.Error, "timed out") {
		t.Logf("timeout error: %s", result.Error)
	}
}

func TestShellTool_OutputTruncation(t *testing.T) {
	registry := NewRegistry()

	// Generate large output
	call := &ToolCall{
		ID:   "test-shell-trunc",
		Name: "shell",
		Args: map[string]interface{}{
			"command": "yes 'test output line' | head -1000",
		},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	// Check if truncation happened (output > 15000 chars triggers truncation)
	if len(result.Output) > 15200 {
		t.Errorf("output should be truncated, got length %d", len(result.Output))
	}
}

func TestRegisterCustomTool(t *testing.T) {
	registry := NewRegistry()

	// Register a custom tool
	customTool := &Tool{
		Name:        "custom_tool",
		Description: "A custom test tool",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "Input parameter",
				},
			},
		},
		Executor: func(args map[string]interface{}) (string, error) {
			input, _ := args["input"].(string)
			return "custom: " + input, nil
		},
	}

	registry.Register(customTool)

	// Verify it's registered
	tool, ok := registry.Get("custom_tool")
	if !ok {
		t.Fatal("custom tool not registered")
	}
	if tool.Name != "custom_tool" {
		t.Errorf("expected name 'custom_tool', got %s", tool.Name)
	}

	// Test execution
	call := &ToolCall{
		ID:   "test-custom",
		Name: "custom_tool",
		Args: map[string]interface{}{"input": "hello"},
	}

	result := registry.Execute(context.Background(), call)
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if result.Output != "custom: hello" {
		t.Errorf("expected 'custom: hello', got %s", result.Output)
	}
}

func TestParseToolCall_EmptyArgs(t *testing.T) {
	call, err := ParseToolCall("call-123", "test", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if call.ID != "call-123" {
		t.Errorf("expected ID 'call-123', got %s", call.ID)
	}
	if call.Name != "test" {
		t.Errorf("expected name 'test', got %s", call.Name)
	}
	if len(call.Args) != 0 {
		t.Errorf("expected empty args, got %v", call.Args)
	}
}

func TestDateTool_CustomFormat(t *testing.T) {
	registry := NewRegistry()

	call := &ToolCall{
		ID:   "test-date-format",
		Name: "date",
		Args: map[string]interface{}{"format": "2006-01-02"},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	// Check that output looks like YYYY-MM-DD
	if len(result.Output) != 10 {
		t.Errorf("expected 10 chars for YYYY-MM-DD format, got %d: %s", len(result.Output), result.Output)
	}
}

// Memory Tools Tests

func setupMemoryTest(t *testing.T) (*Registry, *storage.JSONStore, string) {
	tmpDir, err := os.MkdirTemp("", "igent-memory-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	store, err := storage.NewJSONStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	registry := NewRegistry()
	registry.SetStorage(store)

	return registry, store, tmpDir
}

func TestMemoryAdd(t *testing.T) {
	registry, _, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	call := &ToolCall{
		ID:   "test-memory-add",
		Name: "memory_add",
		Args: map[string]interface{}{
			"content":   "User prefers dark mode",
			"type":      "preference",
			"relevance": 0.9,
		},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "Memory stored successfully") {
		t.Errorf("expected success message, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "preference") {
		t.Errorf("expected type in output, got: %s", result.Output)
	}
}

func TestMemoryAdd_Defaults(t *testing.T) {
	registry, _, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	call := &ToolCall{
		ID:   "test-memory-add-defaults",
		Name: "memory_add",
		Args: map[string]interface{}{
			"content": "Test fact",
			"type":    "fact",
		},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "Memory stored successfully") {
		t.Errorf("expected success message, got: %s", result.Output)
	}
}

func TestMemoryAdd_MissingContent(t *testing.T) {
	registry, _, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	call := &ToolCall{
		ID:   "test-memory-add-no-content",
		Name: "memory_add",
		Args: map[string]interface{}{
			"type": "fact",
		},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error == "" {
		t.Error("expected error for missing content")
	}
}

func TestMemoryAdd_InvalidType(t *testing.T) {
	registry, _, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	call := &ToolCall{
		ID:   "test-memory-add-invalid-type",
		Name: "memory_add",
		Args: map[string]interface{}{
			"content": "Test content",
			"type":    "invalid_type",
		},
	}

	result := registry.Execute(context.Background(), call)

	// Should default to "fact"
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "fact") {
		t.Errorf("expected type to default to 'fact', got: %s", result.Output)
	}
}

func TestMemoryList(t *testing.T) {
	registry, _, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	// Add some memories first
	registry.Execute(context.Background(), &ToolCall{
		ID:   "add1",
		Name: "memory_add",
		Args: map[string]interface{}{"content": "First memory", "type": "fact"},
	})
	registry.Execute(context.Background(), &ToolCall{
		ID:   "add2",
		Name: "memory_add",
		Args: map[string]interface{}{"content": "Second memory", "type": "preference"},
	})

	call := &ToolCall{
		ID:   "test-memory-list",
		Name: "memory_list",
		Args: map[string]interface{}{},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "First memory") {
		t.Errorf("expected 'First memory' in output, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "Second memory") {
		t.Errorf("expected 'Second memory' in output, got: %s", result.Output)
	}
}

func TestMemoryList_Empty(t *testing.T) {
	registry, _, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	call := &ToolCall{
		ID:   "test-memory-list-empty",
		Name: "memory_list",
		Args: map[string]interface{}{},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "No memories") {
		t.Errorf("expected 'No memories' message, got: %s", result.Output)
	}
}

func TestMemorySearch(t *testing.T) {
	registry, _, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	// Add some memories
	registry.Execute(context.Background(), &ToolCall{
		ID:   "add1",
		Name: "memory_add",
		Args: map[string]interface{}{"content": "User prefers dark mode", "type": "preference"},
	})
	registry.Execute(context.Background(), &ToolCall{
		ID:   "add2",
		Name: "memory_add",
		Args: map[string]interface{}{"content": "User's name is Alice", "type": "fact"},
	})

	call := &ToolCall{
		ID:   "test-memory-search",
		Name: "memory_search",
		Args: map[string]interface{}{"query": "dark mode"},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "dark mode") {
		t.Errorf("expected 'dark mode' in output, got: %s", result.Output)
	}
	if strings.Contains(result.Output, "Alice") {
		t.Errorf("should not contain 'Alice', got: %s", result.Output)
	}
}

func TestMemorySearch_CaseInsensitive(t *testing.T) {
	registry, _, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	registry.Execute(context.Background(), &ToolCall{
		ID:   "add1",
		Name: "memory_add",
		Args: map[string]interface{}{"content": "Dark Mode", "type": "preference"},
	})

	call := &ToolCall{
		ID:   "test-memory-search-case",
		Name: "memory_search",
		Args: map[string]interface{}{"query": "DARK MODE"},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "Found") {
		t.Errorf("expected 'Found' in output, got: %s", result.Output)
	}
}

func TestMemorySearch_NotFound(t *testing.T) {
	registry, _, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	registry.Execute(context.Background(), &ToolCall{
		ID:   "add1",
		Name: "memory_add",
		Args: map[string]interface{}{"content": "Some content", "type": "fact"},
	})

	call := &ToolCall{
		ID:   "test-memory-search-notfound",
		Name: "memory_search",
		Args: map[string]interface{}{"query": "nonexistent"},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "No memories found") {
		t.Errorf("expected 'No memories found' message, got: %s", result.Output)
	}
}

func TestMemorySearch_MissingQuery(t *testing.T) {
	registry, _, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	call := &ToolCall{
		ID:   "test-memory-search-noquery",
		Name: "memory_search",
		Args: map[string]interface{}{},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error == "" {
		t.Error("expected error for missing query")
	}
}

func TestMemoryUpdate_ById(t *testing.T) {
	registry, store, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	// Add a memory first
	registry.Execute(context.Background(), &ToolCall{
		ID:   "add1",
		Name: "memory_add",
		Args: map[string]interface{}{"content": "Original content", "type": "fact"},
	})

	// Get the ID
	memories, _ := store.LoadMemories()
	memID := memories[0].ID

	call := &ToolCall{
		ID:   "test-memory-update-id",
		Name: "memory_update",
		Args: map[string]interface{}{
			"id":      memID,
			"content": "Updated content",
		},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "updated successfully") {
		t.Errorf("expected success message, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "Updated content") {
		t.Errorf("expected updated content in output, got: %s", result.Output)
	}
}

func TestMemoryUpdate_BySearch(t *testing.T) {
	registry, _, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	// Add a memory first
	registry.Execute(context.Background(), &ToolCall{
		ID:   "add1",
		Name: "memory_add",
		Args: map[string]interface{}{"content": "User prefers dark mode", "type": "preference"},
	})

	call := &ToolCall{
		ID:   "test-memory-update-search",
		Name: "memory_update",
		Args: map[string]interface{}{
			"search":  "dark mode",
			"content": "User prefers light mode",
		},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "light mode") {
		t.Errorf("expected updated content in output, got: %s", result.Output)
	}
}

func TestMemoryUpdate_NotFound(t *testing.T) {
	registry, _, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	call := &ToolCall{
		ID:   "test-memory-update-notfound",
		Name: "memory_update",
		Args: map[string]interface{}{
			"id":      "nonexistent",
			"content": "Updated content",
		},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error == "" {
		t.Error("expected error for nonexistent memory")
	}
}

func TestMemoryUpdate_NoIdOrSearch(t *testing.T) {
	registry, _, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	call := &ToolCall{
		ID:   "test-memory-update-noargs",
		Name: "memory_update",
		Args: map[string]interface{}{
			"content": "Updated content",
		},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error == "" {
		t.Error("expected error when no id or search provided")
	}
}

func TestMemoryUpdate_NoUpdates(t *testing.T) {
	registry, _, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	// Add a memory first
	registry.Execute(context.Background(), &ToolCall{
		ID:   "add1",
		Name: "memory_add",
		Args: map[string]interface{}{"content": "Original content", "type": "fact"},
	})

	call := &ToolCall{
		ID:   "test-memory-update-noupdates",
		Name: "memory_update",
		Args: map[string]interface{}{
			"search": "Original",
		},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error == "" {
		t.Error("expected error when no updates provided")
	}
}

func TestMemoryDelete_ById(t *testing.T) {
	registry, store, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	// Add a memory first
	registry.Execute(context.Background(), &ToolCall{
		ID:   "add1",
		Name: "memory_add",
		Args: map[string]interface{}{"content": "Memory to delete", "type": "fact"},
	})

	memories, _ := store.LoadMemories()
	memID := memories[0].ID

	call := &ToolCall{
		ID:   "test-memory-delete-id",
		Name: "memory_delete",
		Args: map[string]interface{}{
			"id": memID,
		},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "deleted successfully") {
		t.Errorf("expected success message, got: %s", result.Output)
	}

	// Verify deletion
	memories, _ = store.LoadMemories()
	if len(memories) != 0 {
		t.Errorf("expected 0 memories after delete, got %d", len(memories))
	}
}

func TestMemoryDelete_BySearch(t *testing.T) {
	registry, _, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	// Add a memory first
	registry.Execute(context.Background(), &ToolCall{
		ID:   "add1",
		Name: "memory_add",
		Args: map[string]interface{}{"content": "Memory to delete", "type": "fact"},
	})

	call := &ToolCall{
		ID:   "test-memory-delete-search",
		Name: "memory_delete",
		Args: map[string]interface{}{
			"search": "delete",
		},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "deleted successfully") {
		t.Errorf("expected success message, got: %s", result.Output)
	}
}

func TestMemoryDelete_NotFound(t *testing.T) {
	registry, _, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	call := &ToolCall{
		ID:   "test-memory-delete-notfound",
		Name: "memory_delete",
		Args: map[string]interface{}{
			"id": "nonexistent",
		},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error == "" {
		t.Error("expected error for nonexistent memory")
	}
}

func TestMemoryDelete_NoIdOrSearch(t *testing.T) {
	registry, _, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	call := &ToolCall{
		ID:   "test-memory-delete-noargs",
		Name: "memory_delete",
		Args: map[string]interface{}{},
	}

	result := registry.Execute(context.Background(), call)

	if result.Error == "" {
		t.Error("expected error when no id or search provided")
	}
}

func TestIsSafeTool(t *testing.T) {
	registry, _, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	// Memory tools should be safe
	if !registry.IsSafeTool("memory_add") {
		t.Error("memory_add should be a safe tool")
	}
	if !registry.IsSafeTool("memory_list") {
		t.Error("memory_list should be a safe tool")
	}
	if !registry.IsSafeTool("memory_search") {
		t.Error("memory_search should be a safe tool")
	}
	if !registry.IsSafeTool("memory_update") {
		t.Error("memory_update should be a safe tool")
	}
	if !registry.IsSafeTool("memory_delete") {
		t.Error("memory_delete should be a safe tool")
	}

	// Shell tool should NOT be safe
	if registry.IsSafeTool("shell") {
		t.Error("shell should NOT be a safe tool")
	}
}

func TestSetStorage(t *testing.T) {
	registry := NewRegistry()

	// Without storage, memory tools should not be registered
	if _, ok := registry.Get("memory_add"); ok {
		t.Error("memory_add should not be registered without storage")
	}

	// Create temp storage
	tmpDir, err := os.MkdirTemp("", "igent-storage-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.NewJSONStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Set storage
	registry.SetStorage(store)

	// Now memory tools should be registered
	if _, ok := registry.Get("memory_add"); !ok {
		t.Error("memory_add should be registered after SetStorage")
	}
	if _, ok := registry.Get("memory_list"); !ok {
		t.Error("memory_list should be registered after SetStorage")
	}
}

func TestMemoryToolsWithoutStorage(t *testing.T) {
	// Test that memory tools are not registered without storage
	registry := NewRegistry()

	// Memory tools should not exist
	if _, ok := registry.Get("memory_add"); ok {
		t.Error("memory_add should not be registered without storage")
	}

	// Other tools should still work
	if _, ok := registry.Get("echo"); !ok {
		t.Error("echo should still be available")
	}
}

func TestMemoryEndToEnd(t *testing.T) {
	// End-to-end test: add, list, search, update, delete
	registry, store, tmpDir := setupMemoryTest(t)
	defer os.RemoveAll(tmpDir)

	// 1. Add a memory
	addResult := registry.Execute(context.Background(), &ToolCall{
		ID:   "add",
		Name: "memory_add",
		Args: map[string]interface{}{
			"content":   "User prefers Go programming",
			"type":      "preference",
			"relevance": 0.9,
		},
	})
	if addResult.Error != "" {
		t.Fatalf("failed to add memory: %s", addResult.Error)
	}

	// 2. List memories
	listResult := registry.Execute(context.Background(), &ToolCall{
		ID:   "list",
		Name: "memory_list",
		Args: map[string]interface{}{},
	})
	if listResult.Error != "" {
		t.Fatalf("failed to list memories: %s", listResult.Error)
	}
	if !strings.Contains(listResult.Output, "Go programming") {
		t.Errorf("list should contain 'Go programming', got: %s", listResult.Output)
	}

	// 3. Search for the memory
	searchResult := registry.Execute(context.Background(), &ToolCall{
		ID:   "search",
		Name: "memory_search",
		Args: map[string]interface{}{"query": "Go"},
	})
	if searchResult.Error != "" {
		t.Fatalf("failed to search memories: %s", searchResult.Error)
	}
	if !strings.Contains(searchResult.Output, "Go programming") {
		t.Errorf("search should contain 'Go programming', got: %s", searchResult.Output)
	}

	// 4. Update the memory
	updateResult := registry.Execute(context.Background(), &ToolCall{
		ID:   "update",
		Name: "memory_update",
		Args: map[string]interface{}{
			"search":  "Go programming",
			"content": "User prefers Rust programming",
		},
	})
	if updateResult.Error != "" {
		t.Fatalf("failed to update memory: %s", updateResult.Error)
	}

	// 5. Verify update
	updatedMemories, _ := store.LoadMemories()
	if len(updatedMemories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(updatedMemories))
	}
	if !strings.Contains(updatedMemories[0].Content, "Rust") {
		t.Errorf("memory should be updated to Rust, got: %s", updatedMemories[0].Content)
	}

	// 6. Delete the memory
	deleteResult := registry.Execute(context.Background(), &ToolCall{
		ID:   "delete",
		Name: "memory_delete",
		Args: map[string]interface{}{
			"search": "Rust",
		},
	})
	if deleteResult.Error != "" {
		t.Fatalf("failed to delete memory: %s", deleteResult.Error)
	}

	// 7. Verify deletion
	finalMemories, _ := store.LoadMemories()
	if len(finalMemories) != 0 {
		t.Errorf("expected 0 memories after delete, got %d", len(finalMemories))
	}
}
