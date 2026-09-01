package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/daodao97/xgo/xdb"
)

func TestGeminiService_GetPresets(t *testing.T) {
	svc := NewGeminiService("127.0.0.1:18100")
	presets := svc.GetPresets()

	if len(presets) == 0 {
		t.Fatal("GetPresets should return at least one preset")
	}

	// Check Google Official preset
	var googlePreset *GeminiPreset
	for _, p := range presets {
		if p.Name == "Google Official" {
			googlePreset = &p
			break
		}
	}

	if googlePreset == nil {
		t.Fatal("Google Official preset should exist")
	}

	if googlePreset.Category != "official" {
		t.Errorf("Google Official category should be 'official', got '%s'", googlePreset.Category)
	}

	// Check PackyCode preset
	var packyPreset *GeminiPreset
	for _, p := range presets {
		if p.Name == "PackyCode" {
			packyPreset = &p
			break
		}
	}

	if packyPreset == nil {
		t.Fatal("PackyCode preset should exist")
	}

	if packyPreset.Category != "third_party" {
		t.Errorf("PackyCode category should be 'third_party', got '%s'", packyPreset.Category)
	}

	if packyPreset.BaseURL == "" {
		t.Error("PackyCode should have a BaseURL")
	}
}

func TestDetectGeminiAuthType(t *testing.T) {
	tests := []struct {
		name     string
		provider GeminiProvider
		expected GeminiAuthType
	}{
		{
			name: "Google Official OAuth (empty base and key)",
			provider: GeminiProvider{
				Name:    "Google Official",
				BaseURL: "",
				APIKey:  "",
			},
			expected: GeminiAuthOAuth,
		},
		{
			name: "PackyCode API Key",
			provider: GeminiProvider{
				Name:                "PackyCode",
				BaseURL:             "https://www.packyapi.com",
				APIKey:              "pk-xxx",
				PartnerPromotionKey: "packycode",
			},
			expected: GeminiAuthPackycode,
		},
		{
			name: "Generic API Key",
			provider: GeminiProvider{
				Name:    "Custom",
				BaseURL: "https://custom.api.com",
				APIKey:  "sk-xxx",
			},
			expected: GeminiAuthGeneric,
		},
		{
			name: "Generic provider with no base URL",
			provider: GeminiProvider{
				Name:    "Native Gemini",
				BaseURL: "",
				APIKey:  "AIza-xxx",
			},
			expected: GeminiAuthGeneric, // 无 partner_promotion_key 且名称不匹配 google/packy，默认为 Generic
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectGeminiAuthType(&tt.provider)
			if result != tt.expected {
				t.Errorf("detectGeminiAuthType() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestGeminiSaveProvidersKeepsSingleForcedPriority(t *testing.T) {
	resetProviderStoreForTest(t)
	svc := &GeminiService{
		providers: []GeminiProvider{
			{ID: "primary", Name: "Primary", Enabled: true, ForcedPriority: true},
			{ID: "secondary", Name: "Secondary", Enabled: true, ForcedPriority: true},
		},
		relayAddr: ":18100",
	}

	if err := svc.saveProviders(); err != nil {
		t.Fatal(err)
	}
	if !svc.providers[0].ForcedPriority || svc.providers[1].ForcedPriority {
		t.Fatalf("强制优先唯一性未保持: %#v", svc.providers)
	}
	stored, err := LoadGeminiProvidersFromStore()
	if err != nil || len(stored) != 2 || !stored[0].ForcedPriority || stored[1].ForcedPriority {
		t.Fatalf("强制优先持久化回读异常: %#v, %v", stored, err)
	}
}

func TestGeminiSetForcedPrioritySwitchesProviderAtomically(t *testing.T) {
	resetProviderStoreForTest(t)
	svc := &GeminiService{
		providers: []GeminiProvider{
			{ID: "primary", Name: "Primary", Enabled: true, ForcedPriority: true},
			{ID: "secondary", Name: "Secondary", Enabled: true},
		},
		relayAddr: ":18100",
	}

	if err := svc.SetForcedPriority("secondary", true); err != nil {
		t.Fatal(err)
	}
	if svc.providers[0].ForcedPriority || !svc.providers[1].ForcedPriority {
		t.Fatalf("强制优先切换异常: %#v", svc.providers)
	}
	stored, err := LoadGeminiProvidersFromStore()
	if err != nil || len(stored) != 2 || stored[0].ForcedPriority || !stored[1].ForcedPriority {
		t.Fatalf("强制优先原子持久化异常: %#v, %v", stored, err)
	}
}

func TestGeminiSetForcedPriorityCanCancel(t *testing.T) {
	resetProviderStoreForTest(t)
	svc := &GeminiService{
		providers: []GeminiProvider{
			{ID: "primary", Name: "Primary", Enabled: true, ForcedPriority: true},
			{ID: "secondary", Name: "Secondary", Enabled: true},
		},
		relayAddr: ":18100",
	}

	if err := svc.SetForcedPriority("primary", false); err != nil {
		t.Fatal(err)
	}
	for _, provider := range svc.providers {
		if provider.ForcedPriority {
			t.Fatalf("取消后仍存在强制优先供应商: %#v", svc.providers)
		}
	}
}

func TestGeminiSetForcedPriorityRejectsUnknownProviderWithoutChanges(t *testing.T) {
	resetProviderStoreForTest(t)
	svc := &GeminiService{
		providers: []GeminiProvider{
			{ID: "primary", Name: "Primary", Enabled: true, ForcedPriority: true},
			{ID: "secondary", Name: "Secondary", Enabled: true},
		},
		relayAddr: ":18100",
	}

	if err := svc.SetForcedPriority("missing", true); err == nil {
		t.Fatal("未知供应商应返回错误")
	}
	if !svc.providers[0].ForcedPriority || svc.providers[1].ForcedPriority {
		t.Fatalf("未知供应商不应改变现有状态: %#v", svc.providers)
	}
}

func TestParseEnvFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string]string
	}{
		{
			name:     "Empty file",
			content:  "",
			expected: map[string]string{},
		},
		{
			name:    "Single variable",
			content: "GEMINI_API_KEY=test-key",
			expected: map[string]string{
				"GEMINI_API_KEY": "test-key",
			},
		},
		{
			name: "Multiple variables",
			content: `GEMINI_API_KEY=test-key
GOOGLE_GEMINI_BASE_URL=https://api.test.com
GEMINI_MODEL=gemini-pro`,
			expected: map[string]string{
				"GEMINI_API_KEY":         "test-key",
				"GOOGLE_GEMINI_BASE_URL": "https://api.test.com",
				"GEMINI_MODEL":           "gemini-pro",
			},
		},
		{
			name: "With comments and empty lines",
			content: `# This is a comment
GEMINI_API_KEY=test-key

# Another comment
GOOGLE_GEMINI_BASE_URL=https://api.test.com
`,
			expected: map[string]string{
				"GEMINI_API_KEY":         "test-key",
				"GOOGLE_GEMINI_BASE_URL": "https://api.test.com",
			},
		},
		{
			name:    "Value with equals sign",
			content: "SOME_KEY=value=with=equals",
			expected: map[string]string{
				"SOME_KEY": "value=with=equals",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseEnvFile(tt.content)
			if len(result) != len(tt.expected) {
				t.Errorf("parseEnvFile() returned %d items, expected %d", len(result), len(tt.expected))
			}
			for key, expectedValue := range tt.expected {
				if result[key] != expectedValue {
					t.Errorf("parseEnvFile()[%s] = %q, expected %q", key, result[key], expectedValue)
				}
			}
		})
	}
}

func TestIsValidEnvKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"GEMINI_API_KEY", true},
		{"gemini_api_key", true},
		{"GOOGLE_GEMINI_BASE_URL", true},
		{"KEY123", true},
		{"_KEY", true},
		{"KEY-NAME", false}, // hyphen not allowed
		{"KEY.NAME", false}, // dot not allowed
		{"KEY NAME", false}, // space not allowed
		{"", true},          // empty is technically valid (no invalid chars)
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := isValidEnvKey(tt.key)
			if result != tt.expected {
				t.Errorf("isValidEnvKey(%q) = %v, expected %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestGeminiProvider_DeepCopyMaps(t *testing.T) {
	// Test that provider EnvConfig is properly deep copied when needed
	original := GeminiProvider{
		Name: "Test",
		EnvConfig: map[string]string{
			"KEY1": "value1",
		},
	}

	// Create a copy manually (simulating what should happen in duplication)
	copied := GeminiProvider{
		Name:      original.Name,
		EnvConfig: make(map[string]string),
	}
	for k, v := range original.EnvConfig {
		copied.EnvConfig[k] = v
	}

	// Modify copied
	copied.EnvConfig["KEY2"] = "value2"

	// Original should not be affected
	if _, exists := original.EnvConfig["KEY2"]; exists {
		t.Error("Original EnvConfig was modified when copy was changed")
	}

	if len(original.EnvConfig) != 1 {
		t.Errorf("Original EnvConfig length changed: got %d, expected 1", len(original.EnvConfig))
	}
}

func TestGeminiDuplicateProviderPreservesConcurrencyLimit(t *testing.T) {
	zero := 0
	tests := []struct {
		name  string
		limit *int
	}{
		{name: "未配置保持无限制"},
		{name: "显式零保持满载", limit: &zero},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useIsolatedHomeDir(t)
			svc := &GeminiService{
				providers: []GeminiProvider{{
					ID:                       "source",
					Name:                     "Source Provider",
					ProviderConcurrencyLimit: tt.limit,
				}},
				relayAddr: ":18100",
			}

			duplicated, err := svc.DuplicateProvider("source")
			if err != nil {
				t.Fatalf("复制 Gemini 供应商失败: %v", err)
			}
			if tt.limit == nil {
				if duplicated.ProviderConcurrencyLimit != nil {
					t.Fatalf("复制后的并发上限 = %d，期望 nil", *duplicated.ProviderConcurrencyLimit)
				}
				return
			}
			if duplicated.ProviderConcurrencyLimit == nil || *duplicated.ProviderConcurrencyLimit != *tt.limit {
				t.Fatalf("复制后的并发上限 = %v，期望 %d", duplicated.ProviderConcurrencyLimit, *tt.limit)
			}
		})
	}
}

func TestGeminiDuplicateProviderDoesNotInheritForcedPriority(t *testing.T) {
	useIsolatedHomeDir(t)
	svc := &GeminiService{
		providers: []GeminiProvider{{
			ID:             "source",
			Name:           "Source Provider",
			ForcedPriority: true,
		}},
		relayAddr: ":18100",
	}

	duplicated, err := svc.DuplicateProvider("source")
	if err != nil {
		t.Fatal(err)
	}
	if duplicated.ForcedPriority {
		t.Fatal("复制 Gemini 供应商不应继承强制优先状态")
	}
}

func TestGeminiPreset_Fields(t *testing.T) {
	svc := NewGeminiService("127.0.0.1:18100")
	presets := svc.GetPresets()

	for _, p := range presets {
		// All presets should have Name
		if p.Name == "" {
			t.Error("Preset has empty name")
		}

		// All presets except custom should have WebsiteURL
		if p.WebsiteURL == "" && p.Category != "custom" {
			t.Errorf("Preset %q has empty WebsiteURL", p.Name)
		}

		// All presets should have Category
		if p.Category == "" {
			t.Errorf("Preset %q has empty Category", p.Name)
		}

		// Category should be valid
		validCategories := map[string]bool{
			"official":    true,
			"third_party": true,
			"custom":      true,
		}
		if !validCategories[p.Category] {
			t.Errorf("Preset %q has invalid Category: %q", p.Name, p.Category)
		}
	}
}

func TestGeminiService_LoadProvidersSupportsTotalQuotaFieldAfterInitDatabase(t *testing.T) {
	useIsolatedHomeDir(t)
	resetProviderStoreForTest(t)
	// 数据库与队列已由 TestMain 初始化，此处不再重复 InitDatabase

	path := getGeminiProvidersPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建 Gemini providers 目录失败: %v", err)
	}

	payload, err := json.Marshal([]GeminiProvider{
		{
			ID:           "gemini-total",
			Name:         "Gemini Total",
			BaseURL:      "https://gemini.example.com",
			APIKey:       "gm-key",
			Enabled:      true,
			HideLogBadge: true,
			BudgetQuotaSettings: &BudgetQuotaSettings{
				Total: BudgetQuotaSetting{
					Total:           1024,
					RefreshTime:     "00:00",
					RefreshDay:      1,
					RefreshMonthDay: 1,
				},
			},
			BudgetQuotaUsedAdjustments: &BudgetQuotaAdjustments{
				Total: 45.67,
			},
			ProviderQuotaQueryType: string(ProviderQuotaQueryTypeTokenPlanMiniMax),
			ProviderQuotaQueryConfig: &ProviderQuotaQueryConfig{
				Enabled:           true,
				TemplateType:      string(ProviderQuotaTemplateTypeTokenPlan),
				TokenPlanProvider: "minimax",
				Timeout:           10,
				AutoQueryInterval: 5,
			},
		},
	})
	if err != nil {
		t.Fatalf("序列化 Gemini provider 配置失败: %v", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("写入 Gemini provider 配置失败: %v", err)
	}

	// 触发迁移器：把 fixture JSON 迁入统一存储（生产中由启动时 InitDatabase 内执行）
	migrateFixtureProvidersToStore(t)

	svc := NewGeminiService("127.0.0.1:18100")
	providers := svc.GetProviders()
	if len(providers) != 1 {
		t.Fatalf("期望读取到 1 个 Gemini provider，实际为 %d", len(providers))
	}
	if providers[0].BudgetQuotaSettings == nil || providers[0].BudgetQuotaSettings.Total.Total != 1024 {
		t.Fatalf("Gemini total quota 反序列化失败: %+v", providers[0].BudgetQuotaSettings)
	}
	if providers[0].BudgetQuotaUsedAdjustments == nil || providers[0].BudgetQuotaUsedAdjustments.Total != 45.67 {
		t.Fatalf("Gemini total quota adjustment 反序列化失败: %+v", providers[0].BudgetQuotaUsedAdjustments)
	}
	if providers[0].ProviderQuotaQueryType != string(ProviderQuotaQueryTypeTokenPlanMiniMax) {
		t.Fatalf("Gemini providerQuotaQueryType 反序列化失败: %q", providers[0].ProviderQuotaQueryType)
	}
	if providers[0].ProviderQuotaQueryConfig == nil || providers[0].ProviderQuotaQueryConfig.TokenPlanProvider != "minimax" {
		t.Fatalf("Gemini providerQuotaQueryConfig 反序列化失败: %+v", providers[0].ProviderQuotaQueryConfig)
	}
	if !providers[0].HideLogBadge {
		t.Fatal("Gemini hideLogBadge 反序列化失败")
	}
}

func TestCloneGeminiProvidersPreservesValuesAndIsolation(t *testing.T) {
	original := geminiProviderOwnershipFixture("gemini-owned")
	expected := geminiProviderOwnershipFixture("gemini-owned")
	cloned := cloneGeminiProviders([]GeminiProvider{original})

	if !reflect.DeepEqual(cloned, []GeminiProvider{expected}) {
		t.Fatalf("克隆结果与原值不一致:\nactual: %#v\nexpected: %#v", cloned, []GeminiProvider{expected})
	}
	mutateGeminiProviderOwnedFields(&cloned[0])
	if !reflect.DeepEqual(original, expected) {
		t.Fatalf("修改克隆后原值发生变化:\nactual: %#v\nexpected: %#v", original, expected)
	}

	if cloneGeminiProviders(nil) != nil {
		t.Fatal("nil Provider 切片应保持 nil")
	}
	empty := cloneGeminiProviders([]GeminiProvider{})
	if empty == nil || len(empty) != 0 {
		t.Fatalf("空 Provider 切片语义未保持: %#v", empty)
	}
	emptyFields := cloneGeminiProvider(GeminiProvider{
		EnvConfig:            map[string]string{},
		SettingsConfig:       map[string]any{},
		RequestBodyOverrides: map[string]any{},
	})
	if emptyFields.EnvConfig == nil || emptyFields.SettingsConfig == nil || emptyFields.RequestBodyOverrides == nil {
		t.Fatalf("空 map 不应被克隆为 nil: %#v", emptyFields)
	}
	nilFields := cloneGeminiProvider(GeminiProvider{})
	if nilFields.EnvConfig != nil || nilFields.SettingsConfig != nil || nilFields.RequestBodyOverrides != nil {
		t.Fatalf("nil map 不应被克隆为空 map: %#v", nilFields)
	}
}

func TestGeminiServiceGetProvidersReturnsOwnedCopy(t *testing.T) {
	expected := geminiProviderOwnershipFixture("gemini-read")
	svc := &GeminiService{providers: []GeminiProvider{geminiProviderOwnershipFixture("gemini-read")}}

	returned := svc.GetProviders()
	mutateGeminiProviderOwnedFields(&returned[0])

	actual := svc.GetProviders()
	if !reflect.DeepEqual(actual, []GeminiProvider{expected}) {
		t.Fatalf("修改 GetProviders 返回值影响了内部状态:\nactual: %#v\nexpected: %#v", actual, []GeminiProvider{expected})
	}
}

func TestGeminiServiceAddAndUpdateTakeProviderOwnership(t *testing.T) {
	t.Run("AddProvider", func(t *testing.T) {
		resetProviderStoreForTest(t)
		input := geminiProviderOwnershipFixture("gemini-add")
		expected := geminiProviderOwnershipFixture("gemini-add")
		svc := &GeminiService{relayAddr: ":18100"}

		if err := svc.AddProvider(input); err != nil {
			t.Fatal(err)
		}
		mutateGeminiProviderOwnedFields(&input)

		actual := svc.GetProviders()
		if !reflect.DeepEqual(actual, []GeminiProvider{expected}) {
			t.Fatalf("修改 AddProvider 入参影响了内部状态:\nactual: %#v\nexpected: %#v", actual, []GeminiProvider{expected})
		}
	})

	t.Run("UpdateProvider", func(t *testing.T) {
		resetProviderStoreForTest(t)
		svc := &GeminiService{
			providers: []GeminiProvider{geminiProviderOwnershipFixture("gemini-update")},
			relayAddr: ":18100",
		}
		input := geminiProviderOwnershipFixture("gemini-update")
		input.Description = "updated ownership fixture"
		expected := geminiProviderOwnershipFixture("gemini-update")
		expected.Description = "updated ownership fixture"

		if err := svc.UpdateProvider(input); err != nil {
			t.Fatal(err)
		}
		mutateGeminiProviderOwnedFields(&input)

		actual := svc.GetProviders()
		if !reflect.DeepEqual(actual, []GeminiProvider{expected}) {
			t.Fatalf("修改 UpdateProvider 入参影响了内部状态:\nactual: %#v\nexpected: %#v", actual, []GeminiProvider{expected})
		}
	})
}

func TestGeminiSetForcedPrioritySaveFailureRestoresOwnedSnapshot(t *testing.T) {
	resetProviderStoreForTest(t)
	primary := geminiProviderOwnershipFixture("gemini-primary")
	secondary := geminiProviderOwnershipFixture("gemini-secondary")
	secondary.ForcedPriority = false
	expected := []GeminiProvider{geminiProviderOwnershipFixture("gemini-primary"), geminiProviderOwnershipFixture("gemini-secondary")}
	expected[1].ForcedPriority = false
	svc := &GeminiService{providers: []GeminiProvider{primary, secondary}}

	previousQueue := GlobalDBQueue
	failedQueue := &DBWriteQueue{}
	failedQueue.closed.Store(true)
	GlobalDBQueue = failedQueue
	t.Cleanup(func() {
		GlobalDBQueue = previousQueue
	})

	if err := svc.SetForcedPriority("gemini-secondary", true); err == nil {
		t.Fatal("关闭的写入队列应使强制优先保存失败")
	}
	mutateGeminiProviderOwnedFields(&primary)
	mutateGeminiProviderOwnedFields(&secondary)

	actual := svc.GetProviders()
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("保存失败回退快照仍与旧引用共享:\nactual: %#v\nexpected: %#v", actual, expected)
	}
}

func TestGeminiDuplicateProviderReturnDoesNotAliasStoredProviders(t *testing.T) {
	resetProviderStoreForTest(t)
	svc := &GeminiService{
		providers: []GeminiProvider{geminiProviderOwnershipFixture("gemini-source")},
		relayAddr: ":18100",
	}

	duplicated, err := svc.DuplicateProvider("gemini-source")
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(svc.GetProviders())
	if err != nil {
		t.Fatal(err)
	}
	mutateGeminiDuplicatedOwnedFields(duplicated)
	actual, err := json.Marshal(svc.GetProviders())
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(before) {
		t.Fatalf("修改 DuplicateProvider 返回值影响了内部状态:\nactual: %s\nexpected: %s", actual, before)
	}
}

func TestGeminiServiceProviderCopiesAreRaceFree(t *testing.T) {
	resetProviderStoreForTest(t)
	svc := &GeminiService{providers: []GeminiProvider{geminiProviderOwnershipFixture("gemini-race")}}
	external := svc.GetProviders()
	start := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		<-start
		for i := 0; i < 10_000; i++ {
			*external[0].ProviderConcurrencyLimit = i
		}
	}()
	close(start)
	for i := 0; i < 100; i++ {
		updated := geminiProviderOwnershipFixture("gemini-race")
		updated.SortOrder = i
		if err := svc.UpdateProvider(updated); err != nil {
			t.Fatal(err)
		}
		_ = *svc.GetProviders()[0].ProviderConcurrencyLimit
	}
	<-done
}

func TestGeminiUpdateProviderRenameCommitsStoreAndIdentityWithoutWriterWait(t *testing.T) {
	resetProviderStoreForTest(t)
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM request_log WHERE platform = 'gemini'`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM request_log WHERE platform = 'gemini'`)
	})

	original := geminiProviderOwnershipFixture("gemini-rename-success")
	original.Name = "Gemini Old Name"
	if err := SaveGeminiProvidersToStore([]GeminiProvider{original}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO request_log (platform, provider_id, provider, data_source, created_at)
		VALUES ('gemini', ?, ?, 'proxy', '2026-09-01 00:00:00')
	`, original.ID, original.Name); err != nil {
		t.Fatal(err)
	}

	svc := &GeminiService{providers: []GeminiProvider{cloneGeminiProvider(original)}, relayAddr: ":18100"}
	updated := cloneGeminiProvider(original)
	updated.Name = "Gemini New Name"
	startedAt := time.Now()
	if err := svc.UpdateProvider(updated); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(startedAt); elapsed > 5*time.Second {
		t.Fatalf("Gemini 改名耗时过长: %s", elapsed)
	}

	stored, err := LoadGeminiProvidersFromStore()
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0].Name != updated.Name {
		t.Fatalf("Gemini 存储名称未提交: %#v", stored)
	}
	var logName string
	if err := db.QueryRow(`SELECT provider FROM request_log WHERE platform = 'gemini' AND provider_id = ?`, original.ID).Scan(&logName); err != nil {
		t.Fatal(err)
	}
	if logName != updated.Name {
		t.Fatalf("关联日志名称=%q，期望=%q", logName, updated.Name)
	}
}

func TestGeminiUpdateProviderRenameFailureRollsBackStoreIdentityAndMemory(t *testing.T) {
	resetProviderStoreForTest(t)
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS test_block_gemini_provider_rename`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM request_log WHERE platform = 'gemini'`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DROP TRIGGER IF EXISTS test_block_gemini_provider_rename`)
		_, _ = db.Exec(`DELETE FROM request_log WHERE platform = 'gemini'`)
	})

	original := geminiProviderOwnershipFixture("gemini-rename-rollback")
	original.Name = "Gemini Rollback Old"
	if err := SaveGeminiProvidersToStore([]GeminiProvider{original}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO request_log (platform, provider_id, provider, data_source, created_at)
		VALUES ('gemini', ?, ?, 'proxy', '2026-09-01 00:00:00')
	`, original.ID, original.Name); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TRIGGER test_block_gemini_provider_rename
		BEFORE UPDATE OF provider ON request_log
		WHEN OLD.platform = 'gemini' AND OLD.provider_id = 'gemini-rename-rollback'
		BEGIN
			SELECT RAISE(ABORT, 'forced gemini rename failure');
		END
	`); err != nil {
		t.Fatal(err)
	}

	svc := &GeminiService{providers: []GeminiProvider{cloneGeminiProvider(original)}, relayAddr: ":18100"}
	updated := cloneGeminiProvider(original)
	updated.Name = "Gemini Rollback New"
	if err := svc.UpdateProvider(updated); err == nil {
		t.Fatal("关联身份更新失败时 Gemini 改名应失败")
	}

	stored, err := LoadGeminiProvidersFromStore()
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0].Name != original.Name {
		t.Fatalf("失败后 Gemini 存储未回滚: %#v", stored)
	}
	var logName string
	if err := db.QueryRow(`SELECT provider FROM request_log WHERE platform = 'gemini' AND provider_id = ?`, original.ID).Scan(&logName); err != nil {
		t.Fatal(err)
	}
	if logName != original.Name {
		t.Fatalf("失败后关联日志名称=%q，期望=%q", logName, original.Name)
	}
	providers := svc.GetProviders()
	if len(providers) != 1 || providers[0].Name != original.Name {
		t.Fatalf("失败后内存 Provider 未回滚: %#v", providers)
	}
}

func geminiProviderOwnershipFixture(id string) GeminiProvider {
	limit := 3
	return GeminiProvider{
		ID:                       id,
		Name:                     "Owned Gemini",
		WebsiteURL:               "https://example.com",
		APIKeyURL:                "https://example.com/key",
		BaseURL:                  "https://api.example.com",
		APIKey:                   "test-key",
		Model:                    "gemini-test",
		Description:              "ownership fixture",
		Category:                 "custom",
		PartnerPromotionKey:      "ownership",
		Enabled:                  true,
		ForcedPriority:           true,
		HideLogBadge:             true,
		SortOrder:                7,
		EnabledSortOrder:         8,
		DisabledSortOrder:        9,
		Level:                    4,
		ProviderConcurrencyLimit: &limit,
		SessionMaxSessions:       5,
		SessionTTLMinutes:        6,
		EnvConfig: map[string]string{
			"GEMINI_API_KEY": "test-key",
		},
		SettingsConfig: map[string]any{
			"security": map[string]any{
				"auth": map[string]any{"selectedType": "gemini-api-key"},
			},
			"models": []any{map[string]any{"name": "gemini-test"}},
		},
		RequestBodyOverrides: map[string]any{
			"generationConfig": map[string]any{"temperature": 0.5},
			"stopSequences":    []any{"END"},
		},
		BudgetQuotaSettings: &BudgetQuotaSettings{
			Daily: BudgetQuotaSetting{Total: 10, RefreshTime: "00:00", RefreshDay: 1, RefreshMonthDay: 2},
		},
		BudgetQuotaUsedAdjustments: &BudgetQuotaAdjustments{Daily: 2.5},
		ProviderQuotaQueryType:     string(ProviderQuotaQueryTypeBalance),
		ProviderQuotaQueryConfig: &ProviderQuotaQueryConfig{
			Enabled:           true,
			TemplateType:      string(ProviderQuotaTemplateTypeBalance),
			Code:              "return result",
			Timeout:           12,
			APIKey:            "quota-key",
			BaseURL:           "https://quota.example.com",
			AccessToken:       "quota-token",
			UserID:            "quota-user",
			TokenPlanProvider: "quota-provider",
			AutoQueryInterval: 3,
			AutoIntervalMins:  4,
		},
		QuotaAutoDisabled:      true,
		QuotaAutoDisablePaused: true,
	}
}

func mutateGeminiProviderOwnedFields(provider *GeminiProvider) {
	provider.Name = "Changed Gemini"
	*provider.ProviderConcurrencyLimit = 99
	provider.EnvConfig["GEMINI_API_KEY"] = "changed-key"
	provider.SettingsConfig["security"].(map[string]any)["auth"].(map[string]any)["selectedType"] = "changed"
	provider.SettingsConfig["models"].([]any)[0].(map[string]any)["name"] = "changed-model"
	provider.RequestBodyOverrides["generationConfig"].(map[string]any)["temperature"] = 1.0
	provider.RequestBodyOverrides["stopSequences"].([]any)[0] = "CHANGED"
	provider.BudgetQuotaSettings.Daily.Total = 99
	provider.BudgetQuotaUsedAdjustments.Daily = 99
	provider.ProviderQuotaQueryConfig.BaseURL = "https://changed.example.com"
}

func mutateGeminiDuplicatedOwnedFields(provider *GeminiProvider) {
	*provider.ProviderConcurrencyLimit = 99
	provider.EnvConfig["GEMINI_API_KEY"] = "changed-key"
	provider.SettingsConfig["security"].(map[string]any)["auth"].(map[string]any)["selectedType"] = "changed"
	provider.SettingsConfig["models"].([]any)[0].(map[string]any)["name"] = "changed-model"
	provider.RequestBodyOverrides["generationConfig"].(map[string]any)["temperature"] = 1.0
	provider.RequestBodyOverrides["stopSequences"].([]any)[0] = "CHANGED"
}
