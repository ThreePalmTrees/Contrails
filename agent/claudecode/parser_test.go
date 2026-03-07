package claudecode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"contrails/agent"
)

// TestParser_ImplementsSessionParser verifies interface compliance.
// Style: Verify Interface Compliance (go-style-guide.md)
func TestParser_ImplementsSessionParser(t *testing.T) {
	var _ agent.SessionParser = (*Parser)(nil)
}

func TestParser_ParseFile_BasicSession(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "claudecode", "basic_session.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if parsed.SessionID != "session-abc123" {
		t.Errorf("Expected session ID 'session-abc123', got %q", parsed.SessionID)
	}

	if parsed.Title == "" {
		t.Error("Expected a non-empty derived title")
	}

	if parsed.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Expected model 'claude-sonnet-4-20250514', got %q", parsed.Model)
	}

	// Should have user and assistant messages (tool_result lines are not separate messages)
	userCount := 0
	assistantCount := 0
	for _, message := range parsed.Messages {
		switch message.Role {
		case "user":
			userCount++
		case "assistant":
			assistantCount++
		}
	}

	if userCount < 2 {
		t.Errorf("Expected at least 2 user messages, got %d", userCount)
	}
	if assistantCount < 2 {
		t.Errorf("Expected at least 2 assistant messages, got %d", assistantCount)
	}
}

func TestParser_ParseFile_ToolCallSummaries(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "claudecode", "basic_session.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Look for tool call parts with summarized descriptions
	foundToolCall := false
	for _, message := range parsed.Messages {
		for _, part := range message.Parts {
			if part.Type == agent.PartToolCall {
				foundToolCall = true
				if part.Tool == "" {
					t.Error("Tool call should have a non-empty Tool name")
				}
				if part.ToolArgs == "" {
					t.Error("Tool call should have summarized ToolArgs")
				}
			}
		}
	}

	if !foundToolCall {
		t.Error("Expected at least one tool call part")
	}
}

func TestParser_ParseFile_FilesEdited(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "claudecode", "basic_session.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Check that file-editing tool calls populate FilesEdited
	foundFilesEdited := false
	for _, message := range parsed.Messages {
		if len(message.FilesEdited) > 0 {
			foundFilesEdited = true
		}
	}

	if !foundFilesEdited {
		t.Error("Expected at least one message with FilesEdited populated")
	}
}

func TestParser_ParseFile_StringContent(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "claudecode", "string_content.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(parsed.Messages) == 0 {
		t.Error("Expected at least one message from string content fixture")
	}
}

func TestParser_ParseFile_NoiseFiltering(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "claudecode", "with_noise.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Noise lines (progress, queue-operation, file-history-snapshot) should be skipped
	// The file has user + assistant messages plus noise — verify only real messages appear
	for _, message := range parsed.Messages {
		if message.Role != "user" && message.Role != "assistant" {
			t.Errorf("Unexpected message role %q — noise should be filtered", message.Role)
		}
	}
}

func TestParser_ParseFile_MultiTool(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "claudecode", "multi_tool.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Count tool call parts across all messages
	toolCallCount := 0
	for _, message := range parsed.Messages {
		for _, part := range message.Parts {
			if part.Type == agent.PartToolCall {
				toolCallCount++
			}
		}
	}

	if toolCallCount < 2 {
		t.Errorf("Expected at least 2 tool calls in multi_tool fixture, got %d", toolCallCount)
	}
}

func TestParser_ParseFile_CommandMessagesFiltered(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "claudecode", "with_commands.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// <command-name> and <local-command-stdout> lines should not appear as user messages
	for _, message := range parsed.Messages {
		if message.Role == "user" {
			if strings.HasPrefix(message.Content, "<command-name>") {
				t.Error("command-name message should have been filtered out")
			}
			if strings.HasPrefix(message.Content, "<local-command-stdout>") {
				t.Error("local-command-stdout message should have been filtered out")
			}
		}
	}

	// Should still have the real user message and assistant message
	userCount := 0
	for _, message := range parsed.Messages {
		if message.Role == "user" {
			userCount++
		}
	}
	if userCount != 1 {
		t.Errorf("Expected 1 real user message, got %d", userCount)
	}
}

func TestExtractLastMessageDate_SkipsCommands(t *testing.T) {
	// The with_commands fixture has real messages at 10:00:00 and 10:00:05,
	// and command messages at 10:01:00. ExtractLastMessageDate should return
	// the timestamp of the last real message, not the command messages.
	ts, err := ExtractLastMessageDate(filepath.Join("..", "..", "testdata", "fixtures", "claudecode", "with_commands.jsonl"))
	if err != nil {
		t.Fatalf("ExtractLastMessageDate failed: %v", err)
	}

	// 2026-03-01T10:00:05.000Z in milliseconds
	expected := int64(1772359205000)
	if ts != expected {
		t.Errorf("Expected last message timestamp %d (10:00:05), got %d", expected, ts)
	}
}

func TestIsLocalCommandContent(t *testing.T) {
	tests := []struct {
		name     string
		content  interface{}
		expected bool
	}{
		{"command-name tag", "<command-name>/exit</command-name>", true},
		{"local-command-stdout tag", "<local-command-stdout>Bye!</local-command-stdout>", true},
		{"regular text", "Hello, how are you?", false},
		{"array content", []interface{}{}, false},
		{"nil", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isLocalCommandContent(tc.content); got != tc.expected {
				t.Errorf("isLocalCommandContent(%v) = %v, want %v", tc.content, got, tc.expected)
			}
		})
	}
}

func TestParser_ParseFile_Malformed(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "claudecode", "malformed.jsonl"))
	// Malformed lines should be skipped, not cause a fatal error
	if err != nil {
		t.Fatalf("ParseFile should handle malformed lines gracefully: %v", err)
	}

	// Malformed fixture has only invalid JSON lines — should result in empty session
	if len(parsed.Messages) != 0 {
		t.Errorf("Expected 0 messages from malformed fixture, got %d", len(parsed.Messages))
	}
}

func TestParser_ParseFile_EmptyFile(t *testing.T) {
	// Create a temporary empty file
	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty.jsonl")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	parsed, err := (&Parser{}).ParseFile(emptyFile)
	if err != nil {
		t.Fatalf("ParseFile should handle empty files: %v", err)
	}

	if len(parsed.Messages) != 0 {
		t.Errorf("Expected 0 messages for empty file, got %d", len(parsed.Messages))
	}
}

func TestParser_WriteParsedSession_RoundTrip(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "claudecode", "basic_session.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	outputDirectory := t.TempDir()
	outputPath, err := agent.WriteParsedSession(parsed, outputDirectory)
	if err != nil {
		t.Fatalf("WriteParsedSession failed: %v", err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Reading output file: %v", err)
	}

	markdown := string(content)
	if !strings.Contains(markdown, parsed.SessionID) {
		t.Error("Output should contain the session ID")
	}
	if !strings.Contains(markdown, "Assistant") {
		t.Error("Output should contain assistant messages")
	}
}

func TestDeriveTitle_ShortMessage(t *testing.T) {
	title := deriveTitle("Fix the bug")
	if title != "Fix the bug" {
		t.Errorf("Expected 'Fix the bug', got %q", title)
	}
}

func TestParser_ParseFile_ThinkingAndResults(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "claudecode", "thinking_and_results.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Session metadata should be extracted from the first non-noise line,
	// not from the file-history-snapshot that precedes it.
	if parsed.SessionID != "session-thinking" {
		t.Errorf("Expected session ID 'session-thinking', got %q", parsed.SessionID)
	}
	if parsed.CreatedAt == "" {
		t.Error("Expected non-empty CreatedAt timestamp")
	}

	// Should have 2 user messages and 2 consolidated assistant messages
	userCount := 0
	assistantCount := 0
	for _, message := range parsed.Messages {
		switch message.Role {
		case "user":
			userCount++
		case "assistant":
			assistantCount++
		}
	}
	if userCount != 2 {
		t.Errorf("Expected 2 user messages, got %d", userCount)
	}
	if assistantCount != 2 {
		t.Errorf("Expected 2 assistant messages (consolidated), got %d", assistantCount)
	}
}

func TestParser_ParseFile_ThinkingBlocks(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "claudecode", "thinking_and_results.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	thinkingCount := 0
	for _, message := range parsed.Messages {
		for _, part := range message.Parts {
			if part.Type == agent.PartThinking {
				thinkingCount++
				if part.Content == "" {
					t.Error("Thinking part should have non-empty content")
				}
			}
		}
	}
	if thinkingCount != 2 {
		t.Errorf("Expected 2 thinking parts (one per assistant turn), got %d", thinkingCount)
	}
}

func TestParser_ParseFile_MessageConsolidation(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "claudecode", "thinking_and_results.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// First assistant message should have: thinking + tool_call + tool_result + text
	// (consolidated from 3 JSONL lines with same message.id, plus a tool result)
	firstAssistant := findNthAssistantMessage(parsed.Messages, 1)
	if firstAssistant == nil {
		t.Fatal("Expected at least 1 assistant message")
	}

	partTypes := make(map[agent.MessagePartType]int)
	for _, part := range firstAssistant.Parts {
		partTypes[part.Type]++
	}

	if partTypes[agent.PartThinking] != 1 {
		t.Errorf("First assistant: expected 1 thinking part, got %d", partTypes[agent.PartThinking])
	}
	if partTypes[agent.PartToolCall] != 1 {
		t.Errorf("First assistant: expected 1 tool call part, got %d", partTypes[agent.PartToolCall])
	}
	if partTypes[agent.PartToolResult] != 1 {
		t.Errorf("First assistant: expected 1 tool result part, got %d", partTypes[agent.PartToolResult])
	}
	if partTypes[agent.PartText] != 1 {
		t.Errorf("First assistant: expected 1 text part, got %d", partTypes[agent.PartText])
	}
}

func TestParser_ParseFile_ToolResults(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "claudecode", "thinking_and_results.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Check that tool results are captured
	toolResultCount := 0
	for _, message := range parsed.Messages {
		for _, part := range message.Parts {
			if part.Type == agent.PartToolResult {
				toolResultCount++
				if part.Content == "" {
					t.Error("Tool result part should have non-empty content")
				}
			}
		}
	}
	if toolResultCount < 2 {
		t.Errorf("Expected at least 2 tool result parts, got %d", toolResultCount)
	}
}

func TestParser_ParseFile_SystemReminderStripped(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "claudecode", "thinking_and_results.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// The second assistant message has a Read tool result with a system reminder.
	// The system reminder should be stripped from the output.
	for _, message := range parsed.Messages {
		for _, part := range message.Parts {
			if part.Type == agent.PartToolResult {
				if strings.Contains(part.Content, "<system-reminder>") {
					t.Error("Tool result should not contain <system-reminder> tags")
				}
				if strings.Contains(part.Content, "Do not share secrets") {
					t.Error("Tool result should not contain system reminder content")
				}
			}
		}
	}
}

func TestParser_ParseFile_MetadataSkipsNoise(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "claudecode", "thinking_and_results.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// The first line is a file-history-snapshot (noise) with no sessionId.
	// Metadata should come from the first non-noise line instead.
	if parsed.SessionID == "" {
		t.Error("SessionID should not be empty — metadata extraction should skip noise lines")
	}
	if parsed.CreatedAtMs == 0 {
		t.Error("CreatedAtMs should not be zero")
	}
}

func TestParser_WriteParsedSession_ThinkingRoundTrip(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "claudecode", "thinking_and_results.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	outputDirectory := t.TempDir()
	outputPath, err := agent.WriteParsedSession(parsed, outputDirectory)
	if err != nil {
		t.Fatalf("WriteParsedSession failed: %v", err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Reading output file: %v", err)
	}

	markdown := string(content)

	// Thinking blocks should be rendered
	if !strings.Contains(markdown, "<thinking>") {
		t.Error("Output should contain thinking blocks")
	}
	if !strings.Contains(markdown, "Let me look at the project structure first.") {
		t.Error("Output should contain thinking content")
	}

	// Tool results should be rendered
	if !strings.Contains(markdown, "main.go") {
		t.Error("Output should contain tool result content")
	}

	// Text parts should be present
	if !strings.Contains(markdown, "This is a Go project") {
		t.Error("Output should contain assistant text")
	}

	// System reminders should NOT be present
	if strings.Contains(markdown, "system-reminder") {
		t.Error("Output should not contain system reminder tags")
	}
}

func TestParser_AgentToolResult(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "claudecode", "with_agent.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	outputDirectory := t.TempDir()
	outputPath, err := agent.WriteParsedSession(parsed, outputDirectory)
	if err != nil {
		t.Fatalf("WriteParsedSession failed: %v", err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Reading output file: %v", err)
	}

	markdown := string(content)

	// Agent tool call should show the description
	if !strings.Contains(markdown, "Agent: Analyze cursor change detection") {
		t.Error("Output should contain Agent tool call with description")
	}

	// Agent result (the analysis) should be present as rendered text
	if !strings.Contains(markdown, "timestamp-based tracking") {
		t.Error("Output should contain the Agent's analysis content")
	}

	// Markdown tables from agent result should be preserved
	if !strings.Contains(markdown, "| driver.go | Main logic |") {
		t.Error("Output should preserve markdown tables from Agent result")
	}

	// Agent metadata should be stripped
	if strings.Contains(markdown, "agentId:") {
		t.Error("Output should not contain agentId metadata")
	}
	if strings.Contains(markdown, "<usage>") {
		t.Error("Output should not contain <usage> metadata")
	}
}

func TestStripSystemReminders(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no reminders",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "reminder at end",
			input:    "content here\n\n<system-reminder>\nDo not share.\n</system-reminder>\n",
			expected: "content here",
		},
		{
			name:     "reminder in middle",
			input:    "before<system-reminder>secret</system-reminder>after",
			expected: "beforeafter",
		},
		{
			name:     "unclosed reminder",
			input:    "content\n<system-reminder>\nstuff",
			expected: "content",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := stripSystemReminders(testCase.input)
			if result != testCase.expected {
				t.Errorf("Expected %q, got %q", testCase.expected, result)
			}
		})
	}
}

// findNthAssistantMessage returns the nth (1-based) assistant message from a slice.
func findNthAssistantMessage(messages []agent.ParsedMessage, n int) *agent.ParsedMessage {
	count := 0
	for i := range messages {
		if messages[i].Role == "assistant" {
			count++
			if count == n {
				return &messages[i]
			}
		}
	}
	return nil
}

func TestDeriveTitle_LongMessage(t *testing.T) {
	longMessage := strings.Repeat("a", 100)
	title := deriveTitle(longMessage)
	if len(title) > 84 { // 80 + "..."
		t.Errorf("Title should be truncated, got length %d", len(title))
	}
	if !strings.HasSuffix(title, "...") {
		t.Error("Truncated title should end with ...")
	}
}

func TestDeriveTitle_MultiLine(t *testing.T) {
	title := deriveTitle("First line\nSecond line\nThird line")
	if title != "First line" {
		t.Errorf("Expected 'First line', got %q", title)
	}
}

func TestSummarizeToolCall(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    map[string]interface{}
		expected string
	}{
		{
			name:     "Read",
			toolName: "Read",
			input:    map[string]interface{}{"file_path": "/src/main.go"},
			expected: "Read `/src/main.go`",
		},
		{
			name:     "Edit",
			toolName: "Edit",
			input:    map[string]interface{}{"file_path": "/src/app.go"},
			expected: "Edited `/src/app.go`",
		},
		{
			name:     "Write",
			toolName: "Write",
			input:    map[string]interface{}{"file_path": "/src/new.go"},
			expected: "Created `/src/new.go`",
		},
		{
			name:     "Bash",
			toolName: "Bash",
			input:    map[string]interface{}{"command": "go build ./..."},
			expected: "Ran `go build ./...`",
		},
		{
			name:     "Grep",
			toolName: "Grep",
			input:    map[string]interface{}{"pattern": "TODO"},
			expected: "Grep `TODO`",
		},
		{
			name:     "Glob",
			toolName: "Glob",
			input:    map[string]interface{}{"pattern": "**/*.go"},
			expected: "Glob `**/*.go`",
		},
		{
			name:     "WebSearch",
			toolName: "WebSearch",
			input:    map[string]interface{}{"query": "Go concurrency patterns"},
			expected: "Search `Go concurrency patterns`",
		},
		{
			name:     "WebFetch",
			toolName: "WebFetch",
			input:    map[string]interface{}{"url": "https://example.com"},
			expected: "Fetch `https://example.com`",
		},
		{
			name:     "Task",
			toolName: "Task",
			input:    map[string]interface{}{"description": "Refactor the module"},
			expected: "Task: Refactor the module",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := summarizeToolCall(testCase.toolName, testCase.input)
			if result != testCase.expected {
				t.Errorf("Expected %q, got %q", testCase.expected, result)
			}
		})
	}
}

func TestDecodeProjectPath(t *testing.T) {
	tests := []struct {
		encoded  string
		expected string
	}{
		{
			encoded:  "-Users-user-projects-foo",
			expected: "/Users/user/projects/foo",
		},
		{
			encoded:  "-home-user-code",
			expected: "/home/user/code",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.encoded, func(t *testing.T) {
			result := decodeProjectPath(testCase.encoded)
			if result != testCase.expected {
				t.Errorf("Expected %q, got %q", testCase.expected, result)
			}
		})
	}
}
