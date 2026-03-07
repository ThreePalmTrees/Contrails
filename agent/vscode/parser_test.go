package vscode

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

func TestParser_ParseFile_Minimal(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "vscode", "minimal.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if parsed.SessionID != "jsonl-minimal-123" {
		t.Errorf("Expected SessionID 'jsonl-minimal-123', got %q", parsed.SessionID)
	}
	if parsed.User != "TestUser" {
		t.Errorf("Expected User 'TestUser', got %q", parsed.User)
	}
	if parsed.Agent != "copilot" {
		t.Errorf("Expected Agent 'copilot', got %q", parsed.Agent)
	}
	if len(parsed.Messages) == 0 {
		t.Error("Expected at least one message")
	}
	if len(parsed.Messages) >= 2 {
		// Check user message
		if parsed.Messages[0].Role != "user" {
			t.Errorf("Expected first message role 'user', got %q", parsed.Messages[0].Role)
		}
		if parsed.Messages[0].Content != "Hello" {
			t.Errorf("Expected user content 'Hello', got %q", parsed.Messages[0].Content)
		}
		// Check assistant message
		if parsed.Messages[1].Role != "assistant" {
			t.Errorf("Expected second message role 'assistant', got %q", parsed.Messages[1].Role)
		}
	}
}

func TestParser_ParseFile_WithTitle(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "vscode", "with_title.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if parsed.Title != "My Custom JSONL Title" {
		t.Errorf("Expected title 'My Custom JSONL Title', got %q", parsed.Title)
	}
	if parsed.Agent != "GitHub Copilot" {
		t.Errorf("Expected Agent 'GitHub Copilot', got %q", parsed.Agent)
	}
}

func TestParser_ParseFile_ToolCalls(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "vscode", "tool_calls.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	foundToolCall := false
	for _, message := range parsed.Messages {
		for _, part := range message.Parts {
			if part.Type == agent.PartToolCall {
				foundToolCall = true
				if part.Tool != "read_file" {
					t.Errorf("Expected tool 'read_file', got %q", part.Tool)
				}
				break
			}
		}
	}
	if !foundToolCall {
		t.Error("Expected at least one tool call part")
	}
}

func TestParser_ParseFile_EmptyRequests(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "vscode", "empty_requests.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(parsed.Messages) != 0 {
		t.Errorf("Expected 0 messages for empty requests, got %d", len(parsed.Messages))
	}
}

func TestParser_ParseFile_NoTitle(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "vscode", "no_title.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if parsed.Title != "" {
		t.Errorf("Expected empty title, got %q", parsed.Title)
	}
}

func TestParser_ParseFile_Malformed(t *testing.T) {
	_, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "vscode", "malformed.jsonl"))
	if err == nil {
		t.Error("Expected error for malformed JSONL")
	}
}

func TestParser_ParseFile_ThinkingAndEdits(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "vscode", "thinking_and_edits.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if parsed.Title != "Session with thinking and file edits" {
		t.Errorf("Unexpected title: %q", parsed.Title)
	}
	if parsed.Model != "copilot/claude-opus-4.6" {
		t.Errorf("Unexpected model: %q", parsed.Model)
	}

	// Check that we have thinking, tool call, file edit, and text parts
	foundThinking := false
	foundToolCall := false
	foundFileEdit := false
	foundText := false
	for _, message := range parsed.Messages {
		for _, part := range message.Parts {
			switch part.Type {
			case agent.PartText:
				if strings.Contains(part.Content, "<thinking") {
					foundThinking = true
				} else {
					foundText = true
				}
			case agent.PartToolCall:
				foundToolCall = true
			case agent.PartFileEdit:
				foundFileEdit = true
				if part.FilePath != "/Users/test/project/hello.go" {
					t.Errorf("Expected file path '/Users/test/project/hello.go', got %q", part.FilePath)
				}
			}
		}
		if len(message.FilesEdited) > 0 {
			if message.FilesEdited[0] != "/Users/test/project/hello.go" {
				t.Errorf("Expected filesEdited[0] '/Users/test/project/hello.go', got %q", message.FilesEdited[0])
			}
		}
	}
	if !foundThinking {
		t.Error("Expected a thinking part")
	}
	if !foundToolCall {
		t.Error("Expected a tool call part")
	}
	if !foundFileEdit {
		t.Error("Expected a file edit part")
	}
	if !foundText {
		t.Error("Expected a plain text part")
	}
}

func TestParser_ParseFile_LastMessageDate(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "vscode", "minimal.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// LastMessageAt should be computed from the completedAt timestamp
	if parsed.LastMessageAt == "" {
		t.Error("Expected non-empty LastMessageAt")
	}
}

func TestExtractLastMessageDate(t *testing.T) {
	lastMessageDate, err := ExtractLastMessageDate(filepath.Join("..", "..", "testdata", "fixtures", "vscode", "minimal.jsonl"))
	if err != nil {
		t.Fatalf("ExtractLastMessageDate failed: %v", err)
	}

	// Should pick up the completedAt from the modelState patch
	if lastMessageDate != 1708000060000 {
		t.Errorf("Expected 1708000060000, got %d", lastMessageDate)
	}
}

func TestParser_ParseFile_SubAgentCalls(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "vscode", "subagent.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if parsed.Title != "Session with subagent calls" {
		t.Errorf("Unexpected title: %q", parsed.Title)
	}

	// Locate the assistant message.
	var assistantMsg *agent.ParsedMessage
	for i := range parsed.Messages {
		if parsed.Messages[i].Role == "assistant" {
			assistantMsg = &parsed.Messages[i]
			break
		}
	}
	if assistantMsg == nil {
		t.Fatal("No assistant message found")
	}

	// Collect tool call parts in order.
	var toolParts []agent.MessagePart
	for _, p := range assistantMsg.Parts {
		if p.Type == agent.PartToolCall {
			toolParts = append(toolParts, p)
		}
	}

	// All three tool calls must be present: top-level runSubagent + 2 children.
	if len(toolParts) != 3 {
		t.Fatalf("Expected 3 tool call parts (1 top-level + 2 children), got %d", len(toolParts))
	}

	// First: the top-level runSubagent.
	if toolParts[0].Tool != "runSubagent" {
		t.Errorf("Expected first tool 'runSubagent', got %q", toolParts[0].Tool)
	}

	// Second: copilot_findFiles (first child) with result files.
	if toolParts[1].Tool != "copilot_findFiles" {
		t.Errorf("Expected second tool 'copilot_findFiles', got %q", toolParts[1].Tool)
	}
	if toolParts[1].ToolDetail == nil {
		t.Fatal("Expected ToolDetail on copilot_findFiles (for result files)")
	}
	if len(toolParts[1].ToolDetail.ResultFiles) != 2 {
		t.Fatalf("Expected 2 result files, got %d", len(toolParts[1].ToolDetail.ResultFiles))
	}
	if toolParts[1].ToolDetail.ResultFiles[0] != "/project/src/main.ts" {
		t.Errorf("Expected ResultFiles[0] '/project/src/main.ts', got %q", toolParts[1].ToolDetail.ResultFiles[0])
	}
	if toolParts[1].ToolDetail.ResultFiles[1] != "/project/src/utils.ts" {
		t.Errorf("Expected ResultFiles[1] '/project/src/utils.ts', got %q", toolParts[1].ToolDetail.ResultFiles[1])
	}

	// Third: copilot_readFile (second child).
	if toolParts[2].Tool != "copilot_readFile" {
		t.Errorf("Expected third tool 'copilot_readFile', got %q", toolParts[2].Tool)
	}

	// Text parts must appear in the correct positions (before and after the tool group).
	var textParts []agent.MessagePart
	for _, p := range assistantMsg.Parts {
		if p.Type == agent.PartText {
			textParts = append(textParts, p)
		}
	}
	if len(textParts) != 2 {
		t.Fatalf("Expected 2 text parts, got %d", len(textParts))
	}
	if textParts[0].Content != "Let me research that for you." {
		t.Errorf("Unexpected first text part: %q", textParts[0].Content)
	}
	if textParts[1].Content != "Here is what I found." {
		t.Errorf("Unexpected second text part: %q", textParts[1].Content)
	}
}

func TestParser_WriteParsedSession_RoundTrip(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "vscode", "with_title.jsonl"))
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
	if !strings.Contains(markdown, parsed.Title) {
		t.Error("Output should contain the session title")
	}
	if !strings.Contains(markdown, parsed.SessionID) {
		t.Error("Output should contain the session ID")
	}
}
