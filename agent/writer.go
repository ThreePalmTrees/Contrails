package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// RenderMarkdown renders a parsed session to a markdown string without any
// file I/O. This is the pure rendering step — use WriteParsedSession to also
// write the result to disk.
func RenderMarkdown(session *ParsedSession) string {
	var markdown strings.Builder

	markdown.WriteString(fmt.Sprintf("- **Session ID:** `%s`\n", session.SessionID))
	markdown.WriteString(fmt.Sprintf("- **Created:** %s\n", session.CreatedAt))
	markdown.WriteString(fmt.Sprintf("- **Last Message:** %s\n", session.LastMessageAt))
	markdown.WriteString(fmt.Sprintf("- **User:** %s\n", session.User))
	markdown.WriteString(fmt.Sprintf("- **Agent:** %s\n", session.Agent))
	if session.Model != "" {
		markdown.WriteString(fmt.Sprintf("- **Model:** %s\n", session.Model))
	}
	markdown.WriteString("\n")

	for _, message := range session.Messages {
		if message.Role == "user" {
			if message.Confirmation != "" {
				markdown.WriteString(fmt.Sprintf("## 🧑 User - %s *(%s)*\n\n", message.Timestamp, message.Confirmation))
			} else {
				markdown.WriteString(fmt.Sprintf("## 🧑 User - %s\n\n", message.Timestamp))
			}
			markdown.WriteString(message.Content)
			markdown.WriteString("\n\n")
			if len(message.Attachments) > 0 {
				markdown.WriteString("**Attachments:** ")
				for i, attachment := range message.Attachments {
					if i > 0 {
						markdown.WriteString(", ")
					}
					markdown.WriteString(fmt.Sprintf("`%s`", attachment))
				}
				markdown.WriteString("\n\n")
			}
			if len(message.Images) > 0 {
				for i, img := range message.Images {
					if len(message.Images) > 1 {
						markdown.WriteString(fmt.Sprintf("**Image %d:**\n\n", i+1))
					}
					markdown.WriteString(fmt.Sprintf("{{CONTRAIL_IMAGE:%s;base64,%s}}\n\n", img.MediaType, img.Data))
				}
			}
		} else {
			markdown.WriteString(fmt.Sprintf("## 🤖 Assistant - %s\n\n", message.Timestamp))
			if message.Model != "" {
				markdown.WriteString(fmt.Sprintf("*Model: %s*\n\n", message.Model))
			}
			writeInterleavedParts(&markdown, message.Parts)

			if message.MaxToolCallsExceeded {
				markdown.WriteString("*⚠️ Max tool calls exceeded - the agent hit its tool call limit.*\n\n")
			}
			if message.Canceled {
				markdown.WriteString("*⚠️ This response was canceled.*\n\n")
			}
		}
	}

	return markdown.String()
}

// WriteParsedSession writes the parsed session as a markdown file.
// It renders assistant messages with interleaved text, tool calls,
// and file edits in the order they occurred. This function is
// agent-agnostic — both VS Code and Claude Code sessions use it.
func WriteParsedSession(session *ParsedSession, outputDirectory string) (string, error) {
	if err := os.MkdirAll(outputDirectory, 0755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	// Build filename: prefix with unix timestamp (seconds) for chronological sorting.
	// Use the title when available, fall back to sessionID.
	// Clean up the other variant to prevent duplicates when the title
	// is added or changed after the session was first processed.
	// Performance: Prefer strconv over fmt (go-style-guide.md)
	//
	// If no timestamp was parsed from the session data, fall back to the current
	// time so every contrail file is consistently prefixed for sorting.
	createdMs := session.CreatedAtMs
	if createdMs == 0 {
		createdMs = time.Now().UnixMilli()
	}
	timestampPrefix := strconv.FormatInt(createdMs/1000, 10) + " - "

	var baseName string
	if session.Title != "" {
		baseName = SanitizeFilename(session.Title)
	} else {
		baseName = session.SessionID
	}
	filename := timestampPrefix + baseName + ".md"

	// Remove old files that no longer match the current filename
	// (handles title changes, date prefix additions, sessionID→title transitions)
	idFilename := session.SessionID + ".md"
	if idFilename != filename {
		os.Remove(filepath.Join(outputDirectory, idFilename))
	}
	// Also scan for any previously-titled file for this session ID
	// (handles title *changes*, e.g. "Old Title.md" → "New Title.md")
	existingFiles, _ := os.ReadDir(outputDirectory)
	for _, file := range existingFiles {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") || file.Name() == filename {
			continue
		}
		// Read just enough of the file to check if it contains this session ID
		filePath := filepath.Join(outputDirectory, file.Name())
		head, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		if strings.Contains(string(head[:min(len(head), 512)]), session.SessionID) {
			os.Remove(filePath)
		}
	}

	outputPath := filepath.Join(outputDirectory, filename)

	if err := os.WriteFile(outputPath, []byte(RenderMarkdown(session)), 0644); err != nil {
		return "", fmt.Errorf("writing output: %w", err)
	}

	return outputPath, nil
}

// countToolGroup counts the number of top-level tool calls in a consecutive
// tool group starting at index start. It only counts PartToolCall and
// PartFileEdit/PartCodeBlock (not sub-agent tool calls nested in SubParts).
func countToolGroup(parts []MessagePart, start int) int {
	count := 0
	for i := start; i < len(parts); i++ {
		switch parts[i].Type {
		case PartToolCall, PartFileEdit, PartCodeBlock:
			count++
		case PartToolResult:
			// results stay in the group but don't count
		default:
			return count
		}
	}
	return count
}

// writeInterleavedParts renders message parts in order, grouping consecutive
// tool calls under a single collapsible section with a count in the summary.
func writeInterleavedParts(markdown *strings.Builder, parts []MessagePart) {
	inToolGroup := false

	for i := 0; i < len(parts); i++ {
		part := parts[i]
		switch part.Type {
		case PartText:
			if inToolGroup {
				markdown.WriteString("\n</details>\n\n")
				inToolGroup = false
			}
			markdown.WriteString(part.Content)
			markdown.WriteString("\n\n")

		case PartToolCall:
			if !inToolGroup {
				count := countToolGroup(parts, i)
				summary := "Tool Calls"
				if count > 0 {
					summary = fmt.Sprintf("Tool Calls (%d)", count)
				}
				markdown.WriteString(fmt.Sprintf("<details>\n<summary>%s</summary>\n\n", summary))
				inToolGroup = true
			}
			// Render based on tool-specific detail when available
			if part.ToolDetail != nil {
				writeToolDetailPart(markdown, part)
			} else {
				markdown.WriteString(fmt.Sprintf("- **%s**", part.Tool))
				if part.ToolArgs != "" {
					markdown.WriteString(fmt.Sprintf(": `%s`", part.ToolArgs))
				}
				markdown.WriteString("\n")
			}

			// Render subagent conversation as a nested collapsed section
			if len(part.SubParts) > 0 {
				writeSubagentParts(markdown, part.SubParts)
			}

		case PartFileEdit, PartCodeBlock:
			// These are interleaved with tool calls, keep them in the flow
			if !inToolGroup {
				count := countToolGroup(parts, i)
				summary := "Tool Calls"
				if count > 0 {
					summary = fmt.Sprintf("Tool Calls (%d)", count)
				}
				markdown.WriteString(fmt.Sprintf("<details>\n<summary>%s</summary>\n\n", summary))
				inToolGroup = true
			}
			action := "Edited"
			if part.Type == PartCodeBlock && !part.IsEdit {
				action = "Code block"
			}
			markdown.WriteString(fmt.Sprintf("- *%s:* `%s`\n", action, part.FilePath))

		case PartThinking:
			if inToolGroup {
				markdown.WriteString("\n</details>\n\n")
				inToolGroup = false
			}
			markdown.WriteString("<thinking>\n")
			markdown.WriteString(part.Content)
			markdown.WriteString("\n</thinking>\n\n")

		case PartToolResult:
			// Tool results render as indented code blocks within tool groups
			content := strings.TrimRight(part.Content, "\n\r\t ")
			if content != "" {
				markdown.WriteString("  ```\n")
				for _, resultLine := range strings.Split(content, "\n") {
					markdown.WriteString(fmt.Sprintf("  %s\n", resultLine))
				}
				markdown.WriteString("  ```\n")
			}

		case PartReference:
			// Inline references are now merged into text parts during parsing.
			// This case should not be reached, but handle gracefully.
		}
	}

	if inToolGroup {
		markdown.WriteString("\n</details>\n\n")
	}
}

// writeSubagentParts renders a subagent's conversation as a nested collapsed section.
// Each tool call (with its result) is wrapped in its own nested <details> block.
func writeSubagentParts(markdown *strings.Builder, subParts []MessagePart) {
	// Count tool calls for the summary line
	toolCount := 0
	for _, sp := range subParts {
		if sp.Type == PartToolCall {
			toolCount++
		}
	}

	summary := "Subagent Activity"
	if toolCount > 0 {
		summary = fmt.Sprintf("Subagent Activity (%d tool calls)", toolCount)
	}

	markdown.WriteString(fmt.Sprintf("\n<details>\n<summary>%s</summary>\n\n", summary))

	for i := 0; i < len(subParts); i++ {
		sp := subParts[i]
		switch sp.Type {
		case PartText:
			markdown.WriteString(fmt.Sprintf("%s\n\n", sp.Content))
		case PartToolCall:
			// Build the summary line for this tool call
			toolSummary := sp.Tool
			if sp.ToolArgs != "" {
				toolSummary = fmt.Sprintf("%s: `%s`", sp.Tool, sp.ToolArgs)
			}
			// Check if the next part is a tool result to nest inside
			var resultContent string
			if i+1 < len(subParts) && subParts[i+1].Type == PartToolResult {
				resultContent = strings.TrimRight(subParts[i+1].Content, "\n\r\t ")
				i++ // consume the result
			}
			// Read tool calls have no result content — render as plain list item
			if sp.Tool == "Read" || resultContent == "" {
				markdown.WriteString(fmt.Sprintf("- **%s**", sp.Tool))
				if sp.ToolArgs != "" {
					markdown.WriteString(fmt.Sprintf(": `%s`", sp.ToolArgs))
				}
				markdown.WriteString("\n")
			} else {
				markdown.WriteString(fmt.Sprintf("<details>\n<summary>%s</summary>\n\n", toolSummary))
				markdown.WriteString("```\n")
				for _, line := range strings.Split(resultContent, "\n") {
					markdown.WriteString(fmt.Sprintf("%s\n", line))
				}
				markdown.WriteString("```\n")
				markdown.WriteString("\n</details>\n")
			}
		case PartToolResult:
			// Standalone result (not preceded by a tool call) — render inline
			content := strings.TrimRight(sp.Content, "\n\r\t ")
			if content != "" {
				markdown.WriteString("```\n")
				for _, line := range strings.Split(content, "\n") {
					markdown.WriteString(fmt.Sprintf("%s\n", line))
				}
				markdown.WriteString("```\n")
			}
		}
	}

	markdown.WriteString("\n</details>\n\n")
}

// writeToolDetailPart renders a tool call with rich detail from toolSpecificData.
// Result files (from search tools) are always rendered as sub-bullets after the
// main tool line, regardless of the tool kind.
func writeToolDetailPart(markdown *strings.Builder, part MessagePart) {
	detail := part.ToolDetail
	switch detail.Kind {
	case "terminal":
		if detail.Command != "" {
			markdown.WriteString(fmt.Sprintf("- **run_in_terminal**: `%s`\n", detail.Command))
		} else {
			// Fallback if command not available
			markdown.WriteString(fmt.Sprintf("- **%s**", part.Tool))
			if part.ToolArgs != "" {
				markdown.WriteString(fmt.Sprintf(": `%s`", part.ToolArgs))
			}
			markdown.WriteString("\n")
		}

	case "todoList":
		if len(detail.Todos) > 0 {
			markdown.WriteString(fmt.Sprintf("- **manage_todo_list**: `%s`\n", part.ToolArgs))
			for _, todo := range detail.Todos {
				icon := "⬜"
				switch todo.Status {
				case "completed":
					icon = "✅"
				case "in-progress":
					icon = "🔄"
				}
				markdown.WriteString(fmt.Sprintf("  - %s %s\n", icon, todo.Title))
			}
		} else {
			markdown.WriteString(fmt.Sprintf("- **%s**", part.Tool))
			if part.ToolArgs != "" {
				markdown.WriteString(fmt.Sprintf(": `%s`", part.ToolArgs))
			}
			markdown.WriteString("\n")
		}

	default:
		markdown.WriteString(fmt.Sprintf("- **%s**", part.Tool))
		if part.ToolArgs != "" {
			markdown.WriteString(fmt.Sprintf(": `%s`", part.ToolArgs))
		}
		markdown.WriteString("\n")
	}

	// Render file paths returned by search tools (e.g. copilot_findFiles).
	for _, f := range detail.ResultFiles {
		markdown.WriteString(fmt.Sprintf("  - `%s`\n", f))
	}
}
