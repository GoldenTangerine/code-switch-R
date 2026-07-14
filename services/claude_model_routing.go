/**
 * @name: Claude 模型路由服务
 * @Descripttion: 管理 Claude 供应商模型缓存、严格路由索引与聚合模型列表
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-14 14:39:25
 * @LastEditTime: 2026-07-14 14:39:25
 * @FilePath: services/claude_model_routing.go
 */
package services

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	claudeModelRoutingCacheVersion      = 1
	claudeModelRoutingCacheFilename     = "claude-model-routing-cache.json"
	claudeModelRoutingCacheTTL          = 24 * time.Hour
	claudeModelRoutingRetryInterval     = 30 * time.Minute
	claudeModelRoutingRefreshWorkers    = 4
	claudeModelRoutingMaxProviderModels = 5000
)

type ClaudeModelRoutingStatus struct {
	Refreshing      bool     `json:"refreshing"`
	LastSuccessAt   string   `json:"lastSuccessAt,omitempty"`
	ProviderCount   int      `json:"providerCount"`
	SuccessCount    int      `json:"successCount"`
	FailureCount    int      `json:"failureCount"`
	StaleCount      int      `json:"staleCount"`
	LastFailedNames []string `json:"lastFailedNames,omitempty"`
}

type ClaudeModelRefreshResult struct {
	SuccessCount    int      `json:"successCount"`
	FailureCount    int      `json:"failureCount"`
	FailedProviders []string `json:"failedProviders,omitempty"`
	FinishedAt      string   `json:"finishedAt"`
}

type ClaudeAggregatedModel struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`
	DisplayName    string                 `json:"display_name"`
	CreatedAt      string                 `json:"created_at"`
	MaxInputTokens int64                  `json:"max_input_tokens"`
	MaxTokens      int64                  `json:"max_tokens"`
	Capabilities   map[string]interface{} `json:"capabilities"`
}

type ClaudeModelListResponse struct {
	Data    []ClaudeAggregatedModel `json:"data"`
	HasMore bool                    `json:"has_more"`
	FirstID *string                 `json:"first_id"`
	LastID  *string                 `json:"last_id"`
}

type claudeModelRouteProvider struct {
	ProviderRef    string
	ProviderID     int64
	ProviderName   string
	Level          int
	SortOrder      int
	Order          int
	EffectiveModel string
	Metadata       ProviderModelPricingItem
}

type claudeProviderModelCacheEntry struct {
	ProviderRef       string                       `json:"providerRef"`
	ProviderName      string                       `json:"providerName"`
	ConfigFingerprint string                       `json:"configFingerprint"`
	FetchedAt         time.Time                    `json:"fetchedAt"`
	LastAttemptAt     time.Time                    `json:"lastAttemptAt"`
	LastError         string                       `json:"lastError,omitempty"`
	Source            string                       `json:"source"`
	Response          ProviderModelPricingResponse `json:"response"`
}

type claudeModelRoutingCacheFile struct {
	Version   int                                      `json:"version"`
	UpdatedAt time.Time                                `json:"updatedAt"`
	Providers map[string]claudeProviderModelCacheEntry `json:"providers"`
}

type ClaudeModelRoutingService struct {
	providerService *ProviderService
	appSettings     *AppSettingsService
	modelPricing    *ModelPricingService
	cachePath       string

	mu           sync.RWMutex
	cacheWriteMu sync.Mutex
	cache        map[string]claudeProviderModelCacheEntry
	routes       map[string][]claudeModelRouteProvider
	fingerprints map[string]string
	status       ClaudeModelRoutingStatus
	refreshMu    sync.Mutex
	started      bool
	stopCancel   context.CancelFunc
	stopDone     chan struct{}
}

func NewClaudeModelRoutingService(providerService *ProviderService, appSettings *AppSettingsService, modelPricing *ModelPricingService) *ClaudeModelRoutingService {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return &ClaudeModelRoutingService{
		providerService: providerService,
		appSettings:     appSettings,
		modelPricing:    modelPricing,
		cachePath:       filepath.Join(home, ".code-switch", claudeModelRoutingCacheFilename),
		cache:           map[string]claudeProviderModelCacheEntry{},
		routes:          map[string][]claudeModelRouteProvider{},
		fingerprints:    map[string]string{},
	}
}

func (s *ClaudeModelRoutingService) Start() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = true
	s.stopDone = make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	s.stopCancel = cancel
	s.mu.Unlock()

	if err := s.loadCache(); err != nil {
		fmt.Printf("[ClaudeModelRouting] 加载缓存失败，使用本地配置重建: %v\n", err)
	}
	s.rebuildRoutes()
	go s.refreshLoop(ctx, s.stopDone)
	if s.routingEnabled() {
		go func() { _, _ = s.RefreshAll() }()
	}
	return nil
}

func (s *ClaudeModelRoutingService) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	cancel := s.stopCancel
	done := s.stopDone
	s.stopCancel = nil
	s.started = false
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	s.mu.Lock()
	if s.stopDone == done {
		s.stopDone = nil
	}
	s.mu.Unlock()
}

func (s *ClaudeModelRoutingService) refreshLoop(ctx context.Context, done chan struct{}) {
	ticker := time.NewTicker(claudeModelRoutingRetryInterval)
	defer ticker.Stop()
	defer close(done)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.routingEnabled() && s.hasRefreshDue(time.Now()) {
				go func() { _, _ = s.RefreshAll() }()
			}
		}
	}
}

func (s *ClaudeModelRoutingService) routingEnabled() bool {
	if s == nil || s.appSettings == nil {
		return false
	}
	settings, err := s.appSettings.GetAppSettings()
	return err == nil && settings.ClaudeModelRoutingEnabled
}

func (s *ClaudeModelRoutingService) aggregationSettings() (bool, string) {
	if s == nil || s.appSettings == nil {
		return false, "aggressive"
	}
	settings, err := s.appSettings.GetAppSettings()
	if err != nil || !settings.ClaudeModelRoutingEnabled {
		return false, "aggressive"
	}
	return settings.ClaudeModelAggregationEnabled, settings.ClaudeModelMetadataMergeStrategy
}

func (s *ClaudeModelRoutingService) HandleSettingsChanged(previous AppSettings, next AppSettings) {
	if s == nil {
		return
	}
	if !next.ClaudeModelRoutingEnabled {
		s.rebuildRoutes()
		return
	}
	if !previous.ClaudeModelRoutingEnabled && next.ClaudeModelRoutingEnabled {
		s.rebuildRoutes()
		go func() { _, _ = s.RefreshAll() }()
		return
	}
	if previous.ClaudeModelMetadataMergeStrategy != next.ClaudeModelMetadataMergeStrategy {
		s.rebuildRoutes()
	}
}

func (s *ClaudeModelRoutingService) HandleProvidersChanged(previous []Provider, next []Provider) {
	if s == nil {
		return
	}
	previousByRef := make(map[string]Provider, len(previous))
	for _, provider := range previous {
		previousByRef[providerRefFromProvider(provider)] = provider
	}
	nextByRef := make(map[string]Provider, len(next))
	refreshRefs := make([]string, 0)
	cacheChanged := false
	s.mu.Lock()
	for _, provider := range next {
		ref := providerRefFromProvider(provider)
		nextByRef[ref] = provider
		old, existed := previousByRef[ref]
		if provider.Enabled && (!existed || !old.Enabled || claudeProviderConnectionChanged(old, provider)) {
			refreshRefs = append(refreshRefs, ref)
		}
		if existed && claudeProviderConnectionChanged(old, provider) {
			entry := s.cache[ref]
			entry.ConfigFingerprint = ""
			s.cache[ref] = entry
			cacheChanged = true
		}
	}
	for ref := range previousByRef {
		if _, exists := nextByRef[ref]; !exists {
			delete(s.cache, ref)
			cacheChanged = true
		}
	}
	s.mu.Unlock()
	s.rebuildRoutesWithProviders(next)
	if cacheChanged {
		_ = s.saveCache()
	}
	if len(refreshRefs) > 0 {
		refs := append([]string(nil), refreshRefs...)
		go func() {
			if s.routingEnabled() {
				s.refreshProviderRefs(refs)
			}
		}()
	}
}

func claudeProviderConnectionChanged(left Provider, right Provider) bool {
	return strings.TrimSpace(left.APIURL) != strings.TrimSpace(right.APIURL) ||
		left.APIKey != right.APIKey ||
		strings.TrimSpace(left.ConnectivityAuthType) != strings.TrimSpace(right.ConnectivityAuthType) ||
		normalizeClaudeAPIFormat(left.APIFormat) != normalizeClaudeAPIFormat(right.APIFormat)
}

func (s *ClaudeModelRoutingService) RefreshAll() (ClaudeModelRefreshResult, error) {
	result := ClaudeModelRefreshResult{FinishedAt: time.Now().UTC().Format(time.RFC3339)}
	if s == nil || s.providerService == nil {
		return result, errors.New("Claude 模型路由服务未初始化")
	}
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	providers, err := s.providerService.LoadProviders("claude")
	if err != nil {
		return result, err
	}
	providers = filterRuntimeProviders("claude", providers)
	s.rebuildRoutesWithProviders(providers)
	active := make([]Provider, 0, len(providers))
	for _, provider := range providers {
		if provider.Enabled && strings.TrimSpace(provider.APIURL) != "" && providerHasRelayAuth("claude", provider) {
			active = append(active, provider)
		}
	}

	s.setRefreshing(true, len(active))
	result = s.refreshProviders(active, result)
	result.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	latestProviders, latestErr := s.providerService.LoadProviders("claude")
	if latestErr == nil {
		s.rebuildRoutesWithProviders(filterRuntimeProviders("claude", latestProviders))
	} else {
		s.rebuildRoutesWithProviders(providers)
	}
	cacheErr := s.saveCache()
	s.finishRefreshing(result)
	if latestErr != nil {
		return result, latestErr
	}
	return result, cacheErr
}

func (s *ClaudeModelRoutingService) refreshProviderRefs(refs []string) {
	if s == nil || s.providerService == nil || len(refs) == 0 {
		return
	}
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	providers, err := s.providerService.LoadProviders("claude")
	if err != nil {
		return
	}
	providers = filterRuntimeProviders("claude", providers)
	s.rebuildRoutesWithProviders(providers)
	wanted := make(map[string]bool, len(refs))
	for _, ref := range refs {
		wanted[ref] = true
	}
	active := make([]Provider, 0, len(refs))
	for _, provider := range providers {
		if wanted[providerRefFromProvider(provider)] && provider.Enabled && strings.TrimSpace(provider.APIURL) != "" && providerHasRelayAuth("claude", provider) {
			active = append(active, provider)
		}
	}
	s.refreshProviders(active, ClaudeModelRefreshResult{})
	latestProviders, latestErr := s.providerService.LoadProviders("claude")
	if latestErr == nil {
		s.rebuildRoutesWithProviders(filterRuntimeProviders("claude", latestProviders))
	} else {
		s.rebuildRoutesWithProviders(providers)
	}
	_ = s.saveCache()
}

func (s *ClaudeModelRoutingService) refreshProviders(providers []Provider, result ClaudeModelRefreshResult) ClaudeModelRefreshResult {
	jobs := make(chan Provider)
	results := make(chan struct {
		name string
		err  error
	}, len(providers))
	workerCount := claudeModelRoutingRefreshWorkers
	if len(providers) < workerCount {
		workerCount = len(providers)
	}
	var workers sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for provider := range jobs {
				results <- struct {
					name string
					err  error
				}{name: provider.Name, err: s.refreshProvider(provider)}
			}
		}()
	}
	go func() {
		for _, provider := range providers {
			jobs <- provider
		}
		close(jobs)
		workers.Wait()
		close(results)
	}()

	failed := make([]string, 0)
	for item := range results {
		if errors.Is(item.err, errClaudeModelRefreshSuperseded) {
			continue
		}
		if item.err != nil {
			result.FailureCount++
			failed = append(failed, item.name)
		} else {
			result.SuccessCount++
		}
	}
	result.FailedProviders = failed
	return result
}

var errClaudeModelRefreshSuperseded = errors.New("Claude 供应商配置已更新，忽略旧刷新结果")

func (s *ClaudeModelRoutingService) refreshProvider(provider Provider) error {
	if s == nil || s.providerService == nil || !provider.Enabled {
		return nil
	}
	response, err := s.providerService.FetchProviderModelPricingWithSource(
		provider.APIURL,
		provider.APIKey,
		"claude",
		provider.ConnectivityAuthType,
		providerModelPricingSourceAuto,
	)
	now := time.Now().UTC()
	ref := providerRefFromProvider(provider)
	fingerprint := claudeProviderConfigFingerprint(provider)
	if err != nil || response == nil || strings.TrimSpace(response.FetchError) != "" || len(response.Models) == 0 {
		message := "模型列表为空"
		if err != nil {
			message = err.Error()
		} else if response != nil && strings.TrimSpace(response.FetchError) != "" {
			message = response.FetchError
		}
		if !s.commitProviderRefreshFailure(provider, fingerprint, now, message) {
			return errClaudeModelRefreshSuperseded
		}
		return errors.New(message)
	}
	if len(response.Models) > claudeModelRoutingMaxProviderModels {
		message := fmt.Sprintf("供应商模型数量超过 %d", claudeModelRoutingMaxProviderModels)
		if !s.commitProviderRefreshFailure(provider, fingerprint, now, message) {
			return errClaudeModelRefreshSuperseded
		}
		return errors.New(message)
	}
	clean := cloneProviderModelPricingResponse(*response)
	clean.Debug = nil
	clean.FetchError = ""
	entry := claudeProviderModelCacheEntry{
		ProviderRef:       ref,
		ProviderName:      provider.Name,
		ConfigFingerprint: fingerprint,
		FetchedAt:         now,
		LastAttemptAt:     now,
		Source:            response.PricingSource,
		Response:          clean,
	}
	if !s.commitProviderRefreshSuccess(ref, fingerprint, entry) {
		return errClaudeModelRefreshSuperseded
	}
	return nil
}

func (s *ClaudeModelRoutingService) commitProviderRefreshSuccess(ref string, fingerprint string, entry claudeProviderModelCacheEntry) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fingerprints[ref] != fingerprint {
		return false
	}
	s.cache[ref] = entry
	return true
}

func (s *ClaudeModelRoutingService) commitProviderRefreshFailure(provider Provider, fingerprint string, attemptedAt time.Time, message string) bool {
	ref := providerRefFromProvider(provider)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fingerprints[ref] != fingerprint {
		return false
	}
	entry := s.cache[ref]
	if entry.ConfigFingerprint != fingerprint {
		entry = claudeProviderModelCacheEntry{
			ProviderRef:       ref,
			ProviderName:      provider.Name,
			ConfigFingerprint: fingerprint,
		}
	}
	entry.ProviderRef = ref
	entry.ProviderName = provider.Name
	entry.LastAttemptAt = attemptedAt
	entry.LastError = message
	s.cache[ref] = entry
	return true
}

func cloneProviderModelPricingResponse(source ProviderModelPricingResponse) ProviderModelPricingResponse {
	payload, err := json.Marshal(source)
	if err != nil {
		return ProviderModelPricingResponse{Models: append([]ProviderModelPricingItem(nil), source.Models...)}
	}
	var clone ProviderModelPricingResponse
	if json.Unmarshal(payload, &clone) != nil {
		return ProviderModelPricingResponse{Models: append([]ProviderModelPricingItem(nil), source.Models...)}
	}
	return clone
}

func claudeProviderConfigFingerprint(provider Provider) string {
	apiKeyHash := sha256.Sum256([]byte(strings.TrimSpace(provider.APIKey)))
	return strings.Join([]string{
		providerRefFromProvider(provider),
		strings.TrimSpace(provider.APIURL),
		strings.TrimSpace(provider.ConnectivityAuthType),
		normalizeClaudeAPIFormat(provider.APIFormat),
		fmt.Sprintf("%x", apiKeyHash[:]),
	}, "|")
}

func (s *ClaudeModelRoutingService) ResolveProviders(requestedModel string, providers []Provider) []Provider {
	if s == nil || !s.routingEnabled() || strings.TrimSpace(requestedModel) == "" {
		return providers
	}
	s.mu.RLock()
	routes := append([]claudeModelRouteProvider(nil), s.routes[strings.TrimSpace(requestedModel)]...)
	s.mu.RUnlock()
	if len(routes) == 0 {
		return nil
	}
	allowed := make(map[string]bool, len(routes))
	for _, route := range routes {
		allowed[route.ProviderRef] = true
	}
	filtered := make([]Provider, 0, len(routes))
	for _, provider := range providers {
		if allowed[providerRefFromProvider(provider)] {
			filtered = append(filtered, provider)
		}
	}
	return filtered
}

func (s *ClaudeModelRoutingService) ListModels(limit int, beforeID string, afterID string) (ClaudeModelListResponse, error) {
	response := ClaudeModelListResponse{Data: []ClaudeAggregatedModel{}}
	enabled, strategy := s.aggregationSettings()
	if !enabled {
		return response, errors.New("Claude 模型聚合未开启")
	}
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 1000 {
		return response, errors.New("limit 必须在 1 到 1000 之间")
	}
	beforeID = strings.TrimSpace(beforeID)
	afterID = strings.TrimSpace(afterID)
	if beforeID != "" && afterID != "" {
		return response, errors.New("before_id 和 after_id 不能同时使用")
	}

	s.mu.RLock()
	routeSnapshot := make(map[string][]claudeModelRouteProvider, len(s.routes))
	for model, routes := range s.routes {
		routeSnapshot[model] = append([]claudeModelRouteProvider(nil), routes...)
	}
	s.mu.RUnlock()
	ids := make([]string, 0, len(routeSnapshot))
	for model := range routeSnapshot {
		ids = append(ids, model)
	}
	sort.Strings(ids)
	start, end, hasMore, err := paginateClaudeModelIDs(ids, limit, beforeID, afterID)
	if err != nil {
		return response, err
	}
	for _, id := range ids[start:end] {
		response.Data = append(response.Data, mergeClaudeModelMetadata(id, routeSnapshot[id], strategy))
	}
	response.HasMore = hasMore
	if len(response.Data) > 0 {
		firstID := response.Data[0].ID
		lastID := response.Data[len(response.Data)-1].ID
		response.FirstID = &firstID
		response.LastID = &lastID
	}
	return response, nil
}

func paginateClaudeModelIDs(ids []string, limit int, beforeID string, afterID string) (int, int, bool, error) {
	start := 0
	end := len(ids)
	if afterID != "" {
		index := sort.SearchStrings(ids, afterID)
		if index >= len(ids) || ids[index] != afterID {
			return 0, 0, false, errors.New("after_id 不存在")
		}
		start = index + 1
		end = start + limit
		if end > len(ids) {
			end = len(ids)
		}
		return start, end, end < len(ids), nil
	}
	if beforeID != "" {
		index := sort.SearchStrings(ids, beforeID)
		if index >= len(ids) || ids[index] != beforeID {
			return 0, 0, false, errors.New("before_id 不存在")
		}
		end = index
		start = end - limit
		if start < 0 {
			start = 0
		}
		return start, end, start > 0, nil
	}
	if end > limit {
		end = limit
	}
	return start, end, end < len(ids), nil
}

func mergeClaudeModelMetadata(model string, routes []claudeModelRouteProvider, strategy string) ClaudeAggregatedModel {
	result := ClaudeAggregatedModel{
		ID:           model,
		Type:         "model",
		DisplayName:  model,
		CreatedAt:    time.Unix(0, 0).UTC().Format(time.RFC3339),
		Capabilities: map[string]interface{}{},
	}
	if len(routes) == 0 {
		return result
	}
	metadata := routes[0].Metadata
	if strings.TrimSpace(metadata.DisplayName) != "" {
		result.DisplayName = metadata.DisplayName
	}
	latestCreated := time.Time{}
	capabilityValues := make([]map[string]interface{}, 0, len(routes))
	for _, route := range routes {
		item := route.Metadata
		if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(item.CreatedAt)); err == nil && parsed.After(latestCreated) {
			latestCreated = parsed
			if strings.TrimSpace(item.DisplayName) != "" {
				result.DisplayName = item.DisplayName
			}
		}
		result.MaxInputTokens = mergePositiveInt64(result.MaxInputTokens, item.MaxInputTokens, strategy)
		result.MaxTokens = mergePositiveInt64(result.MaxTokens, item.MaxTokens, strategy)
		if len(item.Capabilities) > 0 {
			capabilityValues = append(capabilityValues, item.Capabilities)
		}
	}
	if !latestCreated.IsZero() {
		result.CreatedAt = latestCreated.UTC().Format(time.RFC3339)
	}
	result.Capabilities = mergeCapabilityMaps(capabilityValues, strategy)
	return result
}

func mergePositiveInt64(current int64, next int64, strategy string) int64 {
	if next <= 0 {
		return current
	}
	if current <= 0 {
		return next
	}
	if strategy == "conservative" && next < current {
		return next
	}
	if strategy != "conservative" && next > current {
		return next
	}
	return current
}

func mergeCapabilityMaps(values []map[string]interface{}, strategy string) map[string]interface{} {
	if len(values) == 0 {
		return map[string]interface{}{}
	}
	result := cloneInterfaceMap(values[0])
	for _, value := range values[1:] {
		result = mergeInterfaceMaps(result, value, strategy)
	}
	return result
}

func mergeInterfaceMaps(left map[string]interface{}, right map[string]interface{}, strategy string) map[string]interface{} {
	result := cloneInterfaceMap(left)
	for key, rightValue := range right {
		leftValue, exists := result[key]
		if !exists {
			result[key] = cloneInterfaceValue(rightValue)
			continue
		}
		result[key] = mergeInterfaceValue(leftValue, rightValue, strategy)
	}
	return result
}

func mergeInterfaceValue(left interface{}, right interface{}, strategy string) interface{} {
	switch leftValue := left.(type) {
	case bool:
		if rightValue, ok := right.(bool); ok {
			if strategy == "conservative" {
				return leftValue && rightValue
			}
			return leftValue || rightValue
		}
	case float64:
		if rightValue, ok := right.(float64); ok {
			if strategy == "conservative" {
				if leftValue <= 0 {
					return rightValue
				}
				if rightValue > 0 && rightValue < leftValue {
					return rightValue
				}
				return leftValue
			}
			if rightValue > leftValue {
				return rightValue
			}
			return leftValue
		}
	case map[string]interface{}:
		if rightValue, ok := right.(map[string]interface{}); ok {
			return mergeInterfaceMaps(leftValue, rightValue, strategy)
		}
	case []interface{}:
		if rightValue, ok := right.([]interface{}); ok {
			return mergeInterfaceSlices(leftValue, rightValue, strategy)
		}
	}
	return cloneInterfaceValue(left)
}

func mergeInterfaceSlices(left []interface{}, right []interface{}, strategy string) []interface{} {
	if strategy == "conservative" {
		result := make([]interface{}, 0)
		for _, leftValue := range left {
			for _, rightValue := range right {
				if reflect.DeepEqual(leftValue, rightValue) {
					result = append(result, cloneInterfaceValue(leftValue))
					break
				}
			}
		}
		return result
	}
	result := append([]interface{}{}, left...)
	for _, rightValue := range right {
		found := false
		for _, current := range result {
			if reflect.DeepEqual(current, rightValue) {
				found = true
				break
			}
		}
		if !found {
			result = append(result, cloneInterfaceValue(rightValue))
		}
	}
	return result
}

func cloneInterfaceMap(source map[string]interface{}) map[string]interface{} {
	clone := make(map[string]interface{}, len(source))
	for key, value := range source {
		clone[key] = cloneInterfaceValue(value)
	}
	return clone
}

func cloneInterfaceValue(value interface{}) interface{} {
	payload, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var clone interface{}
	if json.Unmarshal(payload, &clone) != nil {
		return value
	}
	return clone
}

func (s *ClaudeModelRoutingService) GetStatus() ClaudeModelRoutingStatus {
	if s == nil {
		return ClaudeModelRoutingStatus{}
	}
	s.mu.RLock()
	status := s.status
	status.LastFailedNames = append([]string(nil), s.status.LastFailedNames...)
	cache := make(map[string]claudeProviderModelCacheEntry, len(s.cache))
	for key, entry := range s.cache {
		cache[key] = entry
	}
	fingerprints := make(map[string]string, len(s.fingerprints))
	for key, fingerprint := range s.fingerprints {
		fingerprints[key] = fingerprint
	}
	s.mu.RUnlock()
	now := time.Now()
	status.ProviderCount = len(fingerprints)
	status.StaleCount = 0
	for ref, fingerprint := range fingerprints {
		entry, exists := cache[ref]
		if !exists || entry.ConfigFingerprint != fingerprint || entry.FetchedAt.IsZero() || now.Sub(entry.FetchedAt) >= claudeModelRoutingCacheTTL {
			status.StaleCount++
		}
	}
	return status
}

func (s *ClaudeModelRoutingService) rebuildRoutes() {
	if s == nil || s.providerService == nil {
		return
	}
	providers, err := s.providerService.LoadProviders("claude")
	if err != nil {
		return
	}
	s.rebuildRoutesWithProviders(filterRuntimeProviders("claude", providers))
}

func (s *ClaudeModelRoutingService) rebuildRoutesWithProviders(providers []Provider) {
	routes := make(map[string][]claudeModelRouteProvider)
	fingerprints := make(map[string]string, len(providers))
	localModels := s.localModelItems()
	s.mu.RLock()
	cache := make(map[string]claudeProviderModelCacheEntry, len(s.cache))
	for key, entry := range s.cache {
		cache[key] = entry
	}
	s.mu.RUnlock()
	for order, provider := range providers {
		if !provider.Enabled || strings.TrimSpace(provider.APIURL) == "" || !providerHasRelayAuth("claude", provider) {
			continue
		}
		fingerprints[providerRefFromProvider(provider)] = claudeProviderConfigFingerprint(provider)
		actualModels := resolveClaudeProviderActualModels(provider, cache[providerRefFromProvider(provider)], localModels)
		providerRoutes := buildClaudeProviderRoutes(provider, actualModels, localModels, order)
		for model, route := range providerRoutes {
			routes[model] = append(routes[model], route)
		}
	}
	for model := range routes {
		sort.SliceStable(routes[model], func(i, j int) bool {
			left := routes[model][i]
			right := routes[model][j]
			if left.Level != right.Level {
				return left.Level < right.Level
			}
			if left.SortOrder != right.SortOrder {
				return left.SortOrder < right.SortOrder
			}
			return left.Order < right.Order
		})
	}
	s.mu.Lock()
	s.routes = routes
	s.fingerprints = fingerprints
	s.mu.Unlock()
}

func (s *ClaudeModelRoutingService) HandleModelLibraryChanged() {
	if s == nil {
		return
	}
	s.rebuildRoutes()
}

func (s *ClaudeModelRoutingService) localModelItems() map[string]ProviderModelPricingItem {
	items := map[string]ProviderModelPricingItem{}
	if s == nil || s.modelPricing == nil {
		return items
	}
	service := s.modelPricing.Service()
	if service == nil {
		return items
	}
	for _, model := range service.Models() {
		entry, ok := service.PricingEntryExact(model)
		if !ok {
			continue
		}
		items[model] = ProviderModelPricingItem{
			Model:          model,
			DisplayName:    model,
			MaxInputTokens: entry.MaxInputTokens,
			MaxTokens:      entry.MaxTokens,
			Capabilities:   pricingEntryCapabilities(entry),
		}
	}
	return items
}

func resolveClaudeProviderActualModels(provider Provider, cache claudeProviderModelCacheEntry, localModels map[string]ProviderModelPricingItem) map[string]ProviderModelPricingItem {
	actual := map[string]ProviderModelPricingItem{}
	if cache.ConfigFingerprint == claudeProviderConfigFingerprint(provider) && len(cache.Response.Models) > 0 {
		for _, item := range cache.Response.Models {
			model := strings.TrimSpace(item.Model)
			if model == "" || (len(provider.SupportedModels) > 0 && !provider.IsNativeModelSupported(model)) {
				continue
			}
			actual[model] = item
		}
		return actual
	}
	for pattern, enabled := range provider.SupportedModels {
		if !enabled {
			continue
		}
		if !strings.Contains(pattern, "*") {
			item := localModels[pattern]
			if item.Model == "" {
				item = ProviderModelPricingItem{Model: pattern, DisplayName: pattern}
			}
			actual[pattern] = item
			continue
		}
		for model, item := range localModels {
			if matchWildcard(pattern, model) {
				actual[model] = item
			}
		}
	}
	if len(provider.SupportedModels) == 0 {
		for _, pattern := range provider.ModelMapping {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				continue
			}
			if !strings.Contains(pattern, "*") {
				item := localModels[pattern]
				if item.Model == "" {
					item = ProviderModelPricingItem{Model: pattern, DisplayName: pattern}
				}
				actual[pattern] = item
				continue
			}
			for model, item := range localModels {
				if matchWildcard(pattern, model) {
					actual[model] = item
				}
			}
		}
		if override, ok := provider.RequestBodyOverrides["model"].(string); ok {
			override = strings.TrimSpace(override)
			if override != "" {
				item := localModels[override]
				if item.Model == "" {
					item = ProviderModelPricingItem{Model: override, DisplayName: override}
				}
				actual[override] = item
			}
		}
	}
	return actual
}

func buildClaudeProviderRoutes(provider Provider, actual map[string]ProviderModelPricingItem, localModels map[string]ProviderModelPricingItem, order int) map[string]claudeModelRouteProvider {
	routes := map[string]claudeModelRouteProvider{}
	addRoute := func(requestedModel string, mappedModel string) {
		requestedModel = strings.TrimSpace(requestedModel)
		mappedModel = strings.TrimSpace(mappedModel)
		if requestedModel == "" || mappedModel == "" {
			return
		}
		finalModel := mappedModel
		if override, ok := provider.RequestBodyOverrides["model"].(string); ok && strings.TrimSpace(override) != "" {
			finalModel = strings.TrimSpace(override)
		}
		metadata, supported := actual[finalModel]
		if !supported {
			return
		}
		level := provider.Level
		if level <= 0 {
			level = 1
		}
		routes[requestedModel] = claudeModelRouteProvider{
			ProviderRef:    providerRefFromProvider(provider),
			ProviderID:     provider.ID,
			ProviderName:   provider.Name,
			Level:          level,
			SortOrder:      provider.SortOrder,
			Order:          order,
			EffectiveModel: finalModel,
			Metadata:       metadata,
		}
	}

	if len(provider.ModelMapping) > 0 {
		candidates := map[string]bool{}
		keys := make([]string, 0, len(provider.ModelMapping))
		for key := range provider.ModelMapping {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, requestedPattern := range keys {
			actualPattern := provider.ModelMapping[requestedPattern]
			if !strings.Contains(requestedPattern, "*") {
				candidates[requestedPattern] = true
				continue
			}
			if strings.Count(requestedPattern, "*") != 1 || strings.Count(actualPattern, "*") > 1 {
				continue
			}
			if strings.Contains(actualPattern, "*") {
				for actualModel := range actual {
					if requestedModel, ok := reverseWildcardMapping(requestedPattern, actualPattern, actualModel); ok {
						candidates[requestedModel] = true
					}
				}
				continue
			}
			if _, supported := actual[actualPattern]; supported {
				for requestedModel := range localModels {
					if matchWildcard(requestedPattern, requestedModel) {
						candidates[requestedModel] = true
					}
				}
			}
		}
		requestedModels := make([]string, 0, len(candidates))
		for requestedModel := range candidates {
			requestedModels = append(requestedModels, requestedModel)
		}
		sort.Strings(requestedModels)
		for _, requestedModel := range requestedModels {
			if mappedModel, matched := provider.resolveModelMapping(requestedModel); matched {
				addRoute(requestedModel, mappedModel)
			}
		}
	} else {
		for model := range actual {
			addRoute(model, model)
		}
	}

	if normalizeModelMappingMissPolicy(provider.ModelMappingMissPolicy) == ModelMappingMissPolicyPassthrough {
		patterns := normalizeModelPassthroughPatterns(provider.ModelPassthroughPatterns)
		if len(patterns) == 0 {
			patterns = []string{"*"}
		}
		for _, pattern := range patterns {
			for model := range actual {
				if !provider.hasModelMappingForModel(model) && matchModelPassthroughPattern(pattern, model) {
					addRoute(model, model)
				}
			}
		}
	}
	return routes
}

func reverseWildcardMapping(requestedPattern string, actualPattern string, actualModel string) (string, bool) {
	parts := strings.Split(actualPattern, "*")
	if len(parts) != 2 || !strings.HasPrefix(actualModel, parts[0]) || !strings.HasSuffix(actualModel, parts[1]) {
		return "", false
	}
	end := len(actualModel) - len(parts[1])
	if end < len(parts[0]) {
		return "", false
	}
	capture := actualModel[len(parts[0]):end]
	requested := strings.Replace(requestedPattern, "*", capture, 1)
	return requested, applyWildcardMapping(requestedPattern, actualPattern, requested) == actualModel
}

func normalizeModelPassthroughPatterns(patterns []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" || seen[pattern] {
			continue
		}
		seen[pattern] = true
		result = append(result, pattern)
	}
	return result
}

func matchModelPassthroughPattern(pattern string, model string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == model
	}
	position := 0
	if parts[0] != "" {
		if !strings.HasPrefix(model, parts[0]) {
			return false
		}
		position = len(parts[0])
	}
	for index := 1; index < len(parts); index++ {
		part := parts[index]
		if part == "" {
			continue
		}
		found := strings.Index(model[position:], part)
		if found < 0 {
			return false
		}
		position += found + len(part)
		if index == len(parts)-1 && !strings.HasSuffix(model, part) {
			return false
		}
	}
	return parts[len(parts)-1] == "" || position == len(model)
}

func (s *ClaudeModelRoutingService) loadCache() error {
	data, err := os.ReadFile(s.cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var file claudeModelRoutingCacheFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}
	if file.Version != claudeModelRoutingCacheVersion {
		return fmt.Errorf("不支持的缓存版本: %d", file.Version)
	}
	if file.Providers == nil {
		file.Providers = map[string]claudeProviderModelCacheEntry{}
	}
	s.mu.Lock()
	s.cache = file.Providers
	s.mu.Unlock()
	return nil
}

func (s *ClaudeModelRoutingService) saveCache() error {
	s.cacheWriteMu.Lock()
	defer s.cacheWriteMu.Unlock()
	s.mu.RLock()
	providers := make(map[string]claudeProviderModelCacheEntry, len(s.cache))
	for key, entry := range s.cache {
		providers[key] = entry
	}
	s.mu.RUnlock()
	payload, err := json.MarshalIndent(claudeModelRoutingCacheFile{
		Version:   claudeModelRoutingCacheVersion,
		UpdatedAt: time.Now().UTC(),
		Providers: providers,
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.cachePath), 0o755); err != nil {
		return err
	}
	return atomicWriteFile(s.cachePath, payload, 0o644)
}

func (s *ClaudeModelRoutingService) setRefreshing(refreshing bool, providerCount int) {
	s.mu.Lock()
	s.status.Refreshing = refreshing
	s.status.ProviderCount = providerCount
	s.mu.Unlock()
}

func (s *ClaudeModelRoutingService) finishRefreshing(result ClaudeModelRefreshResult) {
	s.mu.Lock()
	s.status.Refreshing = false
	s.status.SuccessCount = result.SuccessCount
	s.status.FailureCount = result.FailureCount
	s.status.LastFailedNames = append([]string(nil), result.FailedProviders...)
	if result.SuccessCount > 0 {
		s.status.LastSuccessAt = result.FinishedAt
	}
	s.mu.Unlock()
}

func (s *ClaudeModelRoutingService) hasRefreshDue(now time.Time) bool {
	if s == nil || s.providerService == nil {
		return false
	}
	providers, err := s.providerService.LoadProviders("claude")
	if err != nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, provider := range providers {
		if !provider.Enabled {
			continue
		}
		entry, exists := s.cache[providerRefFromProvider(provider)]
		if !exists || entry.ConfigFingerprint != claudeProviderConfigFingerprint(provider) || entry.FetchedAt.IsZero() || now.Sub(entry.FetchedAt) >= claudeModelRoutingCacheTTL {
			return true
		}
		if entry.LastError != "" && now.Sub(entry.LastAttemptAt) >= claudeModelRoutingRetryInterval {
			return true
		}
	}
	return false
}

func parseClaudeModelListLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 20, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errors.New("limit 必须是整数")
	}
	return limit, nil
}
