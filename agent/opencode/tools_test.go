package opencode

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"contrails/agent"
)

// toolPart builds a tool part from JSON pieces, the way it appears in an
// assistant row's content array.
func toolPart(t *testing.T, name, status, input, content, metadata string) partData {
	t.Helper()

	raw := fmt.Sprintf(`{"type":"tool","name":%q,"state":{"status":%q,"input":%s,"content":%s,"metadata":%s}}`,
		name, status, input, content, metadata)
	var part partData
	if err := json.Unmarshal([]byte(raw), &part); err != nil {
		t.Fatalf("decode tool part: %v\n%s", err, raw)
	}
	return part
}

// textContent returns a tool content array holding one text item.
func textContent(text string) string {
	encoded, _ := json.Marshal(text)
	return `[{"type":"text","text":` + string(encoded) + `}]`
}

// Tools are shown by name, with an argument and edits read from the shape
// of their input and metadata. The inputs below are the live shapes of
// OpenCode's own tools, plus tools this code has never seen.
func TestBuildToolParts_ShapeBased(t *testing.T) {
	tests := []struct {
		name     string
		part     partData
		wantArgs string
		wantKind string   // ToolDetail kind, "" for none
		wantEdit []string // file-edit paths, in order
	}{
		{
			name:     "shell command is a terminal call",
			part:     toolPart(t, "shell", "completed", `{"command":"ls -la","workdir":"/repo","timeout":1000}`, `[]`, `{"exit":0}`),
			wantArgs: "ls -la",
			wantKind: "terminal",
		},
		{
			name:     "code runner shows its code on one line",
			part:     toolPart(t, "execute", "completed", `{"code":"const r = await tools.models();\nreturn r"}`, `[]`, `{}`),
			wantArgs: "const r = await tools.models(); return r",
		},
		{
			name:     "search shows its pattern, not its scope",
			part:     toolPart(t, "grep", "completed", `{"path":"/repo","pattern":"TODO","include":"*.go"}`, `[]`, `{"matches":3}`),
			wantArgs: "TODO",
		},
		{
			name:     "read shows the path and is not an edit",
			part:     toolPart(t, "read", "completed", `{"path":"/repo/a.go","offset":10,"limit":50}`, `[]`, `{}`),
			wantArgs: "/repo/a.go",
		},
		{
			name:     "web fetch shows its url",
			part:     toolPart(t, "webfetch", "completed", `{"url":"https://example.com","format":"markdown"}`, `[]`, `{}`),
			wantArgs: "https://example.com",
		},
		{
			name:     "web search shows its query",
			part:     toolPart(t, "websearch", "completed", `{"query":"sqlite wal"}`, `[]`, `{}`),
			wantArgs: "sqlite wal",
		},
		{
			name:     "subagent shows its description, not its prompt",
			part:     toolPart(t, "subagent", "completed", `{"agent":"worker","description":"Check the claim","prompt":"a very long prompt"}`, `[]`, `{}`),
			wantArgs: "Check the claim",
		},
		{
			name:     "skill shows its id",
			part:     toolPart(t, "skill", "completed", `{"id":"haring"}`, `[]`, `{"name":"haring"}`),
			wantArgs: "haring",
		},
		{
			name:     "a tool with no known argument shows only its name",
			part:     toolPart(t, "question", "completed", `{"questions":[{"question":"Relaunch?"}]}`, `[]`, `{}`),
			wantArgs: "",
		},
		{
			name:     "an unknown plugin tool with a known argument",
			part:     toolPart(t, "mcp_github_search", "completed", `{"query":"is:open label:bug"}`, `[]`, `{}`),
			wantArgs: "is:open label:bug",
		},
		{
			name:     "a long argument is shortened",
			part:     toolPart(t, "shell", "completed", `{"description":"`+strings.Repeat("x", 200)+`"}`, `[]`, `{}`),
			wantArgs: strings.Repeat("x", maxSummaryRunes) + "…",
		},
		{
			name:     "todo list",
			part:     toolPart(t, "todowrite", "completed", `{"todos":[{"content":"a","status":"completed"},{"content":"b","status":"in_progress"},{"content":"c","status":"pending"}]}`, `[]`, `{}`),
			wantKind: "todoList",
		},
		{
			name:     "write edits the input path",
			part:     toolPart(t, "write", "completed", `{"path":"/repo/new.go","content":"package x"}`, `[]`, `{}`),
			wantArgs: "/repo/new.go",
			wantEdit: []string{"/repo/new.go"},
		},
		{
			name:     "edit prefers the absolute input path over metadata",
			part:     toolPart(t, "edit", "completed", `{"path":"/repo/bin/tool","oldString":"a","newString":"b"}`, `[]`, `{"files":[{"file":"bin/tool","patch":"@@"}]}`),
			wantArgs: "/repo/bin/tool",
			wantEdit: []string{"/repo/bin/tool"},
		},
		{
			name:     "a multi-file tool lists its files from metadata",
			part:     toolPart(t, "apply_patch", "completed", `{"patchText":"*** Begin Patch"}`, `[]`, `{"files":[{"file":"a.go"},{"path":"b.go"}]}`),
			wantEdit: []string{"a.go", "b.go"},
		},
		{
			name:     "a failed write edits nothing",
			part:     toolPart(t, "write", "error", `{"path":"/repo/new.go","content":"x"}`, `[]`, `{}`),
			wantArgs: "/repo/new.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drafts := buildToolParts(tt.part)
			if len(drafts) == 0 {
				t.Fatal("no parts built")
			}

			call := drafts[0].part
			if call.Type != agent.PartToolCall || call.Tool != tt.part.Name {
				t.Fatalf("first part = %s %q, want the %q tool call", call.Type, call.Tool, tt.part.Name)
			}
			if call.ToolArgs != tt.wantArgs {
				t.Errorf("ToolArgs = %q, want %q", call.ToolArgs, tt.wantArgs)
			}
			kind := ""
			if call.ToolDetail != nil {
				kind = call.ToolDetail.Kind
			}
			if kind != tt.wantKind {
				t.Errorf("ToolDetail kind = %q, want %q", kind, tt.wantKind)
			}

			var edits []string
			for _, draft := range drafts {
				if draft.part.Type == agent.PartFileEdit {
					edits = append(edits, draft.part.FilePath)
				}
			}
			if strings.Join(edits, ",") != strings.Join(tt.wantEdit, ",") {
				t.Errorf("edited files = %v, want %v", edits, tt.wantEdit)
			}
		})
	}
}

func TestBuildToolParts_TodoStatuses(t *testing.T) {
	part := toolPart(t, "todowrite", "completed",
		`{"todos":[{"content":"a","status":"completed"},{"content":"b","status":"in_progress"},{"content":"c","status":"cancelled"}]}`,
		`[]`, `{}`)

	todos := buildToolParts(part)[0].part.ToolDetail.Todos
	var got []string
	for _, todo := range todos {
		got = append(got, todo.ID+":"+todo.Title+":"+todo.Status)
	}
	want := "1:a:completed,2:b:in-progress,3:c:not-started"
	if strings.Join(got, ",") != want {
		t.Errorf("todos = %v, want %s", got, want)
	}
}

// A skill's output is its whole text (tens of KB). It is capped like every
// tool result, with the notes from metadata kept after the cut.
func TestBuildToolParts_OutputCapped(t *testing.T) {
	var skill strings.Builder
	for i := 1; i <= 500; i++ {
		fmt.Fprintf(&skill, "skill line %d\n", i)
	}
	part := toolPart(t, "skill", "completed", `{"id":"haring"}`, textContent(skill.String()), `{"truncated":true}`)

	drafts := buildToolParts(part)
	if len(drafts) != 2 || drafts[1].part.Type != agent.PartToolResult {
		t.Fatalf("parts = %+v, want a call and a result", drafts)
	}
	result := drafts[1].part.Content
	if !strings.HasSuffix(result, "skill line 20\n...\n[output truncated]") {
		t.Errorf("result is not capped at 20 lines with the note kept:\n%s", result)
	}
	if strings.Contains(result, "skill line 21") {
		t.Error("result kept lines past the cap")
	}
}

func TestBuildToolParts_SkipsUnfinishedTools(t *testing.T) {
	for _, status := range []string{"pending", "running"} {
		part := toolPart(t, "shell", status, `{"command":"ls"}`, `[]`, `{}`)
		if drafts := buildToolParts(part); drafts != nil {
			t.Errorf("status %q built %d parts, want none", status, len(drafts))
		}
	}
}
