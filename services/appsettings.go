package services

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	appSettingsDir                        = ".code-switch" // 【修复】修正拼写错误（原为 .codex-swtich）
	appSettingsFile                       = "app.json"
	oldSettingsDir                        = ".codex-swtich"               // 旧的错误拼写
	migrationMarkerFile                   = ".migrated-from-codex-swtich" // 迁移标记文件
	defaultUpdateHistoryKeepCount         = 3
	minUpdateHistoryKeepCount             = 1
	maxUpdateHistoryKeepCount             = 20
	DefaultMainWindowDestroyDelaySeconds  = 30
	minMainWindowDestroyDelaySeconds      = 0
	maxMainWindowDestroyDelaySeconds      = 300
	heatmapGranularityHourly              = "hourly"
	heatmapGranularityDaily               = "daily"
	heatmapDailyModeHourlyScaled          = "hourly_scaled"
	heatmapDailyModeDailyPeak             = "daily_peak"
	heatmapIntensityMetricRequests        = "requests"
	heatmapIntensityMetricCost            = "cost"
	heatmapIntensityMetricTotalTokens     = "total_tokens"
	heatmapIntensityMetricInputTokens     = "input_tokens"
	heatmapIntensityMetricOutputTokens    = "output_tokens"
	heatmapIntensityMetricReasoningTokens = "reasoning_tokens"
	defaultHeatmapDailyScale              = 24
	defaultHeatmapIntensityMetric         = heatmapIntensityMetricRequests
	minHeatmapDailyScale                  = 1
	maxHeatmapDailyScale                  = 72
	defaultHeatmapIntensityL1             = 25
	defaultHeatmapIntensityL2             = 50
	defaultHeatmapIntensityL3             = 75
	defaultLogsRefreshIntervalSeconds     = 30
	defaultQuotaRecoveryIntervalSeconds   = 60
	minQuotaRecoveryIntervalSeconds       = 10
	maxQuotaRecoveryIntervalSeconds       = 3600
	claudeProxyAuthFieldAuthToken         = "auth_token"
	claudeProxyAuthFieldAPIKey            = "api_key"
	minHeatmapIntensityStop               = 1
	maxHeatmapIntensityStop               = 99
	budgetCycleModeDaily                  = "daily"
	budgetCycleModeWeekly                 = "weekly"
	budgetCycleModeMonthly                = "monthly"
	defaultBudgetRefreshWeekday           = 1
	minBudgetRefreshWeekday               = 0
	maxBudgetRefreshWeekday               = 6
	defaultBudgetRefreshMonthDay          = 1
	minBudgetRefreshMonthDay              = 1
	maxBudgetRefreshMonthDay              = 31
)

type AppSettings struct {
	ShowHeatmap                      bool                                     `json:"show_heatmap"`
	HeatmapGranularity               string                                   `json:"heatmap_granularity"`
	HeatmapDailyScaleFactor          int                                      `json:"heatmap_daily_scale_factor"`
	HeatmapDailyIntensityMode        string                                   `json:"heatmap_daily_intensity_mode"`
	HeatmapIntensityMetric           string                                   `json:"heatmap_intensity_metric"`
	HeatmapIntensityStopL1           int                                      `json:"heatmap_intensity_stop_l1"`
	HeatmapIntensityStopL2           int                                      `json:"heatmap_intensity_stop_l2"`
	HeatmapIntensityStopL3           int                                      `json:"heatmap_intensity_stop_l3"`
	ShowHomeTitle                    bool                                     `json:"show_home_title"`
	HomeProviderTabs                 []string                                 `json:"home_provider_tabs"`
	BudgetTotal                      float64                                  `json:"budget_total"`
	BudgetUsedAdjustment             float64                                  `json:"budget_used_adjustment"`
	BudgetCycleEnabled               bool                                     `json:"budget_cycle_enabled"`
	BudgetCycleMode                  string                                   `json:"budget_cycle_mode"`
	BudgetRefreshTime                string                                   `json:"budget_refresh_time"`
	BudgetRefreshDay                 int                                      `json:"budget_refresh_day"`
	BudgetRefreshMonthDay            int                                      `json:"budget_refresh_month_day"`
	BudgetQuotaUsedAdjustments       BudgetQuotaAdjustments                   `json:"budget_quota_used_adjustments"`
	BudgetQuotaSettings              BudgetQuotaSettings                      `json:"budget_quota_settings"`
	BudgetShowCountdown              bool                                     `json:"budget_show_countdown"`
	BudgetShowForecast               bool                                     `json:"budget_show_forecast"`
	BudgetForecastMethod             string                                   `json:"budget_forecast_method"`
	BudgetForecastDisplay            string                                   `json:"budget_forecast_display"`
	BudgetTotalCodex                 float64                                  `json:"budget_total_codex"`
	BudgetUsedAdjustmentCodex        float64                                  `json:"budget_used_adjustment_codex"`
	BudgetCycleEnabledCodex          bool                                     `json:"budget_cycle_enabled_codex"`
	BudgetCycleModeCodex             string                                   `json:"budget_cycle_mode_codex"`
	BudgetRefreshTimeCodex           string                                   `json:"budget_refresh_time_codex"`
	BudgetRefreshDayCodex            int                                      `json:"budget_refresh_day_codex"`
	BudgetRefreshMonthDayCodex       int                                      `json:"budget_refresh_month_day_codex"`
	BudgetQuotaUsedAdjustmentsCodex  BudgetQuotaAdjustments                   `json:"budget_quota_used_adjustments_codex"`
	BudgetQuotaSettingsCodex         BudgetQuotaSettings                      `json:"budget_quota_settings_codex"`
	BudgetShowCountdownCodex         bool                                     `json:"budget_show_countdown_codex"`
	BudgetShowForecastCodex          bool                                     `json:"budget_show_forecast_codex"`
	BudgetForecastMethodCodex        string                                   `json:"budget_forecast_method_codex"`
	BudgetForecastDisplayCodex       string                                   `json:"budget_forecast_display_codex"`
	AutoStart                        bool                                     `json:"auto_start"`
	AutoUpdate                       bool                                     `json:"auto_update"`
	UpdateHistoryKeepCount           int                                      `json:"update_history_keep_count"` // 更新包历史保留数量
	LogsRefreshIntervalSeconds       int                                      `json:"logs_refresh_interval_seconds"`
	MainWindowDestroyDelaySeconds    int                                      `json:"main_window_destroy_delay_seconds"`
	AutoConnectivityTest             bool                                     `json:"auto_connectivity_test"`
	EnableSwitchNotify               bool                                     `json:"enable_switch_notify"` // 供应商切换通知开关
	EnableRoundRobin                 bool                                     `json:"enable_round_robin"`   // 同 Level 轮询负载均衡开关（默认关闭）
	ClaudeModelRoutingEnabled        bool                                     `json:"claude_model_routing_enabled"`
	ClaudeModelAggregationEnabled    bool                                     `json:"claude_model_aggregation_enabled"`
	ClaudeModelMetadataMergeStrategy string                                   `json:"claude_model_metadata_merge_strategy"`
	ClaudeProxyAuthField             string                                   `json:"claude_proxy_auth_field"`
	PreserveCodexOfficialAuth        bool                                     `json:"preserve_codex_official_auth_on_switch"`
	UnifyCodexSessionHistory         bool                                     `json:"unify_codex_session_history"`
	UnifyCodexMigrateExisting        bool                                     `json:"unify_codex_migrate_existing"`
	ProviderConcurrencyLimits        map[string]bool                          `json:"provider_concurrency_limits"`
	ProviderQuotaQueryPresetCodes    map[string]string                        `json:"provider_quota_query_preset_codes,omitempty"`
	ProviderQuotaQueryPresets        map[string]ProviderQuotaQueryPresetGroup `json:"provider_quota_query_presets,omitempty"`
	ProviderQuotaAutoDisableEnabled  bool                                     `json:"provider_quota_auto_disable_enabled"`
	QuotaRecoveryIntervalSeconds     int                                      `json:"provider_quota_recovery_interval_seconds"`
	QuotaRecoveryNotifyEnabled       bool                                     `json:"provider_quota_recovery_notify_enabled"`
	CaptureRequestLogPayload         bool                                     `json:"capture_request_log_payload"`
	SanitizeRequestLogPayload        bool                                     `json:"sanitize_request_log_payload"`
}

type ProviderQuotaQueryPresetEntry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
}

type ProviderQuotaQueryPresetGroup struct {
	DefaultID string                          `json:"defaultId,omitempty"`
	Items     []ProviderQuotaQueryPresetEntry `json:"items"`
}

type BudgetQuotaSetting struct {
	Total           float64 `json:"total"`
	RefreshTime     string  `json:"refresh_time"`
	RefreshDay      int     `json:"refresh_day"`
	RefreshMonthDay int     `json:"refresh_month_day"`
}

func (s *BudgetQuotaSetting) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*s = BudgetQuotaSetting{}
		return nil
	}
	type rawBudgetQuotaSetting struct {
		Total                *float64 `json:"total"`
		RefreshTime          *string  `json:"refresh_time"`
		RefreshTimeCamel     *string  `json:"refreshTime"`
		RefreshDay           *int     `json:"refresh_day"`
		RefreshDayCamel      *int     `json:"refreshDay"`
		RefreshWeekdayCamel  *int     `json:"refreshWeekday"`
		RefreshMonthDay      *int     `json:"refresh_month_day"`
		RefreshMonthDayCamel *int     `json:"refreshMonthDay"`
	}
	var raw rawBudgetQuotaSetting
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var normalized BudgetQuotaSetting
	if raw.Total != nil {
		normalized.Total = *raw.Total
	}
	if raw.RefreshTime != nil {
		normalized.RefreshTime = *raw.RefreshTime
	} else if raw.RefreshTimeCamel != nil {
		normalized.RefreshTime = *raw.RefreshTimeCamel
	}
	if raw.RefreshDay != nil {
		normalized.RefreshDay = *raw.RefreshDay
	} else if raw.RefreshWeekdayCamel != nil {
		normalized.RefreshDay = *raw.RefreshWeekdayCamel
	} else if raw.RefreshDayCamel != nil {
		normalized.RefreshDay = *raw.RefreshDayCamel
	}
	if raw.RefreshMonthDay != nil {
		normalized.RefreshMonthDay = *raw.RefreshMonthDay
	} else if raw.RefreshMonthDayCamel != nil {
		normalized.RefreshMonthDay = *raw.RefreshMonthDayCamel
	}
	*s = normalized
	return nil
}

type BudgetQuotaSettings struct {
	FiveHour BudgetQuotaSetting `json:"five_hour"`
	Daily    BudgetQuotaSetting `json:"daily"`
	Weekly   BudgetQuotaSetting `json:"weekly"`
	Monthly  BudgetQuotaSetting `json:"monthly"`
	Total    BudgetQuotaSetting `json:"total"`
}

type BudgetQuotaAdjustments struct {
	FiveHour float64 `json:"five_hour"`
	Daily    float64 `json:"daily"`
	Weekly   float64 `json:"weekly"`
	Monthly  float64 `json:"monthly"`
	Total    float64 `json:"total"`
}

type AppSettingsService struct {
	path                           string
	mu                             sync.Mutex
	autoStartService               *AutoStartService
	codexSettings                  *CodexSettingsService
	claudeSettings                 *ClaudeSettingsService
	claudeModelRouting             *ClaudeModelRoutingService
	providerQuotaAutomation        *ProviderQuotaAutomationService
	snapshot                       atomic.Value
	fingerprintMu                  sync.Mutex
	fingerprint                    [sha256.Size]byte
	fingerprintExists              bool
	snapshotStop                   chan struct{}
	snapshotDone                   chan struct{}
	snapshotLifecycleMu            sync.Mutex
	snapshotStarted                bool
	snapshotStopped                bool
	mainWindowDestroyDelayRevision int64
}

func cloneAppSettings(settings AppSettings) AppSettings {
	cloned := settings
	cloned.HomeProviderTabs = append([]string(nil), settings.HomeProviderTabs...)
	if settings.ProviderConcurrencyLimits != nil {
		cloned.ProviderConcurrencyLimits = make(map[string]bool, len(settings.ProviderConcurrencyLimits))
		for key, value := range settings.ProviderConcurrencyLimits {
			cloned.ProviderConcurrencyLimits[key] = value
		}
	}
	if settings.ProviderQuotaQueryPresetCodes != nil {
		cloned.ProviderQuotaQueryPresetCodes = make(map[string]string, len(settings.ProviderQuotaQueryPresetCodes))
		for key, value := range settings.ProviderQuotaQueryPresetCodes {
			cloned.ProviderQuotaQueryPresetCodes[key] = value
		}
	}
	if settings.ProviderQuotaQueryPresets != nil {
		cloned.ProviderQuotaQueryPresets = make(map[string]ProviderQuotaQueryPresetGroup, len(settings.ProviderQuotaQueryPresets))
		for key, group := range settings.ProviderQuotaQueryPresets {
			group.Items = append([]ProviderQuotaQueryPresetEntry(nil), group.Items...)
			cloned.ProviderQuotaQueryPresets[key] = group
		}
	}
	return cloned
}

func NewAppSettingsService(autoStartService *AutoStartService) *AppSettingsService {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	newDir := filepath.Join(home, appSettingsDir)
	newPath := filepath.Join(newDir, appSettingsFile)
	oldDir := filepath.Join(home, oldSettingsDir)
	oldPath := filepath.Join(oldDir, appSettingsFile)
	markerPath := filepath.Join(newDir, migrationMarkerFile)

	// 检查是否已经迁移过
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		// 尚未迁移，检查旧目录
		if _, err := os.Stat(oldPath); err == nil {
			// 旧文件存在，执行迁移
			if err := migrateSettings(oldPath, newPath, oldDir, markerPath); err != nil {
				fmt.Printf("[AppSettings] ⚠️  迁移配置失败: %v\n", err)
			}
		}
	}

	service := &AppSettingsService{
		path:             newPath,
		autoStartService: autoStartService,
		snapshotStop:     make(chan struct{}),
		snapshotDone:     make(chan struct{}),
	}
	service.mu.Lock()
	settings, loadErr := service.loadLocked()
	service.mu.Unlock()
	if loadErr != nil {
		fmt.Printf("[AppSettings] 加载设置快照失败，使用默认值: %v\n", loadErr)
		settings = service.defaultSettings()
	}
	service.snapshot.Store(cloneAppSettings(settings))
	service.refreshFingerprint()
	return service
}

func (as *AppSettingsService) Start() error {
	if as == nil {
		return nil
	}
	as.snapshotLifecycleMu.Lock()
	defer as.snapshotLifecycleMu.Unlock()
	if as.snapshotStarted || as.snapshotStopped {
		return nil
	}
	as.snapshotStarted = true
	go as.watchSnapshot()
	return nil
}

func (as *AppSettingsService) Stop() error {
	if as == nil || as.snapshotStop == nil {
		return nil
	}
	as.snapshotLifecycleMu.Lock()
	started := as.snapshotStarted
	if !as.snapshotStopped {
		as.snapshotStopped = true
		if started {
			close(as.snapshotStop)
		}
	}
	as.snapshotLifecycleMu.Unlock()
	if started {
		<-as.snapshotDone
	}
	return nil
}

func (as *AppSettingsService) refreshFingerprint() {
	data, err := os.ReadFile(as.path)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return
	}
	fingerprint := sha256.Sum256(data)
	as.fingerprintMu.Lock()
	as.fingerprint = fingerprint
	as.fingerprintExists = exists
	as.fingerprintMu.Unlock()
}

func (as *AppSettingsService) watchSnapshot() {
	defer close(as.snapshotDone)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			as.reloadSnapshotIfChanged()
		case <-as.snapshotStop:
			return
		}
	}
}

func (as *AppSettingsService) reloadSnapshotIfChanged() {
	data, err := os.ReadFile(as.path)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return
	}
	fingerprint := sha256.Sum256(data)
	as.fingerprintMu.Lock()
	unchanged := as.fingerprintExists == exists && as.fingerprint == fingerprint
	as.fingerprintMu.Unlock()
	if unchanged {
		return
	}
	previous, _ := as.GetAppSettings()

	as.mu.Lock()
	settings, loadErr := as.loadLocked()
	routing := as.claudeModelRouting
	quotaAutomation := as.providerQuotaAutomation
	as.mu.Unlock()
	if loadErr != nil {
		fmt.Printf("[AppSettings] 外部设置解析失败，继续使用上一份快照: %v\n", loadErr)
		return
	}
	as.snapshot.Store(cloneAppSettings(settings))
	as.refreshFingerprint()
	if routing != nil && claudeModelRoutingSettingsChanged(previous, settings) {
		routing.HandleSettingsChanged(previous, settings)
	}
	if quotaAutomation != nil {
		quotaAutomation.HandleSettingsChanged(previous, settings)
	}
}

func (as *AppSettingsService) BindCodexSettingsService(codexSettings *CodexSettingsService) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.codexSettings = codexSettings
}

func (as *AppSettingsService) BindClaudeSettingsService(claudeSettings *ClaudeSettingsService) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.claudeSettings = claudeSettings
}

func (as *AppSettingsService) BindClaudeModelRoutingService(routing *ClaudeModelRoutingService) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.claudeModelRouting = routing
}

func (as *AppSettingsService) BindProviderQuotaAutomationService(service *ProviderQuotaAutomationService) {
	as.mu.Lock()
	as.providerQuotaAutomation = service
	settings, _ := as.GetAppSettings()
	as.mu.Unlock()

	if service != nil && !settings.ProviderQuotaAutoDisableEnabled {
		service.restoreAllProviders()
	}
}

// migrateSettings 完整的配置迁移
// 迁移顺序：写新文件 → 校验 → 标记 → 删旧
func migrateSettings(oldPath, newPath, oldDir, markerPath string) error {
	// 1. 确保新目录存在
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		return fmt.Errorf("创建新目录失败: %w", err)
	}

	// 2. 检查新文件是否已存在
	if _, err := os.Stat(newPath); err == nil {
		// 新文件已存在，不覆盖，但仍创建迁移标记
		fmt.Printf("[AppSettings] 新配置文件已存在，跳过迁移\n")
	} else {
		// 3. 读取旧配置
		data, err := os.ReadFile(oldPath)
		if err != nil {
			return fmt.Errorf("读取旧配置失败: %w", err)
		}

		// 4. 写入新配置
		if err := os.WriteFile(newPath, data, 0644); err != nil {
			return fmt.Errorf("写入新配置失败: %w", err)
		}

		// 5. 校验新文件
		verifyData, err := os.ReadFile(newPath)
		if err != nil {
			// 写入成功但读取失败，回滚
			os.Remove(newPath)
			return fmt.Errorf("校验新配置失败（已回滚）: %w", err)
		}

		// 校验内容一致性
		if !bytes.Equal(data, verifyData) {
			os.Remove(newPath)
			return fmt.Errorf("配置内容校验失败（已回滚）: 写入内容与读取内容不一致")
		}

		// 如果是 JSON 文件，额外校验 JSON 格式有效性
		var jsonTest interface{}
		if err := json.Unmarshal(verifyData, &jsonTest); err != nil {
			os.Remove(newPath)
			return fmt.Errorf("JSON 格式校验失败（已回滚）: %w", err)
		}

		fmt.Printf("[AppSettings] ✅ 已迁移并校验配置: %s → %s\n", oldPath, newPath)
	}

	// 6. 创建迁移标记文件
	markerContent := fmt.Sprintf("迁移时间: %s\n旧路径: %s\n", time.Now().Format(time.RFC3339), oldDir)
	if err := os.WriteFile(markerPath, []byte(markerContent), 0644); err != nil {
		return fmt.Errorf("创建迁移标记失败: %w", err)
	}

	// 7. 只有在新文件校验通过后才删除旧目录
	if err := os.RemoveAll(oldDir); err != nil {
		// 删除失败不是致命错误，只记录警告
		fmt.Printf("[AppSettings] ⚠️  删除旧目录失败: %v（可手动删除 %s）\n", err, oldDir)
	} else {
		fmt.Printf("[AppSettings] ✅ 已删除旧目录: %s\n", oldDir)
	}

	return nil
}

func (as *AppSettingsService) defaultSettings() AppSettings {
	// 检查当前开机自启动状态
	autoStartEnabled := false
	if as.autoStartService != nil {
		if enabled, err := as.autoStartService.IsEnabled(); err == nil {
			autoStartEnabled = enabled
		}
	}

	return AppSettings{
		ShowHeatmap:                      true,
		HeatmapGranularity:               heatmapGranularityDaily,
		HeatmapDailyScaleFactor:          defaultHeatmapDailyScale,
		HeatmapDailyIntensityMode:        heatmapDailyModeHourlyScaled,
		HeatmapIntensityMetric:           defaultHeatmapIntensityMetric,
		HeatmapIntensityStopL1:           defaultHeatmapIntensityL1,
		HeatmapIntensityStopL2:           defaultHeatmapIntensityL2,
		HeatmapIntensityStopL3:           defaultHeatmapIntensityL3,
		ShowHomeTitle:                    true,
		HomeProviderTabs:                 defaultHomeProviderTabs(),
		BudgetTotal:                      0,
		BudgetUsedAdjustment:             0,
		BudgetCycleEnabled:               false,
		BudgetCycleMode:                  budgetCycleModeDaily,
		BudgetRefreshTime:                "00:00",
		BudgetRefreshDay:                 defaultBudgetRefreshWeekday,
		BudgetRefreshMonthDay:            defaultBudgetRefreshMonthDay,
		BudgetQuotaUsedAdjustments:       defaultBudgetQuotaAdjustments(),
		BudgetQuotaSettings:              defaultBudgetQuotaSettings(),
		BudgetShowCountdown:              false,
		BudgetShowForecast:               false,
		BudgetForecastMethod:             "cycle",
		BudgetForecastDisplay:            "datetime",
		BudgetTotalCodex:                 0,
		BudgetUsedAdjustmentCodex:        0,
		BudgetCycleEnabledCodex:          false,
		BudgetCycleModeCodex:             budgetCycleModeDaily,
		BudgetRefreshTimeCodex:           "00:00",
		BudgetRefreshDayCodex:            defaultBudgetRefreshWeekday,
		BudgetRefreshMonthDayCodex:       defaultBudgetRefreshMonthDay,
		BudgetQuotaUsedAdjustmentsCodex:  defaultBudgetQuotaAdjustments(),
		BudgetQuotaSettingsCodex:         defaultBudgetQuotaSettings(),
		BudgetShowCountdownCodex:         false,
		BudgetShowForecastCodex:          false,
		BudgetForecastMethodCodex:        "cycle",
		BudgetForecastDisplayCodex:       "datetime",
		AutoStart:                        autoStartEnabled,
		AutoUpdate:                       true, // 默认开启自动更新
		UpdateHistoryKeepCount:           defaultUpdateHistoryKeepCount,
		LogsRefreshIntervalSeconds:       defaultLogsRefreshIntervalSeconds,
		MainWindowDestroyDelaySeconds:    DefaultMainWindowDestroyDelaySeconds,
		AutoConnectivityTest:             true,  // 默认开启自动可用性监控（开箱即用）
		EnableSwitchNotify:               true,  // 默认开启切换通知
		EnableRoundRobin:                 false, // 默认关闭轮询（使用顺序降级）
		ClaudeModelRoutingEnabled:        false,
		ClaudeModelAggregationEnabled:    false,
		ClaudeModelMetadataMergeStrategy: "aggressive",
		ClaudeProxyAuthField:             claudeProxyAuthFieldAuthToken,
		PreserveCodexOfficialAuth:        false,
		UnifyCodexSessionHistory:         false,
		UnifyCodexMigrateExisting:        false,
		ProviderConcurrencyLimits:        map[string]bool{},
		ProviderQuotaQueryPresetCodes:    map[string]string{},
		ProviderQuotaQueryPresets:        map[string]ProviderQuotaQueryPresetGroup{},
		QuotaRecoveryIntervalSeconds:     defaultQuotaRecoveryIntervalSeconds,
		QuotaRecoveryNotifyEnabled:       false,
		CaptureRequestLogPayload:         false, // 默认关闭 payload 采集，降低隐私与存储风险
		SanitizeRequestLogPayload:        true,  // 默认开启 payload 脱敏，避免敏感信息明文落库
	}
}

func defaultHomeProviderTabs() []string {
	return []string{"claude", "codex", "gemini"}
}

func normalizeHomeProviderTabs(tabs []string) []string {
	if len(tabs) == 0 {
		return defaultHomeProviderTabs()
	}

	allowedTabs := map[string]bool{
		"claude":   true,
		"codex":    true,
		"gemini":   true,
		"opencode": true,
		"others":   true,
	}
	seenTabs := make(map[string]bool, len(tabs))
	normalizedTabs := make([]string, 0, len(tabs))

	for _, tab := range tabs {
		tab = strings.TrimSpace(tab)
		if !allowedTabs[tab] || seenTabs[tab] {
			continue
		}
		seenTabs[tab] = true
		normalizedTabs = append(normalizedTabs, tab)
	}

	if len(normalizedTabs) == 0 {
		return defaultHomeProviderTabs()
	}
	return normalizedTabs
}

func normalizeProviderConcurrencyLimits(settings *AppSettings) {
	if settings.ProviderConcurrencyLimits == nil {
		settings.ProviderConcurrencyLimits = map[string]bool{}
		return
	}
	normalized := make(map[string]bool, len(settings.ProviderConcurrencyLimits))
	for key, enabled := range settings.ProviderConcurrencyLimits {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		normalized[key] = enabled
	}
	settings.ProviderConcurrencyLimits = normalized
}

func normalizeClaudeModelRoutingSettings(settings *AppSettings) {
	if settings == nil {
		return
	}
	if !settings.ClaudeModelRoutingEnabled {
		settings.ClaudeModelAggregationEnabled = false
	}
	switch strings.ToLower(strings.TrimSpace(settings.ClaudeModelMetadataMergeStrategy)) {
	case "conservative":
		settings.ClaudeModelMetadataMergeStrategy = "conservative"
	default:
		settings.ClaudeModelMetadataMergeStrategy = "aggressive"
	}
}

func normalizeClaudeProxyAuthField(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case claudeProxyAuthFieldAPIKey:
		return claudeProxyAuthFieldAPIKey
	default:
		return claudeProxyAuthFieldAuthToken
	}
}

func normalizeLogsRefreshIntervalSeconds(seconds int) int {
	switch seconds {
	case 0, 5, 10, 30, 60:
		return seconds
	default:
		return defaultLogsRefreshIntervalSeconds
	}
}

func normalizeProviderQuotaRecoveryIntervalSeconds(seconds int) int {
	if seconds == 0 {
		return defaultQuotaRecoveryIntervalSeconds
	}
	if seconds < minQuotaRecoveryIntervalSeconds {
		return minQuotaRecoveryIntervalSeconds
	}
	if seconds > maxQuotaRecoveryIntervalSeconds {
		return maxQuotaRecoveryIntervalSeconds
	}
	return seconds
}

func hasConfiguredClaudeModelRouting() bool {
	providers, err := LoadProvidersFromStore("claude")
	if err != nil {
		return false
	}
	for _, provider := range providers {
		if len(provider.SupportedModels) > 0 || len(provider.ModelMapping) > 0 {
			return true
		}
	}
	return false
}

func allowedProviderQuotaQueryPresetTypes() map[string]bool {
	return map[string]bool{
		"custom":  true,
		"general": true,
		"newapi":  true,
	}
}

func normalizeProviderQuotaQueryPresetCodesValue(value map[string]string) map[string]string {
	allowedTypes := allowedProviderQuotaQueryPresetTypes()
	normalized := map[string]string{}
	for key, code := range value {
		normalizedKey := strings.TrimSpace(strings.ToLower(key))
		normalizedCode := strings.TrimSpace(code)
		if !allowedTypes[normalizedKey] || normalizedCode == "" {
			continue
		}
		normalized[normalizedKey] = normalizedCode
	}
	return normalized
}

func normalizeProviderQuotaQueryPresetID(value string, fallback string) string {
	normalized := strings.TrimSpace(value)
	if normalized != "" {
		return normalized
	}
	return fallback
}

func normalizeProviderQuotaQueryPresetName(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized != "" {
		return normalized
	}
	return "自定义预设"
}

func normalizeProviderQuotaQueryPresets(settings *AppSettings) {
	legacyCodes := normalizeProviderQuotaQueryPresetCodesValue(settings.ProviderQuotaQueryPresetCodes)
	settings.ProviderQuotaQueryPresetCodes = legacyCodes

	allowedTypes := allowedProviderQuotaQueryPresetTypes()
	normalized := map[string]ProviderQuotaQueryPresetGroup{}
	for key, group := range settings.ProviderQuotaQueryPresets {
		normalizedKey := strings.TrimSpace(strings.ToLower(key))
		if !allowedTypes[normalizedKey] {
			continue
		}

		seenIDs := map[string]bool{}
		items := make([]ProviderQuotaQueryPresetEntry, 0, len(group.Items))
		for index, item := range group.Items {
			code := strings.TrimSpace(item.Code)
			if code == "" {
				continue
			}
			id := normalizeProviderQuotaQueryPresetID(item.ID, fmt.Sprintf("%s-%d", normalizedKey, index+1))
			if seenIDs[id] {
				continue
			}
			seenIDs[id] = true
			items = append(items, ProviderQuotaQueryPresetEntry{
				ID:        id,
				Name:      normalizeProviderQuotaQueryPresetName(item.Name),
				Code:      code,
				UpdatedAt: item.UpdatedAt,
			})
		}

		defaultID := strings.TrimSpace(group.DefaultID)
		if defaultID != "" && !seenIDs[defaultID] {
			defaultID = ""
		}
		if len(items) > 0 {
			if defaultID == "" {
				defaultID = items[0].ID
			}
			normalized[normalizedKey] = ProviderQuotaQueryPresetGroup{
				DefaultID: defaultID,
				Items:     items,
			}
		}
	}

	for key, code := range legacyCodes {
		group, exists := normalized[key]
		if exists && len(group.Items) > 0 {
			continue
		}
		id := "legacy-" + key
		normalized[key] = ProviderQuotaQueryPresetGroup{
			DefaultID: id,
			Items: []ProviderQuotaQueryPresetEntry{{
				ID:   id,
				Name: "自定义预设",
				Code: code,
			}},
		}
	}

	settings.ProviderQuotaQueryPresets = normalized
}

// GetAppSettings returns the persisted app settings or defaults if the file does not exist.
func (as *AppSettingsService) GetAppSettings() (AppSettings, error) {
	if value := as.snapshot.Load(); value != nil {
		return cloneAppSettings(value.(AppSettings)), nil
	}
	return as.defaultSettings(), nil
}

// SaveAppSettings persists the provided settings to disk.
func (as *AppSettingsService) SaveAppSettings(settings AppSettings) (AppSettings, error) {
	as.mu.Lock()
	defer as.mu.Unlock()
	previous, _ := as.GetAppSettings()
	settings.HeatmapGranularity = normalizeHeatmapGranularity(settings.HeatmapGranularity)
	settings.HomeProviderTabs = normalizeHomeProviderTabs(settings.HomeProviderTabs)
	normalizeHeatmapDisplaySettings(&settings)
	normalizeBudgetSettings(&settings)
	settings.UpdateHistoryKeepCount = normalizeUpdateHistoryKeepCount(settings.UpdateHistoryKeepCount)
	settings.LogsRefreshIntervalSeconds = normalizeLogsRefreshIntervalSeconds(settings.LogsRefreshIntervalSeconds)
	settings.QuotaRecoveryIntervalSeconds = normalizeProviderQuotaRecoveryIntervalSeconds(settings.QuotaRecoveryIntervalSeconds)
	settings.MainWindowDestroyDelaySeconds = normalizeMainWindowDestroyDelaySeconds(settings.MainWindowDestroyDelaySeconds)
	normalizeProviderConcurrencyLimits(&settings)
	normalizeProviderQuotaQueryPresets(&settings)
	normalizeClaudeModelRoutingSettings(&settings)
	settings.ClaudeProxyAuthField = normalizeClaudeProxyAuthField(settings.ClaudeProxyAuthField)

	// 同步开机自启动状态
	if as.autoStartService != nil {
		if settings.AutoStart {
			if err := as.autoStartService.Enable(); err != nil {
				return settings, err
			}
		} else {
			if err := as.autoStartService.Disable(); err != nil {
				return settings, err
			}
		}
	}

	if err := as.saveLocked(settings); err != nil {
		return settings, err
	}
	if as.codexSettings != nil && codexRuntimeSettingsChanged(previous, settings) {
		if err := as.codexSettings.ReapplyCurrentConfigForSettings(settings); err != nil {
			if rollbackErr := as.saveLocked(previous); rollbackErr != nil {
				return settings, fmt.Errorf("%w；回滚应用设置失败: %v", err, rollbackErr)
			}
			return settings, err
		}
		if settings.UnifyCodexSessionHistory && settings.UnifyCodexMigrateExisting {
			go func() {
				if _, err := MigrateCodexHistoryToUnifiedBucket(false); err != nil {
					fmt.Printf("[CodexHistory] 统一历史迁移失败: %v\n", err)
				}
			}()
		}
	}
	if as.claudeSettings != nil && previous.ClaudeProxyAuthField != settings.ClaudeProxyAuthField {
		if err := as.claudeSettings.ReapplyProxyAuthField(settings.ClaudeProxyAuthField); err != nil {
			if rollbackErr := as.saveLocked(previous); rollbackErr != nil {
				return settings, fmt.Errorf("%w；回滚应用设置失败: %v", err, rollbackErr)
			}
			return settings, err
		}
	}
	if as.claudeModelRouting != nil && claudeModelRoutingSettingsChanged(previous, settings) {
		as.claudeModelRouting.HandleSettingsChanged(previous, settings)
	}
	as.snapshot.Store(cloneAppSettings(settings))
	as.refreshFingerprint()
	if as.providerQuotaAutomation != nil {
		as.providerQuotaAutomation.HandleSettingsChanged(previous, settings)
	}
	return settings, nil
}

// SetLogsRefreshInterval persists only the logs refresh preference.
func (as *AppSettingsService) SetLogsRefreshInterval(seconds int) (AppSettings, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	settings, _ := as.GetAppSettings()
	settings.LogsRefreshIntervalSeconds = normalizeLogsRefreshIntervalSeconds(seconds)
	if err := as.saveLocked(settings); err != nil {
		return settings, err
	}
	as.snapshot.Store(cloneAppSettings(settings))
	as.refreshFingerprint()
	return settings, nil
}

// SetMainWindowDestroyDelay persists the main-window release delay without saving unrelated settings.
func (as *AppSettingsService) SetMainWindowDestroyDelay(seconds int, revision int64) (AppSettings, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	previous, _ := as.GetAppSettings()
	if revision < as.mainWindowDestroyDelayRevision {
		return previous, nil
	}
	settings := cloneAppSettings(previous)
	settings.MainWindowDestroyDelaySeconds = normalizeMainWindowDestroyDelaySeconds(seconds)

	// Publish first so a concurrent native close observes the latest delay while disk persistence completes.
	previousRevision := as.mainWindowDestroyDelayRevision
	as.mainWindowDestroyDelayRevision = revision
	as.snapshot.Store(cloneAppSettings(settings))
	if err := as.saveLocked(settings); err != nil {
		as.mainWindowDestroyDelayRevision = previousRevision
		as.snapshot.Store(previous)
		return previous, err
	}
	as.refreshFingerprint()
	return settings, nil
}

func claudeModelRoutingSettingsChanged(previous AppSettings, next AppSettings) bool {
	return previous.ClaudeModelRoutingEnabled != next.ClaudeModelRoutingEnabled ||
		previous.ClaudeModelAggregationEnabled != next.ClaudeModelAggregationEnabled ||
		previous.ClaudeModelMetadataMergeStrategy != next.ClaudeModelMetadataMergeStrategy
}

func codexRuntimeSettingsChanged(previous AppSettings, next AppSettings) bool {
	return previous.PreserveCodexOfficialAuth != next.PreserveCodexOfficialAuth ||
		previous.UnifyCodexSessionHistory != next.UnifyCodexSessionHistory ||
		previous.UnifyCodexMigrateExisting != next.UnifyCodexMigrateExisting
}

func (as *AppSettingsService) loadLocked() (AppSettings, error) {
	settings := as.defaultSettings()
	data, err := os.ReadFile(as.path)
	if err != nil {
		if os.IsNotExist(err) {
			if hasConfiguredClaudeModelRouting() {
				settings.ClaudeModelRoutingEnabled = true
				normalizeClaudeModelRoutingSettings(&settings)
				if err := as.saveLocked(settings); err != nil {
					return settings, err
				}
			}
			return settings, nil
		}
		return settings, err
	}
	if len(data) == 0 {
		if hasConfiguredClaudeModelRouting() {
			settings.ClaudeModelRoutingEnabled = true
			normalizeClaudeModelRoutingSettings(&settings)
			if err := as.saveLocked(settings); err != nil {
				return settings, err
			}
		}
		return settings, nil
	}
	var rawSettings map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawSettings); err != nil {
		return settings, err
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return settings, err
	}
	_, routingSettingExists := rawSettings["claude_model_routing_enabled"]
	if !routingSettingExists && hasConfiguredClaudeModelRouting() {
		settings.ClaudeModelRoutingEnabled = true
	}
	settings.HeatmapGranularity = normalizeHeatmapGranularity(settings.HeatmapGranularity)
	normalizeHeatmapDisplaySettings(&settings)
	settings.HomeProviderTabs = normalizeHomeProviderTabs(settings.HomeProviderTabs)
	normalizeBudgetSettings(&settings)
	settings.UpdateHistoryKeepCount = normalizeUpdateHistoryKeepCount(settings.UpdateHistoryKeepCount)
	settings.LogsRefreshIntervalSeconds = normalizeLogsRefreshIntervalSeconds(settings.LogsRefreshIntervalSeconds)
	settings.QuotaRecoveryIntervalSeconds = normalizeProviderQuotaRecoveryIntervalSeconds(settings.QuotaRecoveryIntervalSeconds)
	settings.MainWindowDestroyDelaySeconds = normalizeMainWindowDestroyDelaySeconds(settings.MainWindowDestroyDelaySeconds)
	normalizeProviderConcurrencyLimits(&settings)
	normalizeProviderQuotaQueryPresets(&settings)
	normalizeClaudeModelRoutingSettings(&settings)
	settings.ClaudeProxyAuthField = normalizeClaudeProxyAuthField(settings.ClaudeProxyAuthField)
	if !routingSettingExists {
		if err := as.saveLocked(settings); err != nil {
			return settings, err
		}
	}
	return settings, nil
}

func (as *AppSettingsService) saveLocked(settings AppSettings) error {
	dir := filepath.Dir(as.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	settings.HeatmapGranularity = normalizeHeatmapGranularity(settings.HeatmapGranularity)
	normalizeHeatmapDisplaySettings(&settings)
	settings.HomeProviderTabs = normalizeHomeProviderTabs(settings.HomeProviderTabs)
	normalizeBudgetSettings(&settings)
	settings.UpdateHistoryKeepCount = normalizeUpdateHistoryKeepCount(settings.UpdateHistoryKeepCount)
	settings.LogsRefreshIntervalSeconds = normalizeLogsRefreshIntervalSeconds(settings.LogsRefreshIntervalSeconds)
	settings.QuotaRecoveryIntervalSeconds = normalizeProviderQuotaRecoveryIntervalSeconds(settings.QuotaRecoveryIntervalSeconds)
	settings.MainWindowDestroyDelaySeconds = normalizeMainWindowDestroyDelaySeconds(settings.MainWindowDestroyDelaySeconds)
	normalizeProviderConcurrencyLimits(&settings)
	normalizeProviderQuotaQueryPresets(&settings)
	normalizeClaudeModelRoutingSettings(&settings)
	settings.ClaudeProxyAuthField = normalizeClaudeProxyAuthField(settings.ClaudeProxyAuthField)
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(as.path, data, 0o644)
}

func normalizeUpdateHistoryKeepCount(count int) int {
	if count < minUpdateHistoryKeepCount {
		return minUpdateHistoryKeepCount
	}
	if count > maxUpdateHistoryKeepCount {
		return maxUpdateHistoryKeepCount
	}
	return count
}

func normalizeMainWindowDestroyDelaySeconds(seconds int) int {
	if seconds < minMainWindowDestroyDelaySeconds {
		return minMainWindowDestroyDelaySeconds
	}
	if seconds > maxMainWindowDestroyDelaySeconds {
		return maxMainWindowDestroyDelaySeconds
	}
	return seconds
}

func normalizeBudgetCycleMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case budgetCycleModeWeekly:
		return budgetCycleModeWeekly
	case budgetCycleModeMonthly:
		return budgetCycleModeMonthly
	default:
		return budgetCycleModeDaily
	}
}

func normalizeBudgetRefreshTimeSetting(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "00:00"
	}
	return trimmed
}

func clampBudgetRefreshWeekday(value int) int {
	if value < minBudgetRefreshWeekday {
		return minBudgetRefreshWeekday
	}
	if value > maxBudgetRefreshWeekday {
		return maxBudgetRefreshWeekday
	}
	return value
}

func clampBudgetRefreshMonthDay(value int) int {
	if value == 0 {
		return defaultBudgetRefreshMonthDay
	}
	if value < minBudgetRefreshMonthDay {
		return minBudgetRefreshMonthDay
	}
	if value > maxBudgetRefreshMonthDay {
		return maxBudgetRefreshMonthDay
	}
	return value
}

func normalizeBudgetQuotaSetting(setting BudgetQuotaSetting) BudgetQuotaSetting {
	if math.IsNaN(setting.Total) || setting.Total < 0 {
		setting.Total = 0
	}
	setting.RefreshTime = normalizeBudgetRefreshTimeSetting(setting.RefreshTime)
	setting.RefreshDay = clampBudgetRefreshWeekday(setting.RefreshDay)
	setting.RefreshMonthDay = clampBudgetRefreshMonthDay(setting.RefreshMonthDay)
	return setting
}

func normalizeBudgetQuotaAdjustment(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

func defaultBudgetQuotaSettings() BudgetQuotaSettings {
	defaultSetting := BudgetQuotaSetting{
		Total:           0,
		RefreshTime:     "00:00",
		RefreshDay:      defaultBudgetRefreshWeekday,
		RefreshMonthDay: defaultBudgetRefreshMonthDay,
	}
	return BudgetQuotaSettings{
		FiveHour: defaultSetting,
		Daily:    defaultSetting,
		Weekly:   defaultSetting,
		Monthly:  defaultSetting,
		Total:    defaultSetting,
	}
}

func defaultBudgetQuotaAdjustments() BudgetQuotaAdjustments {
	return BudgetQuotaAdjustments{}
}

func isBudgetQuotaSettingsEmpty(settings BudgetQuotaSettings) bool {
	return settings.FiveHour.Total <= 0 &&
		settings.Daily.Total <= 0 &&
		settings.Weekly.Total <= 0 &&
		settings.Monthly.Total <= 0 &&
		settings.Total.Total <= 0
}

func resolveLegacyBudgetQuotaMode(cycleEnabled bool, cycleMode string) string {
	if cycleEnabled {
		return normalizeBudgetCycleMode(cycleMode)
	}
	return budgetCycleModeDaily
}

func normalizeBudgetQuotaSettings(settings BudgetQuotaSettings) BudgetQuotaSettings {
	settings.FiveHour = normalizeBudgetQuotaSetting(settings.FiveHour)
	settings.Daily = normalizeBudgetQuotaSetting(settings.Daily)
	settings.Weekly = normalizeBudgetQuotaSetting(settings.Weekly)
	settings.Monthly = normalizeBudgetQuotaSetting(settings.Monthly)
	settings.Total = normalizeBudgetQuotaSetting(settings.Total)
	return settings
}

func applyLegacyBudgetQuotaSetting(
	settings BudgetQuotaSettings,
	legacyTotal float64,
	cycleEnabled bool,
	cycleMode string,
	refreshTime string,
	refreshDay int,
	refreshMonthDay int,
) BudgetQuotaSettings {
	total := normalizeBudgetQuotaAdjustment(legacyTotal)
	if total <= 0 {
		return settings
	}

	target := BudgetQuotaSetting{
		Total:           total,
		RefreshTime:     normalizeBudgetRefreshTimeSetting(refreshTime),
		RefreshDay:      clampBudgetRefreshWeekday(refreshDay),
		RefreshMonthDay: clampBudgetRefreshMonthDay(refreshMonthDay),
	}

	switch resolveLegacyBudgetQuotaMode(cycleEnabled, cycleMode) {
	case budgetCycleModeWeekly:
		settings.Weekly = target
	case budgetCycleModeMonthly:
		settings.Monthly = target
	default:
		settings.Daily = target
	}

	return settings
}

func normalizeBudgetQuotaSettingsWithLegacy(
	settings BudgetQuotaSettings,
	legacyTotal float64,
	cycleEnabled bool,
	cycleMode string,
	refreshTime string,
	refreshDay int,
	refreshMonthDay int,
) BudgetQuotaSettings {
	normalized := normalizeBudgetQuotaSettings(settings)
	if isBudgetQuotaSettingsEmpty(normalized) {
		return applyLegacyBudgetQuotaSetting(
			normalized,
			legacyTotal,
			cycleEnabled,
			cycleMode,
			refreshTime,
			refreshDay,
			refreshMonthDay,
		)
	}
	return normalized
}

func normalizeBudgetQuotaAdjustments(adjustments BudgetQuotaAdjustments) BudgetQuotaAdjustments {
	adjustments.FiveHour = normalizeBudgetQuotaAdjustment(adjustments.FiveHour)
	adjustments.Daily = normalizeBudgetQuotaAdjustment(adjustments.Daily)
	adjustments.Weekly = normalizeBudgetQuotaAdjustment(adjustments.Weekly)
	adjustments.Monthly = normalizeBudgetQuotaAdjustment(adjustments.Monthly)
	adjustments.Total = normalizeBudgetQuotaAdjustment(adjustments.Total)
	return adjustments
}

func isBudgetQuotaAdjustmentsEmpty(adjustments BudgetQuotaAdjustments) bool {
	return adjustments.FiveHour == 0 &&
		adjustments.Daily == 0 &&
		adjustments.Weekly == 0 &&
		adjustments.Monthly == 0 &&
		adjustments.Total == 0
}

func applyLegacyBudgetQuotaAdjustment(
	adjustments BudgetQuotaAdjustments,
	legacy float64,
	cycleEnabled bool,
	cycleMode string,
) BudgetQuotaAdjustments {
	normalizedLegacy := normalizeBudgetQuotaAdjustment(legacy)
	if normalizedLegacy == 0 {
		return adjustments
	}

	switch resolveLegacyBudgetQuotaMode(cycleEnabled, cycleMode) {
	case budgetCycleModeWeekly:
		adjustments.Weekly = normalizedLegacy
	case budgetCycleModeMonthly:
		adjustments.Monthly = normalizedLegacy
	default:
		adjustments.Daily = normalizedLegacy
	}

	return adjustments
}

func normalizeBudgetQuotaUsedAdjustments(
	adjustments BudgetQuotaAdjustments,
	legacy float64,
	cycleEnabled bool,
	cycleMode string,
) BudgetQuotaAdjustments {
	normalized := normalizeBudgetQuotaAdjustments(adjustments)
	if isBudgetQuotaAdjustmentsEmpty(normalized) {
		return applyLegacyBudgetQuotaAdjustment(normalized, legacy, cycleEnabled, cycleMode)
	}
	return normalized
}

type legacyBudgetQuotaProjection struct {
	Total           float64
	UsedAdjustment  float64
	CycleEnabled    bool
	CycleMode       string
	RefreshTime     string
	RefreshDay      int
	RefreshMonthDay int
}

func defaultLegacyBudgetQuotaProjection() legacyBudgetQuotaProjection {
	return legacyBudgetQuotaProjection{
		Total:           0,
		UsedAdjustment:  0,
		CycleEnabled:    false,
		CycleMode:       budgetCycleModeDaily,
		RefreshTime:     normalizeBudgetRefreshTimeSetting(""),
		RefreshDay:      defaultBudgetRefreshWeekday,
		RefreshMonthDay: defaultBudgetRefreshMonthDay,
	}
}

func buildLegacyBudgetQuotaProjection(
	settings BudgetQuotaSettings,
	adjustments BudgetQuotaAdjustments,
) legacyBudgetQuotaProjection {
	projection := defaultLegacyBudgetQuotaProjection()

	candidates := []struct {
		mode       string
		setting    BudgetQuotaSetting
		adjustment float64
	}{
		{mode: budgetCycleModeDaily, setting: settings.Daily, adjustment: adjustments.Daily},
		{mode: budgetCycleModeWeekly, setting: settings.Weekly, adjustment: adjustments.Weekly},
		{mode: budgetCycleModeMonthly, setting: settings.Monthly, adjustment: adjustments.Monthly},
	}

	for _, candidate := range candidates {
		if candidate.setting.Total <= 0 {
			continue
		}
		return legacyBudgetQuotaProjection{
			Total:           candidate.setting.Total,
			UsedAdjustment:  candidate.adjustment,
			CycleEnabled:    true,
			CycleMode:       candidate.mode,
			RefreshTime:     candidate.setting.RefreshTime,
			RefreshDay:      candidate.setting.RefreshDay,
			RefreshMonthDay: candidate.setting.RefreshMonthDay,
		}
	}

	return projection
}

func normalizeBudgetSettings(settings *AppSettings) {
	if settings == nil {
		return
	}
	settings.BudgetCycleMode = normalizeBudgetCycleMode(settings.BudgetCycleMode)
	settings.BudgetRefreshTime = normalizeBudgetRefreshTimeSetting(settings.BudgetRefreshTime)
	settings.BudgetRefreshDay = clampBudgetRefreshWeekday(settings.BudgetRefreshDay)
	settings.BudgetRefreshMonthDay = clampBudgetRefreshMonthDay(settings.BudgetRefreshMonthDay)
	settings.BudgetQuotaUsedAdjustments = normalizeBudgetQuotaUsedAdjustments(
		settings.BudgetQuotaUsedAdjustments,
		settings.BudgetUsedAdjustment,
		settings.BudgetCycleEnabled,
		settings.BudgetCycleMode,
	)
	settings.BudgetQuotaSettings = normalizeBudgetQuotaSettingsWithLegacy(
		settings.BudgetQuotaSettings,
		settings.BudgetTotal,
		settings.BudgetCycleEnabled,
		settings.BudgetCycleMode,
		settings.BudgetRefreshTime,
		settings.BudgetRefreshDay,
		settings.BudgetRefreshMonthDay,
	)
	legacyProjection := buildLegacyBudgetQuotaProjection(
		settings.BudgetQuotaSettings,
		settings.BudgetQuotaUsedAdjustments,
	)
	settings.BudgetTotal = legacyProjection.Total
	settings.BudgetUsedAdjustment = legacyProjection.UsedAdjustment
	settings.BudgetCycleEnabled = legacyProjection.CycleEnabled
	settings.BudgetCycleMode = legacyProjection.CycleMode
	settings.BudgetRefreshTime = legacyProjection.RefreshTime
	settings.BudgetRefreshDay = legacyProjection.RefreshDay
	settings.BudgetRefreshMonthDay = legacyProjection.RefreshMonthDay
	settings.BudgetCycleModeCodex = normalizeBudgetCycleMode(settings.BudgetCycleModeCodex)
	settings.BudgetRefreshTimeCodex = normalizeBudgetRefreshTimeSetting(settings.BudgetRefreshTimeCodex)
	settings.BudgetRefreshDayCodex = clampBudgetRefreshWeekday(settings.BudgetRefreshDayCodex)
	settings.BudgetRefreshMonthDayCodex = clampBudgetRefreshMonthDay(settings.BudgetRefreshMonthDayCodex)
	settings.BudgetQuotaUsedAdjustmentsCodex = normalizeBudgetQuotaUsedAdjustments(
		settings.BudgetQuotaUsedAdjustmentsCodex,
		settings.BudgetUsedAdjustmentCodex,
		settings.BudgetCycleEnabledCodex,
		settings.BudgetCycleModeCodex,
	)
	settings.BudgetQuotaSettingsCodex = normalizeBudgetQuotaSettingsWithLegacy(
		settings.BudgetQuotaSettingsCodex,
		settings.BudgetTotalCodex,
		settings.BudgetCycleEnabledCodex,
		settings.BudgetCycleModeCodex,
		settings.BudgetRefreshTimeCodex,
		settings.BudgetRefreshDayCodex,
		settings.BudgetRefreshMonthDayCodex,
	)
	legacyProjectionCodex := buildLegacyBudgetQuotaProjection(
		settings.BudgetQuotaSettingsCodex,
		settings.BudgetQuotaUsedAdjustmentsCodex,
	)
	settings.BudgetTotalCodex = legacyProjectionCodex.Total
	settings.BudgetUsedAdjustmentCodex = legacyProjectionCodex.UsedAdjustment
	settings.BudgetCycleEnabledCodex = legacyProjectionCodex.CycleEnabled
	settings.BudgetCycleModeCodex = legacyProjectionCodex.CycleMode
	settings.BudgetRefreshTimeCodex = legacyProjectionCodex.RefreshTime
	settings.BudgetRefreshDayCodex = legacyProjectionCodex.RefreshDay
	settings.BudgetRefreshMonthDayCodex = legacyProjectionCodex.RefreshMonthDay
}

func normalizeHeatmapGranularity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case heatmapGranularityDaily:
		return heatmapGranularityDaily
	case heatmapGranularityHourly:
		return heatmapGranularityHourly
	default:
		return heatmapGranularityDaily
	}
}

func normalizeHeatmapDailyIntensityMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case heatmapDailyModeDailyPeak:
		return heatmapDailyModeDailyPeak
	default:
		return heatmapDailyModeHourlyScaled
	}
}

func normalizeHeatmapIntensityMetric(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case heatmapIntensityMetricCost:
		return heatmapIntensityMetricCost
	case heatmapIntensityMetricTotalTokens:
		return heatmapIntensityMetricTotalTokens
	case heatmapIntensityMetricInputTokens:
		return heatmapIntensityMetricInputTokens
	case heatmapIntensityMetricOutputTokens:
		return heatmapIntensityMetricOutputTokens
	case heatmapIntensityMetricReasoningTokens:
		return heatmapIntensityMetricReasoningTokens
	default:
		return heatmapIntensityMetricRequests
	}
}

func clampHeatmapInt(value int, min int, max int, fallback int) int {
	if value == 0 {
		return fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func normalizeHeatmapDisplaySettings(settings *AppSettings) {
	if settings == nil {
		return
	}
	settings.HeatmapDailyScaleFactor = clampHeatmapInt(
		settings.HeatmapDailyScaleFactor,
		minHeatmapDailyScale,
		maxHeatmapDailyScale,
		defaultHeatmapDailyScale,
	)
	settings.HeatmapDailyIntensityMode = normalizeHeatmapDailyIntensityMode(settings.HeatmapDailyIntensityMode)
	settings.HeatmapIntensityMetric = normalizeHeatmapIntensityMetric(settings.HeatmapIntensityMetric)

	l1 := clampHeatmapInt(
		settings.HeatmapIntensityStopL1,
		minHeatmapIntensityStop,
		maxHeatmapIntensityStop,
		defaultHeatmapIntensityL1,
	)
	l2 := clampHeatmapInt(
		settings.HeatmapIntensityStopL2,
		minHeatmapIntensityStop,
		maxHeatmapIntensityStop,
		defaultHeatmapIntensityL2,
	)
	l3 := clampHeatmapInt(
		settings.HeatmapIntensityStopL3,
		minHeatmapIntensityStop,
		maxHeatmapIntensityStop,
		defaultHeatmapIntensityL3,
	)

	if l2 <= l1 {
		l2 = l1 + 1
		if l2 > maxHeatmapIntensityStop {
			l2 = maxHeatmapIntensityStop
		}
	}
	if l3 <= l2 {
		l3 = l2 + 1
		if l3 > maxHeatmapIntensityStop {
			l3 = maxHeatmapIntensityStop
		}
	}
	if l3 <= l2 {
		l2 = l3 - 1
		if l2 < minHeatmapIntensityStop {
			l2 = minHeatmapIntensityStop
		}
	}
	if l2 <= l1 {
		l1 = l2 - 1
		if l1 < minHeatmapIntensityStop {
			l1 = minHeatmapIntensityStop
		}
	}

	settings.HeatmapIntensityStopL1 = l1
	settings.HeatmapIntensityStopL2 = l2
	settings.HeatmapIntensityStopL3 = l3
}

// LoadUpdateHistoryKeepCount 从应用设置读取更新包历史保留数量（读取失败时返回默认值）
func LoadUpdateHistoryKeepCount() int {
	home, err := os.UserHomeDir()
	if err != nil {
		return defaultUpdateHistoryKeepCount
	}

	path := filepath.Join(home, appSettingsDir, appSettingsFile)
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return defaultUpdateHistoryKeepCount
	}

	raw := struct {
		UpdateHistoryKeepCount int `json:"update_history_keep_count"`
	}{
		UpdateHistoryKeepCount: defaultUpdateHistoryKeepCount,
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return defaultUpdateHistoryKeepCount
	}

	return normalizeUpdateHistoryKeepCount(raw.UpdateHistoryKeepCount)
}
