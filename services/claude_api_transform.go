package services

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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
		transformed, err = anthropicToResponsesRequest(body, "")
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
		if strings.TrimSpace(system) != "" && !isAnthropicBillingHeaderText(system) {
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
			if text == "" || isAnthropicBillingHeaderText(text) {
				continue
			}
			sysMsg := map[string]interface{}{
				"role":    "system",
				"content": text,
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
					"parameters":  normalizeOpenAIToolParameters(tool["input_schema"]),
				},
			}
			openAITools = append(openAITools, openAITool)
		}
		if len(openAITools) > 0 {
			result["tools"] = openAITools
			result["parallel_tool_calls"] = !toolChoiceDisablesParallelToolUse(body["tool_choice"])
		}
	}

	if toolChoice, ok := body["tool_choice"]; ok {
		result["tool_choice"] = mapToolChoiceToOpenAIChat(toolChoice)
	}

	return result, nil
}

func anthropicToResponsesRequest(body map[string]interface{}, cacheKey string) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	if model := getString(body, "model"); model != "" {
		result["model"] = model
	}

	var input []interface{}
	var err error
	if msgArray, ok := body["messages"].([]interface{}); ok {
		input, err = convertAnthropicToResponsesInput(body["system"], msgArray)
		if err != nil {
			return nil, err
		}
	} else {
		input, err = convertAnthropicToResponsesInput(body["system"], nil)
		if err != nil {
			return nil, err
		}
	}
	if len(input) > 0 {
		result["input"] = input
	}

	if maxTokens, ok := body["max_tokens"]; ok {
		result["max_output_tokens"] = maxTokens
	}
	copyJSONField(body, result, "temperature")
	copyJSONField(body, result, "top_p")
	copyJSONField(body, result, "stream")
	copyJSONField(body, result, "store")
	copyJSONField(body, result, "include")

	if model := getString(body, "model"); supportsReasoningEffort(model) {
		effort := resolveReasoningEffort(body)
		if effort != "" {
			result["reasoning"] = map[string]interface{}{
				"effort": effort,
			}
			applyResponsesTextVerbosity(result, body)
		}
		if shouldIncludeEncryptedReasoningContent(body) {
			appendResponsesInclude(result, "reasoning.encrypted_content")
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
				"parameters":  normalizeOpenAIToolParameters(tool["input_schema"]),
				"strict":      false,
			})
		}
		if len(responseTools) > 0 {
			result["tools"] = responseTools
			result["parallel_tool_calls"] = !toolChoiceDisablesParallelToolUse(body["tool_choice"])
		}
	}

	if toolChoice, ok := body["tool_choice"]; ok {
		result["tool_choice"] = mapToolChoiceToResponses(toolChoice)
	}

	promptCacheKey := strings.TrimSpace(cacheKey)
	if promptCacheKey == "" {
		promptCacheKey = strings.TrimSpace(getString(body, "prompt_cache_key"))
	}
	if promptCacheKey == "" {
		promptCacheKey = deriveClaudeOpenAICompatPromptCacheKey(body)
	}
	if promptCacheKey != "" {
		result["prompt_cache_key"] = promptCacheKey
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
			name := getString(function, "name")
			input := sanitizeAnthropicToolUseInput(name, parseJSONStringValue(getString(function, "arguments")))
			content = append(content, map[string]interface{}{
				"type":  "tool_use",
				"id":    getString(call, "id"),
				"name":  name,
				"input": input,
			})
		}
	}

	if !hasToolUse {
		if functionCall, ok := message["function_call"].(map[string]interface{}); ok {
			hasArguments := functionCall["arguments"] != nil
			name := getString(functionCall, "name")
			if name != "" || hasArguments {
				input := sanitizeAnthropicToolUseInput(name, parseFlexibleJSONValue(functionCall["arguments"]))
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
	status := strings.TrimSpace(getString(body, "status"))
	output, ok := body["output"].([]interface{})
	if isOpenAIResponsesTerminalFailureStatus(status) ||
		(strings.EqualFold(status, "incomplete") && !responsesOutputHasUsableContent(output)) {
		message := strings.TrimSpace(getNestedString(body, "error", "message"))
		if message == "" {
			message = strings.TrimSpace(getNestedString(body, "incomplete_details", "reason"))
		}
		if message == "" {
			message = status
		}
		return nil, fmt.Errorf("OpenAI Responses response %s: %s", status, message)
	}

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
			name := getString(item, "name")
			input := sanitizeAnthropicToolUseInput(name, parseJSONStringValue(getString(item, "arguments")))
			content = append(content, map[string]interface{}{
				"type":  "tool_use",
				"id":    getString(item, "call_id"),
				"name":  name,
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

func responsesOutputHasUsableContent(output []interface{}) bool {
	for _, entry := range output {
		item, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		switch getString(item, "type") {
		case "function_call":
			return true
		case "message":
			if msgContent, ok := item["content"].([]interface{}); ok {
				for _, blockEntry := range msgContent {
					block, ok := blockEntry.(map[string]interface{})
					if !ok {
						continue
					}
					switch getString(block, "type") {
					case "output_text":
						if strings.TrimSpace(getString(block, "text")) != "" {
							return true
						}
					case "refusal":
						if strings.TrimSpace(getString(block, "refusal")) != "" {
							return true
						}
					}
				}
			}
		case "reasoning":
			if summary, ok := item["summary"].([]interface{}); ok {
				for _, summaryEntry := range summary {
					summaryMap, ok := summaryEntry.(map[string]interface{})
					if ok && getString(summaryMap, "type") == "summary_text" && strings.TrimSpace(getString(summaryMap, "text")) != "" {
						return true
					}
				}
			}
		}
	}
	return false
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
				if text, ok := part["text"]; ok {
					msg["content"] = text
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

func convertAnthropicToResponsesInput(system interface{}, messages []interface{}) ([]interface{}, error) {
	input := make([]interface{}, 0)
	if systemContent := convertAnthropicSystemToResponsesContent(system); len(systemContent) > 0 {
		input = append(input, map[string]interface{}{
			"type":    "message",
			"role":    "developer",
			"content": systemContent,
		})
	}

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
				"type": "message",
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
							"type":    "message",
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
							"type":    "message",
							"role":    role,
							"content": messageContent,
						})
						messageContent = make([]interface{}, 0)
					}
					output, imageParts := convertAnthropicToolResultOutput(block["content"])
					input = append(input, map[string]interface{}{
						"type":    "function_call_output",
						"call_id": getString(block, "tool_use_id"),
						"output":  output,
					})
					if len(imageParts) > 0 {
						input = append(input, map[string]interface{}{
							"type":    "message",
							"role":    "user",
							"content": imageParts,
						})
					}
				case "thinking":
					continue
				}
			}
			if len(messageContent) > 0 {
				input = append(input, map[string]interface{}{
					"type":    "message",
					"role":    role,
					"content": messageContent,
				})
			}
		default:
			input = append(input, map[string]interface{}{
				"type": "message",
				"role": role,
			})
		}
	}

	return input, nil
}

func convertAnthropicMessagesToResponsesInput(messages []interface{}) ([]interface{}, error) {
	return convertAnthropicToResponsesInput(nil, messages)
}

func convertAnthropicSystemToResponsesContent(system interface{}) []interface{} {
	switch value := system.(type) {
	case string:
		text := strings.TrimSpace(value)
		if text == "" || isAnthropicBillingHeaderText(text) {
			return nil
		}
		return []interface{}{
			map[string]interface{}{
				"type": "input_text",
				"text": text,
			},
		}
	case []interface{}:
		content := make([]interface{}, 0, len(value))
		for _, entry := range value {
			block, ok := entry.(map[string]interface{})
			if !ok || getString(block, "type") != "text" {
				continue
			}
			text := strings.TrimSpace(getString(block, "text"))
			if text == "" || isAnthropicBillingHeaderText(text) {
				continue
			}
			part := map[string]interface{}{
				"type": "input_text",
				"text": text,
			}
			content = append(content, part)
		}
		return content
	default:
		return nil
	}
}

func isAnthropicBillingHeaderText(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "x-anthropic-billing-header: ")
}

func convertAnthropicToolResultOutput(content interface{}) (string, []interface{}) {
	if content == nil {
		return "(empty)", nil
	}
	switch value := content.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return "(empty)", nil
		}
		return value, nil
	case []interface{}:
		textParts := make([]string, 0, len(value))
		imageParts := make([]interface{}, 0)
		for _, entry := range value {
			block, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			switch getString(block, "type") {
			case "text":
				if text := getString(block, "text"); text != "" {
					textParts = append(textParts, text)
				}
			case "image":
				source, _ := block["source"].(map[string]interface{})
				if imageURL := anthropicImageSourceToDataURI(source); imageURL != "" {
					imageParts = append(imageParts, map[string]interface{}{
						"type":      "input_image",
						"image_url": imageURL,
					})
				}
			}
		}
		output := strings.Join(textParts, "\n\n")
		if strings.TrimSpace(output) == "" {
			output = "(empty)"
		}
		return output, imageParts
	default:
		text, err := stringifyJSONContent(value)
		if err != nil || strings.TrimSpace(text) == "" {
			return "(empty)", nil
		}
		return text, nil
	}
}

func anthropicImageSourceToDataURI(source map[string]interface{}) string {
	if source == nil {
		return ""
	}
	data := strings.TrimSpace(getString(source, "data"))
	if data == "" {
		return ""
	}
	mediaType := strings.TrimSpace(getString(source, "media_type"))
	if mediaType == "" {
		mediaType = "image/png"
	}
	return fmt.Sprintf("data:%s;base64,%s", mediaType, data)
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
	}

	merged := map[string]interface{}{
		"role":    "system",
		"content": strings.Join(parts, "\n"),
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

func normalizeOpenAIToolParameters(schema interface{}) interface{} {
	cleaned := cleanJSONSchema(schema)
	objectSchema, ok := cleaned.(map[string]interface{})
	if !ok {
		return map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}
	if strings.TrimSpace(getString(objectSchema, "type")) == "" {
		objectSchema["type"] = "object"
	}
	if strings.EqualFold(getString(objectSchema, "type"), "object") {
		if _, ok := objectSchema["properties"]; !ok {
			objectSchema["properties"] = map[string]interface{}{}
		}
	}
	return objectSchema
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

func toolChoiceDisablesParallelToolUse(toolChoice interface{}) bool {
	choice, ok := toolChoice.(map[string]interface{})
	if !ok {
		return false
	}
	if disabled, ok := choice["disable_parallel_tool_use"].(bool); ok {
		return disabled
	}
	return false
}

func applyResponsesTextVerbosity(result map[string]interface{}, body map[string]interface{}) {
	textConfig, _ := cloneJSONMap(body["text"])
	if textConfig == nil {
		textConfig = map[string]interface{}{}
	}
	if strings.TrimSpace(asString(textConfig["verbosity"])) == "" {
		textConfig["verbosity"] = "medium"
	}
	result["text"] = textConfig
}

func shouldIncludeEncryptedReasoningContent(body map[string]interface{}) bool {
	if body == nil {
		return false
	}
	if value, exists := body["store"]; exists {
		if disabled, ok := value.(bool); ok && !disabled {
			return true
		}
	}
	for _, key := range []string{
		"openai_compat_include_encrypted_reasoning",
		"include_encrypted_reasoning",
	} {
		if enabled, ok := body[key].(bool); ok && enabled {
			return true
		}
	}
	return false
}

func appendResponsesInclude(result map[string]interface{}, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	existing := make([]interface{}, 0)
	seen := make(map[string]bool)
	if includeValues, ok := result["include"].([]interface{}); ok {
		for _, entry := range includeValues {
			text := strings.TrimSpace(asString(entry))
			if text == "" || seen[text] {
				continue
			}
			seen[text] = true
			existing = append(existing, text)
		}
	}
	if !seen[value] {
		existing = append(existing, value)
	}
	if len(existing) > 0 {
		result["include"] = existing
	}
}

func cloneJSONMap(value interface{}) (map[string]interface{}, bool) {
	source, ok := value.(map[string]interface{})
	if !ok {
		return nil, false
	}
	cloned := make(map[string]interface{}, len(source))
	for key, entry := range source {
		cloned[key] = entry
	}
	return cloned, true
}

func mapToolChoiceToOpenAIChat(toolChoice interface{}) interface{} {
	switch value := toolChoice.(type) {
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
				"function": map[string]interface{}{
					"name": getString(value, "name"),
				},
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

	inputTokens := numberFromValue(usage["prompt_tokens"])
	cacheReadTokens := int64(0)
	result := map[string]interface{}{
		"input_tokens":  inputTokens,
		"output_tokens": numberFromValue(usage["completion_tokens"]),
	}
	if promptDetails, ok := usage["prompt_tokens_details"].(map[string]interface{}); ok {
		if cached := numberFromValue(promptDetails["cached_tokens"]); cached > 0 {
			cacheReadTokens = cached
		}
	}
	if cached := numberFromValue(usage["cache_read_input_tokens"]); cached > 0 {
		cacheReadTokens = cached
	}
	if cacheReadTokens > 0 {
		result["cache_read_input_tokens"] = cacheReadTokens
		if inputTokens > 0 {
			result["input_tokens"] = max(0, inputTokens-cacheReadTokens)
		}
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

	inputTokens := numberFromValue(usage["input_tokens"])
	cacheReadTokens := int64(0)
	result := map[string]interface{}{
		"input_tokens":  inputTokens,
		"output_tokens": numberFromValue(usage["output_tokens"]),
	}
	if details, ok := usage["input_tokens_details"].(map[string]interface{}); ok {
		if cached := numberFromValue(details["cached_tokens"]); cached > 0 {
			cacheReadTokens = cached
		}
	}
	if details, ok := usage["prompt_tokens_details"].(map[string]interface{}); ok {
		if cached := numberFromValue(details["cached_tokens"]); cached > 0 {
			if cacheReadTokens == 0 {
				cacheReadTokens = cached
			}
		}
	}
	if cached := numberFromValue(usage["cache_read_input_tokens"]); cached > 0 {
		cacheReadTokens = cached
	}
	if cacheReadTokens > 0 {
		result["cache_read_input_tokens"] = cacheReadTokens
		if inputTokens > 0 {
			result["input_tokens"] = max(0, inputTokens-cacheReadTokens)
		}
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
	model = normalizeOpenAICompatFeatureModel(model)
	return len(model) > 1 && strings.HasPrefix(model, "o") && model[1] >= '0' && model[1] <= '9'
}

func supportsReasoningEffort(model string) bool {
	model = normalizeOpenAICompatFeatureModel(model)
	if isOpenAIOSeries(model) {
		return true
	}
	if strings.Contains(model, "codex") {
		return true
	}
	rest, ok := strings.CutPrefix(model, "gpt-")
	return ok && len(rest) > 0 && rest[0] >= '5' && rest[0] <= '9'
}

func supportsOpenAICompatPromptCache(model string) bool {
	model = normalizeOpenAICompatFeatureModel(model)
	return strings.HasPrefix(model, "gpt-5") || strings.Contains(model, "codex")
}

func normalizeOpenAICompatFeatureModel(model string) string {
	model = strings.TrimSpace(strings.ToLower(model))
	if slash := strings.LastIndex(model, "/"); slash >= 0 && slash < len(model)-1 {
		model = model[slash+1:]
	}
	model = strings.ReplaceAll(model, "_", "-")
	if strings.HasPrefix(model, "gpt5") {
		model = "gpt-5" + strings.TrimPrefix(model, "gpt5")
	}
	return model
}

func deriveClaudeOpenAICompatPromptCacheKey(body map[string]interface{}) string {
	if body == nil || !supportsOpenAICompatPromptCache(getString(body, "model")) {
		return ""
	}
	if resolveClaudeOpenAICompatPromptCacheKeyStrategy(body) != "shared" {
		if key := deriveClaudeMetadataPromptCacheKey(body["metadata"]); key != "" {
			return key
		}
	}
	if key := deriveClaudeCacheControlPromptCacheKey(body); key != "" {
		return key
	}

	seedParts := []string{"model=" + strings.TrimSpace(getString(body, "model"))}
	if effort := resolveReasoningEffort(body); effort != "" {
		seedParts = append(seedParts, "effort="+effort)
	}
	if toolChoice, ok := body["tool_choice"]; ok {
		seedParts = append(seedParts, "tool_choice="+canonicalJSONForSeed(toolChoice))
	}
	if tools, ok := body["tools"]; ok {
		seedParts = append(seedParts, "tools="+canonicalJSONForSeed(tools))
	}
	if system, ok := body["system"]; ok {
		seedParts = append(seedParts, "system="+canonicalJSONForSeed(system))
	}
	if firstUser := firstClaudeUserMessageContent(body["messages"]); firstUser != nil {
		seedParts = append(seedParts, "first_user="+canonicalJSONForSeed(firstUser))
	}
	return "compat_cc_" + shortSHA256Hex(strings.Join(seedParts, "|"))
}

func resolveClaudeOpenAICompatPromptCacheKeyStrategy(body map[string]interface{}) string {
	for _, key := range []string{
		"openai_compat_prompt_cache_key_strategy",
		"prompt_cache_key_strategy",
	} {
		switch strings.ToLower(strings.TrimSpace(asString(body[key]))) {
		case "shared", "content", "prompt", "digest":
			return "shared"
		case "metadata", "session", "isolated", "metadata_session":
			return "metadata"
		}
	}
	return "metadata"
}

func deriveClaudeMetadataPromptCacheKey(metadata interface{}) string {
	meta, ok := metadata.(map[string]interface{})
	if !ok {
		return ""
	}
	userID := strings.TrimSpace(asString(meta["user_id"]))
	if userID == "" {
		return ""
	}
	if parsed := parseClaudeMetadataUserID(userID); parsed != nil {
		seed := strings.Join([]string{
			strings.TrimSpace(parsed.DeviceID),
			strings.TrimSpace(parsed.AccountUUID),
			strings.TrimSpace(parsed.SessionID),
		}, "|")
		return "anthropic-metadata-" + shortSHA256Hex(seed)
	}
	return "anthropic-metadata-" + shortSHA256Hex(userID)
}

func deriveClaudeCacheControlPromptCacheKey(body map[string]interface{}) string {
	parts := make([]string, 0)
	collect := func(prefix string, content interface{}) {
		for _, text := range cacheControlledTextBlocks(content) {
			parts = append(parts, prefix+":"+text)
		}
	}
	collect("system", body["system"])
	if messages, ok := body["messages"].([]interface{}); ok {
		seenFirstUser := false
		for _, entry := range messages {
			msg, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			role := strings.TrimSpace(getString(msg, "role"))
			if role == "" {
				role = "user"
			}
			switch role {
			case "assistant":
				collect(role, msg["content"])
			case "user":
				if !seenFirstUser {
					collect(role, msg["content"])
					seenFirstUser = true
				}
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "anthropic-cache-" + shortSHA256Hex(strings.Join(parts, "\n"))
}

func cacheControlledTextBlocks(content interface{}) []string {
	blocks, ok := content.([]interface{})
	if !ok {
		return nil
	}
	texts := make([]string, 0, len(blocks))
	for _, entry := range blocks {
		block, ok := entry.(map[string]interface{})
		if !ok || getString(block, "type") != "text" {
			continue
		}
		cacheControl, ok := block["cache_control"].(map[string]interface{})
		if !ok || strings.TrimSpace(getString(cacheControl, "type")) != "ephemeral" {
			continue
		}
		if text := strings.TrimSpace(getString(block, "text")); text != "" {
			texts = append(texts, text)
		}
	}
	return texts
}

func firstClaudeUserMessageContent(messages interface{}) interface{} {
	msgArray, ok := messages.([]interface{})
	if !ok {
		return nil
	}
	for _, entry := range msgArray {
		msg, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		role := strings.TrimSpace(getString(msg, "role"))
		if role == "" || role == "user" {
			return msg["content"]
		}
	}
	return nil
}

func canonicalJSONForSeed(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}

func shortSHA256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:16])
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
			return "medium"
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

func sanitizeAnthropicToolUseInput(toolName string, input interface{}) interface{} {
	if !isClaudeReadToolName(toolName) {
		return input
	}
	object, ok := input.(map[string]interface{})
	if !ok {
		return input
	}
	if value, exists := object["pages"]; exists && isEmptyToolArgumentValue(value) {
		delete(object, "pages")
	}
	return object
}

func sanitizeAnthropicToolUseArguments(toolName string, arguments string) string {
	if !isClaudeReadToolName(toolName) || strings.TrimSpace(arguments) == "" {
		return arguments
	}
	var value interface{}
	decoder := json.NewDecoder(strings.NewReader(arguments))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return arguments
	}
	sanitized := sanitizeAnthropicToolUseInput(toolName, value)
	data, err := json.Marshal(sanitized)
	if err != nil {
		return arguments
	}
	return string(data)
}

func isClaudeReadToolName(toolName string) bool {
	return strings.EqualFold(strings.TrimSpace(toolName), "Read")
}

func isEmptyToolArgumentValue(value interface{}) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []interface{}:
		return len(typed) == 0
	default:
		return false
	}
}

func sortStringKeys(source map[string]interface{}) []string {
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
