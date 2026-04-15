package services

import "strings"

const (
	claudeAPIFormatAnthropic      = "anthropic"
	claudeAPIFormatOpenAIChat     = "openai_chat"
	claudeAPIFormatOpenAIResponse = "openai_responses"
)

func normalizeClaudeAPIFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case claudeAPIFormatOpenAIChat:
		return claudeAPIFormatOpenAIChat
	case claudeAPIFormatOpenAIResponse:
		return claudeAPIFormatOpenAIResponse
	default:
		return claudeAPIFormatAnthropic
	}
}

func resolveClaudeAPIFormat(provider Provider) string {
	return normalizeClaudeAPIFormat(provider.APIFormat)
}

func claudeAPIFormatNeedsTransform(apiFormat string) bool {
	return normalizeClaudeAPIFormat(apiFormat) != claudeAPIFormatAnthropic
}

func claudeEndpointForAPIFormat(apiFormat string) string {
	switch normalizeClaudeAPIFormat(apiFormat) {
	case claudeAPIFormatOpenAIChat:
		return "/v1/chat/completions"
	case claudeAPIFormatOpenAIResponse:
		return "/responses"
	default:
		return "/v1/messages"
	}
}

func resolveProviderEffectiveEndpoint(kind string, provider Provider, defaultEndpoint string) string {
	if strings.EqualFold(strings.TrimSpace(kind), "claude") {
		apiFormat := resolveClaudeAPIFormat(provider)
		if claudeAPIFormatNeedsTransform(apiFormat) {
			return claudeEndpointForAPIFormat(apiFormat)
		}
	}
	return provider.GetEffectiveEndpoint(defaultEndpoint)
}

