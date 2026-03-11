// Package vscode implements the VSCode Copilot chat session parser.
// It reads VSCode's JSONL event log files (v3 format) and produces
// agent.ParsedSession values via the agent.SessionParser interface.
package vscode

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"contrails/agent"
)

// Parser reads VSCode Copilot chat session files (JSONL v3 format).
// Style: Verify Interface Compliance (go-style-guide.md)
var _ agent.SessionParser = (*Parser)(nil)

// Parser implements agent.SessionParser for VS ode Copilot sessions.
type Parser struct{}

// ParseFile reads and parses a VS ode chat session JSONL file.
func (parser *Parser) ParseFile(filePath string) (*agent.ParsedSession, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty file")
	}

	// Try parsing as a fully materialized JSON object first
	var session chatSession
	if err := json.Unmarshal(data, &session); err == nil && session.Version > 0 {
		normalizeSession(&session)
		return buildParsedSession(&session)
	}

	// Fall back to materializing from JSONL events
	materialized, err := materializeJSONL(data)
	if err != nil {
		return nil, err
	}

	normalizeSession(materialized)
	return buildParsedSession(materialized)
}

// normalizeSession infers missing data from v3 representations for uniform processing.
func normalizeSession(session *chatSession) {
	// v3 sessions don't have requesterUsername at the top level —
	// extract it from inputState.selectedModel.metadata.auth.accountLabel
	if session.RequesterUsername == "" && session.InputState != nil {
		if session.InputState.SelectedModel != nil &&
			session.InputState.SelectedModel.Metadata != nil &&
			session.InputState.SelectedModel.Metadata.Auth != nil {
			session.RequesterUsername = session.InputState.SelectedModel.Metadata.Auth.AccountLabel
		}
	}

	// v3 sessions don't have lastMessageDate — compute from the latest request
	if session.LastMessageDate == 0 && len(session.Requests) > 0 {
		for i := range session.Requests {
			req := &session.Requests[i]
			ts := req.Timestamp
			// Check if modelState has a later completedAt
			if req.ModelState != nil && req.ModelState.CompletedAt > ts {
				ts = req.ModelState.CompletedAt
			}
			if ts > session.LastMessageDate {
				session.LastMessageDate = ts
			}
		}
	}
}

// materializeJSONL replays the JSONL event log to reconstruct the final session state.
// Event kinds:
//   - 0: Initial state snapshot (v contains the full session object)
//   - 1: Set scalar value at path k
//   - 2: Set array/complex value at path k
func materializeJSONL(data []byte) (*chatSession, error) {
	var state map[string]interface{}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	// Set a large buffer for potentially long lines (response arrays can be huge)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var event jsonlEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue // skip malformed lines
		}

		switch event.Kind {
		case 0:
			// Initial state — the value is the full session object
			stateMap, ok := event.V.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("kind 0 value is not an object")
			}
			state = stateMap

		case 1:
			// Scalar patch at path K
			if state == nil || len(event.K) == 0 {
				continue
			}
			setAtPath(state, event.K, event.V)

		case 2:
			// Array/complex patch at path K
			if state == nil || len(event.K) == 0 {
				continue
			}
			if event.I != nil {
				// Splice: replace items from index I onwards
				spliceAtPath(state, event.K, event.V, *event.I)
			} else {
				// Append: add items to the end of the existing array
				appendAtPath(state, event.K, event.V)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning JSONL: %w", err)
	}

	if state == nil {
		return nil, fmt.Errorf("no initial state found (kind:0)")
	}

	// Marshal the materialized state back to JSON and unmarshal into chatSession
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("marshaling materialized state: %w", err)
	}

	var session chatSession
	if err := json.Unmarshal(stateJSON, &session); err != nil {
		return nil, fmt.Errorf("parsing materialized state: %w", err)
	}

	return &session, nil
}

// navigateToParent traverses a nested map/slice structure along the key path,
// stopping one level before the final key. Returns the parent container and
// the final key, or nil if the path is invalid.
func navigateToParent(obj map[string]interface{}, keys []interface{}) (interface{}, interface{}) {
	if len(keys) == 0 {
		return nil, nil
	}

	var current interface{} = obj
	for _, key := range keys[:len(keys)-1] {
		switch k := key.(type) {
		case string:
			m, ok := current.(map[string]interface{})
			if !ok {
				return nil, nil
			}
			current = m[k]
		case float64:
			arr, ok := current.([]interface{})
			idx := int(k)
			if !ok || idx < 0 || idx >= len(arr) {
				return nil, nil
			}
			current = arr[idx]
		default:
			return nil, nil
		}
	}
	return current, keys[len(keys)-1]
}

// setAtPath sets a value at the given key path in a nested map/slice structure.
// Keys can be strings (map keys) or float64 (array indices, as JSON numbers decode to float64).
func setAtPath(obj map[string]interface{}, keys []interface{}, value interface{}) {
	parent, lastKey := navigateToParent(obj, keys)
	if parent == nil {
		return
	}

	switch k := lastKey.(type) {
	case string:
		if m, ok := parent.(map[string]interface{}); ok {
			m[k] = value
		}
	case float64:
		if arr, ok := parent.([]interface{}); ok {
			idx := int(k)
			if idx >= 0 && idx < len(arr) {
				arr[idx] = value
			}
		}
	}
}

// spliceAtPath replaces array items from the given index onwards.
// The existing array becomes existing[:spliceIdx] + newItems.
func spliceAtPath(obj map[string]interface{}, keys []interface{}, value interface{}, spliceIdx int) {
	parent, lastKey := navigateToParent(obj, keys)
	if parent == nil {
		return
	}

	k, ok := lastKey.(string)
	if !ok {
		return
	}
	m, ok := parent.(map[string]interface{})
	if !ok {
		return
	}

	newItems, ok := value.([]interface{})
	if !ok {
		m[k] = value // not an array, fall back to replace
		return
	}

	existing, _ := m[k].([]interface{})
	if spliceIdx > len(existing) {
		spliceIdx = len(existing)
	}
	result := make([]interface{}, spliceIdx, spliceIdx+len(newItems))
	copy(result, existing[:spliceIdx])
	result = append(result, newItems...)
	m[k] = result
}

// appendAtPath appends array items to the existing array at the given path.
// If the existing value is not an array, it falls back to a set.
func appendAtPath(obj map[string]interface{}, keys []interface{}, value interface{}) {
	parent, lastKey := navigateToParent(obj, keys)
	if parent == nil {
		return
	}

	k, ok := lastKey.(string)
	if !ok {
		return
	}
	m, ok := parent.(map[string]interface{})
	if !ok {
		return
	}

	newItems, ok := value.([]interface{})
	if !ok {
		m[k] = value // not an array, fall back to replace
		return
	}

	existing, _ := m[k].([]interface{})
	result := make([]interface{}, len(existing), len(existing)+len(newItems))
	copy(result, existing)
	result = append(result, newItems...)
	m[k] = result
}

// buildParsedSession converts a chatSession into an agent.ParsedSession.
// It uses two strategies depending on data availability:
//   - When result.metadata.toolCallRounds is available (completed requests):
//     Uses toolCallRounds as the authoritative text source and response[] for
//     rich details (tool call descriptions, file edits, thinking blocks).
//   - When toolCallRounds is NOT available (in-progress or canceled requests):
//     Walks the deduplicated response[] array directly.
func buildParsedSession(session *chatSession) (*agent.ParsedSession, error) {
	parsed := &agent.ParsedSession{
		SessionID:     session.SessionID,
		Title:         session.CustomTitle,
		CreatedAt:     agent.FormatTimestamp(session.CreationDate),
		CreatedAtMs:   session.CreationDate,
		LastMessageAt: agent.FormatTimestamp(session.LastMessageDate),
		User:          session.RequesterUsername,
		Agent:         session.ResponderUsername,
	}

	for _, request := range session.Requests {
		// User message
		userMessage := agent.ParsedMessage{
			Timestamp:    agent.FormatTimestamp(request.Timestamp),
			Role:         "user",
			Content:      request.Message.Text,
			Confirmation: request.Confirmation,
		}

		// Extract attached file names from variableData
		if request.VariableData != nil {
			for _, variable := range request.VariableData.Variables {
				if variable.Kind == "file" && variable.Name != "" {
					userMessage.Attachments = append(userMessage.Attachments, variable.Name)
				}
			}
		}
		parsed.Messages = append(parsed.Messages, userMessage)

		// Assistant response
		assistantMessage := agent.ParsedMessage{
			Timestamp: agent.FormatTimestamp(request.Timestamp),
			Role:      "assistant",
			Model:     request.ModelID,
			Canceled:  request.IsCanceled,
		}
		if parsed.Model == "" && request.ModelID != "" {
			parsed.Model = request.ModelID
		}

		// Choose strategy: toolCallRounds (authoritative) vs response[] (streaming)
		hasToolCallRounds := request.Result != nil &&
			request.Result.Metadata != nil &&
			len(request.Result.Metadata.ToolCallRounds) > 0

		if hasToolCallRounds {
			buildFromToolCallRounds(&request, &assistantMessage)
		} else {
			buildFromResponseArray(&request, &assistantMessage)
		}

		// Check metadata for maxToolCallsExceeded
		if request.Result != nil && request.Result.Metadata != nil {
			if request.Result.Metadata.MaxToolCallsExceeded {
				assistantMessage.MaxToolCallsExceeded = true
			}
		}

		// Build flat content from text parts for backward compatibility
		var contentParts []string
		for _, part := range assistantMessage.Parts {
			if part.Type == agent.PartText {
				contentParts = append(contentParts, part.Content)
			}
		}
		assistantMessage.Content = strings.Join(contentParts, "\n\n")

		parsed.Messages = append(parsed.Messages, assistantMessage)
	}

	return parsed, nil
}

// buildFromToolCallRounds builds the assistant message using result.metadata.toolCallRounds
// as the authoritative source for text, augmented with tool call details, thinking blocks,
// and file edits from the response[] array.
//
// Why: The JSONL streaming protocol (kind:2 events) can lose text fragments when
// splice operations overwrite intermediate content. toolCallRounds is computed
// server-side and preserves the complete narrative text between tool invocations.
//
// Positional correlation: toolCallRounds uses model API tool-call IDs (e.g., "toolu_vrtx_...")
// while response[] uses VS Code internal IDs (e.g., "07935334-..."). Since these don't match,
// tool calls are correlated by POSITION — the Nth slot in toolCallRounds corresponds to the
// Nth TOP-LEVEL tool call in the response array.
//
// Subagent handling: when the agent spawns subagents (runSubagent), each subagent produces
// its own child tool calls (copilot_findFiles, copilot_readFile, etc.) that appear in
// response[] with a subAgentInvocationId linking them to their parent. These child calls
// do NOT have slots in toolCallRounds (only top-level calls do), so they must be separated
// before positional mapping. After emitting each top-level call, its children are emitted
// immediately after to preserve the complete contrail of agent activity.
func buildFromToolCallRounds(request *chatRequest, assistantMessage *agent.ParsedMessage) {
	deduped := deduplicateResponse(request.Response)

	// 1. Collect tool calls and (optionally) thinking blocks from the deduplicated response.
	//    Separate top-level calls (no subAgentInvocationId) from subagent child calls.
	//    toolCallRounds only has slots for top-level calls; positional mapping must
	//    use only those. Children are emitted immediately after their parent.
	//
	//    Thinking: When toolCallRounds has per-round Thinking fields, those are used
	//    as the authoritative source (emitted inline with each round). When they don't
	//    (older format), thinking blocks are collected from the response array as a
	//    fallback and emitted at the top — matching the previous behavior.
	type toolCallEntry struct {
		internalID string                 // VS Code internal toolCallId
		detail     map[string]interface{} // full response item
	}
	var topLevelCalls []toolCallEntry
	subCallsByParentID := make(map[string][]toolCallEntry) // parentToolCallId → children

	// Check if toolCallRounds has per-round thinking.
	rounds := request.Result.Metadata.ToolCallRounds
	hasRoundThinking := false
	for _, round := range rounds {
		if round.Thinking != nil && strings.TrimSpace(string(round.Thinking.Text)) != "" {
			hasRoundThinking = true
			break
		}
	}

	// Fallback: collect thinking from the response array when rounds don't have it.
	var thinkingBlocks []agent.MessagePart

	for _, item := range deduped {
		responseMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		kind, _ := responseMap["kind"].(string)
		switch kind {
		case "toolInvocationSerialized":
			callID, _ := responseMap["toolCallId"].(string)
			entry := toolCallEntry{internalID: callID, detail: responseMap}
			subAgentID, _ := responseMap["subAgentInvocationId"].(string)
			if subAgentID == "" {
				topLevelCalls = append(topLevelCalls, entry)
			} else {
				subCallsByParentID[subAgentID] = append(subCallsByParentID[subAgentID], entry)
			}

		case "thinking":
			if !hasRoundThinking {
				value, _ := responseMap["value"].(string)
				value = strings.TrimSpace(value)
				if value != "" {
					title, _ := responseMap["generatedTitle"].(string)
					content := "<thinking"
					if title != "" {
						content += fmt.Sprintf(` title="%s"`, title)
					}
					content += ">\n" + value + "\n</thinking>"
					thinkingBlocks = append(thinkingBlocks, agent.MessagePart{
						Type:    agent.PartText,
						Content: content,
					})
				}
			}
		}
	}

	// 2. Build tool-call → file-edits association from the ORIGINAL response.
	//    Walk forward: when we see a tool call, record its internal ID;
	//    when we see a textEditGroup/codeblockUri, associate it with the last tool call.
	//    This covers both top-level and subagent child tool calls.
	type fileEditPart struct {
		part agent.MessagePart
		path string
	}
	toolEditAssoc := make(map[string][]fileEditPart) // internalID → file edit parts
	toolEditSeen := make(map[string]map[string]bool) // internalID → set of paths seen
	filesEditedSeen := make(map[string]bool)
	var lastInternalID string

	for _, item := range request.Response {
		responseMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		kind, _ := responseMap["kind"].(string)

		switch kind {
		case "toolInvocationSerialized":
			if id, _ := responseMap["toolCallId"].(string); id != "" {
				lastInternalID = id
			}

		case "textEditGroup":
			if lastInternalID == "" {
				continue
			}
			if uri, ok := responseMap["uri"].(map[string]interface{}); ok {
				path := extractPath(uri)
				if path == "" {
					continue
				}
				if toolEditSeen[lastInternalID] == nil {
					toolEditSeen[lastInternalID] = make(map[string]bool)
				}
				if !toolEditSeen[lastInternalID][path] {
					toolEditSeen[lastInternalID][path] = true
					toolEditAssoc[lastInternalID] = append(toolEditAssoc[lastInternalID], fileEditPart{
						part: agent.MessagePart{Type: agent.PartFileEdit, FilePath: path},
						path: path,
					})
					if !filesEditedSeen[path] {
						filesEditedSeen[path] = true
						assistantMessage.FilesEdited = append(assistantMessage.FilesEdited, path)
					}
				}
			}

		case "codeblockUri":
			if lastInternalID == "" {
				continue
			}
			if uri, ok := responseMap["uri"].(map[string]interface{}); ok {
				path := extractPath(uri)
				if path == "" {
					continue
				}
				isEdit, _ := responseMap["isEdit"].(bool)
				if toolEditSeen[lastInternalID] == nil {
					toolEditSeen[lastInternalID] = make(map[string]bool)
				}
				// For edit codeblocks, use the same key as textEditGroup to avoid
				// duplicate "Edited:" lines when both exist for the same file.
				// For non-edit codeblocks, use a prefixed key so they're independent.
				var seenKey string
				if isEdit {
					seenKey = path
				} else {
					seenKey = "cb:" + path
				}
				if !toolEditSeen[lastInternalID][seenKey] {
					toolEditSeen[lastInternalID][seenKey] = true
					toolEditAssoc[lastInternalID] = append(toolEditAssoc[lastInternalID], fileEditPart{
						part: agent.MessagePart{Type: agent.PartCodeBlock, FilePath: path, IsEdit: isEdit},
						path: path,
					})
					if !filesEditedSeen[path] {
						filesEditedSeen[path] = true
						assistantMessage.FilesEdited = append(assistantMessage.FilesEdited, path)
					}
				}
			}
		}
	}

	// 3. Emit fallback thinking blocks first (when rounds don't have per-round thinking).
	assistantMessage.Parts = append(assistantMessage.Parts, thinkingBlocks...)

	// 4. Walk toolCallRounds to build parts in authoritative order.
	//    The Nth slot in toolCallRounds maps to topLevelCalls[N] by position.
	//    After each top-level call, emit its subagent children to preserve the
	//    complete contrail of all tool activity during that invocation.
	//
	//    When rounds have per-round Thinking, it's emitted before each round's text,
	//    preserving the agent's thought process at each step. This avoids the
	//    deduplication issue where multiple rounds share the same thinking ID.
	toolCallIdx := 0

	for _, round := range rounds {
		// Emit per-round thinking (authoritative source from toolCallRounds).
		if round.Thinking != nil {
			thinkingText := strings.TrimSpace(string(round.Thinking.Text))
			if thinkingText != "" {
				content := "<thinking>\n" + thinkingText + "\n</thinking>"
				assistantMessage.Parts = append(assistantMessage.Parts, agent.MessagePart{
					Type:    agent.PartText,
					Content: content,
				})
			}
		}

		// Round response text (what the agent said before invoking tools in this round).
		if text := strings.TrimSpace(round.Response); text != "" {
			assistantMessage.Parts = append(assistantMessage.Parts, agent.MessagePart{
				Type:    agent.PartText,
				Content: text,
			})
		}

		for range round.ToolCalls {
			if toolCallIdx >= len(topLevelCalls) {
				continue
			}
			entry := topLevelCalls[toolCallIdx]
			toolCallIdx++

			// Emit the top-level tool call.
			assistantMessage.Parts = append(assistantMessage.Parts, buildToolCallPart(entry.detail))
			for _, edit := range toolEditAssoc[entry.internalID] {
				assistantMessage.Parts = append(assistantMessage.Parts, edit.part)
			}

			// Emit all subagent child calls made during this invocation.
			// This preserves the full contrail: every search, read, and terminal
			// command the subagent performed is recorded after its parent.
			for _, child := range subCallsByParentID[entry.internalID] {
				assistantMessage.Parts = append(assistantMessage.Parts, buildToolCallPart(child.detail))
				for _, edit := range toolEditAssoc[child.internalID] {
					assistantMessage.Parts = append(assistantMessage.Parts, edit.part)
				}
			}
		}
	}

	// 5. Emit any remaining tool calls that were native/VSCode-side and
	//    not counted in the LLM's toolCallRounds metadata.
	for ; toolCallIdx < len(topLevelCalls); toolCallIdx++ {
		entry := topLevelCalls[toolCallIdx]

		assistantMessage.Parts = append(assistantMessage.Parts, buildToolCallPart(entry.detail))
		for _, edit := range toolEditAssoc[entry.internalID] {
			assistantMessage.Parts = append(assistantMessage.Parts, edit.part)
		}

		for _, child := range subCallsByParentID[entry.internalID] {
			assistantMessage.Parts = append(assistantMessage.Parts, buildToolCallPart(child.detail))
			for _, edit := range toolEditAssoc[child.internalID] {
				assistantMessage.Parts = append(assistantMessage.Parts, edit.part)
			}
		}
	}
}

// buildFromResponseArray builds the assistant message by walking the deduplicated
// response[] array directly. Used for in-progress or canceled requests that
// don't have toolCallRounds metadata.
func buildFromResponseArray(request *chatRequest, assistantMessage *agent.ParsedMessage) {
	deduped := deduplicateResponse(request.Response)

	filesEditedSeen := make(map[string]bool)
	var textAccumulator strings.Builder

	flushText := func() {
		merged := strings.TrimSpace(textAccumulator.String())
		if merged != "" && merged != "```" {
			assistantMessage.Parts = append(assistantMessage.Parts, agent.MessagePart{
				Type:    agent.PartText,
				Content: merged,
			})
		}
		textAccumulator.Reset()
	}

	for _, response := range deduped {
		responseMap, ok := response.(map[string]interface{})
		if !ok {
			continue
		}
		kind, _ := responseMap["kind"].(string)

		switch kind {
		case "":
			if value, ok := responseMap["value"].(string); ok {
				if strings.TrimSpace(value) != "```" {
					textAccumulator.WriteString(value)
				}
			}

		case "inlineReference":
			if reference, ok := responseMap["inlineReference"].(map[string]interface{}); ok {
				referenceName := extractReferenceName(reference)
				if referenceName != "" {
					textAccumulator.WriteString("`")
					textAccumulator.WriteString(referenceName)
					textAccumulator.WriteString("`")
				}
			}

		case "toolInvocationSerialized":
			flushText()
			toolID, _ := responseMap["toolId"].(string)
			if toolID == "" {
				continue
			}
			assistantMessage.Parts = append(assistantMessage.Parts, buildToolCallPart(responseMap))

		case "textEditGroup":
			flushText()
			if uri, ok := responseMap["uri"].(map[string]interface{}); ok {
				path := extractPath(uri)
				if path != "" {
					assistantMessage.Parts = append(assistantMessage.Parts, agent.MessagePart{
						Type:     agent.PartFileEdit,
						FilePath: path,
					})
					if !filesEditedSeen[path] {
						filesEditedSeen[path] = true
						assistantMessage.FilesEdited = append(assistantMessage.FilesEdited, path)
					}
				}
			}

		case "codeblockUri":
			flushText()
			if uri, ok := responseMap["uri"].(map[string]interface{}); ok {
				path := extractPath(uri)
				if path != "" {
					isEdit, _ := responseMap["isEdit"].(bool)
					assistantMessage.Parts = append(assistantMessage.Parts, agent.MessagePart{
						Type:     agent.PartCodeBlock,
						FilePath: path,
						IsEdit:   isEdit,
					})
					if !filesEditedSeen[path] {
						filesEditedSeen[path] = true
						assistantMessage.FilesEdited = append(assistantMessage.FilesEdited, path)
					}
				}
			}

		case "thinking":
			if value, ok := responseMap["value"].(string); ok {
				value = strings.TrimSpace(value)
				if value != "" {
					flushText()
					title, _ := responseMap["generatedTitle"].(string)
					content := "<thinking"
					if title != "" {
						content += fmt.Sprintf(` title="%s"`, title)
					}
					content += ">\n" + value + "\n</thinking>"
					assistantMessage.Parts = append(assistantMessage.Parts, agent.MessagePart{
						Type:    agent.PartText,
						Content: content,
					})
				}
			}
		}
	}

	flushText()
}

// buildToolCallPart constructs a PartToolCall MessagePart from a toolInvocationSerialized
// response item. It is used by both buildFromToolCallRounds (for top-level and subagent
// child calls) and buildFromResponseArray (for the streaming path).
func buildToolCallPart(responseMap map[string]interface{}) agent.MessagePart {
	part := agent.MessagePart{Type: agent.PartToolCall}
	part.Tool, _ = responseMap["toolId"].(string)

	// pastTenseMessage (preferred — past tense, more readable) or invocationMessage.
	if ptm, ok := responseMap["pastTenseMessage"].(map[string]interface{}); ok {
		part.ToolArgs, _ = ptm["value"].(string)
	} else if im, ok := responseMap["invocationMessage"].(map[string]interface{}); ok {
		part.ToolArgs, _ = im["value"].(string)
	} else if im, ok := responseMap["invocationMessage"].(string); ok {
		part.ToolArgs = im
	}

	if tsd, ok := responseMap["toolSpecificData"].(map[string]interface{}); ok {
		part.ToolDetail = extractToolDetail(tsd)
	}

	// Merge in file paths from resultDetails (e.g. copilot_findFiles search results).
	if files := extractResultFiles(responseMap); len(files) > 0 {
		if part.ToolDetail == nil {
			part.ToolDetail = &agent.ToolDetail{}
		}
		part.ToolDetail.ResultFiles = files
	}

	return part
}

// extractResultFiles reads the resultDetails array from a toolInvocationSerialized
// response item and returns the file paths it contains. This captures the actual
// files found by search tools such as copilot_findFiles.
func extractResultFiles(responseMap map[string]interface{}) []string {
	resultDetails, ok := responseMap["resultDetails"].([]interface{})
	if !ok || len(resultDetails) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var files []string
	for _, rd := range resultDetails {
		rdMap, ok := rd.(map[string]interface{})
		if !ok {
			continue
		}
		path := extractPath(rdMap)
		if path != "" && !seen[path] {
			seen[path] = true
			files = append(files, path)
		}
	}
	return files
}

// deduplicateResponse removes duplicate items from the response[] array.
// The JSONL streaming protocol creates duplicates when the same items appear
// in both splice updates (kind:2 with i) and append updates (kind:2 without i).
//
// Strategy: walk from end to start, keeping the LAST (most complete) occurrence
// of each uniquely-identified item. Items without identifiers are kept as-is.
// The result preserves order (reversed back after the backward walk).
//
// Identifiers used for deduplication:
//   - toolInvocationSerialized: toolCallId
//   - thinking: id
//   - textEditGroup: uri.path (within a consecutive edit group)
//   - codeblockUri: uri.path (within a consecutive edit group)
//   - undoStop: always deduplicated (noise, only 1 kept per unique predecessor)
func deduplicateResponse(response []interface{}) []interface{} {
	if len(response) == 0 {
		return response
	}

	// Walk backward, collecting unique items
	seenToolCallIDs := make(map[string]bool)
	seenThinkingContent := make(map[string]bool) // content hash → seen

	var result []interface{}

	for i := len(response) - 1; i >= 0; i-- {
		item := response[i]
		responseMap, ok := item.(map[string]interface{})
		if !ok {
			result = append(result, item)
			continue
		}

		kind, _ := responseMap["kind"].(string)
		switch kind {
		case "toolInvocationSerialized":
			callID, _ := responseMap["toolCallId"].(string)
			if callID != "" && seenToolCallIDs[callID] {
				continue // skip earlier duplicate
			}
			if callID != "" {
				seenToolCallIDs[callID] = true
			}
			result = append(result, item)

		case "thinking":
			// Skip "done" markers (empty value with vscodeReasoningDone metadata)
			if meta, ok := responseMap["metadata"].(map[string]interface{}); ok {
				if done, ok := meta["vscodeReasoningDone"].(bool); ok && done {
					continue
				}
			}
			value, _ := responseMap["value"].(string)
			if value == "" {
				continue // skip empty thinking blocks
			}
			// Deduplicate by content, not by ID. VSCode reuses "thinking_0" across
			// different tool call rounds, but each round's thinking has different content.
			// Using ID-only dedup would lose all but one thinking block per request.
			if seenThinkingContent[value] {
				continue // skip exact content duplicate
			}
			seenThinkingContent[value] = true
			result = append(result, item)

		case "textEditGroup":
			// Keep — deduplication happens at the part level in buildFrom* functions
			result = append(result, item)

		case "codeblockUri":
			result = append(result, item)

		case "undoStop", "mcpServersStarting", "progressTaskSerialized",
			"prepareToolInvocation", "confirmation":
			// Skip noise items entirely
			continue

		default:
			// Text, inlineReference, and anything else: keep
			result = append(result, item)
		}
	}

	// Reverse to restore forward order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// extractPath gets fsPath or path from a URI map.
func extractPath(uri map[string]interface{}) string {
	if fsPath, ok := uri["fsPath"].(string); ok && fsPath != "" {
		return fsPath
	}
	if pathValue, ok := uri["path"].(string); ok && pathValue != "" {
		return pathValue
	}
	return ""
}

// extractReferenceName gets a display name for an inline reference.
// File references have fsPath/path directly; symbol references have a "name" field.
func extractReferenceName(reference map[string]interface{}) string {
	// File reference — has fsPath or path, extract just the filename
	if fsPath, ok := reference["fsPath"].(string); ok && fsPath != "" {
		return filepath.Base(fsPath)
	}
	if pathValue, ok := reference["path"].(string); ok && pathValue != "" {
		return filepath.Base(pathValue)
	}
	// Symbol reference — has a "name" field (e.g., "Confirmation", "Project")
	if name, ok := reference["name"].(string); ok && name != "" {
		return name
	}
	return ""
}

// extractToolDetail extracts rich data from toolSpecificData for supported tool types.
func extractToolDetail(toolSpecificData map[string]interface{}) *agent.ToolDetail {
	kind, _ := toolSpecificData["kind"].(string)
	if kind == "" {
		return nil
	}
	detail := &agent.ToolDetail{Kind: kind}

	switch kind {
	case "terminal":
		// Extract the command that was run
		if commandLine, ok := toolSpecificData["commandLine"].(map[string]interface{}); ok {
			if original, ok := commandLine["original"].(string); ok {
				detail.Command = original
			}
		}

	case "todoList":
		// Extract the todo items
		if todoList, ok := toolSpecificData["todoList"].([]interface{}); ok {
			for _, item := range todoList {
				if todoItem, ok := item.(map[string]interface{}); ok {
					todo := agent.TodoItem{
						ID:     fmt.Sprintf("%v", todoItem["id"]),
						Title:  fmt.Sprintf("%v", todoItem["title"]),
						Status: fmt.Sprintf("%v", todoItem["status"]),
					}
					detail.Todos = append(detail.Todos, todo)
				}
			}
		}

	default:
		return nil // unsupported kind, don't attach
	}

	return detail
}

// ExtractFirstMessageDate reads a VS Code chat session file and returns the
// earliest timestamp found (session creation time) as unix milliseconds.
func ExtractFirstMessageDate(filePath string) (int64, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, err
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return 0, nil
	}

	// Check if it's a fully materialized JSON with creationDate
	var session struct {
		CreationDate int64 `json:"creationDate"`
	}
	if err := json.Unmarshal(data, &session); err == nil && session.CreationDate > 0 {
		return session.CreationDate, nil
	}

	// Fall back to JSONL: find first kind:0 creationDate
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var peek struct {
			Kind int             `json:"kind"`
			V    json.RawMessage `json:"v"`
		}
		if err := json.Unmarshal(line, &peek); err != nil {
			continue
		}
		if peek.Kind == 0 {
			var state struct {
				CreationDate int64 `json:"creationDate"`
			}
			if json.Unmarshal(peek.V, &state) == nil && state.CreationDate > 0 {
				return state.CreationDate, nil
			}
		}
	}
	return 0, nil
}

// ExtractLastMessageDate reads a VS Code chat session JSONL file
// and returns the most recent timestamp found.
// This is cheaper than full parsing and is used to decide whether a file
// has genuinely new content.
func ExtractLastMessageDate(filePath string) (int64, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, err
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return 0, nil
	}

	// Check if it's a fully materialized JSON
	var session struct {
		LastMessageDate int64 `json:"lastMessageDate"`
	}
	if err := json.Unmarshal(data, &session); err == nil && session.LastMessageDate > 0 {
		return session.LastMessageDate, nil
	}

	// Fall back to JSONL scanning
	return extractLastMessageDateJSONL(data)
}

// extractLastMessageDateJSONL scans a JSONL file for the latest timestamp.
// It looks at kind:0 creationDate and any completedAt values in modelState patches
// to find the most recent activity without full materialization.
func extractLastMessageDateJSONL(data []byte) (int64, error) {
	var latest int64

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		// Quick peek at kind
		var peek struct {
			Kind int             `json:"kind"`
			K    []interface{}   `json:"k,omitempty"`
			V    json.RawMessage `json:"v"`
		}
		if err := json.Unmarshal(line, &peek); err != nil {
			continue
		}

		switch peek.Kind {
		case 0:
			// Extract creationDate from initial state
			var state struct {
				CreationDate int64 `json:"creationDate"`
			}
			if json.Unmarshal(peek.V, &state) == nil && state.CreationDate > latest {
				latest = state.CreationDate
			}

		case 1:
			// Look for modelState patches with completedAt
			// k: ["requests", N, "modelState"]
			if len(peek.K) == 3 {
				if key2, ok := peek.K[2].(string); ok && key2 == "modelState" {
					var ms struct {
						CompletedAt int64 `json:"completedAt"`
					}
					if json.Unmarshal(peek.V, &ms) == nil && ms.CompletedAt > latest {
						latest = ms.CompletedAt
					}
				}
			}

		case 2:
			// Look for requests arrays with timestamps
			if len(peek.K) == 1 {
				if key0, ok := peek.K[0].(string); ok && key0 == "requests" {
					var requests []struct {
						Timestamp int64 `json:"timestamp"`
					}
					if json.Unmarshal(peek.V, &requests) == nil {
						for _, req := range requests {
							if req.Timestamp > latest {
								latest = req.Timestamp
							}
						}
					}
				}
			}
		}
	}

	return latest, scanner.Err()
}

// ExtractSessionSignature computes a quick hash of the parts of a chat session
// that matter for rendering (requests array and customTitle). It ignores arbitrary
// metadata changes that don't affect the chat's content.
func ExtractSessionSignature(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return "", nil
	}

	hash := sha256.New()

	// Legacy JSON format check
	var dummy struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(trimmed, &dummy); err == nil && dummy.Version > 0 {
		// Just hash the whole file for legacy cases since they aren't actively streaming
		hash.Write(trimmed)
		return hex.EncodeToString(hash.Sum(nil)), nil
	}

	// JSONL format
	scanner := bufio.NewScanner(bytes.NewReader(trimmed))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var peek struct {
			Kind int             `json:"kind"`
			K    []interface{}   `json:"k,omitempty"`
			V    json.RawMessage `json:"v"`
		}
		if err := json.Unmarshal(line, &peek); err != nil {
			continue
		}

		if peek.Kind == 0 {
			// Extract requests and customTitle
			var state struct {
				Requests    json.RawMessage `json:"requests"`
				CustomTitle string          `json:"customTitle"`
			}
			if err := json.Unmarshal(peek.V, &state); err == nil {
				hash.Write(state.Requests)
				hash.Write([]byte(state.CustomTitle))
			}
			continue
		}

		// Patch events (kind 1 or 2)
		if len(peek.K) > 0 {
			if key0, ok := peek.K[0].(string); ok {
				// Only include patches to "requests" or "customTitle"
				if key0 == "requests" || key0 == "customTitle" {
					hash.Write(line)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// HasRequests returns true if the session file contains at least one request.
// This is a lightweight check used to filter empty sessions from the chat list.
func HasRequests(filePath string) bool {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return false
	}

	// Plain JSON: check if requests array is non-empty
	var session struct {
		Requests []json.RawMessage `json:"requests"`
	}
	if err := json.Unmarshal(trimmed, &session); err == nil {
		return len(session.Requests) > 0
	}

	// JSONL: check initial state and patches
	scanner := bufio.NewScanner(bytes.NewReader(trimmed))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var peek struct {
			Kind int             `json:"kind"`
			K    []interface{}   `json:"k,omitempty"`
			V    json.RawMessage `json:"v"`
		}
		if err := json.Unmarshal(line, &peek); err != nil {
			continue
		}

		if peek.Kind == 0 {
			// Check if initial requests array is non-empty
			var state struct {
				Requests []json.RawMessage `json:"requests"`
			}
			if json.Unmarshal(peek.V, &state) == nil && len(state.Requests) > 0 {
				return true
			}
			continue
		}

		// Any patch targeting "requests" means requests were added
		if (peek.Kind == 1 || peek.Kind == 2) && len(peek.K) > 0 {
			if key0, ok := peek.K[0].(string); ok && key0 == "requests" {
				return true
			}
		}
	}

	return false
}

// IsChatSessionFile returns true if the filename looks like a VS Code
// Copilot chat session JSON or JSONL file.
func IsChatSessionFile(name string) bool {
	return strings.HasSuffix(name, ".jsonl") || strings.HasSuffix(name, ".json")
}

// ExtractTitle reads a VS Code chat session JSON or JSONL file and returns the
// customTitle if present, or derives a title from the first user message.
// Returns empty string if the file cannot be read or has no user messages.
func ExtractTitle(filePath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	// Try parsing as a fully materialized JSON object first
	var session struct {
		CustomTitle string        `json:"customTitle"`
		Requests    []chatRequest `json:"requests"`
	}
	if err := json.Unmarshal(data, &session); err == nil {
		if session.CustomTitle != "" {
			return session.CustomTitle
		}
		if len(session.Requests) > 0 && session.Requests[0].Message.Text != "" {
			return deriveTitle(session.Requests[0].Message.Text)
		}
		return ""
	}

	// JSONL v3 format — the kind:0 event typically has empty requests;
	// customTitle and requests are added via kind:1/kind:2 patch events.
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	var customTitle string
	var firstUserMessage string

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var event struct {
			Kind int             `json:"kind"`
			K    []interface{}   `json:"k,omitempty"`
			V    json.RawMessage `json:"v"`
		}
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		switch event.Kind {
		case 0:
			// Initial state snapshot — check for customTitle and requests
			var state struct {
				CustomTitle string `json:"customTitle"`
				Requests    []struct {
					Message struct {
						Text string `json:"text"`
					} `json:"message"`
				} `json:"requests"`
			}
			if json.Unmarshal(event.V, &state) == nil {
				if state.CustomTitle != "" {
					customTitle = state.CustomTitle
				}
				if firstUserMessage == "" && len(state.Requests) > 0 && state.Requests[0].Message.Text != "" {
					firstUserMessage = state.Requests[0].Message.Text
				}
			}

		case 1:
			// Scalar patch — check if it sets customTitle
			if len(event.K) == 1 {
				if key, ok := event.K[0].(string); ok && key == "customTitle" {
					var title string
					if json.Unmarshal(event.V, &title) == nil && title != "" {
						customTitle = title
					}
				}
			}

		case 2:
			// Array patch — check if it adds requests
			if firstUserMessage == "" && len(event.K) > 0 {
				if key, ok := event.K[0].(string); ok && key == "requests" {
					var requests []struct {
						Message struct {
							Text string `json:"text"`
						} `json:"message"`
					}
					if json.Unmarshal(event.V, &requests) == nil && len(requests) > 0 && requests[0].Message.Text != "" {
						firstUserMessage = requests[0].Message.Text
					}
				}
			}
		}
	}

	if customTitle != "" {
		return customTitle
	}
	if firstUserMessage != "" {
		return deriveTitle(firstUserMessage)
	}
	return ""
}

// deriveTitle generates a session title from the first user message.
// It truncates to the first 100 characters of the first line.
func deriveTitle(firstMessage string) string {
	const maxTitleLength = 100
	firstLine := firstMessage
	if newlineIndex := strings.IndexByte(firstMessage, '\n'); newlineIndex >= 0 {
		firstLine = firstMessage[:newlineIndex]
	}
	firstLine = strings.TrimSpace(firstLine)
	if len(firstLine) > maxTitleLength {
		firstLine = firstLine[:maxTitleLength]
	}
	return firstLine
}

// FindSourceFile locates the chat session JSON/JSONL file for a given session ID
// inside watchDir. Returns the path if found, or empty string if not.
func FindSourceFile(watchDir, sessionID string) string {
	// VS Code can store these as .json
	path := filepath.Join(watchDir, sessionID+".json")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	// Backward compatibility/mock testing
	path = filepath.Join(watchDir, sessionID+".jsonl")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}
