/*
@name: 供应商工具兼容
@Descripttion: 按供应商过滤工具并将动态工具协议转换为普通工具与历史文本。
@version: 1.0.0
@Author: sm
@Date: 2026-10-24 23:20:14
@LastEditTime: 2026-10-24 23:20:14
@FilePath: services/provider_tool_compatibility.go
*/
package services

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type ProviderToolCompatibility struct {
	DisabledNames         []string `json:"disabledNames,omitempty"`
	DisabledTypes         []string `json:"disabledTypes,omitempty"`
	NormalizeDynamicTools bool     `json:"normalizeDynamicTools,omitempty"`
}

func toolCompatibilitySet(values []string) map[string]bool {
	set := make(map[string]bool)
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			set[value] = true
		}
	}
	return set
}

func toolString(object map[string]interface{}, key string) string {
	value, _ := object[key].(string)
	return value
}

// Transform only Anthropic input before protocol conversion, after forced body fields.
// Each provider gets its own decoded copy; failover must retain the original request.
func applyProviderToolCompatibility(provider Provider, endpoint string, body []byte) ([]byte, []string, error) {
	config := provider.ToolCompatibility
	if endpoint != "/v1/messages" || config == nil {
		return body, nil, nil
	}
	names, types := toolCompatibilitySet(config.DisabledNames), toolCompatibilitySet(config.DisabledTypes)
	if len(names) == 0 && len(types) == 0 && !config.NormalizeDynamicTools {
		return body, nil, nil
	}
	var request map[string]interface{}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	if err := decoder.Decode(&request); err != nil {
		return nil, nil, fmt.Errorf("工具兼容请求解析失败: %w", err)
	}
	removed := make(map[string]bool)
	changed := false
	definitions, _ := request["tools"].([]interface{})
	messages, _ := request["messages"].([]interface{})
	// Inline additions may carry full definitions instead of references.
	for _, value := range messages {
		message, _ := value.(map[string]interface{})
		blocks, _ := message["content"].([]interface{})
		for _, value := range blocks {
			block, _ := value.(map[string]interface{})
			tool, _ := block["tool"].(map[string]interface{})
			if toolString(block, "type") == "tool_addition" && tool != nil && !strings.HasSuffix(toolString(tool, "type"), "_reference") {
				if names[toolString(tool, "name")] || types[toolString(tool, "type")] {
					names[toolString(tool, "name")] = true
				}
				if config.NormalizeDynamicTools {
					definitions = append(definitions, tool)
				}
			}
		}
	}
	for _, value := range definitions {
		tool, _ := value.(map[string]interface{})
		if types[toolString(tool, "type")] {
			names[toolString(tool, "name")] = true
		}
	}
	blockedIDs := make(map[string]bool)
	for _, value := range messages {
		message, _ := value.(map[string]interface{})
		blocks, _ := message["content"].([]interface{})
		for _, value := range blocks {
			block, _ := value.(map[string]interface{})
			kind := toolString(block, "type")
			if (kind == "tool_use" || kind == "server_tool_use") && names[toolString(block, "name")] {
				if id := toolString(block, "id"); id != "" {
					blockedIDs[id] = true
				}
			}
		}
	}
	keptTools := make([]interface{}, 0, len(definitions))
	seenTools := make(map[string]bool)
	for _, value := range definitions {
		tool, _ := value.(map[string]interface{})
		name := toolString(tool, "name")
		if names[name] {
			removed[name], changed = true, true
			continue
		}
		if config.NormalizeDynamicTools {
			if _, exists := tool["defer_loading"]; exists {
				delete(tool, "defer_loading")
				changed = true
			}
			if name != "" && seenTools[name] {
				changed = true
				continue
			}
		}
		seenTools[name] = true
		keptTools = append(keptTools, value)
	}
	var cleanBlocks func([]interface{}) []interface{}
	cleanBlocks = func(blocks []interface{}) []interface{} {
		out := make([]interface{}, 0, len(blocks))
		for _, value := range blocks {
			block, ok := value.(map[string]interface{})
			if !ok {
				out = append(out, value)
				continue
			}
			kind, name := toolString(block, "type"), toolString(block, "name")
			if ((kind == "tool_use" || kind == "server_tool_use") && names[name]) || blockedIDs[toolString(block, "tool_use_id")] {
				// Keep historical data as inert text, including paired results, rather than
				// leaving orphan tool_result blocks or erasing previously gathered context.
				encoded, _ := json.Marshal(block)
				out = append(out, map[string]interface{}{"type": "text", "text": "Historical tool activity (tool unavailable on this provider): " + string(encoded)})
				if name != "" {
					removed[name] = true
				}
				changed = true
				continue
			}
			if kind == "tool_addition" || kind == "tool_removal" || kind == "tool_reference" {
				if kind != "tool_reference" {
					if tool, ok := block["tool"].(map[string]interface{}); ok {
						name = toolString(tool, "name")
					} else {
						name, _ = block["tool"].(string)
					}
				}
				if names[name] || config.NormalizeDynamicTools {
					if names[name] {
						removed[name] = true
					}
					changed = true
					continue
				}
			}
			if kind == "tool_result" {
				if nested, ok := block["content"].([]interface{}); ok {
					cleaned := cleanBlocks(nested)
					if len(cleaned) == 0 && len(nested) > 0 {
						cleaned = []interface{}{map[string]interface{}{"type": "text", "text": "Tool availability updated."}}
					}
					block["content"] = cleaned
				}
			}
			out = append(out, block)
		}
		return out
	}
	keptMessages := make([]interface{}, 0, len(messages))
	for _, value := range messages {
		message, ok := value.(map[string]interface{})
		if !ok {
			keptMessages = append(keptMessages, value)
			continue
		}
		if blocks, ok := message["content"].([]interface{}); ok {
			cleaned := cleanBlocks(blocks)
			if len(cleaned) == 0 && len(blocks) > 0 {
				changed = true
				continue
			}
			message["content"] = cleaned
		}
		if config.NormalizeDynamicTools && toolString(message, "role") == "system" {
			message["role"] = "user"
			changed = true
		}
		keptMessages = append(keptMessages, message)
	}
	if choice, ok := request["tool_choice"].(map[string]interface{}); ok {
		if (toolString(choice, "type") == "tool" && names[toolString(choice, "name")]) || (len(keptTools) == 0 && len(definitions) > 0) {
			delete(request, "tool_choice")
			changed = true
		}
	}
	if !changed {
		return body, nil, nil
	}
	if len(keptTools) == 0 {
		delete(request, "tools")
	} else {
		request["tools"] = keptTools
	}
	if messages != nil {
		request["messages"] = keptMessages
	}
	result, err := json.Marshal(request)
	if err != nil {
		return nil, nil, fmt.Errorf("工具兼容请求编码失败: %w", err)
	}
	report := make([]string, 0, len(removed)+1)
	for name := range removed {
		if name != "" {
			report = append(report, name)
		}
	}
	sort.Strings(report)
	if config.NormalizeDynamicTools {
		report = append(report, "dynamic-tools")
	}
	return result, report, nil
}

func applyProviderToolCompatibilityHeaders(provider Provider, endpoint string, headers map[string]string) {
	if endpoint != "/v1/messages" || provider.ToolCompatibility == nil || !provider.ToolCompatibility.NormalizeDynamicTools {
		return
	}
	for key, value := range headers {
		if !strings.EqualFold(key, "anthropic-beta") {
			continue
		}
		tokens := make([]string, 0)
		for _, token := range strings.Split(value, ",") {
			token = strings.TrimSpace(token)
			if strings.HasPrefix(token, "mid-conversation-") || strings.HasPrefix(token, "inline-tools-") {
				continue
			}
			tokens = append(tokens, token)
		}
		if len(tokens) == 0 {
			delete(headers, key)
		} else {
			headers[key] = strings.Join(tokens, ",")
		}
	}
}
