package services

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/daodao97/xgo/xrequest"
)

type anthropicSSEEvent struct {
	Event string
	Data  map[string]interface{}
}

type claudeResponseTransformState struct {
	apiFormat string
	isStream  bool

	pendingEventType string
	pendingDataLines []string

	chat      claudeChatStreamState
	responses claudeResponsesStreamState
}

type claudeChatStreamState struct {
	MessageID string
	Model     string

	HasSentMessageStart bool
	HasMessageStopSent  bool

	NextContentIndex     int
	CurrentBlockType     string
	CurrentBlockIndex    int
	HasCurrentBlock      bool
	ToolBlocksByIndex    map[int]*claudeChatToolBlockState
	OpenToolBlockIndices map[int]bool
	HasSeenToolUse       bool
}

type claudeChatToolBlockState struct {
	AnthropicIndex int
	ID             string
	Name           string
	Started        bool
	PendingArgs    string
}

type claudeResponsesStreamState struct {
	MessageID string
	Model     string

	HasSentMessageStart bool
	HasMessageStopSent  bool
	HasToolUse          bool

	NextContentIndex     int
	IndexByKey           map[string]int
	OpenIndices          map[int]bool
	CurrentTextIndex     int
	HasCurrentTextIndex  bool
	FallbackOpenIndex    int
	HasFallbackOpenIndex bool
	ToolIndexByItemID    map[string]int
	ToolArgsByIndex      map[int]string
	ToolNameByIndex      map[int]string
	FinishedToolKeys     map[string]bool
	LastToolIndex        int
	HasLastToolIndex     bool
}

func newClaudeResponseTransformHook(apiFormat string, isStream bool) xrequest.ResponseHook {
	state := &claudeResponseTransformState{
		apiFormat: normalizeClaudeAPIFormat(apiFormat),
		isStream:  isStream,
		chat: claudeChatStreamState{
			ToolBlocksByIndex:    make(map[int]*claudeChatToolBlockState),
			OpenToolBlockIndices: make(map[int]bool),
		},
		responses: claudeResponsesStreamState{
			IndexByKey:        make(map[string]int),
			OpenIndices:       make(map[int]bool),
			ToolIndexByItemID: make(map[string]int),
			ToolArgsByIndex:   make(map[int]string),
			ToolNameByIndex:   make(map[int]string),
			FinishedToolKeys:  make(map[string]bool),
		},
	}

	return func(data []byte) (bool, []byte) {
		if !state.isStream {
			transformed, err := transformClaudeResponseForAPIFormat(data, state.apiFormat)
			if err != nil {
				return true, data
			}
			return true, transformed
		}

		line := strings.TrimSpace(string(data))
		if line == "" {
			if len(state.pendingDataLines) > 0 {
				payload := combineOpenAIResponsesDataLines(state.pendingDataLines)
				state.pendingDataLines = nil
				return state.transformPayload(payload, data)
			}
			return false, nil
		}

		if strings.HasPrefix(line, "event:") {
			if state.apiFormat == claudeAPIFormatOpenAIResponse {
				state.pendingEventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			}
			return false, nil
		}

		if strings.HasPrefix(line, "id:") || strings.HasPrefix(line, "retry:") || strings.HasPrefix(line, ":") {
			return false, nil
		}

		if !strings.HasPrefix(line, "data:") {
			return true, data
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if len(state.pendingDataLines) > 0 {
			state.pendingDataLines = append(state.pendingDataLines, payload)
			combinedPayload := combineOpenAIResponsesDataLines(state.pendingDataLines)
			if json.Valid([]byte(combinedPayload)) {
				state.pendingDataLines = nil
				return state.transformPayload(combinedPayload, data)
			}
			return false, nil
		}
		if payload != "" && payload != "[DONE]" && !json.Valid([]byte(payload)) {
			state.pendingDataLines = append(state.pendingDataLines, payload)
			return false, nil
		}
		return state.transformPayload(payload, data)
	}
}

func (state *claudeResponseTransformState) transformPayload(payload string, fallback []byte) (bool, []byte) {
	switch payload {
	case "", "[DONE]":
		if state.apiFormat == claudeAPIFormatOpenAIChat && !state.chat.HasMessageStopSent {
			state.chat.HasMessageStopSent = true
			return true, formatAnthropicSSEEvents([]anthropicSSEEvent{
				newAnthropicSSEEvent("message_stop", map[string]interface{}{
					"type": "message_stop",
				}),
			})
		}
		return false, nil
	}

	var parsed map[string]interface{}
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&parsed); err != nil {
		return true, fallback
	}
	if state.apiFormat == claudeAPIFormatOpenAIResponse {
		if getString(parsed, "type") == "" && state.pendingEventType != "" {
			parsed["type"] = state.pendingEventType
		}
		state.pendingEventType = ""
	}

	var events []anthropicSSEEvent
	switch state.apiFormat {
	case claudeAPIFormatOpenAIChat:
		events = state.chat.transformChunk(parsed)
	case claudeAPIFormatOpenAIResponse:
		events = state.responses.transformChunk(parsed)
	default:
		return true, fallback
	}

	if len(events) == 0 {
		return false, nil
	}
	return true, formatAnthropicSSEEvents(events)
}

func newAnthropicSSEEvent(event string, data map[string]interface{}) anthropicSSEEvent {
	return anthropicSSEEvent{
		Event: event,
		Data:  data,
	}
}

func formatAnthropicSSEEvents(events []anthropicSSEEvent) []byte {
	blocks := make([]string, 0, len(events))
	for _, event := range events {
		payload, err := json.Marshal(event.Data)
		if err != nil {
			continue
		}
		blocks = append(blocks, fmt.Sprintf("event: %s\ndata: %s", event.Event, payload))
	}
	if len(blocks) == 0 {
		return nil
	}
	return []byte(strings.Join(blocks, "\n\n") + "\n")
}

func (state *claudeChatStreamState) transformChunk(chunk map[string]interface{}) []anthropicSSEEvent {
	events := make([]anthropicSSEEvent, 0)

	if state.MessageID == "" {
		state.MessageID = getString(chunk, "id")
	}
	if state.Model == "" {
		state.Model = getString(chunk, "model")
	}

	choices, ok := chunk["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return events
	}
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return events
	}
	delta, _ := choice["delta"].(map[string]interface{})

	if !state.HasSentMessageStart {
		state.HasSentMessageStart = true
		events = append(events, newAnthropicSSEEvent("message_start", map[string]interface{}{
			"type": "message_start",
			"message": map[string]interface{}{
				"id":    state.MessageID,
				"type":  "message",
				"role":  "assistant",
				"model": state.Model,
				"usage": buildAnthropicUsageFromOpenAI(chunk["usage"]),
			},
		}))
	}

	if reasoning := getString(delta, "reasoning"); reasoning != "" {
		events = append(events, state.openNonToolBlock("thinking")...)
		if state.HasCurrentBlock {
			events = append(events, newAnthropicSSEEvent("content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": state.CurrentBlockIndex,
				"delta": map[string]interface{}{
					"type":     "thinking_delta",
					"thinking": reasoning,
				},
			}))
		}
	}

	if content := getString(delta, "content"); content != "" {
		events = append(events, state.openNonToolBlock("text")...)
		if state.HasCurrentBlock {
			events = append(events, newAnthropicSSEEvent("content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": state.CurrentBlockIndex,
				"delta": map[string]interface{}{
					"type": "text_delta",
					"text": content,
				},
			}))
		}
	}

	if toolCalls, ok := delta["tool_calls"].([]interface{}); ok && len(toolCalls) > 0 {
		events = append(events, state.closeCurrentNonToolBlock()...)
		for _, entry := range toolCalls {
			call, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			index := int(numberFromValue(call["index"]))
			blockState, exists := state.ToolBlocksByIndex[index]
			if !exists {
				blockState = &claudeChatToolBlockState{
					AnthropicIndex: state.NextContentIndex,
				}
				state.ToolBlocksByIndex[index] = blockState
				state.NextContentIndex++
			}
			if id := getString(call, "id"); id != "" {
				blockState.ID = id
			}
			function, _ := call["function"].(map[string]interface{})
			if name := getString(function, "name"); name != "" {
				blockState.Name = name
			}
			argsDelta := getString(function, "arguments")
			if !blockState.Started {
				if blockState.ID != "" && blockState.Name != "" {
					blockState.Started = true
					state.OpenToolBlockIndices[blockState.AnthropicIndex] = true
					state.HasSeenToolUse = true
					events = append(events, newAnthropicSSEEvent("content_block_start", map[string]interface{}{
						"type":  "content_block_start",
						"index": blockState.AnthropicIndex,
						"content_block": map[string]interface{}{
							"type": "tool_use",
							"id":   blockState.ID,
							"name": blockState.Name,
						},
					}))
					if blockState.PendingArgs != "" {
						if isClaudeReadToolName(blockState.Name) {
							state.HasSeenToolUse = true
						} else {
							events = append(events, newAnthropicSSEEvent("content_block_delta", map[string]interface{}{
								"type":  "content_block_delta",
								"index": blockState.AnthropicIndex,
								"delta": map[string]interface{}{
									"type":         "input_json_delta",
									"partial_json": blockState.PendingArgs,
								},
							}))
							blockState.PendingArgs = ""
						}
					}
				}
			}
			if argsDelta != "" {
				if isClaudeReadToolName(blockState.Name) {
					blockState.PendingArgs += argsDelta
				} else if blockState.Started {
					events = append(events, newAnthropicSSEEvent("content_block_delta", map[string]interface{}{
						"type":  "content_block_delta",
						"index": blockState.AnthropicIndex,
						"delta": map[string]interface{}{
							"type":         "input_json_delta",
							"partial_json": argsDelta,
						},
					}))
				} else {
					blockState.PendingArgs += argsDelta
				}
			}
		}
	}

	finishReason := getString(choice, "finish_reason")
	if finishReason != "" {
		events = append(events, state.closeCurrentNonToolBlock()...)
		toolIndices := make([]int, 0, len(state.OpenToolBlockIndices))
		for index := range state.OpenToolBlockIndices {
			toolIndices = append(toolIndices, index)
		}
		sort.Ints(toolIndices)
		for _, index := range toolIndices {
			if blockState := state.toolBlockForAnthropicIndex(index); blockState != nil && blockState.PendingArgs != "" {
				events = append(events, newAnthropicSSEEvent("content_block_delta", map[string]interface{}{
					"type":  "content_block_delta",
					"index": index,
					"delta": map[string]interface{}{
						"type":         "input_json_delta",
						"partial_json": sanitizeAnthropicToolUseArguments(blockState.Name, blockState.PendingArgs),
					},
				}))
				blockState.PendingArgs = ""
			}
			delete(state.OpenToolBlockIndices, index)
			events = append(events, newAnthropicSSEEvent("content_block_stop", map[string]interface{}{
				"type":  "content_block_stop",
				"index": index,
			}))
		}
		usage := interface{}(nil)
		if chunk["usage"] != nil {
			usage = buildAnthropicUsageFromOpenAI(chunk["usage"])
		}
		events = append(events, newAnthropicSSEEvent("message_delta", map[string]interface{}{
			"type": "message_delta",
			"delta": map[string]interface{}{
				"stop_reason":   mapOpenAIFinishReasonToAnthropic(finishReason, state.HasSeenToolUse),
				"stop_sequence": nil,
			},
			"usage": usage,
		}))
	}

	return events
}

func (state *claudeChatStreamState) openNonToolBlock(blockType string) []anthropicSSEEvent {
	events := make([]anthropicSSEEvent, 0)
	if state.HasCurrentBlock && state.CurrentBlockType == blockType {
		return events
	}

	events = append(events, state.closeCurrentNonToolBlock()...)

	index := state.NextContentIndex
	state.NextContentIndex++
	state.CurrentBlockType = blockType
	state.CurrentBlockIndex = index
	state.HasCurrentBlock = true

	contentBlock := map[string]interface{}{
		"type": blockType,
	}
	if blockType == "text" {
		contentBlock["text"] = ""
	} else {
		contentBlock["thinking"] = ""
	}
	events = append(events, newAnthropicSSEEvent("content_block_start", map[string]interface{}{
		"type":          "content_block_start",
		"index":         index,
		"content_block": contentBlock,
	}))
	return events
}

func (state *claudeChatStreamState) toolBlockForAnthropicIndex(index int) *claudeChatToolBlockState {
	for _, blockState := range state.ToolBlocksByIndex {
		if blockState != nil && blockState.AnthropicIndex == index {
			return blockState
		}
	}
	return nil
}

func (state *claudeChatStreamState) closeCurrentNonToolBlock() []anthropicSSEEvent {
	if !state.HasCurrentBlock {
		return nil
	}
	index := state.CurrentBlockIndex
	state.CurrentBlockType = ""
	state.CurrentBlockIndex = 0
	state.HasCurrentBlock = false
	return []anthropicSSEEvent{
		newAnthropicSSEEvent("content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": index,
		}),
	}
}

func (state *claudeResponsesStreamState) transformChunk(data map[string]interface{}) []anthropicSSEEvent {
	eventType := getString(data, "type")
	if eventType == "" {
		return nil
	}

	events := make([]anthropicSSEEvent, 0)
	switch eventType {
	case "response.created":
		responseObj := responseObjectFromEvent(data)
		if state.MessageID == "" {
			state.MessageID = getString(responseObj, "id")
		}
		if state.Model == "" {
			state.Model = getString(responseObj, "model")
		}
		state.HasSentMessageStart = true
		events = append(events, newAnthropicSSEEvent("message_start", map[string]interface{}{
			"type": "message_start",
			"message": map[string]interface{}{
				"id":    state.MessageID,
				"type":  "message",
				"role":  "assistant",
				"model": state.Model,
				"usage": buildAnthropicUsageFromResponses(responseObj["usage"]),
			},
		}))
	case "response.content_part.added":
		part, _ := data["part"].(map[string]interface{})
		partType := getString(part, "type")
		if partType == "output_text" || partType == "refusal" {
			events = append(events, state.ensureMessageStart()...)
			index := state.currentOrResolvedTextIndex(data)
			if !state.OpenIndices[index] {
				state.OpenIndices[index] = true
				events = append(events, newAnthropicSSEEvent("content_block_start", map[string]interface{}{
					"type":  "content_block_start",
					"index": index,
					"content_block": map[string]interface{}{
						"type": "text",
						"text": "",
					},
				}))
			}
		}
	case "response.output_text.delta", "response.refusal.delta":
		delta := getString(data, "delta")
		if delta == "" {
			break
		}
		index := state.currentOrResolvedTextIndex(data)
		if !state.OpenIndices[index] {
			state.OpenIndices[index] = true
			events = append(events, newAnthropicSSEEvent("content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": index,
				"content_block": map[string]interface{}{
					"type": "text",
					"text": "",
				},
			}))
		}
		events = append(events, newAnthropicSSEEvent("content_block_delta", map[string]interface{}{
			"type":  "content_block_delta",
			"index": index,
			"delta": map[string]interface{}{
				"type": "text_delta",
				"text": delta,
			},
		}))
	case "response.output_text.done", "response.refusal.done", "response.content_part.done":
		events = append(events, state.closeCurrentTextIndex(data)...)
	case "response.output_item.added":
		item, _ := data["item"].(map[string]interface{})
		if getString(item, "type") != "function_call" {
			break
		}
		state.HasToolUse = true
		events = append(events, state.closeCurrentTextIndex(nil)...)
		events = append(events, state.ensureMessageStart()...)
		index := state.resolveToolIndexFromAdded(data, item)
		callID := getString(item, "call_id")
		if callID == "" {
			callID = getString(item, "id")
		}
		name := getString(item, "name")
		if name != "" {
			state.ToolNameByIndex[index] = name
		}
		if !state.OpenIndices[index] {
			state.OpenIndices[index] = true
			events = append(events, newAnthropicSSEEvent("content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": index,
				"content_block": map[string]interface{}{
					"type": "tool_use",
					"id":   callID,
					"name": name,
				},
			}))
		}
	case "response.output_item.done":
		item, _ := data["item"].(map[string]interface{})
		if getString(item, "type") != "function_call" {
			break
		}
		if state.isFunctionCallItemFinished(data) {
			break
		}
		state.HasToolUse = true
		events = append(events, state.closeCurrentTextIndex(nil)...)
		events = append(events, state.ensureMessageStart()...)
		events = append(events, state.finishFunctionCallItem(data, item)...)
	case "response.function_call_arguments.delta":
		delta := getString(data, "delta")
		if delta == "" {
			break
		}
		index := state.resolveToolIndexFromEvent(data)
		if !state.OpenIndices[index] {
			state.OpenIndices[index] = true
			callID := getString(data, "call_id")
			if callID == "" {
				callID = getString(data, "item_id")
			}
			name := getString(data, "name")
			if name != "" {
				state.ToolNameByIndex[index] = name
			} else {
				name = state.toolNameForIndex(index)
			}
			events = append(events, newAnthropicSSEEvent("content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": index,
				"content_block": map[string]interface{}{
					"type": "tool_use",
					"id":   callID,
					"name": name,
				},
			}))
		} else if name := getString(data, "name"); name != "" {
			state.ToolNameByIndex[index] = name
		}
		state.ToolArgsByIndex[index] += delta
		if !isClaudeReadToolName(state.toolNameForIndex(index)) {
			events = append(events, newAnthropicSSEEvent("content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": index,
				"delta": map[string]interface{}{
					"type":         "input_json_delta",
					"partial_json": delta,
				},
			}))
		}
	case "response.function_call_arguments.done":
		index, ok := state.lookupToolIndexFromEvent(data)
		if ok && state.OpenIndices[index] {
			if item, _ := data["item"].(map[string]interface{}); item != nil {
				if name := getString(item, "name"); name != "" {
					state.ToolNameByIndex[index] = name
				}
			}
			toolName := state.toolNameForIndex(index)
			arguments := responsesFunctionCallDoneArguments(data)
			if collected := state.ToolArgsByIndex[index]; collected != "" && isClaudeReadToolName(toolName) {
				arguments = collected
			}
			if arguments != "" && (state.ToolArgsByIndex[index] == "" || isClaudeReadToolName(toolName)) {
				events = append(events, newAnthropicSSEEvent("content_block_delta", map[string]interface{}{
					"type":  "content_block_delta",
					"index": index,
					"delta": map[string]interface{}{
						"type":         "input_json_delta",
						"partial_json": sanitizeAnthropicToolUseArguments(toolName, arguments),
					},
				}))
			}
			delete(state.OpenIndices, index)
			delete(state.ToolArgsByIndex, index)
			delete(state.ToolNameByIndex, index)
			events = append(events, newAnthropicSSEEvent("content_block_stop", map[string]interface{}{
				"type":  "content_block_stop",
				"index": index,
			}))
			state.markFunctionCallItemFinished(data)
			if itemID := toolItemIDFromEvent(data); itemID != "" {
				delete(state.ToolIndexByItemID, itemID)
			}
		}
	case "response.reasoning.delta", "response.reasoning_summary_text.delta":
		delta := getString(data, "delta")
		if delta == "" {
			delta = getString(data, "text")
		}
		if delta == "" {
			break
		}
		events = append(events, state.closeCurrentTextIndex(nil)...)
		index := state.resolveContentIndex(data)
		if !state.OpenIndices[index] {
			state.OpenIndices[index] = true
			events = append(events, newAnthropicSSEEvent("content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": index,
				"content_block": map[string]interface{}{
					"type":     "thinking",
					"thinking": "",
				},
			}))
		}
		events = append(events, newAnthropicSSEEvent("content_block_delta", map[string]interface{}{
			"type":  "content_block_delta",
			"index": index,
			"delta": map[string]interface{}{
				"type":     "thinking_delta",
				"thinking": delta,
			},
		}))
	case "response.reasoning.done", "response.reasoning_summary_text.done":
		index, ok := state.lookupContentIndex(data)
		if ok && state.OpenIndices[index] {
			delete(state.OpenIndices, index)
			events = append(events, newAnthropicSSEEvent("content_block_stop", map[string]interface{}{
				"type":  "content_block_stop",
				"index": index,
			}))
			if state.HasFallbackOpenIndex && state.FallbackOpenIndex == index {
				state.HasFallbackOpenIndex = false
			}
		}
	case "response.completed", "response.done", "response.incomplete", "response.failed":
		events = append(events, state.finishResponseMessage(data)...)
	}

	return events
}

func (state *claudeResponsesStreamState) finishFunctionCallItem(data map[string]interface{}, item map[string]interface{}) []anthropicSSEEvent {
	events := make([]anthropicSSEEvent, 0)
	index := state.resolveToolIndexFromEvent(data)
	toolName := getString(item, "name")
	if toolName != "" {
		state.ToolNameByIndex[index] = toolName
	} else {
		toolName = state.toolNameForIndex(index)
	}
	if !state.OpenIndices[index] {
		state.OpenIndices[index] = true
		callID := getString(item, "call_id")
		if callID == "" {
			callID = getString(item, "id")
		}
		events = append(events, newAnthropicSSEEvent("content_block_start", map[string]interface{}{
			"type":  "content_block_start",
			"index": index,
			"content_block": map[string]interface{}{
				"type": "tool_use",
				"id":   callID,
				"name": toolName,
			},
		}))
	}

	arguments := responsesFunctionCallDoneArguments(data)
	if arguments == "" {
		arguments = getString(item, "arguments")
	}
	if collected := state.ToolArgsByIndex[index]; collected != "" && isClaudeReadToolName(toolName) {
		arguments = collected
	}
	if arguments != "" && (state.ToolArgsByIndex[index] == "" || isClaudeReadToolName(toolName)) {
		events = append(events, newAnthropicSSEEvent("content_block_delta", map[string]interface{}{
			"type":  "content_block_delta",
			"index": index,
			"delta": map[string]interface{}{
				"type":         "input_json_delta",
				"partial_json": sanitizeAnthropicToolUseArguments(toolName, arguments),
			},
		}))
	}

	delete(state.OpenIndices, index)
	delete(state.ToolArgsByIndex, index)
	delete(state.ToolNameByIndex, index)
	events = append(events, newAnthropicSSEEvent("content_block_stop", map[string]interface{}{
		"type":  "content_block_stop",
		"index": index,
	}))
	state.markFunctionCallItemFinished(data)
	if itemID := toolItemIDFromEvent(data); itemID != "" {
		delete(state.ToolIndexByItemID, itemID)
	}
	return events
}

func (state *claudeResponsesStreamState) toolNameForIndex(index int) string {
	if state == nil || state.ToolNameByIndex == nil {
		return ""
	}
	return state.ToolNameByIndex[index]
}

func (state *claudeResponsesStreamState) isFunctionCallItemFinished(data map[string]interface{}) bool {
	if state == nil {
		return false
	}
	for _, key := range functionCallFinishKeys(data) {
		if state.FinishedToolKeys[key] {
			return true
		}
	}
	return false
}

func (state *claudeResponsesStreamState) markFunctionCallItemFinished(data map[string]interface{}) {
	if state == nil {
		return
	}
	if state.FinishedToolKeys == nil {
		state.FinishedToolKeys = make(map[string]bool)
	}
	for _, key := range functionCallFinishKeys(data) {
		state.FinishedToolKeys[key] = true
	}
}

func (state *claudeResponsesStreamState) finishResponseMessage(data map[string]interface{}) []anthropicSSEEvent {
	responseObj := responseObjectFromEvent(data)
	status := responseStatusFromEvent(data, responseObj)

	events := make([]anthropicSSEEvent, 0)
	events = append(events, state.ensureMessageStart()...)
	events = append(events, state.closeAllOpenBlocks()...)
	events = append(events, newAnthropicSSEEvent("message_delta", map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason":   mapResponsesStopReason(status, state.HasToolUse, getNestedString(responseObj, "incomplete_details", "reason")),
			"stop_sequence": nil,
		},
		"usage": buildAnthropicUsageFromResponses(responseObj["usage"]),
	}))
	if !state.HasMessageStopSent {
		state.HasMessageStopSent = true
		events = append(events, newAnthropicSSEEvent("message_stop", map[string]interface{}{
			"type": "message_stop",
		}))
	}
	return events
}

func (state *claudeResponsesStreamState) closeAllOpenBlocks() []anthropicSSEEvent {
	events := make([]anthropicSSEEvent, 0, len(state.OpenIndices))
	indices := make([]int, 0, len(state.OpenIndices))
	for index := range state.OpenIndices {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	for _, index := range indices {
		if arguments := state.ToolArgsByIndex[index]; arguments != "" && isClaudeReadToolName(state.toolNameForIndex(index)) {
			events = append(events, newAnthropicSSEEvent("content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": index,
				"delta": map[string]interface{}{
					"type":         "input_json_delta",
					"partial_json": sanitizeAnthropicToolUseArguments(state.toolNameForIndex(index), arguments),
				},
			}))
		}
		delete(state.OpenIndices, index)
		delete(state.ToolArgsByIndex, index)
		delete(state.ToolNameByIndex, index)
		events = append(events, newAnthropicSSEEvent("content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": index,
		}))
	}
	state.HasCurrentTextIndex = false
	state.HasFallbackOpenIndex = false
	return events
}

func responseStatusFromEvent(data map[string]interface{}, responseObj map[string]interface{}) string {
	if status := getString(responseObj, "status"); status != "" {
		return status
	}
	switch getString(data, "type") {
	case "response.incomplete":
		return "incomplete"
	case "response.failed":
		return "failed"
	case "response.completed", "response.done":
		return "completed"
	default:
		return ""
	}
}

func (state *claudeResponsesStreamState) ensureMessageStart() []anthropicSSEEvent {
	if state.HasSentMessageStart {
		return nil
	}
	state.HasSentMessageStart = true
	return []anthropicSSEEvent{
		newAnthropicSSEEvent("message_start", map[string]interface{}{
			"type": "message_start",
			"message": map[string]interface{}{
				"id":    state.MessageID,
				"type":  "message",
				"role":  "assistant",
				"model": state.Model,
				"usage": map[string]interface{}{
					"input_tokens":  0,
					"output_tokens": 0,
				},
			},
		}),
	}
}

func (state *claudeResponsesStreamState) currentOrResolvedTextIndex(data map[string]interface{}) int {
	if state.HasCurrentTextIndex {
		return state.CurrentTextIndex
	}
	index := state.resolveContentIndex(data)
	state.CurrentTextIndex = index
	state.HasCurrentTextIndex = true
	return index
}

func (state *claudeResponsesStreamState) closeCurrentTextIndex(data map[string]interface{}) []anthropicSSEEvent {
	var index int
	var ok bool
	if state.HasCurrentTextIndex {
		index = state.CurrentTextIndex
		ok = true
		state.HasCurrentTextIndex = false
	} else if data != nil {
		index, ok = state.lookupContentIndex(data)
	}
	if !ok || !state.OpenIndices[index] {
		return nil
	}
	delete(state.OpenIndices, index)
	if state.HasFallbackOpenIndex && state.FallbackOpenIndex == index {
		state.HasFallbackOpenIndex = false
	}
	return []anthropicSSEEvent{
		newAnthropicSSEEvent("content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": index,
		}),
	}
}

func (state *claudeResponsesStreamState) resolveContentIndex(data map[string]interface{}) int {
	if key, ok := contentPartKey(data); ok {
		if existing, exists := state.IndexByKey[key]; exists {
			return existing
		}
		index := state.NextContentIndex
		state.NextContentIndex++
		state.IndexByKey[key] = index
		return index
	}
	if state.HasFallbackOpenIndex {
		return state.FallbackOpenIndex
	}
	index := state.NextContentIndex
	state.NextContentIndex++
	state.FallbackOpenIndex = index
	state.HasFallbackOpenIndex = true
	return index
}

func (state *claudeResponsesStreamState) lookupContentIndex(data map[string]interface{}) (int, bool) {
	if key, ok := contentPartKey(data); ok {
		index, exists := state.IndexByKey[key]
		return index, exists
	}
	if state.HasFallbackOpenIndex {
		return state.FallbackOpenIndex, true
	}
	return 0, false
}

func (state *claudeResponsesStreamState) resolveToolIndexFromAdded(data map[string]interface{}, item map[string]interface{}) int {
	if key, ok := toolItemKeyFromAdded(data, item); ok {
		if existing, exists := state.IndexByKey[key]; exists {
			state.HasLastToolIndex = true
			state.LastToolIndex = existing
			if itemID := getString(item, "id"); itemID != "" {
				state.ToolIndexByItemID[itemID] = existing
			}
			return existing
		}
		index := state.NextContentIndex
		state.NextContentIndex++
		state.IndexByKey[key] = index
		state.HasLastToolIndex = true
		state.LastToolIndex = index
		if itemID := getString(item, "id"); itemID != "" {
			state.ToolIndexByItemID[itemID] = index
		}
		return index
	}
	index := state.NextContentIndex
	state.NextContentIndex++
	state.HasLastToolIndex = true
	state.LastToolIndex = index
	return index
}

func (state *claudeResponsesStreamState) lookupToolIndexFromEvent(data map[string]interface{}) (int, bool) {
	if itemID := toolItemIDFromEvent(data); itemID != "" {
		if index, exists := state.ToolIndexByItemID[itemID]; exists {
			return index, true
		}
		if index, exists := state.IndexByKey["tool:"+itemID]; exists {
			return index, true
		}
	}
	if key, ok := toolItemKeyFromEvent(data); ok {
		if index, exists := state.IndexByKey[key]; exists {
			return index, true
		}
	}
	if state.HasLastToolIndex {
		return state.LastToolIndex, true
	}
	return 0, false
}

func (state *claudeResponsesStreamState) resolveToolIndexFromEvent(data map[string]interface{}) int {
	if index, ok := state.lookupToolIndexFromEvent(data); ok {
		return index
	}
	index := state.NextContentIndex
	state.NextContentIndex++
	state.HasLastToolIndex = true
	state.LastToolIndex = index
	if itemID := toolItemIDFromEvent(data); itemID != "" {
		state.ToolIndexByItemID[itemID] = index
		state.IndexByKey["tool:"+itemID] = index
	}
	return index
}

func responsesFunctionCallDoneArguments(data map[string]interface{}) string {
	if arguments := getString(data, "arguments"); arguments != "" {
		return arguments
	}
	if item, ok := data["item"].(map[string]interface{}); ok {
		if arguments := getString(item, "arguments"); arguments != "" {
			return arguments
		}
	}
	return ""
}

func responseObjectFromEvent(data map[string]interface{}) map[string]interface{} {
	if response, ok := data["response"].(map[string]interface{}); ok {
		return response
	}
	return data
}

func contentPartKey(data map[string]interface{}) (string, bool) {
	if itemID := getString(data, "item_id"); itemID != "" {
		if contentIndex := numberFromValue(data["content_index"]); contentIndex >= 0 {
			return fmt.Sprintf("part:%s:%d", itemID, contentIndex), true
		}
	}
	if outputIndex := numberFromValue(data["output_index"]); outputIndex > 0 || data["output_index"] != nil {
		if contentIndex := numberFromValue(data["content_index"]); contentIndex > 0 || data["content_index"] != nil {
			return fmt.Sprintf("part:out:%d:%d", outputIndex, contentIndex), true
		}
	}
	return "", false
}

func toolItemKeyFromAdded(data map[string]interface{}, item map[string]interface{}) (string, bool) {
	if itemID := getString(item, "id"); itemID != "" {
		return "tool:" + itemID, true
	}
	if itemID := getString(data, "item_id"); itemID != "" {
		return "tool:" + itemID, true
	}
	if outputIndex := numberFromValue(data["output_index"]); outputIndex > 0 || data["output_index"] != nil {
		return fmt.Sprintf("tool:out:%d", outputIndex), true
	}
	return "", false
}

func toolItemKeyFromEvent(data map[string]interface{}) (string, bool) {
	if itemID := toolItemIDFromEvent(data); itemID != "" {
		return "tool:" + itemID, true
	}
	if outputIndex := numberFromValue(data["output_index"]); outputIndex > 0 || data["output_index"] != nil {
		return fmt.Sprintf("tool:out:%d", outputIndex), true
	}
	return "", false
}

func toolItemIDFromEvent(data map[string]interface{}) string {
	if itemID := getString(data, "item_id"); itemID != "" {
		return itemID
	}
	if item, ok := data["item"].(map[string]interface{}); ok {
		if itemID := getString(item, "id"); itemID != "" {
			return itemID
		}
	}
	return ""
}

func functionCallFinishKeys(data map[string]interface{}) []string {
	keys := make([]string, 0, 2)
	if itemID := toolItemIDFromEvent(data); itemID != "" {
		keys = append(keys, "id:"+itemID)
	}
	if outputIndex := numberFromValue(data["output_index"]); outputIndex > 0 || data["output_index"] != nil {
		keys = append(keys, fmt.Sprintf("out:%d", outputIndex))
	}
	return keys
}
