// Package opencode implements the OpenCode agent session parser.
// It reads sessions from OpenCode's global SQLite database (the session
// table and session_message) and produces agent.ParsedSession values via the
// agent.SessionParser interface.
package opencode

import "encoding/json"

// sessionRow mirrors a row in the session table: session_v2 on released
// 2.0.x builds, session in the OpenCode source (see sessionTable).
type sessionRow struct {
	ID           string
	ParentID     *string
	Directory    string
	Title        *string
	Agent        *string
	Model        json.RawMessage
	Cost         float64
	TokensInput  int64
	TokensOutput int64
	TimeCreated  int64
	TimeUpdated  int64
}

// messageRow mirrors a row in the session_message table. Ordering is by Seq.
type messageRow struct {
	ID          string
	SessionID   string
	Type        string // "user" | "assistant" | skipped types
	Seq         int64
	TimeUpdated int64
	Data        json.RawMessage
}

// userData is the decoded JSON payload of a message row with type "user".
// The role lives in the row's type column, never in data.
type userData struct {
	Time struct {
		Created int64 `json:"created"`
	} `json:"time"`
	Text  string     `json:"text"`
	Files []fileData `json:"files"`
}

// fileData is one file the user attached to a message. Inline attachments
// (pasted images) carry their bytes in data, which the parser never reads;
// only the name and, when there is one, the path are kept.
type fileData struct {
	Name   string `json:"name"`
	Mime   string `json:"mime"`
	Path   string `json:"path"`
	URL    string `json:"url"`
	Source struct {
		Type string `json:"type"`
		Path string `json:"path"`
	} `json:"source"`
}

// shellData is the decoded JSON payload of a message row with type
// "shell": a command the user ran themselves with !command.
type shellData struct {
	Time struct {
		Created int64 `json:"created"`
	} `json:"time"`
	Command string `json:"command"`
	Status  string `json:"status"` // "exited" once finished; "running" for background commands
	Exit    *int   `json:"exit"`
	// Output is an object {output, truncated} in the live database.
	Output json.RawMessage `json:"output"`
}

// shellOutput is the object form of shellData.Output.
type shellOutput struct {
	Output    string `json:"output"`
	Truncated bool   `json:"truncated"`
}

// assistantData is the decoded JSON payload of a message row with type
// "assistant". content holds the interleaved parts.
type assistantData struct {
	Time struct {
		Created  int64 `json:"created"`
		Streamed int64 `json:"streamed"`
	} `json:"time"`
	Agent   string     `json:"agent"`
	Model   modelRef   `json:"model"`
	Content []partData `json:"content"`
	Finish  string     `json:"finish"`
	Cost    float64    `json:"cost"`
	Tokens  tokenUsage `json:"tokens"`
}

// compactionData is the decoded JSON payload of a message row with type
// "compaction": the point where OpenCode summarised the conversation so far
// to free up the model's context window.
type compactionData struct {
	Time struct {
		Created int64 `json:"created"`
	} `json:"time"`
	Status  string `json:"status"` // "completed" once the summary is written
	Reason  string `json:"reason"` // "auto" | "manual"
	Summary string `json:"summary"`
}

// modelRef is the nested model reference carried by assistant messages and
// the session row: {id, providerID, variant}.
type modelRef struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerID"`
	Variant    string `json:"variant"`
}

// tokenUsage is the token breakdown attached to an assistant message.
type tokenUsage struct {
	Input     int64 `json:"input"`
	Output    int64 `json:"output"`
	Reasoning int64 `json:"reasoning"`
	Cache     struct {
		Read  int64 `json:"read"`
		Write int64 `json:"write"`
	} `json:"cache"`
}

// partData is one entry in an assistant message's content array. It is a
// discriminated union on Type; only the fields relevant to a given type are
// populated.
//
// The tool-name key is "name", not "tool" — every live tool part carries
// name and none carry tool. A struct tagged json:"tool" reads an empty name
// for every call.
type partData struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"` // tool parts only
	Text string `json:"text"` // text and reasoning parts
	// Files is the list of paths touched by a "patch" part.
	Files []string `json:"files"`
	// Time sits at the part level: time.created, time.ran, time.completed.
	Time  partTime   `json:"time"`
	State *toolState `json:"state"`
}

// partTime holds the part-level timestamps. Not every field is present on
// every part type.
type partTime struct {
	Created   int64 `json:"created"`
	Ran       int64 `json:"ran"`
	Completed int64 `json:"completed"`
}

// toolState is the object carried by a "tool" part, discriminated by Status.
// On completed tools it carries exactly {status, input, content, metadata};
// there is no output string, title, or time inside state. Output text lives
// in Content (an array of typed items) and exit information in Metadata.
type toolState struct {
	Status   string          `json:"status"` // pending | running | completed | error
	Input    json.RawMessage `json:"input"`
	Content  []contentItem   `json:"content"`
	Metadata json.RawMessage `json:"metadata"`
	Error    json.RawMessage `json:"error"`
}

// contentItem is one typed item of tool output. Only "text" items carry
// display text.
type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// toolMetadata holds the metadata fields the parser surfaces. The shape is
// open-ended; unknown fields are ignored.
type toolMetadata struct {
	Exit      *int   `json:"exit"`
	Truncated bool   `json:"truncated"`
	SessionID string `json:"sessionID"` // set on subagent tool calls
}

// toolError holds the error object carried by a tool part with status
// "error".
type toolError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// toolInput is a minimal union of the input shapes the parser reads. Unknown
// fields are ignored.
type toolInput struct {
	Command string `json:"command"` // shell / bash / execute
	Path    string `json:"path"`    // read / write / edit / apply_patch
	Pattern string `json:"pattern"` // grep / glob
	Todos   []struct {
		Content string `json:"content"`
		Status  string `json:"status"`
	} `json:"todos"` // todowrite
}
