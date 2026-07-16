/**
 * @name: Claude 模型路由测试
 * @Descripttion: 验证 Claude 模型路由、聚合、迁移与缓存行为
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-14 14:39:25
 * @LastEditTime: 2026-07-14 14:39:25
 * @FilePath: services/claude_model_routing_test.go
 */
package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildClaudeProviderRoutesUsesRequestedAndActualModels(t *testing.T) {
	provider := Provider{
		ID:      1,
		Name:    "B",
		Enabled: true,
		Level:   1,
		ModelMapping: map[string]string{
			"claude-5": "vendor/claude-5",
		},
	}
	actual := map[string]ProviderModelPricingItem{
		"vendor/claude-5": {Model: "vendor/claude-5", MaxInputTokens: 200000},
	}
	routes := buildClaudeProviderRoutes(provider, actual, nil, 0)
	got, ok := routes["claude-5"]
	if !ok {
		t.Fatal("未生成 claude-5 路由")
	}
	if got.EffectiveModel != "vendor/claude-5" {
		t.Fatalf("实际模型 = %q", got.EffectiveModel)
	}
	if _, exists := routes["claude-4-5"]; exists {
		t.Fatal("不应生成跨模型降级路由")
	}
}

func TestBuildClaudeProviderRoutesTrustsExplicitMappingWithoutActualTarget(t *testing.T) {
	provider := Provider{
		ID:      1,
		Name:    "Trusted Mapping",
		Enabled: true,
		ModelMapping: map[string]string{
			"claude-opus-*": "kimi-k2.7",
		},
	}
	actual := map[string]ProviderModelPricingItem{
		"kimi-for-coding": {Model: "kimi-for-coding", DisplayName: "Kimi for Coding"},
	}
	localModels := map[string]ProviderModelPricingItem{
		"claude-opus-4.8": {Model: "claude-opus-4.8", DisplayName: "Claude Opus 4.8", MaxInputTokens: 200000},
	}
	routes := buildClaudeProviderRoutes(provider, actual, localModels, 0)
	route, ok := routes["claude-opus-4.8"]
	if !ok {
		t.Fatal("显式映射不应依赖供应商真实模型列表")
	}
	if route.EffectiveModel != "kimi-k2.7" {
		t.Fatalf("实际模型 = %q，期望 kimi-k2.7", route.EffectiveModel)
	}
	if route.Metadata.DisplayName != "Claude Opus 4.8" || route.Metadata.MaxInputTokens != 200000 {
		t.Fatalf("缺少目标模型元数据时应回退到请求模型元数据: %#v", route.Metadata)
	}
	if _, exists := routes["claude-opus-*"]; exists {
		t.Fatal("聚合路由不应暴露通配符模型名")
	}
}

func TestBuildClaudeProviderRoutesExcludesDisabledMappings(t *testing.T) {
	provider := Provider{
		ID:      1,
		Name:    "Disabled Mapping",
		Enabled: true,
		ModelMapping: map[string]string{
			"claude-opus-*":   "vendor-opus-*",
			"claude-sonnet-*": "vendor-sonnet-*",
		},
		ModelMappingDisabled: map[string]bool{
			"claude-opus-*": true,
		},
	}
	localModels := map[string]ProviderModelPricingItem{
		"claude-opus-4.8":   {Model: "claude-opus-4.8"},
		"claude-sonnet-4.8": {Model: "claude-sonnet-4.8"},
	}
	routes := buildClaudeProviderRoutes(provider, nil, localModels, 0)
	if _, exists := routes["claude-opus-4.8"]; exists {
		t.Fatal("关闭的映射不应生成 Claude 路由")
	}
	if _, exists := routes["claude-sonnet-4.8"]; !exists {
		t.Fatal("开启的映射应继续生成 Claude 路由")
	}
}

func TestResolveClaudeProviderActualModelsFallsBackToMappingTargets(t *testing.T) {
	provider := Provider{ModelMapping: map[string]string{
		"claude-5": "vendor/claude-5",
	}}
	actual := resolveClaudeProviderActualModels(provider, claudeProviderModelCacheEntry{}, nil)
	if _, ok := actual["vendor/claude-5"]; !ok {
		t.Fatal("映射目标应在远端缓存缺失时生成临时实际模型")
	}
}

func TestResolveClaudeProviderActualModelsExcludesDisabledMappingTargets(t *testing.T) {
	provider := Provider{
		ModelMapping: map[string]string{
			"claude-opus-*":   "vendor-opus-*",
			"claude-sonnet-*": "vendor-sonnet-*",
		},
		ModelMappingDisabled: map[string]bool{"claude-opus-*": true},
	}
	actual := resolveClaudeProviderActualModels(provider, claudeProviderModelCacheEntry{}, map[string]ProviderModelPricingItem{
		"vendor-opus-4.8":   {Model: "vendor-opus-4.8"},
		"vendor-sonnet-4.8": {Model: "vendor-sonnet-4.8"},
	})
	if _, exists := actual["vendor-opus-4.8"]; exists {
		t.Fatal("关闭映射的目标模型不应进入实际模型集合")
	}
	if _, exists := actual["vendor-sonnet-4.8"]; !exists {
		t.Fatal("开启映射的目标模型应进入实际模型集合")
	}
}

func TestClaudeModelRoutingResolveProvidersHonorsSwitch(t *testing.T) {
	useIsolatedHomeDir(t)
	appSettings := NewAppSettingsService(nil)
	settings, err := appSettings.GetAppSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ClaudeModelRoutingEnabled = true
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatal(err)
	}
	service := NewClaudeModelRoutingService(nil, appSettings, nil)
	service.routes = map[string][]claudeModelRouteProvider{
		"claude-5": {{ProviderRef: "2"}, {ProviderRef: "3"}},
	}
	providers := []Provider{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}, {ID: 3, Name: "C"}}
	resolved := service.ResolveProviders("claude-5", providers)
	if len(resolved) != 2 || resolved[0].Name != "B" || resolved[1].Name != "C" {
		t.Fatalf("路由结果 = %#v", resolved)
	}
	settings.ClaudeModelRoutingEnabled = false
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatal(err)
	}
	resolved = service.ResolveProviders("claude-5", providers)
	if len(resolved) != 3 {
		t.Fatalf("关闭路由时应保留全部供应商: %#v", resolved)
	}
}

func TestClaudeModelRoutingResolveProvidersMatchesMappingOutsideRouteIndex(t *testing.T) {
	useIsolatedHomeDir(t)
	appSettings := NewAppSettingsService(nil)
	settings, err := appSettings.GetAppSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ClaudeModelRoutingEnabled = true
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatal(err)
	}
	service := NewClaudeModelRoutingService(nil, appSettings, nil)
	providers := []Provider{
		{ID: 1, Name: "No Match", ModelMapping: map[string]string{"claude-sonnet-*": "vendor-sonnet"}},
		{ID: 2, Name: "Match", ModelMapping: map[string]string{"claude-opus-*": "kimi-k2.7"}},
		{ID: 3, Name: "Verified", SupportedModels: map[string]bool{"claude-opus-4.8": true}},
	}
	service.routes = map[string][]claudeModelRouteProvider{
		"claude-opus-4.8": {{ProviderRef: "3"}},
	}
	resolved := service.ResolveProviders("claude-opus-4.9-unlisted", providers)
	if len(resolved) != 1 || resolved[0].Name != "Match" {
		t.Fatalf("模型库外请求应按映射动态匹配供应商: %#v", resolved)
	}
	resolved = service.ResolveProviders("claude-opus-4.8", providers)
	if len(resolved) != 2 || resolved[0].Name != "Match" || resolved[1].Name != "Verified" {
		t.Fatalf("动态映射与已验证路由应合并并保留供应商顺序: %#v", resolved)
	}
}

func TestClaudeModelRoutingResolveProvidersHonorsPassthroughWhenMappingsDisabled(t *testing.T) {
	useIsolatedHomeDir(t)
	appSettings := NewAppSettingsService(nil)
	settings, err := appSettings.GetAppSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ClaudeModelRoutingEnabled = true
	if _, err := appSettings.SaveAppSettings(settings); err != nil {
		t.Fatal(err)
	}
	service := NewClaudeModelRoutingService(nil, appSettings, nil)
	providers := []Provider{
		{
			ID:                     1,
			Name:                   "Passthrough",
			ModelMapping:           map[string]string{"claude-*": "vendor-*"},
			ModelMappingDisabled:   map[string]bool{"claude-*": true},
			ModelMappingMissPolicy: ModelMappingMissPolicyPassthrough,
		},
		{
			ID:                       2,
			Name:                     "Pattern Miss",
			ModelMapping:             map[string]string{"claude-*": "vendor-*"},
			ModelMappingDisabled:     map[string]bool{"claude-*": true},
			ModelMappingMissPolicy:   ModelMappingMissPolicyPassthrough,
			ModelPassthroughPatterns: []string{"glm-*"},
		},
		{
			ID:                     3,
			Name:                   "Blocked",
			ModelMapping:           map[string]string{"claude-*": "vendor-*"},
			ModelMappingDisabled:   map[string]bool{"claude-*": true},
			ModelMappingMissPolicy: ModelMappingMissPolicyBlock,
		},
	}

	resolved := service.ResolveProviders("claude-opus-4.8", providers)
	if len(resolved) != 1 || resolved[0].Name != "Passthrough" {
		t.Fatalf("全部映射关闭后应继续按未命中透传策略筛选供应商: %#v", resolved)
	}
}

func TestBuildClaudeProviderRoutesExpandsWildcardFromLocalModels(t *testing.T) {
	provider := Provider{
		ID:      2,
		Name:    "Wildcard",
		Enabled: true,
		ModelMapping: map[string]string{
			"claude-*": "anthropic/claude-*",
		},
	}
	actual := map[string]ProviderModelPricingItem{}
	localModels := map[string]ProviderModelPricingItem{
		"claude-opus-5": {Model: "claude-opus-5"},
	}
	routes := buildClaudeProviderRoutes(provider, actual, localModels, 0)
	if got := routes["claude-opus-5"].EffectiveModel; got != "anthropic/claude-opus-5" {
		t.Fatalf("通配符模型库展开结果 = %q", got)
	}
}

func TestBuildClaudeProviderRoutesUsesExplicitPassthroughRules(t *testing.T) {
	provider := Provider{
		ID:                       3,
		Name:                     "Pass",
		Enabled:                  true,
		ModelMappingMissPolicy:   ModelMappingMissPolicyPassthrough,
		ModelPassthroughPatterns: []string{"*glm*"},
		ModelMapping:             map[string]string{"claude-*": "anthropic/claude-*"},
	}
	actual := map[string]ProviderModelPricingItem{
		"glm-5":       {Model: "glm-5"},
		"deepseek-v3": {Model: "deepseek-v3"},
	}
	routes := buildClaudeProviderRoutes(provider, actual, nil, 0)
	if _, ok := routes["glm-5"]; !ok {
		t.Fatal("显式放过规则未生成路由")
	}
	if _, ok := routes["deepseek-v3"]; ok {
		t.Fatal("未命中放过规则的模型不应进入路由")
	}
}

func TestBuildClaudeProviderRoutesDefaultsEmptyPassthroughRulesToVerifiedModels(t *testing.T) {
	provider := Provider{
		ID:                     4,
		Name:                   "DefaultPass",
		Enabled:                true,
		ModelMappingMissPolicy: ModelMappingMissPolicyPassthrough,
		ModelMapping:           map[string]string{"claude-*": "anthropic/claude-*"},
	}
	actual := map[string]ProviderModelPricingItem{
		"glm-5": {Model: "glm-5"},
	}
	routes := buildClaudeProviderRoutes(provider, actual, map[string]ProviderModelPricingItem{
		"glm-5":         {Model: "glm-5"},
		"unknown-model": {Model: "unknown-model"},
	}, 0)
	if _, ok := routes["glm-5"]; !ok {
		t.Fatal("空放过规则应默认放过已验证模型")
	}
	if _, ok := routes["unknown-model"]; ok {
		t.Fatal("空放过规则不应放过未验证模型")
	}
}

func TestBuildClaudeProviderRoutesUsesSameWildcardPrecedenceAsForwarding(t *testing.T) {
	provider := Provider{
		ID:      5,
		Name:    "Overlap",
		Enabled: true,
		ModelMapping: map[string]string{
			"claude-*":  "vendor-a/*",
			"claude-5*": "vendor-b/*",
		},
	}
	actual := map[string]ProviderModelPricingItem{
		"vendor-b/-sonnet": {Model: "vendor-b/-sonnet"},
	}
	localModels := map[string]ProviderModelPricingItem{
		"claude-5-sonnet": {Model: "claude-5-sonnet"},
	}
	routes := buildClaudeProviderRoutes(provider, actual, localModels, 0)
	route, ok := routes["claude-5-sonnet"]
	if !ok {
		t.Fatal("重叠通配符未生成路由")
	}
	if route.EffectiveModel != provider.GetEffectiveModel("claude-5-sonnet") {
		t.Fatalf("索引模型 %q 与真实转发模型 %q 不一致", route.EffectiveModel, provider.GetEffectiveModel("claude-5-sonnet"))
	}
}

func TestProviderModelMappingWildcardPrecedenceIsStable(t *testing.T) {
	provider := Provider{ModelMapping: map[string]string{
		"claude-*":         "broad/*",
		"claude-5-*":       "specific/*",
		"claude-5-sonnet*": "most-specific/*",
	}}
	for index := 0; index < 100; index++ {
		if got := provider.GetEffectiveModel("claude-5-sonnet-latest"); got != "most-specific/-latest" {
			t.Fatalf("第 %d 次解析结果 = %q", index+1, got)
		}
	}
}

func TestClaudeProviderConfigFingerprintIncludesHashedAPIKey(t *testing.T) {
	provider := Provider{ID: 1, APIURL: "https://example.com", APIKey: "secret-a", APIFormat: "anthropic"}
	left := claudeProviderConfigFingerprint(provider)
	provider.APIKey = "secret-b"
	right := claudeProviderConfigFingerprint(provider)
	if left == right {
		t.Fatal("API Key 变化后配置指纹未变化")
	}
	if strings.Contains(left, "secret-a") || strings.Contains(right, "secret-b") {
		t.Fatal("配置指纹不应包含原始 API Key")
	}
}

func TestClaudeModelRefreshFailureDoesNotReuseDifferentConfigCache(t *testing.T) {
	provider := Provider{ID: 1, Name: "New", APIURL: "https://new.example.com", APIKey: "new-key", Enabled: true}
	ref := providerRefFromProvider(provider)
	fingerprint := claudeProviderConfigFingerprint(provider)
	service := &ClaudeModelRoutingService{
		cache: map[string]claudeProviderModelCacheEntry{
			ref: {
				ConfigFingerprint: "old-fingerprint",
				FetchedAt:         time.Now().UTC(),
				Response:          ProviderModelPricingResponse{Models: []ProviderModelPricingItem{{Model: "old-model"}}},
			},
		},
		fingerprints: map[string]string{ref: fingerprint},
	}
	if !service.commitProviderRefreshFailure(provider, fingerprint, time.Now().UTC(), "failed") {
		t.Fatal("当前配置的失败结果应被记录")
	}
	entry := service.cache[ref]
	if !entry.FetchedAt.IsZero() || len(entry.Response.Models) != 0 {
		t.Fatalf("不同配置不应复用旧缓存: %#v", entry)
	}
}

func TestClaudeModelRefreshDiscardsSupersededSuccess(t *testing.T) {
	service := &ClaudeModelRoutingService{
		cache:        map[string]claudeProviderModelCacheEntry{},
		fingerprints: map[string]string{"1": "new-fingerprint"},
	}
	committed := service.commitProviderRefreshSuccess("1", "old-fingerprint", claudeProviderModelCacheEntry{
		ConfigFingerprint: "old-fingerprint",
		Response:          ProviderModelPricingResponse{Models: []ProviderModelPricingItem{{Model: "old-model"}}},
	})
	if committed {
		t.Fatal("旧配置的迟到结果不应提交")
	}
	if _, exists := service.cache["1"]; exists {
		t.Fatal("旧配置结果污染了当前缓存")
	}
}

func TestClaudeProviderChangeRebuildsRoutesBeforeReturn(t *testing.T) {
	oldProvider := Provider{
		ID:           1,
		Name:         "Provider",
		APIURL:       "https://old.example.com",
		APIKey:       "old-key",
		Enabled:      true,
		ModelMapping: map[string]string{"claude-old": "vendor/old"},
	}
	newProvider := oldProvider
	newProvider.APIURL = "https://new.example.com"
	newProvider.APIKey = "new-key"
	newProvider.ModelMapping = map[string]string{"claude-new": "vendor/new"}
	ref := providerRefFromProvider(newProvider)
	service := &ClaudeModelRoutingService{
		cachePath: filepath.Join(t.TempDir(), "cache.json"),
		cache: map[string]claudeProviderModelCacheEntry{
			ref: {
				ConfigFingerprint: claudeProviderConfigFingerprint(oldProvider),
				FetchedAt:         time.Now().UTC(),
				Response:          ProviderModelPricingResponse{Models: []ProviderModelPricingItem{{Model: "vendor/old"}}},
			},
		},
		routes:       map[string][]claudeModelRouteProvider{},
		fingerprints: map[string]string{},
	}
	service.HandleProvidersChanged([]Provider{oldProvider}, []Provider{newProvider})
	if _, ok := service.routes["claude-new"]; !ok {
		t.Fatal("供应商保存返回前未生成新路由")
	}
	if _, ok := service.routes["claude-old"]; ok {
		t.Fatal("供应商保存返回后仍保留旧路由")
	}
	if service.cache[ref].ConfigFingerprint != "" {
		t.Fatal("连接配置变化后旧缓存未立即失效")
	}
}

func TestClaudeModelRoutingRefreshDueWhenFingerprintChanges(t *testing.T) {
	home := useIsolatedHomeDir(t)
	providerService := NewProviderService()
	provider := Provider{ID: 1, Name: "Provider", APIURL: "https://example.com", APIKey: "new-key", Enabled: true}
	if err := providerService.SaveProviders("claude", []Provider{provider}); err != nil {
		t.Fatal(err)
	}
	service := NewClaudeModelRoutingService(providerService, nil, nil)
	service.cachePath = filepath.Join(home, "cache.json")
	service.cache[providerRefFromProvider(provider)] = claudeProviderModelCacheEntry{
		ConfigFingerprint: "old-fingerprint",
		FetchedAt:         time.Now().UTC(),
	}
	if !service.hasRefreshDue(time.Now()) {
		t.Fatal("配置指纹变化后应立即刷新")
	}
}

func TestClaudeModelRoutingStatusCountsFingerprintMismatchAsStale(t *testing.T) {
	service := &ClaudeModelRoutingService{
		cache: map[string]claudeProviderModelCacheEntry{
			"1": {ConfigFingerprint: "old", FetchedAt: time.Now().UTC()},
		},
		fingerprints: map[string]string{"1": "new"},
	}
	status := service.GetStatus()
	if status.ProviderCount != 1 || status.StaleCount != 1 {
		t.Fatalf("状态统计错误: %#v", status)
	}
}

func TestMergeClaudeModelMetadataStrategies(t *testing.T) {
	routes := []claudeModelRouteProvider{
		{Metadata: ProviderModelPricingItem{
			Model: "vendor-a", MaxInputTokens: 100000, MaxTokens: 8000,
			Capabilities: map[string]interface{}{"thinking": map[string]interface{}{"supported": true}},
		}},
		{Metadata: ProviderModelPricingItem{
			Model: "vendor-b", MaxInputTokens: 200000, MaxTokens: 16000,
			Capabilities: map[string]interface{}{"thinking": map[string]interface{}{"supported": false}},
		}},
	}
	aggressive := mergeClaudeModelMetadata("claude-5", routes, "aggressive")
	if aggressive.MaxInputTokens != 200000 || aggressive.MaxTokens != 16000 {
		t.Fatalf("激进策略上限错误: %#v", aggressive)
	}
	thinking := aggressive.Capabilities["thinking"].(map[string]interface{})
	if thinking["supported"] != true {
		t.Fatalf("激进策略能力错误: %#v", thinking)
	}
	conservative := mergeClaudeModelMetadata("claude-5", routes, "conservative")
	if conservative.MaxInputTokens != 100000 || conservative.MaxTokens != 8000 {
		t.Fatalf("保守策略上限错误: %#v", conservative)
	}
	thinking = conservative.Capabilities["thinking"].(map[string]interface{})
	if thinking["supported"] != false {
		t.Fatalf("保守策略能力错误: %#v", thinking)
	}
}

func TestPaginateClaudeModelIDs(t *testing.T) {
	ids := []string{"a", "b", "c", "d"}
	start, end, hasMore, err := paginateClaudeModelIDs(ids, 2, "", "")
	if err != nil || start != 0 || end != 2 || !hasMore {
		t.Fatalf("首页分页错误: start=%d end=%d more=%v err=%v", start, end, hasMore, err)
	}
	start, end, hasMore, err = paginateClaudeModelIDs(ids, 2, "", "b")
	if err != nil || start != 2 || end != 4 || hasMore {
		t.Fatalf("after 分页错误: start=%d end=%d more=%v err=%v", start, end, hasMore, err)
	}
	if _, _, _, err := paginateClaudeModelIDs(ids, 2, "missing", ""); err == nil {
		t.Fatal("无效游标应返回错误")
	}
}

func TestClaudeModelRoutingCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	service := &ClaudeModelRoutingService{
		cachePath: path,
		cache: map[string]claudeProviderModelCacheEntry{
			"1": {
				ProviderRef: "1",
				FetchedAt:   time.Now().UTC(),
				Response:    ProviderModelPricingResponse{Models: []ProviderModelPricingItem{{Model: "claude-5"}}},
			},
		},
	}
	if err := service.saveCache(); err != nil {
		t.Fatal(err)
	}
	loaded := &ClaudeModelRoutingService{cachePath: path, cache: map[string]claudeProviderModelCacheEntry{}}
	if err := loaded.loadCache(); err != nil {
		t.Fatal(err)
	}
	if got := loaded.cache["1"].Response.Models[0].Model; got != "claude-5" {
		t.Fatalf("缓存模型 = %q", got)
	}
}

func TestAppSettingsMigratesExistingClaudeModelConfiguration(t *testing.T) {
	home := useIsolatedHomeDir(t)
	providerPath := filepath.Join(home, ".code-switch", "claude-code.json")
	if err := os.MkdirAll(filepath.Dir(providerPath), 0o755); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(providerEnvelope{Providers: []Provider{{
		ID:           1,
		ModelMapping: map[string]string{"claude-5": "vendor/claude-5"},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(providerPath, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(home, ".code-switch", appSettingsFile)
	if err := os.WriteFile(settingsPath, []byte(`{"show_heatmap":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewAppSettingsService(nil)
	settings, err := service.GetAppSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !settings.ClaudeModelRoutingEnabled {
		t.Fatal("已有模型配置应自动开启 Claude 模型路由")
	}
	if settings.ClaudeModelAggregationEnabled {
		t.Fatal("迁移不应自动开启模型聚合")
	}
}

func TestAppSettingsMigratesRoutingWithoutExistingAppFile(t *testing.T) {
	home := useIsolatedHomeDir(t)
	providerPath := filepath.Join(home, ".code-switch", "claude-code.json")
	if err := os.MkdirAll(filepath.Dir(providerPath), 0o755); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(providerEnvelope{Providers: []Provider{{
		ID:              1,
		SupportedModels: map[string]bool{"claude-5": true},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(providerPath, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	settings, err := NewAppSettingsService(nil).GetAppSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !settings.ClaudeModelRoutingEnabled {
		t.Fatal("缺少 app.json 时也应迁移已有模型配置")
	}
}
