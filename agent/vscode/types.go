package vscode

// --- VSCode chat session structures ---
//
// VSCode Copilot uses two formats:
//   - Legacy (v1): Single JSON file with the full session object.
//   - Current (v3): JSONL event log where each line is a JSON object with
//     kind 0 (initial state), kind 1 (scalar patch), or kind 2 (array/complex patch).
//     The final session state is obtained by replaying all events in order.

// chatSession is the unified representation after materializing from either format.
type chatSession struct {
	Version          int            `json:"version"`
	RequesterUsername string        `json:"requesterUsername"`
	ResponderUsername string        `json:"responderUsername"`
	InitialLocation  string        `json:"initialLocation"`
	Requests         []chatRequest  `json:"requests"`
	SessionID        string        `json:"sessionId"`
	CreationDate     int64         `json:"creationDate"`
	LastMessageDate  int64         `json:"lastMessageDate"`
	IsImported       bool          `json:"isImported"`
	CustomTitle      string        `json:"customTitle"`
	// v3 fields (JSONL format)
	InputState       *chatInputState `json:"inputState,omitempty"`
}

// chatInputState holds the user's input state from v3 JSONL sessions.
type chatInputState struct {
	InputText     string             `json:"inputText"`
	SelectedModel *chatSelectedModel `json:"selectedModel,omitempty"`
}

// chatSelectedModel holds the model selection from v3 sessions.
type chatSelectedModel struct {
	Identifier string               `json:"identifier"`
	Metadata   *chatModelMetadata   `json:"metadata,omitempty"`
}

// chatModelMetadata holds model metadata from v3 sessions.
type chatModelMetadata struct {
	Name string       `json:"name"`
	Auth *chatModelAuth `json:"auth,omitempty"`
}

// chatModelAuth holds authentication info from v3 model metadata.
type chatModelAuth struct {
	AccountLabel string `json:"accountLabel"`
}

// chatRequest represents a single user→assistant exchange in the session.
type chatRequest struct {
	RequestID    string            `json:"requestId"`
	Message      chatMessage       `json:"message"`
	Response     []interface{}     `json:"response"`
	ResponseID   string            `json:"responseId"`
	Result       *chatResult       `json:"result,omitempty"`
	IsCanceled   bool              `json:"isCanceled"`
	Agent        *chatAgent        `json:"agent,omitempty"`
	Timestamp    int64             `json:"timestamp"`
	ModelID      string            `json:"modelId"`
	Confirmation string            `json:"confirmation,omitempty"`
	VariableData *chatVariableData `json:"variableData,omitempty"`
	// v3: model state with completion timestamp
	ModelState   *chatModelState   `json:"modelState,omitempty"`
}

// chatModelState tracks the request's processing state (v3 format).
type chatModelState struct {
	Value       int   `json:"value"`
	CompletedAt int64 `json:"completedAt,omitempty"`
}

// chatMessage holds the user's message content.
type chatMessage struct {
	Parts []chatPart `json:"parts"`
	Text  string     `json:"text"`
}

// chatPart is a single fragment of a user message.
type chatPart struct {
	Text string `json:"text"`
	Kind string `json:"kind"`
}

// chatVariableData contains metadata about variables attached to the request.
type chatVariableData struct {
	Variables []chatVariable `json:"variables"`
}

// chatVariable represents a single variable reference (file, symbol, etc.).
type chatVariable struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

// chatResult holds timing and metadata from the assistant's response.
type chatResult struct {
	Timings  *chatTimings  `json:"timings,omitempty"`
	Metadata *chatMetadata `json:"metadata,omitempty"`
	Details  string        `json:"details"`
}

// chatTimings contains response timing measurements.
type chatTimings struct {
	FirstProgress int64 `json:"firstProgress"`
	TotalElapsed  int64 `json:"totalElapsed"`
}

// chatMetadata contains additional metadata about the response.
type chatMetadata struct {
	ToolCallRounds       []toolCallRound `json:"toolCallRounds,omitempty"`
	AgentID              string          `json:"agentId,omitempty"`
	SessionID            string          `json:"sessionId,omitempty"`
	MaxToolCallsExceeded bool            `json:"maxToolCallsExceeded,omitempty"`
}

// toolCallRound describes a single round of tool calls within a response.
type toolCallRound struct {
	Response  string     `json:"response"`
	ToolCalls []toolCall `json:"toolCalls,omitempty"`
}

// toolCall represents an individual tool invocation.
type toolCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	ID        string `json:"id"`
}

// chatAgent identifies the agent used in a request.
type chatAgent struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"fullName"`
}

// --- JSONL event types ---

// jsonlEvent represents a single line in a v3 JSONL session file.
type jsonlEvent struct {
	Kind int             `json:"kind"` // 0=initial state, 1=scalar patch, 2=array/complex patch
	K    []interface{}   `json:"k,omitempty"` // key path for patches (kind 1/2)
	V    interface{}     `json:"v"`           // value (kind 0: full state, kind 1/2: patch value)
	I    *int            `json:"i,omitempty"` // splice index for kind:2 array updates
}
