package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

type ProviderModelPerCallPrice struct {
	Unified *float64 `json:"unified,omitempty"`
	Input   *float64 `json:"input,omitempty"`
	Output  *float64 `json:"output,omitempty"`
}

type ProviderModelPricingItem struct {
	Model                         string                     `json:"model"`
	Description                   string                     `json:"description,omitempty"`
	QuotaType                     int                        `json:"quotaType"` // 0=按量，1=按次
	ModelRatio                    float64                    `json:"modelRatio"`
	CompletionRatio               float64                    `json:"completionRatio"`
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
	PerCallPrice                  *ProviderModelPerCallPrice `json:"perCallPrice,omitempty"`
}

type ProviderModelPricingResponse struct {
	SiteType      SiteType                   `json:"siteType"`
	PricingSource string                     `json:"pricingSource"`
	GroupRatio    map[string]float64         `json:"groupRatio,omitempty"`
	UsableGroup   map[string]string          `json:"usableGroup,omitempty"`
	Models        []ProviderModelPricingItem `json:"models"`
}

type providerPricingResponse struct {
	Data        []providerModelPricing `json:"data"`
	GroupRatio  map[string]float64     `json:"group_ratio"`
	Success     bool                   `json:"success"`
	UsableGroup map[string]string      `json:"usable_group"`
}

type providerModelPricing struct {
	ModelName              string          `json:"model_name"`
	ModelDescription       string          `json:"model_description,omitempty"`
	QuotaType              int             `json:"quota_type"`
	ModelRatio             float64         `json:"model_ratio"`
	ModelPrice             json.RawMessage `json:"model_price"`
	CacheCreationRatio     float64         `json:"cache_creation_ratio"`
	CacheCreateRatio       float64         `json:"cache_create_ratio"`
	CacheCreateMultiplier  float64         `json:"cache_create_multiplier"`
	CacheRatio             float64         `json:"cache_ratio"`
	CacheReadRatio         float64         `json:"cache_read_ratio"`
	CacheReadMultiplier    float64         `json:"cache_read_multiplier"`
	OwnerBy                string          `json:"owner_by,omitempty"`
	CompletionRatio        float64         `json:"completion_ratio"`
	EnableGroups           []string        `json:"enable_groups"`
	SupportedEndpointTypes []string        `json:"supported_endpoint_types"`
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
		Type   string  `json:"type"` // tokens/times
		Input  float64 `json:"input"`
		Output float64 `json:"output"`
	} `json:"price"`
}

type oneHubModelPricing map[string]oneHubModelPricingItem

type oneHubUserGroupMap map[string]struct {
	Name  string  `json:"name"`
	Ratio float64 `json:"ratio"`
}

type openAIModelList struct {
	Data []struct {
		ID string `json:"id"`
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

func (e *upstreamHTTPError) Error() string {
	if e == nil {
		return "upstream http error: <nil>"
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

func buildAuthCandidates(configured string) []string {
	configured = strings.TrimSpace(configured)
	candidates := make([]string, 0, 3)
	if configured != "" {
		candidates = append(candidates, configured)
	} else {
		candidates = append(candidates, "bearer")
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

func (ps *ProviderService) enrichProviderModelPricingResponse(response *ProviderModelPricingResponse) {
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
		if item.QuotaType != 0 {
			continue
		}
		cacheCreateMultiplier, cacheCreateSource, cacheReadMultiplier, cacheReadSource := resolveProviderCacheMultiplierDetails(*item, pricing, item.Model)
		item.ResolvedCacheCreateMultiplier = cacheCreateMultiplier
		item.ResolvedCacheReadMultiplier = cacheReadMultiplier
		item.CacheCreateMultiplierSource = cacheCreateSource
		item.CacheReadMultiplierSource = cacheReadSource
	}
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
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		ps.clearProviderModelPricingCache(apiURL, apiKey, authType)
		return nil, fmt.Errorf("apiUrl 不能为空")
	}

	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		ps.clearProviderModelPricingCache(apiURL, apiKey, authType)
		return nil, fmt.Errorf("apiKey 不能为空")
	}

	client := &http.Client{Timeout: 20 * time.Second}
	platform = strings.TrimSpace(platform)
	_ = platform // 预留：未来可按平台做更多启发式处理

	authCandidates := buildAuthCandidates(authType)

	var commonErr error
	var oneHubErr error
	for _, candidate := range authCandidates {
		commonPricing, err := fetchCommonPricing(client, apiURL, apiKey, candidate)
		if err == nil {
			response := buildProviderModelPricingResponse(SiteTypeUnknown, "api/pricing", commonPricing)
			ps.enrichProviderModelPricingResponse(response)
			ps.cacheProviderModelPricing(apiURL, apiKey, authType, response)
			return response, nil
		}
		commonErr = err

		oneHubPricing, err := fetchOneHubPricing(client, apiURL, apiKey, candidate)
		if err == nil {
			response := buildProviderModelPricingResponse(SiteTypeOneHub, "one-hub", oneHubPricing)
			ps.enrichProviderModelPricingResponse(response)
			ps.cacheProviderModelPricing(apiURL, apiKey, authType, response)
			return response, nil
		}
		oneHubErr = err

		if isAuthStatusError(commonErr) || isAuthStatusError(oneHubErr) {
			continue
		}
		break
	}

	// 兜底：尝试 /v1/models
	var modelErr error
	for _, candidate := range authCandidates {
		models, err := fetchOpenAIModels(client, apiURL, apiKey, candidate)
		if err == nil && len(models) > 0 {
			items := make([]ProviderModelPricingItem, 0, len(models))
			for _, model := range models {
				model = strings.TrimSpace(model)
				if model == "" {
					continue
				}
				items = append(items, ProviderModelPricingItem{
					Model:     model,
					QuotaType: -1,
				})
			}

			sort.Slice(items, func(i, j int) bool {
				return items[i].Model < items[j].Model
			})

			response := &ProviderModelPricingResponse{
				SiteType:      SiteTypeUnknown,
				PricingSource: "v1/models",
				Models:        items,
			}
			ps.enrichProviderModelPricingResponse(response)
			ps.cacheProviderModelPricing(apiURL, apiKey, authType, response)
			return response, nil
		}

		modelErr = err
		if isAuthStatusError(modelErr) {
			continue
		}
		break
	}

	if commonErr != nil || oneHubErr != nil {
		ps.clearProviderModelPricingCache(apiURL, apiKey, authType)
		return nil, fmt.Errorf("获取模型定价失败：%v；%v", commonErr, oneHubErr)
	}
	ps.clearProviderModelPricingCache(apiURL, apiKey, authType)
	return nil, modelErr
}

func fetchCommonPricing(client *http.Client, apiURL, apiKey, authType string) (*providerPricingResponse, error) {
	targetURL := joinURL(apiURL, "/api/pricing")
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, err
	}
	attachAuthHeader(req, apiKey, authType)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &upstreamHTTPError{
			Endpoint:   "/api/pricing",
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	var pricing providerPricingResponse
	if err := json.Unmarshal(body, &pricing); err != nil {
		return nil, fmt.Errorf("解析 /api/pricing 失败: %w", err)
	}
	if !pricing.Success {
		return nil, fmt.Errorf("/api/pricing 返回 success=false")
	}
	return &pricing, nil
}

func fetchOneHubPricing(client *http.Client, apiURL, apiKey, authType string) (*providerPricingResponse, error) {
	availableURL := joinURL(apiURL, "/api/available_model")
	groupURL := joinURL(apiURL, "/api/user_group_map")

	availableReq, err := http.NewRequest("GET", availableURL, nil)
	if err != nil {
		return nil, err
	}
	attachAuthHeader(availableReq, apiKey, authType)
	availableReq.Header.Set("Accept", "application/json")

	groupReq, err := http.NewRequest("GET", groupURL, nil)
	if err != nil {
		return nil, err
	}
	attachAuthHeader(groupReq, apiKey, authType)
	groupReq.Header.Set("Accept", "application/json")

	availableResp, err := client.Do(availableReq)
	if err != nil {
		return nil, err
	}
	defer availableResp.Body.Close()

	availableBody, err := io.ReadAll(io.LimitReader(availableResp.Body, 8*1024*1024))
	if err != nil {
		return nil, err
	}
	if availableResp.StatusCode < 200 || availableResp.StatusCode >= 300 {
		return nil, &upstreamHTTPError{
			Endpoint:   "/api/available_model",
			StatusCode: availableResp.StatusCode,
			Body:       string(availableBody),
		}
	}

	var availableEnvelope providerApiEnvelope[oneHubModelPricing]
	if err := json.Unmarshal(availableBody, &availableEnvelope); err != nil {
		return nil, fmt.Errorf("解析 /api/available_model 失败: %w", err)
	}
	if !availableEnvelope.Success {
		return nil, fmt.Errorf("/api/available_model 返回 success=false")
	}

	groupResp, err := client.Do(groupReq)
	if err != nil {
		return nil, err
	}
	defer groupResp.Body.Close()

	groupBody, err := io.ReadAll(io.LimitReader(groupResp.Body, 8*1024*1024))
	if err != nil {
		return nil, err
	}
	if groupResp.StatusCode < 200 || groupResp.StatusCode >= 300 {
		return nil, &upstreamHTTPError{
			Endpoint:   "/api/user_group_map",
			StatusCode: groupResp.StatusCode,
			Body:       string(groupBody),
		}
	}

	var groupEnvelope providerApiEnvelope[oneHubUserGroupMap]
	if err := json.Unmarshal(groupBody, &groupEnvelope); err != nil {
		return nil, fmt.Errorf("解析 /api/user_group_map 失败: %w", err)
	}
	if !groupEnvelope.Success {
		return nil, fmt.Errorf("/api/user_group_map 返回 success=false")
	}

	return transformOneHubPricing(availableEnvelope.Data, groupEnvelope.Data), nil
}

func transformOneHubPricing(models oneHubModelPricing, groupMap oneHubUserGroupMap) *providerPricingResponse {
	data := make([]providerModelPricing, 0, len(models))
	for modelName, model := range models {
		enableGroups := model.Groups
		if len(enableGroups) == 0 {
			enableGroups = []string{"default"}
		}
		quotaType := 1
		if strings.EqualFold(model.Price.Type, "tokens") {
			quotaType = 0
		}

		completionRatio := 1.0
		if model.Price.Input > 0 {
			completionRatio = model.Price.Output / model.Price.Input
		}

		modelPriceRaw, _ := json.Marshal(map[string]float64{
			"input":  model.Price.Input,
			"output": model.Price.Output,
		})

		data = append(data, providerModelPricing{
			ModelName:              modelName,
			QuotaType:              quotaType,
			ModelRatio:             1,
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

	authTypeLower := strings.ToLower(strings.TrimSpace(authType))
	switch authTypeLower {
	case "x-api-key":
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	case "", "bearer":
		req.Header.Set("Authorization", "Bearer "+apiKey)
	default:
		headerName := strings.TrimSpace(authType)
		if headerName == "" || strings.EqualFold(headerName, "custom") {
			headerName = "Authorization"
		}
		req.Header.Set(headerName, apiKey)
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
		item := ProviderModelPricingItem{
			Model:           model.ModelName,
			Description:     model.ModelDescription,
			QuotaType:       model.QuotaType,
			ModelRatio:      model.ModelRatio,
			CompletionRatio: model.CompletionRatio,
			CacheCreateMultiplier: firstPositiveFloat(
				model.CacheCreateMultiplier,
				model.CacheCreationRatio,
				model.CacheCreateRatio,
			),
			CacheReadMultiplier: firstPositiveFloat(
				model.CacheReadMultiplier,
				model.CacheReadRatio,
				model.CacheRatio,
			),
			OwnerBy:                model.OwnerBy,
			EnableGroups:           model.EnableGroups,
			SupportedEndpointTypes: model.SupportedEndpointTypes,
		}

		if model.QuotaType == 0 {
			// 参考 all-api-hub：inputUSD(每 1M) = model_ratio × 2 × groupRatio
			// outputUSD(每 1M) = model_ratio × completion_ratio × 2 × groupRatio
			item.InputUSDPerM = model.ModelRatio * 2 * groupMultiplier
			item.OutputUSDPerM = model.ModelRatio * model.CompletionRatio * 2 * groupMultiplier
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
	if err := json.Unmarshal(raw, &numberValue); err == nil {
		unified := numberValue * groupMultiplier
		return &ProviderModelPerCallPrice{Unified: &unified}
	}

	// { input, output }
	var pair struct {
		Input  float64 `json:"input"`
		Output float64 `json:"output"`
	}
	if err := json.Unmarshal(raw, &pair); err != nil {
		return nil
	}

	input := pair.Input * groupMultiplier * doneHubTokenToCallRatio
	output := pair.Output * groupMultiplier * doneHubTokenToCallRatio
	return &ProviderModelPerCallPrice{Input: &input, Output: &output}
}

func fetchOpenAIModels(client *http.Client, apiURL, apiKey, authType string) ([]string, error) {
	targetURL := joinURL(apiURL, "/v1/models")
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, err
	}
	attachAuthHeader(req, apiKey, authType)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &upstreamHTTPError{
			Endpoint:   "/v1/models",
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	var list openAIModelList
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("解析 /v1/models 失败: %w", err)
	}
	if len(list.Data) == 0 {
		return nil, fmt.Errorf("/v1/models 返回空列表")
	}

	models := make([]string, 0, len(list.Data))
	for _, item := range list.Data {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		models = append(models, item.ID)
	}
	return models, nil
}
