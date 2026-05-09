package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

const (
	availabilityProbeExpectedText       = "pong"
	availabilityProbeSystemInstruction  = "You are an echo bot. Reply with exactly pong."
	availabilityProbeUserInput          = "ping"
	availabilityProbeUserAgentCodex     = "Codex-CLI/1.0"
	availabilityProbeUserAgentOpenAI    = "OpenAI-Compatible/2026.04"
	availabilityProbeUserAgentAnthropic = "Claude-CLI/HealthCheck"
	availabilityProbeUserAgentClaudeCLI = "claude-cli/2.1.84 (external, cli)"

	availabilityProbeBodyPresetDefault        = ""
	availabilityProbeBodyPresetResponsesCodex = "responses_codex_basic"
	availabilityProbeBodyPresetResponsesGPT   = "responses_gpt_basic"
)

var availabilityProbeRetryableHTTPStatusCodes = map[int]struct{}{
	400: {},
	404: {},
	405: {},
	415: {},
	422: {},
}

type availabilityProbePlan struct {
	BodyBytes         []byte
	Headers           map[string]string
	ExpectedText      string
	ResponseFormat    string
	EffectiveModel    string
	EffectiveEndpoint string
	PresetID          string
	BodyPreset        string
}

type availabilityProbeExecutionResult struct {
	Plan           availabilityProbePlan
	LatencyMs      int
	HTTPStatusCode int
	ResponseBody   []byte
}

type availabilityProbeBuildError struct {
	cause error
}

type availabilityProbePresetCandidate struct {
	PresetID       string
	Endpoint       string
	ResponseFormat string
	BodyPreset     string
	ExtraHeaders   map[string]string
}

func (e availabilityProbeBuildError) Error() string {
	if e.cause == nil {
		return "构建探测请求失败"
	}
	return e.cause.Error()
}

func (e availabilityProbeBuildError) Unwrap() error {
	return e.cause
}

func defaultAvailabilityProbeModel(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "claude":
		return "claude-3-5-haiku-20241022"
	case "codex":
		return "gpt-4o-mini"
	case "gemini":
		return "gemini-1.5-flash"
	default:
		return "gpt-3.5-turbo"
	}
}

func normalizeAvailabilityEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	return endpoint
}

func defaultAvailabilityProbeEndpoint(provider *Provider, platform string) string {
	if provider != nil {
		if strings.EqualFold(platform, "claude") {
			return normalizeAvailabilityEndpoint(resolveProviderEffectiveEndpoint("claude", *provider, "/v1/messages"))
		}
		if strings.TrimSpace(provider.APIEndpoint) != "" {
			return normalizeAvailabilityEndpoint(provider.GetEffectiveEndpoint(""))
		}
	}

	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "codex":
		return "/responses"
	default:
		return "/v1/chat/completions"
	}
}

func resolveProviderAvailabilityModel(provider *Provider, platform string) string {
	if provider != nil {
		if provider.AvailabilityConfig != nil {
			if model := strings.TrimSpace(provider.AvailabilityConfig.TestModel); model != "" {
				return model
			}
		}
		if model := strings.TrimSpace(provider.ConnectivityTestModel); model != "" {
			return model
		}
	}
	return defaultAvailabilityProbeModel(platform)
}

func resolveProviderAvailabilityEndpoint(provider *Provider, platform string) string {
	if provider != nil {
		if provider.AvailabilityConfig != nil {
			if endpoint := normalizeAvailabilityEndpoint(provider.AvailabilityConfig.TestEndpoint); endpoint != "" {
				return endpoint
			}
		}
		if endpoint := normalizeAvailabilityEndpoint(provider.ConnectivityTestEndpoint); endpoint != "" {
			return endpoint
		}
	}
	return defaultAvailabilityProbeEndpoint(provider, platform)
}

func resolveProviderAvailabilityTimeout(provider *Provider) int {
	if provider != nil && provider.AvailabilityConfig != nil && provider.AvailabilityConfig.Timeout > 0 {
		return provider.AvailabilityConfig.Timeout
	}
	return DefaultTimeoutMs
}

func normalizeAvailabilityConfig(config *AvailabilityConfig, provider *Provider, platform string) *AvailabilityConfig {
	if config == nil {
		return nil
	}

	timeout := config.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeoutMs
	}

	endpoint := normalizeAvailabilityEndpoint(config.TestEndpoint)
	if endpoint == "" {
		endpoint = defaultAvailabilityProbeEndpoint(provider, platform)
	}

	return &AvailabilityConfig{
		TestModel:    strings.TrimSpace(config.TestModel),
		TestEndpoint: endpoint,
		Timeout:      timeout,
	}
}

func buildAvailabilityTargetURL(baseURL string, endpoint string) string {
	endpoint = normalizeAvailabilityEndpoint(endpoint)
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	return joinURL(baseURL, endpoint)
}

func applyProviderAuthHeaders(headers http.Header, provider *Provider, platform string) {
	if provider == nil || strings.TrimSpace(provider.APIKey) == "" {
		return
	}

	apiKey := strings.TrimSpace(provider.APIKey)
	authTypeRaw := strings.TrimSpace(provider.ConnectivityAuthType)
	authType := strings.ToLower(authTypeRaw)
	isClaude := strings.EqualFold(platform, "claude")

	setAnthropicVersion := func() {
		if isClaude {
			headers.Set("anthropic-version", "2023-06-01")
		}
	}
	setClaudeCompatibilityHeaders := func() {
		headers.Set("x-api-key", apiKey)
		headers.Set("Authorization", "Bearer "+apiKey)
		setAnthropicVersion()
	}

	switch authType {
	case "":
		if isClaude {
			setClaudeCompatibilityHeaders()
			return
		}
		headers.Set("Authorization", "Bearer "+apiKey)
	case "x-api-key":
		headers.Set("x-api-key", apiKey)
		setAnthropicVersion()
		if isClaude {
			headers.Set("Authorization", "Bearer "+apiKey)
		}
	case "bearer":
		headers.Set("Authorization", "Bearer "+apiKey)
		if isClaude {
			headers.Set("x-api-key", apiKey)
			setAnthropicVersion()
		}
	default:
		headerName := authTypeRaw
		if headerName == "" || strings.EqualFold(headerName, "custom") {
			headerName = "Authorization"
		}
		headers.Set(headerName, apiKey)
		setAnthropicVersion()
	}
}

func executeAvailabilityProbe(
	ctx context.Context,
	client *http.Client,
	provider *Provider,
	platform string,
	model string,
	endpoint string,
	timeoutMs int,
) (availabilityProbeExecutionResult, error) {
	plans, err := buildAvailabilityProbePlans(platform, provider, model, endpoint)
	if err != nil {
		return availabilityProbeExecutionResult{}, availabilityProbeBuildError{cause: err}
	}

	if client == nil {
		client = &http.Client{Timeout: 0}
	}

	var deadline time.Time
	if timeoutMs > 0 {
		deadline = time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	}

	var fallbackResult availabilityProbeExecutionResult
	hasFallbackResult := false

	for i, plan := range plans {
		attemptTimeoutMs := timeoutMs
		if !deadline.IsZero() {
			remainingMs := int(time.Until(deadline).Milliseconds())
			if remainingMs < 1000 {
				remainingMs = 1000
			}
			attemptTimeoutMs = remainingMs
		}

		result, execErr := executeSingleAvailabilityProbe(ctx, client, provider, platform, plan, attemptTimeoutMs)
		if execErr != nil {
			return result, execErr
		}
		if result.HTTPStatusCode == 0 {
			return result, nil
		}
		if availabilityProbeSucceeded(result) {
			return result, nil
		}
		if shouldRetryAvailabilityProbeWithoutPromptCacheKey(result) {
			retryPlan := result.Plan
			retryPlan.BodyBytes = removeJSONFieldBytes(retryPlan.BodyBytes, "prompt_cache_key")
			disableOpenAICompatPromptCache(*provider, "")
			result, execErr = executeSingleAvailabilityProbe(ctx, client, provider, platform, retryPlan, attemptTimeoutMs)
			if execErr != nil {
				return result, execErr
			}
			if result.HTTPStatusCode == 0 || availabilityProbeSucceeded(result) {
				return result, nil
			}
		}
		if !shouldRetryAvailabilityProbe(result) || i == len(plans)-1 {
			return result, nil
		}

		fallbackResult = result
		hasFallbackResult = true
	}

	if hasFallbackResult {
		return fallbackResult, nil
	}

	return availabilityProbeExecutionResult{}, fmt.Errorf("未生成可执行的可用性探测计划")
}

func executeSingleAvailabilityProbe(
	ctx context.Context,
	client *http.Client,
	provider *Provider,
	platform string,
	plan availabilityProbePlan,
	timeoutMs int,
) (availabilityProbeExecutionResult, error) {
	targetURL := buildAvailabilityTargetURL(provider.APIURL, plan.EffectiveEndpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(plan.BodyBytes))
	if err != nil {
		return availabilityProbeExecutionResult{Plan: plan}, err
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range plan.Headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(key, value)
	}
	applyProviderAuthHeaders(req.Header, provider, platform)

	if timeoutMs > 0 {
		reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()
		req = req.WithContext(reqCtx)
	}

	start := time.Now()
	resp, err := client.Do(req)
	latencyMs := int(time.Since(start).Milliseconds())
	if err != nil {
		return availabilityProbeExecutionResult{
			Plan:      plan,
			LatencyMs: latencyMs,
		}, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if readErr != nil {
		body = []byte{}
	}

	return availabilityProbeExecutionResult{
		Plan:           plan,
		LatencyMs:      latencyMs,
		HTTPStatusCode: resp.StatusCode,
		ResponseBody:   body,
	}, nil
}

func availabilityProbeSucceeded(result availabilityProbeExecutionResult) bool {
	return result.HTTPStatusCode >= 200 &&
		result.HTTPStatusCode < 400 &&
		responseContainsExpectedText(result.ResponseBody, result.Plan.ResponseFormat, result.Plan.ExpectedText)
}

func shouldRetryAvailabilityProbe(result availabilityProbeExecutionResult) bool {
	if _, ok := availabilityProbeRetryableHTTPStatusCodes[result.HTTPStatusCode]; ok {
		return true
	}

	if result.HTTPStatusCode >= 200 && result.HTTPStatusCode < 400 {
		return !responseContainsExpectedText(result.ResponseBody, result.Plan.ResponseFormat, result.Plan.ExpectedText)
	}

	return false
}

func shouldRetryAvailabilityProbeWithoutPromptCacheKey(result availabilityProbeExecutionResult) bool {
	if !gjson.GetBytes(result.Plan.BodyBytes, "prompt_cache_key").Exists() {
		return false
	}
	return isOpenAICompatPromptCacheKeyUnsupportedStatus(result.HTTPStatusCode, result.ResponseBody)
}

func buildAvailabilityProbePlans(platform string, provider *Provider, model string, endpoint string) ([]availabilityProbePlan, error) {
	if provider == nil {
		return nil, fmt.Errorf("provider 不能为空")
	}

	candidates := buildAvailabilityProbeCandidates(platform, provider, model, endpoint)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("未生成可用的探测候选")
	}

	plans := make([]availabilityProbePlan, 0, len(candidates))
	for idx, candidate := range candidates {
		plan, err := buildAvailabilityProbePlanFromCandidate(platform, provider, model, candidate)
		if err != nil {
			if idx == 0 {
				return nil, err
			}
			continue
		}
		plans = append(plans, plan)
	}

	if len(plans) == 0 {
		return nil, fmt.Errorf("未生成可执行的探测计划")
	}

	return plans, nil
}

func buildAvailabilityProbePlan(platform string, provider *Provider, model string, endpoint string) (availabilityProbePlan, error) {
	candidate := availabilityProbePresetCandidate{
		PresetID:       "configured",
		Endpoint:       normalizeAvailabilityEndpoint(endpoint),
		ResponseFormat: resolveAvailabilityProbeResponseFormat(platform, provider, endpoint),
	}
	if strings.EqualFold(platform, "claude") && strings.Contains(strings.ToLower(candidate.Endpoint), "beta=true") {
		candidate.PresetID = "cc_beta_cli"
		candidate.ExtraHeaders = buildClaudeBetaProbeHeaders()
	}
	return buildAvailabilityProbePlanFromCandidate(platform, provider, model, candidate)
}

func buildAvailabilityProbePlanFromCandidate(platform string, provider *Provider, model string, candidate availabilityProbePresetCandidate) (availabilityProbePlan, error) {
	if provider == nil {
		return availabilityProbePlan{}, fmt.Errorf("provider 不能为空")
	}

	endpoint := normalizeAvailabilityEndpoint(candidate.Endpoint)
	if endpoint == "" {
		endpoint = defaultAvailabilityProbeEndpoint(provider, platform)
	}
	responseFormat := strings.TrimSpace(candidate.ResponseFormat)
	if responseFormat == "" {
		responseFormat = resolveAvailabilityProbeResponseFormat(platform, provider, endpoint)
	}

	effectiveModel := provider.GetEffectiveModel(strings.TrimSpace(model))
	bodyBytes, err := buildAvailabilityProbeBody(responseFormat, effectiveModel, candidate.BodyPreset)
	if err != nil {
		return availabilityProbePlan{}, err
	}

	if len(provider.RequestBodyOverrides) > 0 {
		bodyBytes, err = ApplyRequestBodyOverrides(bodyBytes, provider.RequestBodyOverrides)
		if err != nil {
			return availabilityProbePlan{}, fmt.Errorf("应用请求体覆盖失败: %w", err)
		}
	}
	if normalizeClaudeAPIFormat(responseFormat) == claudeAPIFormatOpenAIResponse && isOpenAICompatPromptCacheDisabled(*provider, "") {
		bodyBytes = removeJSONFieldBytes(bodyBytes, "prompt_cache_key")
	}

	effectiveModel = resolveModelFromRequestBody(bodyBytes, effectiveModel)
	headers := buildAvailabilityProbeHeaders(responseFormat)
	mergeAvailabilityProbeHeaders(headers, candidate.ExtraHeaders)

	presetID := strings.TrimSpace(candidate.PresetID)
	if presetID == "" {
		presetID = "configured"
	}

	return availabilityProbePlan{
		BodyBytes:         bodyBytes,
		Headers:           headers,
		ExpectedText:      availabilityProbeExpectedText,
		ResponseFormat:    responseFormat,
		EffectiveModel:    effectiveModel,
		EffectiveEndpoint: endpoint,
		PresetID:          presetID,
		BodyPreset:        candidate.BodyPreset,
	}, nil
}

func buildAvailabilityProbeCandidates(platform string, provider *Provider, model string, endpoint string) []availabilityProbePresetCandidate {
	platform = strings.ToLower(strings.TrimSpace(platform))
	endpoint = normalizeAvailabilityEndpoint(endpoint)
	if endpoint == "" {
		endpoint = defaultAvailabilityProbeEndpoint(provider, platform)
	}
	requestedFormat := resolveAvailabilityProbeResponseFormat(platform, provider, endpoint)
	responseBodyPresets := availabilityProbeResponsesBodyPresets(model)

	candidates := make([]availabilityProbePresetCandidate, 0, 8)
	seen := make(map[string]struct{})
	addCandidates := func(presetID string, endpoint string, responseFormat string, bodyPresets []string, extraHeaders map[string]string) {
		endpoint = normalizeAvailabilityEndpoint(endpoint)
		if endpoint == "" {
			return
		}
		if len(bodyPresets) == 0 {
			bodyPresets = []string{availabilityProbeBodyPresetDefault}
		}
		for _, bodyPreset := range bodyPresets {
			candidate := availabilityProbePresetCandidate{
				PresetID:       presetID,
				Endpoint:       endpoint,
				ResponseFormat: responseFormat,
				BodyPreset:     bodyPreset,
				ExtraHeaders:   cloneAvailabilityProbeHeaders(extraHeaders),
			}
			key := availabilityProbeCandidateKey(candidate)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			candidates = append(candidates, candidate)
		}
	}

	configuredPresetID := "configured"
	configuredHeaders := map[string]string(nil)
	if strings.EqualFold(platform, "claude") && strings.Contains(strings.ToLower(endpoint), "beta=true") {
		configuredPresetID = "cc_beta_cli"
		configuredHeaders = buildClaudeBetaProbeHeaders()
	}
	addCandidates(configuredPresetID, endpoint, requestedFormat, availabilityProbeBodyPresetsForFormat(requestedFormat, responseBodyPresets), configuredHeaders)

	switch platform {
	case "claude":
		addClaudeAnthropicFallbacks := func() {
			addCandidates("cc_haiku_basic", "/v1/messages", claudeAPIFormatAnthropic, nil, nil)
			if shouldAddClaudeBetaProbeCandidate(provider, endpoint) {
				addCandidates("cc_beta_cli", "/v1/messages?beta=true", claudeAPIFormatAnthropic, nil, buildClaudeBetaProbeHeaders())
			}
		}
		addClaudeChatFallbacks := func(configured string) {
			for _, chatEndpoint := range availabilityProbeChatEndpointCandidates(configured) {
				addCandidates("oa_chat_basic", chatEndpoint, claudeAPIFormatOpenAIChat, nil, nil)
			}
		}
		addClaudeResponsesFallbacks := func(configured string) {
			for _, responseEndpoint := range availabilityProbeResponseEndpointCandidates(configured) {
				addCandidates("cx_responses", responseEndpoint, claudeAPIFormatOpenAIResponse, responseBodyPresets, nil)
			}
		}

		switch normalizeClaudeAPIFormat(requestedFormat) {
		case claudeAPIFormatAnthropic:
			addClaudeAnthropicFallbacks()
			addClaudeChatFallbacks("")
			addClaudeResponsesFallbacks("")
		case claudeAPIFormatOpenAIChat:
			addClaudeChatFallbacks(endpoint)
			addClaudeAnthropicFallbacks()
			addClaudeResponsesFallbacks("")
		case claudeAPIFormatOpenAIResponse:
			addClaudeResponsesFallbacks(endpoint)
			addClaudeChatFallbacks("")
			addClaudeAnthropicFallbacks()
		default:
			addClaudeAnthropicFallbacks()
			addClaudeChatFallbacks("")
			addClaudeResponsesFallbacks("")
		}
	case "codex":
		if normalizeClaudeAPIFormat(requestedFormat) == claudeAPIFormatOpenAIChat {
			for _, chatEndpoint := range availabilityProbeChatEndpointCandidates(endpoint) {
				addCandidates("oa_chat_basic", chatEndpoint, claudeAPIFormatOpenAIChat, nil, nil)
			}
		}
		for _, responseEndpoint := range availabilityProbeResponseEndpointCandidates(endpointIfMatchesFormat(endpoint, requestedFormat, claudeAPIFormatOpenAIResponse)) {
			addCandidates("cx_responses", responseEndpoint, claudeAPIFormatOpenAIResponse, responseBodyPresets, nil)
		}
		for _, chatEndpoint := range availabilityProbeChatEndpointCandidates("") {
			addCandidates("oa_chat_basic", chatEndpoint, claudeAPIFormatOpenAIChat, nil, nil)
		}
	default:
		if normalizeClaudeAPIFormat(requestedFormat) == claudeAPIFormatOpenAIResponse {
			for _, responseEndpoint := range availabilityProbeResponseEndpointCandidates(endpoint) {
				addCandidates("cx_responses", responseEndpoint, claudeAPIFormatOpenAIResponse, responseBodyPresets, nil)
			}
		}
		for _, chatEndpoint := range availabilityProbeChatEndpointCandidates(endpointIfMatchesFormat(endpoint, requestedFormat, claudeAPIFormatOpenAIChat)) {
			addCandidates("oa_chat_basic", chatEndpoint, claudeAPIFormatOpenAIChat, nil, nil)
		}
		for _, responseEndpoint := range availabilityProbeResponseEndpointCandidates("") {
			addCandidates("cx_responses", responseEndpoint, claudeAPIFormatOpenAIResponse, responseBodyPresets, nil)
		}
	}

	return candidates
}

func endpointIfMatchesFormat(endpoint string, currentFormat string, wantFormat string) string {
	if normalizeClaudeAPIFormat(currentFormat) == normalizeClaudeAPIFormat(wantFormat) {
		return endpoint
	}
	return ""
}

func availabilityProbeBodyPresetsForFormat(responseFormat string, responseBodyPresets []string) []string {
	if normalizeClaudeAPIFormat(responseFormat) == claudeAPIFormatOpenAIResponse {
		return responseBodyPresets
	}
	return nil
}

func availabilityProbeResponsesBodyPresets(model string) []string {
	normalizedModel := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(normalizedModel, "codex"):
		return []string{availabilityProbeBodyPresetResponsesCodex, availabilityProbeBodyPresetResponsesGPT}
	case strings.Contains(normalizedModel, "gpt-"),
		strings.Contains(normalizedModel, "o1"),
		strings.Contains(normalizedModel, "o3"),
		strings.Contains(normalizedModel, "o4"):
		return []string{availabilityProbeBodyPresetResponsesGPT, availabilityProbeBodyPresetResponsesCodex}
	default:
		return []string{availabilityProbeBodyPresetResponsesCodex, availabilityProbeBodyPresetResponsesGPT}
	}
}

func availabilityProbeResponseEndpointCandidates(configured string) []string {
	return availabilityProbeEndpointCandidates(configured, "/responses", "/v1/responses")
}

func availabilityProbeChatEndpointCandidates(configured string) []string {
	return availabilityProbeEndpointCandidates(configured, "/v1/chat/completions", "/chat/completions")
}

func availabilityProbeEndpointCandidates(configured string, defaults ...string) []string {
	seen := make(map[string]struct{})
	variants := make([]string, 0, len(defaults)+1)
	add := func(endpoint string) {
		endpoint = normalizeAvailabilityEndpoint(endpoint)
		if endpoint == "" {
			return
		}
		if _, exists := seen[endpoint]; exists {
			return
		}
		seen[endpoint] = struct{}{}
		variants = append(variants, endpoint)
	}
	add(configured)
	for _, endpoint := range defaults {
		add(endpoint)
	}
	return variants
}

func availabilityProbeCandidateKey(candidate availabilityProbePresetCandidate) string {
	parts := make([]string, 0, len(candidate.ExtraHeaders))
	for key, value := range candidate.ExtraHeaders {
		parts = append(parts, strings.TrimSpace(key)+"="+strings.TrimSpace(value))
	}
	sort.Strings(parts)
	return strings.Join([]string{
		strings.TrimSpace(candidate.Endpoint),
		normalizeClaudeAPIFormat(candidate.ResponseFormat),
		strings.TrimSpace(candidate.BodyPreset),
		strings.Join(parts, "&"),
	}, "|")
}

func cloneAvailabilityProbeHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(headers))
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}

func mergeAvailabilityProbeHeaders(target map[string]string, extra map[string]string) {
	if len(extra) == 0 {
		return
	}
	for key, value := range extra {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		target[key] = value
	}
}

func shouldAddClaudeBetaProbeCandidate(provider *Provider, endpoint string) bool {
	if strings.Contains(strings.ToLower(strings.TrimSpace(endpoint)), "beta=true") {
		return true
	}
	if provider == nil {
		return false
	}
	return looksLikeAnthropicRelayURL(provider.APIURL)
}

func looksLikeAnthropicRelayURL(rawURL string) bool {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return false
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	hostname := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if hostname == "" {
		return false
	}
	if strings.HasSuffix(hostname, "anthropic.com") || strings.HasSuffix(hostname, "claude.ai") {
		return false
	}

	for _, hint := range []string{"proxy", "relay", "gateway", "router", "worker", "openrouter", "api2d", "oaipro"} {
		if strings.Contains(hostname, hint) {
			return true
		}
	}

	return false
}

func buildClaudeBetaProbeHeaders() map[string]string {
	return map[string]string{
		"User-Agent":     availabilityProbeUserAgentClaudeCLI,
		"Anthropic-Beta": "oauth-2025-04-20,interleaved-thinking-2025-05-14,context-management-2025-06-27,prompt-caching-scope-2026-01-05",
		"Anthropic-Dangerous-Direct-Browser-Access": "true",
		"X-App": "cli",
	}
}

func resolveAvailabilityProbeResponseFormat(platform string, provider *Provider, endpoint string) string {
	normalizedEndpoint := strings.ToLower(strings.TrimSpace(endpoint))
	switch {
	case strings.Contains(normalizedEndpoint, "/responses"):
		return claudeAPIFormatOpenAIResponse
	case strings.Contains(normalizedEndpoint, "/chat/completions"):
		return claudeAPIFormatOpenAIChat
	case strings.Contains(normalizedEndpoint, "/messages"):
		return claudeAPIFormatAnthropic
	}

	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "claude":
		return resolveClaudeProbeAPIFormat(provider, endpoint)
	case "codex":
		return claudeAPIFormatOpenAIResponse
	default:
		return claudeAPIFormatOpenAIChat
	}
}

func buildAvailabilityProbeHeaders(responseFormat string) map[string]string {
	headers := map[string]string{
		"Accept": "application/json",
	}

	switch normalizeClaudeAPIFormat(responseFormat) {
	case claudeAPIFormatOpenAIResponse:
		headers["openai-beta"] = "responses=experimental"
		headers["User-Agent"] = availabilityProbeUserAgentCodex
	case claudeAPIFormatOpenAIChat:
		headers["User-Agent"] = availabilityProbeUserAgentOpenAI
	default:
		headers["User-Agent"] = availabilityProbeUserAgentAnthropic
	}

	return headers
}

func buildAvailabilityProbeBody(responseFormat string, model string, bodyPreset string) ([]byte, error) {
	var body map[string]interface{}
	switch normalizeClaudeAPIFormat(responseFormat) {
	case claudeAPIFormatAnthropic:
		body = buildAnthropicAvailabilityProbeBody(model)
	case claudeAPIFormatOpenAIResponse:
		switch strings.TrimSpace(bodyPreset) {
		case availabilityProbeBodyPresetResponsesGPT:
			body = buildResponsesGPTAvailabilityProbeBody(model)
		default:
			body = buildResponsesCodexAvailabilityProbeBody(model)
		}
	case claudeAPIFormatOpenAIChat:
		fallthrough
	default:
		body = buildOpenAIChatAvailabilityProbeBody(model)
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("构建可用性探测请求失败: %w", err)
	}
	return bodyBytes, nil
}

func buildAnthropicAvailabilityProbeBody(model string) map[string]interface{} {
	return map[string]interface{}{
		"model":      model,
		"system":     availabilityProbeSystemInstruction,
		"max_tokens": 20,
		"stream":     false,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": availabilityProbeUserInput,
			},
		},
	}
}

func buildOpenAIChatAvailabilityProbeBody(model string) map[string]interface{} {
	body := map[string]interface{}{
		"model":  model,
		"stream": false,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": availabilityProbeSystemInstruction,
			},
			{
				"role":    "user",
				"content": availabilityProbeUserInput,
			},
		},
	}

	if isOpenAIOSeries(model) {
		body["max_completion_tokens"] = 20
	} else {
		body["max_tokens"] = 20
	}

	if supportsReasoningEffort(model) {
		body["reasoning_effort"] = "low"
	}

	return body
}

func buildResponsesCodexAvailabilityProbeBody(model string) map[string]interface{} {
	body := map[string]interface{}{
		"model":        model,
		"instructions": availabilityProbeSystemInstruction,
		"input": []map[string]interface{}{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]string{
					{
						"type": "input_text",
						"text": availabilityProbeUserInput,
					},
				},
			},
		},
		"tools":               []interface{}{},
		"tool_choice":         "auto",
		"parallel_tool_calls": false,
		"max_output_tokens":   20,
		"store":               false,
		"stream":              false,
	}

	if supportsReasoningEffort(model) {
		body["reasoning"] = map[string]interface{}{
			"effort": "low",
		}
	}

	return body
}

func buildResponsesGPTAvailabilityProbeBody(model string) map[string]interface{} {
	body := map[string]interface{}{
		"model":        model,
		"instructions": availabilityProbeSystemInstruction,
		"input": []map[string]interface{}{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]string{
					{
						"type": "input_text",
						"text": availabilityProbeUserInput,
					},
				},
			},
		},
		"max_output_tokens": 20,
		"store":             false,
		"stream":            false,
	}

	if supportsReasoningEffort(model) {
		body["reasoning"] = map[string]interface{}{
			"effort": "low",
		}
	}

	return body
}

func responseContainsExpectedText(body []byte, responseFormat string, expectedText string) bool {
	expectedText = strings.TrimSpace(expectedText)
	if expectedText == "" {
		return true
	}

	trimmedBody := bytes.TrimSpace(body)
	responseText := strings.TrimSpace(extractAvailabilityResponseText(trimmedBody, responseFormat))
	if responseText == "" {
		if len(trimmedBody) == 0 {
			return false
		}
		if json.Valid(trimmedBody) {
			return false
		}
		responseText = strings.TrimSpace(string(trimmedBody))
	}

	return strings.Contains(strings.ToLower(responseText), strings.ToLower(expectedText))
}

func extractAvailabilityResponseText(body []byte, responseFormat string) string {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ""
	}

	if !json.Valid(trimmed) {
		return strings.TrimSpace(string(trimmed))
	}

	root := gjson.ParseBytes(trimmed)
	switch normalizeClaudeAPIFormat(responseFormat) {
	case claudeAPIFormatAnthropic:
		if text := extractAnthropicResponseText(root); text != "" {
			return text
		}
	case claudeAPIFormatOpenAIResponse:
		if text := extractResponsesResponseText(root); text != "" {
			return text
		}
	case claudeAPIFormatOpenAIChat:
		if text := extractOpenAIChatResponseText(root); text != "" {
			return text
		}
	}

	if text := extractCommonResponseText(root); text != "" {
		return text
	}

	return ""
}

func extractAnthropicResponseText(root gjson.Result) string {
	var parts []string
	root.Get("content").ForEach(func(_, block gjson.Result) bool {
		if block.Get("type").String() != "text" {
			return true
		}
		text := strings.TrimSpace(block.Get("text").String())
		if text != "" {
			parts = append(parts, text)
		}
		return true
	})
	return strings.Join(parts, "")
}

func extractOpenAIChatResponseText(root gjson.Result) string {
	var parts []string
	root.Get("choices").ForEach(func(_, choice gjson.Result) bool {
		messageContent := choice.Get("message.content")
		switch {
		case messageContent.Type == gjson.String:
			text := strings.TrimSpace(messageContent.String())
			if text != "" {
				parts = append(parts, text)
			}
		case messageContent.IsArray():
			messageContent.ForEach(func(_, block gjson.Result) bool {
				switch block.Get("type").String() {
				case "text", "output_text":
					text := strings.TrimSpace(block.Get("text").String())
					if text != "" {
						parts = append(parts, text)
					}
				}
				return true
			})
		}

		if text := strings.TrimSpace(choice.Get("text").String()); text != "" {
			parts = append(parts, text)
		}

		return true
	})
	return strings.Join(parts, "")
}

func extractResponsesResponseText(root gjson.Result) string {
	var parts []string
	root.Get("output").ForEach(func(_, item gjson.Result) bool {
		item.Get("content").ForEach(func(_, block gjson.Result) bool {
			text := strings.TrimSpace(block.Get("text").String())
			if text != "" {
				parts = append(parts, text)
			}

			return true
		})
		return true
	})
	return strings.Join(parts, "")
}

func extractCommonResponseText(root gjson.Result) string {
	if message := strings.TrimSpace(root.Get("error.message").String()); message != "" {
		return message
	}
	if text := strings.TrimSpace(root.Get("content").String()); text != "" && !strings.HasPrefix(text, "[") {
		return text
	}
	if text := strings.TrimSpace(root.Get("text").String()); text != "" {
		return text
	}
	return ""
}

func buildAvailabilityValidationError(body []byte, responseFormat string, expectedText string) string {
	actualText := strings.TrimSpace(extractAvailabilityResponseText(body, responseFormat))
	if actualText == "" {
		actualText = strings.TrimSpace(string(bytes.TrimSpace(body)))
	}
	actualText = truncateText(actualText, 160)
	if actualText == "" {
		return fmt.Sprintf("响应内容验证失败：期望包含 %q，但响应为空", expectedText)
	}
	return fmt.Sprintf("响应内容验证失败：期望包含 %q，实际返回 %q", expectedText, actualText)
}
