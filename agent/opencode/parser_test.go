package opencode

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"contrails/agent"

	_ "modernc.org/sqlite"
)

// Fixture session ids exported read-only from the live database. All rows
// (session_v2 and session_message) are copied verbatim via
// sqlite3 "file:$HOME/.local/share/opencode/opencode.db?mode=ro".
const (
	parentSessionID   = "ses_f469f4a93ffeVN4pqdjACuePWC" // contains a subagent tool call
	childSessionID    = "ses_f469f40c9ffeR5vnIvBIZGuRik" // its subagent child
	shellSessionID    = "ses_f4722231dffeP00iZHPx6G25SQ" // a completed shell tool
	errorShellID      = "ses_f462b37b0ffeGHItODWl36QASJ" // an errored shell tool
	fixtureChildTitle = "Write report.txt file"
)

//go:embed testdata/fixture.json
var fixtureJSON []byte

// schemaDDL mirrors the live session_v2 and session_message tables. Only the
// foreign key clauses are omitted, so the fixture can be inserted without a
// project row.
const schemaDDL = `
CREATE TABLE session_v2 (
	id text PRIMARY KEY,
	project_id text NOT NULL,
	workspace_id text,
	parent_id text,
	fork_session_id text,
	fork_boundary text,
	slug text NOT NULL,
	directory text NOT NULL,
	path text,
	title text,
	version text NOT NULL,
	share_url text,
	summary_additions integer,
	summary_deletions integer,
	summary_files integer,
	summary_diffs text,
	metadata text,
	cost real DEFAULT 0 NOT NULL,
	tokens_input integer DEFAULT 0 NOT NULL,
	tokens_output integer DEFAULT 0 NOT NULL,
	tokens_reasoning integer DEFAULT 0 NOT NULL,
	tokens_cache_read integer DEFAULT 0 NOT NULL,
	tokens_cache_write integer DEFAULT 0 NOT NULL,
	revert text,
	permission text,
	agent text,
	model text,
	time_created integer NOT NULL,
	time_updated integer NOT NULL,
	time_idle integer,
	time_viewed integer,
	idle_outcome text,
	time_compacting integer,
	time_archived integer,
	time_suspended integer,
	resume_attempts integer DEFAULT 0 NOT NULL
);
CREATE INDEX session_v2_project_idx ON session_v2 (project_id);
CREATE INDEX session_v2_parent_idx ON session_v2 (parent_id);
CREATE TABLE session_message (
	id text PRIMARY KEY,
	session_id text NOT NULL,
	type text NOT NULL,
	seq integer NOT NULL,
	time_created integer NOT NULL,
	time_updated integer NOT NULL,
	data text NOT NULL
);
CREATE UNIQUE INDEX session_message_session_seq_idx ON session_message (session_id, seq);
`

type fixtureFile struct {
	Sessions []map[string]any `json:"sessions"`
	Messages []map[string]any `json:"messages"`
}

// setupTestDB creates an in-memory database with the real v2 table shape and
// inserts the exported fixture rows.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if _, err := db.Exec(schemaDDL); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	var fixture fixtureFile
	if err := json.Unmarshal(fixtureJSON, &fixture); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	for _, row := range fixture.Sessions {
		insertFixtureRow(t, db, "session_v2", row)
	}
	for _, row := range fixture.Messages {
		insertFixtureRow(t, db, "session_message", row)
	}

	return db
}

// insertFixtureRow inserts one decoded fixture row using the table's own
// column order.
func insertFixtureRow(t *testing.T, db *sql.DB, table string, row map[string]any) {
	t.Helper()

	cols := tableColumns(t, db, table)
	placeholders := make([]string, len(cols))
	values := make([]any, len(cols))
	for i, col := range cols {
		placeholders[i] = "?"
		values[i] = row[col]
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table, strings.Join(cols, ", "), strings.Join(placeholders, ", "),
	)
	if _, err := db.Exec(query, values...); err != nil {
		t.Fatalf("insert into %s: %v", table, err)
	}
}

// tableColumns returns the column names of table in declaration order.
func tableColumns(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()

	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		t.Fatalf("table_info(%s): %v", table, err)
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var (
			cid, notNull, pk int
			name, typ        string
			dflt             sql.NullString
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		cols = append(cols, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table_info: %v", err)
	}
	return cols
}

// insertTestSession inserts a minimal valid session_v2 row.
func insertTestSession(t *testing.T, db *sql.DB, id, parentID, title string, created, updated int64) {
	t.Helper()

	var parent any
	if parentID != "" {
		parent = parentID
	}
	_, err := db.Exec(`
		INSERT INTO session_v2 (
			id, project_id, parent_id, slug, directory, title, version,
			tokens_input, tokens_output, tokens_reasoning,
			tokens_cache_read, tokens_cache_write, cost,
			time_created, time_updated, resume_attempts
		) VALUES (?, 'proj_test', ?, 'test-slug', '/tmp/opencode-test', ?, 'test', 0, 0, 0, 0, 0, 0, ?, ?, 0)`,
		id, parent, title, created, updated,
	)
	if err != nil {
		t.Fatalf("insert test session %s: %v", id, err)
	}
}

// insertTestMessage inserts one session_message row.
func insertTestMessage(t *testing.T, db *sql.DB, id, sessionID, typ string, seq int64, data string) {
	t.Helper()

	_, err := db.Exec(`
		INSERT INTO session_message (id, session_id, type, seq, time_created, time_updated, data)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, sessionID, typ, seq, seq, seq, data,
	)
	if err != nil {
		t.Fatalf("insert test message %s: %v", id, err)
	}
}

// findPart returns the first part satisfying match across an assistant
// message's parts.
func findPart(messages []agent.ParsedMessage, match func(agent.MessagePart) bool) (agent.MessagePart, bool) {
	for _, message := range messages {
		for _, part := range message.Parts {
			if match(part) {
				return part, true
			}
		}
	}
	return agent.MessagePart{}, false
}

func TestParser_ImplementsSessionParser(t *testing.T) {
	var parser agent.SessionParser = &Parser{}
	if parser == nil {
		t.Fatal("Parser does not implement agent.SessionParser")
	}
}

func TestParseSession_Basic(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	session, err := parseSession(db, parentSessionID)
	if err != nil {
		t.Fatalf("parseSession: %v", err)
	}

	if session.SessionID != parentSessionID {
		t.Errorf("SessionID = %q, want %q", session.SessionID, parentSessionID)
	}
	if session.Title != "parent-probe" {
		t.Errorf("Title = %q, want %q", session.Title, "parent-probe")
	}
	if session.Agent != "OpenCode" {
		t.Errorf("Agent = %q, want %q", session.Agent, "OpenCode")
	}
	if session.User != "User" {
		t.Errorf("User = %q, want %q", session.User, "User")
	}
	if session.Model != "glm-5.3-flash" {
		t.Errorf("Model = %q, want %q", session.Model, "glm-5.3-flash")
	}
	if session.CreatedAtMs != 1789816517999 {
		t.Errorf("CreatedAtMs = %d, want 1789816517999", session.CreatedAtMs)
	}
	if session.CreatedAt == "unknown" || session.CreatedAt == "" {
		t.Errorf("CreatedAt = %q, want a formatted timestamp", session.CreatedAt)
	}
	if session.LastMessageAt == "unknown" || session.LastMessageAt == "" {
		t.Errorf("LastMessageAt = %q, want a formatted timestamp", session.LastMessageAt)
	}

	if len(session.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(session.Messages))
	}

	user := session.Messages[0]
	if user.Role != "user" {
		t.Errorf("Messages[0].Role = %q, want \"user\"", user.Role)
	}
	if !strings.Contains(user.Content, "probe-zlane") {
		t.Errorf("Messages[0].Content = %q, want it to mention the subagent", user.Content)
	}

	assistant := session.Messages[1]
	if assistant.Role != "assistant" {
		t.Errorf("Messages[1].Role = %q, want \"assistant\"", assistant.Role)
	}
	if assistant.Model != "glm-5.3-flash" {
		t.Errorf("Messages[1].Model = %q, want %q", assistant.Model, "glm-5.3-flash")
	}
}

func TestParseSession_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	if _, err := parseSession(db, "ses_does_not_exist"); err == nil {
		t.Fatal("expected error for missing session, got nil")
	}
}

func TestParseSession_SkipsNonUserAssistantTypes(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	session, err := parseSession(db, parentSessionID)
	if err != nil {
		t.Fatalf("parseSession: %v", err)
	}

	// The parent has a model-switched row (seq 1) and an idle row (seq 18);
	// only the user and assistant rows survive, and the two assistant rows
	// merge into one turn.
	if len(session.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(session.Messages))
	}
	if session.Messages[0].Role != "user" || session.Messages[1].Role != "assistant" {
		t.Errorf("roles = [%q %q], want [user assistant]",
			session.Messages[0].Role, session.Messages[1].Role)
	}
	if strings.Contains(session.Messages[0].Content, "succeeded") {
		t.Errorf("idle row leaked into the user message: %q", session.Messages[0].Content)
	}
}

func TestParseSession_ToolCall(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	session, err := parseSession(db, shellSessionID)
	if err != nil {
		t.Fatalf("parseSession: %v", err)
	}

	call, ok := findPart(session.Messages, func(p agent.MessagePart) bool {
		return p.Type == agent.PartToolCall && p.Tool == "shell"
	})
	if !ok {
		t.Fatal("no shell tool call part found")
	}
	// The tool name must come from the part's "name" field.
	if call.Tool != "shell" {
		t.Errorf("Tool = %q, want \"shell\"", call.Tool)
	}
	if call.ToolArgs != "echo probe-one-ok" {
		t.Errorf("ToolArgs = %q, want %q", call.ToolArgs, "echo probe-one-ok")
	}
	if call.ToolDetail == nil || call.ToolDetail.Kind != "terminal" {
		t.Fatalf("ToolDetail = %+v, want terminal detail", call.ToolDetail)
	}
	if call.ToolDetail.Command != "echo probe-one-ok" {
		t.Errorf("ToolDetail.Command = %q, want %q", call.ToolDetail.Command, "echo probe-one-ok")
	}

	result, ok := findPart(session.Messages, func(p agent.MessagePart) bool {
		return p.Type == agent.PartToolResult
	})
	if !ok {
		t.Fatal("no tool result part found")
	}
	// Output text is the joined state.content[].text items; the exit note
	// already lives in the content, so metadata must not duplicate it.
	if !strings.Contains(result.Content, "probe-one-ok") {
		t.Errorf("result content = %q, want the command output", result.Content)
	}
	if !strings.Contains(result.Content, "Command exited with code 0.") {
		t.Errorf("result content = %q, want the exit line from state.content", result.Content)
	}
	if strings.Contains(result.Content, "Exit code:") {
		t.Errorf("result content = %q, must not duplicate the exit line", result.Content)
	}
}

func TestParseSession_ErroredToolFallsBackToErrorMessage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	session, err := parseSession(db, errorShellID)
	if err != nil {
		t.Fatalf("parseSession: %v", err)
	}

	call, ok := findPart(session.Messages, func(p agent.MessagePart) bool {
		return p.Type == agent.PartToolCall && p.Tool == "shell"
	})
	if !ok {
		t.Fatal("no shell tool call part found")
	}
	if call.ToolArgs != "sleep 90 && echo DONE-AFTER-SLEEP" {
		t.Errorf("ToolArgs = %q", call.ToolArgs)
	}

	result, ok := findPart(session.Messages, func(p agent.MessagePart) bool {
		return p.Type == agent.PartToolResult
	})
	if !ok {
		t.Fatal("no tool result part found for the errored tool")
	}
	if result.Content != "Tool execution interrupted" {
		t.Errorf("result content = %q, want the error message", result.Content)
	}
}

func TestParseSession_Reasoning(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	session, err := parseSession(db, childSessionID)
	if err != nil {
		t.Fatalf("parseSession: %v", err)
	}

	thinking, ok := findPart(session.Messages, func(p agent.MessagePart) bool {
		return p.Type == agent.PartThinking
	})
	if !ok {
		t.Fatal("no thinking part found")
	}
	if !strings.Contains(thinking.Content, "The task is straightforward") {
		t.Errorf("thinking content = %q, want the reasoning text", thinking.Content)
	}
}

func TestParseSession_FileEdit(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	session, err := parseSession(db, childSessionID)
	if err != nil {
		t.Fatalf("parseSession: %v", err)
	}

	const path = "/private/tmp/oc-q2-16007/report.txt"

	call, ok := findPart(session.Messages, func(p agent.MessagePart) bool {
		return p.Type == agent.PartToolCall && p.Tool == "write"
	})
	if !ok {
		t.Fatal("no write tool call part found")
	}
	if call.ToolArgs != path {
		t.Errorf("ToolArgs = %q, want %q", call.ToolArgs, path)
	}

	edit, ok := findPart(session.Messages, func(p agent.MessagePart) bool {
		return p.Type == agent.PartFileEdit
	})
	if !ok {
		t.Fatal("no file edit part found")
	}
	if edit.FilePath != path {
		t.Errorf("FilePath = %q, want %q", edit.FilePath, path)
	}
	if !edit.IsEdit {
		t.Error("IsEdit = false, want true")
	}

	var filesEdited []string
	for _, message := range session.Messages {
		filesEdited = append(filesEdited, message.FilesEdited...)
	}
	if len(filesEdited) != 1 || filesEdited[0] != path {
		t.Errorf("FilesEdited = %v, want [%s]", filesEdited, path)
	}
}

func TestParseSession_NestedSubagent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// A grandchild must not be pulled into the parent; nesting stops at depth 1.
	insertTestSession(t, db, "ses_grandchild-fixture", childSessionID, "grandchild", 1789816529000, 1789816529001)
	insertTestMessage(t, db, "msg_grandchild", "ses_grandchild-fixture", "user", 1,
		`{"time":{"created":1789816529000},"text":"GRANDCHILD-MARKER"}`)

	session, err := parseSession(db, parentSessionID)
	if err != nil {
		t.Fatalf("parseSession: %v", err)
	}

	call, ok := findPart(session.Messages, func(p agent.MessagePart) bool {
		return p.Type == agent.PartToolCall && p.Tool == "subagent"
	})
	if !ok {
		t.Fatal("no subagent tool call part found")
	}
	if len(call.SubParts) == 0 {
		t.Fatal("subagent tool call has no SubParts")
	}

	// The child's prompt is the first sub-part.
	if call.SubParts[0].Type != agent.PartText {
		t.Errorf("SubParts[0].Type = %q, want text", call.SubParts[0].Type)
	}
	if !strings.Contains(call.SubParts[0].Content, "You are a subagent spawned") {
		t.Errorf("SubParts[0].Content = %q, want the subagent prompt", call.SubParts[0].Content)
	}

	var childTool string
	var childResult string
	var childThinking string
	for _, sub := range call.SubParts {
		if sub.Type == agent.PartToolCall {
			childTool = sub.Tool
			if len(sub.SubParts) > 0 {
				t.Errorf("subagent part %q has nested SubParts, want depth 1", sub.Tool)
			}
		}
		if sub.Type == agent.PartToolResult && strings.Contains(sub.Content, "Created file successfully") {
			childResult = sub.Content
		}
		if sub.Type == agent.PartThinking && strings.Contains(sub.Content, "The task is straightforward") {
			childThinking = sub.Content
		}
	}
	if childTool != "read" && childTool != "write" {
		t.Errorf("last child tool = %q, want read or write", childTool)
	}
	if childResult == "" {
		t.Error("child tool result not found in SubParts")
	}
	if childThinking == "" {
		t.Error("child reasoning not found in SubParts")
	}

	for _, part := range call.SubParts {
		if strings.Contains(part.Content, "GRANDCHILD-MARKER") {
			t.Error("grandchild content leaked into depth-1 SubParts")
		}
	}
}

func TestParseSession_CompactionBoundary(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	const id = "ses_compaction-fixture"
	insertTestSession(t, db, id, "", "compacted", 1700000000000, 1700000000000)

	insertTestMessage(t, db, "msg_c1", id, "user", 1,
		`{"time":{"created":1700000000000},"text":"before the first compaction"}`)
	insertTestMessage(t, db, "msg_c2", id, "assistant", 2,
		`{"time":{"created":1700000000001},"content":[{"type":"text","id":"p1","text":"before-both"}]}`)
	// The compaction row's payload shape is from the live database; only its
	// seq matters to the boundary.
	insertTestMessage(t, db, "msg_c3", id, "compaction", 3,
		`{"time":{"created":1700000000002},"status":"completed","reason":"auto","summary":"first"}`)
	insertTestMessage(t, db, "msg_c4", id, "user", 4,
		`{"time":{"created":1700000000003},"text":"still before the last compaction"}`)
	insertTestMessage(t, db, "msg_c5", id, "assistant", 5,
		`{"time":{"created":1700000000004},"content":[{"type":"text","id":"p2","text":"also-before"}]}`)
	insertTestMessage(t, db, "msg_c6", id, "compaction", 6,
		`{"time":{"created":1700000000005},"status":"completed","reason":"auto","summary":"second"}`)
	insertTestMessage(t, db, "msg_c7", id, "user", 7,
		`{"time":{"created":1700000000006},"text":"after both compactions"}`)
	insertTestMessage(t, db, "msg_c8", id, "assistant", 8,
		`{"time":{"created":1700000000007},"content":[{"type":"text","id":"p3","text":"after-both"}]}`)

	session, err := parseSession(db, id)
	if err != nil {
		t.Fatalf("parseSession: %v", err)
	}

	if len(session.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(session.Messages))
	}
	if session.Messages[0].Content != "after both compactions" {
		t.Errorf("Messages[0].Content = %q, want only post-compaction content", session.Messages[0].Content)
	}
	text, ok := findPart(session.Messages[1:], func(p agent.MessagePart) bool {
		return p.Type == agent.PartText
	})
	if !ok || text.Content != "after-both" {
		t.Errorf("post-compaction assistant text = %+v, want \"after-both\"", text)
	}

	rendered := agent.RenderMarkdown(session)
	for _, gone := range []string{"before-both", "also-before", "still before"} {
		if strings.Contains(rendered, gone) {
			t.Errorf("pre-compaction content %q leaked into the rendered session", gone)
		}
	}
}
