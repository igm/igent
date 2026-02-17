package tools

import (
	"context"
	"strings"
	"testing"
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
