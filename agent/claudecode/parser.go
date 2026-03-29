// Package claudecode implements the Claude Code session parser.
// It reads Claude Code's JSONL transcript files and produces
// agent.ParsedSession values via the agent.SessionParser interface.
package claudecode

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"contrails/agent"
)

// Parser reads Claude Code JSONL session transcript files.
// Style: Verify Interface Compliance (go-style-guide.md)
var _ agent.SessionParser = (*Parser)(nil)

// Parser implements agent.SessionParser for Claude Code sessions.
type Parser struct{}

// noisyLineTypes lists JSONL line types that carry no user-visible content
// and should be skipped during parsing. Note: "progress" lines are handled
// separately to extract subagent web searches before skipping.
var noisyLineTypes = map[string]bool{
	"queue-operation":       true,
	"file-history-snapshot": true,
}

// ParseFile reads and parses a Claude Code JSONL transcript file.
// It walks lines sequentially, grouping user and assistant turns into
// ParsedMessage values. Tool use blocks within assistant messages are
// mapped to MessagePart entries with summarized tool call descriptions.
func (parser *Parser) ParseFile(filePath string) (*agent.ParsedSession, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	var lines []jsonlLine
	scanner := bufio.NewScanner(file)
	// Increase buffer size for potentially large JSONL lines
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if trimmed == "" {
			continue
		}
		var line jsonlLine
		if err := json.Unmarshal([]byte(trimmed), &line); err != nil {
			// Skip malformed lines gracefully
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning file: %w", err)
	}

	if len(lines) == 0 {
		return &agent.ParsedSession{
			Agent: "Claude Code",
			User:  "User",
		}, nil
	}

	session := &agent.ParsedSession{
		Agent: "Claude Code",
	}

	// Extract session-level metadata from the first non-noise line.
	// Noise lines (e.g., file-history-snapshot) can appear before the
	// first real line and lack sessionId / timestamp fields.
	for _, line := range lines {
		if noisyLineTypes[line.Type] || line.Type == "progress" {
			continue
		}
		session.SessionID = line.SessionID
		if line.Timestamp != "" {
			session.CreatedAt = agent.FormatISO8601Timestamp(line.Timestamp)
			session.CreatedAtMs = parseISO8601ToMilliseconds(line.Timestamp)
		}
		break
	}

	// Find the last timestamp for LastMessageAt
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i].Timestamp != "" {
			session.LastMessageAt = agent.FormatISO8601Timestamp(lines[i].Timestamp)
			break
		}
	}

	// Walk lines and build messages, consolidating all consecutive assistant
	// turns into a single ParsedMessage until the next real user message.
	var lastAssistantMsg *agent.ParsedMessage

	flushAssistant := func() {
		if lastAssistantMsg != nil {
			session.Messages = append(session.Messages, *lastAssistantMsg)
			lastAssistantMsg = nil
		}
	}

	// Collect agent_progress lines grouped by parentToolUseID for subagent parsing.
	subagentProgress := groupSubagentProgress(lines)

	for _, line := range lines {
		if line.Type == "progress" {
			continue
		}

		// Skip other noisy line types
		if noisyLineTypes[line.Type] {
			continue
		}

		// Skip system lines (hook summaries, turn duration)
		if line.Type == "system" {
			continue
		}

		// Skip meta lines (system prompts, etc.)
		if line.IsMeta {
			continue
		}

		if line.Message == nil {
			continue
		}

		switch line.Message.Role {
		case "user":
			// Check if this is a tool_result line (not a real user message)
			if isToolResultContent(line.Message.Content) {
				if lastAssistantMsg != nil {
					// Skip result content for Read and Edit — the tool call
					// line already conveys all the useful information.
					if name := lastToolCallName(lastAssistantMsg.Parts); name == "Read" || name == "Edit" {
						continue
					}
					toolResultPart := buildToolResultPart(line)
					if toolResultPart != nil {
						lastAssistantMsg.Parts = append(lastAssistantMsg.Parts, *toolResultPart)
					}
				}
				continue
			}

			userMessage := buildUserMessage(line)
			if userMessage != nil {
				flushAssistant()
				// Use the first user message as the session title
				if session.Title == "" {
					session.Title = deriveTitle(userMessage.Content)
				}
				session.Messages = append(session.Messages, *userMessage)
			}

		case "assistant":
			if lastAssistantMsg != nil {
				// Merge into the current assistant turn
				mergeAssistantParts(lastAssistantMsg, line, subagentProgress)
			} else {
				// Start a new assistant turn
				assistantMessage := buildAssistantMessage(line, subagentProgress)
				if assistantMessage != nil {
					if session.Model == "" && assistantMessage.Model != "" {
						session.Model = assistantMessage.Model
					}
					lastAssistantMsg = assistantMessage
				}
			}

		case "tool":
			// Old-format tool results (message.role == "tool"), skip
			continue
		}
	}

	flushAssistant()

	// Set user to "User" (Claude Code doesn't track usernames)
	session.User = "User"

	return session, nil
}

// isLocalCommandContent reports whether the content is a Claude Code local
// command or command output, which should be excluded from the session output.
// These appear as plain strings beginning with <command-name> or
// <local-command-stdout> tags.
func isLocalCommandContent(content interface{}) bool {
	text, ok := content.(string)
	if !ok {
		return false
	}
	return strings.HasPrefix(text, "<command-name>") ||
		strings.HasPrefix(text, "<local-command-stdout>") ||
		strings.HasPrefix(text, "<local-command-caveat>")
}

// buildUserMessage extracts text content and images from a user-role JSONL line.
func buildUserMessage(line jsonlLine) *agent.ParsedMessage {
	if isLocalCommandContent(line.Message.Content) {
		return nil
	}

	content := extractTextContent(line.Message.Content)
	images := extractImageBlocks(line.Message.Content)

	if content == "" && len(images) == 0 {
		return nil
	}

	timestamp := "unknown"
	if line.Timestamp != "" {
		timestamp = agent.FormatISO8601Timestamp(line.Timestamp)
	}

	msg := &agent.ParsedMessage{
		Timestamp: timestamp,
		Role:      "user",
		Content:   content,
	}
	if len(images) > 0 {
		msg.Images = images
	}
	return msg
}

// extractImageBlocks finds all image content blocks and returns them as ImageBlock values.
func extractImageBlocks(content interface{}) []agent.ImageBlock {
	blocks := parseContentBlocks(content)
	var images []agent.ImageBlock
	for _, block := range blocks {
		if block.Type == "image" && block.Source != nil && block.Source.Data != "" {
			images = append(images, agent.ImageBlock{
				MediaType: block.Source.MediaType,
				Data:      block.Source.Data,
			})
		}
	}
	return images
}

// buildAssistantMessage extracts text and tool_use blocks from an assistant-role JSONL line.
func buildAssistantMessage(line jsonlLine, subagentProgress map[string][]jsonlLine) *agent.ParsedMessage {
	timestamp := "unknown"
	if line.Timestamp != "" {
		timestamp = agent.FormatISO8601Timestamp(line.Timestamp)
	}

	message := &agent.ParsedMessage{
		Timestamp: timestamp,
		Role:      "assistant",
		Model:     line.Message.Model,
	}

	// Parse the content blocks
	blocks := parseContentBlocks(line.Message.Content)

	var contentParts []string
	filesEditedSeen := make(map[string]bool)

	for _, block := range blocks {
		switch block.Type {
		case "text":
			if strings.TrimSpace(block.Text) != "" {
				message.Parts = append(message.Parts, agent.MessagePart{
					Type:    agent.PartText,
					Content: block.Text,
				})
				contentParts = append(contentParts, block.Text)
			}

		case "thinking":
			if strings.TrimSpace(block.Thinking) != "" {
				message.Parts = append(message.Parts, agent.MessagePart{
					Type:    agent.PartThinking,
					Content: block.Thinking,
				})
			}

		case "tool_use":
			toolCallPart := buildToolCallPart(block)

			// Attach subagent conversation parts for Agent tool calls
			if block.Name == "Agent" && block.ID != "" {
				if progressLines, ok := subagentProgress[block.ID]; ok {
					toolCallPart.SubParts = buildSubagentParts(progressLines)
				}
			}

			message.Parts = append(message.Parts, toolCallPart)

			// Track file edits from tool calls
			filePath := extractFilePathFromToolInput(block.Name, block.Input)
			if filePath != "" && isFileEditTool(block.Name) && !filesEditedSeen[filePath] {
				filesEditedSeen[filePath] = true
				message.FilesEdited = append(message.FilesEdited, filePath)
			}
		}
	}

	// Build flat content from text parts
	message.Content = strings.Join(contentParts, "\n\n")

	// Only return the message if it has some content
	if len(message.Parts) == 0 {
		return nil
	}

	return message
}

// buildToolCallPart creates a MessagePart for a tool_use content block,
// mapping Claude Code tool names to human-readable summaries.
func buildToolCallPart(block contentBlock) agent.MessagePart {
	part := agent.MessagePart{
		Type: agent.PartToolCall,
		Tool: block.Name,
	}

	part.ToolArgs = summarizeToolCall(block.Name, block.Input)
	return part
}

// groupSubagentProgress collects agent_progress lines from the JSONL,
// grouped by parentToolUseID. Each group represents one subagent's full
// conversation (prompt, tool calls, tool results, text).
func groupSubagentProgress(lines []jsonlLine) map[string][]jsonlLine {
	groups := make(map[string][]jsonlLine)
	for _, line := range lines {
		if line.Type != "progress" {
			continue
		}
		if line.Data == nil || line.Data.Type != "agent_progress" {
			continue
		}
		if line.ParentToolUseID == "" {
			continue
		}
		groups[line.ParentToolUseID] = append(groups[line.ParentToolUseID], line)
	}
	return groups
}

// buildSubagentParts converts a sequence of agent_progress lines into
// MessagePart entries, applying the same filtering as the main agent:
// tool calls are summarized, Read/Edit results are skipped, other results
// are included. The subagent prompt is included as the first text part.
func buildSubagentParts(progressLines []jsonlLine) []agent.MessagePart {
	var parts []agent.MessagePart
	var lastToolName string

	for _, line := range progressLines {
		if line.Data == nil || line.Data.Message == nil || line.Data.Message.Message == nil {
			continue
		}
		msg := line.Data.Message
		inner := msg.Message

		blocks := parseContentBlocks(inner.Content)
		for _, block := range blocks {
			switch block.Type {
			case "text":
				text := strings.TrimSpace(block.Text)
				if text == "" {
					continue
				}
				// The first "user" text is the subagent prompt
				if msg.Type == "user" && len(parts) == 0 {
					// Check if data.prompt is available (more reliable)
					prompt := line.Data.Prompt
					if prompt == "" {
						prompt = text
					}
					parts = append(parts, agent.MessagePart{
						Type:    agent.PartText,
						Content: prompt,
					})
					continue
				}
				// Assistant text (reasoning between tool calls)
				if msg.Type == "assistant" {
					parts = append(parts, agent.MessagePart{
						Type:    agent.PartText,
						Content: text,
					})
				}

			case "thinking":
				// Skip thinking blocks from subagents to keep output concise

			case "tool_use":
				lastToolName = block.Name
				parts = append(parts, buildToolCallPart(block))

			case "tool_result":
				// Same filtering as main agent: skip Read/Edit results
				if lastToolName == "Read" || lastToolName == "Edit" {
					continue
				}
				resultContent := extractToolResultContent(block)
				if resultContent == "" {
					continue
				}
				resultContent = stripSystemReminders(resultContent)
				resultContent = stripAgentMetadata(resultContent)

				// Cap at 20 lines
				resultLines := strings.Split(resultContent, "\n")
				if len(resultLines) > 20 {
					resultLines = resultLines[:20]
					resultLines = append(resultLines, "...")
				}
				resultContent = strings.Join(resultLines, "\n")

				parts = append(parts, agent.MessagePart{
					Type:    agent.PartToolResult,
					Content: resultContent,
				})
			}
		}
	}
	return parts
}

// extractToolResultContent extracts the text content from a tool_result block.
func extractToolResultContent(block contentBlock) string {
	if content, ok := block.Content.(string); ok {
		return content
	}
	return extractTextContent(block.Content)
}

// summarizeToolCall produces a human-readable summary for a Claude Code tool call.
func summarizeToolCall(toolName string, input map[string]interface{}) string {
	switch toolName {
	case "Read":
		if filePath, ok := input["file_path"].(string); ok {
			return fmt.Sprintf("Read `%s`", filePath)
		}
	case "Edit":
		if filePath, ok := input["file_path"].(string); ok {
			return fmt.Sprintf("Edited `%s`", filePath)
		}
	case "Write":
		if filePath, ok := input["file_path"].(string); ok {
			return fmt.Sprintf("Created `%s`", filePath)
		}
	case "MultiEdit":
		if filePath, ok := input["file_path"].(string); ok {
			return fmt.Sprintf("Multi-edited `%s`", filePath)
		}
	case "Bash", "bash":
		if command, ok := input["command"].(string); ok {
			return fmt.Sprintf("Ran `%s`", command)
		}
	case "Glob", "glob":
		if pattern, ok := input["pattern"].(string); ok {
			return fmt.Sprintf("Glob `%s`", pattern)
		}
	case "Grep", "grep":
		if pattern, ok := input["pattern"].(string); ok {
			return fmt.Sprintf("Grep `%s`", pattern)
		}
	case "WebSearch":
		if query, ok := input["query"].(string); ok {
			return fmt.Sprintf("Search `%s`", query)
		}
	case "WebFetch":
		if url, ok := input["url"].(string); ok {
			return fmt.Sprintf("Fetch `%s`", url)
		}
	case "Agent":
		if description, ok := input["description"].(string); ok {
			return fmt.Sprintf("Agent: %s", description)
		}
		return "Agent"
	case "Task":
		if description, ok := input["description"].(string); ok {
			return fmt.Sprintf("Task: %s", description)
		}
	case "TodoRead":
		return "Read todo list"
	case "TodoWrite":
		return "Updated todo list"
	case "TaskCreate":
		if subject, ok := input["subject"].(string); ok {
			return fmt.Sprintf("Created task: %s", subject)
		}
		return "Created task"
	case "TaskUpdate":
		var parts []string
		if taskId, ok := input["taskId"].(string); ok {
			parts = append(parts, fmt.Sprintf("task #%s", taskId))
		}
		if status, ok := input["status"].(string); ok {
			parts = append(parts, status)
		}
		if len(parts) > 0 {
			return fmt.Sprintf("Updated %s", strings.Join(parts, " → "))
		}
		return "Updated task"
	}

	// Fallback: serialize input keys
	if len(input) > 0 {
		var keys []string
		for key := range input {
			keys = append(keys, key)
		}
		return strings.Join(keys, ", ")
	}
	return ""
}

// extractFilePathFromToolInput returns the file path argument from a tool's input,
// if the tool operates on files.
func extractFilePathFromToolInput(toolName string, input map[string]interface{}) string {
	switch toolName {
	case "Read", "Edit", "Write", "MultiEdit":
		if filePath, ok := input["file_path"].(string); ok {
			return filePath
		}
	}
	return ""
}

// isFileEditTool returns true for tools that modify files.
func isFileEditTool(toolName string) bool {
	switch toolName {
	case "Edit", "Write", "MultiEdit":
		return true
	}
	return false
}

// extractTextContent extracts plain text from a message's content field,
// which may be a string or an array of content blocks.
func extractTextContent(content interface{}) string {
	if content == nil {
		return ""
	}

	// Simple string content
	if text, ok := content.(string); ok {
		return text
	}

	// Array of content blocks
	blocks := parseContentBlocks(content)
	var textParts []string
	for _, block := range blocks {
		if block.Type == "text" && block.Text != "" {
			textParts = append(textParts, block.Text)
		}
	}
	return strings.Join(textParts, "\n")
}

// parseContentBlocks converts a content field (string or array) into
// a slice of typed contentBlock values.
func parseContentBlocks(content interface{}) []contentBlock {
	if content == nil {
		return nil
	}

	// If content is a string, wrap it in a single text block
	if text, ok := content.(string); ok {
		return []contentBlock{{Type: "text", Text: text}}
	}

	// If content is an array, unmarshal each element
	array, ok := content.([]interface{})
	if !ok {
		return nil
	}

	blocks := make([]contentBlock, 0, len(array))
	for _, element := range array {
		// Re-marshal and unmarshal to get a properly typed contentBlock.
		// This handles the dynamic typing of JSON interfaces cleanly.
		data, err := json.Marshal(element)
		if err != nil {
			continue
		}
		var block contentBlock
		if err := json.Unmarshal(data, &block); err != nil {
			continue
		}
		blocks = append(blocks, block)
	}
	return blocks
}

// ExtractFirstMessageDate reads the first line with a timestamp and returns it as unix ms.
func ExtractFirstMessageDate(filePath string) (int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var record struct {
			Timestamp string `json:"timestamp"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}
		if record.Timestamp != "" {
			return parseISO8601ToMilliseconds(record.Timestamp), nil
		}
	}
	return 0, nil
}

// ExtractLastMessageDate reads sequentially and tracks the most recent
// timestamp, skipping meta and local-command lines that the parser ignores.
func ExtractLastMessageDate(filePath string) (int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var maxTime int64
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var record struct {
			Timestamp string          `json:"timestamp"`
			IsMeta    bool            `json:"isMeta,omitempty"`
			Message   *json.RawMessage `json:"message,omitempty"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}
		// Skip lines that the parser filters out — meta lines and
		// local command messages (e.g. /exit, command stdout).
		if record.IsMeta {
			continue
		}
		if record.Message != nil {
			var msg struct {
				Content interface{} `json:"content"`
			}
			if json.Unmarshal(*record.Message, &msg) == nil && isLocalCommandContent(msg.Content) {
				continue
			}
		}
		if record.Timestamp != "" {
			t := parseISO8601ToMilliseconds(record.Timestamp)
			if t > maxTime {
				maxTime = t
			}
		}
	}
	if maxTime == 0 {
		return 0, fmt.Errorf("no timestamp found")
	}
	return maxTime, nil
}

// HasContent performs a lightweight scan of a Claude Code JSONL transcript to
// check whether it contains at least one real user message (not a local-command
// or tool result). This is used to filter empty sessions from the chat list.
func HasContent(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var record jsonlLine
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}
		if record.Message == nil || record.Message.Role != "user" {
			continue
		}
		if isToolResultContent(record.Message.Content) {
			continue
		}
		if isLocalCommandContent(record.Message.Content) {
			continue
		}
		// Found a real user message
		return true
	}
	return false
}

// ExtractTitle reads a Claude Code JSONL transcript file and returns a title
// derived from the first user message, matching the logic used during full parsing.
// Returns empty string if the file cannot be read or has no user messages.
func ExtractTitle(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var record struct {
			Type    string `json:"type"`
			Message struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}
		if record.Type != "assistant" && record.Type != "user" {
			continue
		}
		if record.Message.Role != "user" {
			continue
		}
		// Extract text from content (may be string or array)
		var text string
		switch v := record.Message.Content.(type) {
		case string:
			text = v
		case []interface{}:
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					if m["type"] == "text" {
						if t, ok := m["text"].(string); ok {
							text = t
							break
						}
					}
				}
			}
		}
		if text != "" && !isLocalCommandContent(text) {
			return deriveTitle(text)
		}
	}
	return ""
}

// deriveTitle generates a session title from the first user message.
// Claude Code does not have customTitle — we truncate the first message.
func deriveTitle(firstMessage string) string {
	const maxTitleLength = 100
	// Take the first line only
	firstLine := firstMessage
	if newlineIndex := strings.IndexByte(firstMessage, '\n'); newlineIndex >= 0 {
		firstLine = firstMessage[:newlineIndex]
	}
	firstLine = strings.TrimSpace(firstLine)
	if len(firstLine) > maxTitleLength {
		firstLine = firstLine[:maxTitleLength]
	}
	return firstLine
}

// parseISO8601ToMilliseconds converts an ISO 8601 timestamp to Unix milliseconds.
func parseISO8601ToMilliseconds(isoTimestamp string) int64 {
	if isoTimestamp == "" {
		return 0
	}
	parsed, err := parseISO8601(isoTimestamp)
	if err != nil {
		return 0
	}
	return parsed
}

// parseISO8601 attempts to parse an ISO 8601 timestamp string and returns
// the Unix time in milliseconds.
func parseISO8601(isoTimestamp string) (int64, error) {
	formats := []string{
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05.999999999Z",
		"2006-01-02T15:04:05Z",
	}
	for _, format := range formats {
		if t, err := parseTimeFormat(isoTimestamp, format); err == nil {
			return t, nil
		}
	}
	return 0, fmt.Errorf("cannot parse timestamp: %s", isoTimestamp)
}

// parseTimeFormat is a helper that parses a timestamp with the given layout
// and returns Unix milliseconds.
func parseTimeFormat(value, layout string) (int64, error) {
	t, err := time.Parse(layout, value)
	if err != nil {
		return 0, err
	}
	return t.UnixMilli(), nil
}

// mergeAssistantParts appends content blocks from a new assistant JSONL line
// into an existing ParsedMessage. Used when multiple JSONL lines share the
// same API message ID and should be consolidated into one message.
func mergeAssistantParts(message *agent.ParsedMessage, line jsonlLine, subagentProgress map[string][]jsonlLine) {
	blocks := parseContentBlocks(line.Message.Content)
	for _, block := range blocks {
		switch block.Type {
		case "text":
			if strings.TrimSpace(block.Text) != "" {
				message.Parts = append(message.Parts, agent.MessagePart{
					Type:    agent.PartText,
					Content: block.Text,
				})
				if message.Content != "" {
					message.Content += "\n\n"
				}
				message.Content += block.Text
			}

		case "thinking":
			if strings.TrimSpace(block.Thinking) != "" {
				message.Parts = append(message.Parts, agent.MessagePart{
					Type:    agent.PartThinking,
					Content: block.Thinking,
				})
			}

		case "tool_use":
			toolCallPart := buildToolCallPart(block)

			if block.Name == "Agent" && block.ID != "" {
				if progressLines, ok := subagentProgress[block.ID]; ok {
					toolCallPart.SubParts = buildSubagentParts(progressLines)
				}
			}

			message.Parts = append(message.Parts, toolCallPart)

			filePath := extractFilePathFromToolInput(block.Name, block.Input)
			if filePath != "" && isFileEditTool(block.Name) {
				alreadySeen := false
				for _, f := range message.FilesEdited {
					if f == filePath {
						alreadySeen = true
						break
					}
				}
				if !alreadySeen {
					message.FilesEdited = append(message.FilesEdited, filePath)
				}
			}
		}
	}
}

// isToolResultContent checks whether a message's content field contains
// tool_result blocks (indicating this is a tool response, not a human message).
func isToolResultContent(content interface{}) bool {
	if content == nil {
		return false
	}
	// Plain string content is never a tool result
	if _, ok := content.(string); ok {
		return false
	}
	blocks := parseContentBlocks(content)
	for _, block := range blocks {
		if block.Type == "tool_result" {
			return true
		}
	}
	return false
}

// lastToolCallName returns the Tool name of the last PartToolCall in parts,
// or "" if there is none.
func lastToolCallName(parts []agent.MessagePart) string {
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i].Type == agent.PartToolCall {
			return parts[i].Tool
		}
	}
	return ""
}

// buildToolResultPart extracts a compact summary from a tool result line
// and returns it as a PartToolResult MessagePart (or PartText for Agent results
// whose content is markdown that should render natively).
func buildToolResultPart(line jsonlLine) *agent.MessagePart {
	var resultContent string
	isNestedContent := false

	// Extract content from tool_result blocks in the message
	blocks := parseContentBlocks(line.Message.Content)
	for _, block := range blocks {
		if block.Type == "tool_result" {
			if content, ok := block.Content.(string); ok && content != "" {
				resultContent = content
				break
			}
			// Handle nested content arrays (e.g. Agent tool results contain
			// [{type:"text", text:"..."}] instead of a plain string).
			if nested := extractTextContent(block.Content); nested != "" {
				resultContent = nested
				isNestedContent = true
				break
			}
		}
	}

	// Fall back to toolUseResult.Stdout (Bash results)
	if resultContent == "" && line.ToolUseResult != nil && line.ToolUseResult.Stdout != "" {
		resultContent = line.ToolUseResult.Stdout
	}

	if resultContent == "" {
		return nil
	}

	// Strip system reminders injected by Claude Code into tool results
	resultContent = stripSystemReminders(resultContent)

	// Strip Agent tool metadata (agentId line and <usage> block)
	resultContent = stripAgentMetadata(resultContent)

	// Agent subagent results are curated markdown — render as text so
	// headings, tables, and code snippets display properly.
	if isNestedContent {
		return &agent.MessagePart{
			Type:    agent.PartText,
			Content: resultContent,
		}
	}

	// Cap at 20 lines to keep output manageable
	resultLines := strings.Split(resultContent, "\n")
	if len(resultLines) > 20 {
		resultLines = resultLines[:20]
		resultLines = append(resultLines, "...")
	}
	resultContent = strings.Join(resultLines, "\n")

	return &agent.MessagePart{
		Type:    agent.PartToolResult,
		Content: resultContent,
	}
}

// stripAgentMetadata removes the agentId line and <usage> block appended to
// Agent subagent tool results.
func stripAgentMetadata(content string) string {
	// Remove <usage>...</usage> blocks
	for {
		idx := strings.Index(content, "<usage>")
		if idx == -1 {
			break
		}
		endTag := "</usage>"
		endIdx := strings.Index(content[idx:], endTag)
		if endIdx == -1 {
			content = strings.TrimSpace(content[:idx])
		} else {
			content = content[:idx] + content[idx+endIdx+len(endTag):]
		}
	}

	// Remove "agentId: ..." lines
	lines := strings.Split(content, "\n")
	filtered := lines[:0]
	for _, line := range lines {
		if !strings.HasPrefix(line, "agentId:") {
			filtered = append(filtered, line)
		}
	}
	return strings.TrimRight(strings.Join(filtered, "\n"), "\n\r\t ")
}

// stripSystemReminders removes <system-reminder>...</system-reminder> blocks
// that Claude Code injects into tool result content.
func stripSystemReminders(content string) string {
	for {
		idx := strings.Index(content, "<system-reminder>")
		if idx == -1 {
			break
		}
		endTag := "</system-reminder>"
		endIdx := strings.Index(content[idx:], endTag)
		if endIdx == -1 {
			// No closing tag — strip everything from the opening tag onward
			content = strings.TrimSpace(content[:idx])
		} else {
			content = content[:idx] + content[idx+endIdx+len(endTag):]
		}
	}
	return strings.TrimRight(content, "\n\r\t ")
}
