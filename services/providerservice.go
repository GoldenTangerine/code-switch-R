package services

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/daodao97/xgo/xdb"
)

const (
	claudeManagedSubagentModel   = "code-switch-r-subagent"
	claudeDefaultModelMappingKey = "*"
)

// AvailabilityConfig 可用性监控高级配置
// 在可用性页面的"高级配置"弹窗中设置，可选
type AvailabilityConfig struct {
	TestModel    string `json:"testModel,omitempty"`    // 覆盖默认测试模型
	TestEndpoint string `json:"testEndpoint,omitempty"` // 覆盖默认测试端点
	Timeout      int    `json:"timeout,omitempty"`      // 覆盖默认超时（毫秒）
}

type Provider struct {
	ID      int64  `json:"id"` // 修复：使用 int64 支持大 ID 值
	Name    string `json:"name"`
	APIURL  string `json:"apiUrl"`
	APIKey  string `json:"apiKey"`
	Site    string `json:"officialSite"`
	Icon    string `json:"icon"`
	Tint    string `json:"tint"`
	Accent  string `json:"accent"`
	Enabled bool   `json:"enabled"`
	// 首页供应商日志图标是否隐藏未读红点；缺省为 false，兼容旧配置默认显示。
	HideLogBadge bool `json:"hideLogBadge,omitempty"`
	// 供应商分类：official / third_party / custom 等。
	Category string `json:"category,omitempty"`
	// 托管认证来源，例如 codex_oauth。为空时使用 APIKey。
	AuthProvider string `json:"authProvider,omitempty"`
	// 托管认证绑定账号 ID。为空时使用该认证来源的默认账号。
	AuthAccountID string `json:"authAccountId,omitempty"`
	// Claude API 格式（仅 Claude 供应商使用）
	// - anthropic: 原生 Anthropic Messages API，直接透传
	// - openai_chat: OpenAI Chat Completions，需要格式转换
	// - openai_responses: OpenAI Responses API，需要格式转换
	APIFormat string `json:"apiFormat,omitempty"`
	// Anthropic Cache TTL 覆盖（仅 Claude 原生 Anthropic Messages 供应商使用）
	// 空值表示不覆盖；支持 5m / 1h。
	AnthropicCacheTTL string `json:"anthropicCacheTTL,omitempty"`
	// 隐藏排序字段：仅控制启用 / 未启用组内顺序
	SortOrder int `json:"sortOrder,omitempty"`
	// 持久化启用组内顺序
	EnabledSortOrder int `json:"enabledSortOrder,omitempty"`
	// 持久化未启用组内顺序
	DisabledSortOrder int `json:"disabledSortOrder,omitempty"`

	// CLI 配置草稿 - 存储供应商关联的 CLI 可编辑配置
	CLIConfig map[string]interface{} `json:"cliConfig,omitempty"`

	// API 端点路径（可选）- 覆盖平台默认端点
	// 如：GLM 模型需要使用 /v1/chat/completions 而非 /v1/messages
	// 留空则使用平台默认（claude: /v1/messages, codex: /responses）
	APIEndpoint string `json:"apiEndpoint,omitempty"`

	// 模型白名单 - Provider 原生支持的模型名
	// 使用 map 实现 O(1) 查找，向后兼容（omitempty）
	SupportedModels map[string]bool `json:"supportedModels,omitempty"`

	// 模型映射 - 外部模型名 -> Provider 内部模型名
	// 支持精确匹配和通配符（如 "claude-*" -> "anthropic/claude-*"）
	ModelMapping map[string]string `json:"modelMapping,omitempty"`

	// 已关闭的模型映射 - 模型映射 key -> true
	// 未配置或不存在的 key 默认开启，确保兼容历史配置
	ModelMappingDisabled map[string]bool `json:"modelMappingDisabled,omitempty"`

	// 模型映射思考强度 - 模型映射 key -> 强制思考强度
	// 空值或未配置表示保留请求原有强度
	ModelMappingReasoningEfforts map[string]string `json:"modelMappingReasoningEfforts,omitempty"`

	// 模型映射 1M 上下文声明 - 模型映射 key -> true
	ModelMappingSupports1M map[string]bool `json:"modelMappingSupports1M,omitempty"`

	// 模型映射未命中策略：
	// - block: 未命中映射时跳过该 Provider（默认）
	// - passthrough: 未命中映射时按原模型名转发给该 Provider
	ModelMappingMissPolicy string `json:"modelMappingMissPolicy,omitempty"`

	// 模型映射未命中时允许透传的请求模型规则。
	// 仅在 Claude 模型路由开启且 miss policy=passthrough 时参与路由。
	ModelPassthroughPatterns []string `json:"modelPassthroughPatterns,omitempty"`

	// 请求体强制覆盖字段 - 仅在命中当前 Provider 转发时生效
	// 同名字段会覆盖，不存在的字段会新增；嵌套对象按层级递归写入
	RequestBodyOverrides map[string]interface{} `json:"requestBodyOverrides,omitempty"`

	// 优先级分组 - 数字越小优先级越高（1-10，默认 1）
	// 使用 omitempty 确保零值不序列化，向后兼容
	Level int `json:"level,omitempty"`

	// 实时并发：供应商最多同时处理的请求数（默认 5，范围 1-999）
	ProviderConcurrencyLimit int `json:"providerConcurrencyLimit,omitempty"`

	// 会话隔离：供应商最多承载的会话数（默认 5，范围 1-999）
	SessionMaxSessions int `json:"sessionMaxSessions,omitempty"`

	// 会话隔离：会话空闲释放时间，单位分钟（默认 5，范围 1-1440）
	SessionTTLMinutes int `json:"sessionTTLMinutes,omitempty"`

	// ========== 可用性监控字段（新增 v0.5.0） ==========

	// 可用性监控开关 - 在可用性页面配置
	// 启用后才会执行后台健康检查
	AvailabilityMonitorEnabled bool `json:"availabilityMonitorEnabled,omitempty"`

	// 连通性自动拉黑开关 - 在 Provider 编辑页面配置
	// 前置条件：AvailabilityMonitorEnabled 必须为 true
	// 启用后，当健康检查连续失败达到阈值时自动拉黑
	ConnectivityAutoBlacklist bool `json:"connectivityAutoBlacklist,omitempty"`

	// 可用性高级配置 - 可选，在可用性页面的"高级配置"中设置
	AvailabilityConfig *AvailabilityConfig `json:"availabilityConfig,omitempty"`

	// 认证方式 - bearer / x-api-key / 自定义 Header 名
	// 空值时使用平台默认（claude: x-api-key, codex: bearer）
	ConnectivityAuthType string `json:"connectivityAuthType,omitempty"`

	// ========== 供应商级别预算额度 ==========

	// 供应商级别预算额度配置（5 小时 / 日 / 周 / 月 / 总额度）
	// nil 表示未配置，各子项 Total 为 0 时不生效
	BudgetQuotaSettings *BudgetQuotaSettings `json:"budgetQuotaSettings,omitempty"`
	// 供应商级别预算额度当前已使用校准值（5 小时 / 日 / 周 / 月 / 总额度）
	// 用于在统计值基础上做手动校准，nil / 全 0 均视为未配置
	BudgetQuotaUsedAdjustments *BudgetQuotaAdjustments `json:"budgetQuotaUsedAdjustments,omitempty"`
	// 供应商额度查询类型：启用后首页卡片会改为按远端 Token Plan 查询结果展示额度
	ProviderQuotaQueryType string `json:"providerQuotaQueryType,omitempty"`
	// 供应商额度查询完整配置：兼容旧类型字段，并承载脚本/官方余额/NewAPI 等模板能力
	ProviderQuotaQueryConfig *ProviderQuotaQueryConfig `json:"providerQuotaQueryConfig,omitempty"`

	// ========== 旧字段（已废弃，仅用于读取迁移） ==========
	// 这些字段在保存时不再写入，但读取时会自动迁移到新字段

	// [已废弃] 连通性检测开关 - 迁移到 AvailabilityMonitorEnabled
	ConnectivityCheck bool `json:"connectivityCheck,omitempty"`

	// [已废弃] 连通性检测模型 - 迁移到 AvailabilityConfig.TestModel
	ConnectivityTestModel string `json:"connectivityTestModel,omitempty"`

	// [已废弃] 连通性检测端点 - 迁移到 AvailabilityConfig.TestEndpoint
	ConnectivityTestEndpoint string `json:"connectivityTestEndpoint,omitempty"`

	// 内部字段：配置验证错误（不持久化）
	configErrors []string `json:"-"`
}

type providerEnvelope struct {
	Providers []Provider `json:"providers"`
}

type providerRename struct {
	ProviderID string
	OldName    string
	NewName    string
}

const (
	ModelMappingMissPolicyBlock       = "block"
	ModelMappingMissPolicyPassthrough = "passthrough"
)

func cloneJSONLikeMap(value map[string]interface{}) map[string]interface{} {
	if value == nil {
		return map[string]interface{}{}
	}

	payload, err := json.Marshal(value)
	if err != nil || len(payload) == 0 {
		return map[string]interface{}{}
	}

	var clone map[string]interface{}
	if err := json.Unmarshal(payload, &clone); err != nil || clone == nil {
		return map[string]interface{}{}
	}

	return clone
}

type ProviderService struct {
	mu                        sync.Mutex
	claudeModelRouting        *ClaudeModelRoutingService
	claudeSettings            *ClaudeSettingsService
	claudeSubagentReconcileMu sync.Mutex

	pricingCacheMu sync.RWMutex
	pricingCache   map[string]providerModelPricingCacheEntry
	modelPricing   *ModelPricingService

	providerPricingOverridesMu sync.RWMutex
	providerPricingOverrides   providerPricingOverrideStore

	snapshotMu          sync.RWMutex
	snapshots           map[string]providerConfigSnapshot
	snapshotStop        chan struct{}
	snapshotDone        chan struct{}
	snapshotLifecycleMu sync.Mutex
	snapshotStarted     bool
	snapshotStopped     bool
}

type providerConfigSnapshot struct {
	providers   []Provider
	fingerprint [sha256.Size]byte
	exists      bool
}

func cloneProvider(provider Provider) Provider {
	cloned := provider
	if provider.CLIConfig != nil {
		cloned.CLIConfig = cloneJSONLikeMap(provider.CLIConfig)
	}
	if provider.SupportedModels != nil {
		cloned.SupportedModels = make(map[string]bool, len(provider.SupportedModels))
		for key, value := range provider.SupportedModels {
			cloned.SupportedModels[key] = value
		}
	}
	if provider.ModelMapping != nil {
		cloned.ModelMapping = make(map[string]string, len(provider.ModelMapping))
		for key, value := range provider.ModelMapping {
			cloned.ModelMapping[key] = value
		}
	}
	if provider.ModelMappingDisabled != nil {
		cloned.ModelMappingDisabled = make(map[string]bool, len(provider.ModelMappingDisabled))
		for key, value := range provider.ModelMappingDisabled {
			cloned.ModelMappingDisabled[key] = value
		}
	}
	if provider.ModelMappingReasoningEfforts != nil {
		cloned.ModelMappingReasoningEfforts = make(map[string]string, len(provider.ModelMappingReasoningEfforts))
		for key, value := range provider.ModelMappingReasoningEfforts {
			cloned.ModelMappingReasoningEfforts[key] = value
		}
	}
	if provider.ModelMappingSupports1M != nil {
		cloned.ModelMappingSupports1M = make(map[string]bool, len(provider.ModelMappingSupports1M))
		for key, value := range provider.ModelMappingSupports1M {
			cloned.ModelMappingSupports1M[key] = value
		}
	}
	cloned.ModelPassthroughPatterns = append([]string(nil), provider.ModelPassthroughPatterns...)
	if provider.RequestBodyOverrides != nil {
		cloned.RequestBodyOverrides = cloneJSONLikeMap(provider.RequestBodyOverrides)
	}
	if provider.AvailabilityConfig != nil {
		availability := *provider.AvailabilityConfig
		cloned.AvailabilityConfig = &availability
	}
	if provider.BudgetQuotaSettings != nil {
		budget := *provider.BudgetQuotaSettings
		cloned.BudgetQuotaSettings = &budget
	}
	if provider.BudgetQuotaUsedAdjustments != nil {
		adjustments := *provider.BudgetQuotaUsedAdjustments
		cloned.BudgetQuotaUsedAdjustments = &adjustments
	}
	if provider.ProviderQuotaQueryConfig != nil {
		quotaConfig := *provider.ProviderQuotaQueryConfig
		cloned.ProviderQuotaQueryConfig = &quotaConfig
	}
	cloned.configErrors = append([]string(nil), provider.configErrors...)
	return cloned
}

func cloneProviders(providers []Provider) []Provider {
	if providers == nil {
		return nil
	}
	cloned := make([]Provider, len(providers))
	for index, provider := range providers {
		cloned[index] = cloneProvider(provider)
	}
	return cloned
}

func snapshotProviderFile(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return data, true, nil
	}
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	return nil, false, err
}

func restoreProviderFile(path string, existed bool, data []byte) error {
	if !existed {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	restoreTmp := path + ".restore.tmp"
	if err := os.WriteFile(restoreTmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(restoreTmp, path)
}

func NewProviderService() *ProviderService {
	svc := &ProviderService{
		pricingCache:             make(map[string]providerModelPricingCacheEntry),
		providerPricingOverrides: newProviderPricingOverrideStore(),
		snapshots:                make(map[string]providerConfigSnapshot),
		snapshotStop:             make(chan struct{}),
		snapshotDone:             make(chan struct{}),
	}
	overrides, err := loadProviderPricingOverridesFromDB()
	if err != nil {
		log.Printf("provider pricing overrides load failed: %v", err)
		return svc
	}
	svc.providerPricingOverrides = overrides
	return svc
}

func (ps *ProviderService) BindModelPricingService(modelPricing *ModelPricingService) {
	if ps == nil {
		return
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.modelPricing = modelPricing
}

func (ps *ProviderService) BindClaudeModelRoutingService(routing *ClaudeModelRoutingService) {
	if ps == nil {
		return
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.claudeModelRouting = routing
}

func (ps *ProviderService) BindClaudeSettingsService(settings *ClaudeSettingsService) {
	if ps == nil {
		return
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.claudeSettings = settings
}

func (ps *ProviderService) ReconcileClaudeSubagentModel() error {
	if ps == nil {
		return nil
	}
	ps.claudeSubagentReconcileMu.Lock()
	defer ps.claudeSubagentReconcileMu.Unlock()

	providers, err := ps.LoadProviders("claude")
	if err != nil {
		return err
	}
	ps.mu.Lock()
	settings := ps.claudeSettings
	ps.mu.Unlock()
	if settings == nil {
		return nil
	}
	return settings.SetSubagentModelRequired(claudeProvidersRequireManagedSubagentModel(providers))
}

func (ps *ProviderService) Start() error {
	if ps == nil {
		return nil
	}
	ps.snapshotLifecycleMu.Lock()
	defer ps.snapshotLifecycleMu.Unlock()
	if ps.snapshotStarted || ps.snapshotStopped {
		return nil
	}
	ps.snapshotStarted = true
	go ps.watchProviderSnapshots()
	return nil
}
func (ps *ProviderService) Stop() error {
	if ps == nil || ps.snapshotStop == nil {
		return nil
	}
	ps.snapshotLifecycleMu.Lock()
	started := ps.snapshotStarted
	if !ps.snapshotStopped {
		ps.snapshotStopped = true
		if started {
			close(ps.snapshotStop)
		}
	}
	ps.snapshotLifecycleMu.Unlock()
	if started {
		<-ps.snapshotDone
	}
	return nil
}

func providerFilePath(kind string) (string, error) {
	return providerConfigPath(kind, true)
}

func providerConfigPath(kind string, create bool) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".code-switch")
	if create {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}

	switch strings.ToLower(kind) {
	case "claude", "claude-code", "claude_code":
		return filepath.Join(dir, "claude-code.json"), nil
	case "codex":
		providersDir := filepath.Join(dir, "providers")
		if create {
			if err := os.MkdirAll(providersDir, 0o755); err != nil {
				return "", err
			}
		}
		return filepath.Join(providersDir, "codex.json"), nil
	case "opencode":
		providersDir := filepath.Join(dir, "providers")
		if create {
			if err := os.MkdirAll(providersDir, 0o755); err != nil {
				return "", err
			}
		}
		return filepath.Join(providersDir, "opencode.json"), nil
	default:
		// 支持自定义 CLI 工具的供应商存储：custom:{tool-id}
		if strings.HasPrefix(kind, "custom:") {
			toolId := strings.TrimPrefix(kind, "custom:")
			if toolId == "" {
				return "", fmt.Errorf("invalid custom provider kind: %s", kind)
			}
			// 存储在 providers 子目录下
			providersDir := filepath.Join(dir, "providers")
			if create {
				if err := os.MkdirAll(providersDir, 0o755); err != nil {
					return "", err
				}
			}
			return filepath.Join(providersDir, toolId+".json"), nil
		}
		return "", fmt.Errorf("unknown provider type: %s", kind)
	}
}

func legacyProviderFilePathNoCreate(kind string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch strings.ToLower(kind) {
	case "codex":
		return filepath.Join(home, ".code-switch", "codex.json"), nil
	default:
		return "", nil
	}
}

func resolveProviderReadPath(kind string) (string, error) {
	path, err := providerConfigPath(kind, false)
	if err != nil {
		return "", err
	}
	if providerConfigFileExists(path) {
		return path, nil
	}

	legacyPath, err := legacyProviderFilePathNoCreate(kind)
	if err != nil {
		return "", err
	}
	if providerConfigFileExists(legacyPath) {
		return legacyPath, nil
	}

	return path, nil
}

func providerConfigFileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}

	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (ps *ProviderService) SaveProviders(kind string, providers []Provider) error {
	ps.mu.Lock()
	err := ps.saveProvidersLocked(kind, providers)
	ps.mu.Unlock()
	if err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(kind), "claude") {
		if err := ps.ReconcileClaudeSubagentModel(); err != nil {
			log.Printf("[ProviderService] 协调 Claude Subagent 模型失败: %v", err)
		}
	}
	return nil
}

// loadProvidersRaw 原样读取配置文件（不迁移、不保存）
// 用于内部需要读取现有配置但不触发迁移的场景（如名称校验）
func (ps *ProviderService) loadProvidersRaw(kind string) ([]Provider, error) {
	path, err := resolveProviderReadPath(kind)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var envelope providerEnvelope
	if len(data) == 0 {
		return []Provider{}, nil
	}

	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}

	return envelope.Providers, nil
}

// saveProvidersLocked 内部保存方法，调用方必须已持有锁
func (ps *ProviderService) saveProvidersLocked(kind string, providers []Provider) error {
	providers = append([]Provider(nil), providers...)
	path, err := providerFilePath(kind)
	if err != nil {
		return err
	}
	providers = filterPersistentProviders(kind, providers)

	// 加载现有配置，用于检查 name 是否被修改
	// 使用原样读取，避免触发迁移导致死锁
	existingProviders, err := ps.loadProvidersRaw(kind)
	if err != nil {
		return err
	}
	existingProviders = filterPersistentProviders(kind, existingProviders)
	nameByID := make(map[int64]string, len(existingProviders))
	for _, p := range existingProviders {
		nameByID[p.ID] = p.Name
	}
	renames := make([]providerRename, 0)

	// 验证每个 provider 的配置，并清除旧字段
	validationErrors := make([]string, 0)
	for i := range providers {
		p := &providers[i]

		if oldName, ok := nameByID[p.ID]; ok && strings.TrimSpace(oldName) != strings.TrimSpace(p.Name) {
			renames = append(renames, providerRename{
				ProviderID: fmt.Sprintf("%d", p.ID),
				OldName:    oldName,
				NewName:    p.Name,
			})
		}

		// 验证模型配置
		p.ModelPassthroughPatterns = normalizeModelPassthroughPatterns(p.ModelPassthroughPatterns)
		p.ModelMappingDisabled = normalizeModelMappingDisabled(p.ModelMappingDisabled, p.ModelMapping)
		p.ModelMappingSupports1M = normalizeModelMappingSupports1M(p.ModelMappingSupports1M, p.ModelMapping)
		if errs := p.validateConfigurationForKind(kind); len(errs) > 0 {
			for _, errMsg := range errs {
				validationErrors = append(validationErrors, fmt.Sprintf("[%s] %s", p.Name, errMsg))
			}
		}

		// 清除旧连通性字段，确保保存时不再写入
		p.clearLegacyFields()
	}

	// 如果有验证错误，返回汇总错误
	if len(validationErrors) > 0 {
		return fmt.Errorf("配置验证失败：\n  - %s", strings.Join(validationErrors, "\n  - "))
	}

	data, err := json.MarshalIndent(providerEnvelope{Providers: providers}, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	ps.storeProviderSnapshot(kind, providers, sha256.Sum256(data), true)
	if len(renames) > 0 {
		syncProviderIdentityRenamesBestEffort(kind, renames)
	}
	if strings.EqualFold(strings.TrimSpace(kind), "claude") && ps.claudeModelRouting != nil {
		previousSnapshot := cloneProviders(existingProviders)
		nextSnapshot := cloneProviders(providers)
		ps.claudeModelRouting.HandleProvidersChanged(previousSnapshot, nextSnapshot)
	}
	return nil
}

func syncProviderIdentityRenamesBestEffort(kind string, renames []providerRename) {
	db, err := xdb.DB("default")
	if err != nil {
		log.Printf("[ProviderService] 跳过供应商改名关联数据同步: %v", err)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("[ProviderService] 启动供应商改名关联数据同步事务失败: %v", err)
		return
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	synced := false
	for _, rename := range renames {
		hasAssociatedData, err := providerRenameHasAssociatedDataWithExec(tx, kind, rename.ProviderID, rename.OldName, rename.NewName)
		if err != nil {
			log.Printf("[ProviderService] 检查供应商改名关联数据失败 (kind=%s,id=%s,%q->%q): %v",
				kind, rename.ProviderID, rename.OldName, rename.NewName, err)
			continue
		}
		if !hasAssociatedData {
			continue
		}

		if err := syncProviderIdentityRenameTx(tx, kind, rename.ProviderID, rename.OldName, rename.NewName); err != nil {
			log.Printf("[ProviderService] 同步供应商改名关联数据失败 (kind=%s,id=%s,%q->%q): %v",
				kind, rename.ProviderID, rename.OldName, rename.NewName, err)
			return
		}
		synced = true
	}

	if !synced {
		_ = tx.Rollback()
		committed = true
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("[ProviderService] 提交供应商改名关联数据同步事务失败: %v", err)
		return
	}
	committed = true
}

func filterPersistentProviders(kind string, providers []Provider) []Provider {
	if kind != "codex" {
		return providers
	}
	filtered := make([]Provider, 0, len(providers))
	for _, provider := range providers {
		if isCodexOfficialProviderCard(provider) {
			continue
		}
		filtered = append(filtered, provider)
	}
	return filtered
}

func filterRuntimeProviders(kind string, providers []Provider) []Provider {
	if kind != "codex" {
		return providers
	}
	filtered := make([]Provider, 0, len(providers))
	for _, provider := range providers {
		if isCodexOfficialProviderCard(provider) {
			continue
		}
		filtered = append(filtered, provider)
	}
	return filtered
}

func (ps *ProviderService) LoadProviders(kind string) ([]Provider, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	ps.snapshotMu.RLock()
	snapshot, ok := ps.snapshots[kind]
	ps.snapshotMu.RUnlock()
	if ok {
		return cloneProviders(snapshot.providers), nil
	}

	providers, err := ps.loadProvidersFromDisk(kind)
	if err != nil {
		return nil, err
	}
	fingerprint, exists, fingerprintErr := providerConfigFingerprint(kind)
	if fingerprintErr != nil {
		return nil, fingerprintErr
	}
	ps.storeProviderSnapshot(kind, providers, fingerprint, exists)
	return cloneProviders(providers), nil
}

func (ps *ProviderService) loadProvidersFromDisk(kind string) ([]Provider, error) {
	path, err := resolveProviderReadPath(kind)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var envelope providerEnvelope
	if len(data) == 0 {
		return []Provider{}, nil
	}

	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}

	// 执行字段迁移：将旧字段值迁移到新字段
	migrated := false
	envelope.Providers, migrated = ensureBuiltInProviders(kind, envelope.Providers, migrated)
	for i := range envelope.Providers {
		if envelope.Providers[i].migrateFromLegacy() {
			migrated = true
		}
	}

	// 如果有迁移，记录日志并持久化到磁盘
	if migrated {
		fmt.Printf("[ProviderService] 已从旧配置迁移可用性字段 (kind=%s)\n", kind)
		// 自动保存迁移后的配置（使用带锁的保存方法避免死锁）
		ps.mu.Lock()
		err := ps.saveProvidersLocked(kind, envelope.Providers)
		ps.mu.Unlock()

		if err != nil {
			log.Printf("[ProviderService] 迁移后写入失败: %v\n", err)
		} else {
			fmt.Printf("[ProviderService] 迁移后的配置已保存到磁盘 (kind=%s)\n", kind)
		}
	}

	return envelope.Providers, nil
}

func providerConfigFingerprint(kind string) ([sha256.Size]byte, bool, error) {
	path, err := resolveProviderReadPath(kind)
	if err != nil {
		return [sha256.Size]byte{}, false, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return [sha256.Size]byte{}, false, nil
	}
	if err != nil {
		return [sha256.Size]byte{}, false, err
	}
	return sha256.Sum256(data), true, nil
}

func (ps *ProviderService) storeProviderSnapshot(kind string, providers []Provider, fingerprint [sha256.Size]byte, exists bool) {
	ps.snapshotMu.Lock()
	ps.snapshots[kind] = providerConfigSnapshot{
		providers:   cloneProviders(providers),
		fingerprint: fingerprint,
		exists:      exists,
	}
	ps.snapshotMu.Unlock()
}

func (ps *ProviderService) watchProviderSnapshots() {
	defer close(ps.snapshotDone)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ps.refreshProviderSnapshots()
		case <-ps.snapshotStop:
			return
		}
	}
}

func (ps *ProviderService) refreshProviderSnapshots() {
	ps.snapshotMu.RLock()
	kinds := make([]string, 0, len(ps.snapshots))
	for kind := range ps.snapshots {
		kinds = append(kinds, kind)
	}
	ps.snapshotMu.RUnlock()

	for _, kind := range kinds {
		fingerprint, exists, err := providerConfigFingerprint(kind)
		if err != nil {
			log.Printf("[ProviderService] 检查供应商快照失败 (kind=%s): %v", kind, err)
			continue
		}
		ps.snapshotMu.RLock()
		current, ok := ps.snapshots[kind]
		ps.snapshotMu.RUnlock()
		if ok && current.exists == exists && current.fingerprint == fingerprint {
			continue
		}
		providers, err := ps.loadProvidersFromDisk(kind)
		if err != nil {
			log.Printf("[ProviderService] 外部供应商配置解析失败，继续使用上一份快照 (kind=%s): %v", kind, err)
			continue
		}
		latestFingerprint, latestExists, latestErr := providerConfigFingerprint(kind)
		if latestErr != nil || latestExists != exists || latestFingerprint != fingerprint {
			continue
		}
		ps.storeProviderSnapshot(kind, providers, fingerprint, exists)
		ps.mu.Lock()
		routing := ps.claudeModelRouting
		ps.mu.Unlock()
		if kind == "claude" && routing != nil {
			routing.HandleProvidersChanged(
				cloneProviders(current.providers),
				cloneProviders(providers),
			)
		}
		if kind == "claude" {
			if err := ps.ReconcileClaudeSubagentModel(); err != nil {
				log.Printf("[ProviderService] 协调外部 Claude Subagent 模型配置失败: %v", err)
			}
		}
	}
}

// loadProvidersNoLock 内部加载方法，在持有锁的情况下调用（避免递归加锁）
// 执行配置加载和迁移，如有迁移则直接保存（不再加锁）
// 仅在已持有 ps.mu 锁的上下文中调用（如 DuplicateProvider）
func (ps *ProviderService) loadProvidersNoLock(kind string) ([]Provider, error) {
	path, err := resolveProviderReadPath(kind)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var envelope providerEnvelope
	if len(data) == 0 {
		return []Provider{}, nil
	}

	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}

	// 执行字段迁移（但不保存，避免在持锁时再次加锁）
	migrated := false
	envelope.Providers, migrated = ensureBuiltInProviders(kind, envelope.Providers, migrated)
	for i := range envelope.Providers {
		if envelope.Providers[i].migrateFromLegacy() {
			migrated = true
		}
	}

	if migrated {
		fmt.Printf("[ProviderService] 已从旧配置迁移可用性字段 (kind=%s, 锁内模式)\n", kind)
		// 在锁内模式下，直接保存而不再加锁
		if err := ps.saveProvidersLocked(kind, envelope.Providers); err != nil {
			log.Printf("[ProviderService] 锁内迁移保存失败: %v\n", err)
		}
	}

	return envelope.Providers, nil
}

func ensureBuiltInProviders(kind string, providers []Provider, migrated bool) ([]Provider, bool) {
	if kind != "codex" {
		return providers, migrated
	}
	return filterPersistentProviders(kind, providers), migrated
}

func hasCodexOfficialProvider(providers []Provider) bool {
	for _, provider := range providers {
		if isCodexOfficialProviderCard(provider) {
			return true
		}
	}
	return false
}

func codexOfficialProviderCard() Provider {
	return Provider{
		ID:       200,
		Name:     "Codex 官方登录",
		Site:     "https://chatgpt.com/codex",
		Icon:     "openai",
		Tint:     "rgba(16, 185, 129, 0.16)",
		Accent:   "#10b981",
		Enabled:  true,
		Category: "official",
	}
}

func isCodexOfficialProviderCard(provider Provider) bool {
	if provider.ID != 200 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(provider.Category), "official") || strings.EqualFold(strings.TrimSpace(provider.Name), "Codex 官方登录")
}

// migrateFromLegacy 将旧连通性字段迁移到新可用性字段
// 返回 true 表示发生了迁移
func (p *Provider) migrateFromLegacy() bool {
	migrated := false

	// 迁移 ConnectivityCheck -> AvailabilityMonitorEnabled
	// 仅当新字段未设置（false）且旧字段已设置（true）时迁移
	if p.ConnectivityCheck && !p.AvailabilityMonitorEnabled {
		p.AvailabilityMonitorEnabled = true
		migrated = true
	}

	// 迁移测试模型和端点到 AvailabilityConfig
	if p.ConnectivityTestModel != "" || p.ConnectivityTestEndpoint != "" {
		if p.AvailabilityConfig == nil {
			p.AvailabilityConfig = &AvailabilityConfig{}
		}
		// 仅当新字段为空时才从旧字段迁移
		if p.AvailabilityConfig.TestModel == "" && p.ConnectivityTestModel != "" {
			p.AvailabilityConfig.TestModel = p.ConnectivityTestModel
			migrated = true
		}
		if p.AvailabilityConfig.TestEndpoint == "" && p.ConnectivityTestEndpoint != "" {
			p.AvailabilityConfig.TestEndpoint = p.ConnectivityTestEndpoint
			migrated = true
		}
	}

	return migrated
}

// clearLegacyFields 清除旧字段值，使其在序列化时被 omitempty 跳过
func (p *Provider) clearLegacyFields() {
	p.ConnectivityCheck = false
	p.ConnectivityTestModel = ""
	p.ConnectivityTestEndpoint = ""
	// 注意：ConnectivityAuthType 现在是活跃字段，不再清除
}

// DuplicateProvider 复制供应商配置，生成新的副本
// 返回新创建的 Provider 对象
func (ps *ProviderService) DuplicateProvider(kind string, sourceID int64) (*Provider, error) {
	// 1. 先加锁，避免并发修改
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// 2. 加载现有配置（在锁内完成，确保数据一致性）
	// 注意：LoadProviders 内部可能触发迁移保存，会再次尝试加锁导致死锁
	// 因此使用不加锁的内部加载逻辑
	providers, err := ps.loadProvidersNoLock(kind)
	if err != nil {
		return nil, fmt.Errorf("加载供应商配置失败: %w", err)
	}

	// 3. 查找源供应商
	var source *Provider
	for i := range providers {
		if providers[i].ID == sourceID {
			source = &providers[i]
			break
		}
	}
	if source == nil {
		return nil, fmt.Errorf("未找到 ID 为 %d 的供应商", sourceID)
	}

	// 4. 生成新 ID（当前最大 ID + 1）
	maxID := int64(0)
	for _, p := range providers {
		if p.ID > maxID {
			maxID = p.ID
		}
	}
	newID := maxID + 1

	// 5. 克隆配置（深拷贝）
	cloned := &Provider{
		ID:                     newID,
		Name:                   source.Name + " (副本)",
		APIURL:                 source.APIURL,
		APIKey:                 source.APIKey,
		Site:                   source.Site,
		Icon:                   source.Icon,
		Tint:                   source.Tint,
		Accent:                 source.Accent,
		Enabled:                false, // 默认禁用，避免与源供应商冲突
		APIFormat:              source.APIFormat,
		AuthProvider:           source.AuthProvider,
		AuthAccountID:          source.AuthAccountID,
		Level:                  source.Level,
		APIEndpoint:            source.APIEndpoint, // 复制端点配置
		ModelMappingMissPolicy: normalizeModelMappingMissPolicy(source.ModelMappingMissPolicy),
		// 可用性监控配置
		AvailabilityMonitorEnabled: source.AvailabilityMonitorEnabled,
		ConnectivityAutoBlacklist:  false, // 副本默认关闭自动拉黑
	}

	if source.CLIConfig != nil {
		cloned.CLIConfig = cloneCLIEditableMap(source.CLIConfig)
	}

	// 6. 深拷贝 map（避免共享引用）
	if source.SupportedModels != nil {
		cloned.SupportedModels = make(map[string]bool, len(source.SupportedModels))
		for k, v := range source.SupportedModels {
			cloned.SupportedModels[k] = v
		}
	}

	// 深拷贝 AvailabilityConfig
	if source.AvailabilityConfig != nil {
		cloned.AvailabilityConfig = &AvailabilityConfig{
			TestModel:    source.AvailabilityConfig.TestModel,
			TestEndpoint: source.AvailabilityConfig.TestEndpoint,
			Timeout:      source.AvailabilityConfig.Timeout,
		}
	}

	if source.ModelMapping != nil {
		cloned.ModelMapping = make(map[string]string, len(source.ModelMapping))
		for k, v := range source.ModelMapping {
			cloned.ModelMapping[k] = v
		}
	}
	if source.ModelMappingDisabled != nil {
		cloned.ModelMappingDisabled = make(map[string]bool, len(source.ModelMappingDisabled))
		for k, v := range source.ModelMappingDisabled {
			cloned.ModelMappingDisabled[k] = v
		}
	}
	if source.ModelMappingReasoningEfforts != nil {
		cloned.ModelMappingReasoningEfforts = make(map[string]string, len(source.ModelMappingReasoningEfforts))
		for k, v := range source.ModelMappingReasoningEfforts {
			cloned.ModelMappingReasoningEfforts[k] = v
		}
	}
	if source.ModelMappingSupports1M != nil {
		cloned.ModelMappingSupports1M = make(map[string]bool, len(source.ModelMappingSupports1M))
		for k, v := range source.ModelMappingSupports1M {
			cloned.ModelMappingSupports1M[k] = v
		}
	}
	if source.ModelPassthroughPatterns != nil {
		cloned.ModelPassthroughPatterns = append([]string(nil), source.ModelPassthroughPatterns...)
	}

	if source.RequestBodyOverrides != nil {
		cloned.RequestBodyOverrides = cloneJSONLikeMap(source.RequestBodyOverrides)
	}

	// 7. 添加到列表并保存（使用内部方法避免死锁）
	providers = append(providers, *cloned)
	if err := ps.saveProvidersLocked(kind, providers); err != nil {
		return nil, fmt.Errorf("保存副本失败: %w", err)
	}

	return cloned, nil
}

// IsModelSupported 检查 provider 是否支持指定的模型
// 支持条件：1) 模型在 SupportedModels 中（精确或通配符匹配）
//  2. 模型在 ModelMapping 的 key 中（精确或通配符匹配）
//  3. ModelMapping 未命中，但 modelMappingMissPolicy=passthrough
func (p *Provider) IsModelSupported(modelName string) bool {
	// 向后兼容：如果未配置白名单和映射，假设支持所有模型
	if (p.SupportedModels == nil || len(p.SupportedModels) == 0) &&
		(p.ModelMapping == nil || len(p.ModelMapping) == 0) {
		return true
	}

	// 场景 A：Provider 原生支持该模型（精确匹配）
	if p.SupportedModels != nil && p.SupportedModels[modelName] {
		return true
	}

	// 场景 A+：Provider 原生支持该模型（通配符匹配）
	if p.SupportedModels != nil {
		for supportedModel := range p.SupportedModels {
			if matchWildcard(supportedModel, modelName) {
				return true
			}
		}
	}

	// 场景 B：Provider 通过映射支持该模型（精确匹配）
	if p.hasModelMappingForModel(modelName) {
		return true
	}

	// 场景 B-：映射未命中，但允许按原模型名透传
	if p.shouldPassthroughModelMappingMiss() {
		return true
	}

	// 场景 C：不支持
	return false
}

// IsNativeModelSupported 检查 Provider 原生支持的模型名（只看 SupportedModels）
// 当未配置 SupportedModels 时，默认视为不限制原生模型
func (p *Provider) IsNativeModelSupported(modelName string) bool {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return true
	}

	if p.SupportedModels == nil || len(p.SupportedModels) == 0 {
		return true
	}

	if p.SupportedModels[modelName] {
		return true
	}

	for supportedModel := range p.SupportedModels {
		if matchWildcard(supportedModel, modelName) {
			return true
		}
	}

	return false
}

// IsResolvedModelSupported 检查在映射 / 请求体覆盖完成后，最终模型是否允许路由到当前 Provider
// requestedModel 表示原始请求模型，effectiveModel 表示最终将发往上游的模型
func (p *Provider) IsResolvedModelSupported(requestedModel, effectiveModel string) bool {
	requestedModel = strings.TrimSpace(requestedModel)
	effectiveModel = strings.TrimSpace(effectiveModel)

	if requestedModel == "" && effectiveModel == "" {
		return true
	}

	// 最终模型未变化时，沿用原有的兼容性判断逻辑
	if effectiveModel == "" || effectiveModel == requestedModel {
		if requestedModel == "" {
			return true
		}
		return p.IsModelSupported(requestedModel)
	}

	// 最终模型发生变化（模型映射或请求体强制覆盖），
	// 应按 Provider 原生模型能力判断，而不是继续看外部模型名。
	return p.IsNativeModelSupported(effectiveModel)
}

func (p *Provider) isClaudeRoutedModelSupported(requestedModel, effectiveModel string) bool {
	// Claude 显式映射代表用户确认，映射命中后不再校验最终模型白名单。
	if mappedModel, matched := p.resolveModelMapping(strings.TrimSpace(requestedModel)); matched && strings.TrimSpace(mappedModel) != "" {
		return true
	}
	return p.IsResolvedModelSupported(requestedModel, effectiveModel)
}

// GetEffectiveModel 获取实际应该使用的模型名
// 如果存在映射（精确或通配符），返回映射后的模型名；否则返回原模型名
func (p *Provider) GetEffectiveModel(requestedModel string) string {
	detail := p.resolveModelMappingDetail(requestedModel)
	if detail.Matched {
		return detail.MappedModel
	}
	return requestedModel
}

type providerModelMappingDetail struct {
	MappedModel     string
	Pattern         string
	TargetPattern   string
	ReasoningEffort string
	Supports1M      bool
	Matched         bool
}

func (p *Provider) resolveModelMapping(requestedModel string) (string, bool) {
	detail := p.resolveModelMappingDetail(requestedModel)
	return detail.MappedModel, detail.Matched
}

func (p *Provider) resolveModelMappingDetail(requestedModel string) providerModelMappingDetail {
	if p == nil || len(p.ModelMapping) == 0 {
		return providerModelMappingDetail{MappedModel: requestedModel}
	}

	// 优先查找精确映射
	if mappedModel, exists := p.ModelMapping[requestedModel]; exists && p.isModelMappingEnabled(requestedModel) &&
		(requestedModel != claudeManagedSubagentModel || strings.TrimSpace(mappedModel) != "") {
		return providerModelMappingDetail{
			MappedModel:     mappedModel,
			Pattern:         requestedModel,
			TargetPattern:   mappedModel,
			ReasoningEffort: strings.TrimSpace(p.ModelMappingReasoningEfforts[requestedModel]),
			Supports1M:      p.ModelMappingSupports1M[requestedModel],
			Matched:         true,
		}
	}

	// 内部 Subagent 别名仅允许命中专用配置或默认兜底，避免被普通通配符意外转发。
	if requestedModel == claudeManagedSubagentModel {
		if mappedModel, exists := p.ModelMapping[claudeDefaultModelMappingKey]; exists &&
			strings.TrimSpace(mappedModel) != "" && p.isModelMappingEnabled(claudeDefaultModelMappingKey) {
			return providerModelMappingDetail{
				MappedModel:     mappedModel,
				Pattern:         claudeDefaultModelMappingKey,
				TargetPattern:   mappedModel,
				ReasoningEffort: strings.TrimSpace(p.ModelMappingReasoningEfforts[claudeDefaultModelMappingKey]),
				Supports1M:      p.ModelMappingSupports1M[claudeDefaultModelMappingKey],
				Matched:         true,
			}
		}
		return providerModelMappingDetail{MappedModel: requestedModel}
	}

	// 查找通配符映射
	type wildcardMapping struct {
		pattern     string
		replacement string
		literalSize int
	}
	matches := make([]wildcardMapping, 0)
	for pattern, replacement := range p.ModelMapping {
		if !p.isModelMappingEnabled(pattern) {
			continue
		}
		if strings.Count(pattern, "*") != 1 || strings.Count(replacement, "*") > 1 || !matchWildcard(pattern, requestedModel) {
			continue
		}
		matches = append(matches, wildcardMapping{
			pattern:     pattern,
			replacement: replacement,
			literalSize: len(strings.ReplaceAll(pattern, "*", "")),
		})
	}
	if len(matches) == 0 {
		// 无映射，返回原模型名
		return providerModelMappingDetail{MappedModel: requestedModel}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].literalSize != matches[j].literalSize {
			return matches[i].literalSize > matches[j].literalSize
		}
		return matches[i].pattern < matches[j].pattern
	})
	selected := matches[0]
	return providerModelMappingDetail{
		MappedModel:     applyWildcardMapping(selected.pattern, selected.replacement, requestedModel),
		Pattern:         selected.pattern,
		TargetPattern:   selected.replacement,
		ReasoningEffort: strings.TrimSpace(p.ModelMappingReasoningEfforts[selected.pattern]),
		Supports1M:      p.ModelMappingSupports1M[selected.pattern],
		Matched:         true,
	}
}

func (p *Provider) supportsManagedClaudeSubagentModel() bool {
	if p == nil {
		return false
	}
	detail := p.resolveModelMappingDetail(claudeManagedSubagentModel)
	return detail.Matched &&
		(detail.Pattern == claudeManagedSubagentModel || detail.Pattern == claudeDefaultModelMappingKey) &&
		strings.TrimSpace(detail.MappedModel) != ""
}

func claudeProvidersRequireManagedSubagentModel(providers []Provider) bool {
	for i := range providers {
		if providers[i].Enabled && providers[i].supportsManagedClaudeSubagentModel() {
			return true
		}
	}
	return false
}

func normalizeModelMappingDisabled(disabled map[string]bool, modelMapping map[string]string) map[string]bool {
	if len(disabled) == 0 || len(modelMapping) == 0 {
		return nil
	}
	normalized := make(map[string]bool)
	for key, isDisabled := range disabled {
		if !isDisabled {
			continue
		}
		if _, exists := modelMapping[key]; exists {
			normalized[key] = true
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeModelMappingSupports1M(supports1M map[string]bool, modelMapping map[string]string) map[string]bool {
	if len(supports1M) == 0 || len(modelMapping) == 0 {
		return nil
	}
	normalized := make(map[string]bool)
	for key, enabled := range supports1M {
		if enabled {
			if _, exists := modelMapping[key]; exists {
				normalized[key] = true
			}
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func (p *Provider) isModelMappingEnabled(key string) bool {
	return p == nil || !p.ModelMappingDisabled[key]
}

func (p *Provider) activeModelMappingCount() int {
	if p == nil {
		return 0
	}
	count := 0
	for key := range p.ModelMapping {
		if p.isModelMappingEnabled(key) {
			count++
		}
	}
	return count
}

func normalizeModelMappingMissPolicy(policy string) string {
	switch strings.TrimSpace(strings.ToLower(policy)) {
	case ModelMappingMissPolicyPassthrough:
		return ModelMappingMissPolicyPassthrough
	default:
		return ModelMappingMissPolicyBlock
	}
}

func (p *Provider) shouldPassthroughModelMappingMiss() bool {
	return len(p.ModelMapping) > 0 &&
		normalizeModelMappingMissPolicy(p.ModelMappingMissPolicy) == ModelMappingMissPolicyPassthrough
}

func (p *Provider) hasModelMappingForModel(modelName string) bool {
	_, matched := p.resolveModelMapping(modelName)
	return matched
}

// GetEffectiveEndpoint 获取有效的 API 端点
// 优先使用用户配置的端点，否则使用平台默认
func (p *Provider) GetEffectiveEndpoint(defaultEndpoint string) string {
	ep := strings.TrimSpace(p.APIEndpoint)
	if ep == "" {
		return defaultEndpoint
	}

	// 校验：必须是相对路径，不能是完整 URL
	if strings.HasPrefix(ep, "http://") || strings.HasPrefix(ep, "https://") {
		log.Printf("[Provider] 警告: apiEndpoint 应该是相对路径（如 /v1/chat/completions），而非完整 URL: %s，使用默认端点", ep)
		return defaultEndpoint
	}

	// 确保以 / 开头
	if !strings.HasPrefix(ep, "/") {
		ep = "/" + ep
	}

	return ep
}

// ValidateConfiguration 验证 provider 的模型配置
// 返回验证错误列表（空则表示验证通过）
func (p *Provider) ValidateConfiguration() []string {
	return p.validateConfiguration(true)
}

func (p *Provider) validateConfigurationForKind(kind string) []string {
	return p.validateConfiguration(!strings.EqualFold(strings.TrimSpace(kind), "claude"))
}

func (p *Provider) validateConfiguration(validateMappingTargets bool) []string {
	errors := make([]string, 0)

	// 规则 1：ModelMapping 的 value 必须在 SupportedModels 中
	// 仅当两者都有实际内容时才校验（空 map 不触发校验）
	if validateMappingTargets && len(p.ModelMapping) > 0 && len(p.SupportedModels) > 0 {
		for externalModel, internalModel := range p.ModelMapping {
			if !p.isModelMappingEnabled(externalModel) {
				continue
			}
			// 检查是否为通配符映射
			if strings.Contains(internalModel, "*") {
				// 通配符映射暂不验证（需要具体请求才能展开）
				continue
			}

			// 精确映射需要验证
			supported := false
			if p.SupportedModels[internalModel] {
				supported = true
			} else {
				// 检查通配符白名单
				for supportedPattern := range p.SupportedModels {
					if matchWildcard(supportedPattern, internalModel) {
						supported = true
						break
					}
				}
			}

			if !supported {
				errors = append(errors, fmt.Sprintf(
					"模型映射无效：'%s' -> '%s'，目标模型 '%s' 不在 supportedModels 中",
					externalModel, internalModel, internalModel,
				))
			}
		}
	}

	// 允许仅配置 modelMapping（无 supportedModels 时不阻塞保存）
	// 用户可能只想映射模型名，不需要白名单过滤

	// 规则 3 移除：自映射不会破坏功能，最多是无效配置，不阻塞保存

	p.configErrors = errors
	return errors
}

// matchWildcard 通配符匹配函数
// 支持 * 通配符，如 "claude-*" 匹配 "claude-sonnet-4"
func matchWildcard(pattern, text string) bool {
	// 如果没有通配符，使用精确匹配
	if !strings.Contains(pattern, "*") {
		return pattern == text
	}

	// 简化实现：只支持单个 * 通配符
	parts := strings.Split(pattern, "*")
	if len(parts) == 2 {
		// 前缀 + * 或 * + 后缀
		prefix, suffix := parts[0], parts[1]
		return strings.HasPrefix(text, prefix) && strings.HasSuffix(text, suffix)
	}

	// 多个 * 的情况（更复杂，暂不支持）
	return false
}

// applyWildcardMapping 应用通配符映射
// 将 pattern 中的 * 匹配部分替换到 replacement 的 * 位置
// 示例: pattern="claude-*", replacement="anthropic/claude-*", input="claude-sonnet-4"
//
//	输出: "anthropic/claude-sonnet-4"
func applyWildcardMapping(pattern, replacement, input string) string {
	// 如果 pattern 或 replacement 没有通配符，直接返回 replacement
	if !strings.Contains(pattern, "*") || !strings.Contains(replacement, "*") {
		return replacement
	}

	// 提取通配符匹配的部分
	parts := strings.Split(pattern, "*")
	if len(parts) != 2 {
		return replacement // 不支持多个通配符
	}

	prefix, suffix := parts[0], parts[1]

	// 验证 input 确实匹配 pattern
	if !strings.HasPrefix(input, prefix) || !strings.HasSuffix(input, suffix) {
		return replacement
	}

	// 提取中间部分
	wildcardPart := input[len(prefix) : len(input)-len(suffix)]

	// 替换 replacement 中的 *
	return strings.Replace(replacement, "*", wildcardPart, 1)
}

// EnsureCodexOAuthProvider 确保 ChatGPT Codex OAuth 登录后有一个可管理的 provider。
func (ps *ProviderService) EnsureCodexOAuthProvider(accountID string, login string) (*Provider, error) {
	if ps == nil {
		return nil, fmt.Errorf("provider service 未初始化")
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, fmt.Errorf("accountID 不能为空")
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	providers, err := ps.loadProvidersNoLock("codex")
	if err != nil {
		return nil, err
	}
	providers = filterPersistentProviders("codex", providers)

	for i := range providers {
		if !isCodexOAuthProvider(providers[i]) {
			continue
		}
		if strings.TrimSpace(providers[i].AuthAccountID) != "" && strings.TrimSpace(providers[i].AuthAccountID) != accountID {
			continue
		}
		normalizeCodexOAuthProvider(&providers[i], accountID, login)
		providers[i].Enabled = false
		if err := ps.saveProvidersLocked("codex", providers); err != nil {
			return nil, err
		}
		provider := providers[i]
		return &provider, nil
	}

	maxID := codexOAuthDefaultProviderID - 1
	for _, provider := range providers {
		if provider.ID > maxID {
			maxID = provider.ID
		}
	}
	provider := Provider{
		ID:            maxID + 1,
		Name:          codexOAuthProviderDisplayName(login),
		APIURL:        codexOAuthBackendAPIBaseURL,
		APIKey:        "",
		Site:          "https://chatgpt.com/codex",
		Icon:          "openai",
		Tint:          "rgba(16, 185, 129, 0.16)",
		Accent:        "#10b981",
		Enabled:       false,
		Category:      "official",
		AuthProvider:  CodexOAuthProviderName,
		AuthAccountID: accountID,
		Level:         1,
	}
	providers = append(providers, provider)
	if err := ps.saveProvidersLocked("codex", providers); err != nil {
		return nil, err
	}
	return &provider, nil
}

// SelectCodexOAuthProvider 将指定 ChatGPT 账号对应的 OAuth provider 设为唯一启用项。
func (ps *ProviderService) SelectCodexOAuthProvider(accountID string, login string) (*Provider, error) {
	if ps == nil {
		return nil, fmt.Errorf("provider service 未初始化")
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, fmt.Errorf("accountID 不能为空")
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	providers, err := ps.loadProvidersNoLock("codex")
	if err != nil {
		return nil, err
	}
	providers = filterPersistentProviders("codex", providers)

	selectedIndex := -1
	maxID := codexOAuthDefaultProviderID - 1
	for i := range providers {
		if providers[i].ID > maxID {
			maxID = providers[i].ID
		}
		if !isCodexOAuthProvider(providers[i]) {
			continue
		}
		boundAccountID := strings.TrimSpace(providers[i].AuthAccountID)
		if boundAccountID == accountID {
			if selectedIndex >= 0 {
				providers[selectedIndex].Enabled = false
			}
			selectedIndex = i
			continue
		}
		if boundAccountID == "" && selectedIndex == -1 {
			selectedIndex = i
			continue
		}
		providers[i].Enabled = false
	}

	if selectedIndex >= 0 {
		normalizeCodexOAuthProvider(&providers[selectedIndex], accountID, login)
		providers[selectedIndex].Enabled = true
		if err := ps.saveProvidersLocked("codex", providers); err != nil {
			return nil, err
		}
		provider := providers[selectedIndex]
		return &provider, nil
	}

	provider := Provider{
		ID:            maxID + 1,
		Name:          codexOAuthProviderDisplayName(login),
		APIURL:        codexOAuthBackendAPIBaseURL,
		APIKey:        "",
		Site:          "https://chatgpt.com/codex",
		Icon:          "openai",
		Tint:          "rgba(16, 185, 129, 0.16)",
		Accent:        "#10b981",
		Enabled:       true,
		Category:      "official",
		AuthProvider:  CodexOAuthProviderName,
		AuthAccountID: accountID,
		Level:         1,
	}
	providers = append(providers, provider)
	if err := ps.saveProvidersLocked("codex", providers); err != nil {
		return nil, err
	}
	return &provider, nil
}

// DisableCodexOAuthProviders 禁用已失效账号绑定的 Codex OAuth provider。
func (ps *ProviderService) DisableCodexOAuthProviders(accountID string) error {
	if ps == nil {
		return nil
	}
	accountID = strings.TrimSpace(accountID)
	ps.mu.Lock()
	defer ps.mu.Unlock()
	providers, err := ps.loadProvidersNoLock("codex")
	if err != nil {
		return err
	}
	changed := false
	for i := range providers {
		if !isCodexOAuthProvider(providers[i]) {
			continue
		}
		if accountID != "" && strings.TrimSpace(providers[i].AuthAccountID) != accountID {
			continue
		}
		if providers[i].Enabled {
			providers[i].Enabled = false
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return ps.saveProvidersLocked("codex", providers)
}

func normalizeCodexOAuthProvider(provider *Provider, accountID string, login string) {
	provider.AuthAccountID = accountID
	provider.AuthProvider = CodexOAuthProviderName
	provider.APIURL = codexOAuthBackendAPIBaseURL
	provider.APIKey = ""
	provider.Site = "https://chatgpt.com/codex"
	if strings.TrimSpace(provider.Name) == "" {
		provider.Name = codexOAuthProviderDisplayName(login)
	}
	if strings.TrimSpace(provider.Icon) == "" {
		provider.Icon = "openai"
	}
	if strings.TrimSpace(provider.Tint) == "" {
		provider.Tint = "rgba(16, 185, 129, 0.16)"
	}
	if strings.TrimSpace(provider.Accent) == "" {
		provider.Accent = "#10b981"
	}
	if provider.Level <= 0 {
		provider.Level = 1
	}
}
