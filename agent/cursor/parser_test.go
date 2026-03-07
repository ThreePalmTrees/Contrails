package cursor

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// createTestDB opens an in-memory SQLite database and inserts the supplied
// rows into the cursorDiskKV table.
func createTestDB(t *testing.T, rows map[string]any) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE cursorDiskKV (key TEXT PRIMARY KEY, value TEXT)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	for k, v := range rows {
		var raw string
		switch val := v.(type) {
		case string:
			raw = val
		default:
			b, err := json.Marshal(val)
			if err != nil {
				t.Fatalf("marshal value for key %q: %v", k, err)
			}
			raw = string(b)
		}
		if _, err := db.Exec(`INSERT INTO cursorDiskKV (key, value) VALUES (?, ?)`, k, raw); err != nil {
			t.Fatalf("insert key %q: %v", k, err)
		}
	}
	return db
}

// bubbleKey returns the cursorDiskKV key for a bubble.
func bubbleKey(composerID, bubbleID string) string {
	return fmt.Sprintf("bubbleId:%s:%s", composerID, bubbleID)
}

// composerKey returns the cursorDiskKV key for a composer.
func composerKey(composerID string) string {
	return "composerData:" + composerID
}

func TestParseComposer_BasicConversation(t *testing.T) {
	const composerID = "aaaaaaaa-0000-0000-0000-000000000001"
	const userBubbleID = "bbbbbbbb-0000-0000-0000-000000000001"
	const thinkBubbleID = "bbbbbbbb-0000-0000-0000-000000000002"
	const textBubbleID = "bbbbbbbb-0000-0000-0000-000000000003"
	const toolBubbleID = "bbbbbbbb-0000-0000-0000-000000000004"

	capThink := capabilityThinking
	capTool := capabilityTool

	rows := map[string]any{
		composerKey(composerID): composerRecord{
			ComposerId:    composerID,
			Name:          "Build a widget",
			CreatedAt:     1700000000000,
			LastUpdatedAt: 1700000060000,
			ModelConfig:   modelConfig{ModelName: "claude-opus-4"},
			FullConversationHeadersOnly: []bubbleHeader{
				{BubbleId: userBubbleID, Type: 1},
				{BubbleId: thinkBubbleID, Type: 2},
				{BubbleId: textBubbleID, Type: 2},
				{BubbleId: toolBubbleID, Type: 2},
			},
		},
		bubbleKey(composerID, userBubbleID): bubbleRecord{
			BubbleId:  userBubbleID,
			Type:      1,
			CreatedAt: "2023-11-14T22:13:20Z",
			Text:      "Please build me a widget.",
		},
		bubbleKey(composerID, thinkBubbleID): bubbleRecord{
			BubbleId:       thinkBubbleID,
			Type:           2,
			CreatedAt:      "2023-11-14T22:13:21Z",
			CapabilityType: &capThink,
			Thinking: &thinkingBlock{
				Text: "The user wants a widget. I should start by reading the codebase.",
			},
		},
		bubbleKey(composerID, textBubbleID): bubbleRecord{
			BubbleId:  textBubbleID,
			Type:      2,
			CreatedAt: "2023-11-14T22:13:22Z",
			Text:      "I'll start by exploring the project structure.",
		},
		bubbleKey(composerID, toolBubbleID): bubbleRecord{
			BubbleId:       toolBubbleID,
			Type:           2,
			CreatedAt:      "2023-11-14T22:13:23Z",
			CapabilityType: &capTool,
			ToolFormerData: &toolFormerData{
				Name:   "list_dir_v2",
				Status: "completed",
				Params: `{"path":"/home/dev/myproject"}`,
				Result: `{"entries":["src","README.md"]}`,
			},
		},
	}

	db := createTestDB(t, rows)
	defer db.Close()

	session, err := parseComposer(db, composerID)
	if err != nil {
		t.Fatalf("parseComposer: %v", err)
	}

	if session.Title != "Build a widget" {
		t.Errorf("Title = %q, want %q", session.Title, "Build a widget")
	}
	if session.Model != "claude-opus-4" {
		t.Errorf("Model = %q, want %q", session.Model, "claude-opus-4")
	}
	if session.Agent != "Cursor" {
		t.Errorf("Agent = %q, want %q", session.Agent, "Cursor")
	}

	if len(session.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(session.Messages))
	}

	user := session.Messages[0]
	if user.Role != "user" {
		t.Errorf("Messages[0].Role = %q, want \"user\"", user.Role)
	}
	if user.Content != "Please build me a widget." {
		t.Errorf("Messages[0].Content = %q", user.Content)
	}

	asst := session.Messages[1]
	if asst.Role != "assistant" {
		t.Errorf("Messages[1].Role = %q, want \"assistant\"", asst.Role)
	}
	if len(asst.Parts) != 3 {
		t.Fatalf("len(assistant.Parts) = %d, want 3", len(asst.Parts))
	}

	// Part 0: thinking
	if asst.Parts[0].Type != "thinking" {
		t.Errorf("Parts[0].Type = %q, want %q", asst.Parts[0].Type, "thinking")
	}
	if !strings.Contains(asst.Parts[0].Content, "widget") {
		t.Errorf("Parts[0].Content missing expected text: %q", asst.Parts[0].Content)
	}

	// Part 1: plain text
	if asst.Parts[1].Type != "text" {
		t.Errorf("Parts[1].Type = %q, want %q", asst.Parts[1].Type, "text")
	}

	// Part 2: tool call
	if asst.Parts[2].Type != "tool_call" {
		t.Errorf("Parts[2].Type = %q, want %q", asst.Parts[2].Type, "tool_call")
	}
	if asst.Parts[2].Tool != "list_dir_v2" {
		t.Errorf("Parts[2].Tool = %q, want %q", asst.Parts[2].Tool, "list_dir_v2")
	}
}

func TestParseComposer_TerminalToolDetail(t *testing.T) {
	const composerID = "aaaaaaaa-0000-0000-0000-000000000002"
	const userBubbleID = "cccccccc-0000-0000-0000-000000000001"
	const toolBubbleID = "cccccccc-0000-0000-0000-000000000002"

	capTool := capabilityTool

	rows := map[string]any{
		composerKey(composerID): composerRecord{
			ComposerId:    composerID,
			Name:          "Run tests",
			CreatedAt:     1700001000000,
			LastUpdatedAt: 1700001030000,
			FullConversationHeadersOnly: []bubbleHeader{
				{BubbleId: userBubbleID, Type: 1},
				{BubbleId: toolBubbleID, Type: 2},
			},
		},
		bubbleKey(composerID, userBubbleID): bubbleRecord{
			BubbleId:  userBubbleID,
			Type:      1,
			CreatedAt: "2023-11-14T22:17:00Z",
			Text:      "Run the test suite.",
		},
		bubbleKey(composerID, toolBubbleID): bubbleRecord{
			BubbleId:       toolBubbleID,
			Type:           2,
			CreatedAt:      "2023-11-14T22:17:01Z",
			CapabilityType: &capTool,
			ToolFormerData: &toolFormerData{
				Name:   "run_terminal_command_v2",
				Status: "completed",
				Params: `{"command":"go test ./..."}`,
				Result: `{"output":"ok  \tcontrails/agent\t0.042s"}`,
			},
		},
	}

	db := createTestDB(t, rows)
	defer db.Close()

	session, err := parseComposer(db, composerID)
	if err != nil {
		t.Fatalf("parseComposer: %v", err)
	}

	if len(session.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(session.Messages))
	}

	asst := session.Messages[1]
	if len(asst.Parts) != 1 {
		t.Fatalf("len(Parts) = %d, want 1", len(asst.Parts))
	}
	part := asst.Parts[0]
	if part.Tool != "run_terminal_command_v2" {
		t.Errorf("Tool = %q", part.Tool)
	}
	if part.ToolDetail == nil {
		t.Fatal("ToolDetail is nil, want terminal detail")
	}
	if part.ToolDetail.Kind != "terminal" {
		t.Errorf("ToolDetail.Kind = %q, want \"terminal\"", part.ToolDetail.Kind)
	}
	if part.ToolDetail.Command != "go test ./..." {
		t.Errorf("ToolDetail.Command = %q, want \"go test ./...\"", part.ToolDetail.Command)
	}
}

func TestParseComposer_EmptyBubbles(t *testing.T) {
	const composerID = "aaaaaaaa-0000-0000-0000-000000000003"

	rows := map[string]any{
		composerKey(composerID): composerRecord{
			ComposerId:                  composerID,
			Name:                        "Empty chat",
			FullConversationHeadersOnly: []bubbleHeader{},
		},
	}

	db := createTestDB(t, rows)
	defer db.Close()

	session, err := parseComposer(db, composerID)
	if err != nil {
		t.Fatalf("parseComposer: %v", err)
	}
	if len(session.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(session.Messages))
	}
}

func TestParseComposer_NotFound(t *testing.T) {
	db := createTestDB(t, nil)
	defer db.Close()

	_, err := parseComposer(db, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for missing composer, got nil")
	}
}

func TestResolveWorkspaceJSON_Folder(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/workspace.json"
	if err := os.WriteFile(path, []byte(`{"folder":"file:///home/dev/myproject"}`), 0644); err != nil {
		t.Fatal(err)
	}
	wsPath, name := resolveWorkspaceJSON(path)
	if wsPath != "/home/dev/myproject" {
		t.Errorf("workspacePath = %q, want /home/dev/myproject", wsPath)
	}
	if name != "myproject" {
		t.Errorf("displayName = %q, want myproject", name)
	}
}

func TestResolveWorkspaceJSON_Workspace(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/workspace.json"
	if err := os.WriteFile(path, []byte(`{"workspace":"file:///home/dev/workspaces/myapp.code-workspace"}`), 0644); err != nil {
		t.Fatal(err)
	}
	wsPath, name := resolveWorkspaceJSON(path)
	if wsPath != "/home/dev/workspaces/myapp.code-workspace" {
		t.Errorf("workspacePath = %q, want /home/dev/workspaces/myapp.code-workspace", wsPath)
	}
	if name != "myapp.code-workspace" {
		t.Errorf("displayName = %q, want myapp.code-workspace", name)
	}
}

func TestReadWorkspaceComposers(t *testing.T) {
	// Build a temporary workspace state.vscdb with an ItemTable.
	dir := t.TempDir()
	dbPath := dir + "/state.vscdb"

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE ItemTable (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	payload := `{"allComposers":[
		{"composerId":"aaaa-0001","name":"Build widget","lastUpdatedAt":1700002000000},
		{"composerId":"aaaa-0002","name":"Fix tests","lastUpdatedAt":1700001000000}
	]}`
	if _, err := db.Exec(`INSERT INTO ItemTable (key, value) VALUES (?, ?)`, "composer.composerData", payload); err != nil {
		t.Fatalf("insert: %v", err)
	}
	db.Close()

	composers, err := readWorkspaceComposers(dbPath)
	if err != nil {
		t.Fatalf("readWorkspaceComposers: %v", err)
	}
	if len(composers) != 2 {
		t.Fatalf("len(composers) = %d, want 2", len(composers))
	}
	if composers[0].ComposerID != "aaaa-0001" {
		t.Errorf("composers[0].ComposerID = %q, want aaaa-0001", composers[0].ComposerID)
	}
	if composers[1].Name != "Fix tests" {
		t.Errorf("composers[1].Name = %q, want Fix tests", composers[1].Name)
	}
}

func TestCommonAncestor(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		want  string
	}{
		{
			name:  "single path returns parent",
			paths: []string{"/home/dev/project/src/main.go"},
			want:  "/home/dev/project/src",
		},
		{
			name: "siblings in same directory",
			paths: []string{
				"/home/dev/project/src/a.go",
				"/home/dev/project/src/b.go",
			},
			want: "/home/dev/project/src",
		},
		{
			name: "files across subdirectories",
			paths: []string{
				"/home/dev/project/src/a.go",
				"/home/dev/project/tests/a_test.go",
			},
			want: "/home/dev/project",
		},
		{
			name:  "empty slice",
			paths: nil,
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := commonAncestor(tc.paths)
			if got != tc.want {
				t.Errorf("commonAncestor(%v) = %q, want %q", tc.paths, got, tc.want)
			}
		})
	}
}
