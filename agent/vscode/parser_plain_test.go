package vscode

import (
	"testing"
)

func TestPlainJSONParsing(t *testing.T) {
	parser := &Parser{}
	parsed, err := parser.ParseFile("../../testdata/fixtures/vscode/plain.json")
	if err != nil {
		t.Fatalf("Failed to parse plain.json: %v", err)
	}

	if parsed.Title != "General greeting inquiry" {
		t.Errorf("Expected title 'General greeting inquiry', got '%s'", parsed.Title)
	}
	if parsed.SessionID != "b4f38011-2e7a-4ef1-9ae0-274197e8c8c6" {
		t.Errorf("Expected session ID 'b4f38011-...', got '%s'", parsed.SessionID)
	}
	if len(parsed.Messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(parsed.Messages))
	}
	if parsed.Messages[1].Role != "assistant" {
		t.Errorf("Expected second message role to be 'assistant', got '%s'", parsed.Messages[1].Role)
	}
}

func TestPlainJSONParsing_ThinkingArrayText(t *testing.T) {
	// VSCode sometimes serializes thinking.text as an empty array []
	// instead of a string. The parser must handle this gracefully.
	parser := &Parser{}
	parsed, err := parser.ParseFile("../../testdata/fixtures/vscode/thinking_array_text.json")
	if err != nil {
		t.Fatalf("Failed to parse thinking_array_text.json: %v", err)
	}

	if parsed.Title != "Thinking array text test" {
		t.Errorf("Expected title 'Thinking array text test', got '%s'", parsed.Title)
	}
	if len(parsed.Messages) != 2 {
		t.Fatalf("Expected 2 messages (user + assistant), got %d", len(parsed.Messages))
	}

	assistant := parsed.Messages[1]
	if assistant.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", assistant.Role)
	}
	if assistant.Content == "" {
		t.Error("Expected non-empty assistant content")
	}
}

func TestExtractTitlePlain(t *testing.T) {
	title := ExtractTitle("../../testdata/fixtures/vscode/plain.json")
	if title != "General greeting inquiry" {
		t.Errorf("Expected 'General greeting inquiry', got '%s'", title)
	}
}

func TestExtractLastMessageDatePlain(t *testing.T) {
	date, err := ExtractLastMessageDate("../../testdata/fixtures/vscode/plain.json")
	if err != nil {
		t.Fatalf("ExtractLastMessageDate failed: %v", err)
	}
	if date != 1772147926365 {
		t.Errorf("Expected 1772147926365, got %d", date)
	}
}
