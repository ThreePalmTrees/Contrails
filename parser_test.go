package main

import (
	"contrails/agent"
	"contrails/agent/vscode"
	"os"
	"strings"
	"testing"
)

func TestParseChatSessionFile(t *testing.T) {
	parser := &vscode.Parser{}
	parsed, err := parser.ParseFile("testdata/fixtures/vscode/comprehensive.jsonl")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if parsed.SessionID == "" {
		t.Error("SessionID is empty")
	}
	if parsed.Title == "" {
		t.Error("Title is empty")
	}

	// 10 requests → 20 messages (10 user + 10 assistant)
	if len(parsed.Messages) != 20 {
		t.Errorf("Expected 20 messages, got %d", len(parsed.Messages))
	}

	// Message 0 (user): should have the attached file
	if parsed.Messages[0].Role != "user" {
		t.Error("Message 0 should be a user message")
	}
	if len(parsed.Messages[0].Attachments) == 0 {
		t.Error("User message should have attachments")
	}

	// Message 1 (assistant): should have rich interleaved parts
	assistant := parsed.Messages[1]
	if assistant.Role != "assistant" {
		t.Error("Message 1 should be an assistant message")
	}
	if len(assistant.Parts) < 50 {
		t.Errorf("Expected many assistant parts (tool calls, thinking, text, edits), got %d", len(assistant.Parts))
	}

	// Count part types
	textCount, toolCount, editCount := 0, 0, 0
	for _, p := range assistant.Parts {
		switch p.Type {
		case PartText:
			textCount++
		case PartToolCall:
			toolCount++
		case PartFileEdit:
			editCount++
		}
	}
	if toolCount == 0 {
		t.Error("Expected tool call parts in assistant response")
	}
	if editCount == 0 {
		t.Error("Expected file edit parts in assistant response")
	}
	if textCount == 0 {
		t.Error("Expected text parts in assistant response")
	}
	if len(assistant.FilesEdited) == 0 {
		t.Error("Assistant response should have files edited")
	}

	// Write output and verify markdown
	outPath, err := agent.WriteParsedSession(parsed, t.TempDir())
	if err != nil {
		t.Fatalf("WriteParsedSession failed: %v", err)
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	if !strings.Contains(string(content), "Tool Calls") {
		t.Error("Expected 'Tool Calls' section in output")
	}
}
