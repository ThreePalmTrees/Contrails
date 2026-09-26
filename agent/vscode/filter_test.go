package vscode

import (
	"path/filepath"
	"strings"
	"testing"

	"contrails/agent"
)

// TestApplyContrailFilters_RemovesCopilotThinking verifies the Thinking
// filter works for Copilot: thinking is emitted as PartThinking, so
// ApplyContrailFilters removes it, and the rendered markdown contains no
// thinking block at all.
func TestApplyContrailFilters_RemovesCopilotThinking(t *testing.T) {
	parsed, err := (&Parser{}).ParseFile(filepath.Join("..", "..", "testdata", "fixtures", "vscode", "thinking_and_edits.jsonl"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	agent.ApplyContrailFilters(parsed, agent.ContrailFilters{
		Thinking:        false,
		ToolCalls:       true,
		SubagentContent: true,
	})

	for _, message := range parsed.Messages {
		for _, part := range message.Parts {
			if part.Type == agent.PartThinking {
				t.Errorf("Thinking part survived filter: %q", part.Content)
			}
		}
	}

	if markdown := agent.RenderMarkdown(parsed); strings.Contains(markdown, "<thinking") {
		t.Errorf("Rendered markdown still contains a thinking block:\n%s", markdown)
	}
}
