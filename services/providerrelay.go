package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	modelpricing "codeswitch/resources/model-pricing"

	"github.com/daodao97/xgo/xdb"
	"github.com/daodao97/xgo/xrequest"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// LastUsedProvider 最后使用的供应商信息
// @author sm
type LastUsedProvider struct {
	Platform     string `json:"platform"`      // claude/codex/gemini
	ProviderID   string `json:"provider_id"`   // 供应商 ID（字符串，兼容 int64/string）
	ProviderName string `json:"provider_name"` // 供应商名称
	UpdatedAt    int64  `json:"updated_at"`    // 更新时间（毫秒）
}

type ProviderRelayService struct {
	providerService     *ProviderService
	geminiService       *GeminiService
	blacklistService    *BlacklistService
	notificationService *NotificationService
	appSettings         *AppSettingsService // 应用设置服务（用于获取轮询开关状态）
	modelPricing        *ModelPricingService
	server              *http.Server
	addr                string
	lastUsed            map[string]*LastUsedProvider // 各平台最后使用的供应商
	lastUsedMu          sync.RWMutex                 // 保护 lastUsed 的锁
	rrMu                sync.Mutex                   // 轮询状态锁
	rrLastStart         map[string]string            // 轮询状态：key="platform:level" → value=上次起始 Provider ID（回退为 Name）
	claudeResponsesMu   sync.Mutex
	claudeResponses     map[string]claudeResponsesSessionBinding
}

// errClientAbort 表示客户端中断连接，不应计入 provider 失败次数
var errClientAbort = errors.New("client aborted, skip failure count")
var errResponseStarted = errors.New("response already started")
var errIncompleteStream = errors.New("stream ended before completion")

type responseStartedError struct {
	cause error
}

func (e *responseStartedError) Error() string {
	if e == nil {
		return errResponseStarted.Error()
	}
	if e.cause != nil {
		return e.cause.Error()
	}
	return errResponseStarted.Error()
}

func (e *responseStartedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (e *responseStartedError) Is(target error) bool {
	return target == errResponseStarted
}

func markResponseStarted(err error) error {
	return &responseStartedError{cause: err}
}

var (
	responseModelRegex                     = regexp.MustCompile(`"model"\s*:\s*"([^"]+)"`)
	responseModelVersionRegex              = regexp.MustCompile(`"modelVersion"\s*:\s*"([^"]+)"`)
	claudeMetadataLegacyUserIDRegex        = regexp.MustCompile(`^user_([a-fA-F0-9]{64})_account_([a-fA-F0-9-]*)_session_([a-fA-F0-9-]{36})$`)
	requestLogSensitiveJSONValuePattern    = regexp.MustCompile(`(?i)("(?:api[_-]?key|x-api-key|x-goog-api-key|authorization|auth[_-]?token|access[_-]?token|refresh[_-]?token|password|secret)"\s*:\s*)"[^"]*"`)
	requestLogAuthorizationBearerPattern   = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*bearer\s+)[^\s",]+`)
	requestLogSensitiveQueryValuePattern   = regexp.MustCompile(`(?i)((?:api[_-]?key|x-api-key|x-goog-api-key|auth[_-]?token|access[_-]?token)=)[^&\s]+`)
	requestLogSensitiveKeywordQuickPattern = regexp.MustCompile(`(?i)(api[_-]?key|x-api-key|x-goog-api-key|authorization|auth[_-]?token|access[_-]?token|refresh[_-]?token|password|secret)`)
)

var (
	openAICompatPromptCacheDisabledMu sync.Mutex
	openAICompatPromptCacheDisabled   = make(map[string]time.Time)
)

const requestLogPayloadMaxBytes = 8 * 1024 * 1024
const requestLogPayloadRedactedValue = "[REDACTED]"
const claudeResponsesSessionTTL = 30 * time.Minute
const claudeResponsesMaxSessionBindings = 4096
const claudeResponsesTailReplayMaxInputItems = 80
const openAICompatPromptCacheDisableTTL = 30 * time.Minute

type claudeResponsesContinuationRejection int

const (
	claudeResponsesContinuationRejectionNone claudeResponsesContinuationRejection = iota
	claudeResponsesContinuationRejectionNotFound
	claudeResponsesContinuationRejectionUnsupported
)

type claudeResponsesSessionBinding struct {
	ResponseID string
	Disabled   bool
	ExpiresAt  time.Time
}

// upstreamErrorResponse 保存上游非 2xx 响应，供“最终失败”场景透传给客户端
type upstreamErrorResponse struct {
	statusCode  int
	contentType string
	headers     http.Header
	body        []byte
}

func newUpstreamErrorResponse(statusCode int, contentType string, headers http.Header, body []byte) *upstreamErrorResponse {
	bodyCopy := make([]byte, len(body))
	copy(bodyCopy, body)

	headerCopy := make(http.Header, len(headers))
	for key, values := range headers {
		copied := make([]string, len(values))
		copy(copied, values)
		headerCopy[key] = copied
	}

	return &upstreamErrorResponse{
		statusCode:  statusCode,
		contentType: contentType,
		headers:     headerCopy,
		body:        bodyCopy,
	}
}

func (e *upstreamErrorResponse) Error() string {
	if e == nil {
		return "upstream error: <nil>"
	}
	trimmed := strings.TrimSpace(string(e.body))
	if trimmed != "" {
		return fmt.Sprintf("upstream status %d: %s", e.statusCode, truncateText(trimmed, 2048))
	}
	return fmt.Sprintf("upstream status %d", e.statusCode)
}

func isHopByHopHeader(headerName string) bool {
	switch strings.ToLower(strings.TrimSpace(headerName)) {
	case "connection",
		"keep-alive",
		"proxy-authenticate",
		"proxy-authorization",
		"te",
		"trailer",
		"transfer-encoding",
		"upgrade",
		"content-length":
		return true
	default:
		return false
	}
}

func (e *upstreamErrorResponse) WriteTo(c *gin.Context) {
	if e == nil || c == nil {
		return
	}

	// 复制上游响应头（过滤 hop-by-hop 头），尽量保持原样透传
	dstHeaders := c.Writer.Header()
	for key, values := range e.headers {
		if isHopByHopHeader(key) {
			continue
		}
		dstHeaders.Del(key)
		for _, value := range values {
			dstHeaders.Add(key, value)
		}
	}

	// 若上游 Header 缺失 Content-Type，尝试使用记录值补全；否则保持原样
	if strings.TrimSpace(dstHeaders.Get("Content-Type")) == "" {
		contentType := strings.TrimSpace(e.contentType)
		if contentType != "" {
			dstHeaders.Set("Content-Type", contentType)
		}
	}

	c.Status(e.statusCode)
	if len(e.body) == 0 {
		return
	}
	_, _ = c.Writer.Write(e.body)
}

// writeLastUpstreamErrorIfAny 在“所有重试与降级均失败”时透传最后一次上游错误
func writeLastUpstreamErrorIfAny(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	var upstreamErr *upstreamErrorResponse
	if !errors.As(err, &upstreamErr) {
		return false
	}
	upstreamErr.WriteTo(c)
	return true
}

func NewProviderRelayService(providerService *ProviderService, geminiService *GeminiService, blacklistService *BlacklistService, notificationService *NotificationService, appSettings *AppSettingsService, modelPricing *ModelPricingService, addr string) *ProviderRelayService {
	if addr == "" {
		addr = "127.0.0.1:18100" // 【安全修复】仅监听本地回环地址，防止 API Key 暴露到局域网
	}

	// 【修复】数据库初始化已移至 main.go 的 InitDatabase()
	// 此处不再调用 xdb.Inits()、ensureRequestLogTable()、ensureBlacklistTables()

	return &ProviderRelayService{
		providerService:     providerService,
		geminiService:       geminiService,
		blacklistService:    blacklistService,
		notificationService: notificationService,
		appSettings:         appSettings,
		modelPricing:        modelPricing,
		addr:                addr,
		lastUsed: map[string]*LastUsedProvider{
			"claude": nil,
			"codex":  nil,
			"gemini": nil,
		},
		rrLastStart:     make(map[string]string),
		claudeResponses: make(map[string]claudeResponsesSessionBinding),
	}
}

func providerRefFromProvider(provider Provider) string {
	return providerRefFromNumericID(provider.ID, provider.Name)
}

func providerRefFromGeminiProvider(provider GeminiProvider) string {
	return providerRefFromStringID(provider.ID, provider.Name)
}

// setLastUsedProvider 记录最后使用的供应商
// @author sm
func (prs *ProviderRelayService) setLastUsedProvider(platform, providerID, providerName string) {
	prs.lastUsedMu.Lock()
	previous := prs.lastUsed[platform]
	changed := previous == nil || previous.ProviderID != providerID || previous.ProviderName != providerName
	prs.lastUsed[platform] = &LastUsedProvider{
		Platform:     platform,
		ProviderID:   providerID,
		ProviderName: providerName,
		UpdatedAt:    time.Now().UnixMilli(),
	}
	prs.lastUsedMu.Unlock()

	if changed && prs.notificationService != nil {
		go prs.notificationService.EmitProviderRouted(platform, providerID, providerName)
	}
}

// GetLastUsedProvider 获取指定平台最后使用的供应商
// @author sm
func (prs *ProviderRelayService) GetLastUsedProvider(platform string) *LastUsedProvider {
	prs.lastUsedMu.RLock()
	defer prs.lastUsedMu.RUnlock()
	return prs.lastUsed[platform]
}

// GetAllLastUsedProviders 获取所有平台最后使用的供应商
// @author sm
func (prs *ProviderRelayService) GetAllLastUsedProviders() map[string]*LastUsedProvider {
	prs.lastUsedMu.RLock()
	defer prs.lastUsedMu.RUnlock()
	result := make(map[string]*LastUsedProvider)
	for k, v := range prs.lastUsed {
		result[k] = v
	}
	return result
}

// isRoundRobinEnabled 检查轮询功能是否启用
// 条件：1. 应用设置开关启用 2. 拉黑模式关闭（Fixed Mode 跳过轮询）
func (prs *ProviderRelayService) isRoundRobinEnabled() bool {
	// 检查拉黑模式是否启用（Fixed Mode 优先级高于轮询）
	if prs.blacklistService.ShouldUseFixedMode() {
		return false
	}

	// 检查应用设置开关
	if prs.appSettings == nil {
		return false
	}
	settings, err := prs.appSettings.GetAppSettings()
	if err != nil {
		return false
	}
	return settings.EnableRoundRobin
}

// isRequestLogPayloadSanitizationEnabled 检查 request_log payload 脱敏是否启用。
// 默认开启，优先保证敏感信息不明文落库。
func (prs *ProviderRelayService) isRequestLogPayloadSanitizationEnabled() bool {
	_, sanitizeEnabled := prs.resolveRequestLogPayloadCaptureAndSanitization()
	return sanitizeEnabled
}

// resolveRequestLogPayloadCaptureAndSanitization 一次性读取 payload 相关配置，
// 避免在单次请求里重复读取 app settings 文件。
func (prs *ProviderRelayService) resolveRequestLogPayloadCaptureAndSanitization() (captureEnabled bool, sanitizeEnabled bool) {
	captureEnabled = false
	sanitizeEnabled = true
	if prs.appSettings == nil {
		return
	}
	settings, err := prs.appSettings.GetAppSettings()
	if err != nil {
		return
	}
	captureEnabled = settings.CaptureRequestLogPayload
	sanitizeEnabled = settings.SanitizeRequestLogPayload
	return
}

// roundRobinOrder 对同 Level 的 providers 进行轮询排序
// 算法：基于 provider ID（回退 name）追踪，将上次起始 provider 移到末尾，实现轮询效果
// 参数：
//   - platform: 平台标识（claude/codex/gemini/custom:xxx）
//   - level: 当前 Level
//   - providers: 同 Level 的 providers 列表（已过滤、按用户排序）
//
// 返回：轮询排序后的 providers 列表（新切片，不修改原切片）
func (prs *ProviderRelayService) roundRobinOrder(platform string, level int, providers []Provider) []Provider {
	if len(providers) <= 1 {
		return providers
	}

	// 构建 key: "platform:level"
	key := fmt.Sprintf("%s:%d", platform, level)

	prs.rrMu.Lock()
	defer prs.rrMu.Unlock()

	lastStart := prs.rrLastStart[key]

	// 记录本次起始 provider 标识（更新状态）
	prs.rrLastStart[key] = providerRefFromProvider(providers[0])

	// 如果没有历史记录，返回原顺序
	if lastStart == "" {
		return providers
	}

	// 查找上次起始 provider 在当前列表中的位置
	lastIdx := -1
	for i, p := range providers {
		if providerRefFromProvider(p) == lastStart {
			lastIdx = i
			break
		}
	}

	// 上次起始 provider 不在当前列表（可能被禁用/黑名单），返回原顺序
	if lastIdx == -1 {
		return providers
	}

	// 构建轮询顺序：从 lastIdx+1 开始，环形遍历
	result := make([]Provider, len(providers))
	for i := 0; i < len(providers); i++ {
		idx := (lastIdx + 1 + i) % len(providers)
		result[i] = providers[idx]
	}

	// 更新本次起始 provider 标识
	prs.rrLastStart[key] = providerRefFromProvider(result[0])

	return result
}

// roundRobinOrderGemini 对 Gemini providers 进行轮询排序（复用相同逻辑）
func (prs *ProviderRelayService) roundRobinOrderGemini(level int, providers []GeminiProvider) []GeminiProvider {
	if len(providers) <= 1 {
		return providers
	}

	// 构建 key: "gemini:level"
	key := fmt.Sprintf("gemini:%d", level)

	prs.rrMu.Lock()
	defer prs.rrMu.Unlock()

	lastStart := prs.rrLastStart[key]

	// 记录本次起始 provider 标识
	prs.rrLastStart[key] = providerRefFromGeminiProvider(providers[0])

	// 如果没有历史记录，返回原顺序
	if lastStart == "" {
		return providers
	}

	// 查找上次起始 provider 在当前列表中的位置
	lastIdx := -1
	for i, p := range providers {
		if providerRefFromGeminiProvider(p) == lastStart {
			lastIdx = i
			break
		}
	}

	// 上次起始 provider 不在当前列表，返回原顺序
	if lastIdx == -1 {
		return providers
	}

	// 构建轮询顺序
	result := make([]GeminiProvider, len(providers))
	for i := 0; i < len(providers); i++ {
		idx := (lastIdx + 1 + i) % len(providers)
		result[i] = providers[idx]
	}

	// 更新本次起始 provider 标识
	prs.rrLastStart[key] = providerRefFromGeminiProvider(result[0])

	return result
}

func (prs *ProviderRelayService) Start() error {
	// 启动前验证配置
	if warnings := prs.validateConfig(); len(warnings) > 0 {
		fmt.Println("======== Provider 配置验证警告 ========")
		for _, warn := range warnings {
			fmt.Printf("⚠️  %s\n", warn)
		}
		fmt.Println("========================================")
	}

	if gin.Mode() == gin.DebugMode {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()
	prs.registerRoutes(router)

	prs.server = &http.Server{
		Addr:              prs.addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      30 * time.Minute,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	fmt.Printf("provider relay server listening on %s\n", prs.addr)

	go func() {
		if err := prs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("provider relay server error: %v\n", err)
		}
	}()
	return nil
}

// validateConfig 验证所有 provider 的配置
// 返回警告列表（非阻塞性错误）
func (prs *ProviderRelayService) validateConfig() []string {
	warnings := make([]string, 0)

	for _, kind := range []string{"claude", "codex"} {
		providers, err := prs.providerService.LoadProviders(kind)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("[%s] 加载配置失败: %v", kind, err))
			continue
		}

		enabledCount := 0
		for _, p := range providers {
			if !p.Enabled {
				continue
			}
			enabledCount++

			// 验证每个启用的 provider
			if errs := p.ValidateConfiguration(); len(errs) > 0 {
				for _, errMsg := range errs {
					warnings = append(warnings, fmt.Sprintf("[%s/%s] %s", kind, p.Name, errMsg))
				}
			}

			// 检查是否配置了模型白名单或映射
			if (p.SupportedModels == nil || len(p.SupportedModels) == 0) &&
				(p.ModelMapping == nil || len(p.ModelMapping) == 0) {
				warnings = append(warnings, fmt.Sprintf(
					"[%s/%s] 未配置 supportedModels 或 modelMapping，将假设支持所有模型（可能导致降级失败）",
					kind, p.Name))
			}

			// 检查是否只配置了映射但没有白名单
			if len(p.ModelMapping) > 0 && len(p.SupportedModels) == 0 {
				warnings = append(warnings, fmt.Sprintf(
					"[%s/%s] 配置了 modelMapping 但未配置 supportedModels，映射目标将不做校验，请确认目标模型在供应商处可用",
					kind, p.Name))
			}
		}

		if enabledCount == 0 {
			warnings = append(warnings, fmt.Sprintf("[%s] 没有启用的 provider", kind))
		}
	}

	return warnings
}

func (prs *ProviderRelayService) Stop() error {
	if prs.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return prs.server.Shutdown(ctx)
}

func (prs *ProviderRelayService) Addr() string {
	return prs.addr
}

func (prs *ProviderRelayService) registerRoutes(router gin.IRouter) {
	router.GET("/debug/memory", func(c *gin.Context) {
		if !isLoopbackClientIP(c.ClientIP()) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		c.JSON(http.StatusOK, gin.H{
			"alloc_mb":      stats.Alloc / 1024 / 1024,
			"heap_alloc_mb": stats.HeapAlloc / 1024 / 1024,
			"heap_idle_mb":  stats.HeapIdle / 1024 / 1024,
			"heap_sys_mb":   stats.HeapSys / 1024 / 1024,
			"sys_mb":        stats.Sys / 1024 / 1024,
			"num_gc":        stats.NumGC,
			"goroutines":    runtime.NumGoroutine(),
		})
	})

	router.POST("/v1/messages", prs.proxyHandler("claude", "/v1/messages"))
	router.POST("/responses", prs.proxyHandler("codex", "/responses"))

	// /v1/models 端点（OpenAI-compatible API）
	// 支持 Claude 和 Codex 平台
	router.GET("/v1/models", prs.modelsHandler("claude"))

	// Gemini API 端点（使用专门的路径前缀避免与 Claude 冲突）
	router.POST("/gemini/v1beta/*any", prs.geminiProxyHandler("/v1beta"))
	router.POST("/gemini/v1/*any", prs.geminiProxyHandler("/v1"))

	// 自定义 CLI 工具端点（路由格式: /custom/:toolId/v1/messages）
	// toolId 用于区分不同的 CLI 工具，对应 provider kind 为 "custom:{toolId}"
	router.POST("/custom/:toolId/v1/messages", prs.customCliProxyHandler())

	// 自定义 CLI 工具的 /v1/models 端点
	router.GET("/custom/:toolId/v1/models", prs.customModelsHandler())
}

func isLoopbackClientIP(raw string) bool {
	ip := net.ParseIP(strings.TrimSpace(raw))
	return ip != nil && ip.IsLoopback()
}

func (prs *ProviderRelayService) proxyHandler(kind string, endpoint string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var bodyBytes []byte
		if c.Request.Body != nil {
			data, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
				return
			}
			bodyBytes = data
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		isStream := gjson.GetBytes(bodyBytes, "stream").Bool()
		requestedModel := gjson.GetBytes(bodyBytes, "model").String()

		// 如果未指定模型，记录警告但不拦截
		if requestedModel == "" {
			fmt.Printf("[WARN] 请求未指定模型名，无法执行模型智能降级\n")
		}

		providers, err := prs.providerService.LoadProviders(kind)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load providers"})
			return
		}

		active := make([]Provider, 0, len(providers))
		requestPlans := make(map[string]providerRequestPlan, len(providers))
		skippedCount := 0
		for _, provider := range providers {
			// 基础过滤：enabled、URL、APIKey
			if !provider.Enabled || provider.APIURL == "" || provider.APIKey == "" {
				continue
			}

			// 配置验证：失败则自动跳过
			if errs := provider.ValidateConfiguration(); len(errs) > 0 {
				fmt.Printf("[WARN] Provider %s 配置验证失败，已自动跳过: %v\n", provider.Name, errs)
				skippedCount++
				continue
			}

			// 黑名单检查：跳过已拉黑的 provider
			if isBlacklisted, until := prs.blacklistService.IsBlacklistedByID(kind, providerRefFromProvider(provider), provider.Name); isBlacklisted {
				fmt.Printf("⛔ Provider %s 已拉黑，过期时间: %v\n", provider.Name, until.Format("15:04:05"))
				skippedCount++
				continue
			}

			plan, err := prs.buildProviderRequestPlan(provider, bodyBytes, endpoint, requestedModel)
			if err != nil {
				fmt.Printf("[WARN] Provider %s 请求体预处理失败，已自动跳过: %v\n", provider.Name, err)
				skippedCount++
				continue
			}

			if !provider.IsResolvedModelSupported(requestedModel, plan.EffectiveModel) {
				fmt.Printf("[INFO] Provider %s 不支持最终模型 %s（原始请求模型: %s），已跳过\n",
					provider.Name,
					displayModelForLog(plan.EffectiveModel),
					displayModelForLog(requestedModel),
				)
				skippedCount++
				continue
			}

			requestPlans[providerRefFromProvider(provider)] = plan
			active = append(active, provider)
		}

		if len(active) == 0 {
			if requestedModel != "" {
				c.JSON(http.StatusNotFound, gin.H{
					"error": fmt.Sprintf("没有可用的 provider 支持模型 '%s'（已跳过 %d 个不兼容的 provider）", requestedModel, skippedCount),
				})
			} else {
				c.JSON(http.StatusNotFound, gin.H{"error": "no providers available"})
			}
			return
		}

		fmt.Printf("[INFO] 找到 %d 个可用的 provider（已过滤 %d 个）：", len(active), skippedCount)
		for _, p := range active {
			fmt.Printf("%s ", p.Name)
		}
		fmt.Println()

		// 按 Level 分组
		levelGroups := make(map[int][]Provider)
		for _, provider := range active {
			level := provider.Level
			if level <= 0 {
				level = 1 // 未配置或零值时默认为 Level 1
			}
			levelGroups[level] = append(levelGroups[level], provider)
		}

		// 获取所有 level 并升序排序
		levels := make([]int, 0, len(levelGroups))
		for level := range levelGroups {
			levels = append(levels, level)
		}
		sort.Ints(levels)

		fmt.Printf("[INFO] 共 %d 个 Level 分组：%v\n", len(levels), levels)

		query := flattenQuery(c.Request.URL.Query())
		clientHeaders := cloneHeaders(c.Request.Header)

		// 获取拉黑功能开关状态
		blacklistEnabled := prs.blacklistService.ShouldUseFixedMode()

		// 【拉黑模式】：同 Provider 重试直到被拉黑，然后切换到下一个 Provider
		// 设计目标：Claude Code 单次请求最多重试 3 次，但拉黑阈值可能是 5
		// 通过内部重试机制，在单次请求中累积足够失败次数触发拉黑
		if blacklistEnabled {
			fmt.Printf("[INFO] 🔒 拉黑模式已开启（同 Provider 重试到拉黑再切换）\n")

			// 获取重试配置
			retryConfig := prs.blacklistService.GetRetryConfig()
			maxRetryPerProvider := retryConfig.FailureThreshold
			retryWaitSeconds := retryConfig.RetryWaitSeconds
			fmt.Printf("[INFO] 重试配置: 每 Provider 最多 %d 次重试，间隔 %d 秒\n",
				maxRetryPerProvider, retryWaitSeconds)

			var lastError error
			var lastProvider string
			totalAttempts := 0

			// 遍历所有 Level 和 Provider
			for _, level := range levels {
				providersInLevel := levelGroups[level]
				fmt.Printf("[INFO] === 尝试 Level %d（%d 个 provider）===\n", level, len(providersInLevel))

				for _, provider := range providersInLevel {
					// 检查是否已被拉黑（跳过已拉黑的 provider）
					if blacklisted, until := prs.blacklistService.IsBlacklistedByID(kind, providerRefFromProvider(provider), provider.Name); blacklisted {
						fmt.Printf("[INFO] ⏭️ 跳过已拉黑的 Provider: %s (解禁时间: %v)\n", provider.Name, until)
						continue
					}

					plan, err := prs.getProviderRequestPlan(requestPlans, provider, bodyBytes, endpoint, requestedModel)
					if err != nil {
						fmt.Printf("[ERROR] Provider %s 请求体预处理失败: %v，跳过此 Provider\n", provider.Name, err)
						continue
					}

					// 同 Provider 内重试循环
					for retryCount := 0; retryCount < maxRetryPerProvider; retryCount++ {
						totalAttempts++

						// 再次检查是否已被拉黑（重试过程中可能被拉黑）
						if blacklisted, _ := prs.blacklistService.IsBlacklistedByID(kind, providerRefFromProvider(provider), provider.Name); blacklisted {
							fmt.Printf("[INFO] 🚫 Provider %s 已被拉黑，切换到下一个\n", provider.Name)
							break
						}

						fmt.Printf("[INFO] [拉黑模式] Provider: %s (Level %d) | 重试 %d/%d | Model: %s\n",
							provider.Name, level, retryCount+1, maxRetryPerProvider, plan.EffectiveModel)

						startTime := time.Now()
						ok, err := prs.forwardRequestWithPlan(c, kind, provider, plan.EffectiveEndpoint, query, clientHeaders, plan.BodyBytes, isStream, plan.EffectiveModel, requestedModel, plan)
						duration := time.Since(startTime)

						if ok {
							fmt.Printf("[INFO] ✓ 成功: %s | 重试 %d 次 | 耗时: %.2fs\n",
								provider.Name, retryCount+1, duration.Seconds())
							if err := prs.blacklistService.RecordSuccessByID(kind, providerRefFromProvider(provider), provider.Name); err != nil {
								fmt.Printf("[WARN] 清零失败计数失败: %v\n", err)
							}
							prs.setLastUsedProvider(kind, providerRefFromProvider(provider), provider.Name)
							return
						}

						// 失败处理
						lastError = err
						lastProvider = provider.Name

						errorMsg := "未知错误"
						if err != nil {
							errorMsg = err.Error()
						}
						if errors.Is(err, errResponseStarted) {
							fmt.Printf("[WARN] ⚠️ 响应已部分写入，无法重试: %s | 错误: %s | 耗时: %.2fs\n",
								provider.Name, errorMsg, duration.Seconds())
							if errors.Is(err, errClientAbort) {
								fmt.Printf("[INFO] 客户端中断，停止重试\n")
								return
							}
							if err := prs.blacklistService.RecordFailureByID(kind, providerRefFromProvider(provider), provider.Name); err != nil {
								fmt.Printf("[ERROR] 记录失败到黑名单失败: %v\n", err)
							}
							return
						}
						fmt.Printf("[WARN] ✗ 失败: %s | 重试 %d/%d | 错误: %s | 耗时: %.2fs\n",
							provider.Name, retryCount+1, maxRetryPerProvider, errorMsg, duration.Seconds())

						// 客户端中断不计入失败次数，直接返回
						if errors.Is(err, errClientAbort) {
							fmt.Printf("[INFO] 客户端中断，停止重试\n")
							return
						}

						// 记录失败次数（可能触发拉黑）
						if err := prs.blacklistService.RecordFailureByID(kind, providerRefFromProvider(provider), provider.Name); err != nil {
							fmt.Printf("[ERROR] 记录失败到黑名单失败: %v\n", err)
						}

						// 检查是否刚被拉黑
						if blacklisted, _ := prs.blacklistService.IsBlacklistedByID(kind, providerRefFromProvider(provider), provider.Name); blacklisted {
							fmt.Printf("[INFO] 🚫 Provider %s 达到失败阈值，已被拉黑，切换到下一个\n", provider.Name)
							break
						}

						// 等待后重试（除非是最后一次）
						if retryCount < maxRetryPerProvider-1 {
							fmt.Printf("[INFO] ⏳ 等待 %d 秒后重试...\n", retryWaitSeconds)
							time.Sleep(time.Duration(retryWaitSeconds) * time.Second)
						}
					}
				}
			}

			// 所有 Provider 都失败或被拉黑
			fmt.Printf("[ERROR] 💥 拉黑模式：所有 Provider 都失败或被拉黑（共尝试 %d 次）\n", totalAttempts)

			// 按用户要求：仅在所有重试/降级都失败后，透传最后一次上游错误
			if writeLastUpstreamErrorIfAny(c, lastError) {
				return
			}

			errorMsg := "未知错误"
			if lastError != nil {
				errorMsg = lastError.Error()
			}
			c.JSON(http.StatusBadGateway, gin.H{
				"error":         fmt.Sprintf("所有 Provider 都失败或被拉黑，最后尝试: %s - %s", lastProvider, errorMsg),
				"lastProvider":  lastProvider,
				"totalAttempts": totalAttempts,
				"mode":          "blacklist_retry",
				"hint":          "拉黑模式已开启，同 Provider 重试到拉黑再切换。如需立即降级请关闭拉黑功能",
			})
			return
		}

		// 【降级模式】：拉黑功能关闭，失败自动尝试下一个 provider
		roundRobinEnabled := prs.isRoundRobinEnabled()
		if roundRobinEnabled {
			fmt.Printf("[INFO] 🔄 降级模式 + 轮询负载均衡\n")
		} else {
			fmt.Printf("[INFO] 🔄 降级模式（顺序降级）\n")
		}

		var lastError error
		var lastProvider string
		var lastDuration time.Duration
		totalAttempts := 0

		for _, level := range levels {
			providersInLevel := levelGroups[level]

			// 如果启用轮询，对同 Level 的 providers 进行轮询排序
			if roundRobinEnabled {
				providersInLevel = prs.roundRobinOrder(kind, level, providersInLevel)
			}

			fmt.Printf("[INFO] === 尝试 Level %d（%d 个 provider）===\n", level, len(providersInLevel))

			for i, provider := range providersInLevel {
				totalAttempts++

				plan, err := prs.getProviderRequestPlan(requestPlans, provider, bodyBytes, endpoint, requestedModel)
				if err != nil {
					fmt.Printf("[ERROR] Provider %s 请求体预处理失败: %v\n", provider.Name, err)
					continue
				}

				fmt.Printf("[INFO]   [%d/%d] Provider: %s | Model: %s\n", i+1, len(providersInLevel), provider.Name, plan.EffectiveModel)

				// 尝试发送请求
				startTime := time.Now()
				ok, err := prs.forwardRequestWithPlan(c, kind, provider, plan.EffectiveEndpoint, query, clientHeaders, plan.BodyBytes, isStream, plan.EffectiveModel, requestedModel, plan)
				duration := time.Since(startTime)

				if ok {
					fmt.Printf("[INFO]   ✓ Level %d 成功: %s | 耗时: %.2fs\n", level, provider.Name, duration.Seconds())

					// 成功：清零连续失败计数
					if err := prs.blacklistService.RecordSuccessByID(kind, providerRefFromProvider(provider), provider.Name); err != nil {
						fmt.Printf("[WARN] 清零失败计数失败: %v\n", err)
					}

					// 记录最后使用的供应商
					prs.setLastUsedProvider(kind, providerRefFromProvider(provider), provider.Name)

					return // 成功，立即返回
				}

				// 失败：记录错误并尝试下一个
				lastError = err
				lastProvider = provider.Name
				lastDuration = duration

				errorMsg := "未知错误"
				if err != nil {
					errorMsg = err.Error()
				}
				if errors.Is(err, errResponseStarted) {
					fmt.Printf("[WARN]   ⚠️ 响应已部分写入，无法降级: %s | 错误: %s | 耗时: %.2fs\n",
						provider.Name, errorMsg, duration.Seconds())
					if errors.Is(err, errClientAbort) {
						fmt.Printf("[INFO] 客户端中断，跳过失败计数: %s\n", provider.Name)
						return
					}
					if err := prs.blacklistService.RecordFailureByID(kind, providerRefFromProvider(provider), provider.Name); err != nil {
						fmt.Printf("[ERROR] 记录失败到黑名单失败: %v\n", err)
					}
					return
				}
				fmt.Printf("[WARN]   ✗ Level %d 失败: %s | 错误: %s | 耗时: %.2fs\n",
					level, provider.Name, errorMsg, duration.Seconds())

				// 客户端中断不计入失败次数
				if errors.Is(err, errClientAbort) {
					fmt.Printf("[INFO] 客户端中断，跳过失败计数: %s\n", provider.Name)
				} else if err := prs.blacklistService.RecordFailureByID(kind, providerRefFromProvider(provider), provider.Name); err != nil {
					fmt.Printf("[ERROR] 记录失败到黑名单失败: %v\n", err)
				}

				// 发送切换通知：检查是否有下一个可用的 provider
				if prs.notificationService != nil {
					nextProviderName := ""
					nextProviderID := ""
					// 先查找同级别的下一个
					if i+1 < len(providersInLevel) {
						nextProvider := providersInLevel[i+1]
						nextProviderName = nextProvider.Name
						nextProviderID = providerRefFromProvider(nextProvider)
					} else {
						// 查找下一个 level 的第一个 provider
						for _, nextLevel := range levels {
							if nextLevel > level && len(levelGroups[nextLevel]) > 0 {
								nextProvider := levelGroups[nextLevel][0]
								nextProviderName = nextProvider.Name
								nextProviderID = providerRefFromProvider(nextProvider)
								break
							}
						}
					}
					if nextProviderName != "" {
						prs.notificationService.NotifyProviderSwitch(SwitchNotification{
							FromProviderID: providerRefFromProvider(provider),
							FromProvider:   provider.Name,
							ToProviderID:   nextProviderID,
							ToProvider:     nextProviderName,
							Reason:         errorMsg,
							Platform:       kind,
						})
					}
				}
			}

			fmt.Printf("[WARN] Level %d 的所有 %d 个 provider 均失败，尝试下一 Level\n", level, len(providersInLevel))
		}

		// 所有 provider 都失败：优先透传最后一次上游错误，否则返回 502 聚合信息
		if writeLastUpstreamErrorIfAny(c, lastError) {
			return
		}

		errorMsg := "未知错误"
		if lastError != nil {
			errorMsg = lastError.Error()
		}
		fmt.Printf("[ERROR] 所有 %d 个 provider 均失败，最后尝试: %s | 错误: %s\n",
			totalAttempts, lastProvider, errorMsg)

		c.JSON(http.StatusBadGateway, gin.H{
			"error":          fmt.Sprintf("所有 %d 个 provider 均失败，最后错误: %s", totalAttempts, errorMsg),
			"last_provider":  lastProvider,
			"last_duration":  fmt.Sprintf("%.2fs", lastDuration.Seconds()),
			"total_attempts": totalAttempts,
		})
	}
}

func (prs *ProviderRelayService) forwardRequest(
	c *gin.Context,
	kind string,
	provider Provider,
	endpoint string,
	query map[string]string,
	clientHeaders map[string]string,
	bodyBytes []byte,
	isStream bool,
	model string,
	requestedModel string,
) (bool, error) {
	return prs.forwardRequestWithPlan(c, kind, provider, endpoint, query, clientHeaders, bodyBytes, isStream, model, requestedModel, providerRequestPlan{
		BodyBytes:         bodyBytes,
		EffectiveModel:    model,
		EffectiveEndpoint: endpoint,
	})
}

func (prs *ProviderRelayService) forwardRequestWithPlan(
	c *gin.Context,
	kind string,
	provider Provider,
	endpoint string,
	query map[string]string,
	clientHeaders map[string]string,
	bodyBytes []byte,
	isStream bool,
	model string,
	requestedModel string,
	plan providerRequestPlan,
) (bool, error) {
	if kind == "claude" &&
		resolveClaudeAPIFormat(provider) == claudeAPIFormatOpenAIResponse &&
		strings.TrimSpace(plan.ContinuationSessionKey) != "" &&
		strings.TrimSpace(plan.PreviousResponseID) != "" &&
		prs.isClaudeResponsesContinuationDisabled(provider, plan.ContinuationSessionKey) {
		if len(plan.ContinuationRetryBodyBytes) > 0 {
			bodyBytes = plan.ContinuationRetryBodyBytes
			plan.BodyBytes = plan.ContinuationRetryBodyBytes
		}
		plan.PreviousResponseID = ""
		plan.ContinuationRetryBodyBytes = nil
	}

	targetURL := joinURL(provider.APIURL, endpoint)
	headers := cloneMap(clientHeaders)

	// 根据认证方式设置请求头（默认 Bearer，与 v2.2.x 保持一致）
	authType := strings.ToLower(strings.TrimSpace(provider.ConnectivityAuthType))
	switch authType {
	case "x-api-key":
		// 仅当用户显式选择 x-api-key 时使用（Anthropic 官方 API）
		headers["x-api-key"] = provider.APIKey
		headers["anthropic-version"] = "2023-06-01"
	case "", "bearer":
		// 默认使用 Bearer token（兼容所有第三方中转）
		headers["Authorization"] = fmt.Sprintf("Bearer %s", provider.APIKey)
	default:
		// 自定义 Header 名
		headerName := strings.TrimSpace(provider.ConnectivityAuthType)
		if headerName == "" || strings.EqualFold(headerName, "custom") {
			headerName = "Authorization"
		}
		headers[headerName] = provider.APIKey
	}

	if _, ok := headers["Accept"]; !ok {
		headers["Accept"] = "application/json"
	}
	if kind == "claude" && resolveClaudeAPIFormat(provider) == claudeAPIFormatOpenAIResponse {
		setHeaderIfAbsentCaseInsensitive(headers, "openai-beta", "responses=experimental")
	}

	start := time.Now()
	capturePayloadEnabled, sanitizePayloadEnabled := prs.resolveRequestLogPayloadCaptureAndSanitization()
	requestLog := &ReqeustLog{
		Platform:         kind,
		ProviderID:       providerRefFromProvider(provider),
		Provider:         provider.Name,
		Model:            model,
		RequestedModel:   strings.TrimSpace(requestedModel),
		IsStream:         isStream,
		CapturePayload:   capturePayloadEnabled,
		SanitizePayload:  sanitizePayloadEnabled,
		RequestStartedAt: start,
		ProviderAPIURL:   provider.APIURL,
		ProviderAPIKey:   provider.APIKey,
		ProviderAuthType: provider.ConnectivityAuthType,
	}
	requestLog.streamCompletionRequired = kind == "claude" &&
		isStream &&
		resolveClaudeAPIFormat(provider) == claudeAPIFormatOpenAIResponse
	captureRequestLogRequestBody(requestLog, bodyBytes)
	pricingSnapshot := (*modelpricing.Service)(nil)
	if prs != nil && prs.modelPricing != nil {
		pricingSnapshot = prs.modelPricing.Service()
	}
	if pricingSnapshot == nil {
		if svc, err := modelpricing.DefaultService(); err == nil {
			pricingSnapshot = svc
		}
	}
	defer func() {
		requestLog.DurationSec = time.Since(start).Seconds()
		normalizeRequestLogCacheCreateTokens(requestLog)
		normalizeRequestLogInputTokens(requestLog)
		costResult := calculateRequestLogCost(
			prs.providerService,
			pricingSnapshot,
			requestLog.ProviderAPIURL,
			requestLog.ProviderAPIKey,
			requestLog.ProviderAuthType,
			requestLog.ResponseModel,
			requestLog.Model,
			requestLog.RequestedModel,
			requestLog.InputTokens,
			requestLog.OutputTokens,
			requestLog.ReasoningTokens,
			requestLog.CacheCreateTokens,
			requestLog.Ephemeral5mTokens,
			requestLog.Ephemeral1hTokens,
			requestLog.CacheReadTokens,
		)
		applyRequestLogCostResult(requestLog, costResult)
		prepareRequestLogPayloadForPersistence(requestLog)

		// 【修复】判空保护：避免队列未初始化时 panic
		if GlobalDBQueueLogs == nil {
			fmt.Printf("⚠️  写入 request_log 失败: 队列未初始化\n")
			return
		}

		// 使用批量队列写入 request_log（高频同构操作，批量提交）
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := GlobalDBQueueLogs.ExecBatchCtx(ctx, `
				INSERT INTO request_log (
					platform, model, requested_model, response_model, provider_id, provider, http_code,
					input_tokens, output_tokens, cache_create_tokens, ephemeral_5m_tokens, ephemeral_1h_tokens, cache_read_tokens,
					reasoning_tokens, is_stream, duration_sec, first_token_sec, total_cost, group_multiplier, price_source,
					input_cost, output_cost, reasoning_cost, cache_create_cost, cache_read_cost,
					ephemeral_5m_cost, ephemeral_1h_cost, has_pricing, matched_pricing_model,
					provider_pricing_available, provider_quota_type, provider_input_usd_per_m, provider_output_usd_per_m,
					provider_per_call_unified, provider_per_call_input, provider_per_call_output,
					provider_per_call_unified_set, provider_per_call_input_set, provider_per_call_output_set,
					request_body, response_body, request_body_truncated, response_body_truncated, payload_bytes, payload_captured
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`,
			requestLog.Platform,
			requestLog.Model,
			requestLog.RequestedModel,
			requestLog.ResponseModel,
			requestLog.ProviderID,
			requestLog.Provider,
			requestLog.HttpCode,
			requestLog.InputTokens,
			requestLog.OutputTokens,
			requestLog.CacheCreateTokens,
			requestLog.Ephemeral5mTokens,
			requestLog.Ephemeral1hTokens,
			requestLog.CacheReadTokens,
			requestLog.ReasoningTokens,
			boolToInt(requestLog.IsStream),
			requestLog.DurationSec,
			requestLog.FirstTokenSec,
			requestLog.TotalCost,
			requestLog.GroupMultiplier,
			requestLog.PriceSource,
			requestLog.InputCost,
			requestLog.OutputCost,
			requestLog.ReasoningCost,
			requestLog.CacheCreateCost,
			requestLog.CacheReadCost,
			requestLog.Ephemeral5mCost,
			requestLog.Ephemeral1hCost,
			boolToInt(requestLog.HasPricing),
			requestLog.MatchedPricingModel,
			boolToInt(requestLog.ProviderPricingAvailable),
			requestLog.ProviderQuotaType,
			requestLog.ProviderInputUSDPerM,
			requestLog.ProviderOutputUSDPerM,
			requestLog.ProviderPerCallUnified,
			requestLog.ProviderPerCallInput,
			requestLog.ProviderPerCallOutput,
			boolToInt(requestLog.ProviderPerCallUnifiedSet),
			boolToInt(requestLog.ProviderPerCallInputSet),
			boolToInt(requestLog.ProviderPerCallOutputSet),
			requestLog.RequestBody,
			requestLog.ResponseBody,
			boolToInt(requestLog.RequestBodyTruncated),
			boolToInt(requestLog.ResponseBodyTruncated),
			requestLog.PayloadBytes,
			boolToInt(requestLog.PayloadCaptured),
		)

		if err != nil {
			fmt.Printf("写入 request_log 失败: %v\n", err)
		}
	}()

	doForward := func(currentBody []byte, currentPlan providerRequestPlan) (bool, error, bool, bool) {
		req := xrequest.New().
			SetHeaders(headers).
			SetQueryParams(query).
			SetRetry(1, 500*time.Millisecond).
			SetTimeout(32 * time.Hour) // 32小时超时，适配超大型项目分析

		reqBody := bytes.NewReader(currentBody)
		req = req.SetBody(reqBody)

		resp, err := req.Post(targetURL)

		// 无论成功失败，先尝试记录 HttpCode
		if resp != nil {
			requestLog.HttpCode = resp.StatusCode()
		}

		status := 0
		if resp != nil {
			status = resp.StatusCode()
		}

		if err != nil {
			// resp 存在但 err != nil：可能是客户端中断，不计入失败
			if resp != nil && status == 0 {
				fmt.Printf("[INFO] Provider %s 响应存在但状态码为0，判定为客户端中断\n", provider.Name)
				return false, fmt.Errorf("%w: %v", errClientAbort, err), false, false
			}

			// xrequest 在 5xx 重试耗尽后会同时返回 resp 和 err。
			// 这里不能直接丢弃 resp，否则上游原始错误 body 会被吞掉，只剩一个笼统的 retry 错误。
			if resp == nil || status < http.StatusMultipleChoices {
				return false, err, false, false
			}
		}

		if resp == nil {
			return false, fmt.Errorf("empty response"), false, false
		}

		// 状态码为 0 且无错误：当作成功处理
		if status == 0 {
			fmt.Printf("[WARN] Provider %s 返回状态码 0，但无错误，当作成功处理\n", provider.Name)
			hooks := buildClaudeProviderResponseHooks(prs, c, kind, provider, currentPlan, isStream, requestLog)
			writtenBytes, copyErr := resp.ToHttpResponseWriter(c.Writer, hooks...)
			ok, forwardErr := finalizeForwardSuccess(c, kind, requestLog, writtenBytes, copyErr)
			return ok, forwardErr, false, false
		}

		if status >= http.StatusOK && status < http.StatusMultipleChoices {
			if !isStream && kind == "claude" && resolveClaudeAPIFormat(provider) == claudeAPIFormatOpenAIResponse {
				if protocolErr := newClaudeResponsesNonStreamTerminalErrorResponse(resp.Bytes(), resp.RawResponse); protocolErr != nil {
					requestLog.HttpCode = protocolErr.statusCode
					setRequestLogResponseBody(requestLog, protocolErr.body)
					return false, protocolErr, false, false
				}
			}
			hooks := buildClaudeProviderResponseHooks(prs, c, kind, provider, currentPlan, isStream, requestLog)
			writtenBytes, copyErr := resp.ToHttpResponseWriter(c.Writer, hooks...)
			ok, forwardErr := finalizeForwardSuccess(c, kind, requestLog, writtenBytes, copyErr)
			return ok, forwardErr, false, false
		}

		// 非 2xx：打印上游错误信息，便于在控制台追踪原因
		contentType := ""
		if resp.RawResponse != nil {
			contentType = resp.RawResponse.Header.Get("Content-Type")
		}
		upstreamBody := resp.Bytes()
		setRequestLogResponseBody(requestLog, upstreamBody)
		retryWithoutContinuation := false
		retryWithoutPromptCacheKey := false
		switch prs.classifyClaudeResponsesContinuationRejection(kind, provider, currentPlan, status, upstreamBody) {
		case claudeResponsesContinuationRejectionNotFound:
			prs.deleteClaudeResponsesPreviousResponseID(provider, currentPlan.ContinuationSessionKey)
			retryWithoutContinuation = claudeResponsesCanRetryWithoutContinuation(currentPlan)
		case claudeResponsesContinuationRejectionUnsupported:
			prs.disableClaudeResponsesContinuation(provider, currentPlan.ContinuationSessionKey)
			retryWithoutContinuation = claudeResponsesCanRetryWithoutContinuation(currentPlan)
		}
		if !retryWithoutContinuation && prs.isOpenAICompatPromptCacheKeyUnsupported(kind, provider, currentPlan, status, upstreamBody) {
			prs.disableOpenAICompatPromptCache(provider, currentPlan.ContinuationSessionKey)
			retryWithoutPromptCacheKey = true
		}
		body := strings.TrimSpace(string(upstreamBody))
		if body != "" {
			level := "ERROR"
			if status >= http.StatusMultipleChoices && status < http.StatusBadRequest {
				level = "WARN"
			}
			fmt.Printf("[%s] Upstream %s provider=%s status=%d url=%s content_type=%s\n%s\n",
				level,
				kind,
				provider.Name,
				status,
				targetURL,
				contentType,
				truncateText(body, 12*1024),
			)
		} else {
			fmt.Printf("[ERROR] Upstream %s provider=%s status=%d url=%s content_type=%s (empty body)\n",
				kind,
				provider.Name,
				status,
				targetURL,
				contentType,
			)
		}

		var upstreamHeaders http.Header
		if resp.RawResponse != nil && resp.RawResponse.Header != nil {
			upstreamHeaders = resp.RawResponse.Header
		}
		return false, newUpstreamErrorResponse(status, contentType, upstreamHeaders, upstreamBody), retryWithoutContinuation, retryWithoutPromptCacheKey
	}

	ok, err, retryWithoutContinuation, retryWithoutPromptCacheKey := doForward(bodyBytes, plan)
	if (!retryWithoutContinuation && !retryWithoutPromptCacheKey) || responseHasStarted(c) {
		return ok, err
	}
	retryPlan := plan
	retryMessage := ""
	if retryWithoutContinuation {
		retryPlan.BodyBytes = plan.ContinuationRetryBodyBytes
		retryPlan.PreviousResponseID = ""
		retryPlan.ContinuationRetryBodyBytes = nil
		retryMessage = "previous_response_id 失效"
	} else {
		retryPlan.BodyBytes = removeJSONFieldBytes(plan.BodyBytes, "prompt_cache_key")
		retryPlan.ContinuationRetryBodyBytes = removeJSONFieldBytes(plan.ContinuationRetryBodyBytes, "prompt_cache_key")
		retryPlan.PromptCacheKey = ""
		retryMessage = "prompt_cache_key 不兼容"
	}
	captureRequestLogRequestBody(requestLog, retryPlan.BodyBytes)
	requestLog.ResponseBody = ""
	requestLog.ResponseBodyTruncated = false
	requestLog.responseBodyBuffer = nil
	fmt.Printf("[INFO] Claude Responses %s，Provider %s 已回退请求重试一次\n", retryMessage, provider.Name)
	ok, err, _, _ = doForward(retryPlan.BodyBytes, retryPlan)
	return ok, err
}

func cloneHeaders(header http.Header) map[string]string {
	cloned := make(map[string]string, len(header))
	for key, values := range header {
		if len(values) > 0 {
			cloned[key] = values[len(values)-1]
		}
	}
	return cloned
}

func cloneMap(m map[string]string) map[string]string {
	cloned := make(map[string]string, len(m))
	for k, v := range m {
		cloned[k] = v
	}
	return cloned
}

func setHeaderIfAbsentCaseInsensitive(headers map[string]string, key string, value string) {
	if headers == nil {
		return
	}
	for existingKey := range headers {
		if strings.EqualFold(existingKey, key) {
			return
		}
	}
	headers[key] = value
}

func removeJSONFieldBytes(bodyBytes []byte, path string) []byte {
	if len(bodyBytes) == 0 || strings.TrimSpace(path) == "" || !gjson.GetBytes(bodyBytes, path).Exists() {
		return bodyBytes
	}
	updated, err := sjson.DeleteBytes(bodyBytes, path)
	if err != nil {
		return bodyBytes
	}
	return updated
}

func responseHasStarted(c *gin.Context) bool {
	return c != nil && c.Writer != nil && c.Writer.Written()
}

func buildClaudeProviderResponseHooks(
	prs *ProviderRelayService,
	c *gin.Context,
	kind string,
	provider Provider,
	plan providerRequestPlan,
	isStream bool,
	requestLog *ReqeustLog,
) []xrequest.ResponseHook {
	hooks := make([]xrequest.ResponseHook, 0, 4)
	if hook := prs.newClaudeResponsesSessionHook(kind, provider, plan, isStream); hook != nil {
		hooks = append(hooks, hook)
	}
	if requestLog != nil && requestLog.streamCompletionRequired {
		hooks = append(hooks, openAIResponsesStreamLifecycleHook(requestLog))
	}
	if kind == "claude" && claudeAPIFormatNeedsTransform(resolveClaudeAPIFormat(provider)) {
		hooks = append(hooks, newClaudeResponseTransformHook(resolveClaudeAPIFormat(provider), isStream))
	}
	hooks = append(hooks, ReqeustLogHook(c, kind, requestLog))
	return hooks
}

func openAIResponsesStreamLifecycleHook(reqLog *ReqeustLog) xrequest.ResponseHook {
	var pendingEventType string
	var pendingDataLines []string
	var rawJSONBuffer strings.Builder

	return func(data []byte) (bool, []byte) {
		if reqLog == nil || !reqLog.IsStream || !reqLog.streamCompletionRequired {
			return true, data
		}

		line := strings.TrimSpace(string(data))
		switch {
		case line == "":
			if len(pendingDataLines) > 0 {
				payload := combineOpenAIResponsesDataLines(pendingDataLines)
				updateOpenAIResponsesStreamLifecyclePayload(payload, pendingEventType, reqLog)
				pendingDataLines = nil
				pendingEventType = ""
			}
			return true, data
		case strings.HasPrefix(line, "event:"):
			pendingEventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if len(pendingDataLines) > 0 {
				pendingDataLines = append(pendingDataLines, payload)
				combinedPayload := combineOpenAIResponsesDataLines(pendingDataLines)
				if gjson.Valid(combinedPayload) {
					updateOpenAIResponsesStreamLifecyclePayload(combinedPayload, pendingEventType, reqLog)
					pendingDataLines = nil
					pendingEventType = ""
				}
				return true, data
			}
			if payload != "" && payload != "[DONE]" && !gjson.Valid(payload) {
				pendingDataLines = append(pendingDataLines, payload)
				return true, data
			}
			updateOpenAIResponsesStreamLifecyclePayload(payload, pendingEventType, reqLog)
			pendingEventType = ""
		default:
			rawJSONBuffer.WriteString(string(data))
			payload := strings.TrimSpace(rawJSONBuffer.String())
			if payload != "" && gjson.Valid(payload) {
				updateOpenAIResponsesStreamLifecyclePayload(payload, "", reqLog)
				rawJSONBuffer.Reset()
			}
		}

		return true, data
	}
}

func combineOpenAIResponsesDataLines(lines []string) string {
	payload := strings.TrimSpace(strings.Join(lines, "\n"))
	if payload == "" || gjson.Valid(payload) {
		return payload
	}
	compactPayload := strings.TrimSpace(strings.Join(lines, ""))
	if compactPayload == "" || gjson.Valid(compactPayload) {
		return compactPayload
	}
	return payload
}

func updateOpenAIResponsesStreamLifecyclePayload(payload string, eventType string, reqLog *ReqeustLog) {
	payload = strings.TrimSpace(payload)
	if payload == "" || payload == "[DONE]" || !gjson.Valid(payload) {
		return
	}
	eventType = strings.TrimSpace(eventType)
	if eventType != "" && strings.TrimSpace(gjson.Get(payload, "type").String()) == "" {
		if withType, err := sjson.Set(payload, "type", eventType); err == nil {
			payload = withType
		}
	}
	updateResponseModelFromPayload(payload, reqLog)
	updateFirstTokenFromPayload(payload, reqLog)
	updateStreamLifecycleFromPayload("claude", payload, reqLog)
}

func newClaudeResponsesNonStreamTerminalErrorResponse(upstreamBody []byte, rawResponse *http.Response) *upstreamErrorResponse {
	payload := strings.TrimSpace(string(upstreamBody))
	if payload == "" || !gjson.Valid(payload) {
		return nil
	}
	status := extractOpenAIResponsesResponseStatus(payload)
	if !isOpenAIResponsesTerminalFailureStatus(status) &&
		!(strings.EqualFold(status, "incomplete") && !openAIResponsesPayloadHasUsableOutput(payload)) {
		return nil
	}
	message := strings.TrimSpace(extractOpenAIResponsesStreamFailureMessage(payload))
	if message == "" {
		message = strings.TrimSpace(status)
	}
	if message == "" {
		message = "unknown terminal status"
	}
	errorBody, err := json.Marshal(map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    "api_error",
			"message": fmt.Sprintf("OpenAI Responses response %s: %s", status, message),
		},
	})
	if err != nil {
		errorBody = []byte(`{"type":"error","error":{"type":"api_error","message":"OpenAI Responses response failed"}}`)
	}

	headers := http.Header{}
	if rawResponse != nil && rawResponse.Header != nil {
		for key, values := range rawResponse.Header {
			copied := make([]string, len(values))
			copy(copied, values)
			headers[key] = copied
		}
	}
	headers.Set("Content-Type", "application/json")
	return newUpstreamErrorResponse(http.StatusBadGateway, "application/json", headers, errorBody)
}

func finalizeForwardSuccess(c *gin.Context, kind string, requestLog *ReqeustLog, writtenBytes int64, copyErr error) (bool, error) {
	if copyErr != nil {
		if isClientWriteAbortError(copyErr) {
			requestLog.HttpCode = 499
			clientErr := fmt.Errorf("%w: %v", errClientAbort, copyErr)
			if responseHasStarted(c) || writtenBytes > 0 {
				return false, markResponseStarted(clientErr)
			}
			return false, clientErr
		}
		requestLog.HttpCode = http.StatusBadGateway
		streamErr := fmt.Errorf("%w: %v", errIncompleteStream, copyErr)
		if responseHasStarted(c) || writtenBytes > 0 {
			return false, markResponseStarted(streamErr)
		}
		return false, streamErr
	}
	if err := validateStreamCompletion(kind, requestLog); err != nil {
		requestLog.HttpCode = http.StatusBadGateway
		if responseHasStarted(c) || writtenBytes > 0 {
			return false, markResponseStarted(err)
		}
		return false, err
	}
	// 2xx 只是“上游接受请求”，流式请求还要确认复制过程未中断且收到了协议级完成事件。
	return true, nil
}

func flattenQuery(values map[string][]string) map[string]string {
	query := make(map[string]string, len(values))
	for key, items := range values {
		if len(items) > 0 {
			query[key] = items[len(items)-1]
		}
	}
	return query
}

func joinURL(base string, endpoint string) string {
	base = strings.TrimSuffix(base, "/")
	endpoint = "/" + strings.TrimPrefix(endpoint, "/")
	return base + endpoint
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func truncateText(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	truncated := value[:maxBytes]
	// 避免把 UTF-8 字符截断成乱码（最多回退 3 字节，成本很小）
	for len(truncated) > 0 && !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated + "\n...(truncated)"
}

func truncateRequestLogPayload(payload string, maxBytes int) (string, bool) {
	if maxBytes <= 0 {
		return "", len(payload) > 0
	}
	if len(payload) <= maxBytes {
		return payload, false
	}
	clipped := payload[:maxBytes]
	for len(clipped) > 0 && !utf8.ValidString(clipped) {
		clipped = clipped[:len(clipped)-1]
	}
	return clipped, true
}

func sanitizeRequestLogPayload(payload string) string {
	if strings.TrimSpace(payload) == "" {
		return payload
	}
	// 快速短路：多数 payload 不含敏感键，避免每次都跑重正则。
	if !requestLogSensitiveKeywordQuickPattern.MatchString(payload) {
		return payload
	}
	sanitized := requestLogSensitiveJSONValuePattern.ReplaceAllString(payload, `${1}"`+requestLogPayloadRedactedValue+`"`)
	sanitized = requestLogAuthorizationBearerPattern.ReplaceAllString(sanitized, `${1}`+requestLogPayloadRedactedValue)
	sanitized = requestLogSensitiveQueryValuePattern.ReplaceAllString(sanitized, `${1}`+requestLogPayloadRedactedValue)
	return sanitized
}

func maybeSanitizeRequestLogPayload(reqLog *ReqeustLog, payload string) string {
	if reqLog == nil || !reqLog.SanitizePayload {
		return payload
	}
	return sanitizeRequestLogPayload(payload)
}

func trimInvalidUTF8Suffix(value []byte) []byte {
	trimmed := value
	for len(trimmed) > 0 && !utf8.Valid(trimmed) {
		trimmed = trimmed[:len(trimmed)-1]
	}
	return trimmed
}

func captureRequestLogRequestBody(reqLog *ReqeustLog, bodyBytes []byte) {
	if reqLog == nil || !reqLog.CapturePayload {
		return
	}
	payload, truncated := truncateRequestLogPayload(string(bodyBytes), requestLogPayloadMaxBytes)
	reqLog.RequestBody = maybeSanitizeRequestLogPayload(reqLog, payload)
	reqLog.RequestBodyTruncated = truncated
}

func setRequestLogResponseBody(reqLog *ReqeustLog, bodyBytes []byte) {
	if reqLog == nil || !reqLog.CapturePayload {
		return
	}
	payload, truncated := truncateRequestLogPayload(string(bodyBytes), requestLogPayloadMaxBytes)
	reqLog.ResponseBody = maybeSanitizeRequestLogPayload(reqLog, payload)
	reqLog.ResponseBodyTruncated = truncated
	reqLog.responseBodyBuffer = nil
}

func appendRequestLogResponseBody(reqLog *ReqeustLog, chunk []byte) {
	if reqLog == nil || !reqLog.CapturePayload || len(chunk) == 0 || reqLog.ResponseBodyTruncated {
		return
	}
	if reqLog.responseBodyBuffer == nil {
		initialCap := len(chunk)
		if initialCap < 1024 {
			initialCap = 1024
		}
		if initialCap > requestLogPayloadMaxBytes {
			initialCap = requestLogPayloadMaxBytes
		}
		reqLog.responseBodyBuffer = make([]byte, 0, initialCap)
	}
	remaining := requestLogPayloadMaxBytes - len(reqLog.responseBodyBuffer)
	if remaining <= 0 {
		reqLog.ResponseBodyTruncated = true
		return
	}
	if len(chunk) <= remaining {
		reqLog.responseBodyBuffer = append(reqLog.responseBodyBuffer, chunk...)
		return
	}
	reqLog.responseBodyBuffer = append(reqLog.responseBodyBuffer, chunk[:remaining]...)
	reqLog.ResponseBodyTruncated = true
}

func resetRequestLogResponseBody(reqLog *ReqeustLog) {
	if reqLog == nil {
		return
	}
	reqLog.ResponseBody = ""
	reqLog.ResponseBodyTruncated = false
	reqLog.responseBodyBuffer = nil
}

func materializeRequestLogResponseBody(reqLog *ReqeustLog) {
	if reqLog == nil {
		return
	}
	if len(reqLog.responseBodyBuffer) == 0 {
		reqLog.responseBodyBuffer = nil
		return
	}
	buffer := reqLog.responseBodyBuffer
	if reqLog.ResponseBodyTruncated {
		buffer = trimInvalidUTF8Suffix(buffer)
	}
	reqLog.ResponseBody = maybeSanitizeRequestLogPayload(reqLog, string(buffer))
	reqLog.responseBodyBuffer = nil
}

func prepareRequestLogPayloadForPersistence(reqLog *ReqeustLog) {
	if reqLog == nil {
		return
	}
	if !reqLog.CapturePayload {
		reqLog.RequestBody = ""
		reqLog.ResponseBody = ""
		reqLog.RequestBodyTruncated = false
		reqLog.ResponseBodyTruncated = false
		reqLog.PayloadBytes = 0
		reqLog.PayloadCaptured = false
		reqLog.responseBodyBuffer = nil
		return
	}
	reqLog.RequestBody = maybeSanitizeRequestLogPayload(reqLog, reqLog.RequestBody)
	materializeRequestLogResponseBody(reqLog)
	reqLog.PayloadBytes = int64(len([]byte(reqLog.RequestBody)) + len([]byte(reqLog.ResponseBody)))
	reqLog.PayloadCaptured = reqLog.RequestBody != "" || reqLog.ResponseBody != "" || reqLog.RequestBodyTruncated || reqLog.ResponseBodyTruncated
}

func backfillRequestLogPayloadMetricsWithDB(db *sql.DB) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec(`
		UPDATE request_log
		SET
			payload_bytes = COALESCE(LENGTH(CAST(request_body AS BLOB)), 0) + COALESCE(LENGTH(CAST(response_body AS BLOB)), 0),
			payload_captured = CASE
				WHEN request_body != '' OR response_body != '' OR request_body_truncated != 0 OR response_body_truncated != 0 THEN 1
				ELSE 0
			END
		WHERE
			(request_body != '' OR response_body != '' OR request_body_truncated != 0 OR response_body_truncated != 0)
			AND (payload_bytes = 0 OR payload_captured = 0)
	`)
	return err
}

func ensureRequestLogColumn(db *sql.DB, column string, definition string) error {
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('request_log') WHERE name = '%s'", column)
	var count int
	if err := db.QueryRow(query).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		alter := fmt.Sprintf("ALTER TABLE request_log ADD COLUMN %s %s", column, definition)
		if _, err := db.Exec(alter); err != nil {
			return err
		}
	}
	return nil
}

func ensureRequestLogIndex(db *sql.DB, name string, columns string) error {
	stmt := fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON request_log(%s)", name, columns)
	_, err := db.Exec(stmt)
	return err
}

func ensureRequestLogTable() error {
	db, err := xdb.DB("default")
	if err != nil {
		return err
	}
	return ensureRequestLogTableWithDB(db)
}

func ensureRequestLogTableWithDB(db *sql.DB) error {
	const createTableSQL = `CREATE TABLE IF NOT EXISTS request_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		platform TEXT,
		model TEXT,
		requested_model TEXT DEFAULT '',
		response_model TEXT DEFAULT '',
		provider_id TEXT DEFAULT '',
		provider TEXT,
		http_code INTEGER,
		input_tokens INTEGER,
		output_tokens INTEGER,
		cache_create_tokens INTEGER,
		ephemeral_5m_tokens INTEGER DEFAULT 0,
		ephemeral_1h_tokens INTEGER DEFAULT 0,
		cache_read_tokens INTEGER,
		reasoning_tokens INTEGER,
		is_stream INTEGER DEFAULT 0,
		duration_sec REAL DEFAULT 0,
		first_token_sec REAL DEFAULT 0,
		total_cost REAL DEFAULT 0,
		group_multiplier REAL DEFAULT 1,
		price_source TEXT DEFAULT '',
		input_cost REAL DEFAULT 0,
		output_cost REAL DEFAULT 0,
		reasoning_cost REAL DEFAULT 0,
		cache_create_cost REAL DEFAULT 0,
		cache_read_cost REAL DEFAULT 0,
		ephemeral_5m_cost REAL DEFAULT 0,
		ephemeral_1h_cost REAL DEFAULT 0,
		has_pricing INTEGER DEFAULT 0,
		matched_pricing_model TEXT DEFAULT '',
		provider_pricing_available INTEGER DEFAULT 0,
		provider_quota_type INTEGER DEFAULT -1,
		provider_input_usd_per_m REAL DEFAULT 0,
		provider_output_usd_per_m REAL DEFAULT 0,
			provider_per_call_unified REAL DEFAULT 0,
			provider_per_call_input REAL DEFAULT 0,
			provider_per_call_output REAL DEFAULT 0,
			provider_per_call_unified_set INTEGER DEFAULT 0,
			provider_per_call_input_set INTEGER DEFAULT 0,
			provider_per_call_output_set INTEGER DEFAULT 0,
			request_body TEXT DEFAULT '',
			response_body TEXT DEFAULT '',
			request_body_truncated INTEGER DEFAULT 0,
			response_body_truncated INTEGER DEFAULT 0,
			payload_bytes INTEGER DEFAULT 0,
			payload_captured INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`

	if _, err := db.Exec(createTableSQL); err != nil {
		return err
	}

	// 兼容旧库：历史版本可能残留了 request_log_stats_* 触发器，
	// 但统计表被删除/未创建，后续 ALTER TABLE request_log 会报
	// "error in trigger ... no such table"。先移除，最后统一重建。
	if err := dropRequestLogStatsInsertTriggersWithDB(db); err != nil {
		return err
	}

	if err := ensureRequestLogColumn(db, "created_at", "DATETIME DEFAULT CURRENT_TIMESTAMP"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "is_stream", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "duration_sec", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "first_token_sec", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "total_cost", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "group_multiplier", "REAL DEFAULT 1"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "price_source", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "requested_model", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "response_model", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "provider_id", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "input_cost", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "output_cost", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "reasoning_cost", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "cache_create_cost", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "cache_read_cost", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "ephemeral_5m_tokens", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "ephemeral_1h_tokens", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "ephemeral_5m_cost", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "ephemeral_1h_cost", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "has_pricing", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "matched_pricing_model", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "provider_pricing_available", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "provider_quota_type", "INTEGER DEFAULT -1"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "provider_input_usd_per_m", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "provider_output_usd_per_m", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "provider_per_call_unified", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "provider_per_call_input", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "provider_per_call_output", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "provider_per_call_unified_set", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "provider_per_call_input_set", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "provider_per_call_output_set", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "request_body", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "response_body", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "request_body_truncated", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "response_body_truncated", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "payload_bytes", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogColumn(db, "payload_captured", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := backfillRequestLogPayloadMetricsWithDB(db); err != nil {
		return err
	}
	if err := ensureRequestLogIndex(db, "idx_request_log_created_at", "created_at"); err != nil {
		return err
	}
	if err := ensureRequestLogIndex(db, "idx_request_log_platform_created_at", "platform, created_at"); err != nil {
		return err
	}
	if err := ensureRequestLogIndex(db, "idx_request_log_platform_provider_created_at", "platform, provider, created_at"); err != nil {
		return err
	}
	if err := ensureRequestLogIndex(db, "idx_request_log_platform_provider_id_created_at", "platform, provider_id, created_at"); err != nil {
		return err
	}

	if err := ensureRequestLogStatsStorageWithDB(db); err != nil {
		return err
	}
	if err := ensureRequestLogQuotaCycleStorageWithDB(db); err != nil {
		return err
	}

	return nil
}

func ReqeustLogHook(c *gin.Context, kind string, usage *ReqeustLog) func(data []byte) (bool, []byte) { // SSE 钩子：累计字节和解析 token 用量
	var sseRemainder strings.Builder
	var rawJSONBuffer strings.Builder

	return func(data []byte) (bool, []byte) {
		parserFn := ClaudeCodeParseTokenUsageFromResponse
		switch kind {
		case "codex":
			parserFn = CodexParseTokenUsageFromResponse
		case "gemini":
			parserFn = GeminiParseTokenUsageFromResponse
		}
		appendRequestLogResponseBody(usage, data)
		parseTokenUsageChunk(data, usage, parserFn, &sseRemainder, &rawJSONBuffer, kind)

		return true, data
	}
}

func parseTokenUsageChunk(
	chunk []byte,
	usage *ReqeustLog,
	parser func(string, *ReqeustLog),
	sseRemainder *strings.Builder,
	rawJSONBuffer *strings.Builder,
	kind string,
) {
	if usage == nil || parser == nil || len(chunk) == 0 {
		return
	}

	payload := string(chunk)
	if usage.IsStream {
		if sseRemainder != nil && (sseRemainder.Len() > 0 || looksLikeSSEPayload(payload)) {
			parseEventPayload(payload, parser, usage, sseRemainder, kind)
			return
		}
	}

	parseRawJSONPayload(payload, parser, usage, rawJSONBuffer, kind)
}

func parseEventPayload(payload string, parser func(string, *ReqeustLog), usage *ReqeustLog, remainder *strings.Builder, kind string) {
	if strings.TrimSpace(payload) == "" || parser == nil || usage == nil || remainder == nil {
		return
	}

	remainder.WriteString(payload)
	combined := remainder.String()
	if combined == "" {
		return
	}

	offset := 0
	for {
		newlineIdx := strings.IndexByte(combined[offset:], '\n')
		if newlineIdx < 0 {
			break
		}
		lineEnd := offset + newlineIdx
		line := combined[offset:lineEnd]
		parseSSEDataLine(line, parser, usage, kind)
		offset = lineEnd + 1
	}

	tail := combined[offset:]
	remainder.Reset()
	if strings.TrimSpace(tail) == "" {
		return
	}

	if shouldProcessStandaloneSSELine(tail) {
		parseSSEDataLine(tail, parser, usage, kind)
		return
	}

	remainder.WriteString(tail)
}

func parseSSEDataLine(line string, parser func(string, *ReqeustLog), usage *ReqeustLog, kind string) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "data:") {
		return
	}
	dataLine := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if dataLine == "" || dataLine == "[DONE]" || !gjson.Valid(dataLine) {
		return
	}
	parser(dataLine, usage)
	updateResponseModelFromPayload(dataLine, usage)
	updateFirstTokenFromPayload(dataLine, usage)
	updateStreamLifecycleFromPayload(kind, dataLine, usage)
}

func looksLikeSSEPayload(payload string) bool {
	trimmed := strings.TrimLeft(payload, " \t\r\n")
	return strings.HasPrefix(trimmed, "data:") ||
		strings.HasPrefix(trimmed, "event:") ||
		strings.HasPrefix(trimmed, "id:") ||
		strings.HasPrefix(trimmed, "retry:") ||
		strings.HasPrefix(trimmed, ":")
}

func shouldProcessStandaloneSSELine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}

	if strings.HasPrefix(trimmed, "data:") {
		dataLine := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		return dataLine == "" || dataLine == "[DONE]" || gjson.Valid(dataLine)
	}

	return strings.HasPrefix(trimmed, "event:") ||
		strings.HasPrefix(trimmed, "id:") ||
		strings.HasPrefix(trimmed, "retry:") ||
		strings.HasPrefix(trimmed, ":")
}

func updateResponseModelFromPayload(payload string, reqLog *ReqeustLog) {
	if reqLog == nil || strings.TrimSpace(reqLog.ResponseModel) != "" {
		return
	}
	if model := extractResponseModelFromPayload(payload); model != "" {
		reqLog.ResponseModel = model
	}
}

func extractResponseModelFromPayload(payload string) string {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" {
		return ""
	}

	for _, path := range []string{
		"model",
		"response.model",
		"message.model",
		"data.model",
		"modelVersion",
	} {
		if value := strings.TrimSpace(gjson.Get(trimmed, path).String()); value != "" {
			return value
		}
	}

	if matches := responseModelRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	if matches := responseModelVersionRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func parseRawJSONPayload(payload string, parser func(string, *ReqeustLog), usage *ReqeustLog, rawJSONBuffer *strings.Builder, kind string) {
	if parser == nil || usage == nil || rawJSONBuffer == nil || payload == "" {
		return
	}
	rawJSONBuffer.WriteString(payload)
	buffered := strings.TrimSpace(rawJSONBuffer.String())
	if buffered == "" || !gjson.Valid(buffered) {
		return
	}
	parser(buffered, usage)
	updateResponseModelFromPayload(buffered, usage)
	updateFirstTokenFromPayload(buffered, usage)
	updateStreamLifecycleFromPayload(kind, buffered, usage)
	rawJSONBuffer.Reset()
}

func updateFirstTokenFromPayload(payload string, reqLog *ReqeustLog) {
	if reqLog == nil || !reqLog.IsStream || reqLog.FirstTokenSec > 0 {
		return
	}
	if payloadHasGeneratedToken(payload) {
		markFirstTokenTimestamp(reqLog)
	}
}

func updateStreamLifecycleFromPayload(kind string, payload string, reqLog *ReqeustLog) {
	if reqLog == nil || !reqLog.IsStream {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(kind), "codex") && !reqLog.streamCompletionRequired {
		return
	}

	eventType, completed, failureMessage, terminal := detectOpenAIResponsesStreamTerminalState(payload)
	if !terminal {
		return
	}

	reqLog.streamTerminalEvent = eventType
	if completed {
		reqLog.streamFailureMessage = ""
		return
	}
	reqLog.streamFailureMessage = failureMessage
}

func detectCodexStreamTerminalState(payload string) (eventType string, completed bool, failureMessage string, terminal bool) {
	return detectOpenAIResponsesStreamTerminalState(payload)
}

func detectOpenAIResponsesStreamTerminalState(payload string) (eventType string, completed bool, failureMessage string, terminal bool) {
	eventType = strings.TrimSpace(gjson.Get(payload, "type").String())
	switch eventType {
	case "response.completed":
		return eventType, true, "", true
	case "response.done":
		switch extractOpenAIResponsesResponseStatus(payload) {
		case "failed":
			return "response.failed", false, extractOpenAIResponsesStreamFailureMessage(payload), true
		case "incomplete":
			return "response.incomplete", false, extractOpenAIResponsesStreamFailureMessage(payload), true
		case "cancelled", "canceled":
			return "response.cancelled", false, extractOpenAIResponsesStreamFailureMessage(payload), true
		default:
			return eventType, true, "", true
		}
	case "response.failed", "response.incomplete", "response.cancelled", "error":
		return eventType, false, extractOpenAIResponsesStreamFailureMessage(payload), true
	}

	status := extractOpenAIResponsesResponseStatus(payload)
	switch status {
	case "completed":
		return "response.completed", true, "", true
	case "failed":
		return "response.failed", false, extractOpenAIResponsesStreamFailureMessage(payload), true
	case "incomplete":
		return "response.incomplete", false, extractOpenAIResponsesStreamFailureMessage(payload), true
	case "cancelled", "canceled":
		return "response.cancelled", false, extractOpenAIResponsesStreamFailureMessage(payload), true
	default:
		return "", false, "", false
	}
}

func extractCodexResponseStatus(payload string) string {
	return extractOpenAIResponsesResponseStatus(payload)
}

func extractOpenAIResponsesResponseStatus(payload string) string {
	for _, path := range []string{"status", "response.status"} {
		if value := strings.TrimSpace(gjson.Get(payload, path).String()); value != "" {
			return strings.ToLower(value)
		}
	}
	return ""
}

func isOpenAIResponsesTerminalFailureStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func openAIResponsesPayloadHasUsableOutput(payload string) bool {
	output := gjson.Get(payload, "output")
	if !output.IsArray() {
		return false
	}
	for _, item := range output.Array() {
		switch strings.TrimSpace(item.Get("type").String()) {
		case "function_call":
			return true
		case "message":
			content := item.Get("content")
			if !content.IsArray() {
				continue
			}
			for _, block := range content.Array() {
				switch strings.TrimSpace(block.Get("type").String()) {
				case "output_text":
					if strings.TrimSpace(block.Get("text").String()) != "" {
						return true
					}
				case "refusal":
					if strings.TrimSpace(block.Get("refusal").String()) != "" {
						return true
					}
				}
			}
		case "reasoning":
			summary := item.Get("summary")
			if !summary.IsArray() {
				continue
			}
			for _, block := range summary.Array() {
				if strings.TrimSpace(block.Get("type").String()) == "summary_text" &&
					strings.TrimSpace(block.Get("text").String()) != "" {
					return true
				}
			}
		}
	}
	return false
}

func extractCodexStreamFailureMessage(payload string) string {
	return extractOpenAIResponsesStreamFailureMessage(payload)
}

func extractOpenAIResponsesStreamFailureMessage(payload string) string {
	for _, path := range []string{
		"status_details.error.message",
		"status_details.reason",
		"incomplete_details.reason",
		"error.message",
		"message",
		"response.status_details.error.message",
		"response.error.message",
		"response.status_details.reason",
		"response.incomplete_details.reason",
	} {
		if value := strings.TrimSpace(gjson.Get(payload, path).String()); value != "" {
			return value
		}
	}
	return ""
}

func validateStreamCompletion(kind string, reqLog *ReqeustLog) error {
	if reqLog == nil || !reqLog.IsStream {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(kind), "codex") && !reqLog.streamCompletionRequired {
		return nil
	}

	switch strings.TrimSpace(reqLog.streamTerminalEvent) {
	case "response.completed", "response.done":
		return nil
	case "":
		return fmt.Errorf("%w: missing response.completed event", errIncompleteStream)
	default:
		if msg := strings.TrimSpace(reqLog.streamFailureMessage); msg != "" {
			return fmt.Errorf("%w: %s (%s)", errIncompleteStream, reqLog.streamTerminalEvent, msg)
		}
		return fmt.Errorf("%w: %s", errIncompleteStream, reqLog.streamTerminalEvent)
	}
}

func isClientWriteAbortError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "error streaming response") ||
		strings.Contains(message, "error peeking response") ||
		strings.Contains(message, "error reading non-standard response") {
		return false
	}
	if !strings.Contains(message, "error writing response") &&
		!strings.Contains(message, "error writing final line") &&
		!strings.Contains(message, "error writing non-standard response") &&
		!strings.Contains(message, "error copying response") {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, net.ErrClosed) {
		return true
	}
	return strings.Contains(message, "broken pipe") ||
		strings.Contains(message, "connection reset by peer") ||
		strings.Contains(message, "use of closed network connection") ||
		strings.Contains(message, "context canceled") ||
		strings.Contains(message, "operation was canceled")
}

func markFirstTokenTimestamp(reqLog *ReqeustLog) {
	if reqLog == nil || reqLog.FirstTokenSec > 0 || reqLog.RequestStartedAt.IsZero() {
		return
	}
	elapsed := time.Since(reqLog.RequestStartedAt).Seconds()
	if elapsed <= 0 || math.IsNaN(elapsed) || math.IsInf(elapsed, 0) {
		return
	}
	reqLog.FirstTokenSec = elapsed
}

func payloadHasGeneratedToken(payload string) bool {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" || !gjson.Valid(trimmed) {
		return false
	}

	eventType := strings.TrimSpace(gjson.Get(trimmed, "type").String())
	if strings.EqualFold(eventType, "response.output_text.delta") {
		if delta := strings.TrimSpace(gjson.Get(trimmed, "delta").String()); delta != "" {
			return true
		}
	}

	for _, path := range []string{
		"delta.text",
		"delta.content",
		"content_block.delta.text",
		"content_block.text",
		"response.output_text",
		"response.output_text.delta",
		"output_text",
	} {
		if value := strings.TrimSpace(gjson.Get(trimmed, path).String()); value != "" {
			return true
		}
	}

	for _, choice := range gjson.Get(trimmed, "choices").Array() {
		if value := strings.TrimSpace(choice.Get("delta.content").String()); value != "" {
			return true
		}
		if value := strings.TrimSpace(choice.Get("text").String()); value != "" {
			return true
		}
	}

	for _, output := range gjson.Get(trimmed, "response.output").Array() {
		for _, content := range output.Get("content").Array() {
			if value := strings.TrimSpace(content.Get("text").String()); value != "" {
				return true
			}
			if value := strings.TrimSpace(content.Get("delta").String()); value != "" {
				return true
			}
		}
	}

	for _, block := range gjson.Get(trimmed, "content").Array() {
		if value := strings.TrimSpace(block.Get("text").String()); value != "" {
			return true
		}
	}

	for _, block := range gjson.Get(trimmed, "message.content").Array() {
		if value := strings.TrimSpace(block.Get("text").String()); value != "" {
			return true
		}
	}

	for _, candidate := range gjson.Get(trimmed, "candidates").Array() {
		for _, part := range candidate.Get("content.parts").Array() {
			if value := strings.TrimSpace(part.Get("text").String()); value != "" {
				return true
			}
		}
	}

	return false
}

// normalizeRequestLogInputTokens 将 input_tokens 规范化为“非缓存输入”。
//
// 口径说明：
// - Codex(OpenAI) 与 Gemini 的 usage 会把「缓存 tokens」计入 input_tokens，因此需要减去 cache_* tokens。
// - Claude(Anthropic) 的 input_tokens 本身就是“非缓存输入”，cache_* tokens 另给字段，无需减法。
//
// 本项目统一约定：
// - request_log.input_tokens 存“非缓存输入”（用于 input cost 与 UI「输入」展示）
// - request_log.cache_create_tokens / cache_read_tokens 单独存储（用于成本拆分与命中率）
//
// 不做这一步的话，会导致：
// 1) UI 看起来像“输入一直在累加历史对话”；
// 2) 成本计算把缓存 tokens 按 input + cache 两次计费。
func normalizeRequestLogInputTokens(reqLog *ReqeustLog) {
	if reqLog == nil {
		return
	}

	platform := strings.ToLower(strings.TrimSpace(reqLog.Platform))
	if platform != "codex" && platform != "gemini" {
		return
	}

	totalInput := reqLog.InputTokens
	if totalInput <= 0 {
		return
	}

	cacheCreate := reqLog.CacheCreateTokens
	cacheRead := reqLog.CacheReadTokens
	if cacheCreate < 0 {
		cacheCreate = 0
	}
	if cacheRead < 0 {
		cacheRead = 0
	}
	cacheTotal := cacheCreate + cacheRead
	if cacheTotal <= 0 {
		return
	}

	// 兜底：如果缓存 tokens 反而比 input_tokens 还大，说明上游口径不一致或解析异常，
	// 这里不做减法，避免记录成负数。
	if cacheTotal > totalInput {
		return
	}

	reqLog.InputTokens = totalInput - cacheTotal
}

func normalizeRequestLogCacheCreateTokens(reqLog *ReqeustLog) {
	if reqLog == nil {
		return
	}
	hasExplicitSplit := reqLog.Ephemeral5mTokens > 0 || reqLog.Ephemeral1hTokens > 0
	isClaudeLog := strings.EqualFold(strings.TrimSpace(reqLog.Platform), "claude") ||
		strings.Contains(strings.ToLower(strings.TrimSpace(reqLog.Model)), "claude")
	if !hasExplicitSplit && !isClaudeLog {
		return
	}
	normalizedTotal, normalized5m, normalized1h := normalizeCacheCreationTokenSplit(
		reqLog.CacheCreateTokens,
		reqLog.Ephemeral5mTokens,
		reqLog.Ephemeral1hTokens,
	)
	reqLog.CacheCreateTokens = normalizedTotal
	reqLog.Ephemeral5mTokens = normalized5m
	reqLog.Ephemeral1hTokens = normalized1h
}

func applyRequestLogCostResult(reqLog *ReqeustLog, result requestLogCostResult) {
	if reqLog == nil {
		return
	}
	reqLog.InputCost = result.InputCost
	reqLog.OutputCost = result.OutputCost
	reqLog.ReasoningCost = result.ReasoningCost
	reqLog.CacheCreateCost = result.CacheCreateCost
	reqLog.CacheReadCost = result.CacheReadCost
	reqLog.Ephemeral5mCost = result.Ephemeral5mCost
	reqLog.Ephemeral1hCost = result.Ephemeral1hCost
	reqLog.TotalCost = result.TotalCost
	reqLog.GroupMultiplier = result.GroupMultiplier
	reqLog.HasPricing = result.HasPricing
	reqLog.MatchedPricingModel = result.MatchedPricingModel
	reqLog.PriceSource = result.PriceSource
	reqLog.ProviderPricingAvailable = result.ProviderPricingAvailable
	reqLog.ProviderQuotaType = result.ProviderQuotaType
	reqLog.ProviderInputUSDPerM = result.ProviderInputUSDPerM
	reqLog.ProviderOutputUSDPerM = result.ProviderOutputUSDPerM
	reqLog.ProviderPerCallUnified = result.ProviderPerCallUnified
	reqLog.ProviderPerCallInput = result.ProviderPerCallInput
	reqLog.ProviderPerCallOutput = result.ProviderPerCallOutput
	reqLog.ProviderPerCallUnifiedSet = result.ProviderPerCallUnifiedSet
	reqLog.ProviderPerCallInputSet = result.ProviderPerCallInputSet
	reqLog.ProviderPerCallOutputSet = result.ProviderPerCallOutputSet
}

type ReqeustLog struct {
	ID                        int64   `json:"id"`
	Platform                  string  `json:"platform"` // claude、codex 或 gemini
	Model                     string  `json:"model"`
	RequestedModel            string  `json:"requested_model,omitempty"`
	ResponseModel             string  `json:"response_model,omitempty"`
	ProviderID                string  `json:"provider_id,omitempty"`
	Provider                  string  `json:"provider"` // provider name
	PriceSource               string  `json:"price_source,omitempty"`
	HttpCode                  int     `json:"http_code"`
	InputTokens               int     `json:"input_tokens"`
	OutputTokens              int     `json:"output_tokens"`
	CacheCreateTokens         int     `json:"cache_create_tokens"`
	Ephemeral5mTokens         int     `json:"ephemeral_5m_tokens"`
	Ephemeral1hTokens         int     `json:"ephemeral_1h_tokens"`
	CacheReadTokens           int     `json:"cache_read_tokens"`
	ReasoningTokens           int     `json:"reasoning_tokens"`
	IsStream                  bool    `json:"is_stream"`
	DurationSec               float64 `json:"duration_sec"`
	FirstTokenSec             float64 `json:"first_token_sec"`
	CreatedAt                 string  `json:"created_at"`
	InputCost                 float64 `json:"input_cost"`
	OutputCost                float64 `json:"output_cost"`
	ReasoningCost             float64 `json:"reasoning_cost"`
	CacheCreateCost           float64 `json:"cache_create_cost"`
	CacheReadCost             float64 `json:"cache_read_cost"`
	Ephemeral5mCost           float64 `json:"ephemeral_5m_cost"`
	Ephemeral1hCost           float64 `json:"ephemeral_1h_cost"`
	TotalCost                 float64 `json:"total_cost"`
	GroupMultiplier           float64 `json:"group_multiplier"`
	HasPricing                bool    `json:"has_pricing"`
	MatchedPricingModel       string  `json:"matched_pricing_model,omitempty"`
	ProviderPricingAvailable  bool    `json:"provider_pricing_available"`
	ProviderQuotaType         int     `json:"provider_quota_type"`
	ProviderInputUSDPerM      float64 `json:"provider_input_usd_per_m"`
	ProviderOutputUSDPerM     float64 `json:"provider_output_usd_per_m"`
	ProviderPerCallUnified    float64 `json:"provider_per_call_unified"`
	ProviderPerCallInput      float64 `json:"provider_per_call_input"`
	ProviderPerCallOutput     float64 `json:"provider_per_call_output"`
	ProviderPerCallUnifiedSet bool    `json:"provider_per_call_unified_set"`
	ProviderPerCallInputSet   bool    `json:"provider_per_call_input_set"`
	ProviderPerCallOutputSet  bool    `json:"provider_per_call_output_set"`
	RequestBody               string  `json:"request_body,omitempty"`
	ResponseBody              string  `json:"response_body,omitempty"`
	RequestBodyTruncated      bool    `json:"request_body_truncated"`
	ResponseBodyTruncated     bool    `json:"response_body_truncated"`
	PayloadBytes              int64   `json:"payload_bytes"`
	PayloadCaptured           bool    `json:"payload_captured"`

	CapturePayload  bool `json:"-"`
	SanitizePayload bool `json:"-"`

	ProviderAPIURL   string    `json:"-"`
	ProviderAPIKey   string    `json:"-"`
	ProviderAuthType string    `json:"-"`
	RequestStartedAt time.Time `json:"-"`

	responseBodyBuffer       []byte
	streamCompletionRequired bool
	streamTerminalEvent      string
	streamFailureMessage     string
}

// claude code usage parser
func ClaudeCodeParseTokenUsageFromResponse(data string, usage *ReqeustLog) {
	if usage == nil || strings.TrimSpace(data) == "" {
		return
	}

	input := firstPositiveIntFromPaths(
		data,
		"message.usage.input_tokens",
		"usage.input_tokens",
		"message.usage.prompt_tokens",
		"usage.prompt_tokens",
		"message.usage.prompt_token_count",
		"usage.prompt_token_count",
		"message.usage.inputTokens",
		"usage.inputTokens",
		"input_tokens",
		"prompt_tokens",
	)
	if usage.IsStream {
		if input > usage.InputTokens {
			usage.InputTokens = input
		}
	} else if input > 0 {
		usage.InputTokens = input
	}

	output := firstPositiveIntFromPaths(
		data,
		"message.usage.output_tokens",
		"usage.output_tokens",
		"message.usage.completion_tokens",
		"usage.completion_tokens",
		"message.usage.output_token_count",
		"usage.output_token_count",
		"message.usage.outputTokens",
		"usage.outputTokens",
		"output_tokens",
		"completion_tokens",
	)
	if usage.IsStream {
		if output > usage.OutputTokens {
			usage.OutputTokens = output
		}
	} else if output > 0 {
		usage.OutputTokens = output
	}

	cacheCreate := firstPositiveIntFromPaths(
		data,
		"message.usage.cache_creation_input_tokens",
		"usage.cache_creation_input_tokens",
		"usage.cache_create_tokens",
		"cache_creation_input_tokens",
		"cache_create_tokens",
	)
	cacheCreateEphemeral5m := firstPositiveIntFromPaths(
		data,
		"message.usage.cache_creation.ephemeral_5m_input_tokens",
		"usage.cache_creation.ephemeral_5m_input_tokens",
		"message.usage.cache_creation.ephemeral_5m_tokens",
		"usage.cache_creation.ephemeral_5m_tokens",
		"message.usage.cache_creation_input_tokens_details.ephemeral_5m_input_tokens",
		"usage.cache_creation_input_tokens_details.ephemeral_5m_input_tokens",
	)
	cacheCreateEphemeral1h := firstPositiveIntFromPaths(
		data,
		"message.usage.cache_creation.ephemeral_1h_input_tokens",
		"usage.cache_creation.ephemeral_1h_input_tokens",
		"message.usage.cache_creation.ephemeral_1h_tokens",
		"usage.cache_creation.ephemeral_1h_tokens",
		"message.usage.cache_creation_input_tokens_details.ephemeral_1h_input_tokens",
		"usage.cache_creation_input_tokens_details.ephemeral_1h_input_tokens",
	)
	applyCacheCreateTokenBreakdown(usage, cacheCreate, cacheCreateEphemeral5m, cacheCreateEphemeral1h)

	cacheRead := firstPositiveIntFromPaths(
		data,
		"message.usage.cache_read_input_tokens",
		"usage.cache_read_input_tokens",
		"usage.input_tokens_details.cached_tokens",
		"cache_read_input_tokens",
	)
	if usage.IsStream {
		if cacheRead > usage.CacheReadTokens {
			usage.CacheReadTokens = cacheRead
		}
	} else if cacheRead > 0 {
		usage.CacheReadTokens = cacheRead
	}
}

func applyCacheCreateTokenBreakdown(usage *ReqeustLog, cacheCreateTokens int, ephemeral5mTokens int, ephemeral1hTokens int) {
	if usage == nil {
		return
	}
	normalizedTotal, normalized5m, normalized1h := normalizeCacheCreationTokenSplit(
		cacheCreateTokens,
		ephemeral5mTokens,
		ephemeral1hTokens,
	)
	if normalizedTotal == 0 && normalized5m == 0 && normalized1h == 0 {
		return
	}

	if usage.IsStream {
		currentTotal, current5m, current1h := normalizeCacheCreationTokenSplit(
			usage.CacheCreateTokens,
			usage.Ephemeral5mTokens,
			usage.Ephemeral1hTokens,
		)
		if normalizedTotal > currentTotal {
			currentTotal = normalizedTotal
		}
		if normalized5m > current5m {
			current5m = normalized5m
		}
		if normalized1h > current1h {
			current1h = normalized1h
		}
		currentTotal, current5m, current1h = normalizeCacheCreationTokenSplit(currentTotal, current5m, current1h)
		usage.CacheCreateTokens = currentTotal
		usage.Ephemeral5mTokens = current5m
		usage.Ephemeral1hTokens = current1h
		return
	}

	usage.CacheCreateTokens = normalizedTotal
	usage.Ephemeral5mTokens = normalized5m
	usage.Ephemeral1hTokens = normalized1h
}

func firstPositiveIntFromPaths(data string, paths ...string) int {
	for _, path := range paths {
		value := int(gjson.Get(data, path).Int())
		if value > 0 {
			return value
		}
	}
	return 0
}

// codex usage parser
func CodexParseTokenUsageFromResponse(data string, usage *ReqeustLog) {
	if usage == nil {
		return
	}

	input := int(gjson.Get(data, "response.usage.input_tokens").Int())
	output := int(gjson.Get(data, "response.usage.output_tokens").Int())
	cacheRead := int(gjson.Get(data, "response.usage.input_tokens_details.cached_tokens").Int())
	reasoning := int(gjson.Get(data, "response.usage.output_tokens_details.reasoning_tokens").Int())

	if usage.IsStream {
		if input > usage.InputTokens {
			usage.InputTokens = input
		}
		if output > usage.OutputTokens {
			usage.OutputTokens = output
		}
		if cacheRead > usage.CacheReadTokens {
			usage.CacheReadTokens = cacheRead
		}
		if reasoning > usage.ReasoningTokens {
			usage.ReasoningTokens = reasoning
		}
		return
	}

	if input > 0 {
		usage.InputTokens = input
	}
	if output > 0 {
		usage.OutputTokens = output
	}
	if cacheRead > 0 {
		usage.CacheReadTokens = cacheRead
	}
	if reasoning > 0 {
		usage.ReasoningTokens = reasoning
	}
}

// gemini usage parser (流式响应专用)
// Gemini SSE 流中每个 chunk 都会携带完整的 usageMetadata，需取最大值而非累加
func GeminiParseTokenUsageFromResponse(data string, usage *ReqeustLog) {
	usageResult := gjson.Get(data, "usageMetadata")
	if !usageResult.Exists() {
		return
	}
	mergeGeminiUsageMetadata(usageResult, usage)
}

// mergeGeminiUsageMetadata 合并 Gemini usageMetadata 到 ReqeustLog（取最大值去重）
// Gemini 流式响应特点：每个 chunk 包含截止当前的累计用量，因此取最大值即可
func mergeGeminiUsageMetadata(usage gjson.Result, reqLog *ReqeustLog) {
	if !usage.Exists() || reqLog == nil {
		return
	}

	// 取最大值（流式响应中后续 chunk 包含前面的累计值）
	if v := int(usage.Get("promptTokenCount").Int()); v > reqLog.InputTokens {
		reqLog.InputTokens = v
	}
	if v := int(usage.Get("candidatesTokenCount").Int()); v > reqLog.OutputTokens {
		reqLog.OutputTokens = v
	}
	if v := int(usage.Get("cachedContentTokenCount").Int()); v > reqLog.CacheReadTokens {
		reqLog.CacheReadTokens = v
	}
	// Gemini thinking/reasoning tokens (thoughtsTokenCount)
	// 参考: https://ai.google.dev/gemini-api/docs/thinking
	if v := int(usage.Get("thoughtsTokenCount").Int()); v > reqLog.ReasoningTokens {
		reqLog.ReasoningTokens = v
	}

	// 若仅提供 totalTokenCount，按 total - input 估算输出 token
	total := usage.Get("totalTokenCount").Int()
	if total > 0 && reqLog.OutputTokens == 0 && reqLog.InputTokens > 0 && reqLog.InputTokens < int(total) {
		reqLog.OutputTokens = int(total) - reqLog.InputTokens
	}
}

// streamGeminiResponseWithHook 流式传输 Gemini 响应并通过 Hook 提取 token 用量
// 【修复】维护跨 chunk 缓冲，确保完整 SSE 事件解析
// Gemini SSE 格式: "data: {json}\n\n" 或 "data: [DONE]\n\n"
func streamGeminiResponseWithHook(body io.Reader, writer io.Writer, requestLog *ReqeustLog) error {
	buf := make([]byte, 8192)   // 增大缓冲区减少系统调用
	var lineBuf strings.Builder // 跨 chunk 行缓冲

	for {
		n, err := body.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			appendRequestLogResponseBody(requestLog, chunk)
			// 写入客户端（优先保证数据传输）
			if _, writeErr := writer.Write(chunk); writeErr != nil {
				return writeErr
			}
			// 如果是 http.Flusher，立即刷新
			if flusher, ok := writer.(http.Flusher); ok {
				flusher.Flush()
			}
			// 解析 SSE 数据提取 token 用量（使用缓冲处理跨 chunk 情况）
			parseGeminiSSEWithBuffer(string(chunk), &lineBuf, requestLog)
		}
		if err != nil {
			// 处理缓冲区残留数据
			if lineBuf.Len() > 0 {
				parseGeminiSSELine(lineBuf.String(), requestLog)
				lineBuf.Reset()
			}
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// parseGeminiSSEWithBuffer 使用缓冲处理跨 chunk 的 SSE 事件
// 【修复】解决 JSON 被 TCP 分割到多个 chunk 导致解析失败的问题
func parseGeminiSSEWithBuffer(chunk string, lineBuf *strings.Builder, requestLog *ReqeustLog) {
	// 将当前 chunk 追加到缓冲
	lineBuf.WriteString(chunk)
	content := lineBuf.String()

	// 按双换行符分割完整的 SSE 事件
	// SSE 格式: "data: {...}\n\n" 或 "data: {...}\r\n\r\n"
	for {
		// 查找事件分隔符（双换行）
		idx := strings.Index(content, "\n\n")
		if idx == -1 {
			// 尝试 \r\n\r\n 分隔符
			idx = strings.Index(content, "\r\n\r\n")
			if idx == -1 {
				break // 没有完整事件，等待更多数据
			}
			idx += 4 // \r\n\r\n 长度
		} else {
			idx += 2 // \n\n 长度
		}

		// 提取完整事件
		event := content[:idx]
		content = content[idx:]

		// 解析事件中的 data 行
		parseGeminiSSELine(event, requestLog)
	}

	// 更新缓冲区为未处理的残留数据
	lineBuf.Reset()
	lineBuf.WriteString(content)
}

// parseGeminiSSELine 解析单个 SSE 事件提取 usageMetadata
// 【优化】只在包含 usageMetadata 时才调用 gjson 解析
func parseGeminiSSELine(event string, requestLog *ReqeustLog) {
	lines := strings.Split(event, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" || data == "" {
			continue
		}
		updateResponseModelFromPayload(data, requestLog)
		updateFirstTokenFromPayload(data, requestLog)
		// 【优化】快速检查是否包含 usageMetadata，避免无效解析
		if !strings.Contains(data, "usageMetadata") {
			continue
		}
		GeminiParseTokenUsageFromResponse(data, requestLog)
	}
}

// ReplaceModelInRequestBody 替换请求体中的模型名
// 使用 gjson + sjson 实现高性能 JSON 操作，避免完整反序列化
func ReplaceModelInRequestBody(bodyBytes []byte, newModel string) ([]byte, error) {
	// 检查请求体中是否存在 model 字段
	result := gjson.GetBytes(bodyBytes, "model")
	if !result.Exists() {
		return bodyBytes, fmt.Errorf("请求体中未找到 model 字段")
	}

	// 使用 sjson.SetBytes 替换模型名（高性能操作）
	modified, err := sjson.SetBytes(bodyBytes, "model", newModel)
	if err != nil {
		return bodyBytes, fmt.Errorf("替换模型名失败: %w", err)
	}

	return modified, nil
}

func ApplyRequestBodyOverrides(bodyBytes []byte, overrides map[string]interface{}) ([]byte, error) {
	if len(overrides) == 0 {
		return bodyBytes, nil
	}

	trimmedBody := bytes.TrimSpace(bodyBytes)
	if len(trimmedBody) == 0 {
		trimmedBody = []byte("{}")
	}

	if !json.Valid(trimmedBody) {
		return bodyBytes, fmt.Errorf("请求体不是合法 JSON")
	}
	if trimmedBody[0] != '{' {
		return bodyBytes, fmt.Errorf("请求体根节点必须是 JSON 对象")
	}

	currentBody := append([]byte(nil), trimmedBody...)
	return applyRequestBodyOverrideMap(currentBody, "", overrides)
}

func applyRequestBodyOverrideMap(bodyBytes []byte, prefix string, values map[string]interface{}) ([]byte, error) {
	if len(values) == 0 {
		return bodyBytes, nil
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		if strings.TrimSpace(key) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	currentBody := bodyBytes
	for _, key := range keys {
		fullPath := key
		if prefix != "" {
			fullPath = prefix + "." + key
		}

		value := values[key]
		nested, isNestedMap := value.(map[string]interface{})
		if isNestedMap && len(nested) > 0 {
			modifiedBody, err := applyRequestBodyOverrideMap(currentBody, fullPath, nested)
			if err != nil {
				return bodyBytes, err
			}
			currentBody = modifiedBody
			continue
		}

		modifiedBody, err := sjson.SetBytes(currentBody, fullPath, value)
		if err != nil {
			return bodyBytes, fmt.Errorf("设置请求体字段 %q 失败: %w", fullPath, err)
		}
		currentBody = modifiedBody
	}

	return currentBody, nil
}

func resolveModelFromRequestBody(bodyBytes []byte, fallback string) string {
	model := strings.TrimSpace(gjson.GetBytes(bodyBytes, "model").String())
	if model != "" {
		return model
	}
	return fallback
}

type providerRequestPlan struct {
	BodyBytes                  []byte
	ContinuationRetryBodyBytes []byte
	EffectiveModel             string
	EffectiveEndpoint          string
	PromptCacheKey             string
	ContinuationSessionKey     string
	PreviousResponseID         string
}

func (prs *ProviderRelayService) buildProviderRequestPlan(provider Provider, bodyBytes []byte, endpoint string, requestedModel string) (providerRequestPlan, error) {
	effectiveModel := provider.GetEffectiveModel(requestedModel)
	currentBodyBytes := bodyBytes

	if effectiveModel != requestedModel && requestedModel != "" {
		modifiedBody, err := ReplaceModelInRequestBody(bodyBytes, effectiveModel)
		if err != nil {
			return providerRequestPlan{}, err
		}
		currentBodyBytes = modifiedBody
	}

	if len(provider.RequestBodyOverrides) > 0 {
		modifiedBody, err := ApplyRequestBodyOverrides(currentBodyBytes, provider.RequestBodyOverrides)
		if err != nil {
			return providerRequestPlan{}, err
		}
		currentBodyBytes = modifiedBody
	}

	currentBodyBytes = applyProviderAnthropicCacheTTLOverride(provider, endpoint, currentBodyBytes)

	effectiveEndpoint := provider.GetEffectiveEndpoint(endpoint)
	promptCacheKey := ""
	continuationSessionKey := ""
	previousResponseID := ""
	if endpoint == "/v1/messages" {
		effectiveEndpoint = resolveProviderEffectiveEndpoint("claude", provider, endpoint)
		if claudeAPIFormatNeedsTransform(resolveClaudeAPIFormat(provider)) {
			claudeBodyBytes := currentBodyBytes
			hasExplicitPromptCacheKey := strings.TrimSpace(gjson.GetBytes(claudeBodyBytes, "prompt_cache_key").String()) != ""
			modifiedBody, err := transformClaudeRequestForAPIFormat(currentBodyBytes, provider)
			if err != nil {
				return providerRequestPlan{}, err
			}
			currentBodyBytes = modifiedBody
			if resolveClaudeAPIFormat(provider) == claudeAPIFormatOpenAIResponse {
				continuationSessionKey = deriveClaudeResponsesContinuationSessionKey(claudeBodyBytes)
				if !hasExplicitPromptCacheKey && prs != nil && prs.isOpenAICompatPromptCacheDisabled(provider, continuationSessionKey) {
					currentBodyBytes = removeJSONFieldBytes(currentBodyBytes, "prompt_cache_key")
				}
				promptCacheKey = strings.TrimSpace(gjson.GetBytes(currentBodyBytes, "prompt_cache_key").String())
				baseResponsesBodyBytes := currentBodyBytes
				continuationRetryBodyBytes := trimClaudeResponsesInputForTailReplayGuard(baseResponsesBodyBytes, claudeResponsesTailReplayMaxInputItems)
				storeDisabled := gjson.GetBytes(currentBodyBytes, "store").Exists() &&
					!gjson.GetBytes(currentBodyBytes, "store").Bool()
				if continuationSessionKey != "" && prs != nil && !storeDisabled && !prs.isClaudeResponsesContinuationDisabled(provider, continuationSessionKey) {
					previousResponseID = prs.getClaudeResponsesPreviousResponseID(provider, continuationSessionKey)
					if previousResponseID != "" {
						currentBodyBytes = trimClaudeResponsesInputToLatestTurn(baseResponsesBodyBytes)
					}
					if previousResponseID != "" {
						withPrevious, err := sjson.SetBytes(currentBodyBytes, "previous_response_id", previousResponseID)
						if err != nil {
							return providerRequestPlan{}, fmt.Errorf("设置 previous_response_id 失败: %w", err)
						}
						currentBodyBytes = withPrevious
					}
				}
				if previousResponseID == "" {
					currentBodyBytes = baseResponsesBodyBytes
				}
				return providerRequestPlan{
					BodyBytes:                  currentBodyBytes,
					ContinuationRetryBodyBytes: continuationRetryBodyBytes,
					EffectiveModel:             resolveModelFromRequestBody(currentBodyBytes, effectiveModel),
					EffectiveEndpoint:          effectiveEndpoint,
					PromptCacheKey:             promptCacheKey,
					ContinuationSessionKey:     continuationSessionKey,
					PreviousResponseID:         previousResponseID,
				}, nil
			}
		}
	}

	return providerRequestPlan{
		BodyBytes:                  currentBodyBytes,
		ContinuationRetryBodyBytes: nil,
		EffectiveModel:             resolveModelFromRequestBody(currentBodyBytes, effectiveModel),
		EffectiveEndpoint:          effectiveEndpoint,
		PromptCacheKey:             promptCacheKey,
		ContinuationSessionKey:     continuationSessionKey,
		PreviousResponseID:         previousResponseID,
	}, nil
}

func buildProviderRequestPlan(provider Provider, bodyBytes []byte, endpoint string, requestedModel string) (providerRequestPlan, error) {
	return (*ProviderRelayService)(nil).buildProviderRequestPlan(provider, bodyBytes, endpoint, requestedModel)
}

func deriveClaudeResponsesContinuationSessionKey(bodyBytes []byte) string {
	metadata := gjson.GetBytes(bodyBytes, "metadata")
	if !metadata.Exists() {
		return ""
	}

	if parsed := parseClaudeMetadataUserID(metadata.Get("user_id").String()); parsed != nil {
		seed := strings.Join([]string{
			strings.TrimSpace(parsed.DeviceID),
			strings.TrimSpace(parsed.AccountUUID),
			strings.TrimSpace(parsed.SessionID),
		}, "|")
		return "metadata-user-" + shortSHA256Hex(seed)
	}

	for _, path := range []string{
		"session_id",
		"sessionId",
		"conversation_id",
		"conversationId",
		"thread_id",
		"threadId",
	} {
		if value := strings.TrimSpace(metadata.Get(path).String()); value != "" {
			return "metadata-session-" + shortSHA256Hex(path+"="+value)
		}
	}
	return ""
}

type claudeMetadataUserIDParts struct {
	DeviceID    string
	AccountUUID string
	SessionID   string
}

func parseClaudeMetadataUserID(raw string) *claudeMetadataUserIDParts {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.HasPrefix(raw, "{") {
		var parsed struct {
			DeviceID    string `json:"device_id"`
			AccountUUID string `json:"account_uuid"`
			SessionID   string `json:"session_id"`
		}
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			return nil
		}
		if strings.TrimSpace(parsed.DeviceID) == "" || strings.TrimSpace(parsed.SessionID) == "" {
			return nil
		}
		return &claudeMetadataUserIDParts{
			DeviceID:    parsed.DeviceID,
			AccountUUID: parsed.AccountUUID,
			SessionID:   parsed.SessionID,
		}
	}
	matches := claudeMetadataLegacyUserIDRegex.FindStringSubmatch(raw)
	if matches == nil {
		return nil
	}
	return &claudeMetadataUserIDParts{
		DeviceID:    matches[1],
		AccountUUID: matches[2],
		SessionID:   matches[3],
	}
}

func (prs *ProviderRelayService) claudeResponsesSessionKey(provider Provider, continuationSessionKey string) string {
	key := strings.TrimSpace(continuationSessionKey)
	if key == "" {
		return ""
	}
	return strings.Join([]string{
		providerRefFromProvider(provider),
		strings.TrimSpace(provider.APIURL),
		hashProviderAPIKeyForMemoryKey(provider.APIKey),
		key,
	}, "\x00")
}

func (prs *ProviderRelayService) openAICompatPromptCacheDisableKey(provider Provider, continuationSessionKey string) string {
	return openAICompatPromptCacheDisableKey(provider, continuationSessionKey)
}

func openAICompatPromptCacheDisableKey(provider Provider, continuationSessionKey string) string {
	scope := strings.TrimSpace(continuationSessionKey)
	if scope == "" {
		scope = "provider"
	}
	return strings.Join([]string{
		providerRefFromProvider(provider),
		strings.TrimSpace(provider.APIURL),
		hashProviderAPIKeyForMemoryKey(provider.APIKey),
		scope,
	}, "\x00")
}

func hashProviderAPIKeyForMemoryKey(apiKey string) string {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return ""
	}
	return "sha256:" + shortSHA256Hex(apiKey)
}

func (prs *ProviderRelayService) isOpenAICompatPromptCacheDisabled(provider Provider, continuationSessionKey string) bool {
	return isOpenAICompatPromptCacheDisabled(provider, continuationSessionKey)
}

func isOpenAICompatPromptCacheDisabled(provider Provider, continuationSessionKey string) bool {
	providerKey := openAICompatPromptCacheDisableKey(provider, "")
	key := openAICompatPromptCacheDisableKey(provider, continuationSessionKey)
	if key == "" {
		return false
	}
	openAICompatPromptCacheDisabledMu.Lock()
	defer openAICompatPromptCacheDisabledMu.Unlock()
	if openAICompatPromptCacheDisabled == nil {
		return false
	}
	now := time.Now()
	sweepOpenAICompatPromptCacheDisabledLocked(now)
	if providerKey != "" && providerKey != key {
		if promptCacheDisableEntryActiveLocked(providerKey, now) {
			return true
		}
	}
	return promptCacheDisableEntryActiveLocked(key, now)
}

func promptCacheDisableEntryActiveLocked(key string, now time.Time) bool {
	expiresAt, ok := openAICompatPromptCacheDisabled[key]
	if !ok {
		return false
	}
	if !expiresAt.IsZero() && now.After(expiresAt) {
		delete(openAICompatPromptCacheDisabled, key)
		return false
	}
	return true
}

func (prs *ProviderRelayService) disableOpenAICompatPromptCache(provider Provider, continuationSessionKey string) {
	disableOpenAICompatPromptCache(provider, continuationSessionKey)
}

func disableOpenAICompatPromptCache(provider Provider, continuationSessionKey string) {
	key := openAICompatPromptCacheDisableKey(provider, continuationSessionKey)
	if key == "" {
		return
	}
	providerKey := openAICompatPromptCacheDisableKey(provider, "")
	openAICompatPromptCacheDisabledMu.Lock()
	defer openAICompatPromptCacheDisabledMu.Unlock()
	if openAICompatPromptCacheDisabled == nil {
		openAICompatPromptCacheDisabled = make(map[string]time.Time)
	}
	now := time.Now()
	expiresAt := now.Add(openAICompatPromptCacheDisableTTL)
	openAICompatPromptCacheDisabled[key] = expiresAt
	if providerKey != "" {
		openAICompatPromptCacheDisabled[providerKey] = expiresAt
	}
	sweepOpenAICompatPromptCacheDisabledLocked(now)
}

func (prs *ProviderRelayService) sweepOpenAICompatPromptCacheDisabledLocked(now time.Time) {
	sweepOpenAICompatPromptCacheDisabledLocked(now)
}

func sweepOpenAICompatPromptCacheDisabledLocked(now time.Time) {
	if len(openAICompatPromptCacheDisabled) == 0 {
		return
	}
	for key, expiresAt := range openAICompatPromptCacheDisabled {
		if !expiresAt.IsZero() && now.After(expiresAt) {
			delete(openAICompatPromptCacheDisabled, key)
		}
	}
}

func (prs *ProviderRelayService) getClaudeResponsesPreviousResponseID(provider Provider, continuationSessionKey string) string {
	if prs == nil {
		return ""
	}
	key := prs.claudeResponsesSessionKey(provider, continuationSessionKey)
	if key == "" {
		return ""
	}
	prs.claudeResponsesMu.Lock()
	defer prs.claudeResponsesMu.Unlock()
	if prs.claudeResponses == nil {
		return ""
	}
	now := time.Now()
	prs.sweepClaudeResponsesSessionsLocked(now)
	binding, ok := prs.claudeResponses[key]
	if !ok {
		return ""
	}
	if !binding.ExpiresAt.IsZero() && now.After(binding.ExpiresAt) {
		delete(prs.claudeResponses, key)
		return ""
	}
	if binding.Disabled {
		return ""
	}
	return strings.TrimSpace(binding.ResponseID)
}

func (prs *ProviderRelayService) bindClaudeResponsesPreviousResponseID(provider Provider, continuationSessionKey string, responseID string) {
	if prs == nil {
		return
	}
	key := prs.claudeResponsesSessionKey(provider, continuationSessionKey)
	id := strings.TrimSpace(responseID)
	if key == "" || id == "" {
		return
	}
	prs.claudeResponsesMu.Lock()
	defer prs.claudeResponsesMu.Unlock()
	if prs.claudeResponses == nil {
		prs.claudeResponses = make(map[string]claudeResponsesSessionBinding)
	}
	now := time.Now()
	existing := prs.claudeResponses[key]
	if existing.Disabled {
		existing.ExpiresAt = now.Add(claudeResponsesSessionTTL)
		prs.claudeResponses[key] = existing
		prs.sweepClaudeResponsesSessionsLocked(now)
		return
	}
	prs.claudeResponses[key] = claudeResponsesSessionBinding{
		ResponseID: id,
		ExpiresAt:  now.Add(claudeResponsesSessionTTL),
	}
	prs.sweepClaudeResponsesSessionsLocked(now)
}

func (prs *ProviderRelayService) disableClaudeResponsesContinuation(provider Provider, continuationSessionKey string) {
	if prs == nil {
		return
	}
	key := prs.claudeResponsesSessionKey(provider, continuationSessionKey)
	if key == "" {
		return
	}
	prs.claudeResponsesMu.Lock()
	defer prs.claudeResponsesMu.Unlock()
	if prs.claudeResponses == nil {
		prs.claudeResponses = make(map[string]claudeResponsesSessionBinding)
	}
	now := time.Now()
	prs.claudeResponses[key] = claudeResponsesSessionBinding{
		Disabled:  true,
		ExpiresAt: now.Add(claudeResponsesSessionTTL),
	}
	prs.sweepClaudeResponsesSessionsLocked(now)
}

func (prs *ProviderRelayService) deleteClaudeResponsesPreviousResponseID(provider Provider, continuationSessionKey string) {
	if prs == nil {
		return
	}
	key := prs.claudeResponsesSessionKey(provider, continuationSessionKey)
	if key == "" {
		return
	}
	prs.claudeResponsesMu.Lock()
	defer prs.claudeResponsesMu.Unlock()
	if prs.claudeResponses == nil {
		return
	}
	delete(prs.claudeResponses, key)
}

func (prs *ProviderRelayService) isClaudeResponsesContinuationDisabled(provider Provider, continuationSessionKey string) bool {
	if prs == nil {
		return false
	}
	key := prs.claudeResponsesSessionKey(provider, continuationSessionKey)
	if key == "" {
		return false
	}
	prs.claudeResponsesMu.Lock()
	defer prs.claudeResponsesMu.Unlock()
	if prs.claudeResponses == nil {
		return false
	}
	now := time.Now()
	prs.sweepClaudeResponsesSessionsLocked(now)
	binding, ok := prs.claudeResponses[key]
	if !ok {
		return false
	}
	if !binding.ExpiresAt.IsZero() && now.After(binding.ExpiresAt) {
		delete(prs.claudeResponses, key)
		return false
	}
	return binding.Disabled
}

func (prs *ProviderRelayService) sweepClaudeResponsesSessionsLocked(now time.Time) {
	if prs == nil || len(prs.claudeResponses) == 0 {
		return
	}
	for key, binding := range prs.claudeResponses {
		if !binding.ExpiresAt.IsZero() && now.After(binding.ExpiresAt) {
			delete(prs.claudeResponses, key)
		}
	}
	if claudeResponsesMaxSessionBindings <= 0 || len(prs.claudeResponses) <= claudeResponsesMaxSessionBindings {
		return
	}

	type sessionExpiry struct {
		Key       string
		ExpiresAt time.Time
	}
	entries := make([]sessionExpiry, 0, len(prs.claudeResponses))
	for key, binding := range prs.claudeResponses {
		entries = append(entries, sessionExpiry{Key: key, ExpiresAt: binding.ExpiresAt})
	}
	sort.Slice(entries, func(i, j int) bool {
		left := entries[i].ExpiresAt
		right := entries[j].ExpiresAt
		if left.IsZero() && right.IsZero() {
			return entries[i].Key < entries[j].Key
		}
		if left.IsZero() {
			return false
		}
		if right.IsZero() {
			return true
		}
		if left.Equal(right) {
			return entries[i].Key < entries[j].Key
		}
		return left.Before(right)
	})
	for i := 0; i < len(entries)-claudeResponsesMaxSessionBindings; i++ {
		delete(prs.claudeResponses, entries[i].Key)
	}
}

func (prs *ProviderRelayService) newClaudeResponsesSessionHook(kind string, provider Provider, plan providerRequestPlan, isStream bool) xrequest.ResponseHook {
	if prs == nil ||
		kind != "claude" ||
		resolveClaudeAPIFormat(provider) != claudeAPIFormatOpenAIResponse ||
		strings.TrimSpace(plan.ContinuationSessionKey) == "" {
		return nil
	}
	if isStream {
		return prs.newClaudeResponsesStreamSessionHook(provider, plan)
	}
	return func(data []byte) (bool, []byte) {
		responseID := strings.TrimSpace(gjson.GetBytes(data, "id").String())
		if responseID != "" {
			prs.bindClaudeResponsesPreviousResponseID(provider, plan.ContinuationSessionKey, responseID)
		}
		return true, data
	}
}

func (prs *ProviderRelayService) newClaudeResponsesStreamSessionHook(provider Provider, plan providerRequestPlan) xrequest.ResponseHook {
	var pendingEventType string
	var pendingResponseID string
	var pendingDataLines []string
	return func(data []byte) (bool, []byte) {
		line := strings.TrimSpace(string(data))
		if line == "" {
			if len(pendingDataLines) > 0 {
				payload := combineOpenAIResponsesDataLines(pendingDataLines)
				prs.updateClaudeResponsesStreamSessionBinding(provider, plan, payload, pendingEventType, &pendingResponseID)
				pendingDataLines = nil
				pendingEventType = ""
			}
			return true, data
		}
		if strings.HasPrefix(line, "event:") {
			pendingEventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			return true, data
		}
		if !strings.HasPrefix(line, "data:") {
			return true, data
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if len(pendingDataLines) > 0 {
			pendingDataLines = append(pendingDataLines, payload)
			combinedPayload := combineOpenAIResponsesDataLines(pendingDataLines)
			if gjson.Valid(combinedPayload) {
				prs.updateClaudeResponsesStreamSessionBinding(provider, plan, combinedPayload, pendingEventType, &pendingResponseID)
				pendingDataLines = nil
				pendingEventType = ""
			}
			return true, data
		}
		if payload != "" && payload != "[DONE]" && !gjson.Valid(payload) {
			pendingDataLines = append(pendingDataLines, payload)
			return true, data
		}
		prs.updateClaudeResponsesStreamSessionBinding(provider, plan, payload, pendingEventType, &pendingResponseID)
		if payload != "" && payload != "[DONE]" {
			pendingEventType = ""
		}
		return true, data
	}
}

func (prs *ProviderRelayService) updateClaudeResponsesStreamSessionBinding(
	provider Provider,
	plan providerRequestPlan,
	payload string,
	pendingEventType string,
	pendingResponseID *string,
) {
	payload = strings.TrimSpace(payload)
	if payload == "" || payload == "[DONE]" || !gjson.Valid(payload) {
		return
	}
	eventType := strings.TrimSpace(gjson.Get(payload, "type").String())
	if eventType == "" {
		eventType = strings.TrimSpace(pendingEventType)
	}
	switch eventType {
	case "response.created":
		responseID := strings.TrimSpace(gjson.Get(payload, "response.id").String())
		if responseID == "" {
			responseID = strings.TrimSpace(gjson.Get(payload, "id").String())
		}
		if responseID != "" && pendingResponseID != nil {
			*pendingResponseID = responseID
		}
	case "response.completed", "response.done":
		if eventType == "response.done" {
			switch extractOpenAIResponsesResponseStatus(payload) {
			case "failed", "incomplete", "cancelled", "canceled":
				return
			}
		}
		responseID := strings.TrimSpace(gjson.Get(payload, "response.id").String())
		if responseID == "" {
			responseID = strings.TrimSpace(gjson.Get(payload, "id").String())
		}
		if responseID == "" && pendingResponseID != nil {
			responseID = *pendingResponseID
		}
		if responseID != "" {
			prs.bindClaudeResponsesPreviousResponseID(provider, plan.ContinuationSessionKey, responseID)
		}
	}
}

func (prs *ProviderRelayService) classifyClaudeResponsesContinuationRejection(kind string, provider Provider, plan providerRequestPlan, status int, body []byte) claudeResponsesContinuationRejection {
	if prs == nil ||
		kind != "claude" ||
		resolveClaudeAPIFormat(provider) != claudeAPIFormatOpenAIResponse ||
		strings.TrimSpace(plan.ContinuationSessionKey) == "" ||
		strings.TrimSpace(plan.PreviousResponseID) == "" {
		return claudeResponsesContinuationRejectionNone
	}
	if isClaudeResponsesPreviousResponseNotFound(status, body) {
		return claudeResponsesContinuationRejectionNotFound
	}
	if isClaudeResponsesPreviousResponseUnsupported(status, body) {
		return claudeResponsesContinuationRejectionUnsupported
	}
	return claudeResponsesContinuationRejectionNone
}

func (prs *ProviderRelayService) isOpenAICompatPromptCacheKeyUnsupported(kind string, provider Provider, plan providerRequestPlan, status int, body []byte) bool {
	if prs == nil ||
		kind != "claude" ||
		resolveClaudeAPIFormat(provider) != claudeAPIFormatOpenAIResponse ||
		strings.TrimSpace(plan.PromptCacheKey) == "" {
		return false
	}
	return isOpenAICompatPromptCacheKeyUnsupportedStatus(status, body)
}

func isOpenAICompatPromptCacheKeyUnsupportedStatus(status int, body []byte) bool {
	if status < http.StatusBadRequest || status >= http.StatusInternalServerError {
		return false
	}
	bodyText := strings.ToLower(strings.TrimSpace(string(body)))
	if bodyText == "" || !strings.Contains(bodyText, "prompt_cache_key") {
		return false
	}
	return strings.Contains(bodyText, "unsupported") ||
		strings.Contains(bodyText, "not supported") ||
		strings.Contains(bodyText, "unsupported parameter") ||
		strings.Contains(bodyText, "unknown parameter") ||
		strings.Contains(bodyText, "unrecognized parameter") ||
		strings.Contains(bodyText, "invalid parameter")
}

func isClaudeResponsesPreviousResponseNotFound(status int, body []byte) bool {
	if status != http.StatusBadRequest && status != http.StatusNotFound {
		return false
	}
	bodyText := strings.ToLower(strings.TrimSpace(string(body)))
	if bodyText == "" {
		return false
	}
	if strings.Contains(bodyText, "previous_response_not_found") ||
		(strings.Contains(bodyText, "previous response") && strings.Contains(bodyText, "not found")) {
		return true
	}
	return false
}

func isClaudeResponsesPreviousResponseUnsupported(status int, body []byte) bool {
	if status != http.StatusBadRequest && status != http.StatusNotFound {
		return false
	}
	bodyText := strings.ToLower(strings.TrimSpace(string(body)))
	if bodyText == "" {
		return false
	}
	mentionsPreviousResponseID := strings.Contains(bodyText, "previous_response_id") ||
		strings.Contains(bodyText, "previous response id") ||
		strings.Contains(bodyText, "previous response")
	if mentionsPreviousResponseID &&
		(strings.Contains(bodyText, "unsupported") ||
			strings.Contains(bodyText, "not supported") ||
			strings.Contains(bodyText, "only supported on responses websocket") ||
			strings.Contains(bodyText, "unsupported parameter") ||
			strings.Contains(bodyText, "unknown parameter") ||
			strings.Contains(bodyText, "unrecognized parameter")) {
		return true
	}
	return false
}

func claudeResponsesInputHasFunctionCallOutput(bodyBytes []byte) bool {
	return gjson.GetBytes(bodyBytes, `input.#(type=="function_call_output")`).Exists()
}

func claudeResponsesCanRetryWithoutContinuation(plan providerRequestPlan) bool {
	if len(plan.ContinuationRetryBodyBytes) == 0 {
		return false
	}
	if !claudeResponsesInputHasFunctionCallOutput(plan.ContinuationRetryBodyBytes) {
		return true
	}
	return claudeResponsesInputHasSafeFunctionCallReplay(plan.ContinuationRetryBodyBytes)
}

func claudeResponsesInputHasSafeFunctionCallReplay(bodyBytes []byte) bool {
	input := gjson.GetBytes(bodyBytes, "input")
	if !input.IsArray() {
		return false
	}
	functionCallIDs := make(map[string]bool)
	functionCallOutputIDs := make(map[string]bool)
	for _, item := range input.Array() {
		itemType := strings.TrimSpace(item.Get("type").String())
		callID := strings.TrimSpace(item.Get("call_id").String())
		if callID == "" {
			callID = strings.TrimSpace(item.Get("id").String())
		}
		if callID == "" {
			continue
		}
		switch itemType {
		case "function_call":
			functionCallIDs[callID] = true
		case "function_call_output":
			functionCallOutputIDs[callID] = true
		}
	}
	if len(functionCallOutputIDs) == 0 {
		return true
	}
	for callID := range functionCallOutputIDs {
		if !functionCallIDs[callID] {
			return false
		}
	}
	return true
}

func trimClaudeResponsesInputToLatestTurn(bodyBytes []byte) []byte {
	input := gjson.GetBytes(bodyBytes, "input")
	if !input.IsArray() {
		return bodyBytes
	}
	rawItems := input.Array()
	if len(rawItems) == 0 {
		return bodyBytes
	}
	start := claudeResponsesLatestTurnStartIndex(rawItems)
	if start <= 0 {
		return bodyBytes
	}
	trimmed := make([]interface{}, 0, len(rawItems)-start)
	for _, item := range rawItems[start:] {
		var decoded interface{}
		if err := json.Unmarshal([]byte(item.Raw), &decoded); err != nil {
			return bodyBytes
		}
		trimmed = append(trimmed, decoded)
	}
	updated, err := sjson.SetBytes(bodyBytes, "input", trimmed)
	if err != nil {
		return bodyBytes
	}
	return updated
}

func trimClaudeResponsesInputForTailReplayGuard(bodyBytes []byte, maxItems int) []byte {
	if maxItems <= 0 {
		return bodyBytes
	}
	input := gjson.GetBytes(bodyBytes, "input")
	if !input.IsArray() {
		return bodyBytes
	}
	rawItems := input.Array()
	if len(rawItems) <= maxItems {
		return bodyBytes
	}

	leading := 0
	for leading < len(rawItems) &&
		strings.TrimSpace(rawItems[leading].Get("type").String()) == "message" &&
		strings.TrimSpace(rawItems[leading].Get("role").String()) == "developer" {
		leading++
	}
	tailBudget := maxItems - leading
	if tailBudget <= 0 {
		return bodyBytes
	}
	start := len(rawItems) - tailBudget
	if start < leading {
		return bodyBytes
	}
	start = claudeResponsesTailReplayStartIndex(rawItems, start, leading)
	if start <= leading {
		return bodyBytes
	}

	trimmed := make([]interface{}, 0, leading+len(rawItems)-start)
	for _, item := range rawItems[:leading] {
		var decoded interface{}
		if err := json.Unmarshal([]byte(item.Raw), &decoded); err != nil {
			return bodyBytes
		}
		trimmed = append(trimmed, decoded)
	}
	for _, item := range rawItems[start:] {
		var decoded interface{}
		if err := json.Unmarshal([]byte(item.Raw), &decoded); err != nil {
			return bodyBytes
		}
		trimmed = append(trimmed, decoded)
	}
	if len(trimmed) >= len(rawItems) {
		return bodyBytes
	}
	updated, err := sjson.SetBytes(bodyBytes, "input", trimmed)
	if err != nil {
		return bodyBytes
	}
	return updated
}

func claudeResponsesTailReplayStartIndex(rawItems []gjson.Result, start int, minStart int) int {
	if start < minStart {
		start = minStart
	}
	for start > minStart &&
		isClaudeResponsesToolResultImageMessage(rawItems[start]) &&
		strings.TrimSpace(rawItems[start-1].Get("type").String()) == "function_call_output" {
		start--
	}

	requiredCallIDs := make(map[string]bool)
	for _, item := range rawItems[start:] {
		if strings.TrimSpace(item.Get("type").String()) != "function_call_output" {
			continue
		}
		if callID := strings.TrimSpace(item.Get("call_id").String()); callID != "" {
			requiredCallIDs[callID] = true
		}
	}
	if len(requiredCallIDs) == 0 {
		return start
	}
	for i := start - 1; i >= minStart; i-- {
		if strings.TrimSpace(rawItems[i].Get("type").String()) != "function_call" {
			continue
		}
		callID := strings.TrimSpace(rawItems[i].Get("call_id").String())
		if callID == "" {
			callID = strings.TrimSpace(rawItems[i].Get("id").String())
		}
		if !requiredCallIDs[callID] {
			continue
		}
		start = i
		delete(requiredCallIDs, callID)
		if len(requiredCallIDs) == 0 {
			break
		}
	}
	return start
}

func claudeResponsesLatestTurnStartIndex(rawItems []gjson.Result) int {
	if len(rawItems) == 0 {
		return 0
	}

	start := len(rawItems)
	for i := len(rawItems) - 1; i >= 0; {
		for i >= 0 && isClaudeResponsesToolResultImageMessage(rawItems[i]) {
			i--
		}
		if i < 0 || strings.TrimSpace(rawItems[i].Get("type").String()) != "function_call_output" {
			break
		}
		for i >= 0 && strings.TrimSpace(rawItems[i].Get("type").String()) == "function_call_output" {
			i--
		}
		start = i + 1
	}
	if start < len(rawItems) {
		return start
	}
	return len(rawItems) - 1
}

func isClaudeResponsesToolResultImageMessage(item gjson.Result) bool {
	if strings.TrimSpace(item.Get("type").String()) != "message" ||
		strings.TrimSpace(item.Get("role").String()) != "user" {
		return false
	}
	content := item.Get("content")
	if !content.IsArray() {
		return false
	}
	parts := content.Array()
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		if strings.TrimSpace(part.Get("type").String()) != "input_image" {
			return false
		}
	}
	return true
}

func (prs *ProviderRelayService) getProviderRequestPlan(
	plans map[string]providerRequestPlan,
	provider Provider,
	bodyBytes []byte,
	endpoint string,
	requestedModel string,
) (providerRequestPlan, error) {
	if plans != nil {
		if plan, ok := plans[providerRefFromProvider(provider)]; ok {
			if resolveClaudeAPIFormat(provider) == claudeAPIFormatOpenAIResponse &&
				strings.TrimSpace(plan.ContinuationSessionKey) != "" &&
				strings.TrimSpace(plan.PreviousResponseID) != "" &&
				prs.isClaudeResponsesContinuationDisabled(provider, plan.ContinuationSessionKey) {
				return prs.buildProviderRequestPlan(provider, bodyBytes, endpoint, requestedModel)
			}
			return plan, nil
		}
	}
	return prs.buildProviderRequestPlan(provider, bodyBytes, endpoint, requestedModel)
}

func displayModelForLog(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return "<empty>"
	}
	return model
}

func buildGeminiRequestBody(bodyBytes []byte, provider GeminiProvider) ([]byte, error) {
	if len(provider.RequestBodyOverrides) == 0 {
		return bodyBytes, nil
	}
	return ApplyRequestBodyOverrides(bodyBytes, provider.RequestBodyOverrides)
}

// geminiProxyHandler 处理 Gemini API 请求（支持 Level 分组降级和黑名单）
func (prs *ProviderRelayService) geminiProxyHandler(apiVersion string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取完整路径（例如 /v1beta/models/gemini-2.5-pro:generateContent）
		fullPath := c.Param("any")
		endpoint := apiVersion + fullPath

		// 保留查询参数（如 ?alt=sse, ?key= 等）
		query := c.Request.URL.RawQuery
		if query != "" {
			endpoint = endpoint + "?" + query
		}

		fmt.Printf("[Gemini] 收到请求: %s\n", endpoint)

		// 读取请求体
		var bodyBytes []byte
		if c.Request.Body != nil {
			data, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
				return
			}
			bodyBytes = data
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// 判断是否为流式请求
		isStream := strings.Contains(endpoint, ":streamGenerateContent") || strings.Contains(query, "alt=sse")
		requestedModel := extractGeminiModelFromEndpoint(endpoint)

		// 加载 Gemini providers
		providers := prs.geminiService.GetProviders()
		if len(providers) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "no gemini providers configured"})
			return
		}

		// 1. 过滤可用的 providers（启用 + BaseURL 配置 + 未被拉黑）
		var activeProviders []GeminiProvider
		for _, p := range providers {
			if !p.Enabled || p.BaseURL == "" {
				continue
			}
			// 检查黑名单
			if isBlacklisted, until := prs.blacklistService.IsBlacklistedByID("gemini", providerRefFromGeminiProvider(p), p.Name); isBlacklisted {
				fmt.Printf("[Gemini] ⛔ Provider %s 已拉黑，过期时间: %v\n", p.Name, until.Format("15:04:05"))
				continue
			}
			// Level 默认值处理
			if p.Level <= 0 {
				p.Level = 1
			}
			activeProviders = append(activeProviders, p)
		}

		if len(activeProviders) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "no active gemini provider (all disabled or blacklisted)"})
			return
		}

		// 2. 按 Level 分组
		levelGroups := make(map[int][]GeminiProvider)
		for _, p := range activeProviders {
			levelGroups[p.Level] = append(levelGroups[p.Level], p)
		}

		// 获取排序后的 Level 列表
		var sortedLevels []int
		for level := range levelGroups {
			sortedLevels = append(sortedLevels, level)
		}
		sort.Ints(sortedLevels)

		fmt.Printf("[Gemini] 共 %d 个 Level 分组: %v\n", len(sortedLevels), sortedLevels)

		// 请求日志
		start := time.Now()
		capturePayloadEnabled, sanitizePayloadEnabled := prs.resolveRequestLogPayloadCaptureAndSanitization()
		requestLog := &ReqeustLog{
			Platform:         "gemini",
			RequestedModel:   requestedModel,
			IsStream:         isStream,
			CapturePayload:   capturePayloadEnabled,
			SanitizePayload:  sanitizePayloadEnabled,
			InputTokens:      0,
			OutputTokens:     0,
			ProviderAuthType: "",
			RequestStartedAt: start,
		}
		captureRequestLogRequestBody(requestLog, bodyBytes)
		pricingSnapshot := (*modelpricing.Service)(nil)
		if prs != nil && prs.modelPricing != nil {
			pricingSnapshot = prs.modelPricing.Service()
		}
		if pricingSnapshot == nil {
			if svc, err := modelpricing.DefaultService(); err == nil {
				pricingSnapshot = svc
			}
		}

		// 保存日志的 defer
		defer func() {
			requestLog.DurationSec = time.Since(start).Seconds()
			normalizeRequestLogCacheCreateTokens(requestLog)
			normalizeRequestLogInputTokens(requestLog)
			costResult := calculateRequestLogCost(
				prs.providerService,
				pricingSnapshot,
				requestLog.ProviderAPIURL,
				requestLog.ProviderAPIKey,
				requestLog.ProviderAuthType,
				requestLog.ResponseModel,
				requestLog.Model,
				requestLog.RequestedModel,
				requestLog.InputTokens,
				requestLog.OutputTokens,
				requestLog.ReasoningTokens,
				requestLog.CacheCreateTokens,
				requestLog.Ephemeral5mTokens,
				requestLog.Ephemeral1hTokens,
				requestLog.CacheReadTokens,
			)
			applyRequestLogCostResult(requestLog, costResult)
			prepareRequestLogPayloadForPersistence(requestLog)
			if GlobalDBQueueLogs == nil {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = GlobalDBQueueLogs.ExecBatchCtx(ctx, `
					INSERT INTO request_log (
						platform, model, requested_model, response_model, provider_id, provider, http_code,
						input_tokens, output_tokens, cache_create_tokens, ephemeral_5m_tokens, ephemeral_1h_tokens, cache_read_tokens,
						reasoning_tokens, is_stream, duration_sec, first_token_sec, total_cost, group_multiplier, price_source,
						input_cost, output_cost, reasoning_cost, cache_create_cost, cache_read_cost,
						ephemeral_5m_cost, ephemeral_1h_cost, has_pricing, matched_pricing_model,
						provider_pricing_available, provider_quota_type, provider_input_usd_per_m, provider_output_usd_per_m,
						provider_per_call_unified, provider_per_call_input, provider_per_call_output,
						provider_per_call_unified_set, provider_per_call_input_set, provider_per_call_output_set,
						request_body, response_body, request_body_truncated, response_body_truncated, payload_bytes, payload_captured
					) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				`,
				requestLog.Platform, requestLog.Model, requestLog.RequestedModel, requestLog.ResponseModel, requestLog.ProviderID, requestLog.Provider, requestLog.HttpCode,
				requestLog.InputTokens, requestLog.OutputTokens, requestLog.CacheCreateTokens, requestLog.Ephemeral5mTokens, requestLog.Ephemeral1hTokens,
				requestLog.CacheReadTokens, requestLog.ReasoningTokens,
				boolToInt(requestLog.IsStream), requestLog.DurationSec, requestLog.FirstTokenSec, requestLog.TotalCost, requestLog.GroupMultiplier, requestLog.PriceSource,
				requestLog.InputCost, requestLog.OutputCost, requestLog.ReasoningCost, requestLog.CacheCreateCost, requestLog.CacheReadCost,
				requestLog.Ephemeral5mCost, requestLog.Ephemeral1hCost, boolToInt(requestLog.HasPricing), requestLog.MatchedPricingModel,
				boolToInt(requestLog.ProviderPricingAvailable), requestLog.ProviderQuotaType, requestLog.ProviderInputUSDPerM, requestLog.ProviderOutputUSDPerM,
				requestLog.ProviderPerCallUnified, requestLog.ProviderPerCallInput, requestLog.ProviderPerCallOutput,
				boolToInt(requestLog.ProviderPerCallUnifiedSet), boolToInt(requestLog.ProviderPerCallInputSet), boolToInt(requestLog.ProviderPerCallOutputSet),
				requestLog.RequestBody, requestLog.ResponseBody, boolToInt(requestLog.RequestBodyTruncated), boolToInt(requestLog.ResponseBodyTruncated), requestLog.PayloadBytes, boolToInt(requestLog.PayloadCaptured),
			)
		}()

		// 获取拉黑功能开关状态
		blacklistEnabled := prs.blacklistService.ShouldUseFixedMode()

		// 【拉黑模式】：同 Provider 重试直到被拉黑，然后切换到下一个 Provider
		if blacklistEnabled {
			fmt.Printf("[Gemini] 🔒 拉黑模式已开启（同 Provider 重试到拉黑再切换）\n")

			// 获取重试配置
			retryConfig := prs.blacklistService.GetRetryConfig()
			maxRetryPerProvider := retryConfig.FailureThreshold
			retryWaitSeconds := retryConfig.RetryWaitSeconds
			fmt.Printf("[Gemini] 重试配置: 每 Provider 最多 %d 次重试，间隔 %d 秒\n",
				maxRetryPerProvider, retryWaitSeconds)

			var lastError error
			var lastProvider string
			totalAttempts := 0

			// 遍历所有 Level 和 Provider
			for _, level := range sortedLevels {
				providersInLevel := levelGroups[level]
				fmt.Printf("[Gemini] === 尝试 Level %d（%d 个 provider）===\n", level, len(providersInLevel))

				for _, provider := range providersInLevel {
					// 检查是否已被拉黑（跳过已拉黑的 provider）
					if blacklisted, until := prs.blacklistService.IsBlacklistedByID("gemini", providerRefFromGeminiProvider(provider), provider.Name); blacklisted {
						fmt.Printf("[Gemini] ⏭️ 跳过已拉黑的 Provider: %s (解禁时间: %v)\n", provider.Name, until)
						continue
					}

					// 预填日志
					requestLog.ProviderID = providerRefFromGeminiProvider(provider)
					requestLog.Provider = provider.Name
					requestLog.Model = provider.Model
					requestLog.ProviderAPIURL = provider.BaseURL
					requestLog.ProviderAPIKey = provider.APIKey
					currentBodyBytes, err := buildGeminiRequestBody(bodyBytes, provider)
					if err != nil {
						fmt.Printf("[Gemini][ERROR] 应用 Provider %s 的请求体强制字段失败: %v，跳过此 Provider\n", provider.Name, err)
						lastError = err
						lastProvider = provider.Name
						continue
					}

					// 同 Provider 内重试循环
					for retryCount := 0; retryCount < maxRetryPerProvider; retryCount++ {
						totalAttempts++

						// 再次检查是否已被拉黑（重试过程中可能被拉黑）
						if blacklisted, _ := prs.blacklistService.IsBlacklistedByID("gemini", providerRefFromGeminiProvider(provider), provider.Name); blacklisted {
							fmt.Printf("[Gemini] 🚫 Provider %s 已被拉黑，切换到下一个\n", provider.Name)
							break
						}

						fmt.Printf("[Gemini] [拉黑模式] Provider: %s (Level %d) | 重试 %d/%d\n",
							provider.Name, level, retryCount+1, maxRetryPerProvider)

						ok, err, responseWritten := prs.forwardGeminiRequest(c, &provider, endpoint, currentBodyBytes, isStream, requestLog)
						if ok {
							fmt.Printf("[Gemini] ✓ 成功: %s | 重试 %d 次\n", provider.Name, retryCount+1)
							_ = prs.blacklistService.RecordSuccessByID("gemini", providerRefFromGeminiProvider(provider), provider.Name)
							prs.setLastUsedProvider("gemini", providerRefFromGeminiProvider(provider), provider.Name)
							return
						}

						// 【关键修复】如果响应已写入客户端，不能重试或降级，直接返回
						errorMsg := "未知错误"
						if err != nil {
							errorMsg = err.Error()
						}
						if responseWritten {
							fmt.Printf("[Gemini] ⚠️ 响应已部分写入，无法重试: %s | 错误: %s\n", provider.Name, errorMsg)
							_ = prs.blacklistService.RecordFailureByID("gemini", providerRefFromGeminiProvider(provider), provider.Name)
							return
						}

						// 失败处理
						lastError = err
						lastProvider = provider.Name

						fmt.Printf("[Gemini] ✗ 失败: %s | 重试 %d/%d | 错误: %s\n",
							provider.Name, retryCount+1, maxRetryPerProvider, errorMsg)

						// 记录失败次数（可能触发拉黑）
						_ = prs.blacklistService.RecordFailureByID("gemini", providerRefFromGeminiProvider(provider), provider.Name)

						// 检查是否刚被拉黑
						if blacklisted, _ := prs.blacklistService.IsBlacklistedByID("gemini", providerRefFromGeminiProvider(provider), provider.Name); blacklisted {
							fmt.Printf("[Gemini] 🚫 Provider %s 达到失败阈值，已被拉黑，切换到下一个\n", provider.Name)
							break
						}

						// 等待后重试（除非是最后一次）
						if retryCount < maxRetryPerProvider-1 {
							fmt.Printf("[Gemini] ⏳ 等待 %d 秒后重试...\n", retryWaitSeconds)
							time.Sleep(time.Duration(retryWaitSeconds) * time.Second)
						}
					}
				}
			}

			// 所有 Provider 都失败或被拉黑
			fmt.Printf("[Gemini] 💥 拉黑模式：所有 Provider 都失败或被拉黑（共尝试 %d 次）\n", totalAttempts)

			// 按用户要求：仅在所有重试/降级都失败后，透传最后一次上游错误
			if writeLastUpstreamErrorIfAny(c, lastError) {
				return
			}

			errorMsg := "未知错误"
			if lastError != nil {
				errorMsg = lastError.Error()
			}

			if requestLog.HttpCode == 0 {
				requestLog.HttpCode = http.StatusBadGateway
			}
			c.JSON(http.StatusBadGateway, gin.H{
				"error":         fmt.Sprintf("所有 Provider 都失败或被拉黑，最后尝试: %s - %s", lastProvider, errorMsg),
				"lastProvider":  lastProvider,
				"totalAttempts": totalAttempts,
				"mode":          "blacklist_retry",
				"hint":          "拉黑模式已开启，同 Provider 重试到拉黑再切换。如需立即降级请关闭拉黑功能",
			})
			return
		}

		// 【降级模式】：按 Level 顺序尝试所有 provider
		roundRobinEnabled := prs.isRoundRobinEnabled()
		if roundRobinEnabled {
			fmt.Printf("[Gemini] 🔄 降级模式 + 轮询负载均衡\n")
		} else {
			fmt.Printf("[Gemini] 🔄 降级模式（顺序降级）\n")
		}

		var lastError error
		for _, level := range sortedLevels {
			providersInLevel := levelGroups[level]

			// 如果启用轮询，对同 Level 的 providers 进行轮询排序
			if roundRobinEnabled {
				providersInLevel = prs.roundRobinOrderGemini(level, providersInLevel)
			}

			fmt.Printf("[Gemini] === 尝试 Level %d（%d 个 provider）===\n", level, len(providersInLevel))

			for idx, provider := range providersInLevel {
				fmt.Printf("[Gemini]   [%d/%d] Provider: %s\n", idx+1, len(providersInLevel), provider.Name)

				// 预填日志，失败也能落库
				requestLog.ProviderID = providerRefFromGeminiProvider(provider)
				requestLog.Provider = provider.Name
				requestLog.Model = provider.Model
				requestLog.ProviderAPIURL = provider.BaseURL
				requestLog.ProviderAPIKey = provider.APIKey
				currentBodyBytes, err := buildGeminiRequestBody(bodyBytes, provider)
				if err != nil {
					fmt.Printf("[Gemini][ERROR] 应用 Provider %s 的请求体强制字段失败: %v\n", provider.Name, err)
					lastError = err
					continue
				}

				ok, err, responseWritten := prs.forwardGeminiRequest(c, &provider, endpoint, currentBodyBytes, isStream, requestLog)
				if ok {
					_ = prs.blacklistService.RecordSuccessByID("gemini", providerRefFromGeminiProvider(provider), provider.Name)
					// 记录最后使用的供应商
					prs.setLastUsedProvider("gemini", providerRefFromGeminiProvider(provider), provider.Name)
					fmt.Printf("[Gemini] ✓ 请求完成 | Provider: %s | 总耗时: %.2fs\n", provider.Name, time.Since(start).Seconds())
					return // 成功，退出
				}

				// 【关键修复】如果响应已写入客户端，不能降级到其他 provider，直接返回
				errorMsg := "未知错误"
				if err != nil {
					errorMsg = err.Error()
				}
				if responseWritten {
					fmt.Printf("[Gemini] ⚠️ 响应已部分写入，无法降级: %s | 错误: %s\n", provider.Name, errorMsg)
					_ = prs.blacklistService.RecordFailureByID("gemini", providerRefFromGeminiProvider(provider), provider.Name)
					return
				}

				// 失败，记录并继续
				lastError = err
				fmt.Printf("[Gemini] ✗ 失败: %s | 错误: %s\n", provider.Name, errorMsg)
				_ = prs.blacklistService.RecordFailureByID("gemini", providerRefFromGeminiProvider(provider), provider.Name)
			}

			fmt.Printf("[Gemini] Level %d 的所有 %d 个 provider 均失败，尝试下一 Level\n", level, len(providersInLevel))
		}

		// 所有 Level 都失败：优先透传最后一次上游错误，否则返回 502 聚合信息
		if writeLastUpstreamErrorIfAny(c, lastError) {
			return
		}

		errorMsg := "未知错误"
		if lastError != nil {
			errorMsg = lastError.Error()
		}

		if requestLog.HttpCode == 0 {
			requestLog.HttpCode = http.StatusBadGateway
		}
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "all gemini providers failed",
			"details": errorMsg,
		})
		fmt.Printf("[Gemini] ✗ 所有 provider 均失败 | 最后错误: %s\n", errorMsg)
	}
}

// extractGeminiModelFromEndpoint 从 Gemini API endpoint 中提取模型名
// 例如 "/v1beta/models/gemini-2.5-pro:generateContent?alt=sse" -> "gemini-2.5-pro"
func extractGeminiModelFromEndpoint(endpoint string) string {
	if endpoint == "" {
		return ""
	}
	// 移除查询参数
	if qIdx := strings.Index(endpoint, "?"); qIdx >= 0 {
		endpoint = endpoint[:qIdx]
	}
	// 查找 models/ 后面的部分
	idx := strings.Index(endpoint, "models/")
	if idx == -1 {
		return ""
	}
	rest := endpoint[idx+len("models/"):]
	if rest == "" {
		return ""
	}
	// 移除动作部分（如 :generateContent, :streamGenerateContent）
	if colonIdx := strings.Index(rest, ":"); colonIdx >= 0 {
		rest = rest[:colonIdx]
	}
	return strings.TrimSpace(rest)
}

// forwardGeminiRequest 转发 Gemini 请求到指定 provider
// 返回 (成功, 错误对象, 是否已写入响应)
// 【重要】当 responseWritten=true 时，调用方不得重试或降级，因为响应头/数据已发送给客户端
func (prs *ProviderRelayService) forwardGeminiRequest(
	c *gin.Context,
	provider *GeminiProvider,
	endpoint string,
	bodyBytes []byte,
	isStream bool,
	requestLog *ReqeustLog,
) (success bool, err error, responseWritten bool) {
	providerStart := time.Now()

	// 构建目标 URL
	targetURL := strings.TrimSuffix(provider.BaseURL, "/") + endpoint

	// 预先填充日志，保证失败也能记录 provider 和模型
	requestLog.ProviderID = providerRefFromGeminiProvider(*provider)
	requestLog.Provider = provider.Name
	captureRequestLogRequestBody(requestLog, bodyBytes)
	resetRequestLogResponseBody(requestLog)
	// 【修复】每次尝试开始前重置 HttpCode，避免重试时沿用上一次的状态码
	requestLog.HttpCode = 0
	// 优先从 endpoint 提取模型名（如 gemini-2.5-pro），否则回退到 provider.Model
	if extractedModel := extractGeminiModelFromEndpoint(endpoint); extractedModel != "" {
		requestLog.Model = extractedModel
		if strings.TrimSpace(requestLog.RequestedModel) == "" {
			requestLog.RequestedModel = extractedModel
		}
	} else {
		requestLog.Model = provider.Model
	}

	// 创建 HTTP 请求
	req, err := http.NewRequest("POST", targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return false, fmt.Errorf("创建请求失败: %w", err), false
	}

	// 复制请求头
	for key, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// 设置 API Key
	if provider.APIKey != "" {
		req.Header.Set("x-goog-api-key", provider.APIKey)
	}

	// 发送请求
	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	providerDuration := time.Since(providerStart).Seconds()

	if err != nil {
		fmt.Printf("[Gemini]   ✗ 失败: %s | 错误: %v | 耗时: %.2fs\n", provider.Name, err, providerDuration)
		return false, fmt.Errorf("请求失败: %w", err), false
	}
	defer resp.Body.Close()

	// 先记录上游状态码，失败场景也能落库
	requestLog.HttpCode = resp.StatusCode

	// 检查响应状态
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errorBody, _ := io.ReadAll(resp.Body)
		setRequestLogResponseBody(requestLog, errorBody)
		fmt.Printf("[Gemini]   ✗ 失败: %s | HTTP %d | 耗时: %.2fs\n", provider.Name, resp.StatusCode, providerDuration)
		return false, newUpstreamErrorResponse(resp.StatusCode, resp.Header.Get("Content-Type"), resp.Header, errorBody), false
	}

	fmt.Printf("[Gemini]   ✓ 连接成功: %s | HTTP %d | 耗时: %.2fs\n", provider.Name, resp.StatusCode, providerDuration)

	// 处理响应
	if isStream {
		// 流式模式：先写 header 再流式传输
		for key, values := range resp.Header {
			for _, value := range values {
				c.Header(key, value)
			}
		}
		c.Status(resp.StatusCode)
		c.Writer.Flush()
		// 【重要】从 Flush() 开始，响应头已写入客户端，任何失败都不能重试
		copyErr := streamGeminiResponseWithHook(resp.Body, c.Writer, requestLog)
		if copyErr != nil {
			fmt.Printf("[Gemini]   ⚠️ 流式传输中断: %s | 错误: %v\n", provider.Name, copyErr)
			// 流式传输中断：已写入部分响应，客户端会收到不完整数据
			return false, fmt.Errorf("流式传输中断: %w", copyErr), true
		}
	} else {
		// 非流式模式：先读完 body 再写 header（允许读取失败时重试）
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			fmt.Printf("[Gemini]   ⚠️ 读取响应失败: %s | 错误: %v\n", provider.Name, readErr)
			// 【修复】此时 header 尚未写入客户端，可以重试/降级
			return false, fmt.Errorf("读取响应失败: %w", readErr), false
		}
		setRequestLogResponseBody(requestLog, body)
		// 解析 Gemini 用量数据
		parseGeminiUsageMetadata(body, requestLog)
		// 读取成功后再写 header 和 body
		for key, values := range resp.Header {
			for _, value := range values {
				c.Header(key, value)
			}
		}
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}

	return true, nil, true
}

// parseGeminiUsageMetadata 从 Gemini 非流式响应中提取用量，填充 request_log
// 复用 mergeGeminiUsageMetadata 统一解析逻辑
func parseGeminiUsageMetadata(body []byte, reqLog *ReqeustLog) {
	if len(body) == 0 || reqLog == nil {
		return
	}
	updateResponseModelFromPayload(string(body), reqLog)
	updateFirstTokenFromPayload(string(body), reqLog)
	usage := gjson.GetBytes(body, "usageMetadata")
	if !usage.Exists() {
		return
	}
	mergeGeminiUsageMetadata(usage, reqLog)
}

// customCliProxyHandler 处理自定义 CLI 工具的 API 请求
// 路由格式: /custom/:toolId/v1/messages
// toolId 用于区分不同的 CLI 工具，对应 provider kind 为 "custom:{toolId}"
func (prs *ProviderRelayService) customCliProxyHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 URL 参数提取 toolId
		toolId := c.Param("toolId")
		if toolId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "toolId is required"})
			return
		}

		// 构建 provider kind（格式: "custom:{toolId}"）
		kind := "custom:" + toolId
		endpoint := "/v1/messages"

		fmt.Printf("[CustomCLI] 收到请求: toolId=%s, kind=%s\n", toolId, kind)

		// 读取请求体
		var bodyBytes []byte
		if c.Request.Body != nil {
			data, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
				return
			}
			bodyBytes = data
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		isStream := gjson.GetBytes(bodyBytes, "stream").Bool()
		requestedModel := gjson.GetBytes(bodyBytes, "model").String()

		if requestedModel == "" {
			fmt.Printf("[CustomCLI][WARN] 请求未指定模型名，无法执行模型智能降级\n")
		}

		// 加载该 CLI 工具的 providers
		providers, err := prs.providerService.LoadProviders(kind)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to load providers for %s: %v", kind, err)})
			return
		}

		// 过滤可用的 providers
		active := make([]Provider, 0, len(providers))
		requestPlans := make(map[string]providerRequestPlan, len(providers))
		skippedCount := 0
		for _, provider := range providers {
			if !provider.Enabled || provider.APIURL == "" || provider.APIKey == "" {
				continue
			}

			if errs := provider.ValidateConfiguration(); len(errs) > 0 {
				fmt.Printf("[CustomCLI][WARN] Provider %s 配置验证失败，已自动跳过: %v\n", provider.Name, errs)
				skippedCount++
				continue
			}

			// 黑名单检查
			if isBlacklisted, until := prs.blacklistService.IsBlacklistedByID(kind, providerRefFromProvider(provider), provider.Name); isBlacklisted {
				fmt.Printf("[CustomCLI] ⛔ Provider %s 已拉黑，过期时间: %v\n", provider.Name, until.Format("15:04:05"))
				skippedCount++
				continue
			}

			plan, err := prs.buildProviderRequestPlan(provider, bodyBytes, endpoint, requestedModel)
			if err != nil {
				fmt.Printf("[CustomCLI][WARN] Provider %s 请求体预处理失败，已自动跳过: %v\n", provider.Name, err)
				skippedCount++
				continue
			}

			if !provider.IsResolvedModelSupported(requestedModel, plan.EffectiveModel) {
				fmt.Printf("[CustomCLI][INFO] Provider %s 不支持最终模型 %s（原始请求模型: %s），已跳过\n",
					provider.Name,
					displayModelForLog(plan.EffectiveModel),
					displayModelForLog(requestedModel),
				)
				skippedCount++
				continue
			}

			requestPlans[providerRefFromProvider(provider)] = plan
			active = append(active, provider)
		}

		if len(active) == 0 {
			if requestedModel != "" {
				c.JSON(http.StatusNotFound, gin.H{
					"error": fmt.Sprintf("没有可用的 provider 支持模型 '%s'（已跳过 %d 个不兼容的 provider）", requestedModel, skippedCount),
				})
			} else {
				c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("no providers available for %s", kind)})
			}
			return
		}

		fmt.Printf("[CustomCLI][INFO] 找到 %d 个可用的 provider（已过滤 %d 个）：", len(active), skippedCount)
		for _, p := range active {
			fmt.Printf("%s ", p.Name)
		}
		fmt.Println()

		// 按 Level 分组
		levelGroups := make(map[int][]Provider)
		for _, provider := range active {
			level := provider.Level
			if level <= 0 {
				level = 1
			}
			levelGroups[level] = append(levelGroups[level], provider)
		}

		levels := make([]int, 0, len(levelGroups))
		for level := range levelGroups {
			levels = append(levels, level)
		}
		sort.Ints(levels)

		fmt.Printf("[CustomCLI][INFO] 共 %d 个 Level 分组：%v\n", len(levels), levels)

		query := flattenQuery(c.Request.URL.Query())
		clientHeaders := cloneHeaders(c.Request.Header)

		// 获取拉黑功能开关状态
		blacklistEnabled := prs.blacklistService.ShouldUseFixedMode()

		// 【拉黑模式】：同 Provider 重试直到被拉黑，然后切换到下一个 Provider
		if blacklistEnabled {
			fmt.Printf("[CustomCLI][INFO] 🔒 拉黑模式已开启（同 Provider 重试到拉黑再切换）\n")

			// 获取重试配置
			retryConfig := prs.blacklistService.GetRetryConfig()
			maxRetryPerProvider := retryConfig.FailureThreshold
			retryWaitSeconds := retryConfig.RetryWaitSeconds
			fmt.Printf("[CustomCLI][INFO] 重试配置: 每 Provider 最多 %d 次重试，间隔 %d 秒\n",
				maxRetryPerProvider, retryWaitSeconds)

			var lastError error
			var lastProvider string
			totalAttempts := 0

			// 遍历所有 Level 和 Provider
			for _, level := range levels {
				providersInLevel := levelGroups[level]
				fmt.Printf("[CustomCLI][INFO] === 尝试 Level %d（%d 个 provider）===\n", level, len(providersInLevel))

				for _, provider := range providersInLevel {
					// 检查是否已被拉黑（跳过已拉黑的 provider）
					if blacklisted, until := prs.blacklistService.IsBlacklistedByID(kind, providerRefFromProvider(provider), provider.Name); blacklisted {
						fmt.Printf("[CustomCLI][INFO] ⏭️ 跳过已拉黑的 Provider: %s (解禁时间: %v)\n", provider.Name, until)
						continue
					}

					plan, err := prs.getProviderRequestPlan(requestPlans, provider, bodyBytes, endpoint, requestedModel)
					if err != nil {
						fmt.Printf("[CustomCLI][ERROR] Provider %s 请求体预处理失败: %v，跳过此 Provider\n", provider.Name, err)
						continue
					}

					// 同 Provider 内重试循环
					for retryCount := 0; retryCount < maxRetryPerProvider; retryCount++ {
						totalAttempts++

						// 再次检查是否已被拉黑（重试过程中可能被拉黑）
						if blacklisted, _ := prs.blacklistService.IsBlacklistedByID(kind, providerRefFromProvider(provider), provider.Name); blacklisted {
							fmt.Printf("[CustomCLI][INFO] 🚫 Provider %s 已被拉黑，切换到下一个\n", provider.Name)
							break
						}

						fmt.Printf("[CustomCLI][INFO] [拉黑模式] Provider: %s (Level %d) | 重试 %d/%d | Model: %s\n",
							provider.Name, level, retryCount+1, maxRetryPerProvider, plan.EffectiveModel)

						startTime := time.Now()
						ok, err := prs.forwardRequestWithPlan(c, kind, provider, plan.EffectiveEndpoint, query, clientHeaders, plan.BodyBytes, isStream, plan.EffectiveModel, requestedModel, plan)
						duration := time.Since(startTime)

						if ok {
							fmt.Printf("[CustomCLI][INFO] ✓ 成功: %s | 重试 %d 次 | 耗时: %.2fs\n",
								provider.Name, retryCount+1, duration.Seconds())
							if err := prs.blacklistService.RecordSuccessByID(kind, providerRefFromProvider(provider), provider.Name); err != nil {
								fmt.Printf("[CustomCLI][WARN] 清零失败计数失败: %v\n", err)
							}
							prs.setLastUsedProvider(kind, providerRefFromProvider(provider), provider.Name)
							return
						}

						// 失败处理
						lastError = err
						lastProvider = provider.Name

						errorMsg := "未知错误"
						if err != nil {
							errorMsg = err.Error()
						}
						if errors.Is(err, errResponseStarted) {
							fmt.Printf("[CustomCLI][WARN] ⚠️ 响应已部分写入，无法重试: %s | 错误: %s | 耗时: %.2fs\n",
								provider.Name, errorMsg, duration.Seconds())
							if errors.Is(err, errClientAbort) {
								fmt.Printf("[CustomCLI][INFO] 客户端中断，停止重试\n")
								return
							}
							if err := prs.blacklistService.RecordFailureByID(kind, providerRefFromProvider(provider), provider.Name); err != nil {
								fmt.Printf("[CustomCLI][ERROR] 记录失败到黑名单失败: %v\n", err)
							}
							return
						}
						fmt.Printf("[CustomCLI][WARN] ✗ 失败: %s | 重试 %d/%d | 错误: %s | 耗时: %.2fs\n",
							provider.Name, retryCount+1, maxRetryPerProvider, errorMsg, duration.Seconds())

						// 客户端中断不计入失败次数，直接返回
						if errors.Is(err, errClientAbort) {
							fmt.Printf("[CustomCLI][INFO] 客户端中断，停止重试\n")
							return
						}

						// 记录失败次数（可能触发拉黑）
						if err := prs.blacklistService.RecordFailureByID(kind, providerRefFromProvider(provider), provider.Name); err != nil {
							fmt.Printf("[CustomCLI][ERROR] 记录失败到黑名单失败: %v\n", err)
						}

						// 检查是否刚被拉黑
						if blacklisted, _ := prs.blacklistService.IsBlacklistedByID(kind, providerRefFromProvider(provider), provider.Name); blacklisted {
							fmt.Printf("[CustomCLI][INFO] 🚫 Provider %s 达到失败阈值，已被拉黑，切换到下一个\n", provider.Name)
							break
						}

						// 等待后重试（除非是最后一次）
						if retryCount < maxRetryPerProvider-1 {
							fmt.Printf("[CustomCLI][INFO] ⏳ 等待 %d 秒后重试...\n", retryWaitSeconds)
							time.Sleep(time.Duration(retryWaitSeconds) * time.Second)
						}
					}
				}
			}

			// 所有 Provider 都失败或被拉黑
			fmt.Printf("[CustomCLI][ERROR] 💥 拉黑模式：所有 Provider 都失败或被拉黑（共尝试 %d 次）\n", totalAttempts)

			// 按用户要求：仅在所有重试/降级都失败后，透传最后一次上游错误
			if writeLastUpstreamErrorIfAny(c, lastError) {
				return
			}

			errorMsg := "未知错误"
			if lastError != nil {
				errorMsg = lastError.Error()
			}
			c.JSON(http.StatusBadGateway, gin.H{
				"error":         fmt.Sprintf("所有 Provider 都失败或被拉黑，最后尝试: %s - %s", lastProvider, errorMsg),
				"lastProvider":  lastProvider,
				"totalAttempts": totalAttempts,
				"mode":          "blacklist_retry",
				"hint":          "拉黑模式已开启，同 Provider 重试到拉黑再切换。如需立即降级请关闭拉黑功能",
			})
			return
		}

		// 【降级模式】：失败自动尝试下一个 provider
		roundRobinEnabled := prs.isRoundRobinEnabled()
		if roundRobinEnabled {
			fmt.Printf("[CustomCLI][INFO] 🔄 降级模式 + 轮询负载均衡\n")
		} else {
			fmt.Printf("[CustomCLI][INFO] 🔄 降级模式（顺序降级）\n")
		}

		var lastError error
		var lastProvider string
		var lastDuration time.Duration
		totalAttempts := 0

		for _, level := range levels {
			providersInLevel := levelGroups[level]

			// 如果启用轮询，对同 Level 的 providers 进行轮询排序
			if roundRobinEnabled {
				providersInLevel = prs.roundRobinOrder(kind, level, providersInLevel)
			}

			fmt.Printf("[CustomCLI][INFO] === 尝试 Level %d（%d 个 provider）===\n", level, len(providersInLevel))

			for i, provider := range providersInLevel {
				totalAttempts++

				plan, err := prs.getProviderRequestPlan(requestPlans, provider, bodyBytes, endpoint, requestedModel)
				if err != nil {
					fmt.Printf("[CustomCLI][ERROR] Provider %s 请求体预处理失败: %v\n", provider.Name, err)
					continue
				}

				fmt.Printf("[CustomCLI][INFO]   [%d/%d] Provider: %s | Model: %s\n", i+1, len(providersInLevel), provider.Name, plan.EffectiveModel)
				// 获取有效的端点（用户配置优先）

				startTime := time.Now()
				ok, err := prs.forwardRequestWithPlan(c, kind, provider, plan.EffectiveEndpoint, query, clientHeaders, plan.BodyBytes, isStream, plan.EffectiveModel, requestedModel, plan)
				duration := time.Since(startTime)

				if ok {
					fmt.Printf("[CustomCLI][INFO]   ✓ Level %d 成功: %s | 耗时: %.2fs\n", level, provider.Name, duration.Seconds())
					if err := prs.blacklistService.RecordSuccessByID(kind, providerRefFromProvider(provider), provider.Name); err != nil {
						fmt.Printf("[CustomCLI][WARN] 清零失败计数失败: %v\n", err)
					}
					prs.setLastUsedProvider(kind, providerRefFromProvider(provider), provider.Name)
					return
				}

				lastError = err
				lastProvider = provider.Name
				lastDuration = duration

				errorMsg := "未知错误"
				if err != nil {
					errorMsg = err.Error()
				}
				if errors.Is(err, errResponseStarted) {
					fmt.Printf("[CustomCLI][WARN]   ⚠️ 响应已部分写入，无法降级: %s | 错误: %s | 耗时: %.2fs\n",
						provider.Name, errorMsg, duration.Seconds())
					if errors.Is(err, errClientAbort) {
						fmt.Printf("[CustomCLI][INFO] 客户端中断，跳过失败计数: %s\n", provider.Name)
						return
					}
					if err := prs.blacklistService.RecordFailureByID(kind, providerRefFromProvider(provider), provider.Name); err != nil {
						fmt.Printf("[CustomCLI][ERROR] 记录失败到黑名单失败: %v\n", err)
					}
					return
				}
				fmt.Printf("[CustomCLI][WARN]   ✗ Level %d 失败: %s | 错误: %s | 耗时: %.2fs\n",
					level, provider.Name, errorMsg, duration.Seconds())

				if errors.Is(err, errClientAbort) {
					fmt.Printf("[CustomCLI][INFO] 客户端中断，跳过失败计数: %s\n", provider.Name)
				} else if err := prs.blacklistService.RecordFailureByID(kind, providerRefFromProvider(provider), provider.Name); err != nil {
					fmt.Printf("[CustomCLI][ERROR] 记录失败到黑名单失败: %v\n", err)
				}

				// 发送切换通知
				if prs.notificationService != nil {
					nextProviderName := ""
					nextProviderID := ""
					if i+1 < len(providersInLevel) {
						nextProvider := providersInLevel[i+1]
						nextProviderName = nextProvider.Name
						nextProviderID = providerRefFromProvider(nextProvider)
					} else {
						for _, nextLevel := range levels {
							if nextLevel > level && len(levelGroups[nextLevel]) > 0 {
								nextProvider := levelGroups[nextLevel][0]
								nextProviderName = nextProvider.Name
								nextProviderID = providerRefFromProvider(nextProvider)
								break
							}
						}
					}
					if nextProviderName != "" {
						prs.notificationService.NotifyProviderSwitch(SwitchNotification{
							FromProviderID: providerRefFromProvider(provider),
							FromProvider:   provider.Name,
							ToProviderID:   nextProviderID,
							ToProvider:     nextProviderName,
							Reason:         errorMsg,
							Platform:       kind,
						})
					}
				}
			}

			fmt.Printf("[CustomCLI][WARN] Level %d 的所有 %d 个 provider 均失败，尝试下一 Level\n", level, len(providersInLevel))
		}

		// 所有 provider 都失败：优先透传最后一次上游错误，否则返回 502 聚合信息
		if writeLastUpstreamErrorIfAny(c, lastError) {
			return
		}

		errorMsg := "未知错误"
		if lastError != nil {
			errorMsg = lastError.Error()
		}
		fmt.Printf("[CustomCLI][ERROR] 所有 %d 个 provider 均失败，最后尝试: %s | 错误: %s\n",
			totalAttempts, lastProvider, errorMsg)

		c.JSON(http.StatusBadGateway, gin.H{
			"error":          fmt.Sprintf("所有 %d 个 provider 均失败，最后错误: %s", totalAttempts, errorMsg),
			"last_provider":  lastProvider,
			"last_duration":  fmt.Sprintf("%.2fs", lastDuration.Seconds()),
			"total_attempts": totalAttempts,
		})
	}
}

// forwardModelsRequest 共享的 /v1/models 请求转发逻辑
// 返回 (selectedProvider, error)
func (prs *ProviderRelayService) forwardModelsRequest(
	c *gin.Context,
	kind string,
	logPrefix string,
) error {
	fmt.Printf("[%s] 收到 /v1/models 请求, kind=%s\n", logPrefix, kind)

	// 加载 providers
	providers, err := prs.providerService.LoadProviders(kind)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load providers"})
		return fmt.Errorf("failed to load providers: %w", err)
	}

	// 过滤可用的 providers（启用 + URL + APIKey）
	var activeProviders []Provider
	for _, provider := range providers {
		if !provider.Enabled || provider.APIURL == "" || provider.APIKey == "" {
			continue
		}

		// 黑名单检查：跳过已拉黑的 provider
		if isBlacklisted, until := prs.blacklistService.IsBlacklistedByID(kind, providerRefFromProvider(provider), provider.Name); isBlacklisted {
			fmt.Printf("[%s] ⛔ Provider %s 已拉黑，过期时间: %v\n", logPrefix, provider.Name, until.Format("15:04:05"))
			continue
		}

		activeProviders = append(activeProviders, provider)
	}

	if len(activeProviders) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no providers available"})
		return fmt.Errorf("no providers available")
	}

	// 按 Level 分组并排序
	levelGroups := make(map[int][]Provider)
	for _, provider := range activeProviders {
		level := provider.Level
		if level <= 0 {
			level = 1
		}
		levelGroups[level] = append(levelGroups[level], provider)
	}

	levels := make([]int, 0, len(levelGroups))
	for level := range levelGroups {
		levels = append(levels, level)
	}
	sort.Ints(levels)

	// 尝试第一个可用的 provider（按 Level 升序）
	var selectedProvider *Provider
	for _, level := range levels {
		if len(levelGroups[level]) > 0 {
			p := levelGroups[level][0]
			selectedProvider = &p
			break
		}
	}

	if selectedProvider == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no providers available"})
		return fmt.Errorf("no providers available after filtering")
	}

	fmt.Printf("[%s] 使用 Provider: %s | URL: %s\n", logPrefix, selectedProvider.Name, selectedProvider.APIURL)

	// 构建目标 URL（拼接 provider 的 APIURL 和 /v1/models）
	targetURL := joinURL(selectedProvider.APIURL, "/v1/models")

	// 创建 HTTP 请求
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建请求失败: %v", err)})
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 复制客户端请求头
	for key, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// 根据认证方式设置请求头（默认 Bearer，与 v2.2.x 保持一致）
	authType := strings.ToLower(strings.TrimSpace(selectedProvider.ConnectivityAuthType))
	switch authType {
	case "x-api-key":
		req.Header.Set("x-api-key", selectedProvider.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	case "", "bearer":
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", selectedProvider.APIKey))
	default:
		headerName := strings.TrimSpace(selectedProvider.ConnectivityAuthType)
		if headerName == "" || strings.EqualFold(headerName, "custom") {
			headerName = "Authorization"
		}
		req.Header.Set(headerName, selectedProvider.APIKey)
	}

	// 设置默认 Accept 头
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[%s] ✗ 请求失败: %s | 错误: %v\n", logPrefix, selectedProvider.Name, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("请求失败: %v", err)})
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("[%s] ✗ 读取响应失败: %s | 错误: %v\n", logPrefix, selectedProvider.Name, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("读取响应失败: %v", err)})
		return fmt.Errorf("failed to read response: %w", err)
	}

	// 复制响应头
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	fmt.Printf("[%s] ✓ 成功: %s | HTTP %d\n", logPrefix, selectedProvider.Name, resp.StatusCode)

	// 返回响应
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
	return nil
}

// modelsHandler 处理 /v1/models 请求（OpenAI-compatible API）
// 将请求转发到第一个可用的 provider 并注入 API Key
func (prs *ProviderRelayService) modelsHandler(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		_ = prs.forwardModelsRequest(c, kind, "Models")
	}
}

// customModelsHandler 处理自定义 CLI 工具的 /v1/models 请求
// 路由格式: /custom/:toolId/v1/models
func (prs *ProviderRelayService) customModelsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 URL 参数提取 toolId
		toolId := c.Param("toolId")
		if toolId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "toolId is required"})
			return
		}

		// 构建 provider kind（格式: "custom:{toolId}"）
		kind := "custom:" + toolId

		_ = prs.forwardModelsRequest(c, kind, "CustomModels")
	}
}
