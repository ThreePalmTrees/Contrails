// Package cursor implements the Cursor agent session parser and driver.
// It reads composer sessions from the Cursor SQLite state database and
// produces agent.ParsedSession values via the agent.SessionParser interface.
package cursor

import "encoding/json"

// composerRecord holds the fields we extract from a composerData JSON value.
// Only the fields relevant to session reconstruction are decoded; the full
// JSON blob is much larger and intentionally ignored.
type composerRecord struct {
	ComposerId                  string              `json:"composerId"`
	Name                        string              `json:"name"`
	CreatedAt                   int64               `json:"createdAt"`    // Unix ms
	LastUpdatedAt               int64               `json:"lastUpdatedAt"` // Unix ms
	Status                      string              `json:"status"`
	ModelConfig                 modelConfig         `json:"modelConfig"`
	FullConversationHeadersOnly []bubbleHeader      `json:"fullConversationHeadersOnly"`
	OriginalFileStates          map[string]json.RawMessage `json:"originalFileStates"`
	NewlyCreatedFiles           []createdFileEntry  `json:"newlyCreatedFiles"`
}

type modelConfig struct {
	ModelName string `json:"modelName"`
}

// bubbleHeader is one entry in the fullConversationHeadersOnly ordered list.
// It provides the authoritative turn sequence for the conversation.
type bubbleHeader struct {
	BubbleId string `json:"bubbleId"`
	Type     int    `json:"type"` // 1 = USER, 2 = AI
}

// createdFileEntry holds the URI of a file newly created during the session.
type createdFileEntry struct {
	URI createdFileURI `json:"uri"`
}

type createdFileURI struct {
	External string `json:"external"` // e.g. "file:///Users/..."
	Path     string `json:"path"`     // e.g. "/Users/..."
}

// bubbleRecord holds the fields we decode from a bubbleId JSON value.
type bubbleRecord struct {
	BubbleId           string          `json:"bubbleId"`
	Type               int             `json:"type"`       // 1 = USER, 2 = AI
	CreatedAt          string          `json:"createdAt"`  // ISO 8601
	CapabilityType     *int            `json:"capabilityType"` // nil = plain text, 30 = thinking, 15 = tool
	Text               string          `json:"text,omitempty"`
	Thinking           *thinkingBlock  `json:"thinking,omitempty"`
	ThinkingDurationMs int64           `json:"thinkingDurationMs,omitempty"`
	ToolFormerData     *toolFormerData `json:"toolFormerData,omitempty"`
}

// capabilityType sentinel values observed in Cursor bubbles.
const (
	capabilityThinking = 30 // extended thinking block
	capabilityTool     = 15 // tool call + result
)

type thinkingBlock struct {
	Text      string `json:"text"`
	Signature string `json:"signature"`
}

// toolFormerData holds the tool call and its result for a capability-15 bubble.
// Both params and result are stored as JSON strings by Cursor.
type toolFormerData struct {
	Name    string `json:"name"`    // e.g. "read_file_v2", "edit_file_v2"
	Status  string `json:"status"`  // "completed"
	Params  string `json:"params"`  // JSON string — tool input
	RawArgs string `json:"rawArgs"` // JSON string — same as params
	Result  string `json:"result"`  // JSON string — tool output
}

// toolParams is a minimal union of parameter shapes for the tools we
// extract rich detail from. Unknown fields are ignored.
type toolParams struct {
	// run_terminal_command_v2
	Command string `json:"command"`
	// todo_write
	Todos []todoWriteItem `json:"todos"`
	// read_file / read_file_v2 / edit_file_v2
	TargetFile string `json:"targetFile"`
	Offset     int    `json:"offset"`
	Limit      int    `json:"limit"`
	// codebase_search
	Query string `json:"query"`
	// grep / ripgrep_raw_search
	Pattern string `json:"pattern"`
	// list_dir / list_dir_v2 / glob_file_search (path variant)
	Path string `json:"path"`
}

type todoWriteItem struct {
	ID     string `json:"id"`
	Content string `json:"content"`
	Status string `json:"status"`
}
