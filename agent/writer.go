package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// RenderMarkdown renders a parsed session to a markdown string without any
// file I/O. This is the pure rendering step — use WriteParsedSession to also
// write the result to disk.
func RenderMarkdown(session *ParsedSession) string {
	var markdown strings.Builder

	displayTitle := session.Title
	if displayTitle == "" {
		displayTitle = session.SessionID
	}
	markdown.WriteString(fmt.Sprintf("# %s\n\n", displayTitle))
	markdown.WriteString(fmt.Sprintf("- **Session ID:** `%s`\n", session.SessionID))
	markdown.WriteString(fmt.Sprintf("- **Created:** %s\n", session.CreatedAt))
	markdown.WriteString(fmt.Sprintf("- **Last Message:** %s\n", session.LastMessageAt))
	markdown.WriteString(fmt.Sprintf("- **User:** %s\n", session.User))
	markdown.WriteString(fmt.Sprintf("- **Agent:** %s\n", session.Agent))
	if session.Model != "" {
		markdown.WriteString(fmt.Sprintf("- **Model:** %s\n", session.Model))
	}
	markdown.WriteString("\n---\n\n")

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
		} else {
			markdown.WriteString(fmt.Sprintf("## 🤖 Assistant - %s\n\n", message.Timestamp))
			if message.Model != "" {
				markdown.WriteString(fmt.Sprintf("*Model: %s*\n\n", message.Model))
			}
			writeInterleavedParts(&markdown, message.Parts)
			if len(message.FilesEdited) > 0 {
				markdown.WriteString("### Files Edited\n\n")
				for _, file := range message.FilesEdited {
					markdown.WriteString(fmt.Sprintf("- `%s`\n", file))
				}
				markdown.WriteString("\n")
			}
			if message.MaxToolCallsExceeded {
				markdown.WriteString("*⚠️ Max tool calls exceeded - the agent hit its tool call limit.*\n\n")
			}
			if message.Canceled {
				markdown.WriteString("*⚠️ This response was canceled.*\n\n")
			}
		}
		markdown.WriteString("---\n\n")
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
	timestampPrefix := ""
	if session.CreatedAtMs > 0 {
		timestampPrefix = strconv.FormatInt(session.CreatedAtMs/1000, 10) + " - "
	}

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

// writeInterleavedParts renders message parts in order, grouping consecutive
// tool calls under a single "### Tool Calls" heading.
func writeInterleavedParts(markdown *strings.Builder, parts []MessagePart) {
	inToolGroup := false

	for _, part := range parts {
		switch part.Type {
		case PartText:
			if inToolGroup {
				markdown.WriteString("\n")
				inToolGroup = false
			}
			markdown.WriteString(part.Content)
			markdown.WriteString("\n\n")

		case PartToolCall:
			if !inToolGroup {
				markdown.WriteString("### Tool Calls\n\n")
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

		case PartFileEdit, PartCodeBlock:
			// These are interleaved with tool calls, keep them in the flow
			if !inToolGroup {
				markdown.WriteString("### Tool Calls\n\n")
				inToolGroup = true
			}
			action := "Edited"
			if part.Type == PartCodeBlock && !part.IsEdit {
				action = "Code block"
			}
			markdown.WriteString(fmt.Sprintf("- *%s:* `%s`\n", action, part.FilePath))

		case PartThinking:
			if inToolGroup {
				markdown.WriteString("\n")
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
		markdown.WriteString("\n")
	}
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
