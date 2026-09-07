/*
@name: 供应商模型价格
@Descripttion: 获取供应商模型报价并保留价格字段存在性。
@version: 1.0.0
@Author: sm
@Date: 2026-09-07 11:20:00
@LastEditTime: 2026-09-07 11:20:00
@FilePath: services/provider_model_pricing.go
*/
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	modelpricing "codeswitch/resources/model-pricing"
)

type SiteType string

const (
	SiteTypeOneAPI     SiteType = "one-api"
	SiteTypeNewAPI     SiteType = "new-api"
	SiteTypeAnyRouter  SiteType = "anyrouter"
	SiteTypeVeloera    SiteType = "Veloera"
	SiteTypeOneHub     SiteType = "one-hub"
	SiteTypeDoneHub    SiteType = "done-hub"
	SiteTypeVoAPI      SiteType = "VoAPI"
	SiteTypeSuperAPI   SiteType = "Super-API"
	SiteTypeRixAPI     SiteType = "Rix-Api"
	SiteTypeNeoAPI     SiteType = "neo-Api"
	SiteTypeWongGongyi SiteType = "wong-gongyi"
	SiteTypeSub2API    SiteType = "sub2api"
	SiteTypeUnknown    SiteType = "unknown"
)

const (
	providerModelPricingSourceAuto       = "auto"
	providerModelPricingSourceCommon     = "api/pricing"
	providerModelPricingSourceOneHub     = "one-hub"
	providerModelPricingSourceOpenAIList = "v1/models"
	providerPricingDebugBodyLimit        = 64 * 1024
)

type ProviderModelPerCallPrice struct {
	Unified *float64 `json:"unified,omitempty"`
	Input   *float64 `json:"input,omitempty"`
	Output  *float64 `json:"output,omitempty"`
}

type ProviderModelPricingItem struct {
	PriceFieldsKnown              bool                       `json:"priceFieldsKnown"`
	HasInputPrice                 bool                       `json:"hasInputPrice"`
	HasOutputPrice                bool                       `json:"hasOutputPrice"`
	HasCacheCreatePrice           bool                       `json:"hasCacheCreatePrice"`
	HasCacheReadPrice             bool                       `json:"hasCacheReadPrice"`
	HasCacheCreate1hPrice         bool                       `json:"hasCacheCreate1hPrice"`
	Model                         string                     `json:"model"`
	DisplayName                   string                     `json:"displayName,omitempty"`
	CreatedAt                     string                     `json:"createdAt,omitempty"`
	MaxInputTokens                int64                      `json:"maxInputTokens,omitempty"`
	MaxTokens                     int64                      `json:"maxTokens,omitempty"`
	Capabilities                  map[string]interface{}     `json:"capabilities,omitempty"`
	Description                   string                     `json:"description,omitempty"`
	QuotaType                     int                        `json:"quotaType"` // 0=按量，1=按次
	ModelRatio                    float64                    `json:"modelRatio"`
	CompletionRatio               float64                    `json:"completionRatio"`
	GroupMultiplier               float64                    `json:"groupMultiplier,omitempty"`
	GroupMultiplierSource         string                     `json:"groupMultiplierSource,omitempty"`
	CacheCreateMultiplier         float64                    `json:"cacheCreateMultiplier,omitempty"`
	CacheReadMultiplier           float64                    `json:"cacheReadMultiplier,omitempty"`
	ResolvedCacheCreateMultiplier float64                    `json:"resolvedCacheCreateMultiplier,omitempty"`
	ResolvedCacheReadMultiplier   float64                    `json:"resolvedCacheReadMultiplier,omitempty"`
	CacheCreateMultiplierSource   string                     `json:"cacheCreateMultiplierSource,omitempty"`
	CacheReadMultiplierSource     string                     `json:"cacheReadMultiplierSource,omitempty"`
	OwnerBy                       string                     `json:"ownerBy,omitempty"`
	EnableGroups                  []string                   `json:"enableGroups,omitempty"`
	SupportedEndpointTypes        []string                   `json:"supportedEndpointTypes,omitempty"`
	InputUSDPerM                  float64                    `json:"inputUsdPerM,omitempty"`
	OutputUSDPerM                 float64                    `json:"outputUsdPerM,omitempty"`
	CacheCreate1hUSDPerM          float64                    `json:"cacheCreate1hUsdPerM,omitempty"`
	PerCallPrice                  *ProviderModelPerCallPrice `json:"perCallPrice,omitempty"`
}

type ProviderModelPricingResponse struct {
	SiteType          SiteType                   `json:"siteType"`
	PricingSource     string                     `json:"pricingSource"`
	GroupRatio        map[string]float64         `json:"groupRatio,omitempty"`
	UsableGroup       map[string]string          `json:"usableGroup,omitempty"`
	Models            []ProviderModelPricingItem `json:"models"`
	FetchError        string                     `json:"fetchError,omitempty"`
	Imported          bool                       `json:"imported,omitempty"`
	ChallengeDetected bool                       `json:"challengeDetected,omitempty"`
	ChallengeMessage  string                     `json:"challengeMessage,omitempty"`
	Debug             *ProviderModelPricingDebug `json:"debug,omitempty"`
}

type ProviderModelPricingDebug struct {
	BaseURL            string                             `json:"baseUrl"`
	Platform           string                             `json:"platform,omitempty"`
	RequestedSource    string                             `json:"requestedSource"`
	ResolvedSource     string                             `json:"resolvedSource,omitempty"`
	ConfiguredAuthType string                             `json:"configuredAuthType"`
	AuthCandidates     []string                           `json:"authCandidates,omitempty"`
	Attempts           []ProviderModelPricingDebugAttempt `json:"attempts,omitempty"`
}

type ProviderModelPricingDebugAttempt struct {
	Source                string            `json:"source"`
	Endpoint              string            `json:"endpoint"`
	Method                string            `json:"method"`
	URL                   string            `json:"url"`
	AuthType              string            `json:"authType"`
	RequestHeaders        map[string]string `json:"requestHeaders,omitempty"`
	StatusCode            int               `json:"statusCode,omitempty"`
	ResponseHeaders       map[string]string `json:"responseHeaders,omitempty"`
	ContentType           string            `json:"contentType,omitempty"`
	ResponseBody          string            `json:"responseBody,omitempty"`
	ResponseBodyBytes     int               `json:"responseBodyBytes,omitempty"`
	ResponseBodyTruncated bool              `json:"responseBodyTruncated,omitempty"`
	DurationMs            int64             `json:"durationMs,omitempty"`
	Error                 string            `json:"error,omitempty"`
}

type providerPricingResponse struct {
	Data        []providerModelPricing `json:"data"`
	GroupRatio  map[string]float64     `json:"group_ratio"`
	Success     bool                   `json:"success"`
	UsableGroup map[string]string      `json:"usable_group"`
}

type providerModelPricing struct {
	priceFields            map[string]bool
	directInputUSDPerM     *float64
	directOutputUSDPerM    *float64
	ModelName              string                 `json:"model_name"`
	ModelDescription       string                 `json:"model_description,omitempty"`
	QuotaType              int                    `json:"quota_type"`
	ModelRatio             float64                `json:"model_ratio"`
	ModelPrice             json.RawMessage        `json:"model_price"`
	CacheCreationRatio     float64                `json:"cache_creation_ratio"`
	CacheCreateRatio       float64                `json:"cache_create_ratio"`
	CacheCreateMultiplier  float64                `json:"cache_create_multiplier"`
	CacheRatio             float64                `json:"cache_ratio"`
	CacheReadRatio         float64                `json:"cache_read_ratio"`
	CacheReadMultiplier    float64                `json:"cache_read_multiplier"`
	OwnerBy                string                 `json:"owner_by,omitempty"`
	CompletionRatio        float64                `json:"completion_ratio"`
	EnableGroups           []string               `json:"enable_groups"`
	SupportedEndpointTypes []string               `json:"supported_endpoint_types"`
	DisplayName            string                 `json:"display_name,omitempty"`
	CreatedAt              string                 `json:"created_at,omitempty"`
	MaxInputTokens         int64                  `json:"max_input_tokens,omitempty"`
	MaxTokens              int64                  `json:"max_tokens,omitempty"`
	Capabilities           map[string]interface{} `json:"capabilities,omitempty"`
}

func (p *providerModelPricing) UnmarshalJSON(data []byte) error {
	type alias providerModelPricing
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*p = providerModelPricing(value)
	p.priceFields = make(map[string]bool)
	for name, raw := range fields {
		var number float64
		if string(raw) != "null" && json.Unmarshal(raw, &number) == nil && validProviderPrice(number) {
			p.priceFields[name] = true
		}
	}
	return nil
}

func validProviderPrice(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func (p providerModelPricing) hasPriceField(name string, value float64) bool {
	if p.priceFields != nil {
		return p.priceFields[name]
	}
	return value > 0 && validProviderPrice(value)
}

func (p providerModelPricing) cacheMultiplier(names []string, values []float64) (float64, bool) {
	for i, name := range names {
		if p.hasPriceField(name, values[i]) {
			return values[i], true
		}
	}
	return 0, false
}

type providerApiEnvelope[T any] struct {
	Success bool   `json:"success"`
	Data    T      `json:"data"`
	Message string `json:"message"`
}

type oneHubModelPricingItem struct {
	Groups  []string `json:"groups"`
	OwnedBy string   `json:"owned_by"`
	Price   struct {
		Type   string   `json:"type"` // tokens/times
		Input  *float64 `json:"input"`
		Output *float64 `json:"output"`
	} `json:"price"`
}

type oneHubModelPricing map[string]oneHubModelPricingItem

type oneHubUserGroupMap map[string]struct {
	Name  string  `json:"name"`
	Ratio float64 `json:"ratio"`
}

type openAIModelList struct {
	Data []struct {
		ID             string                 `json:"id"`
		DisplayName    string                 `json:"display_name"`
		CreatedAt      interface{}            `json:"created_at"`
		Created        int64                  `json:"created"`
		MaxInputTokens int64                  `json:"max_input_tokens"`
		MaxTokens      int64                  `json:"max_tokens"`
		Capabilities   map[string]interface{} `json:"capabilities"`
	} `json:"data"`
}

const doneHubTokenToCallRatio = 0.002

var providerPricingModelNameReplacer = strings.NewReplacer("-", "", "_", "", ".", "", ":", "", "/", "", " ", "")

type providerModelPricingCacheEntry struct {
	Response           *ProviderModelPricingResponse
	ModelsByNormalized map[string]ProviderModelPricingItem
	UpdatedAt          time.Time
}

type upstreamHTTPError struct {
	Endpoint   string
	StatusCode int
	Body       string
}

type providerPricingParseError struct {
	Endpoint    string
	StatusCode  int
	ContentType string
	Body        string
	Challenge   *providerPricingChallengeInfo
	Err         error
}

type providerPricingChallengeInfo struct {
	Type    string
	Message string
}

func (e *providerPricingParseError) Error() string {
	if e == nil {
		return "provider pricing parse error: <nil>"
	}
	if e.Challenge != nil && strings.TrimSpace(e.Challenge.Message) != "" {
		return fmt.Sprintf("%s（原始解析错误: %v）", e.Challenge.Message, e.Err)
	}
	message := fmt.Sprintf("解析 %s 失败: %v", e.Endpoint, e.Err)
	contentType := strings.TrimSpace(e.ContentType)
	if contentType != "" {
		message += fmt.Sprintf("（content-type: %s）", contentType)
	}
	if isLikelyHTMLResponse(contentType, e.Body) {
		message += "，上游返回的看起来是 HTML 页面而不是 JSON"
	}
	return message
}

func (e *providerPricingParseError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *upstreamHTTPError) Error() string {
	if e == nil {
		return "upstream http error: <nil>"
	}
	if challenge := detectProviderPricingChallenge(e.Endpoint, "", e.Body); challenge != nil && strings.TrimSpace(challenge.Message) != "" {
		return fmt.Sprintf("%s（HTTP %d）", challenge.Message, e.StatusCode)
	}
	body := strings.TrimSpace(e.Body)
	if body != "" {
		return fmt.Sprintf("%s HTTP %d: %s", e.Endpoint, e.StatusCode, truncateText(body, 2048))
	}
	return fmt.Sprintf("%s HTTP %d", e.Endpoint, e.StatusCode)
}

func isAuthStatusError(err error) bool {
	var httpErr *upstreamHTTPError
	if !errors.As(err, &httpErr) {
		return false
	}
	return httpErr.StatusCode == http.StatusUnauthorized || httpErr.StatusCode == http.StatusForbidden
}

func defaultProviderPricingAuthType(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "claude", "claude-code", "claude_code":
		return "x-api-key"
	default:
		return "bearer"
	}
}

func buildAuthCandidates(configured, platform string) []string {
	configured = strings.TrimSpace(configured)
	if strings.EqualFold(strings.TrimSpace(platform), "claude") ||
		strings.EqualFold(strings.TrimSpace(platform), "claude-code") ||
		strings.EqualFold(strings.TrimSpace(platform), "claude_code") {
		return []string{normalizeClaudeProviderAuthType(configured)}
	}
	candidates := make([]string, 0, 3)
	if configured != "" {
		candidates = append(candidates, configured)
	} else {
		candidates = append(candidates, defaultProviderPricingAuthType(platform))
	}

	hasBearer := false
	hasXApiKey := false
	for _, v := range candidates {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "bearer":
			hasBearer = true
		case "x-api-key":
			hasXApiKey = true
		}
	}

	if !hasBearer {
		candidates = append(candidates, "bearer")
	}
	if !hasXApiKey {
		candidates = append(candidates, "x-api-key")
	}

	return candidates
}

func normalizeProviderPricingDebugAuthType(authType string) string {
	authType = strings.TrimSpace(authType)
	if authType == "" {
		return "bearer"
	}
	return authType
}

func newProviderModelPricingDebug(apiURL, platform, requestedSource, configuredAuthType string, authCandidates []string) *ProviderModelPricingDebug {
	debug := &ProviderModelPricingDebug{
		BaseURL:            strings.TrimSpace(apiURL),
		Platform:           strings.TrimSpace(platform),
		RequestedSource:    strings.TrimSpace(requestedSource),
		ConfiguredAuthType: strings.TrimSpace(configuredAuthType),
	}
	if len(authCandidates) > 0 {
		debug.AuthCandidates = make([]string, 0, len(authCandidates))
		for _, candidate := range authCandidates {
			debug.AuthCandidates = append(debug.AuthCandidates, normalizeProviderPricingDebugAuthType(candidate))
		}
	}
	return debug
}

func appendProviderModelPricingDebugAttempt(debug *ProviderModelPricingDebug, attempt *ProviderModelPricingDebugAttempt) {
	if debug == nil || attempt == nil {
		return
	}
	debug.Attempts = append(debug.Attempts, *attempt)
}

func newProviderModelPricingDebugAttempt(source, endpoint, method, url, authType string) *ProviderModelPricingDebugAttempt {
	return &ProviderModelPricingDebugAttempt{
		Source:   strings.TrimSpace(source),
		Endpoint: strings.TrimSpace(endpoint),
		Method:   strings.TrimSpace(method),
		URL:      strings.TrimSpace(url),
		AuthType: normalizeProviderPricingDebugAuthType(authType),
	}
}

func isSensitiveProviderPricingHeader(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(name, "authorization") ||
		strings.Contains(name, "api-key") ||
		strings.Contains(name, "token") ||
		strings.Contains(name, "secret")
}

func maskProviderPricingSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= 2 {
		return strings.Repeat("*", len(runes))
	}
	if len(runes) <= 8 {
		return string(runes[:1]) + "***" + string(runes[len(runes)-1:])
	}
	return string(runes[:4]) + "***" + string(runes[len(runes)-4:])
}

func maskProviderPricingHeaderValue(name, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(name), "Authorization") {
		parts := strings.Fields(trimmed)
		if len(parts) >= 2 {
			return parts[0] + " " + maskProviderPricingSecret(strings.Join(parts[1:], " "))
		}
	}
	return maskProviderPricingSecret(trimmed)
}

func buildProviderPricingHeaderSnapshot(header http.Header) map[string]string {
	if len(header) == 0 {
		return nil
	}
	keys := make([]string, 0, len(header))
	for key := range header {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	snapshot := make(map[string]string, len(keys))
	for _, key := range keys {
		values := header.Values(key)
		if len(values) == 0 {
			continue
		}
		joined := strings.Join(values, ", ")
		if isSensitiveProviderPricingHeader(key) {
			snapshot[key] = maskProviderPricingHeaderValue(key, joined)
			continue
		}
		snapshot[key] = truncateText(strings.ToValidUTF8(joined, "?"), 1024)
	}
	if len(snapshot) == 0 {
		return nil
	}
	return snapshot
}

func setProviderPricingDebugResponse(attempt *ProviderModelPricingDebugAttempt, resp *http.Response, body []byte, duration time.Duration) {
	if attempt == nil {
		return
	}
	attempt.DurationMs = duration.Milliseconds()
	if resp == nil {
		return
	}
	attempt.StatusCode = resp.StatusCode
	attempt.ResponseHeaders = buildProviderPricingHeaderSnapshot(resp.Header)
	attempt.ContentType = strings.TrimSpace(resp.Header.Get("Content-Type"))

	if len(body) == 0 {
		return
	}

	bodyText := strings.ToValidUTF8(string(body), "?")
	attempt.ResponseBodyBytes = len(body)
	attempt.ResponseBodyTruncated = len(bodyText) > providerPricingDebugBodyLimit
	attempt.ResponseBody = truncateText(bodyText, providerPricingDebugBodyLimit)
}

func isLikelyHTMLResponse(contentType, body string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml") {
		return true
	}
	trimmed := strings.TrimSpace(strings.ToLower(body))
	return strings.HasPrefix(trimmed, "<!doctype html") ||
		strings.HasPrefix(trimmed, "<html") ||
		strings.HasPrefix(trimmed, "<head") ||
		strings.HasPrefix(trimmed, "<body") ||
		strings.HasPrefix(trimmed, "<")
}

func normalizeProviderPricingChallengeEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "当前接口"
	}
	return endpoint
}

func buildProviderPricingChallengeFollowup(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" || endpoint == "/api/pricing" {
		return "请先在浏览器中打开 /api/pricing 完成验证，再使用“粘贴 JSON 导入”导入返回结果。"
	}
	return fmt.Sprintf("请先在浏览器中打开 %s 完成验证，确认它返回的是 JSON 而不是挑战页。", endpoint)
}

func buildProviderPricingChallengeMessage(endpoint, challengeType string) string {
	endpointLabel := normalizeProviderPricingChallengeEndpoint(endpoint)
	followup := buildProviderPricingChallengeFollowup(endpoint)
	switch strings.TrimSpace(challengeType) {
	case "acw_sc__v2":
		return fmt.Sprintf("检测到浏览器挑战页：%s 返回了 acw_sc__v2 挑战，自动绕过未能完成，请在浏览器中手动完成验证。%s", endpointLabel, followup)
	default:
		return fmt.Sprintf("检测到浏览器挑战页：%s 要求客户端先执行页面脚本并回写挑战 Cookie，非浏览器请求无法直接拿到 JSON。%s", endpointLabel, followup)
	}
}

func detectProviderPricingChallenge(endpoint, contentType, body string) *providerPricingChallengeInfo {
	normalizedBody := strings.ToLower(strings.TrimSpace(body))
	if normalizedBody == "" {
		return nil
	}

	looksLikeMarkupOrScript := isLikelyHTMLResponse(contentType, body) ||
		strings.Contains(normalizedBody, "<script") ||
		strings.Contains(normalizedBody, "var arg1=")

	if strings.Contains(normalizedBody, "acw_sc__v2") && looksLikeMarkupOrScript {
		return &providerPricingChallengeInfo{
			Type:    "acw_sc__v2",
			Message: buildProviderPricingChallengeMessage(endpoint, "acw_sc__v2"),
		}
	}

	if strings.Contains(normalizedBody, "acw_sc__v2") &&
		strings.Contains(normalizedBody, "document.cookie") &&
		strings.Contains(normalizedBody, "location.reload") {
		return &providerPricingChallengeInfo{
			Type:    "acw_sc__v2",
			Message: buildProviderPricingChallengeMessage(endpoint, "acw_sc__v2"),
		}
	}

	if isLikelyHTMLResponse(contentType, body) &&
		strings.Contains(normalizedBody, "document.cookie") &&
		strings.Contains(normalizedBody, "location.reload") {
		return &providerPricingChallengeInfo{
			Type:    "browser-js-cookie",
			Message: buildProviderPricingChallengeMessage(endpoint, "browser-js-cookie"),
		}
	}

	return nil
}

func shouldRetryWithNextAuthCandidate(err error) bool {
	if err == nil {
		return false
	}
	if isAuthStatusError(err) {
		return true
	}
	var parseErr *providerPricingParseError
	return errors.As(err, &parseErr) && isLikelyHTMLResponse(parseErr.ContentType, parseErr.Body)
}

func attachProviderModelPricingDebug(response *ProviderModelPricingResponse, debug *ProviderModelPricingDebug) *ProviderModelPricingResponse {
	if response == nil {
		return nil
	}
	if response.Models == nil {
		response.Models = []ProviderModelPricingItem{}
	}
	response.Debug = debug
	if debug != nil && strings.TrimSpace(response.PricingSource) != "" {
		debug.ResolvedSource = response.PricingSource
	}
	return response
}

func buildProviderModelPricingFailureResponse(source, message string, debug *ProviderModelPricingDebug) *ProviderModelPricingResponse {
	response := &ProviderModelPricingResponse{
		SiteType:      SiteTypeUnknown,
		PricingSource: strings.TrimSpace(source),
		Models:        []ProviderModelPricingItem{},
		FetchError:    strings.TrimSpace(message),
	}
	return attachProviderModelPricingDebug(response, debug)
}

func applyProviderModelPricingFailureHints(response *ProviderModelPricingResponse, cause error) *ProviderModelPricingResponse {
	if response == nil || cause == nil {
		return response
	}

	var parseErr *providerPricingParseError
	if errors.As(cause, &parseErr) && parseErr != nil && parseErr.Challenge != nil {
		return applyProviderModelPricingChallenge(response, parseErr.Challenge)
	}

	var httpErr *upstreamHTTPError
	if errors.As(cause, &httpErr) && httpErr != nil {
		if challenge := detectProviderPricingChallenge(httpErr.Endpoint, "", httpErr.Body); challenge != nil {
			return applyProviderModelPricingChallenge(response, challenge)
		}
	}

	return response
}

func buildProviderModelPricingFailureResponseWithCause(source, message string, debug *ProviderModelPricingDebug, cause error) *ProviderModelPricingResponse {
	return applyProviderModelPricingFailureHints(
		buildProviderModelPricingFailureResponse(source, message, debug),
		cause,
	)
}

func applyProviderModelPricingChallenge(response *ProviderModelPricingResponse, challenge *providerPricingChallengeInfo) *ProviderModelPricingResponse {
	if response == nil || challenge == nil {
		return response
	}
	response.ChallengeDetected = true
	response.ChallengeMessage = strings.TrimSpace(challenge.Message)
	if response.ChallengeMessage != "" {
		response.FetchError = response.ChallengeMessage
	}
	return response
}

func providerPricingCacheKey(apiURL, apiKey, authType string) string {
	url := strings.TrimSpace(strings.ToLower(apiURL))
	key := strings.TrimSpace(apiKey)
	auth := strings.TrimSpace(strings.ToLower(authType))
	if url == "" || key == "" {
		return ""
	}
	return url + "|" + key + "|" + auth
}

func normalizeProviderPricingModelName(name string) string {
	return providerPricingModelNameReplacer.Replace(strings.ToLower(strings.TrimSpace(name)))
}

func buildProviderModelPricingIndex(models []ProviderModelPricingItem) map[string]ProviderModelPricingItem {
	index := make(map[string]ProviderModelPricingItem, len(models))
	for _, item := range models {
		modelName := strings.TrimSpace(item.Model)
		if modelName == "" {
			continue
		}
		normalized := normalizeProviderPricingModelName(modelName)
		if normalized == "" {
			continue
		}
		if _, exists := index[normalized]; exists {
			continue
		}
		index[normalized] = item
	}
	return index
}

func providerPricingMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func providerPricingMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func firstPositiveFloat(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func normalizeProviderModelPricingSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "", providerModelPricingSourceAuto:
		return providerModelPricingSourceAuto
	case providerModelPricingSourceCommon, "/api/pricing", "pricing":
		return providerModelPricingSourceCommon
	case providerModelPricingSourceOneHub, "one_hub", "onehub", "/api/available_model", "available_model":
		return providerModelPricingSourceOneHub
	case providerModelPricingSourceOpenAIList, "/v1/models", "models":
		return providerModelPricingSourceOpenAIList
	default:
		return ""
	}
}

func shouldClearProviderModelPricingCacheOnFailure(source string) bool {
	return normalizeProviderModelPricingSource(source) == providerModelPricingSourceAuto
}

func shouldCacheProviderModelPricingResponse(requestedSource string, response *ProviderModelPricingResponse) bool {
	if response == nil {
		return false
	}

	// 只有 auto 探测路径会回填共享缓存，避免用户在弹窗里手动切换来源时
	// 污染日志计价等依赖该缓存的系统行为。
	switch normalizeProviderModelPricingSource(requestedSource) {
	case "", providerModelPricingSourceAuto:
		return true
	default:
		return false
	}
}

func (ps *ProviderService) cacheProviderModelPricing(apiURL, apiKey, authType string, response *ProviderModelPricingResponse) {
	if ps == nil || response == nil {
		return
	}
	key := providerPricingCacheKey(apiURL, apiKey, authType)
	if key == "" {
		return
	}

	ps.pricingCacheMu.Lock()
	defer ps.pricingCacheMu.Unlock()
	ps.pricingCache[key] = providerModelPricingCacheEntry{
		Response:           response,
		ModelsByNormalized: buildProviderModelPricingIndex(response.Models),
		UpdatedAt:          time.Now(),
	}
}

func (ps *ProviderService) clearProviderModelPricingCache(apiURL, apiKey, authType string) {
	if ps == nil {
		return
	}
	key := providerPricingCacheKey(apiURL, apiKey, authType)
	if key == "" {
		return
	}

	ps.pricingCacheMu.Lock()
	defer ps.pricingCacheMu.Unlock()
	delete(ps.pricingCache, key)
}

func (ps *ProviderService) enrichProviderModelPricingResponse(response *ProviderModelPricingResponse, apiURL, apiKey, authType string) {
	if ps == nil || response == nil || len(response.Models) == 0 {
		return
	}

	var pricing *modelpricing.Service
	ps.mu.Lock()
	if ps.modelPricing != nil {
		pricing = ps.modelPricing.Service()
	}
	ps.mu.Unlock()

	for index := range response.Models {
		item := &response.Models[index]
		if pricing != nil {
			if entry, ok := pricing.PricingEntryExact(item.Model); ok {
				if item.MaxInputTokens <= 0 {
					item.MaxInputTokens = entry.MaxInputTokens
				}
				if item.MaxTokens <= 0 {
					item.MaxTokens = entry.MaxTokens
				}
				if len(item.Capabilities) == 0 {
					item.Capabilities = pricingEntryCapabilities(entry)
				}
			}
		}
		groupMultiplier, groupSource := resolveProviderGroupMultiplierDetails(ps, apiURL, apiKey, authType, item.Model)
		item.GroupMultiplier = groupMultiplier
		item.GroupMultiplierSource = groupSource
		if item.QuotaType != 0 {
			continue
		}
		cacheCreateMultiplier, cacheCreateSource, cacheReadMultiplier, cacheReadSource := resolveProviderCacheMultiplierDetails(
			ps,
			apiURL,
			apiKey,
			authType,
			*item,
			pricing,
			item.Model,
		)
		item.ResolvedCacheCreateMultiplier = cacheCreateMultiplier
		item.ResolvedCacheReadMultiplier = cacheReadMultiplier
		item.CacheCreateMultiplierSource = cacheCreateSource
		item.CacheReadMultiplierSource = cacheReadSource

		cacheCreate1hPerToken := resolveExplicitProviderEphemeral1hPerToken(pricing, item.Model)
		if cacheCreate1hPerToken > 0 {
			item.CacheCreate1hUSDPerM = cacheCreate1hPerToken * 1_000_000
		}
	}
}

func pricingEntryCapabilities(entry modelpricing.PricingEntry) map[string]interface{} {
	capabilities := map[string]interface{}{}
	values := map[string]bool{
		"computer_use":       entry.SupportsComputerUse,
		"tool_use":           entry.SupportsFunctionCalling,
		"pdf_input":          entry.SupportsPDFInput,
		"prompt_caching":     entry.SupportsPromptCaching,
		"thinking":           entry.SupportsReasoning,
		"structured_outputs": entry.SupportsResponseSchema,
		"image_input":        entry.SupportsVision,
	}
	for key, supported := range values {
		if supported {
			capabilities[key] = map[string]interface{}{"supported": true}
		}
	}
	return capabilities
}

// ResolveCachedProviderModelPricing 尝试从本地缓存里按模型名获取供应商接口价格条目。
// 匹配顺序：精确归一化匹配 -> 包含关系的相似匹配。
func (ps *ProviderService) ResolveCachedProviderModelPricing(apiURL, apiKey, authType, model string) (ProviderModelPricingItem, bool) {
	if ps == nil {
		return ProviderModelPricingItem{}, false
	}

	key := providerPricingCacheKey(apiURL, apiKey, authType)
	if key == "" {
		return ProviderModelPricingItem{}, false
	}

	modelNorm := normalizeProviderPricingModelName(model)
	if modelNorm == "" {
		return ProviderModelPricingItem{}, false
	}

	ps.pricingCacheMu.RLock()
	entry, ok := ps.pricingCache[key]
	ps.pricingCacheMu.RUnlock()
	if !ok || len(entry.ModelsByNormalized) == 0 {
		return ProviderModelPricingItem{}, false
	}

	if item, exists := entry.ModelsByNormalized[modelNorm]; exists {
		return item, true
	}

	bestScore := -1.0
	bestItem := ProviderModelPricingItem{}
	for cachedNorm, item := range entry.ModelsByNormalized {
		if cachedNorm == "" {
			continue
		}
		if !(strings.Contains(cachedNorm, modelNorm) || strings.Contains(modelNorm, cachedNorm)) {
			continue
		}

		maxLen := providerPricingMax(len(cachedNorm), len(modelNorm))
		if maxLen <= 0 {
			continue
		}
		score := float64(providerPricingMin(len(cachedNorm), len(modelNorm))) / float64(maxLen)
		if score > bestScore {
			bestScore = score
			bestItem = item
		}
	}

	if bestScore < 0 {
		return ProviderModelPricingItem{}, false
	}
	return bestItem, true
}

// FetchProviderModelPricing 获取单个供应商的模型列表与价格信息。
// 该实现参考 all-api-hub 的站点适配逻辑：
// - 默认：GET /api/pricing
// - one-hub/done-hub：GET /api/available_model + /api/user_group_map
// - 均失败时兜底：GET /v1/models（仅模型名，无价格）
func (ps *ProviderService) FetchProviderModelPricing(apiURL, apiKey, platform, authType string) (*ProviderModelPricingResponse, error) {
	return ps.FetchProviderModelPricingWithSource(apiURL, apiKey, platform, authType, providerModelPricingSourceAuto)
}

func (ps *ProviderService) FetchProviderModelPricingWithSource(apiURL, apiKey, platform, authType, source string) (*ProviderModelPricingResponse, error) {
	requestedSource := strings.TrimSpace(source)
	apiURL = strings.TrimSpace(apiURL)
	platform = strings.TrimSpace(platform)
	source = normalizeProviderModelPricingSource(source)
	authCandidates := buildAuthCandidates(authType, platform)
	debug := newProviderModelPricingDebug(apiURL, platform, source, authType, authCandidates)
	if requestedSource != "" && source == "" {
		debug.RequestedSource = requestedSource
	}

	if apiURL == "" {
		ps.clearProviderModelPricingCache(apiURL, apiKey, authType)
		return buildProviderModelPricingFailureResponse(source, "apiUrl 不能为空", debug), nil
	}

	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		ps.clearProviderModelPricingCache(apiURL, apiKey, authType)
		return buildProviderModelPricingFailureResponse(source, "apiKey 不能为空", debug), nil
	}

	client := &http.Client{Timeout: 20 * time.Second}
	if source == "" {
		return buildProviderModelPricingFailureResponse(requestedSource, "不支持的数据来源", debug), nil
	}

	finalizeResponse := func(response *ProviderModelPricingResponse) *ProviderModelPricingResponse {
		ps.enrichProviderModelPricingResponse(response, apiURL, apiKey, authType)
		if shouldCacheProviderModelPricingResponse(source, response) {
			ps.cacheProviderModelPricing(apiURL, apiKey, authType, response)
		}
		return attachProviderModelPricingDebug(response, debug)
	}

	fetchFromOpenAIModelList := func() (*ProviderModelPricingResponse, error) {
		var modelErr error
		for _, candidate := range authCandidates {
			items, err := fetchOpenAIModels(client, apiURL, apiKey, candidate, debug)
			if err == nil && len(items) > 0 {
				sort.Slice(items, func(i, j int) bool {
					return items[i].Model < items[j].Model
				})

				return finalizeResponse(&ProviderModelPricingResponse{
					SiteType:      SiteTypeUnknown,
					PricingSource: providerModelPricingSourceOpenAIList,
					Models:        items,
				}), nil
			}

			modelErr = err
			if shouldRetryWithNextAuthCandidate(modelErr) {
				continue
			}
			break
		}
		return nil, modelErr
	}

	fetchFromCommonPricing := func() (*ProviderModelPricingResponse, error) {
		var commonErr error
		for _, candidate := range authCandidates {
			commonPricing, err := fetchCommonPricing(client, apiURL, apiKey, candidate, debug)
			if err == nil {
				return finalizeResponse(buildProviderModelPricingResponse(
					SiteTypeUnknown,
					providerModelPricingSourceCommon,
					commonPricing,
				)), nil
			}
			commonErr = err
			if shouldRetryWithNextAuthCandidate(commonErr) {
				continue
			}
			break
		}
		return nil, commonErr
	}

	fetchFromOneHubPricing := func() (*ProviderModelPricingResponse, error) {
		var oneHubErr error
		for _, candidate := range authCandidates {
			oneHubPricing, err := fetchOneHubPricing(client, apiURL, apiKey, candidate, debug)
			if err == nil {
				return finalizeResponse(buildProviderModelPricingResponse(
					SiteTypeOneHub,
					providerModelPricingSourceOneHub,
					oneHubPricing,
				)), nil
			}
			oneHubErr = err
			if shouldRetryWithNextAuthCandidate(oneHubErr) {
				continue
			}
			break
		}
		return nil, oneHubErr
	}

	switch source {
	case providerModelPricingSourceCommon:
		response, err := fetchFromCommonPricing()
		if err != nil {
			if shouldClearProviderModelPricingCacheOnFailure(source) {
				ps.clearProviderModelPricingCache(apiURL, apiKey, authType)
			}
			return buildProviderModelPricingFailureResponseWithCause(
				source,
				fmt.Sprintf("通过 %s 获取模型定价失败: %v", providerModelPricingSourceCommon, err),
				debug,
				err,
			), nil
		}
		return response, nil
	case providerModelPricingSourceOneHub:
		response, err := fetchFromOneHubPricing()
		if err != nil {
			if shouldClearProviderModelPricingCacheOnFailure(source) {
				ps.clearProviderModelPricingCache(apiURL, apiKey, authType)
			}
			return buildProviderModelPricingFailureResponseWithCause(
				source,
				fmt.Sprintf("通过 %s 获取模型定价失败: %v", providerModelPricingSourceOneHub, err),
				debug,
				err,
			), nil
		}
		return response, nil
	case providerModelPricingSourceOpenAIList:
		response, err := fetchFromOpenAIModelList()
		if err != nil {
			if shouldClearProviderModelPricingCacheOnFailure(source) {
				ps.clearProviderModelPricingCache(apiURL, apiKey, authType)
			}
			return buildProviderModelPricingFailureResponseWithCause(
				source,
				fmt.Sprintf("通过 %s 获取模型列表失败: %v", providerModelPricingSourceOpenAIList, err),
				debug,
				err,
			), nil
		}
		return response, nil
	}

	var commonErr error
	var oneHubErr error
	for _, candidate := range authCandidates {
		commonPricing, err := fetchCommonPricing(client, apiURL, apiKey, candidate, debug)
		if err == nil {
			return finalizeResponse(buildProviderModelPricingResponse(
				SiteTypeUnknown,
				providerModelPricingSourceCommon,
				commonPricing,
			)), nil
		}
		commonErr = err

		oneHubPricing, err := fetchOneHubPricing(client, apiURL, apiKey, candidate, debug)
		if err == nil {
			return finalizeResponse(buildProviderModelPricingResponse(
				SiteTypeOneHub,
				providerModelPricingSourceOneHub,
				oneHubPricing,
			)), nil
		}
		oneHubErr = err

		if shouldRetryWithNextAuthCandidate(commonErr) || shouldRetryWithNextAuthCandidate(oneHubErr) {
			continue
		}
		break
	}

	// 兜底：尝试 /v1/models
	response, modelErr := fetchFromOpenAIModelList()
	if modelErr == nil {
		return response, nil
	}

	if commonErr != nil || oneHubErr != nil {
		if shouldClearProviderModelPricingCacheOnFailure(source) {
			ps.clearProviderModelPricingCache(apiURL, apiKey, authType)
		}
		return buildProviderModelPricingFailureResponseWithCause(
			source,
			fmt.Sprintf("获取模型定价失败：%v；%v", commonErr, oneHubErr),
			debug,
			errors.Join(commonErr, oneHubErr),
		), nil
	}
	if shouldClearProviderModelPricingCacheOnFailure(source) {
		ps.clearProviderModelPricingCache(apiURL, apiKey, authType)
	}
	return buildProviderModelPricingFailureResponseWithCause(
		source,
		fmt.Sprintf("通过 %s 获取模型列表失败: %v", providerModelPricingSourceOpenAIList, modelErr),
		debug,
		modelErr,
	), nil
}

func (ps *ProviderService) ImportProviderModelPricingJSON(apiURL, apiKey, platform, authType, raw string) (*ProviderModelPricingResponse, error) {
	apiURL = strings.TrimSpace(apiURL)
	apiKey = strings.TrimSpace(apiKey)
	platform = strings.TrimSpace(platform)
	raw = strings.TrimSpace(raw)
	authCandidates := buildAuthCandidates(authType, platform)
	debug := newProviderModelPricingDebug(apiURL, platform, providerModelPricingSourceCommon, authType, authCandidates)
	if raw == "" {
		return buildProviderModelPricingFailureResponse(
			providerModelPricingSourceCommon,
			"请先粘贴 /api/pricing 返回的 JSON 内容",
			debug,
		), nil
	}

	targetURL := "/api/pricing"
	if apiURL != "" {
		targetURL = joinURL(apiURL, "/api/pricing")
	}
	attempt := newProviderModelPricingDebugAttempt(providerModelPricingSourceCommon, "/api/pricing", "PASTE", targetURL, authType)
	attempt.ContentType = "application/json (manual import)"
	attempt.ResponseBodyBytes = len(raw)
	attempt.ResponseBodyTruncated = len(raw) > providerPricingDebugBodyLimit
	attempt.ResponseBody = truncateText(strings.ToValidUTF8(raw, "?"), providerPricingDebugBodyLimit)

	if challenge := detectProviderPricingChallenge("/api/pricing", attempt.ContentType, raw); challenge != nil {
		attempt.Error = challenge.Message
		appendProviderModelPricingDebugAttempt(debug, attempt)
		return applyProviderModelPricingChallenge(
			buildProviderModelPricingFailureResponse(providerModelPricingSourceCommon, challenge.Message, debug),
			challenge,
		), nil
	}

	var pricing providerPricingResponse
	if err := json.Unmarshal([]byte(raw), &pricing); err != nil {
		parseErr := &providerPricingParseError{
			Endpoint:    "/api/pricing",
			ContentType: attempt.ContentType,
			Body:        raw,
			Challenge:   detectProviderPricingChallenge("/api/pricing", attempt.ContentType, raw),
			Err:         err,
		}
		attempt.Error = parseErr.Error()
		appendProviderModelPricingDebugAttempt(debug, attempt)
		return buildProviderModelPricingFailureResponseWithCause(
			providerModelPricingSourceCommon,
			fmt.Sprintf("导入 /api/pricing JSON 失败: %v", parseErr),
			debug,
			parseErr,
		), nil
	}
	if !pricing.Success {
		attempt.Error = "/api/pricing 返回 success=false"
		appendProviderModelPricingDebugAttempt(debug, attempt)
		return buildProviderModelPricingFailureResponse(
			providerModelPricingSourceCommon,
			"/api/pricing 返回 success=false",
			debug,
		), nil
	}

	attempt.StatusCode = http.StatusOK
	appendProviderModelPricingDebugAttempt(debug, attempt)

	response := buildProviderModelPricingResponse(
		SiteTypeUnknown,
		providerModelPricingSourceCommon,
		&pricing,
	)
	ps.enrichProviderModelPricingResponse(response, apiURL, apiKey, authType)
	response.Imported = true
	return attachProviderModelPricingDebug(response, debug), nil
}

func fetchCommonPricing(client *http.Client, apiURL, apiKey, authType string, debug *ProviderModelPricingDebug) (*providerPricingResponse, error) {
	targetURL := joinURL(apiURL, "/api/pricing")
	attempt := newProviderModelPricingDebugAttempt(providerModelPricingSourceCommon, "/api/pricing", "GET", targetURL, authType)
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		attempt.Error = err.Error()
		appendProviderModelPricingDebugAttempt(debug, attempt)
		return nil, err
	}
	attachAuthHeader(req, apiKey, authType)
	req.Header.Set("Accept", "application/json")
	attempt.RequestHeaders = buildProviderPricingHeaderSnapshot(req.Header)

	startedAt := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		attempt.DurationMs = time.Since(startedAt).Milliseconds()
		attempt.Error = err.Error()
		appendProviderModelPricingDebugAttempt(debug, attempt)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	setProviderPricingDebugResponse(attempt, resp, body, time.Since(startedAt))
	if err != nil {
		attempt.Error = err.Error()
		appendProviderModelPricingDebugAttempt(debug, attempt)
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		httpErr := &upstreamHTTPError{
			Endpoint:   "/api/pricing",
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
		attempt.Error = httpErr.Error()
		appendProviderModelPricingDebugAttempt(debug, attempt)
		return nil, httpErr
	}

	var pricing providerPricingResponse
	if err := json.Unmarshal(body, &pricing); err != nil {
		// 尝试自动绕过 acw_sc__v2 WAF 挑战
		challenge := detectProviderPricingChallenge("/api/pricing", attempt.ContentType, string(body))
		if challenge != nil && challenge.Type == "acw_sc__v2" {
			if wafResult, wafErr := tryAcwBypass(client, targetURL, apiKey, authType, string(body), debug); wafErr == nil {
				attempt.Error = fmt.Sprintf("JSON 解析失败 (检测到 acw_sc__v2 挑战, 自动绕过成功): %v", err)
				appendProviderModelPricingDebugAttempt(debug, attempt)
				return wafResult, nil
			} else {
				// 绕过失败 → 更新提示文案，告知用户自动绕过已尝试但失败
				endpointLabel := normalizeProviderPricingChallengeEndpoint("/api/pricing")
				followup := buildProviderPricingChallengeFollowup("/api/pricing")
				challenge.Message = fmt.Sprintf("检测到浏览器挑战页：%s 返回了 acw_sc__v2 挑战，已尝试自动绕过但失败（%v），请在浏览器中手动完成验证。%s", endpointLabel, wafErr, followup)
			}
		}

		parseErr := &providerPricingParseError{
			Endpoint:    "/api/pricing",
			StatusCode:  resp.StatusCode,
			ContentType: attempt.ContentType,
			Body:        string(body),
			Challenge:   challenge,
			Err:         err,
		}
		attempt.Error = parseErr.Error()
		appendProviderModelPricingDebugAttempt(debug, attempt)
		return nil, parseErr
	}
	if !pricing.Success {
		apiErr := fmt.Errorf("/api/pricing 返回 success=false")
		attempt.Error = apiErr.Error()
		appendProviderModelPricingDebugAttempt(debug, attempt)
		return nil, apiErr
	}
	appendProviderModelPricingDebugAttempt(debug, attempt)
	return &pricing, nil
}

func fetchOneHubPricing(client *http.Client, apiURL, apiKey, authType string, debug *ProviderModelPricingDebug) (*providerPricingResponse, error) {
	availableURL := joinURL(apiURL, "/api/available_model")
	groupURL := joinURL(apiURL, "/api/user_group_map")
	availableAttempt := newProviderModelPricingDebugAttempt(providerModelPricingSourceOneHub, "/api/available_model", "GET", availableURL, authType)
	groupAttempt := newProviderModelPricingDebugAttempt(providerModelPricingSourceOneHub, "/api/user_group_map", "GET", groupURL, authType)

	availableReq, err := http.NewRequest("GET", availableURL, nil)
	if err != nil {
		availableAttempt.Error = err.Error()
		appendProviderModelPricingDebugAttempt(debug, availableAttempt)
		return nil, err
	}
	attachAuthHeader(availableReq, apiKey, authType)
	availableReq.Header.Set("Accept", "application/json")
	availableAttempt.RequestHeaders = buildProviderPricingHeaderSnapshot(availableReq.Header)

	groupReq, err := http.NewRequest("GET", groupURL, nil)
	if err != nil {
		groupAttempt.Error = err.Error()
		appendProviderModelPricingDebugAttempt(debug, groupAttempt)
		return nil, err
	}
	attachAuthHeader(groupReq, apiKey, authType)
	groupReq.Header.Set("Accept", "application/json")
	groupAttempt.RequestHeaders = buildProviderPricingHeaderSnapshot(groupReq.Header)

	availableStartedAt := time.Now()
	availableResp, err := client.Do(availableReq)
	if err != nil {
		availableAttempt.DurationMs = time.Since(availableStartedAt).Milliseconds()
		availableAttempt.Error = err.Error()
		appendProviderModelPricingDebugAttempt(debug, availableAttempt)
		return nil, err
	}
	defer availableResp.Body.Close()

	availableBody, err := io.ReadAll(io.LimitReader(availableResp.Body, 8*1024*1024))
	setProviderPricingDebugResponse(availableAttempt, availableResp, availableBody, time.Since(availableStartedAt))
	if err != nil {
		availableAttempt.Error = err.Error()
		appendProviderModelPricingDebugAttempt(debug, availableAttempt)
		return nil, err
	}
	if availableResp.StatusCode < 200 || availableResp.StatusCode >= 300 {
		httpErr := &upstreamHTTPError{
			Endpoint:   "/api/available_model",
			StatusCode: availableResp.StatusCode,
			Body:       string(availableBody),
		}
		availableAttempt.Error = httpErr.Error()
		appendProviderModelPricingDebugAttempt(debug, availableAttempt)
		return nil, httpErr
	}

	var availableEnvelope providerApiEnvelope[oneHubModelPricing]
	if err := json.Unmarshal(availableBody, &availableEnvelope); err != nil {
		parseErr := &providerPricingParseError{
			Endpoint:    "/api/available_model",
			StatusCode:  availableResp.StatusCode,
			ContentType: availableAttempt.ContentType,
			Body:        string(availableBody),
			Challenge:   detectProviderPricingChallenge("/api/available_model", availableAttempt.ContentType, string(availableBody)),
			Err:         err,
		}
		availableAttempt.Error = parseErr.Error()
		appendProviderModelPricingDebugAttempt(debug, availableAttempt)
		return nil, parseErr
	}
	if !availableEnvelope.Success {
		apiErr := fmt.Errorf("/api/available_model 返回 success=false")
		availableAttempt.Error = apiErr.Error()
		appendProviderModelPricingDebugAttempt(debug, availableAttempt)
		return nil, apiErr
	}
	appendProviderModelPricingDebugAttempt(debug, availableAttempt)

	groupStartedAt := time.Now()
	groupResp, err := client.Do(groupReq)
	if err != nil {
		groupAttempt.DurationMs = time.Since(groupStartedAt).Milliseconds()
		groupAttempt.Error = err.Error()
		appendProviderModelPricingDebugAttempt(debug, groupAttempt)
		return nil, err
	}
	defer groupResp.Body.Close()

	groupBody, err := io.ReadAll(io.LimitReader(groupResp.Body, 8*1024*1024))
	setProviderPricingDebugResponse(groupAttempt, groupResp, groupBody, time.Since(groupStartedAt))
	if err != nil {
		groupAttempt.Error = err.Error()
		appendProviderModelPricingDebugAttempt(debug, groupAttempt)
		return nil, err
	}
	if groupResp.StatusCode < 200 || groupResp.StatusCode >= 300 {
		httpErr := &upstreamHTTPError{
			Endpoint:   "/api/user_group_map",
			StatusCode: groupResp.StatusCode,
			Body:       string(groupBody),
		}
		groupAttempt.Error = httpErr.Error()
		appendProviderModelPricingDebugAttempt(debug, groupAttempt)
		return nil, httpErr
	}

	var groupEnvelope providerApiEnvelope[oneHubUserGroupMap]
	if err := json.Unmarshal(groupBody, &groupEnvelope); err != nil {
		parseErr := &providerPricingParseError{
			Endpoint:    "/api/user_group_map",
			StatusCode:  groupResp.StatusCode,
			ContentType: groupAttempt.ContentType,
			Body:        string(groupBody),
			Challenge:   detectProviderPricingChallenge("/api/user_group_map", groupAttempt.ContentType, string(groupBody)),
			Err:         err,
		}
		groupAttempt.Error = parseErr.Error()
		appendProviderModelPricingDebugAttempt(debug, groupAttempt)
		return nil, parseErr
	}
	if !groupEnvelope.Success {
		apiErr := fmt.Errorf("/api/user_group_map 返回 success=false")
		groupAttempt.Error = apiErr.Error()
		appendProviderModelPricingDebugAttempt(debug, groupAttempt)
		return nil, apiErr
	}
	appendProviderModelPricingDebugAttempt(debug, groupAttempt)

	return transformOneHubPricing(availableEnvelope.Data, groupEnvelope.Data), nil
}

func transformOneHubPricing(models oneHubModelPricing, groupMap oneHubUserGroupMap) *providerPricingResponse {
	data := make([]providerModelPricing, 0, len(models))
	for modelName, model := range models {
		enableGroups := model.Groups
		if len(enableGroups) == 0 {
			enableGroups = []string{"default"}
		}
		quotaType := 0
		if strings.EqualFold(model.Price.Type, "times") {
			quotaType = 1
		}

		completionRatio := 0.0
		modelRatio := 0.0
		if model.Price.Input != nil && validProviderPrice(*model.Price.Input) {
			modelRatio = *model.Price.Input / 2
			if *model.Price.Input > 0 && model.Price.Output != nil {
				completionRatio = *model.Price.Output / *model.Price.Input
			}
		}

		modelPriceRaw, _ := json.Marshal(map[string]*float64{
			"input":  model.Price.Input,
			"output": model.Price.Output,
		})

		data = append(data, providerModelPricing{
			ModelName:              modelName,
			QuotaType:              quotaType,
			ModelRatio:             modelRatio,
			directInputUSDPerM:     model.Price.Input,
			directOutputUSDPerM:    model.Price.Output,
			ModelPrice:             modelPriceRaw,
			OwnerBy:                model.OwnedBy,
			CompletionRatio:        completionRatio,
			EnableGroups:           enableGroups,
			SupportedEndpointTypes: []string{},
		})
	}

	groupRatio := make(map[string]float64, len(groupMap))
	usableGroup := make(map[string]string, len(groupMap))
	for key, group := range groupMap {
		ratio := group.Ratio
		if ratio <= 0 {
			ratio = 1
		}
		groupRatio[key] = ratio
		usableGroup[key] = group.Name
	}

	return &providerPricingResponse{
		Data:        data,
		GroupRatio:  groupRatio,
		Success:     true,
		UsableGroup: usableGroup,
	}
}

func attachAuthHeader(req *http.Request, apiKey, authType string) {
	if req == nil {
		return
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return
	}

	headerName, headerValue := resolveProviderAuthHeader(apiKey, authType)
	req.Header.Set(headerName, headerValue)
	if strings.EqualFold(headerName, "x-api-key") {
		req.Header.Set("anthropic-version", "2023-06-01")
	}
}

func buildProviderModelPricingResponse(siteType SiteType, pricingSource string, pricing *providerPricingResponse) *ProviderModelPricingResponse {
	groupRatio := pricing.GroupRatio
	if groupRatio == nil {
		groupRatio = map[string]float64{}
	}
	groupMultiplier := groupRatio["default"]
	if groupMultiplier <= 0 {
		groupMultiplier = 1
	}

	items := make([]ProviderModelPricingItem, 0, len(pricing.Data))
	for _, model := range pricing.Data {
		cacheCreate, hasCacheCreate := model.cacheMultiplier([]string{"cache_create_multiplier", "cache_creation_ratio", "cache_create_ratio"}, []float64{model.CacheCreateMultiplier, model.CacheCreationRatio, model.CacheCreateRatio})
		cacheRead, hasCacheRead := model.cacheMultiplier([]string{"cache_read_multiplier", "cache_read_ratio", "cache_ratio"}, []float64{model.CacheReadMultiplier, model.CacheReadRatio, model.CacheRatio})
		item := ProviderModelPricingItem{
			PriceFieldsKnown:       true,
			Model:                  model.ModelName,
			DisplayName:            model.DisplayName,
			CreatedAt:              model.CreatedAt,
			MaxInputTokens:         model.MaxInputTokens,
			MaxTokens:              model.MaxTokens,
			Capabilities:           model.Capabilities,
			Description:            model.ModelDescription,
			QuotaType:              model.QuotaType,
			ModelRatio:             model.ModelRatio,
			CompletionRatio:        model.CompletionRatio,
			CacheCreateMultiplier:  cacheCreate,
			CacheReadMultiplier:    cacheRead,
			OwnerBy:                model.OwnerBy,
			EnableGroups:           model.EnableGroups,
			SupportedEndpointTypes: model.SupportedEndpointTypes,
		}

		if model.QuotaType == 0 {
			// 参考 all-api-hub：inputUSD(每 1M) = model_ratio × 2 × groupRatio
			// outputUSD(每 1M) = model_ratio × completion_ratio × 2 × groupRatio
			item.HasInputPrice = model.hasPriceField("model_ratio", model.ModelRatio)
			item.HasOutputPrice = item.HasInputPrice && model.hasPriceField("completion_ratio", model.CompletionRatio)
			if item.HasInputPrice {
				item.InputUSDPerM = model.ModelRatio * 2 * groupMultiplier
			}
			if item.HasOutputPrice {
				item.OutputUSDPerM = model.ModelRatio * model.CompletionRatio * 2 * groupMultiplier
			}
			if model.directInputUSDPerM != nil && validProviderPrice(*model.directInputUSDPerM) {
				item.InputUSDPerM = *model.directInputUSDPerM * groupMultiplier
				item.HasInputPrice = true
			}
			if model.directOutputUSDPerM != nil && validProviderPrice(*model.directOutputUSDPerM) {
				item.OutputUSDPerM = *model.directOutputUSDPerM * groupMultiplier
				item.HasOutputPrice = true
			}
			item.HasCacheCreatePrice = item.HasInputPrice && hasCacheCreate
			item.HasCacheReadPrice = item.HasInputPrice && hasCacheRead
		} else {
			item.PerCallPrice = parsePerCallPrice(model.ModelPrice, groupMultiplier)
		}

		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Model < items[j].Model
	})

	return &ProviderModelPricingResponse{
		SiteType:      siteType,
		PricingSource: pricingSource,
		GroupRatio:    pricing.GroupRatio,
		UsableGroup:   pricing.UsableGroup,
		Models:        items,
	}
}

func parsePerCallPrice(raw json.RawMessage, groupMultiplier float64) *ProviderModelPerCallPrice {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}

	// number
	var numberValue float64
	if err := json.Unmarshal(raw, &numberValue); err == nil && validProviderPrice(numberValue) {
		unified := numberValue * groupMultiplier
		return &ProviderModelPerCallPrice{Unified: &unified}
	}

	// { input, output }
	var pair struct {
		Input  *float64 `json:"input"`
		Output *float64 `json:"output"`
	}
	if err := json.Unmarshal(raw, &pair); err != nil {
		return nil
	}

	result := &ProviderModelPerCallPrice{}
	if pair.Input != nil && validProviderPrice(*pair.Input) {
		input := *pair.Input * groupMultiplier * doneHubTokenToCallRatio
		result.Input = &input
	}
	if pair.Output != nil && validProviderPrice(*pair.Output) {
		output := *pair.Output * groupMultiplier * doneHubTokenToCallRatio
		result.Output = &output
	}
	if result.Input == nil && result.Output == nil {
		return nil
	}
	return result
}

func fetchOpenAIModels(client *http.Client, apiURL, apiKey, authType string, debug *ProviderModelPricingDebug) ([]ProviderModelPricingItem, error) {
	targetURL := joinURL(apiURL, "/v1/models")
	attempt := newProviderModelPricingDebugAttempt(providerModelPricingSourceOpenAIList, "/v1/models", "GET", targetURL, authType)
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		attempt.Error = err.Error()
		appendProviderModelPricingDebugAttempt(debug, attempt)
		return nil, err
	}
	attachAuthHeader(req, apiKey, authType)
	req.Header.Set("Accept", "application/json")
	attempt.RequestHeaders = buildProviderPricingHeaderSnapshot(req.Header)

	startedAt := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		attempt.DurationMs = time.Since(startedAt).Milliseconds()
		attempt.Error = err.Error()
		appendProviderModelPricingDebugAttempt(debug, attempt)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	setProviderPricingDebugResponse(attempt, resp, body, time.Since(startedAt))
	if err != nil {
		attempt.Error = err.Error()
		appendProviderModelPricingDebugAttempt(debug, attempt)
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		httpErr := &upstreamHTTPError{
			Endpoint:   "/v1/models",
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
		attempt.Error = httpErr.Error()
		appendProviderModelPricingDebugAttempt(debug, attempt)
		return nil, httpErr
	}

	var list openAIModelList
	if err := json.Unmarshal(body, &list); err != nil {
		parseErr := &providerPricingParseError{
			Endpoint:    "/v1/models",
			StatusCode:  resp.StatusCode,
			ContentType: attempt.ContentType,
			Body:        string(body),
			Challenge:   detectProviderPricingChallenge("/v1/models", attempt.ContentType, string(body)),
			Err:         err,
		}
		attempt.Error = parseErr.Error()
		appendProviderModelPricingDebugAttempt(debug, attempt)
		return nil, parseErr
	}
	if len(list.Data) == 0 {
		apiErr := fmt.Errorf("/v1/models 返回空列表")
		attempt.Error = apiErr.Error()
		appendProviderModelPricingDebugAttempt(debug, attempt)
		return nil, apiErr
	}

	models := make([]ProviderModelPricingItem, 0, len(list.Data))
	for _, item := range list.Data {
		modelID := strings.TrimSpace(item.ID)
		if modelID == "" {
			continue
		}
		createdAt := ""
		switch value := item.CreatedAt.(type) {
		case string:
			createdAt = strings.TrimSpace(value)
		case float64:
			createdAt = time.Unix(int64(value), 0).UTC().Format(time.RFC3339)
		}
		if createdAt == "" && item.Created > 0 {
			createdAt = time.Unix(item.Created, 0).UTC().Format(time.RFC3339)
		}
		models = append(models, ProviderModelPricingItem{
			PriceFieldsKnown: true,
			Model:            modelID,
			DisplayName:      strings.TrimSpace(item.DisplayName),
			CreatedAt:        createdAt,
			MaxInputTokens:   item.MaxInputTokens,
			MaxTokens:        item.MaxTokens,
			Capabilities:     item.Capabilities,
			QuotaType:        -1,
		})
	}
	appendProviderModelPricingDebugAttempt(debug, attempt)
	return models, nil
}
