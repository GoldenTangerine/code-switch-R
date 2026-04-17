package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ProviderQuotaQueryType string

const (
	ProviderQuotaQueryTypeNone             ProviderQuotaQueryType = "none"
	ProviderQuotaQueryTypeTokenPlanGLM     ProviderQuotaQueryType = "token_plan_glm"
	ProviderQuotaQueryTypeTokenPlanKimi    ProviderQuotaQueryType = "token_plan_kimi"
	ProviderQuotaQueryTypeTokenPlanMiniMax ProviderQuotaQueryType = "token_plan_minimax"
)

type ProviderQuotaQueryItem struct {
	Key       string  `json:"key"`
	Used      float64 `json:"used"`
	Total     float64 `json:"total"`
	NextReset string  `json:"nextReset,omitempty"`
	Active    bool    `json:"active"`
}

type ProviderQuotaQueryResult struct {
	Success   bool                     `json:"success"`
	QueryType string                   `json:"queryType"`
	Items     []ProviderQuotaQueryItem `json:"items"`
	Error     string                   `json:"error,omitempty"`
	QueriedAt int64                    `json:"queriedAt,omitempty"`
}

type ProviderQuotaQueryService struct {
	client *http.Client
}

func NewProviderQuotaQueryService() *ProviderQuotaQueryService {
	return &ProviderQuotaQueryService{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *ProviderQuotaQueryService) QueryQuota(queryType string, apiURL string, apiKey string) *ProviderQuotaQueryResult {
	normalizedType := normalizeProviderQuotaQueryType(queryType)
	result := &ProviderQuotaQueryResult{
		Success:   false,
		QueryType: string(normalizedType),
		Items:     []ProviderQuotaQueryItem{},
		QueriedAt: time.Now().UnixMilli(),
	}

	if normalizedType == ProviderQuotaQueryTypeNone {
		return result
	}

	trimmedKey := strings.TrimSpace(apiKey)
	if trimmedKey == "" {
		result.Error = "缺少 API Key"
		return result
	}

	targetBaseURL := resolveProviderQuotaQueryTargetBaseURL(normalizedType, apiURL)
	if targetBaseURL == "" {
		result.Error = "缺少可用的 API 地址"
		return result
	}

	items, err := s.queryQuotaByType(normalizedType, targetBaseURL, trimmedKey)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Success = true
	result.Items = items
	return result
}

func normalizeProviderQuotaQueryType(value string) ProviderQuotaQueryType {
	switch ProviderQuotaQueryType(strings.TrimSpace(strings.ToLower(value))) {
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

func (s *ProviderQuotaQueryService) queryQuotaByType(
	queryType ProviderQuotaQueryType,
	baseURL string,
	apiKey string,
) ([]ProviderQuotaQueryItem, error) {
	switch queryType {
	case ProviderQuotaQueryTypeTokenPlanGLM:
		return s.queryGLMQuota(baseURL, apiKey)
	case ProviderQuotaQueryTypeTokenPlanKimi:
		return s.queryKimiQuota(baseURL, apiKey)
	case ProviderQuotaQueryTypeTokenPlanMiniMax:
		return s.queryMiniMaxQuota(baseURL, apiKey)
	default:
		return nil, fmt.Errorf("不支持的供应商查询类型：%s", queryType)
	}
}

func (s *ProviderQuotaQueryService) queryGLMQuota(baseURL string, apiKey string) ([]ProviderQuotaQueryItem, error) {
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

		items = append(items, ProviderQuotaQueryItem{
			Key:       "five_hour",
			Used:      clampNonNegativeFloat(used),
			Total:     clampNonNegativeFloat(total),
			NextReset: isoTimeFromAny(limitItem["nextResetTime"]),
			Active:    true,
		})
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("GLM Token Plan 未返回可展示的额度窗口")
	}

	return items, nil
}

func (s *ProviderQuotaQueryService) queryKimiQuota(baseURL string, apiKey string) ([]ProviderQuotaQueryItem, error) {
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
			})
		}
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("Kimi Token Plan 未返回可展示的额度窗口")
	}

	return items, nil
}

func (s *ProviderQuotaQueryService) queryMiniMaxQuota(baseURL string, apiKey string) ([]ProviderQuotaQueryItem, error) {
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

func floatFromAny(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case int32:
		return float64(typed)
	case json.Number:
		result, _ := typed.Float64()
		return result
	case string:
		number, err := json.Number(strings.TrimSpace(typed)).Float64()
		if err == nil {
			return number
		}
	}
	return 0
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
