package opencode

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"contrails/agent"

	_ "modernc.org/sqlite" // register "sqlite" driver
)

// Parser reads OpenCode sessions from its global SQLite database.
// Style: Verify Interface Compliance (go-style-guide.md)
var _ agent.SessionParser = (*Parser)(nil)

// Parser implements agent.SessionParser for OpenCode sessions.
// The sessionID argument to ParseFile is a session row id (e.g.
// "ses_f469f4a93ffeVN4pqdjACuePWC"), not a filesystem path — OpenCode keeps
// every session in one database.
type Parser struct{}

// dbPath is defined in paths.go.

// openDB opens the OpenCode database read-only. Read-only plus a busy
// timeout means a running OpenCode instance is never blocked. WAL content is
// read through the -wal sidecar as-is; this code never checkpoints.
func openDB() (*sql.DB, error) {
	path, err := dbPath()
	if err != nil {
		return nil, err
	}
	dsn := "file:" + path + "?mode=ro&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	return db, nil
}

// ParseFile reads and parses an OpenCode session identified by its
// session id. It opens a fresh read-only connection, resolves the
// session, walks its messages in seq order, and nests subagent sessions
// found by parent_id.
func (p *Parser) ParseFile(sessionID string) (*agent.ParsedSession, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	return parseSession(db, sessionID)
}

// parseSession is the testable core: it accepts an already-open *sql.DB so
// tests can supply an in-memory database.
//
// Every message row is read, including the history before a compaction:
// OpenCode only drops that history from what it sends the model, and a
// contrail is a record of the whole conversation.
func parseSession(db *sql.DB, sessionID string) (*agent.ParsedSession, error) {
	table, err := sessionTable(db)
	if err != nil {
		return nil, err
	}

	row, err := fetchSession(db, table, sessionID)
	if err != nil {
		return nil, err
	}

	session := &agent.ParsedSession{
		SessionID: sessionID,
		Title:     row.title(),
		Agent:     "OpenCode",
		User:      "User",
		Model:     row.modelID(),
	}
	if row.TimeCreated > 0 {
		session.CreatedAt = agent.FormatTimestamp(row.TimeCreated)
		session.CreatedAtMs = row.TimeCreated
	}

	rows, err := fetchMessages(db, sessionID)
	if err != nil {
		return nil, err
	}

	// The session row's time_updated only moves on session-metadata events
	// (title, agent/model switch, archive) — message writes never touch
	// it. The newest fetched message row is the real last activity.
	if last := lastMessageTime(rows); last > 0 {
		session.LastMessageAt = agent.FormatTimestamp(last)
	} else if row.TimeUpdated > 0 {
		session.LastMessageAt = agent.FormatTimestamp(row.TimeUpdated)
	}

	drafts := buildDrafts(rows)
	if err := attachChildren(db, table, sessionID, drafts); err != nil {
		return nil, err
	}
	session.Messages = materialize(drafts)

	return session, nil
}

// draftMessage is one user turn or one assistant turn assembled from the
// message rows. Consecutive assistant rows are merged into a single
// assistant turn.
type draftMessage struct {
	role        string
	timestamp   string
	content     string   // user turns
	attachments []string // user turns
	model       string   // assistant turns
	parts       []partDraft
}

// partDraft is a MessagePart plus the child session it links to, if any.
// The link is resolved into SubParts once the child sessions are parsed.
type partDraft struct {
	part    agent.MessagePart
	childID string
}

// buildDrafts walks the message rows in seq order and groups them into
// turns: a user row starts a user turn, and consecutive assistant rows are
// merged into one assistant turn until the next user row. A shell row (a
// command the user ran with !command) is a user turn too. A compaction row
// becomes a marker part at its position in the assistant turn (starting one
// when it follows a user turn). Every other row type is skipped.
func buildDrafts(rows []messageRow) []draftMessage {
	var drafts []draftMessage
	current := -1

	for _, row := range rows {
		switch row.Type {
		case "user":
			var data userData
			if err := json.Unmarshal(row.Data, &data); err != nil {
				continue
			}
			attachments := attachmentNames(data.Files)
			if strings.TrimSpace(data.Text) == "" && len(attachments) == 0 {
				continue
			}
			current = -1
			drafts = append(drafts, draftMessage{
				role:        "user",
				timestamp:   agent.FormatTimestamp(data.Time.Created),
				content:     data.Text,
				attachments: attachments,
			})

		case "shell":
			var data shellData
			if err := json.Unmarshal(row.Data, &data); err != nil {
				continue
			}
			if strings.TrimSpace(data.Command) == "" {
				continue
			}
			current = -1
			drafts = append(drafts, draftMessage{
				role:      "user",
				timestamp: agent.FormatTimestamp(data.Time.Created),
				content:   shellContent(data),
			})

		case "assistant":
			var data assistantData
			if err := json.Unmarshal(row.Data, &data); err != nil {
				continue
			}
			if current < 0 {
				drafts = append(drafts, draftMessage{
					role:      "assistant",
					timestamp: agent.FormatTimestamp(data.Time.Created),
				})
				current = len(drafts) - 1
			}
			// A turn started by a compaction row has no model yet.
			if drafts[current].model == "" {
				drafts[current].model = data.Model.ID
			}
			drafts[current].parts = append(drafts[current].parts, buildParts(data.Content)...)

		case "compaction":
			var data compactionData
			if err := json.Unmarshal(row.Data, &data); err != nil {
				continue
			}
			if current < 0 {
				drafts = append(drafts, draftMessage{
					role:      "assistant",
					timestamp: agent.FormatTimestamp(data.Time.Created),
				})
				current = len(drafts) - 1
			}
			drafts[current].parts = append(drafts[current].parts, partDraft{part: compactionPart(data)})
		}
	}

	return drafts
}

// attachmentNames describes the files attached to a user message: the file
// name, followed by its path when OpenCode recorded one. The file contents
// are never included.
func attachmentNames(files []fileData) []string {
	var names []string
	for _, file := range files {
		path := file.Path
		if path == "" {
			path = file.Source.Path
		}
		if path == "" && file.URL != "" && !strings.HasPrefix(file.URL, "data:") {
			path = file.URL
		}

		name := file.Name
		switch {
		case name != "" && path != "" && path != name:
			names = append(names, name+" ("+path+")")
		case name != "":
			names = append(names, name)
		case path != "":
			names = append(names, path)
		case file.Mime != "":
			names = append(names, "unnamed "+file.Mime)
		}
	}
	return names
}

// shellContent renders a command the user ran with !command as the text of
// a user turn: the command, then its output (capped like tool output) in a
// collapsed section.
func shellContent(data shellData) string {
	var b strings.Builder

	command := strings.TrimSpace(data.Command)
	fence := agent.CodeFenceFor(command)
	b.WriteString("**Shell command:**\n\n")
	b.WriteString(fence + "sh\n" + command + "\n" + fence)

	output, truncated := decodeShellOutput(data.Output)
	output = capOutput(strings.TrimRight(output, "\n\r\t "))
	if truncated {
		output = appendLine(output, "[output truncated]")
	}

	var notes []string
	if data.Exit != nil {
		notes = append(notes, fmt.Sprintf("exit %d", *data.Exit))
	}
	if data.Status != "" && data.Status != "exited" {
		notes = append(notes, data.Status)
	}
	summary := "Shell output"
	if len(notes) > 0 {
		summary += " (" + strings.Join(notes, ", ") + ")"
	}

	if output == "" {
		if len(notes) > 0 {
			b.WriteString("\n\n*" + summary + "*")
		}
		return b.String()
	}

	fence = agent.CodeFenceFor(output)
	b.WriteString("\n\n<details>\n<summary>" + summary + "</summary>\n\n")
	b.WriteString(fence + "\n" + output + "\n" + fence)
	b.WriteString("\n\n</details>")
	return b.String()
}

// decodeShellOutput reads a shell row's output, which is an object
// {output, truncated}; a plain string is accepted as well.
func decodeShellOutput(raw json.RawMessage) (string, bool) {
	var object shellOutput
	if err := json.Unmarshal(raw, &object); err == nil {
		return object.Output, object.Truncated
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, false
	}
	return "", false
}

// Output caps, matching the Claude Code parser's 20-line limit on tool
// results. The character cap covers output with very long lines (minified
// JSON, a fetched web page). Both keep a contrail readable: a skill's or a
// file's full text belongs in the repository, not in every call to it.
const (
	maxOutputLines = 20
	maxOutputChars = 4000
)

// capOutput shortens output to maxOutputLines lines and maxOutputChars
// characters, ending it with "..." when anything was cut.
func capOutput(output string) string {
	cut := false
	lines := strings.Split(output, "\n")
	if len(lines) > maxOutputLines {
		lines = lines[:maxOutputLines]
		cut = true
	}
	output = strings.Join(lines, "\n")

	if len(output) > maxOutputChars {
		// Dropping invalid bytes removes a character cut in half.
		output = strings.ToValidUTF8(output[:maxOutputChars], "")
		cut = true
	}

	if cut {
		output = strings.TrimRight(output, "\n\r\t ") + "\n..."
	}
	return output
}

// compactionPart builds the marker for a compaction row. The note says how
// it was triggered and, when the summary was not written, how it ended.
func compactionPart(data compactionData) agent.MessagePart {
	var notes []string
	if data.Reason != "" {
		notes = append(notes, data.Reason)
	}
	if data.Status != "" && data.Status != "completed" {
		notes = append(notes, data.Status)
	}
	return agent.MessagePart{
		Type:    agent.PartCompaction,
		Title:   strings.Join(notes, ", "),
		Content: data.Summary,
	}
}

// materialize converts the draft turns into agent.ParsedMessage values,
// dropping assistant turns that produced no renderable parts.
func materialize(drafts []draftMessage) []agent.ParsedMessage {
	var messages []agent.ParsedMessage
	for _, draft := range drafts {
		if draft.role == "user" {
			messages = append(messages, agent.ParsedMessage{
				Timestamp:   draft.timestamp,
				Role:        "user",
				Content:     draft.content,
				Attachments: draft.attachments,
			})
			continue
		}

		if len(draft.parts) == 0 {
			continue
		}
		parts, files := materializeParts(draft.parts)
		messages = append(messages, agent.ParsedMessage{
			Timestamp:   draft.timestamp,
			Role:        "assistant",
			Model:       draft.model,
			Parts:       parts,
			FilesEdited: files,
		})
	}
	return messages
}

// materializeParts flattens the drafted parts and collects the edited file
// paths in first-seen order.
func materializeParts(drafts []partDraft) ([]agent.MessagePart, []string) {
	parts := make([]agent.MessagePart, 0, len(drafts))
	var files []string
	seen := make(map[string]bool, len(drafts))

	for _, draft := range drafts {
		parts = append(parts, draft.part)
		if draft.part.Type != agent.PartFileEdit || draft.part.FilePath == "" {
			continue
		}
		if !seen[draft.part.FilePath] {
			seen[draft.part.FilePath] = true
			files = append(files, draft.part.FilePath)
		}
	}

	return parts, files
}

// attachChildren resolves subagent sessions (session rows whose parent_id
// is this session) and attaches their parts to the matching subagent tool
// call. Nesting is depth-limited to one: grandchildren wait for v2.
func attachChildren(db *sql.DB, table, sessionID string, drafts []draftMessage) error {
	childIDs, err := fetchChildren(db, table, sessionID)
	if err != nil {
		return err
	}

	for _, childID := range childIDs {
		subParts, err := parseSubParts(db, childID)
		if err != nil || len(subParts) == 0 {
			continue // a broken child must not fail the parent session
		}
		for i := range drafts {
			for j := range drafts[i].parts {
				link := &drafts[i].parts[j]
				if link.childID == childID && len(link.part.SubParts) == 0 {
					link.part.SubParts = subParts
				}
			}
		}
	}
	return nil
}

// parseSubParts builds the flat part list for a child session. It walks the
// child's user and assistant rows in seq order; parts that link further
// children are ignored so the nesting stops at depth one.
func parseSubParts(db *sql.DB, sessionID string) ([]agent.MessagePart, error) {
	rows, err := fetchMessages(db, sessionID)
	if err != nil {
		return nil, err
	}

	var parts []agent.MessagePart
	for _, row := range rows {
		switch row.Type {
		case "user":
			var data userData
			if err := json.Unmarshal(row.Data, &data); err != nil {
				continue
			}
			if strings.TrimSpace(data.Text) == "" {
				continue
			}
			parts = append(parts, agent.MessagePart{Type: agent.PartText, Content: data.Text})

		case "assistant":
			var data assistantData
			if err := json.Unmarshal(row.Data, &data); err != nil {
				continue
			}
			for _, draft := range buildParts(data.Content) {
				parts = append(parts, draft.part)
			}
		}
	}
	return parts, nil
}

// buildParts maps an assistant content array to message parts. Only the
// types contrails renders are mapped; unknown part types are skipped.
func buildParts(content []partData) []partDraft {
	var drafts []partDraft
	for _, part := range content {
		switch part.Type {
		case "text":
			if strings.TrimSpace(part.Text) == "" {
				continue
			}
			drafts = append(drafts, partDraft{part: agent.MessagePart{
				Type:    agent.PartText,
				Content: part.Text,
			}})

		case "reasoning":
			if strings.TrimSpace(part.Text) == "" {
				continue
			}
			drafts = append(drafts, partDraft{part: agent.MessagePart{
				Type:    agent.PartThinking,
				Content: part.Text,
			}})

		case "tool":
			drafts = append(drafts, buildToolParts(part)...)
		}
	}
	return drafts
}

// buildToolParts maps a single tool part to a tool-call part, its result,
// and a file-edit part per file it changed. Only completed and errored tools
// are emitted; a tool that is still pending or running has no output worth
// rendering.
//
// Nothing here depends on a tool's name: every tool is shown by its name,
// and what is shown with it is read from the shape of its input and
// metadata. OpenCode can add or rename tools (and plugins and MCP servers
// add their own) without this code knowing about them.
func buildToolParts(part partData) []partDraft {
	state := part.State
	if state == nil || (state.Status != "completed" && state.Status != "error") {
		return nil
	}

	name := part.Name
	if name == "" {
		name = "unknown"
	}

	input := decodeObject(state.Input)
	call := agent.MessagePart{
		Type:       agent.PartToolCall,
		Tool:       name,
		ToolArgs:   toolSummary(input),
		ToolDetail: toolDetail(input),
	}

	drafts := []partDraft{{
		part:    call,
		childID: linkedSessionID(state),
	}}

	if result := toolResult(state); result != "" {
		drafts = append(drafts, partDraft{part: agent.MessagePart{
			Type:    agent.PartToolResult,
			Content: result,
		}})
	}

	if state.Status == "completed" {
		for _, path := range editedFiles(input, state.Metadata) {
			drafts = append(drafts, partDraft{part: agent.MessagePart{
				Type:     agent.PartFileEdit,
				FilePath: path,
				IsEdit:   true,
			}})
		}
	}

	return drafts
}

// summaryKeys are the input fields that best describe a tool call, most
// telling first. They are common argument names, not tool names: a shell
// tool's command, a search's pattern or query, a fetch's url, a file tool's
// path, a subagent's description, a skill's id, a code runner's code.
var summaryKeys = []string{
	"command", "pattern", "query", "url",
	"path", "filePath", "file_path",
	"description", "name", "id", "code",
}

// maxSummaryRunes caps the one-line summary shown next to a tool name.
const maxSummaryRunes = 120

// toolSummary returns the one-line argument shown next to a tool's name:
// the first non-empty summaryKeys field of its input, on one line and
// shortened to maxSummaryRunes. Only that field is shown, so a large input
// (a file's new content, a subagent's full prompt) never is.
func toolSummary(input map[string]json.RawMessage) string {
	for _, key := range summaryKeys {
		value := oneLine(stringField(input, key))
		if value == "" {
			continue
		}
		if runes := []rune(value); len(runes) > maxSummaryRunes {
			value = string(runes[:maxSummaryRunes]) + "…"
		}
		return value
	}
	return ""
}

// toolDetail returns rich ToolDetail when the input has a shape contrails
// renders natively: a shell command, or a todo list. It returns nil for
// everything else.
func toolDetail(input map[string]json.RawMessage) *agent.ToolDetail {
	if command := stringField(input, "command"); strings.TrimSpace(command) != "" {
		return &agent.ToolDetail{Kind: "terminal", Command: command}
	}
	if todos := parseTodos(input["todos"]); len(todos) > 0 {
		return &agent.ToolDetail{Kind: "todoList", Todos: todos}
	}
	return nil
}

// editBodyKeys are the input fields that carry new file content. A tool
// whose input names a path and one of these writes that file; a path alone
// (a read, a search scope) does not.
var editBodyKeys = []string{"content", "newString", "new_string", "patch", "diff", "edits"}

// editedFiles returns the files a completed tool changed. A tool whose input
// has a path and new content changed that path. Otherwise the files are read
// from metadata.files, where OpenCode lists the files an edit touched
// (relative to the project, so the input path is preferred when there is
// one).
func editedFiles(input map[string]json.RawMessage, metadata json.RawMessage) []string {
	path := firstString(input, "path", "filePath", "file_path")
	if path != "" {
		for _, key := range editBodyKeys {
			if _, ok := input[key]; ok {
				return []string{path}
			}
		}
	}

	var meta struct {
		Files []struct {
			File string `json:"file"`
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal(metadata, &meta); err != nil {
		return nil
	}
	var files []string
	for _, file := range meta.Files {
		if file.File != "" {
			files = append(files, file.File)
		} else if file.Path != "" {
			files = append(files, file.Path)
		}
	}
	return files
}

// decodeObject decodes a JSON object into its raw fields, or returns nil
// when raw is not an object.
func decodeObject(raw json.RawMessage) map[string]json.RawMessage {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil
	}
	return object
}

// stringField returns object[key] when it is a JSON string, else "".
func stringField(object map[string]json.RawMessage, key string) string {
	raw, ok := object[key]
	if !ok {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

// firstString returns the first non-empty string field among keys.
func firstString(object map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		if value := stringField(object, key); value != "" {
			return value
		}
	}
	return ""
}

// oneLine collapses all whitespace, including newlines, to single spaces.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// parseTodos converts a todo list input ([{content, status}]) into
// contrails todo items.
func parseTodos(raw json.RawMessage) []agent.TodoItem {
	var input []struct {
		Content string `json:"content"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil
	}
	todos := make([]agent.TodoItem, 0, len(input))
	for i, todo := range input {
		todos = append(todos, agent.TodoItem{
			ID:     strconv.Itoa(i + 1),
			Title:  todo.Content,
			Status: normalizeTodoStatus(todo.Status),
		})
	}
	return todos
}

// normalizeTodoStatus maps OpenCode todo statuses (pending, in_progress,
// completed, cancelled) to the values agent.ToolDetail expects.
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

// toolResult assembles the rendered output of a tool: the joined text items
// (capped, see capOutput), falling back to the error message, plus exit and
// truncation notes drawn from state.metadata.
func toolResult(state *toolState) string {
	result := joinToolContent(state.Content)
	if result == "" {
		result = toolErrorMessage(state.Error)
	}
	result = capOutput(strings.TrimRight(result, "\n\r\t "))

	meta := parseMetadata(state.Metadata)
	if meta.Exit != nil && !strings.Contains(strings.ToLower(result), "exit") {
		result = appendLine(result, fmt.Sprintf("Exit code: %d", *meta.Exit))
	}
	if meta.Truncated {
		result = appendLine(result, "[output truncated]")
	}
	return result
}

// linkedSessionID returns the subagent session a tool part spawned, if any.
// Subagent tool parts carry it in state.metadata.sessionID; when that is
// absent the child id is still embedded in the output text as
// <subagent sessionID="ses_…">.
func linkedSessionID(state *toolState) string {
	if id := parseMetadata(state.Metadata).SessionID; id != "" {
		return id
	}
	const marker = `sessionID="`
	output := joinToolContent(state.Content)
	start := strings.Index(output, marker)
	if start < 0 {
		return ""
	}
	rest := output[start+len(marker):]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// joinToolContent concatenates the text items of a tool's output.
func joinToolContent(items []contentItem) string {
	var b strings.Builder
	for _, item := range items {
		if item.Type == "text" {
			b.WriteString(item.Text)
		}
	}
	return b.String()
}

// toolErrorMessage extracts the message from a tool's error object.
func toolErrorMessage(raw json.RawMessage) string {
	var e toolError
	if err := json.Unmarshal(raw, &e); err != nil {
		return ""
	}
	return e.Message
}

// parseMetadata decodes the open-ended tool metadata object. Unknown fields
// are ignored; a decode failure yields the zero value.
func parseMetadata(raw json.RawMessage) toolMetadata {
	var meta toolMetadata
	// Errors are intentionally ignored — metadata is best effort.
	_ = json.Unmarshal(raw, &meta)
	return meta
}

// appendLine appends a line to a possibly empty result.
func appendLine(result, line string) string {
	if result == "" {
		return line
	}
	return strings.TrimRight(result, "\n") + "\n" + line
}

// fetchSession loads one session row from table (see sessionTable).
func fetchSession(db *sql.DB, table, sessionID string) (sessionRow, error) {
	query := `SELECT id, parent_id, directory, title, agent, model,
		cost, tokens_input, tokens_output, time_created, time_updated
		FROM ` + table + ` WHERE id = ?`

	var (
		row      sessionRow
		parentID sql.NullString
		title    sql.NullString
		agent    sql.NullString
		model    sql.NullString
	)
	err := db.QueryRow(query, sessionID).Scan(
		&row.ID, &parentID, &row.Directory, &title, &agent, &model,
		&row.Cost, &row.TokensInput, &row.TokensOutput,
		&row.TimeCreated, &row.TimeUpdated,
	)
	if err == sql.ErrNoRows {
		return sessionRow{}, fmt.Errorf("session %s not found", sessionID)
	}
	if err != nil {
		return sessionRow{}, fmt.Errorf("fetching session: %w", err)
	}

	if parentID.Valid {
		row.ParentID = &parentID.String
	}
	if title.Valid {
		row.Title = &title.String
	}
	if agent.Valid {
		row.Agent = &agent.String
	}
	if model.Valid {
		row.Model = json.RawMessage(model.String)
	}
	return row, nil
}

// fetchMessages loads all of a session's message rows, ordered by seq.
func fetchMessages(db *sql.DB, sessionID string) ([]messageRow, error) {
	const query = `SELECT id, session_id, type, seq, time_updated, data
		FROM session_message
		WHERE session_id = ?
		ORDER BY seq ASC`

	rows, err := db.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("fetching messages: %w", err)
	}
	defer rows.Close()

	var messages []messageRow
	for rows.Next() {
		var (
			row  messageRow
			data string
		)
		if err := rows.Scan(&row.ID, &row.SessionID, &row.Type, &row.Seq, &row.TimeUpdated, &data); err != nil {
			return nil, fmt.Errorf("scanning message: %w", err)
		}
		row.Data = json.RawMessage(data)
		messages = append(messages, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating messages: %w", err)
	}
	return messages, nil
}

// lastMessageTime returns the newest message-row time_updated in a fetched
// message set, 0 when there are none.
func lastMessageTime(rows []messageRow) int64 {
	var last int64
	for _, row := range rows {
		if row.TimeUpdated > last {
			last = row.TimeUpdated
		}
	}
	return last
}

// fetchChildren returns the ids of the sessions in table whose parent_id is
// sessionID, oldest first.
func fetchChildren(db *sql.DB, table, sessionID string) ([]string, error) {
	rows, err := db.Query(
		"SELECT id FROM "+table+" WHERE parent_id = ? ORDER BY time_created ASC",
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("fetching child sessions: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning child session: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating child sessions: %w", err)
	}
	return ids, nil
}

// title returns the session title, or an empty string when null.
func (r sessionRow) title() string {
	if r.Title == nil {
		return ""
	}
	return *r.Title
}

// modelID returns the model id from the session row's model reference, or an
// empty string when absent.
func (r sessionRow) modelID() string {
	var model modelRef
	if err := json.Unmarshal(r.Model, &model); err != nil {
		return ""
	}
	return model.ID
}
