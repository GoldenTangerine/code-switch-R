package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

type claudeProbeRequestSpec struct {
	Body            []byte
	SuccessContains string
}

func resolveClaudeProbeAPIFormat(provider *Provider, endpoint string) string {
	normalizedEndpoint := strings.ToLower(strings.TrimSpace(endpoint))
	switch {
	case strings.Contains(normalizedEndpoint, "/responses"):
		return claudeAPIFormatOpenAIResponse
	case strings.Contains(normalizedEndpoint, "/chat/completions"):
		return claudeAPIFormatOpenAIChat
	case strings.Contains(normalizedEndpoint, "/messages"):
		return claudeAPIFormatAnthropic
	}

	if provider != nil {
		return resolveClaudeAPIFormat(*provider)
	}
	return claudeAPIFormatAnthropic
}

func buildClaudeProbeRequest(provider *Provider, endpoint string, model string) (claudeProbeRequestSpec, error) {
	apiFormat := resolveClaudeProbeAPIFormat(provider, endpoint)
	maxTokens := 1
	if apiFormat == claudeAPIFormatOpenAIResponse {
		maxTokens = 16
	}

	anthropicBody := map[string]interface{}{
		"model":      model,
		"max_tokens": maxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
		},
	}

	bodyBytes, err := json.Marshal(anthropicBody)
	if err != nil {
		return claudeProbeRequestSpec{}, fmt.Errorf("构建 Claude 探测请求失败: %w", err)
	}

	if claudeAPIFormatNeedsTransform(apiFormat) {
		effectiveProvider := Provider{}
		if provider != nil {
			effectiveProvider = *provider
		}
		effectiveProvider.APIFormat = apiFormat

		bodyBytes, err = transformClaudeRequestForAPIFormat(bodyBytes, effectiveProvider)
		if err != nil {
			return claudeProbeRequestSpec{}, fmt.Errorf("转换 Claude 探测请求失败: %w", err)
		}
	}

	return claudeProbeRequestSpec{
		Body:            bodyBytes,
		SuccessContains: claudeProbeSuccessContains(apiFormat),
	}, nil
}

func claudeProbeSuccessContains(apiFormat string) string {
	switch normalizeClaudeAPIFormat(apiFormat) {
	case claudeAPIFormatOpenAIChat:
		return "choices"
	case claudeAPIFormatOpenAIResponse:
		return "output"
	default:
		return "content"
	}
}

func responseContainsExpectedField(body []byte, successContains string) bool {
	successContains = strings.TrimSpace(successContains)
	if successContains == "" {
		return true
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		return gjson.GetBytes(trimmed, successContains).Exists()
	}

	return strings.Contains(string(body), successContains)
}
