package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ConnectivityStatus 连通性状态常量（与 relay-pulse 一致）
const (
	StatusAvailable   = 1  // 绿色：可用
	StatusDegraded    = 2  // 黄色：波动
	StatusUnavailable = 0  // 红色：不可用
	StatusMissing     = -1 // 灰色：无数据
)

// SubStatus 细分状态常量
const (
	SubStatusNone            = ""
	SubStatusSlowLatency     = "slow_latency"
	SubStatusRateLimit       = "rate_limit"
	SubStatusServerError     = "server_error"
	SubStatusClientError     = "client_error"
	SubStatusAuthError       = "auth_error"
	SubStatusInvalidRequest  = "invalid_request"
	SubStatusNetworkError    = "network_error"
	SubStatusContentMismatch = "content_mismatch"
)

// ConnectivityResult 连通性测试结果
type ConnectivityResult struct {
	ProviderID   int64     `json:"providerId"`
	ProviderName string    `json:"providerName"`
	Platform     string    `json:"platform"`
	Status       int       `json:"status"`
	SubStatus    string    `json:"subStatus"`
	LatencyMs    int       `json:"latencyMs"`
	LastChecked  time.Time `json:"lastChecked"`
	Message      string    `json:"message,omitempty"`
	HTTPCode     int       `json:"httpCode,omitempty"`
}

// ConnectivityTestService 连通性测试服务
type ConnectivityTestService struct {
	providerService  *ProviderService
	blacklistService *BlacklistService
	settingsService  *SettingsService

	mu      sync.RWMutex
	results map[string]map[int64]*ConnectivityResult // platform -> providerID -> result

	autoTestEnabled bool
	stopChan        chan struct{}
	running         bool

	client *http.Client
}

// NewConnectivityTestService 创建连通性测试服务
func NewConnectivityTestService(
	providerService *ProviderService,
	blacklistService *BlacklistService,
	settingsService *SettingsService,
) *ConnectivityTestService {
	return &ConnectivityTestService{
		providerService:  providerService,
		blacklistService: blacklistService,
		settingsService:  settingsService,
		results: map[string]map[int64]*ConnectivityResult{
			"claude": {},
			"codex":  {},
			"gemini": {},
		},
		autoTestEnabled: false,
		client: &http.Client{
			Timeout: 0,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  true,
				MaxIdleConnsPerHost: 5,
			},
		},
	}
}

// TestProvider 测试单个供应商连通性
func (cts *ConnectivityTestService) TestProvider(ctx context.Context, provider Provider, platform string) *ConnectivityResult {
	result := &ConnectivityResult{
		ProviderID:   provider.ID,
		ProviderName: provider.Name,
		Platform:     platform,
		Status:       StatusUnavailable,
		SubStatus:    SubStatusNone,
		LastChecked:  time.Now(),
	}

	model := cts.getEffectiveModel(&provider, platform)
	endpoint := cts.getEffectiveEndpoint(&provider, platform)
	timeoutMs := cts.getEffectiveTimeout(&provider)
	probeResult, err := executeAvailabilityProbe(ctx, cts.client, &provider, platform, model, endpoint, timeoutMs)
	result.LatencyMs = probeResult.LatencyMs

	if err != nil {
		var buildErr availabilityProbeBuildError
		if errors.As(err, &buildErr) {
			result.Status = StatusUnavailable
			result.SubStatus = SubStatusClientError
			result.Message = cts.truncateMessage(fmt.Sprintf("无法构建测试请求: %v", buildErr))
			return result
		}
		// 检测是否为超时错误 - 超时应视为"慢但可用"（黄色），而非"不可用"（红色）
		// 这样可以避免慢响应的 Provider 被误判为失败而拉黑
		if isTimeoutError(err) {
			result.Status = StatusDegraded
			result.SubStatus = SubStatusSlowLatency
			result.Message = fmt.Sprintf("响应超时 (>%dms)", timeoutMs)
			return result
		}
		// 真正的网络错误（连接失败、DNS 解析失败等）
		result.Status = StatusUnavailable
		result.SubStatus = SubStatusNetworkError
		result.Message = cts.truncateMessage(fmt.Sprintf("网络错误: %v", err))
		return result
	}
	result.HTTPCode = probeResult.HTTPStatusCode
	body := probeResult.ResponseBody

	// 第一阶段：HTTP 状态码 + 延迟判定
	result.Status, result.SubStatus = cts.determineStatus(probeResult.HTTPStatusCode, result.LatencyMs, 5000)

	// 第二阶段：内容校验（仅对成功响应）
	if result.Status != StatusUnavailable {
		result.Status, result.SubStatus = cts.evaluateContent(
			result.Status,
			result.SubStatus,
			body,
			probeResult.Plan.ResponseFormat,
			probeResult.Plan.ExpectedText,
		)
	}

	// 设置错误消息
	if result.Status == StatusUnavailable {
		if result.SubStatus == SubStatusContentMismatch {
			result.Message = cts.truncateMessage(
				buildAvailabilityValidationError(body, probeResult.Plan.ResponseFormat, probeResult.Plan.ExpectedText),
			)
		} else {
			result.Message = cts.truncateMessage(string(body))
		}
	}

	return result
}

func (cts *ConnectivityTestService) getEffectiveModel(provider *Provider, platform string) string {
	return resolveProviderAvailabilityModel(provider, platform)
}

// getEffectiveEndpoint 获取有效端点（含平台默认值）
func (cts *ConnectivityTestService) getEffectiveEndpoint(provider *Provider, platform string) string {
	return resolveProviderAvailabilityEndpoint(provider, platform)
}

func (cts *ConnectivityTestService) getEffectiveTimeout(provider *Provider) int {
	return resolveProviderAvailabilityTimeout(provider)
}

// determineStatus 根据 HTTP 状态码和延迟判定状态
func (cts *ConnectivityTestService) determineStatus(statusCode, latencyMs, slowThresholdMs int) (int, string) {
	// 2xx = 成功
	if statusCode >= 200 && statusCode < 300 {
		if slowThresholdMs > 0 && latencyMs > slowThresholdMs {
			return StatusDegraded, SubStatusSlowLatency
		}
		return StatusAvailable, SubStatusNone
	}

	// 3xx = 重定向，视为正常
	if statusCode >= 300 && statusCode < 400 {
		return StatusAvailable, SubStatusNone
	}

	// 特殊 4xx
	if statusCode == 400 {
		return StatusUnavailable, SubStatusInvalidRequest
	}
	if statusCode == 401 || statusCode == 403 {
		return StatusUnavailable, SubStatusAuthError
	}
	if statusCode == 429 {
		return StatusUnavailable, SubStatusRateLimit
	}

	// 5xx = 服务器错误
	if statusCode >= 500 {
		return StatusUnavailable, SubStatusServerError
	}

	// 其他 4xx
	if statusCode >= 400 {
		return StatusUnavailable, SubStatusClientError
	}

	// 其他异常
	return StatusUnavailable, SubStatusClientError
}

// evaluateContent 内容校验
func (cts *ConnectivityTestService) evaluateContent(baseStatus int, subStatus string, body []byte, responseFormat string, expectedText string) (int, string) {
	if strings.TrimSpace(expectedText) == "" {
		return baseStatus, subStatus
	}

	if baseStatus == StatusUnavailable {
		return baseStatus, subStatus
	}

	if !responseContainsExpectedText(body, responseFormat, expectedText) {
		return StatusUnavailable, SubStatusContentMismatch
	}

	return baseStatus, subStatus
}

// truncateMessage 截断消息（最多 512 字符）
func (cts *ConnectivityTestService) truncateMessage(msg string) string {
	if len(msg) > 512 {
		return msg[:512] + "..."
	}
	return msg
}

// isTimeoutError 检测错误是否为超时类型
// 超时包括：context.DeadlineExceeded、net.Error.Timeout()、以及错误消息中包含 timeout 的情况
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	// 检查 context.DeadlineExceeded
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// 检查 os.ErrDeadlineExceeded（Go 1.15+）
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}

	// 检查 net.Error 接口的 Timeout() 方法
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// 检查错误消息（兜底方案）
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "deadline exceeded") ||
		strings.Contains(errMsg, "context canceled")
}

// TestAll 测试指定平台的所有启用检测的供应商
func (cts *ConnectivityTestService) TestAll(platform string) []ConnectivityResult {
	providers, err := cts.providerService.LoadProviders(platform)
	if err != nil {
		log.Printf("[ConnectivityTest] 加载 %s 供应商失败: %v", platform, err)
		return nil
	}

	var results []ConnectivityResult
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, 5) // 最多 5 个并发

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, provider := range providers {
		// 只测试启用了连通性检测的供应商
		if !provider.AvailabilityMonitorEnabled && !provider.ConnectivityCheck {
			continue
		}

		wg.Add(1)
		go func(p Provider) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := cts.TestProvider(ctx, p, platform)

			// 保存结果
			cts.mu.Lock()
			if cts.results[platform] == nil {
				cts.results[platform] = make(map[int64]*ConnectivityResult)
			}
			cts.results[platform][p.ID] = result
			cts.mu.Unlock()

			// 与拉黑服务联动
			cts.handleBlacklistIntegration(platform, p.ID, p.Name, result)

			mu.Lock()
			results = append(results, *result)
			mu.Unlock()

			log.Printf("[ConnectivityTest] %s/%s: status=%d, subStatus=%s, latency=%dms",
				platform, p.Name, result.Status, result.SubStatus, result.LatencyMs)
		}(provider)
	}

	wg.Wait()
	return results
}

// handleBlacklistIntegration 处理与拉黑服务的联动
func (cts *ConnectivityTestService) handleBlacklistIntegration(platform string, providerID int64, providerName string, result *ConnectivityResult) {
	if cts.blacklistService == nil {
		return
	}
	providerRef := providerRefFromNumericID(providerID, providerName)

	switch result.Status {
	case StatusAvailable:
		// 绿色：调用 RecordSuccess 清零失败计数
		if err := cts.blacklistService.RecordSuccessByID(platform, providerRef, providerName); err != nil {
			log.Printf("[ConnectivityTest] RecordSuccess 失败: %v", err)
		}
	case StatusUnavailable:
		// 红色：调用 RecordFailure 累计失败
		if err := cts.blacklistService.RecordFailureByID(platform, providerRef, providerName); err != nil {
			log.Printf("[ConnectivityTest] RecordFailure 失败: %v", err)
		}
	case StatusDegraded:
		// 黄色：不操作，避免误判
	}
}

// GetResults 获取指定平台的测试结果
func (cts *ConnectivityTestService) GetResults(platform string) []ConnectivityResult {
	cts.mu.RLock()
	defer cts.mu.RUnlock()

	var results []ConnectivityResult
	if platformResults, ok := cts.results[platform]; ok {
		for _, r := range platformResults {
			results = append(results, *r)
		}
	}
	return results
}

// GetAllResults 获取所有平台的测试结果
func (cts *ConnectivityTestService) GetAllResults() map[string][]ConnectivityResult {
	cts.mu.RLock()
	defer cts.mu.RUnlock()

	allResults := make(map[string][]ConnectivityResult)
	for platform, platformResults := range cts.results {
		var results []ConnectivityResult
		for _, r := range platformResults {
			results = append(results, *r)
		}
		allResults[platform] = results
	}
	return allResults
}

// RunSingleTest 手动触发单个供应商测试
func (cts *ConnectivityTestService) RunSingleTest(platform string, providerID int64) (*ConnectivityResult, error) {
	providers, err := cts.providerService.LoadProviders(platform)
	if err != nil {
		return nil, fmt.Errorf("加载供应商失败: %w", err)
	}

	var targetProvider *Provider
	for i := range providers {
		if providers[i].ID == providerID {
			targetProvider = &providers[i]
			break
		}
	}

	if targetProvider == nil {
		return nil, fmt.Errorf("未找到供应商 ID: %d", providerID)
	}

	timeout := cts.getEffectiveTimeout(targetProvider)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)
	defer cancel()

	result := cts.TestProvider(ctx, *targetProvider, platform)

	// 保存结果
	cts.mu.Lock()
	if cts.results[platform] == nil {
		cts.results[platform] = make(map[int64]*ConnectivityResult)
	}
	cts.results[platform][providerID] = result
	cts.mu.Unlock()

	// 与拉黑服务联动
	cts.handleBlacklistIntegration(platform, targetProvider.ID, targetProvider.Name, result)

	return result, nil
}

// SetAutoTestEnabled 设置自动测试开关
func (cts *ConnectivityTestService) SetAutoTestEnabled(enabled bool) error {
	cts.mu.Lock()
	defer cts.mu.Unlock()

	if enabled == cts.autoTestEnabled {
		return nil
	}

	cts.autoTestEnabled = enabled

	if enabled {
		cts.startAutoTest()
	} else {
		cts.stopAutoTest()
	}

	log.Printf("[ConnectivityTest] 自动测试已%s", map[bool]string{true: "开启", false: "关闭"}[enabled])
	return nil
}

// GetAutoTestEnabled 获取自动测试开关状态
func (cts *ConnectivityTestService) GetAutoTestEnabled() bool {
	cts.mu.RLock()
	defer cts.mu.RUnlock()
	return cts.autoTestEnabled
}

// startAutoTest 启动自动测试定时器
func (cts *ConnectivityTestService) startAutoTest() {
	if cts.running {
		return
	}

	cts.stopChan = make(chan struct{})
	cts.running = true

	go func() {
		// 启动时立即执行一次
		cts.runAllPlatformTests()

		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				cts.runAllPlatformTests()
			case <-cts.stopChan:
				log.Println("[ConnectivityTest] 自动测试定时器已停止")
				return
			}
		}
	}()

	log.Println("[ConnectivityTest] 自动测试定时器已启动（间隔: 1分钟）")
}

// stopAutoTest 停止自动测试定时器
func (cts *ConnectivityTestService) stopAutoTest() {
	if !cts.running {
		return
	}

	close(cts.stopChan)
	cts.running = false
}

// runAllPlatformTests 执行所有平台的测试
func (cts *ConnectivityTestService) runAllPlatformTests() {
	// 仅轮询 ProviderService 支持的平台，避免无意义的错误日志
	// Gemini 使用独立的 GeminiService，暂未接入
	platforms := []string{"claude", "codex"}
	for _, platform := range platforms {
		cts.TestAll(platform)
	}
}

// Wails 生命周期方法
func (cts *ConnectivityTestService) Start() error {
	return nil
}

func (cts *ConnectivityTestService) Stop() error {
	cts.mu.Lock()
	defer cts.mu.Unlock()

	if cts.running {
		close(cts.stopChan)
		cts.running = false
	}
	return nil
}

// ManualTestResult 手动测试结果
type ManualTestResult struct {
	Success   bool   `json:"success"`
	LatencyMs int    `json:"latencyMs"`
	HTTPCode  int    `json:"httpCode"`
	Message   string `json:"message"`
}

// TestProviderManual 手动测试供应商连通性（供前端测试按钮调用）
func (cts *ConnectivityTestService) TestProviderManual(
	platform string,
	apiURL string,
	apiKey string,
	model string,
	endpoint string,
	authType string,
) ManualTestResult {
	// 调试日志：打印前端传递的参数
	fmt.Printf("[DEBUG] TestProviderManual 收到参数:\n")
	fmt.Printf("  platform: %q\n", platform)
	fmt.Printf("  apiURL:   %q\n", apiURL)
	fmt.Printf("  apiKey:   %q (len=%d)\n", maskAPIKey(apiKey), len(apiKey))
	fmt.Printf("  model:    %q\n", model)
	fmt.Printf("  endpoint: %q\n", endpoint)
	fmt.Printf("  authType: %q\n", authType)

	// 平台参数校验
	if platform == "" {
		platform = "claude"
	}

	// 构建临时 Provider
	provider := Provider{
		APIURL: apiURL,
		APIKey: apiKey,
		AvailabilityConfig: &AvailabilityConfig{
			TestModel:    strings.TrimSpace(model),
			TestEndpoint: normalizeAvailabilityEndpoint(endpoint),
			Timeout:      DefaultTimeoutMs,
		},
		ConnectivityAuthType: strings.TrimSpace(authType),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result := cts.TestProvider(ctx, provider, platform)

	return ManualTestResult{
		Success:   result.Status == StatusAvailable || result.Status == StatusDegraded,
		LatencyMs: result.LatencyMs,
		HTTPCode:  result.HTTPCode,
		Message:   result.Message,
	}
}

// maskAPIKey 隐藏 API Key 的中间部分，用于安全日志输出
func maskAPIKey(key string) string {
	if len(key) <= 10 {
		return "***"
	}
	return key[:6] + "..." + key[len(key)-4:]
}
