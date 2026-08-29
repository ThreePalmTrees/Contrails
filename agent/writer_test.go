package agent

import (
	"strings"
	"testing"
)

// A tool result that itself contains a fenced code block must not end the
// fence that wraps it. When it did, the rest of the result leaked into the
// document and any HTML in it (e.g. a <details> tag echoed from an earlier
// contrail) unbalanced the collapsible section around the tool group.
func TestRenderMarkdown_ToolResultWithFence_KeepsDetailsBalanced(t *testing.T) {
	session := &ParsedSession{
		Agent: "Claude Code",
		Messages: []ParsedMessage{
			{
				Role: "assistant",
				Parts: []MessagePart{
					{Type: PartToolCall, Tool: "Bash", ToolArgs: "cat contrail.md"},
					{Type: PartToolResult, Content: "<details>\n<summary>Tool Calls (3)</summary>\n\n```\nsome output\n```\n"},
				},
			},
		},
	}

	markdown := RenderMarkdown(session)
	outside := outsideCodeFences(markdown)

	if opens, closes := strings.Count(outside, "<details>"), strings.Count(outside, "</details>"); opens != closes {
		t.Errorf("Unbalanced details tags: %d opening, %d closing", opens, closes)
	}
	if !strings.Contains(markdown, "````") {
		t.Error("Expected the wrapping fence to be longer than the fence inside the result")
	}
	if strings.Count(markdown, "````") != 2 {
		t.Errorf("Expected exactly one wrapping fence pair, got %d fence lines", strings.Count(markdown, "````"))
	}
}

func TestCodeFenceFor(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"plain text", "hello\nworld", "```"},
		{"inline backticks only", "run `ls` first", "```"},
		{"contains a fence", "```\ncode\n```", "````"},
		{"contains an indented fence", "  ```go\ncode\n  ```", "````"},
		{"contains a longer fence", "````\n```\ncode\n```\n````", "`````"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := codeFenceFor(testCase.content); got != testCase.want {
				t.Errorf("codeFenceFor(%q) = %q, want %q", testCase.content, got, testCase.want)
			}
		})
	}
}

// outsideCodeFences drops every fenced code block from markdown so that tags
// quoted inside a tool result are not mistaken for markup of the document.
// A fence closes only on a run of backticks at least as long as the one that
// opened it, which is the rule the renderer follows.
func outsideCodeFences(markdown string) string {
	var kept []string
	openFence := ""
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		run := 0
		for run < len(trimmed) && trimmed[run] == '`' {
			run++
		}
		if openFence != "" {
			if run >= len(openFence) {
				openFence = ""
			}
			continue
		}
		if run >= 3 {
			openFence = strings.Repeat("`", run)
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
