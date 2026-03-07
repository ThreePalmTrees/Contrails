package cursor

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"contrails/agent"

	_ "modernc.org/sqlite" // register "sqlite" driver
)

// Parser reads Cursor composer sessions from the state.vscdb SQLite database.
// Style: Verify Interface Compliance (go-style-guide.md)
var _ agent.SessionParser = (*Parser)(nil)

// Parser implements agent.SessionParser for Cursor composer sessions.
// The filePath argument to ParseFile is a composer UUID, not a filesystem path.
type Parser struct{}

// dbPath returns the absolute path to the Cursor state database.
func dbPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(
		home, "Library", "Application Support",
		"Cursor", "User", "globalStorage", "state.vscdb",
	), nil
}

// openDB opens the Cursor state database read-only.
func openDB() (*sql.DB, error) {
	path, err := dbPath()
	if err != nil {
		return nil, err
	}
	// Open with read-only mode and a 5 s busy timeout so we never block
	// a running Cursor instance.
	dsn := "file:" + path + "?mode=ro&_busy_timeout=5000"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	return db, nil
}

// ParseFile reads and parses a Cursor composer session.
// composerId must be a valid Cursor composer UUID (e.g. "ca592c57-57ed-...").
// The function opens a fresh read-only connection to the database, fetches the
// composerData record plus every bubble referenced in fullConversationHeadersOnly,
// and assembles a ParsedSession.
func (p *Parser) ParseFile(composerId string) (*agent.ParsedSession, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	return parseComposer(db, composerId)
}

// parseComposer is the testable core: it accepts an already-open *sql.DB so
// tests can supply an in-memory database.
func parseComposer(db *sql.DB, composerId string) (*agent.ParsedSession, error) {
	rec, err := fetchComposerRecord(db, composerId)
	if err != nil {
		return nil, err
	}

	session := &agent.ParsedSession{
		SessionID: composerId,
		Title:     rec.Name,
		Agent:     "Cursor",
		User:      "User",
		Model:     rec.ModelConfig.ModelName,
	}

	if rec.CreatedAt > 0 {
		session.CreatedAt = agent.FormatTimestamp(rec.CreatedAt)
		session.CreatedAtMs = rec.CreatedAt
	}
	if rec.LastUpdatedAt > 0 {
		session.LastMessageAt = agent.FormatTimestamp(rec.LastUpdatedAt)
	}

	messages, err := buildMessages(db, composerId, rec.FullConversationHeadersOnly)
	if err != nil {
		return nil, err
	}
	session.Messages = messages

	return session, nil
}

// fetchComposerRecord fetches and decodes the composerData record for composerId.
func fetchComposerRecord(db *sql.DB, composerId string) (composerRecord, error) {
	var raw string
	err := db.QueryRow(
		"SELECT value FROM cursorDiskKV WHERE key = ?",
		"composerData:"+composerId,
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return composerRecord{}, fmt.Errorf("composer %s not found", composerId)
	}
	if err != nil {
		return composerRecord{}, fmt.Errorf("fetching composerData: %w", err)
	}

	var rec composerRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		return composerRecord{}, fmt.Errorf("decoding composerData: %w", err)
	}
	return rec, nil
}

// buildMessages iterates the ordered bubble headers, fetches each bubble, and
// assembles ParsedMessage values grouped by conversation turn.
//
// Grouping rules:
//   - A USER bubble (type 1) starts a new user message.
//   - All subsequent AI bubbles (type 2) until the next USER bubble are
//     combined into one assistant ParsedMessage.
//   - Within the assistant message, plain-text, thinking, and tool-call bubbles
//     are mapped to MessagePart entries in sequence.
func buildMessages(db *sql.DB, composerId string, headers []bubbleHeader) ([]agent.ParsedMessage, error) {
	if len(headers) == 0 {
		return nil, nil
	}

	// Fetch all bubbles in one pass to avoid N+1 queries.
	bubbles, err := fetchBubbles(db, composerId, headers)
	if err != nil {
		return nil, err
	}

	var messages []agent.ParsedMessage
	var currentAssistant *agent.ParsedMessage

	flushAssistant := func() {
		if currentAssistant != nil {
			messages = append(messages, *currentAssistant)
			currentAssistant = nil
		}
	}

	for _, header := range headers {
		bubble, ok := bubbles[header.BubbleId]
		if !ok {
			continue
		}

		switch header.Type {
		case 1: // USER
			flushAssistant()
			if bubble.Text == "" {
				continue // blank user bubble — skip
			}
			messages = append(messages, agent.ParsedMessage{
				Timestamp: formatBubbleTime(bubble.CreatedAt),
				Role:      "user",
				Content:   bubble.Text,
			})

		case 2: // AI
			part, ok := bubbleToPart(bubble)
			if !ok {
				continue
			}

			if currentAssistant == nil {
				ts := formatBubbleTime(bubble.CreatedAt)
				currentAssistant = &agent.ParsedMessage{
					Timestamp: ts,
					Role:      "assistant",
				}
			}
			currentAssistant.Parts = append(currentAssistant.Parts, part)
		}
	}

	flushAssistant()
	return messages, nil
}

// bubbleToPart converts a single AI bubble to a MessagePart.
// Returns (part, true) when the bubble carries meaningful content,
// (zero, false) when it should be silently skipped.
func bubbleToPart(b bubbleRecord) (agent.MessagePart, bool) {
	ct := 0
	if b.CapabilityType != nil {
		ct = *b.CapabilityType
	}

	switch ct {
	case 0: // plain text (capabilityType absent → zero value)
		text := strings.TrimSpace(b.Text)
		if text == "" {
			return agent.MessagePart{}, false
		}
		return agent.MessagePart{
			Type:    agent.PartText,
			Content: text,
		}, true

	case capabilityThinking: // 30
		if b.Thinking == nil || b.Thinking.Text == "" {
			return agent.MessagePart{}, false
		}
		return agent.MessagePart{
			Type:    agent.PartThinking,
			Content: b.Thinking.Text,
		}, true

	case capabilityTool: // 15
		if b.ToolFormerData == nil {
			return agent.MessagePart{}, false
		}
		return toolFormerToPart(b.ToolFormerData), true

	default:
		return agent.MessagePart{}, false
	}
}

// toolFormerToPart converts a toolFormerData bubble into a MessagePart,
// attaching ToolDetail for terminal commands and todo lists.
func toolFormerToPart(tf *toolFormerData) agent.MessagePart {
	// Prefer rawArgs when params is empty; both hold the tool inputs.
	argsJSON := tf.Params
	if argsJSON == "" {
		argsJSON = tf.RawArgs
	}

	part := agent.MessagePart{
		Type:     agent.PartToolCall,
		Tool:     tf.Name,
		ToolArgs: argsJSON,
		Content:  tf.Result,
	}

	// Attach rich ToolDetail for the tool types we know how to render.
	var params toolParams
	if argsJSON != "" {
		// Unmarshal errors are intentionally ignored — we degrade gracefully.
		_ = json.Unmarshal([]byte(argsJSON), &params)
	}

	switch tf.Name {
	case "run_terminal_command_v2":
		if params.Command != "" {
			part.ToolDetail = &agent.ToolDetail{
				Kind:    "terminal",
				Command: params.Command,
			}
			part.ToolArgs = params.Command
		}

	case "todo_write":
		if len(params.Todos) > 0 {
			todos := make([]agent.TodoItem, len(params.Todos))
			for i, t := range params.Todos {
				todos[i] = agent.TodoItem{
					ID:     t.ID,
					Title:  t.Content,
					Status: normalizeTodoStatus(t.Status),
				}
			}
			part.ToolDetail = &agent.ToolDetail{
				Kind:  "todoList",
				Todos: todos,
			}
		}

	case "read_file", "read_file_v2":
		if params.TargetFile != "" {
			clean := params.TargetFile
			if params.Offset > 0 || params.Limit > 0 {
				clean = fmt.Sprintf("%s (offset=%d, limit=%d)", params.TargetFile, params.Offset, params.Limit)
			}
			part.ToolArgs = clean
		}

	case "edit_file_v2":
		if params.TargetFile != "" {
			part.ToolArgs = params.TargetFile
		}

	case "codebase_search":
		if params.Query != "" {
			part.ToolArgs = params.Query
		}

	case "grep", "ripgrep_raw_search":
		if params.Pattern != "" {
			if params.Path != "" {
				part.ToolArgs = fmt.Sprintf("%s in %s", params.Pattern, params.Path)
			} else {
				part.ToolArgs = params.Pattern
			}
		}

	case "list_dir", "list_dir_v2", "glob_file_search":
		if params.Path != "" {
			part.ToolArgs = params.Path
		} else if params.Query != "" {
			part.ToolArgs = params.Query
		}
	}

	return part
}

// normalizeTodoStatus maps Cursor todo statuses to the values expected by
// agent.ToolDetail ("not-started", "in-progress", "completed").
func normalizeTodoStatus(status string) string {
	switch status {
	case "completed":
		return "completed"
	case "in_progress", "in-progress":
		return "in-progress"
	default:
		return "not-started"
	}
}

// fetchBubbles retrieves all bubble records for the given composer in a single
// query and returns them keyed by bubbleId.
func fetchBubbles(db *sql.DB, composerId string, headers []bubbleHeader) (map[string]bubbleRecord, error) {
	if len(headers) == 0 {
		return nil, nil
	}

	// Build a parameterised IN clause for the bubble keys we need.
	keys := make([]string, len(headers))
	args := make([]any, len(headers))
	for i, h := range headers {
		keys[i] = "bubbleId:" + composerId + ":" + h.BubbleId
		args[i] = keys[i]
	}

	placeholders := strings.Repeat("?,", len(args))
	placeholders = placeholders[:len(placeholders)-1] // trim trailing comma

	query := "SELECT key, value FROM cursorDiskKV WHERE key IN (" + placeholders + ")"
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetching bubbles: %w", err)
	}
	defer rows.Close()

	result := make(map[string]bubbleRecord, len(headers))
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			continue
		}

		var bubble bubbleRecord
		if err := json.Unmarshal([]byte(raw), &bubble); err != nil {
			continue
		}
		result[bubble.BubbleId] = bubble
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating bubble rows: %w", err)
	}

	return result, nil
}

// formatBubbleTime parses an ISO 8601 bubble timestamp and returns a
// human-readable string suitable for ParsedMessage.Timestamp.
// Falls back to the raw string if parsing fails.
func formatBubbleTime(iso string) string {
	if iso == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339Nano, iso)
	if err != nil {
		t, err = time.Parse(time.RFC3339, iso)
		if err != nil {
			return iso
		}
	}
	return t.Format("2006-01-02 15:04:05")
}

// ExtractLastMessageDate returns the lastUpdatedAt timestamp (Unix ms) from
// the composerData record identified by composerId.
// It is used for incremental change detection.
func ExtractLastMessageDate(composerId string) (int64, error) {
	db, err := openDB()
	if err != nil {
		return 0, err
	}
	defer db.Close()

	var raw string
	err = db.QueryRow(
		"SELECT value FROM cursorDiskKV WHERE key = ?",
		"composerData:"+composerId,
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("fetching composerData: %w", err)
	}

	var rec struct {
		LastUpdatedAt int64 `json:"lastUpdatedAt"`
	}
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		return 0, fmt.Errorf("decoding composerData: %w", err)
	}
	return rec.LastUpdatedAt, nil
}
