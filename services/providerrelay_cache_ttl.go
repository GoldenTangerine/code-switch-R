package services

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	anthropicCacheTTL5m = "5m"
	anthropicCacheTTL1h = "1h"
)

func normalizeAnthropicCacheTTL(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case anthropicCacheTTL5m:
		return anthropicCacheTTL5m
	case anthropicCacheTTL1h:
		return anthropicCacheTTL1h
	default:
		return ""
	}
}

func applyProviderAnthropicCacheTTLOverride(provider Provider, endpoint string, body []byte) []byte {
	if strings.TrimSpace(endpoint) != "/v1/messages" {
		return body
	}
	if resolveClaudeAPIFormat(provider) != claudeAPIFormatAnthropic {
		return body
	}
	ttl := normalizeAnthropicCacheTTL(provider.AnthropicCacheTTL)
	if ttl == "" {
		return body
	}
	return forceEphemeralCacheControlTTL(body, ttl)
}

// forceEphemeralCacheControlTTL only changes existing Anthropic ephemeral
// cache_control blocks. It does not create new cache breakpoints.
func forceEphemeralCacheControlTTL(body []byte, ttl string) []byte {
	if len(body) == 0 || ttl == "" {
		return body
	}

	out := body
	paths := make([]string, 0)
	addPath := func(path string, value gjson.Result) {
		cc := value.Get("cache_control")
		if !cc.Exists() || cc.Get("type").String() != "ephemeral" {
			return
		}
		if cc.Get("ttl").String() == ttl {
			return
		}
		paths = append(paths, path+".cache_control.ttl")
	}

	if topCC := gjson.GetBytes(body, "cache_control"); topCC.Exists() &&
		topCC.Get("type").String() == "ephemeral" &&
		topCC.Get("ttl").String() != ttl {
		paths = append(paths, "cache_control.ttl")
	}

	system := gjson.GetBytes(body, "system")
	if system.IsArray() {
		idx := -1
		system.ForEach(func(_, block gjson.Result) bool {
			idx++
			addPath(fmt.Sprintf("system.%d", idx), block)
			return true
		})
	}

	messages := gjson.GetBytes(body, "messages")
	if messages.IsArray() {
		msgIdx := -1
		messages.ForEach(func(_, msg gjson.Result) bool {
			msgIdx++
			content := msg.Get("content")
			if !content.IsArray() {
				return true
			}
			contentIdx := -1
			content.ForEach(func(_, block gjson.Result) bool {
				contentIdx++
				addPath(fmt.Sprintf("messages.%d.content.%d", msgIdx, contentIdx), block)
				return true
			})
			return true
		})
	}

	tools := gjson.GetBytes(body, "tools")
	if tools.IsArray() {
		idx := -1
		tools.ForEach(func(_, tool gjson.Result) bool {
			idx++
			addPath(fmt.Sprintf("tools.%d", idx), tool)
			return true
		})
	}

	for _, path := range paths {
		if next, err := sjson.SetBytes(out, path, ttl); err == nil {
			out = next
		}
	}
	return out
}
