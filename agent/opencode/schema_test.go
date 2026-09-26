package opencode

import (
	"database/sql"
	"errors"
	"testing"
)

// headSessionDDL is the session table as the OpenCode source (HEAD) creates
// it, without the project foreign key.
const headSessionDDL = `
CREATE TABLE session (
	id text PRIMARY KEY,
	project_id text NOT NULL,
	workspace_id text,
	parent_id text,
	slug text NOT NULL,
	directory text NOT NULL,
	path text,
	title text NOT NULL,
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
	time_compacting integer,
	time_archived integer
);`

// messageDDL returns a session_message table whose session_id references
// table, or has no foreign key when table is empty.
func messageDDL(table string) string {
	fk := ""
	if table != "" {
		fk = ", FOREIGN KEY (session_id) REFERENCES " + table + "(id)"
	}
	return `CREATE TABLE session_message (
		id text PRIMARY KEY,
		session_id text NOT NULL,
		type text NOT NULL,
		seq integer NOT NULL,
		time_created integer NOT NULL,
		time_updated integer NOT NULL,
		data text NOT NULL` + fk + `);`
}

// v2SessionDDL is a minimal session_v2 table; sessionTable only needs it
// to exist.
const v2SessionDDL = `CREATE TABLE session_v2 (id text PRIMARY KEY);`

// openSchemaDB opens an empty in-memory database and runs each statement.
func openSchemaDB(t *testing.T, statements ...string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}
	return db
}

func TestSessionTable(t *testing.T) {
	tests := []struct {
		name       string
		statements []string
		want       string
	}{
		{
			name:       "released 2.0.x schema",
			statements: []string{v2SessionDDL, messageDDL("session_v2")},
			want:       "session_v2",
		},
		{
			name:       "OpenCode source schema",
			statements: []string{headSessionDDL, messageDDL("session")},
			want:       "session",
		},
		{
			name:       "both tables, messages point at session_v2",
			statements: []string{headSessionDDL, v2SessionDDL, messageDDL("session_v2")},
			want:       "session_v2",
		},
		{
			name:       "both tables, messages point at session",
			statements: []string{headSessionDDL, v2SessionDDL, messageDDL("session")},
			want:       "session",
		},
		{
			name:       "both tables, no foreign key prefers session",
			statements: []string{headSessionDDL, v2SessionDDL, messageDDL("")},
			want:       "session",
		},
		{
			name:       "foreign key to an unknown table is ignored",
			statements: []string{v2SessionDDL, `CREATE TABLE other (id text PRIMARY KEY);`, messageDDL("other")},
			want:       "session_v2",
		},
		{
			name:       "no session_message table",
			statements: []string{v2SessionDDL},
			want:       "session_v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openSchemaDB(t, tt.statements...)
			got, err := sessionTable(db)
			if err != nil {
				t.Fatalf("sessionTable: %v", err)
			}
			if got != tt.want {
				t.Errorf("sessionTable = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSessionTable_NeitherTableFails(t *testing.T) {
	db := openSchemaDB(t, messageDDL(""))

	if _, err := sessionTable(db); !errors.Is(err, errNoSessionTable) {
		t.Errorf("sessionTable error = %v, want errNoSessionTable", err)
	}
	if _, err := parseSession(db, "ses_any"); !errors.Is(err, errNoSessionTable) {
		t.Errorf("parseSession error = %v, want errNoSessionTable", err)
	}
	if _, err := browseProjects(db); !errors.Is(err, errNoSessionTable) {
		t.Errorf("browseProjects error = %v, want errNoSessionTable", err)
	}
}

// A database with the OpenCode source's session table must list, parse and
// nest sessions exactly like a session_v2 database.
func TestSessionTable_HeadSchemaEndToEnd(t *testing.T) {
	db := openSchemaDB(t, headSessionDDL, messageDDL("session"))

	const dir = "/tmp/opencode-head-schema"
	insertSession := func(id, parentID string) {
		t.Helper()
		var parent any
		if parentID != "" {
			parent = parentID
		}
		_, err := db.Exec(`INSERT INTO session (id, project_id, parent_id, slug, directory, title,
			version, model, time_created, time_updated)
			VALUES (?, 'proj', ?, 'slug', ?, ?, 'test', '{"id":"head-model"}', 1700000000000, 1700000000000)`,
			id, parent, dir, "title "+id)
		if err != nil {
			t.Fatalf("insert session %s: %v", id, err)
		}
	}
	insertSession("ses_head", "")
	insertSession("ses_head_child", "ses_head")

	insertTestMessage(t, db, "m1", "ses_head", "user", 1,
		`{"time":{"created":1700000000001},"text":"hello head"}`)
	insertTestMessage(t, db, "m2", "ses_head", "assistant", 2,
		`{"time":{"created":1700000000002},"content":[{"type":"tool","name":"subagent",
		"state":{"status":"completed","input":{},"content":[],"metadata":{"sessionID":"ses_head_child"}}}]}`)
	insertTestMessage(t, db, "m3", "ses_head_child", "user", 1,
		`{"time":{"created":1700000000003},"text":"child prompt"}`)

	sessions, err := listSessions(db, dir)
	if err != nil {
		t.Fatalf("listSessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "ses_head" {
		t.Fatalf("listSessions = %+v, want only ses_head", sessions)
	}

	session, err := parseSession(db, "ses_head")
	if err != nil {
		t.Fatalf("parseSession: %v", err)
	}
	if session.Title != "title ses_head" || session.Model != "head-model" {
		t.Errorf("Title/Model = %q/%q", session.Title, session.Model)
	}
	if len(session.Messages) != 2 || session.Messages[0].Content != "hello head" {
		t.Fatalf("Messages = %+v", session.Messages)
	}
	call := session.Messages[1].Parts[0]
	if len(call.SubParts) != 1 || call.SubParts[0].Content != "child prompt" {
		t.Errorf("subagent SubParts = %+v, want the child prompt", call.SubParts)
	}
}
