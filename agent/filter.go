package agent

// ContrailFilters controls which kinds of content are retained when a
// parsed session is written to a contrail markdown file. All fields
// default to true for callers that use DefaultContrailFilters.
//
// Note on semantics: sub-agents run as Agent tool calls, so when
// ToolCalls is false, sub-agent content is necessarily excluded as
// well — the dedicated SubagentContent flag only matters while
// ToolCalls is true.
type ContrailFilters struct {
	Thinking        bool
	ToolCalls       bool
	SubagentContent bool
}

// DefaultContrailFilters returns the "include everything" filter set —
// the behavior contrails shipped with before per-section toggles existed.
func DefaultContrailFilters() ContrailFilters {
	return ContrailFilters{Thinking: true, ToolCalls: true, SubagentContent: true}
}

// ApplyContrailFilters strips message parts from the session in-place
// according to the filters. Disabled filters remove the corresponding
// parts entirely:
//   - Thinking=false     → drop PartThinking
//   - ToolCalls=false    → drop PartToolCall/PartFileEdit/PartCodeBlock/PartToolResult,
//     and clear message FilesEdited (otherwise the rendered file-edit
//     summary would claim edits that no longer appear in the output).
//   - SubagentContent=false → keep the Agent tool call line itself,
//     but drop its nested SubParts (the sub-agent's own conversation).
func ApplyContrailFilters(session *ParsedSession, filters ContrailFilters) {
	if session == nil {
		return
	}
	if filters.Thinking && filters.ToolCalls && filters.SubagentContent {
		return
	}
	for i := range session.Messages {
		msg := &session.Messages[i]
		if len(msg.Parts) > 0 {
			filtered := msg.Parts[:0]
			for _, part := range msg.Parts {
				switch part.Type {
				case PartThinking:
					if !filters.Thinking {
						continue
					}
				case PartToolCall:
					if !filters.ToolCalls {
						continue
					}
					if !filters.SubagentContent {
						part.SubParts = nil
					}
				case PartFileEdit, PartCodeBlock, PartToolResult:
					if !filters.ToolCalls {
						continue
					}
				}
				filtered = append(filtered, part)
			}
			msg.Parts = filtered
		}
		if !filters.ToolCalls {
			msg.FilesEdited = nil
		}
	}
}
