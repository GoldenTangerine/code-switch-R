package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/dop251/goja"
)

type ProviderQuotaQueryType string

type ProviderQuotaTemplateType string

type ProviderQuotaValueMode string

const (
	ProviderQuotaQueryTypeNone             ProviderQuotaQueryType = "none"
	ProviderQuotaQueryTypeBalance          ProviderQuotaQueryType = "balance"
	ProviderQuotaQueryTypeCustom           ProviderQuotaQueryType = "custom"
	ProviderQuotaQueryTypeGeneral          ProviderQuotaQueryType = "general"
	ProviderQuotaQueryTypeNewAPI           ProviderQuotaQueryType = "newapi"
	ProviderQuotaQueryTypeSub2API          ProviderQuotaQueryType = "sub2api"
	ProviderQuotaQueryTypeTokenPlanGLM     ProviderQuotaQueryType = "token_plan_glm"
	ProviderQuotaQueryTypeTokenPlanKimi    ProviderQuotaQueryType = "token_plan_kimi"
	ProviderQuotaQueryTypeTokenPlanMiniMax ProviderQuotaQueryType = "token_plan_minimax"
)

const (
	ProviderQuotaTemplateTypeBalance   ProviderQuotaTemplateType = "balance"
	ProviderQuotaTemplateTypeCustom    ProviderQuotaTemplateType = "custom"
	ProviderQuotaTemplateTypeGeneral   ProviderQuotaTemplateType = "general"
	ProviderQuotaTemplateTypeNewAPI    ProviderQuotaTemplateType = "newapi"
	ProviderQuotaTemplateTypeSub2API   ProviderQuotaTemplateType = "sub2api"
	ProviderQuotaTemplateTypeTokenPlan ProviderQuotaTemplateType = "token_plan"
)

const (
	ProviderQuotaValueModeCurrency ProviderQuotaValueMode = "currency"
	ProviderQuotaValueModeCount    ProviderQuotaValueMode = "count"
)

type ProviderQuotaQueryConfig struct {
	Enabled           bool   `json:"enabled"`
	TemplateType      string `json:"templateType,omitempty"`
	Code              string `json:"code,omitempty"`
	Timeout           int    `json:"timeout,omitempty"`
	APIKey            string `json:"apiKey,omitempty"`
	BaseURL           string `json:"baseUrl,omitempty"`
	AccessToken       string `json:"accessToken,omitempty"`
	UserID            string `json:"userId,omitempty"`
	TokenPlanProvider string `json:"tokenPlanProvider,omitempty"`
	AutoQueryInterval int    `json:"autoQueryInterval,omitempty"`
	AutoIntervalMins  int    `json:"autoIntervalMinutes,omitempty"`
}

type ProviderQuotaQueryItem struct {
	Key            string  `json:"key"`
	Label          string  `json:"label,omitempty"`
	Used           float64 `json:"used"`
	Total          float64 `json:"total"`
	Unlimited      bool    `json:"unlimited,omitempty"`
	NextReset      string  `json:"nextReset,omitempty"`
	Active         bool    `json:"active"`
	ValueMode      string  `json:"valueMode,omitempty"`
	Unit           string  `json:"unit,omitempty"`
	Extra          string  `json:"extra,omitempty"`
	InvalidMessage string  `json:"invalidMessage,omitempty"`
}

type ProviderQuotaQueryResult struct {
	Success   bool                     `json:"success"`
	QueryType string                   `json:"queryType"`
	Items     []ProviderQuotaQueryItem `json:"items"`
	Error     string                   `json:"error,omitempty"`
	QueriedAt int64                    `json:"queriedAt,omitempty"`
}

type ProviderQuotaScriptValidationResult struct {
	Valid bool   `json:"valid"`
	Error string `json:"error,omitempty"`
}

type ProviderQuotaQueryService struct {
	client *http.Client
}

type providerQuotaBalanceTarget struct {
	Provider string
	BaseURL  string
}

type providerQuotaScriptRequestConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    any               `json:"body,omitempty"`
}

func NewProviderQuotaQueryService() *ProviderQuotaQueryService {
	return &ProviderQuotaQueryService{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *ProviderQuotaQueryService) ValidateScriptPreset(
	templateType string,
	scriptCode string,
) *ProviderQuotaScriptValidationResult {
	if err := validateProviderQuotaScriptPreset(templateType, scriptCode); err != nil {
		return &ProviderQuotaScriptValidationResult{
			Valid: false,
			Error: err.Error(),
		}
	}
	return &ProviderQuotaScriptValidationResult{Valid: true}
}

func (s *ProviderQuotaQueryService) QueryQuota(
	queryType string,
	apiURL string,
	apiKey string,
	queryConfig *ProviderQuotaQueryConfig,
) *ProviderQuotaQueryResult {
	normalizedType := normalizeProviderQuotaQueryType(queryType)
	normalizedConfig := normalizeProviderQuotaQueryConfig(queryConfig, normalizedType)
	result := &ProviderQuotaQueryResult{
		Success:   false,
		QueryType: string(normalizedType),
		Items:     []ProviderQuotaQueryItem{},
		QueriedAt: time.Now().UnixMilli(),
	}

	if normalizedConfig != nil && !normalizedConfig.Enabled {
		result.QueryType = string(ProviderQuotaQueryTypeNone)
		return result
	}

	if normalizedType == ProviderQuotaQueryTypeNone {
		return result
	}

	effectiveBaseURL := strings.TrimSpace(apiURL)
	effectiveAPIKey := strings.TrimSpace(apiKey)
	if normalizedConfig != nil {
		if normalizedConfig.BaseURL != "" {
			effectiveBaseURL = normalizedConfig.BaseURL
		}
		if normalizedConfig.APIKey != "" {
			effectiveAPIKey = normalizedConfig.APIKey
		}
	}

	items, err := s.queryQuotaByType(normalizedType, effectiveBaseURL, effectiveAPIKey, normalizedConfig)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Success = true
	result.Items = normalizeProviderQuotaQueryItems(items)
	return result
}

func normalizeProviderQuotaQueryType(value string) ProviderQuotaQueryType {
	switch ProviderQuotaQueryType(strings.TrimSpace(strings.ToLower(value))) {
	case ProviderQuotaQueryTypeBalance:
		return ProviderQuotaQueryTypeBalance
	case ProviderQuotaQueryTypeCustom:
		return ProviderQuotaQueryTypeCustom
	case ProviderQuotaQueryTypeGeneral:
		return ProviderQuotaQueryTypeGeneral
	case ProviderQuotaQueryTypeNewAPI:
		return ProviderQuotaQueryTypeNewAPI
	case ProviderQuotaQueryTypeSub2API:
		return ProviderQuotaQueryTypeSub2API
	case ProviderQuotaQueryTypeTokenPlanGLM:
		return ProviderQuotaQueryTypeTokenPlanGLM
	case ProviderQuotaQueryTypeTokenPlanKimi:
		return ProviderQuotaQueryTypeTokenPlanKimi
	case ProviderQuotaQueryTypeTokenPlanMiniMax:
		return ProviderQuotaQueryTypeTokenPlanMiniMax
	default:
		return ProviderQuotaQueryTypeNone
	}
}

func normalizeProviderQuotaQueryConfig(
	config *ProviderQuotaQueryConfig,
	fallbackType ProviderQuotaQueryType,
) *ProviderQuotaQueryConfig {
	if config == nil {
		return nil
	}

	normalized := *config
	normalized.TemplateType = strings.TrimSpace(strings.ToLower(normalized.TemplateType))
	normalized.Code = strings.TrimSpace(normalized.Code)
	normalized.APIKey = strings.TrimSpace(normalized.APIKey)
	normalized.BaseURL = strings.TrimSpace(normalized.BaseURL)
	normalized.AccessToken = strings.TrimSpace(normalized.AccessToken)
	normalized.UserID = strings.TrimSpace(normalized.UserID)
	normalized.TokenPlanProvider = strings.TrimSpace(strings.ToLower(normalized.TokenPlanProvider))
	normalized.Timeout = normalizeProviderQuotaTimeout(normalized.Timeout)
	if normalized.AutoQueryInterval <= 0 && normalized.AutoIntervalMins > 0 {
		normalized.AutoQueryInterval = normalized.AutoIntervalMins
	}
	if normalized.TemplateType == "" {
		normalized.TemplateType = string(queryTypeToProviderQuotaTemplateType(fallbackType))
	}
	if normalized.TokenPlanProvider == "" {
		normalized.TokenPlanProvider = tokenPlanProviderFromQueryType(fallbackType)
	}
	sanitizeProviderQuotaQueryConfigForTemplate(&normalized)
	return &normalized
}

func sanitizeProviderQuotaQueryConfigForTemplate(config *ProviderQuotaQueryConfig) {
	if config == nil {
		return
	}

	switch ProviderQuotaTemplateType(strings.TrimSpace(strings.ToLower(config.TemplateType))) {
	case ProviderQuotaTemplateTypeBalance:
		config.Code = ""
		config.APIKey = ""
		config.BaseURL = ""
		config.AccessToken = ""
		config.UserID = ""
	case ProviderQuotaTemplateTypeGeneral, ProviderQuotaTemplateTypeSub2API:
		config.AccessToken = ""
		config.UserID = ""
	case ProviderQuotaTemplateTypeNewAPI:
		config.APIKey = ""
	case ProviderQuotaTemplateTypeTokenPlan:
		config.Code = ""
		config.APIKey = ""
		config.BaseURL = ""
		config.AccessToken = ""
		config.UserID = ""
		if config.TokenPlanProvider == "" {
			config.TokenPlanProvider = "kimi"
		}
	}
}

func normalizeProviderQuotaTimeout(timeoutSeconds int) int {
	if timeoutSeconds <= 0 {
		return 10
	}
	if timeoutSeconds < 2 {
		return 2
	}
	if timeoutSeconds > 30 {
		return 30
	}
	return timeoutSeconds
}

func queryTypeToProviderQuotaTemplateType(queryType ProviderQuotaQueryType) ProviderQuotaTemplateType {
	switch queryType {
	case ProviderQuotaQueryTypeBalance:
		return ProviderQuotaTemplateTypeBalance
	case ProviderQuotaQueryTypeCustom:
		return ProviderQuotaTemplateTypeCustom
	case ProviderQuotaQueryTypeGeneral:
		return ProviderQuotaTemplateTypeGeneral
	case ProviderQuotaQueryTypeNewAPI:
		return ProviderQuotaTemplateTypeNewAPI
	case ProviderQuotaQueryTypeSub2API:
		return ProviderQuotaTemplateTypeSub2API
	case ProviderQuotaQueryTypeTokenPlanGLM, ProviderQuotaQueryTypeTokenPlanKimi, ProviderQuotaQueryTypeTokenPlanMiniMax:
		return ProviderQuotaTemplateTypeTokenPlan
	default:
		return ""
	}
}

func tokenPlanProviderFromQueryType(queryType ProviderQuotaQueryType) string {
	switch queryType {
	case ProviderQuotaQueryTypeTokenPlanGLM:
		return "glm"
	case ProviderQuotaQueryTypeTokenPlanMiniMax:
		return "minimax"
	case ProviderQuotaQueryTypeTokenPlanKimi:
		fallthrough
	default:
		return "kimi"
	}
}

func extractProviderQuotaQueryURL(rawURL string) (*url.URL, bool) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, false
	}

	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return parsed, true
	}

	if strings.Contains(trimmed, "://") {
		return nil, false
	}

	parsed, err = url.Parse("https://" + strings.TrimPrefix(trimmed, "//"))
	if err != nil || parsed.Host == "" {
		return nil, false
	}

	return parsed, true
}

func extractProviderQuotaQueryOrigin(rawURL string) string {
	parsed, ok := extractProviderQuotaQueryURL(rawURL)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
}

func isLocalProviderQuotaQueryOrigin(rawURL string) bool {
	parsed, ok := extractProviderQuotaQueryURL(rawURL)
	if !ok {
		return false
	}

	host := strings.TrimSpace(strings.ToLower(parsed.Hostname()))
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}

	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func resolveProviderQuotaQueryTargetBaseURL(queryType ProviderQuotaQueryType, apiURL string) string {
	if isLocalProviderQuotaQueryOrigin(apiURL) {
		return extractProviderQuotaQueryOrigin(apiURL)
	}

	normalizedURL := strings.ToLower(strings.TrimSpace(apiURL))

	switch queryType {
	case ProviderQuotaQueryTypeTokenPlanGLM:
		return "https://api.z.ai"
	case ProviderQuotaQueryTypeTokenPlanKimi:
		return "https://api.kimi.com"
	case ProviderQuotaQueryTypeTokenPlanMiniMax:
		if strings.Contains(normalizedURL, "minimax.io") {
			return "https://api.minimax.io"
		}
		return "https://api.minimaxi.com"
	default:
		return ""
	}
}

func resolveProviderQuotaBalanceTarget(baseURL string) (providerQuotaBalanceTarget, error) {
	normalized := strings.ToLower(strings.TrimSpace(baseURL))
	switch {
	case strings.Contains(normalized, "api.deepseek.com"):
		return providerQuotaBalanceTarget{Provider: "deepseek", BaseURL: "https://api.deepseek.com"}, nil
	case strings.Contains(normalized, "api.stepfun.ai"), strings.Contains(normalized, "api.stepfun.com"):
		return providerQuotaBalanceTarget{Provider: "stepfun", BaseURL: "https://api.stepfun.com"}, nil
	case strings.Contains(normalized, "api.siliconflow.cn"):
		return providerQuotaBalanceTarget{Provider: "siliconflow", BaseURL: "https://api.siliconflow.cn"}, nil
	case strings.Contains(normalized, "api.siliconflow.com"):
		return providerQuotaBalanceTarget{Provider: "siliconflow", BaseURL: "https://api.siliconflow.com"}, nil
	case strings.Contains(normalized, "openrouter.ai"):
		return providerQuotaBalanceTarget{Provider: "openrouter", BaseURL: "https://openrouter.ai"}, nil
	case strings.Contains(normalized, "api.novita.ai"):
		return providerQuotaBalanceTarget{Provider: "novita", BaseURL: "https://api.novita.ai"}, nil
	default:
		return providerQuotaBalanceTarget{}, fmt.Errorf("暂不支持当前 Base URL 的官方余额查询")
	}
}

func (s *ProviderQuotaQueryService) queryQuotaByType(
	queryType ProviderQuotaQueryType,
	baseURL string,
	apiKey string,
	queryConfig *ProviderQuotaQueryConfig,
) ([]ProviderQuotaQueryItem, error) {
	switch queryType {
	case ProviderQuotaQueryTypeBalance:
		return s.queryBalanceQuota(baseURL, apiKey)
	case ProviderQuotaQueryTypeCustom, ProviderQuotaQueryTypeGeneral, ProviderQuotaQueryTypeNewAPI, ProviderQuotaQueryTypeSub2API:
		return s.queryScriptQuota(queryType, baseURL, apiKey, queryConfig)
	case ProviderQuotaQueryTypeTokenPlanGLM:
		return s.queryGLMQuota(resolveProviderQuotaQueryTargetBaseURL(queryType, baseURL), apiKey)
	case ProviderQuotaQueryTypeTokenPlanKimi:
		return s.queryKimiQuota(resolveProviderQuotaQueryTargetBaseURL(queryType, baseURL), apiKey)
	case ProviderQuotaQueryTypeTokenPlanMiniMax:
		return s.queryMiniMaxQuota(resolveProviderQuotaQueryTargetBaseURL(queryType, baseURL), apiKey)
	default:
		return nil, fmt.Errorf("不支持的供应商查询类型：%s", queryType)
	}
}

func (s *ProviderQuotaQueryService) queryBalanceQuota(baseURL string, apiKey string) ([]ProviderQuotaQueryItem, error) {
	trimmedKey := strings.TrimSpace(apiKey)
	if trimmedKey == "" {
		return nil, fmt.Errorf("缺少 API Key")
	}

	target, err := resolveProviderQuotaBalanceTarget(baseURL)
	if err != nil {
		return nil, err
	}

	switch target.Provider {
	case "deepseek":
		return s.queryDeepSeekBalance(trimmedKey)
	case "stepfun":
		return s.queryStepFunBalance(trimmedKey)
	case "siliconflow":
		return s.querySiliconFlowBalance(target.BaseURL, trimmedKey)
	case "openrouter":
		return s.queryOpenRouterBalance(trimmedKey)
	case "novita":
		return s.queryNovitaBalance(trimmedKey)
	default:
		return nil, fmt.Errorf("暂不支持当前官方余额查询供应商")
	}
}

func (s *ProviderQuotaQueryService) queryScriptQuota(
	queryType ProviderQuotaQueryType,
	baseURL string,
	apiKey string,
	queryConfig *ProviderQuotaQueryConfig,
) ([]ProviderQuotaQueryItem, error) {
	if queryConfig == nil {
		return nil, fmt.Errorf("缺少额度查询配置")
	}
	if strings.TrimSpace(queryConfig.Code) == "" {
		return nil, fmt.Errorf("缺少额度查询脚本")
	}
	if queryType != ProviderQuotaQueryTypeCustom && strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("缺少 Base URL")
	}

	return s.executeScriptQuotaQuery(
		queryConfig.Code,
		apiKey,
		baseURL,
		queryConfig.AccessToken,
		queryConfig.UserID,
		queryConfig.TemplateType,
		queryConfig.Timeout,
	)
}

func (s *ProviderQuotaQueryService) executeScriptQuotaQuery(
	scriptCode string,
	apiKey string,
	baseURL string,
	accessToken string,
	userID string,
	templateType string,
	timeoutSeconds int,
) ([]ProviderQuotaQueryItem, error) {
	scriptWithVars := buildProviderQuotaScriptWithVars(scriptCode, apiKey, baseURL, accessToken, userID)
	trimmedBaseURL := strings.TrimSpace(baseURL)
	isCustomTemplate := strings.EqualFold(strings.TrimSpace(templateType), string(ProviderQuotaTemplateTypeCustom))

	if trimmedBaseURL != "" {
		if err := validateProviderQuotaBaseURL(trimmedBaseURL); err != nil {
			return nil, err
		}
	}

	vm := goja.New()
	compiledValue, err := vm.RunString(scriptWithVars)
	if err != nil {
		return nil, fmt.Errorf("解析额度查询脚本失败: %w", err)
	}

	configObject := compiledValue.ToObject(vm)
	if configObject == nil {
		return nil, fmt.Errorf("额度查询脚本必须返回配置对象")
	}

	var requestConfig providerQuotaScriptRequestConfig
	requestPayload, err := json.Marshal(configObject.Get("request").Export())
	if err != nil {
		return nil, fmt.Errorf("序列化 request 配置失败: %w", err)
	}
	if err := json.Unmarshal(requestPayload, &requestConfig); err != nil {
		return nil, fmt.Errorf("解析 request 配置失败: %w", err)
	}
	if requestConfig.Headers == nil {
		requestConfig.Headers = map[string]string{}
	}

	requestConfig.URL = strings.TrimSpace(requestConfig.URL)
	requestConfig.Method = strings.TrimSpace(requestConfig.Method)
	if requestConfig.URL == "" {
		return nil, fmt.Errorf("request.url 不能为空")
	}
	if requestConfig.Method == "" {
		requestConfig.Method = http.MethodGet
	}
	if err := validateProviderQuotaRequestURL(requestConfig.URL, trimmedBaseURL, isCustomTemplate); err != nil {
		return nil, err
	}

	responseBody, err := s.sendScriptRequest(requestConfig, timeoutSeconds)
	if err != nil {
		return nil, err
	}

	var responsePayload any
	if err := json.Unmarshal(responseBody, &responsePayload); err != nil {
		return nil, fmt.Errorf("解析脚本响应 JSON 失败: %w", err)
	}

	extractor, ok := goja.AssertFunction(configObject.Get("extractor"))
	if !ok {
		return nil, fmt.Errorf("缺少 extractor 函数")
	}

	resultValue, err := extractor(goja.Undefined(), vm.ToValue(responsePayload))
	if err != nil {
		return nil, fmt.Errorf("执行 extractor 失败: %w", err)
	}

	items, err := buildProviderQuotaItemsFromScriptResult(resultValue.Export(), templateType)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("脚本未返回可展示的额度数据")
	}
	applySub2APIUnlimitedSemantics(items, templateType, responsePayload)
	return normalizeProviderQuotaQueryItems(items), nil
}

func applySub2APIUnlimitedSemantics(items []ProviderQuotaQueryItem, templateType string, responsePayload any) {
	if !strings.EqualFold(strings.TrimSpace(templateType), string(ProviderQuotaTemplateTypeSub2API)) {
		return
	}

	isUnlimitedSubscription := hasSub2APIUnlimitedSubscription(responsePayload)
	for index := range items {
		item := &items[index]
		canDisplayUnlimited := item.Active &&
			strings.TrimSpace(item.InvalidMessage) == "" &&
			(item.Unlimited || strings.EqualFold(strings.TrimSpace(item.Key), "balance"))
		item.Unlimited = isUnlimitedSubscription && canDisplayUnlimited
		if item.Unlimited {
			item.Used = 0
			item.Total = 0
		}
	}
}

func hasSub2APIUnlimitedSubscription(responsePayload any) bool {
	payload, ok := responsePayload.(map[string]any)
	if !ok {
		return false
	}
	subscription, ok := payload["subscription"].(map[string]any)
	if !ok {
		return false
	}

	for _, key := range []string{"daily_limit_usd", "weekly_limit_usd", "monthly_limit_usd"} {
		limit, hasLimit := floatFromAnyOk(subscription[key])
		if !hasLimit || limit != 0 {
			return false
		}
	}
	return true
}

func validateProviderQuotaScriptPreset(templateType string, scriptCode string) error {
	switch ProviderQuotaTemplateType(strings.TrimSpace(strings.ToLower(templateType))) {
	case ProviderQuotaTemplateTypeCustom, ProviderQuotaTemplateTypeGeneral, ProviderQuotaTemplateTypeNewAPI, ProviderQuotaTemplateTypeSub2API:
	default:
		return fmt.Errorf("当前模版不支持编辑脚本预设")
	}

	scriptCode = strings.TrimSpace(scriptCode)
	if scriptCode == "" {
		return fmt.Errorf("查询脚本不能为空")
	}

	vm := goja.New()
	timer := time.AfterFunc(2*time.Second, func() {
		vm.Interrupt("额度查询脚本校验超时")
	})
	defer timer.Stop()

	compiledValue, err := vm.RunString(scriptCode)
	if err != nil {
		return fmt.Errorf("解析额度查询脚本失败: %w", err)
	}

	configObject := compiledValue.ToObject(vm)
	if configObject == nil {
		return fmt.Errorf("额度查询脚本必须返回配置对象")
	}

	requestValue := configObject.Get("request")
	if goja.IsUndefined(requestValue) || goja.IsNull(requestValue) {
		return fmt.Errorf("缺少 request 配置")
	}

	var requestConfig providerQuotaScriptRequestConfig
	requestPayload, err := json.Marshal(requestValue.Export())
	if err != nil {
		return fmt.Errorf("序列化 request 配置失败: %w", err)
	}
	if err := json.Unmarshal(requestPayload, &requestConfig); err != nil {
		return fmt.Errorf("解析 request 配置失败: %w", err)
	}
	requestConfig.URL = strings.TrimSpace(requestConfig.URL)
	requestConfig.Method = strings.TrimSpace(requestConfig.Method)
	if requestConfig.URL == "" {
		return fmt.Errorf("request.url 不能为空")
	}
	if requestConfig.Method != "" {
		method := strings.ToUpper(requestConfig.Method)
		if _, err := http.NewRequest(method, "http://localhost", nil); err != nil {
			return fmt.Errorf("request.method 不合法: %w", err)
		}
	}

	if _, ok := goja.AssertFunction(configObject.Get("extractor")); !ok {
		return fmt.Errorf("缺少 extractor 函数")
	}
	return nil
}

func buildProviderQuotaScriptWithVars(
	scriptCode string,
	apiKey string,
	baseURL string,
	accessToken string,
	userID string,
) string {
	replaced := strings.ReplaceAll(scriptCode, "{{apiKey}}", escapeProviderQuotaScriptTemplateValue(apiKey))
	replaced = strings.ReplaceAll(replaced, "{{baseUrl}}", escapeProviderQuotaScriptTemplateValue(baseURL))
	replaced = strings.ReplaceAll(replaced, "{{accessToken}}", escapeProviderQuotaScriptTemplateValue(accessToken))
	replaced = strings.ReplaceAll(replaced, "{{userId}}", escapeProviderQuotaScriptTemplateValue(userID))
	return replaced
}

func escapeProviderQuotaScriptTemplateValue(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return strings.ReplaceAll(strings.ReplaceAll(value, "\\", "\\\\"), "'", "\\'")
	}

	escaped := strings.TrimPrefix(strings.TrimSuffix(string(encoded), `"`), `"`)
	return strings.ReplaceAll(escaped, `'`, `\'`)
}

func validateProviderQuotaBaseURL(baseURL string) error {
	parsedURL, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("无效的 Base URL")
	}
	if parsedURL.Scheme != "https" && !isLoopbackProviderQuotaURL(parsedURL) {
		return fmt.Errorf("Base URL 必须使用 HTTPS（localhost 例外）")
	}
	if strings.TrimSpace(parsedURL.Hostname()) == "" {
		return fmt.Errorf("Base URL 缺少有效主机名")
	}
	return nil
}

func validateProviderQuotaRequestURL(requestURL string, baseURL string, isCustomTemplate bool) error {
	parsedRequest, err := url.Parse(strings.TrimSpace(requestURL))
	if err != nil || parsedRequest.Scheme == "" || parsedRequest.Host == "" {
		return fmt.Errorf("无效的请求 URL")
	}
	if !isCustomTemplate && parsedRequest.Scheme != "https" && !isLoopbackProviderQuotaURL(parsedRequest) {
		return fmt.Errorf("请求 URL 必须使用 HTTPS（localhost 例外）")
	}
	if isCustomTemplate || strings.TrimSpace(baseURL) == "" {
		return nil
	}

	parsedBase, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsedBase.Host == "" {
		return fmt.Errorf("无效的 Base URL")
	}
	if !strings.EqualFold(parsedRequest.Host, parsedBase.Host) {
		return fmt.Errorf("请求 URL 必须与 Base URL 同源")
	}
	return nil
}

func isLoopbackProviderQuotaURL(parsedURL *url.URL) bool {
	if parsedURL == nil {
		return false
	}
	host := strings.TrimSpace(strings.ToLower(parsedURL.Hostname()))
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *ProviderQuotaQueryService) sendScriptRequest(
	config providerQuotaScriptRequestConfig,
	timeoutSeconds int,
) ([]byte, error) {
	var bodyReader io.Reader
	if config.Body != nil {
		serializedBody, err := serializeProviderQuotaScriptBody(config.Body)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		bodyReader = bytes.NewReader(serializedBody)
	}

	method := strings.ToUpper(strings.TrimSpace(config.Method))
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequest(method, config.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("创建额度查询请求失败: %w", err)
	}
	for key, value := range config.Headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(key, value)
	}

	client := s.resolveHTTPClientWithTimeout(time.Duration(normalizeProviderQuotaTimeout(timeoutSeconds)) * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求额度查询接口失败: %w", err)
	}
	defer resp.Body.Close()

	responseBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("读取额度查询响应失败: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("额度查询失败 (HTTP %d): %s", resp.StatusCode, truncateText(string(responseBody), 512))
	}
	return responseBody, nil
}

func serializeProviderQuotaScriptBody(body any) ([]byte, error) {
	switch typed := body.(type) {
	case nil:
		return nil, nil
	case string:
		return []byte(typed), nil
	case []byte:
		return typed, nil
	default:
		return json.Marshal(typed)
	}
}

func (s *ProviderQuotaQueryService) resolveHTTPClientWithTimeout(timeout time.Duration) *http.Client {
	if s == nil || s.client == nil {
		return &http.Client{Timeout: timeout}
	}
	if timeout <= 0 || s.client.Timeout == timeout {
		return s.client
	}
	clone := *s.client
	clone.Timeout = timeout
	return &clone
}

func buildProviderQuotaItemsFromScriptResult(result any, templateType string) ([]ProviderQuotaQueryItem, error) {
	switch typed := result.(type) {
	case map[string]any:
		item, err := buildProviderQuotaItemFromScriptObject(typed, 0, templateType)
		if err != nil {
			return nil, err
		}
		return []ProviderQuotaQueryItem{item}, nil
	case []any:
		items := make([]ProviderQuotaQueryItem, 0, len(typed))
		for index, rawItem := range typed {
			objectItem, ok := rawItem.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("脚本返回数组时，第 %d 项不是对象", index+1)
			}
			item, err := buildProviderQuotaItemFromScriptObject(objectItem, index, templateType)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil
	default:
		return nil, fmt.Errorf("脚本必须返回对象或对象数组")
	}
}

func buildProviderQuotaItemFromScriptObject(
	payload map[string]any,
	index int,
	templateType string,
) (ProviderQuotaQueryItem, error) {
	label := firstNonEmptyProviderQuotaString(payload["label"], payload["planName"], payload["name"])
	key := firstNonEmptyProviderQuotaString(payload["key"])
	if key == "" {
		key = buildProviderQuotaItemFallbackKey(label, index, templateType)
	}
	invalidMessage := strings.TrimSpace(firstNonEmptyProviderQuotaString(payload["invalidMessage"]))
	extra := strings.TrimSpace(firstNonEmptyProviderQuotaString(payload["extra"]))

	active, hasActive := boolFromAny(payload["active"])
	isValid, hasIsValid := boolFromAny(payload["isValid"])
	switch {
	case hasActive && hasIsValid:
		active = active && isValid
	case !hasActive && hasIsValid:
		active = isValid
	case !hasActive:
		active = true
	}

	total, hasTotal := floatFromAnyOk(payload["total"])
	used, hasUsed := floatFromAnyOk(payload["used"])
	remaining, hasRemaining := floatFromAnyOk(payload["remaining"])
	unlimited, _ := boolFromAny(payload["unlimited"])
	unlimited = unlimited && strings.EqualFold(strings.TrimSpace(templateType), string(ProviderQuotaTemplateTypeSub2API))

	switch {
	case hasTotal && hasUsed:
	case hasTotal && hasRemaining:
		used = total - remaining
		hasUsed = true
	case hasUsed && hasRemaining:
		total = used + remaining
		hasTotal = true
	case hasRemaining:
		total = remaining
		used = 0
		hasTotal = true
		hasUsed = true
	case hasTotal:
		used = 0
		hasUsed = true
	case hasUsed:
		total = used
		used = 0
		hasTotal = true
		hasUsed = true
	case unlimited:
		total = 0
		used = 0
		hasTotal = true
		hasUsed = true
	default:
		if !active {
			total = 0
			used = 0
			hasTotal = true
			hasUsed = true
			break
		}
		return ProviderQuotaQueryItem{}, fmt.Errorf("脚本返回结果缺少 total / used / remaining 数值字段")
	}

	if !hasTotal {
		total = 0
	}
	if !hasUsed {
		used = 0
	}

	unit := strings.TrimSpace(firstNonEmptyProviderQuotaString(payload["unit"]))
	valueMode := normalizeProviderQuotaValueMode(payload["valueMode"], unit)
	nextReset := isoTimeFromAny(payload["nextReset"])
	if nextReset == "" {
		nextReset = isoTimeFromAny(payload["resetTime"])
	}
	if label == "" {
		label = defaultProviderQuotaItemLabel(valueMode, templateType, index)
	}

	return ProviderQuotaQueryItem{
		Key:            key,
		Label:          label,
		Used:           clampNonNegativeFloat(used),
		Total:          clampNonNegativeFloat(total),
		Unlimited:      unlimited,
		NextReset:      nextReset,
		Active:         active,
		ValueMode:      valueMode,
		Unit:           unit,
		Extra:          extra,
		InvalidMessage: invalidMessage,
	}, nil
}

func buildProviderQuotaItemFallbackKey(label string, index int, templateType string) string {
	slug := slugifyProviderQuotaKey(label)
	if slug != "" {
		return slug
	}
	switch strings.TrimSpace(strings.ToLower(templateType)) {
	case string(ProviderQuotaTemplateTypeBalance):
		return "balance"
	case string(ProviderQuotaTemplateTypeGeneral), string(ProviderQuotaTemplateTypeNewAPI), string(ProviderQuotaTemplateTypeSub2API), string(ProviderQuotaTemplateTypeCustom):
		return fmt.Sprintf("quota_%d", index+1)
	default:
		return fmt.Sprintf("item_%d", index+1)
	}
}

func slugifyProviderQuotaKey(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	lastUnderscore := false
	for _, r := range trimmed {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastUnderscore = false
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if builder.Len() > 0 && !lastUnderscore {
				builder.WriteRune('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(builder.String(), "_")
}

func defaultProviderQuotaItemLabel(valueMode string, templateType string, index int) string {
	if valueMode == string(ProviderQuotaValueModeCurrency) || strings.EqualFold(templateType, string(ProviderQuotaTemplateTypeBalance)) {
		if index == 0 {
			return "Balance"
		}
		return fmt.Sprintf("Balance %d", index+1)
	}
	if index == 0 {
		return "Quota"
	}
	return fmt.Sprintf("Quota %d", index+1)
}

func firstNonEmptyProviderQuotaString(values ...any) string {
	for _, value := range values {
		if normalized := strings.TrimSpace(stringFromAny(value)); normalized != "" {
			return normalized
		}
	}
	return ""
}

func normalizeProviderQuotaValueMode(value any, unit string) string {
	normalized := strings.TrimSpace(strings.ToLower(stringFromAny(value)))
	switch normalized {
	case string(ProviderQuotaValueModeCurrency):
		return string(ProviderQuotaValueModeCurrency)
	case string(ProviderQuotaValueModeCount):
		return string(ProviderQuotaValueModeCount)
	}
	switch strings.TrimSpace(strings.ToUpper(unit)) {
	case "USD", "CNY", "RMB", "JPY", "EUR", "GBP", "HKD", "SGD", "AUD", "CAD", "CHF", "NZD", "KRW", "INR", "￥", "¥":
		return string(ProviderQuotaValueModeCurrency)
	default:
		return string(ProviderQuotaValueModeCount)
	}
}

func buildProviderQuotaInvalidItem(label string, invalidMessage string, extra string) ProviderQuotaQueryItem {
	key := buildProviderQuotaItemFallbackKey(label, 0, string(ProviderQuotaTemplateTypeBalance))
	if strings.TrimSpace(key) == "" {
		key = "balance"
	}
	return ProviderQuotaQueryItem{
		Key:            key,
		Label:          strings.TrimSpace(label),
		Used:           0,
		Total:          0,
		Active:         false,
		ValueMode:      string(ProviderQuotaValueModeCurrency),
		Extra:          strings.TrimSpace(extra),
		InvalidMessage: strings.TrimSpace(invalidMessage),
	}
}

func buildProviderQuotaAuthFailureItems(label string, status int) []ProviderQuotaQueryItem {
	return []ProviderQuotaQueryItem{
		buildProviderQuotaInvalidItem(
			label,
			fmt.Sprintf("官方余额查询认证失败 (HTTP %d)", status),
			"",
		),
	}
}

func (s *ProviderQuotaQueryService) queryDeepSeekBalance(apiKey string) ([]ProviderQuotaQueryItem, error) {
	body, status, err := s.sendJSONRequest(
		http.MethodGet,
		"https://api.deepseek.com/user/balance",
		nil,
		map[string]string{
			"Authorization": "Bearer " + apiKey,
			"Accept":        "application/json",
			"User-Agent":    "code-switch-R/1.0",
		},
	)
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return buildProviderQuotaAuthFailureItems("DeepSeek", status), nil
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("官方余额查询失败 (HTTP %d): %s", status, truncateText(string(body), 512))
	}

	payload, err := decodeProviderQuotaJSONMap(body)
	if err != nil {
		return nil, fmt.Errorf("解析 DeepSeek 余额响应失败: %w", err)
	}

	isAvailable, hasAvailable := boolFromAny(payload["is_available"])
	if !hasAvailable {
		isAvailable = true
	}

	infos, ok := payload["balance_infos"].([]any)
	if !ok || len(infos) == 0 {
		return nil, fmt.Errorf("DeepSeek 余额响应缺少 balance_infos 数据")
	}

	items := make([]ProviderQuotaQueryItem, 0, len(infos))
	for index, rawInfo := range infos {
		info, ok := rawInfo.(map[string]any)
		if !ok {
			continue
		}
		currency := firstNonEmptyProviderQuotaString(info["currency"])
		if currency == "" {
			currency = "CNY"
		}
		remaining := clampNonNegativeFloat(floatFromAny(info["total_balance"]))
		items = append(items, ProviderQuotaQueryItem{
			Key:       buildProviderQuotaItemFallbackKey(currency, index, string(ProviderQuotaTemplateTypeBalance)),
			Label:     currency,
			Used:      0,
			Total:     remaining,
			Active:    isAvailable && remaining > 0,
			ValueMode: string(ProviderQuotaValueModeCurrency),
			Unit:      currency,
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("DeepSeek 余额响应未返回可展示的数据")
	}
	return items, nil
}

func (s *ProviderQuotaQueryService) queryStepFunBalance(apiKey string) ([]ProviderQuotaQueryItem, error) {
	body, status, err := s.sendJSONRequest(
		http.MethodGet,
		"https://api.stepfun.com/v1/accounts",
		nil,
		map[string]string{
			"Authorization": "Bearer " + apiKey,
			"Accept":        "application/json",
			"User-Agent":    "code-switch-R/1.0",
		},
	)
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return buildProviderQuotaAuthFailureItems("StepFun", status), nil
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("官方余额查询失败 (HTTP %d): %s", status, truncateText(string(body), 512))
	}

	payload, err := decodeProviderQuotaJSONMap(body)
	if err != nil {
		return nil, fmt.Errorf("解析 StepFun 余额响应失败: %w", err)
	}
	remaining := clampNonNegativeFloat(floatFromAny(payload["balance"]))
	return []ProviderQuotaQueryItem{{
		Key:       "balance",
		Label:     "StepFun",
		Used:      0,
		Total:     remaining,
		Active:    remaining > 0,
		ValueMode: string(ProviderQuotaValueModeCurrency),
		Unit:      "CNY",
	}}, nil
}

func (s *ProviderQuotaQueryService) querySiliconFlowBalance(baseURL string, apiKey string) ([]ProviderQuotaQueryItem, error) {
	body, status, err := s.sendJSONRequest(
		http.MethodGet,
		joinURL(baseURL, "/v1/user/info"),
		nil,
		map[string]string{
			"Authorization": "Bearer " + apiKey,
			"Accept":        "application/json",
			"User-Agent":    "code-switch-R/1.0",
		},
	)
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return buildProviderQuotaAuthFailureItems("SiliconFlow", status), nil
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("官方余额查询失败 (HTTP %d): %s", status, truncateText(string(body), 512))
	}

	payload, err := decodeProviderQuotaJSONMap(body)
	if err != nil {
		return nil, fmt.Errorf("解析 SiliconFlow 余额响应失败: %w", err)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("SiliconFlow 余额响应缺少 data 字段")
	}
	remaining := clampNonNegativeFloat(floatFromAny(data["totalBalance"]))
	return []ProviderQuotaQueryItem{{
		Key:       "balance",
		Label:     "SiliconFlow",
		Used:      0,
		Total:     remaining,
		Active:    remaining > 0,
		ValueMode: string(ProviderQuotaValueModeCurrency),
		Unit:      "CNY",
	}}, nil
}

func (s *ProviderQuotaQueryService) queryOpenRouterBalance(apiKey string) ([]ProviderQuotaQueryItem, error) {
	body, status, err := s.sendJSONRequest(
		http.MethodGet,
		"https://openrouter.ai/api/v1/credits",
		nil,
		map[string]string{
			"Authorization": "Bearer " + apiKey,
			"Accept":        "application/json",
			"User-Agent":    "code-switch-R/1.0",
		},
	)
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return buildProviderQuotaAuthFailureItems("OpenRouter", status), nil
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("官方余额查询失败 (HTTP %d): %s", status, truncateText(string(body), 512))
	}

	payload, err := decodeProviderQuotaJSONMap(body)
	if err != nil {
		return nil, fmt.Errorf("解析 OpenRouter 余额响应失败: %w", err)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		data = payload
	}
	total := clampNonNegativeFloat(floatFromAny(data["total_credits"]))
	used := clampNonNegativeFloat(floatFromAny(data["total_usage"]))
	if total <= 0 && used > 0 {
		total = used
	}
	active := total-used > 0
	return []ProviderQuotaQueryItem{{
		Key:       "balance",
		Label:     "OpenRouter",
		Used:      used,
		Total:     total,
		Active:    active,
		ValueMode: string(ProviderQuotaValueModeCurrency),
		Unit:      "USD",
	}}, nil
}

func (s *ProviderQuotaQueryService) queryNovitaBalance(apiKey string) ([]ProviderQuotaQueryItem, error) {
	body, status, err := s.sendJSONRequest(
		http.MethodGet,
		"https://api.novita.ai/v3/user/balance",
		nil,
		map[string]string{
			"Authorization": "Bearer " + apiKey,
			"Accept":        "application/json",
			"User-Agent":    "code-switch-R/1.0",
		},
	)
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return buildProviderQuotaAuthFailureItems("Novita AI", status), nil
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("官方余额查询失败 (HTTP %d): %s", status, truncateText(string(body), 512))
	}

	payload, err := decodeProviderQuotaJSONMap(body)
	if err != nil {
		return nil, fmt.Errorf("解析 Novita 余额响应失败: %w", err)
	}
	remaining := clampNonNegativeFloat(floatFromAny(payload["availableBalance"]) / 10000)
	return []ProviderQuotaQueryItem{{
		Key:       "balance",
		Label:     "Novita AI",
		Used:      0,
		Total:     remaining,
		Active:    remaining > 0,
		ValueMode: string(ProviderQuotaValueModeCurrency),
		Unit:      "USD",
	}}, nil
}

func (s *ProviderQuotaQueryService) queryGLMQuota(baseURL string, apiKey string) ([]ProviderQuotaQueryItem, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("缺少 API Key")
	}
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("缺少可用的 API 地址")
	}

	body, status, err := s.sendJSONRequest(
		http.MethodGet,
		joinURL(baseURL, "/api/monitor/usage/quota/limit"),
		nil,
		map[string]string{
			"Authorization":   apiKey,
			"Content-Type":    "application/json",
			"Accept":          "application/json",
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
			"User-Agent":      "code-switch-R/1.0",
		},
	)
	if err != nil {
		return nil, err
	}

	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return nil, fmt.Errorf("GLM Token Plan 查询认证失败 (HTTP %d)", status)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("GLM Token Plan 查询失败 (HTTP %d): %s", status, truncateText(string(body), 512))
	}

	payload, err := decodeProviderQuotaJSONMap(body)
	if err != nil {
		return nil, fmt.Errorf("解析 GLM Token Plan 响应失败: %w", err)
	}

	if success, ok := payload["success"].(bool); ok && !success {
		return nil, fmt.Errorf("GLM Token Plan 查询失败: %s", stringFromAny(payload["msg"]))
	}

	data, ok := payload["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("GLM Token Plan 响应缺少 data 字段")
	}

	limits, ok := data["limits"].([]any)
	if !ok || len(limits) == 0 {
		return nil, fmt.Errorf("GLM Token Plan 响应缺少 limits 数据")
	}

	items := make([]ProviderQuotaQueryItem, 0, len(limits))
	for _, rawLimit := range limits {
		limitItem, ok := rawLimit.(map[string]any)
		if !ok {
			continue
		}

		if !strings.EqualFold(stringFromAny(limitItem["type"]), "TOKENS_LIMIT") {
			continue
		}

		total := floatFromAny(limitItem["usage"])
		used := floatFromAny(limitItem["currentValue"])
		if total <= 0 {
			percentage := floatFromAny(limitItem["percentage"])
			if percentage > 0 {
				total = 100
				used = percentage
			}
		}
		if total <= 0 {
			continue
		}

		key := resolveGLMTokenPlanQuotaKey(limitItem)
		if key == "" {
			continue
		}

		items = append(items, ProviderQuotaQueryItem{
			Key:       key,
			Used:      clampNonNegativeFloat(used),
			Total:     clampNonNegativeFloat(total),
			NextReset: isoTimeFromAny(limitItem["nextResetTime"]),
			Active:    true,
			ValueMode: string(ProviderQuotaValueModeCount),
		})
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("GLM Token Plan 未返回可展示的额度窗口")
	}

	return items, nil
}

func resolveGLMTokenPlanQuotaKey(limitItem map[string]any) string {
	unit := int(floatFromAny(limitItem["unit"]))

	switch unit {
	case 6:
		return "weekly"
	case 3:
		return "five_hour"
	}

	windowLabels := []string{
		stringFromAny(limitItem["window"]),
		stringFromAny(limitItem["cycle"]),
		stringFromAny(limitItem["name"]),
	}
	if matchGLMQuotaWindowLabel(windowLabels, []string{
		"weekly", "week", "7d", "7day", "7days",
		"每周", "周额度", "周限制", "周窗口", "周配额", "7天", "七天",
	}) {
		return "weekly"
	}
	if matchGLMQuotaWindowLabel(windowLabels, []string{
		"5h", "5hour", "5hours", "5小时", "五小时",
		"5小时额度", "5小时限制", "5小时窗口", "5小时配额",
	}) {
		return "five_hour"
	}

	return ""
}

func matchGLMQuotaWindowLabel(labels []string, aliases []string) bool {
	for _, label := range labels {
		normalizedLabel := normalizeGLMQuotaWindowLabel(label)
		if normalizedLabel == "" {
			continue
		}
		for _, alias := range aliases {
			if strings.Contains(normalizedLabel, normalizeGLMQuotaWindowLabel(alias)) {
				return true
			}
		}
	}
	return false
}

func normalizeGLMQuotaWindowLabel(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}

	lowerCased := strings.ToLower(strings.TrimSpace(value))
	return strings.NewReplacer(
		" ", "",
		"\t", "",
		"\n", "",
		"\r", "",
		"-", "",
		"_", "",
	).Replace(lowerCased)
}

func normalizeProviderQuotaQueryItems(items []ProviderQuotaQueryItem) []ProviderQuotaQueryItem {
	if len(items) == 0 {
		return items
	}

	dedupedItems := make(map[string]ProviderQuotaQueryItem, len(items))
	orderedKeys := make([]string, 0, len(items))

	for _, item := range items {
		key := strings.TrimSpace(item.Key)
		if key == "" {
			continue
		}

		item.Key = key
		if existing, ok := dedupedItems[key]; ok {
			dedupedItems[key] = mergeProviderQuotaQueryItem(existing, item)
			continue
		}

		dedupedItems[key] = item
		orderedKeys = append(orderedKeys, key)
	}

	sort.SliceStable(orderedKeys, func(left, right int) bool {
		leftRank := providerQuotaQueryItemSortRank(orderedKeys[left])
		rightRank := providerQuotaQueryItemSortRank(orderedKeys[right])
		if leftRank == rightRank {
			return left < right
		}
		return leftRank < rightRank
	})

	normalizedItems := make([]ProviderQuotaQueryItem, 0, len(orderedKeys))
	for _, key := range orderedKeys {
		normalizedItems = append(normalizedItems, dedupedItems[key])
	}

	return normalizedItems
}

func mergeProviderQuotaQueryItem(existing ProviderQuotaQueryItem, candidate ProviderQuotaQueryItem) ProviderQuotaQueryItem {
	merged := existing

	if merged.Total <= 0 && candidate.Total > 0 {
		merged.Total = candidate.Total
	}
	if merged.Used <= 0 && candidate.Used > 0 {
		merged.Used = candidate.Used
	}
	merged.Unlimited = merged.Unlimited || candidate.Unlimited
	if strings.TrimSpace(merged.NextReset) == "" && strings.TrimSpace(candidate.NextReset) != "" {
		merged.NextReset = candidate.NextReset
	}
	if !merged.Active && candidate.Active {
		merged.Active = true
	}
	if strings.TrimSpace(merged.Label) == "" && strings.TrimSpace(candidate.Label) != "" {
		merged.Label = candidate.Label
	}
	if strings.TrimSpace(merged.ValueMode) == "" && strings.TrimSpace(candidate.ValueMode) != "" {
		merged.ValueMode = candidate.ValueMode
	}
	if strings.TrimSpace(merged.Unit) == "" && strings.TrimSpace(candidate.Unit) != "" {
		merged.Unit = candidate.Unit
	}
	if strings.TrimSpace(merged.Extra) == "" && strings.TrimSpace(candidate.Extra) != "" {
		merged.Extra = candidate.Extra
	}
	if merged.Active {
		merged.InvalidMessage = ""
	} else if strings.TrimSpace(merged.InvalidMessage) == "" && strings.TrimSpace(candidate.InvalidMessage) != "" {
		merged.InvalidMessage = candidate.InvalidMessage
	}

	return merged
}

func providerQuotaQueryItemSortRank(key string) int {
	switch key {
	case "five_hour":
		return 0
	case "daily":
		return 1
	case "weekly":
		return 2
	case "monthly":
		return 3
	case "total":
		return 4
	case "balance":
		return 5
	default:
		return 100
	}
}

func (s *ProviderQuotaQueryService) queryKimiQuota(baseURL string, apiKey string) ([]ProviderQuotaQueryItem, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("缺少 API Key")
	}
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("缺少可用的 API 地址")
	}

	body, status, err := s.sendJSONRequest(
		http.MethodGet,
		joinURL(baseURL, "/coding/v1/usages"),
		nil,
		map[string]string{
			"Authorization": "Bearer " + apiKey,
			"Accept":        "application/json",
			"User-Agent":    "code-switch-R/1.0",
		},
	)
	if err != nil {
		return nil, err
	}

	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return nil, fmt.Errorf("Kimi Token Plan 查询认证失败 (HTTP %d)", status)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("Kimi Token Plan 查询失败 (HTTP %d): %s", status, truncateText(string(body), 512))
	}

	payload, err := decodeProviderQuotaJSONMap(body)
	if err != nil {
		return nil, fmt.Errorf("解析 Kimi Token Plan 响应失败: %w", err)
	}

	items := make([]ProviderQuotaQueryItem, 0, 2)
	if limits, ok := payload["limits"].([]any); ok {
		for _, rawLimit := range limits {
			limitItem, ok := rawLimit.(map[string]any)
			if !ok {
				continue
			}
			detail, ok := limitItem["detail"].(map[string]any)
			if !ok {
				continue
			}
			total := floatFromAny(detail["limit"])
			remaining := floatFromAny(detail["remaining"])
			if total <= 0 {
				continue
			}
			items = append(items, ProviderQuotaQueryItem{
				Key:       "five_hour",
				Used:      clampNonNegativeFloat(total - remaining),
				Total:     clampNonNegativeFloat(total),
				NextReset: isoTimeFromAny(detail["resetTime"]),
				Active:    true,
				ValueMode: string(ProviderQuotaValueModeCount),
			})
		}
	}

	if usage, ok := payload["usage"].(map[string]any); ok {
		total := floatFromAny(usage["limit"])
		remaining := floatFromAny(usage["remaining"])
		if total > 0 {
			items = append(items, ProviderQuotaQueryItem{
				Key:       "weekly",
				Used:      clampNonNegativeFloat(total - remaining),
				Total:     clampNonNegativeFloat(total),
				NextReset: isoTimeFromAny(usage["resetTime"]),
				Active:    true,
				ValueMode: string(ProviderQuotaValueModeCount),
			})
		}
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("Kimi Token Plan 未返回可展示的额度窗口")
	}

	return items, nil
}

func (s *ProviderQuotaQueryService) queryMiniMaxQuota(baseURL string, apiKey string) ([]ProviderQuotaQueryItem, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("缺少 API Key")
	}
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("缺少可用的 API 地址")
	}

	body, status, err := s.sendJSONRequest(
		http.MethodGet,
		joinURL(baseURL, "/v1/api/openplatform/coding_plan/remains"),
		nil,
		map[string]string{
			"Authorization": "Bearer " + apiKey,
			"Content-Type":  "application/json",
			"Accept":        "application/json",
			"User-Agent":    "code-switch-R/1.0",
		},
	)
	if err != nil {
		return nil, err
	}

	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return nil, fmt.Errorf("MiniMax Token Plan 查询认证失败 (HTTP %d)", status)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("MiniMax Token Plan 查询失败 (HTTP %d): %s", status, truncateText(string(body), 512))
	}

	payload, err := decodeProviderQuotaJSONMap(body)
	if err != nil {
		return nil, fmt.Errorf("解析 MiniMax Token Plan 响应失败: %w", err)
	}

	if baseResp, ok := payload["base_resp"].(map[string]any); ok {
		statusCode := int(floatFromAny(baseResp["status_code"]))
		if statusCode != 0 {
			return nil, fmt.Errorf(
				"MiniMax Token Plan 查询失败 (code %d): %s",
				statusCode,
				stringFromAny(baseResp["status_msg"]),
			)
		}
	}

	modelRemains, ok := payload["model_remains"].([]any)
	if !ok || len(modelRemains) == 0 {
		return nil, fmt.Errorf("MiniMax Token Plan 响应缺少 model_remains 数据")
	}

	item, ok := modelRemains[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("MiniMax Token Plan 响应格式无效")
	}

	items := make([]ProviderQuotaQueryItem, 0, 2)
	intervalTotal := floatFromAny(item["current_interval_total_count"])
	intervalRemaining := floatFromAny(item["current_interval_usage_count"])
	if intervalTotal > 0 {
		items = append(items, ProviderQuotaQueryItem{
			Key:       "five_hour",
			Used:      clampNonNegativeFloat(intervalTotal - intervalRemaining),
			Total:     clampNonNegativeFloat(intervalTotal),
			NextReset: isoTimeFromAny(item["end_time"]),
			Active:    true,
			ValueMode: string(ProviderQuotaValueModeCount),
		})
	}

	weeklyTotal := floatFromAny(item["current_weekly_total_count"])
	weeklyRemaining := floatFromAny(item["current_weekly_usage_count"])
	if weeklyTotal > 0 {
		items = append(items, ProviderQuotaQueryItem{
			Key:       "weekly",
			Used:      clampNonNegativeFloat(weeklyTotal - weeklyRemaining),
			Total:     clampNonNegativeFloat(weeklyTotal),
			NextReset: isoTimeFromAny(item["weekly_end_time"]),
			Active:    true,
			ValueMode: string(ProviderQuotaValueModeCount),
		})
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("MiniMax Token Plan 未返回可展示的额度窗口")
	}

	return items, nil
}

func (s *ProviderQuotaQueryService) sendJSONRequest(
	method string,
	targetURL string,
	body io.Reader,
	headers map[string]string,
) ([]byte, int, error) {
	req, err := http.NewRequest(method, targetURL, body)
	if err != nil {
		return nil, 0, fmt.Errorf("创建请求失败: %w", err)
	}
	for key, value := range headers {
		if strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(key, value)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("请求上游额度接口失败: %w", err)
	}
	defer resp.Body.Close()

	responseBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, resp.StatusCode, fmt.Errorf("读取上游额度响应失败: %w", readErr)
	}

	return responseBody, resp.StatusCode, nil
}

func decodeProviderQuotaJSONMap(body []byte) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func floatFromAnyOk(value any) (float64, bool) {
	switch typed := value.(type) {
	case nil:
		return 0, false
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case json.Number:
		result, err := typed.Float64()
		return result, err == nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, false
		}
		number, err := json.Number(trimmed).Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

func floatFromAny(value any) float64 {
	result, _ := floatFromAnyOk(value)
	return result
}

func boolFromAny(value any) (bool, bool) {
	switch typed := value.(type) {
	case nil:
		return false, false
	case bool:
		return typed, true
	case string:
		normalized := strings.TrimSpace(strings.ToLower(typed))
		switch normalized {
		case "true", "1", "yes", "on":
			return true, true
		case "false", "0", "no", "off":
			return false, true
		default:
			return false, false
		}
	case float64:
		return typed != 0, true
	case int:
		return typed != 0, true
	default:
		return false, false
	}
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case fmt.Stringer:
		return typed.String()
	default:
		if typed == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

func isoTimeFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return millisOrSecondsToISO(int64(typed))
	case int64:
		return millisOrSecondsToISO(typed)
	case int:
		return millisOrSecondsToISO(int64(typed))
	case json.Number:
		if millis, err := typed.Int64(); err == nil {
			return millisOrSecondsToISO(millis)
		}
	}
	return ""
}

func millisOrSecondsToISO(value int64) string {
	if value <= 0 {
		return ""
	}
	if value < 1_000_000_000_000 {
		value *= 1000
	}
	return time.UnixMilli(value).Format(time.RFC3339)
}

func clampNonNegativeFloat(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}
