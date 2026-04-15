package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func transformClaudeRequestForAPIFormat(bodyBytes []byte, provider Provider) ([]byte, error) {
	apiFormat := resolveClaudeAPIFormat(provider)
	if !claudeAPIFormatNeedsTransform(apiFormat) {
		return bodyBytes, nil
	}

	body, err := decodeJSONMap(bodyBytes)
	if err != nil {
		return nil, err
	}

	var transformed map[string]interface{}
	switch apiFormat {
	case claudeAPIFormatOpenAIChat:
		transformed, err = anthropicToOpenAIRequest(body)
	case claudeAPIFormatOpenAIResponse:
		transformed, err = anthropicToResponsesRequest(body, providerRefFromProvider(provider))
	default:
		return bodyBytes, nil
	}
	if err != nil {
		return nil, err
	}

	return encodeJSONMap(transformed)
}

func transformClaudeResponseForAPIFormat(bodyBytes []byte, apiFormat string) ([]byte, error) {
	if !claudeAPIFormatNeedsTransform(apiFormat) {
		return bodyBytes, nil
	}

	body, err := decodeJSONMap(bodyBytes)
	if err != nil {
		return nil, err
	}

	var transformed map[string]interface{}
	switch normalizeClaudeAPIFormat(apiFormat) {
	case claudeAPIFormatOpenAIChat:
		transformed, err = openAIToAnthropicResponse(body)
	case claudeAPIFormatOpenAIResponse:
		transformed, err = responsesToAnthropicResponse(body)
	default:
		return bodyBytes, nil
	}
	if err != nil {
		return nil, err
	}

	return encodeJSONMap(transformed)
}

func decodeJSONMap(bodyBytes []byte) (map[string]interface{}, error) {
	trimmed := bytes.TrimSpace(bodyBytes)
	if len(trimmed) == 0 {
		return map[string]interface{}{}, nil
	}

	var value map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}
	if value == nil {
		return map[string]interface{}{}, nil
	}
	return value, nil
}

func encodeJSONMap(value map[string]interface{}) ([]byte, error) {
	if value == nil {
		value = map[string]interface{}{}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("序列化 JSON 失败: %w", err)
	}
	return data, nil
}

func anthropicToOpenAIRequest(body map[string]interface{}) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	if model := getString(body, "model"); model != "" {
		result["model"] = model
	}

	messages := make([]interface{}, 0)
	switch system := body["system"].(type) {
	case string:
		if strings.TrimSpace(system) != "" {
			messages = append(messages, map[string]interface{}{
				"role":    "system",
				"content": system,
			})
		}
	case []interface{}:
		for _, entry := range system {
			msgMap, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			text := getString(msgMap, "text")
			if text == "" {
				continue
			}
			sysMsg := map[string]interface{}{
				"role":    "system",
				"content": text,
			}
			if cacheControl, ok := msgMap["cache_control"]; ok {
				sysMsg["cache_control"] = cacheControl
			}
			messages = append(messages, sysMsg)
		}
	}

	if msgArray, ok := body["messages"].([]interface{}); ok {
		for _, msg := range msgArray {
			msgMap, ok := msg.(map[string]interface{})
			if !ok {
				continue
			}
			role := getString(msgMap, "role")
			if role == "" {
				role = "user"
			}
			converted, err := convertAnthropicMessageToOpenAI(role, msgMap["content"])
			if err != nil {
				return nil, err
			}
			messages = append(messages, converted...)
		}
	}

	normalizeOpenAISystemMessages(&messages)
	result["messages"] = messages

	model := getString(body, "model")
	if maxTokens, ok := body["max_tokens"]; ok {
		if isOpenAIOSeries(model) {
			result["max_completion_tokens"] = maxTokens
		} else {
			result["max_tokens"] = maxTokens
		}
	}
	copyJSONField(body, result, "temperature")
	copyJSONField(body, result, "top_p")
	copyJSONField(body, result, "stream")
	if stopSequences, ok := body["stop_sequences"]; ok {
		result["stop"] = stopSequences
	}

	if supportsReasoningEffort(model) {
		if effort := resolveReasoningEffort(body); effort != "" {
			result["reasoning_effort"] = effort
		}
	}

	if tools, ok := body["tools"].([]interface{}); ok {
		openAITools := make([]interface{}, 0, len(tools))
		for _, entry := range tools {
			tool, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			if getString(tool, "type") == "BatchTool" {
				continue
			}
			openAITool := map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        getString(tool, "name"),
					"description": tool["description"],
					"parameters":  cleanJSONSchema(tool["input_schema"]),
				},
			}
			if cacheControl, ok := tool["cache_control"]; ok {
				openAITool["cache_control"] = cacheControl
			}
			openAITools = append(openAITools, openAITool)
		}
		if len(openAITools) > 0 {
			result["tools"] = openAITools
		}
	}

	if toolChoice, ok := body["tool_choice"]; ok {
		result["tool_choice"] = toolChoice
	}

	return result, nil
}

func anthropicToResponsesRequest(body map[string]interface{}, cacheKey string) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	if model := getString(body, "model"); model != "" {
		result["model"] = model
	}

	if system, ok := body["system"]; ok {
		instructions := extractAnthropicSystemInstructions(system)
		if instructions != "" {
			result["instructions"] = instructions
		}
	}

	if msgArray, ok := body["messages"].([]interface{}); ok {
		input, err := convertAnthropicMessagesToResponsesInput(msgArray)
		if err != nil {
			return nil, err
		}
		result["input"] = input
	}

	if maxTokens, ok := body["max_tokens"]; ok {
		result["max_output_tokens"] = maxTokens
	}
	copyJSONField(body, result, "temperature")
	copyJSONField(body, result, "top_p")
	copyJSONField(body, result, "stream")

	if model := getString(body, "model"); supportsReasoningEffort(model) {
		if effort := resolveReasoningEffort(body); effort != "" {
			result["reasoning"] = map[string]interface{}{
				"effort": effort,
			}
		}
	}

	if tools, ok := body["tools"].([]interface{}); ok {
		responseTools := make([]interface{}, 0, len(tools))
		for _, entry := range tools {
			tool, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			if getString(tool, "type") == "BatchTool" {
				continue
			}
			responseTools = append(responseTools, map[string]interface{}{
				"type":        "function",
				"name":        getString(tool, "name"),
				"description": tool["description"],
				"parameters":  cleanJSONSchema(tool["input_schema"]),
			})
		}
		if len(responseTools) > 0 {
			result["tools"] = responseTools
		}
	}

	if toolChoice, ok := body["tool_choice"]; ok {
		result["tool_choice"] = mapToolChoiceToResponses(toolChoice)
	}

	if strings.TrimSpace(cacheKey) != "" {
		result["prompt_cache_key"] = cacheKey
	}

	return result, nil
}

func openAIToAnthropicResponse(body map[string]interface{}) (map[string]interface{}, error) {
	choices, ok := body["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("OpenAI 响应缺少 choices")
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("OpenAI 响应 choice 结构无效")
	}

	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("OpenAI 响应缺少 message")
	}

	content := make([]interface{}, 0)
	hasToolUse := false

	switch msgContent := message["content"].(type) {
	case string:
		if strings.TrimSpace(msgContent) != "" {
			content = append(content, map[string]interface{}{
				"type": "text",
				"text": msgContent,
			})
		}
	case []interface{}:
		for _, entry := range msgContent {
			part, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			switch getString(part, "type") {
			case "text", "output_text":
				if text := getString(part, "text"); text != "" {
					content = append(content, map[string]interface{}{
						"type": "text",
						"text": text,
					})
				}
			case "refusal":
				if refusal := getString(part, "refusal"); refusal != "" {
					content = append(content, map[string]interface{}{
						"type": "text",
						"text": refusal,
					})
				}
			}
		}
	}

	if refusal := getString(message, "refusal"); refusal != "" {
		content = append(content, map[string]interface{}{
			"type": "text",
			"text": refusal,
		})
	}

	if toolCalls, ok := message["tool_calls"].([]interface{}); ok {
		if len(toolCalls) > 0 {
			hasToolUse = true
		}
		for _, entry := range toolCalls {
			call, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			function, _ := call["function"].(map[string]interface{})
			input := parseJSONStringValue(getString(function, "arguments"))
			content = append(content, map[string]interface{}{
				"type":  "tool_use",
				"id":    getString(call, "id"),
				"name":  getString(function, "name"),
				"input": input,
			})
		}
	}

	if !hasToolUse {
		if functionCall, ok := message["function_call"].(map[string]interface{}); ok {
			hasArguments := functionCall["arguments"] != nil
			name := getString(functionCall, "name")
			if name != "" || hasArguments {
				input := parseFlexibleJSONValue(functionCall["arguments"])
				content = append(content, map[string]interface{}{
					"type":  "tool_use",
					"id":    getString(functionCall, "id"),
					"name":  name,
					"input": input,
				})
				hasToolUse = true
			}
		}
	}

	stopReason := mapOpenAIFinishReasonToAnthropic(getString(choice, "finish_reason"), hasToolUse)
	usage := buildAnthropicUsageFromOpenAI(body["usage"])

	return map[string]interface{}{
		"id":            getString(body, "id"),
		"type":          "message",
		"role":          "assistant",
		"content":       content,
		"model":         getString(body, "model"),
		"stop_reason":   stopReason,
		"stop_sequence": nil,
		"usage":         usage,
	}, nil
}

func responsesToAnthropicResponse(body map[string]interface{}) (map[string]interface{}, error) {
	output, ok := body["output"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("Responses 响应缺少 output")
	}

	content := make([]interface{}, 0)
	hasToolUse := false

	for _, entry := range output {
		item, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		switch getString(item, "type") {
		case "message":
			if msgContent, ok := item["content"].([]interface{}); ok {
				for _, blockEntry := range msgContent {
					block, ok := blockEntry.(map[string]interface{})
					if !ok {
						continue
					}
					switch getString(block, "type") {
					case "output_text":
						if text := getString(block, "text"); text != "" {
							content = append(content, map[string]interface{}{
								"type": "text",
								"text": text,
							})
						}
					case "refusal":
						if refusal := getString(block, "refusal"); refusal != "" {
							content = append(content, map[string]interface{}{
								"type": "text",
								"text": refusal,
							})
						}
					}
				}
			}
		case "function_call":
			input := parseJSONStringValue(getString(item, "arguments"))
			content = append(content, map[string]interface{}{
				"type":  "tool_use",
				"id":    getString(item, "call_id"),
				"name":  getString(item, "name"),
				"input": input,
			})
			hasToolUse = true
		case "reasoning":
			if summary, ok := item["summary"].([]interface{}); ok {
				parts := make([]string, 0, len(summary))
				for _, summaryEntry := range summary {
					summaryMap, ok := summaryEntry.(map[string]interface{})
					if !ok || getString(summaryMap, "type") != "summary_text" {
						continue
					}
					if text := getString(summaryMap, "text"); text != "" {
						parts = append(parts, text)
					}
				}
				thinkingText := strings.Join(parts, "")
				if thinkingText != "" {
					content = append(content, map[string]interface{}{
						"type":     "thinking",
						"thinking": thinkingText,
					})
				}
			}
		}
	}

	stopReason := mapResponsesStopReason(getString(body, "status"), hasToolUse, getNestedString(body, "incomplete_details", "reason"))
	usage := buildAnthropicUsageFromResponses(body["usage"])

	return map[string]interface{}{
		"id":            getString(body, "id"),
		"type":          "message",
		"role":          "assistant",
		"content":       content,
		"model":         getString(body, "model"),
		"stop_reason":   stopReason,
		"stop_sequence": nil,
		"usage":         usage,
	}, nil
}

func convertAnthropicMessageToOpenAI(role string, content interface{}) ([]interface{}, error) {
	result := make([]interface{}, 0)

	switch value := content.(type) {
	case nil:
		result = append(result, map[string]interface{}{
			"role":    role,
			"content": nil,
		})
		return result, nil
	case string:
		result = append(result, map[string]interface{}{
			"role":    role,
			"content": value,
		})
		return result, nil
	case []interface{}:
		contentParts := make([]interface{}, 0)
		toolCalls := make([]interface{}, 0)

		for _, entry := range value {
			block, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			switch getString(block, "type") {
			case "text":
				text := getString(block, "text")
				if text == "" {
					continue
				}
				part := map[string]interface{}{
					"type": "text",
					"text": text,
				}
				if cacheControl, ok := block["cache_control"]; ok {
					part["cache_control"] = cacheControl
				}
				contentParts = append(contentParts, part)
			case "image":
				source, _ := block["source"].(map[string]interface{})
				mediaType := getString(source, "media_type")
				if mediaType == "" {
					mediaType = "image/png"
				}
				data := getString(source, "data")
				contentParts = append(contentParts, map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": fmt.Sprintf("data:%s;base64,%s", mediaType, data),
					},
				})
			case "tool_use":
				inputJSON, err := json.Marshal(block["input"])
				if err != nil {
					inputJSON = []byte("{}")
				}
				toolCalls = append(toolCalls, map[string]interface{}{
					"id":   getString(block, "id"),
					"type": "function",
					"function": map[string]interface{}{
						"name":      getString(block, "name"),
						"arguments": string(inputJSON),
					},
				})
			case "tool_result":
				toolCallID := getString(block, "tool_use_id")
				contentString, err := stringifyJSONContent(block["content"])
				if err != nil {
					return nil, err
				}
				result = append(result, map[string]interface{}{
					"role":         "tool",
					"tool_call_id": toolCallID,
					"content":      contentString,
				})
			case "thinking":
				continue
			}
		}

		if len(contentParts) > 0 || len(toolCalls) > 0 {
			msg := map[string]interface{}{
				"role": role,
			}
			switch {
			case len(contentParts) == 0:
				msg["content"] = nil
			case len(contentParts) == 1:
				part, _ := contentParts[0].(map[string]interface{})
				if _, hasCacheControl := part["cache_control"]; !hasCacheControl {
					if text, ok := part["text"]; ok {
						msg["content"] = text
					} else {
						msg["content"] = contentParts
					}
				} else {
					msg["content"] = contentParts
				}
			default:
				msg["content"] = contentParts
			}
			if len(toolCalls) > 0 {
				msg["tool_calls"] = toolCalls
			}
			result = append(result, msg)
		}
		return result, nil
	default:
		result = append(result, map[string]interface{}{
			"role":    role,
			"content": content,
		})
		return result, nil
	}
}

func convertAnthropicMessagesToResponsesInput(messages []interface{}) ([]interface{}, error) {
	input := make([]interface{}, 0)

	for _, entry := range messages {
		msg, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		role := getString(msg, "role")
		if role == "" {
			role = "user"
		}

		switch content := msg["content"].(type) {
		case string:
			contentType := "input_text"
			if role == "assistant" {
				contentType = "output_text"
			}
			input = append(input, map[string]interface{}{
				"role": role,
				"content": []interface{}{
					map[string]interface{}{
						"type": contentType,
						"text": content,
					},
				},
			})
		case []interface{}:
			messageContent := make([]interface{}, 0)
			for _, blockEntry := range content {
				block, ok := blockEntry.(map[string]interface{})
				if !ok {
					continue
				}
				switch getString(block, "type") {
				case "text":
					text := getString(block, "text")
					if text == "" {
						continue
					}
					contentType := "input_text"
					if role == "assistant" {
						contentType = "output_text"
					}
					messageContent = append(messageContent, map[string]interface{}{
						"type": contentType,
						"text": text,
					})
				case "image":
					source, _ := block["source"].(map[string]interface{})
					mediaType := getString(source, "media_type")
					if mediaType == "" {
						mediaType = "image/png"
					}
					data := getString(source, "data")
					messageContent = append(messageContent, map[string]interface{}{
						"type":      "input_image",
						"image_url": fmt.Sprintf("data:%s;base64,%s", mediaType, data),
					})
				case "tool_use":
					if len(messageContent) > 0 {
						input = append(input, map[string]interface{}{
							"role":    role,
							"content": messageContent,
						})
						messageContent = make([]interface{}, 0)
					}
					arguments, err := json.Marshal(block["input"])
					if err != nil {
						arguments = []byte("{}")
					}
					input = append(input, map[string]interface{}{
						"type":      "function_call",
						"call_id":   getString(block, "id"),
						"name":      getString(block, "name"),
						"arguments": string(arguments),
					})
				case "tool_result":
					if len(messageContent) > 0 {
						input = append(input, map[string]interface{}{
							"role":    role,
							"content": messageContent,
						})
						messageContent = make([]interface{}, 0)
					}
					output, err := stringifyJSONContent(block["content"])
					if err != nil {
						return nil, err
					}
					input = append(input, map[string]interface{}{
						"type":    "function_call_output",
						"call_id": getString(block, "tool_use_id"),
						"output":  output,
					})
				case "thinking":
					continue
				}
			}
			if len(messageContent) > 0 {
				input = append(input, map[string]interface{}{
					"role":    role,
					"content": messageContent,
				})
			}
		default:
			input = append(input, map[string]interface{}{
				"role": role,
			})
		}
	}

	return input, nil
}

func extractAnthropicSystemInstructions(system interface{}) string {
	switch value := system.(type) {
	case string:
		return strings.TrimSpace(value)
	case []interface{}:
		parts := make([]string, 0, len(value))
		for _, entry := range value {
			msgMap, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			if text := getString(msgMap, "text"); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n\n")
	default:
		return ""
	}
}

func normalizeOpenAISystemMessages(messages *[]interface{}) {
	if messages == nil || len(*messages) == 0 {
		return
	}

	systemMessages := make([]map[string]interface{}, 0)
	others := make([]interface{}, 0, len(*messages))
	for _, entry := range *messages {
		msg, ok := entry.(map[string]interface{})
		if ok && getString(msg, "role") == "system" {
			systemMessages = append(systemMessages, msg)
			continue
		}
		others = append(others, entry)
	}

	if len(systemMessages) == 0 {
		return
	}
	if len(systemMessages) == 1 {
		*messages = append([]interface{}{systemMessages[0]}, others...)
		return
	}

	parts := make([]string, 0, len(systemMessages))
	var inheritedCacheControl interface{}
	cacheControlConflict := false
	sawCacheControl := false
	sawMissingCacheControl := false

	for _, msg := range systemMessages {
		switch content := msg["content"].(type) {
		case string:
			if content != "" {
				parts = append(parts, content)
			}
		case []interface{}:
			textParts := make([]string, 0, len(content))
			for _, entry := range content {
				part, ok := entry.(map[string]interface{})
				if !ok {
					continue
				}
				if text := getString(part, "text"); text != "" {
					textParts = append(textParts, text)
				}
			}
			if len(textParts) > 0 {
				parts = append(parts, strings.Join(textParts, "\n"))
			}
		}

		if cacheControl, ok := msg["cache_control"]; ok {
			sawCacheControl = true
			if inheritedCacheControl == nil {
				inheritedCacheControl = cacheControl
			} else if !jsonDeepEqual(inheritedCacheControl, cacheControl) {
				cacheControlConflict = true
			}
		} else {
			sawMissingCacheControl = true
		}
	}

	merged := map[string]interface{}{
		"role":    "system",
		"content": strings.Join(parts, "\n"),
	}
	if !cacheControlConflict && !(sawCacheControl && sawMissingCacheControl) && inheritedCacheControl != nil {
		merged["cache_control"] = inheritedCacheControl
	}
	*messages = append([]interface{}{merged}, others...)
}

func cleanJSONSchema(schema interface{}) interface{} {
	switch value := schema.(type) {
	case map[string]interface{}:
		cleaned := make(map[string]interface{}, len(value))
		for key, entry := range value {
			if key == "format" && getString(value, "format") == "uri" {
				continue
			}
			cleaned[key] = cleanJSONSchema(entry)
		}
		return cleaned
	case []interface{}:
		items := make([]interface{}, 0, len(value))
		for _, entry := range value {
			items = append(items, cleanJSONSchema(entry))
		}
		return items
	default:
		return value
	}
}

func mapToolChoiceToResponses(toolChoice interface{}) interface{} {
	switch value := toolChoice.(type) {
	case string:
		return value
	case map[string]interface{}:
		switch getString(value, "type") {
		case "any":
			return "required"
		case "auto":
			return "auto"
		case "none":
			return "none"
		case "tool":
			return map[string]interface{}{
				"type": "function",
				"name": getString(value, "name"),
			}
		default:
			return value
		}
	default:
		return toolChoice
	}
}

func buildAnthropicUsageFromOpenAI(raw interface{}) map[string]interface{} {
	usage, _ := raw.(map[string]interface{})
	if usage == nil {
		return map[string]interface{}{
			"input_tokens":  0,
			"output_tokens": 0,
		}
	}

	result := map[string]interface{}{
		"input_tokens":  numberFromValue(usage["prompt_tokens"]),
		"output_tokens": numberFromValue(usage["completion_tokens"]),
	}
	if promptDetails, ok := usage["prompt_tokens_details"].(map[string]interface{}); ok {
		if cached := numberFromValue(promptDetails["cached_tokens"]); cached > 0 {
			result["cache_read_input_tokens"] = cached
		}
	}
	if cached := numberFromValue(usage["cache_read_input_tokens"]); cached > 0 {
		result["cache_read_input_tokens"] = cached
	}
	if created := numberFromValue(usage["cache_creation_input_tokens"]); created > 0 {
		result["cache_creation_input_tokens"] = created
	}
	return result
}

func buildAnthropicUsageFromResponses(raw interface{}) map[string]interface{} {
	usage, _ := raw.(map[string]interface{})
	if usage == nil {
		return map[string]interface{}{
			"input_tokens":  0,
			"output_tokens": 0,
		}
	}

	result := map[string]interface{}{
		"input_tokens":  numberFromValue(usage["input_tokens"]),
		"output_tokens": numberFromValue(usage["output_tokens"]),
	}
	if details, ok := usage["input_tokens_details"].(map[string]interface{}); ok {
		if cached := numberFromValue(details["cached_tokens"]); cached > 0 {
			result["cache_read_input_tokens"] = cached
		}
	}
	if details, ok := usage["prompt_tokens_details"].(map[string]interface{}); ok {
		if cached := numberFromValue(details["cached_tokens"]); cached > 0 {
			if _, exists := result["cache_read_input_tokens"]; !exists {
				result["cache_read_input_tokens"] = cached
			}
		}
	}
	if cached := numberFromValue(usage["cache_read_input_tokens"]); cached > 0 {
		result["cache_read_input_tokens"] = cached
	}
	if created := numberFromValue(usage["cache_creation_input_tokens"]); created > 0 {
		result["cache_creation_input_tokens"] = created
	}
	return result
}

func mapOpenAIFinishReasonToAnthropic(finishReason string, hasToolUse bool) string {
	switch strings.TrimSpace(finishReason) {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls", "function_call":
		return "tool_use"
	case "content_filter":
		return "end_turn"
	case "":
		if hasToolUse {
			return "tool_use"
		}
		return ""
	default:
		if hasToolUse {
			return "tool_use"
		}
		return "end_turn"
	}
}

func mapResponsesStopReason(status string, hasToolUse bool, incompleteReason string) string {
	switch strings.TrimSpace(status) {
	case "completed":
		if hasToolUse {
			return "tool_use"
		}
		return "end_turn"
	case "incomplete":
		switch incompleteReason {
		case "max_output_tokens", "max_tokens", "":
			return "max_tokens"
		default:
			return "end_turn"
		}
	default:
		return "end_turn"
	}
}

func isOpenAIOSeries(model string) bool {
	model = strings.TrimSpace(strings.ToLower(model))
	return len(model) > 1 && strings.HasPrefix(model, "o") && model[1] >= '0' && model[1] <= '9'
}

func supportsReasoningEffort(model string) bool {
	model = strings.TrimSpace(strings.ToLower(model))
	if isOpenAIOSeries(model) {
		return true
	}
	rest, ok := strings.CutPrefix(model, "gpt-")
	return ok && len(rest) > 0 && rest[0] >= '5' && rest[0] <= '9'
}

func resolveReasoningEffort(body map[string]interface{}) string {
	if outputConfig, ok := body["output_config"].(map[string]interface{}); ok {
		switch getString(outputConfig, "effort") {
		case "low", "medium", "high":
			return getString(outputConfig, "effort")
		case "max":
			return "xhigh"
		}
	}

	thinking, ok := body["thinking"].(map[string]interface{})
	if !ok {
		return ""
	}
	switch getString(thinking, "type") {
	case "adaptive":
		return "xhigh"
	case "enabled":
		budget := numberFromValue(thinking["budget_tokens"])
		switch {
		case budget == 0:
			return "high"
		case budget < 4_000:
			return "low"
		case budget < 16_000:
			return "medium"
		default:
			return "high"
		}
	default:
		return ""
	}
}

func copyJSONField(source map[string]interface{}, target map[string]interface{}, key string) {
	if value, ok := source[key]; ok {
		target[key] = value
	}
}

func getString(source map[string]interface{}, key string) string {
	if source == nil {
		return ""
	}
	return asString(source[key])
}

func getNestedString(source map[string]interface{}, keys ...string) string {
	current := source
	for index, key := range keys {
		value, ok := current[key]
		if !ok {
			return ""
		}
		if index == len(keys)-1 {
			return asString(value)
		}
		next, ok := value.(map[string]interface{})
		if !ok {
			return ""
		}
		current = next
	}
	return ""
}

func asString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func numberFromValue(value interface{}) int64 {
	switch typed := value.(type) {
	case json.Number:
		if intValue, err := typed.Int64(); err == nil {
			return intValue
		}
		if floatValue, err := typed.Float64(); err == nil {
			return int64(floatValue)
		}
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case int:
		return int64(typed)
	case int64:
		return typed
	case int32:
		return int64(typed)
	case uint:
		return int64(typed)
	case uint64:
		return int64(typed)
	case uint32:
		return int64(typed)
	}
	return 0
}

func stringifyJSONContent(value interface{}) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return "", fmt.Errorf("序列化 JSON 内容失败: %w", err)
		}
		return string(data), nil
	}
}

func parseJSONStringValue(raw string) interface{} {
	if strings.TrimSpace(raw) == "" {
		return map[string]interface{}{}
	}
	var value interface{}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return map[string]interface{}{}
	}
	return value
}

func parseFlexibleJSONValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case string:
		return parseJSONStringValue(typed)
	case map[string]interface{}, []interface{}:
		return typed
	default:
		return map[string]interface{}{}
	}
}

func jsonDeepEqual(left interface{}, right interface{}) bool {
	leftBytes, leftErr := json.Marshal(left)
	rightBytes, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return bytes.Equal(leftBytes, rightBytes)
}

func sortStringKeys(source map[string]interface{}) []string {
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

