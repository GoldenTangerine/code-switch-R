// services/healthcheckservice.go
// 可用性监控服务 - 健康检查核心引擎
// Author: Half open flowers

package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/daodao97/xgo/xdb"
)

// HealthStatus 健康状态常量
const (
	HealthStatusOperational     = "operational"       // 正常（响应 ≤6s）
	HealthStatusDegraded        = "degraded"          // 延迟（响应 >6s 但成功）
	HealthStatusFailed          = "failed"            // 故障（请求失败/超时）
	HealthStatusValidationError = "validation_failed" // 验证失败（回复内容异常）
)

// 默认配置常量
const (
	DefaultOperationalThresholdMs = 6000  // 默认正常阈值（毫秒）
	DefaultTimeoutMs              = 15000 // 默认超时（毫秒）
	DefaultPollIntervalSeconds    = 60    // 默认检测间隔（秒）
	DefaultFailureThreshold       = 2     // 默认拉黑阈值（连续失败次数）
	MaxConcurrentChecks           = 5     // 最大并发检测数
	MaxHistoryPerProvider         = 60    // 每个 Provider 最多保留历史数
	LogAvailabilityHistoryLimit   = 72    // 日志模式最多返回的时间桶数量
	LogAvailabilityWarningRate    = 5.0   // 日志模式黄色错误率阈值（%）
	LogAvailabilityFailedRate     = 20.0  // 日志模式红色错误率阈值（%）
)

const (
	LogAvailabilityRange15Min   = "15min"
	LogAvailabilityRange1H      = "1h"
	LogAvailabilityRange6H      = "6h"
	LogAvailabilityRange24H     = "24h"
	LogAvailabilityRange7D      = "7d"
	LogAvailabilityRangeDefault = LogAvailabilityRange24H
)

var availabilityPlatforms = []string{"claude", "codex"}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	ID             int64     `json:"id"`
	ProviderID     int64     `json:"providerId"`
	ProviderName   string    `json:"providerName"`
	Platform       string    `json:"platform"`
	Model          string    `json:"model,omitempty"`
	Endpoint       string    `json:"endpoint,omitempty"`
	Status         string    `json:"status"`                   // operational/degraded/failed/validation_failed
	LatencyMs      int       `json:"latencyMs"`                // 响应延迟（毫秒）
	ErrorMessage   string    `json:"errorMessage"`             // 错误消息
	CheckedAt      time.Time `json:"checkedAt"`                // 检测时间
	TotalRequests  int       `json:"totalRequests,omitempty"`  // 日志聚合请求数
	FailedRequests int       `json:"failedRequests,omitempty"` // 日志聚合失败请求数
	SlowRequests   int       `json:"slowRequests,omitempty"`   // 日志聚合慢请求数
	ErrorRate      float64   `json:"errorRate,omitempty"`      // 日志聚合错误占比（%）
}

// HealthCheckHistory 健康检查历史（单个 Provider 的时间线）
type HealthCheckHistory struct {
	ProviderID   int64               `json:"providerId"`
	ProviderName string              `json:"providerName"`
	Platform     string              `json:"platform"`
	Items        []HealthCheckResult `json:"items"`        // 历史记录（最近 N 条）
	Latest       *HealthCheckResult  `json:"latest"`       // 最新一条
	Uptime       float64             `json:"uptime"`       // 可用率（%）
	AvgLatencyMs int                 `json:"avgLatencyMs"` // 平均延迟
}

// ProviderTimeline Provider 时间线（用于前端展示）
type ProviderTimeline struct {
	ProviderID                 int64               `json:"providerId"`
	ProviderName               string              `json:"providerName"`
	Platform                   string              `json:"platform"`
	AvailabilityMonitorEnabled bool                `json:"availabilityMonitorEnabled"`
	ConnectivityAutoBlacklist  bool                `json:"connectivityAutoBlacklist"`
	AvailabilityConfig         *AvailabilityConfig `json:"availabilityConfig,omitempty"` // 高级配置
	Items                      []HealthCheckResult `json:"items"`                        // 历史记录
	Latest                     *HealthCheckResult  `json:"latest"`                       // 最新一条
	Uptime                     float64             `json:"uptime"`                       // 可用率
	AvgLatencyMs               int                 `json:"avgLatencyMs"`                 // 平均延迟
}

// AvailabilityFailureCounter 可用性失败计数器（独立于真实请求）
type AvailabilityFailureCounter struct {
	Platform         string
	ProviderName     string
	ConsecutiveFails int       // 连续失败次数
	LastFailedAt     time.Time // 最后失败时间
}

// HealthCheckService 健康检查服务
type HealthCheckService struct {
	providerService  *ProviderService
	blacklistService *BlacklistService
	settingsService  *SettingsService

	mu            sync.RWMutex
	failCounters  map[string]*AvailabilityFailureCounter  // key: platform:providerRef
	latestResults map[string]map[int64]*HealthCheckResult // platform -> providerID -> result

	// 后台轮询
	running      bool
	stopChan     chan struct{}
	pollInterval time.Duration

	// HTTP 客户端（带连接池）
	client *http.Client
}

// NewHealthCheckService 创建健康检查服务
func NewHealthCheckService(
	providerService *ProviderService,
	blacklistService *BlacklistService,
	settingsService *SettingsService,
) *HealthCheckService {
	return &HealthCheckService{
		providerService:  providerService,
		blacklistService: blacklistService,
		settingsService:  settingsService,
		failCounters:     make(map[string]*AvailabilityFailureCounter),
		latestResults: map[string]map[int64]*HealthCheckResult{
			"claude": {},
			"codex":  {},
			"gemini": {},
		},
		pollInterval: time.Duration(DefaultPollIntervalSeconds) * time.Second,
		client: &http.Client{
			// 由每次请求的 context 控制超时，避免固定值截断自定义配置
			Timeout: 0,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     15 * time.Second,
				DisableCompression:  true,
				MaxIdleConnsPerHost: 3,
			},
		},
	}
}

// Start Wails 生命周期方法
func (hcs *HealthCheckService) Start() error {
	// 初始化数据库表
	if err := hcs.ensureTable(); err != nil {
		return fmt.Errorf("初始化健康检查表失败: %w", err)
	}
	return nil
}

// Stop Wails 生命周期方法
func (hcs *HealthCheckService) Stop() error {
	hcs.StopBackgroundPolling()
	return nil
}

// ensureTable 确保健康检查历史表存在
func (hcs *HealthCheckService) ensureTable() error {
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	const createTableSQL = `CREATE TABLE IF NOT EXISTS health_check_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider_id INTEGER NOT NULL,
		provider_name TEXT NOT NULL,
		platform TEXT NOT NULL,
		model TEXT,
		endpoint TEXT,
		status TEXT NOT NULL,
		latency_ms INTEGER,
		error_message TEXT,
		checked_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("创建 health_check_history 表失败: %w", err)
	}

	// 创建索引
	const createIndexSQL = `
		CREATE INDEX IF NOT EXISTS idx_health_provider ON health_check_history(platform, provider_name);
		CREATE INDEX IF NOT EXISTS idx_health_provider_id ON health_check_history(platform, provider_id);
		CREATE INDEX IF NOT EXISTS idx_health_checked_at ON health_check_history(checked_at);
	`
	if _, err := db.Exec(createIndexSQL); err != nil {
		log.Printf("[HealthCheck] 创建索引警告: %v", err)
	}

	return nil
}

// GetLatestResults 获取所有 Provider 的最新状态（按平台分组）
// 优化：使用批量查询避免 N+1 查询问题
func (hcs *HealthCheckService) GetLatestResults() (map[string][]ProviderTimeline, error) {
	results := make(map[string][]ProviderTimeline)

	// 遍历所有平台
	for _, platform := range availabilityPlatforms {
		providers, err := hcs.providerService.LoadProviders(platform)
		if err != nil {
			log.Printf("[HealthCheck] 加载 %s 供应商失败: %v", platform, err)
			continue
		}

		providers = filterAvailabilityProviders(platform, providers)

		// 批量查询该平台的所有历史记录
		historiesMap, err := hcs.batchGetHistories(platform)
		if err != nil {
			log.Printf("[HealthCheck] 批量查询 %s 历史记录失败: %v", platform, err)
		}

		// 组装结果
		var timelines []ProviderTimeline
		for _, p := range providers {
			timeline := ProviderTimeline{
				ProviderID:                 p.ID,
				ProviderName:               p.Name,
				Platform:                   platform,
				AvailabilityMonitorEnabled: p.AvailabilityMonitorEnabled,
				ConnectivityAutoBlacklist:  p.ConnectivityAutoBlacklist,
				AvailabilityConfig:         p.AvailabilityConfig,
			}

			// 从批量查询结果中获取该 provider 的历史记录（优先按 provider ID 关联）
			if history, ok := historiesMap[healthCheckHistoryKey(p.ID, p.Name)]; ok {
				timeline.Items = history.Items
				timeline.Latest = history.Latest
				timeline.Uptime = history.Uptime
				timeline.AvgLatencyMs = history.AvgLatencyMs
			}

			timelines = append(timelines, timeline)
		}

		results[platform] = timelines
	}

	return results, nil
}

// GetLogBasedResults 获取基于真实请求日志聚合的 Provider 可用性时间线。
// 与 GetLatestResults 返回同一种 ProviderTimeline，方便前端复用同一套卡片和状态条。
func (hcs *HealthCheckService) GetLogBasedResults(rangeKey string) (map[string][]ProviderTimeline, error) {
	results := make(map[string][]ProviderTimeline)
	rangeSpec := resolveLogAvailabilityRange(rangeKey, time.Now())
	operationalThresholdMs := hcs.getOperationalThresholdMs()

	for _, platform := range availabilityPlatforms {
		providers, err := hcs.providerService.LoadProviders(platform)
		if err != nil {
			log.Printf("[HealthCheck] 加载 %s 供应商失败: %v", platform, err)
			continue
		}

		providers = filterAvailabilityProviders(platform, providers)

		historiesMap, err := hcs.batchGetLogBasedHistories(platform, providers, rangeSpec, operationalThresholdMs)
		if err != nil {
			log.Printf("[HealthCheck] 批量聚合 %s 日志可用性失败: %v", platform, err)
		}

		var timelines []ProviderTimeline
		for _, p := range providers {
			timeline := ProviderTimeline{
				ProviderID:                 p.ID,
				ProviderName:               p.Name,
				Platform:                   platform,
				AvailabilityMonitorEnabled: true,
				ConnectivityAutoBlacklist:  p.ConnectivityAutoBlacklist,
				AvailabilityConfig:         p.AvailabilityConfig,
			}

			if history, ok := historiesMap[healthCheckHistoryKey(p.ID, p.Name)]; ok {
				timeline.Items = history.Items
				timeline.Latest = history.Latest
				timeline.Uptime = history.Uptime
				timeline.AvgLatencyMs = history.AvgLatencyMs
			}

			timelines = append(timelines, timeline)
		}

		results[platform] = timelines
	}

	return results, nil
}

func filterAvailabilityProviders(platform string, providers []Provider) []Provider {
	filtered := make([]Provider, 0, len(providers))
	for _, provider := range providers {
		if platform == "codex" && isCodexOfficialProviderCard(provider) {
			continue
		}
		filtered = append(filtered, provider)
	}
	return filtered
}

type logAvailabilityBucket struct {
	ProviderID     int64
	ProviderName   string
	BucketStart    time.Time
	TotalRequests  int
	FailedCount    int
	WarningCount   int
	LatencyTotalMs int64
	LatencySamples int
	AvgLatencyMs   int
	LatestModel    string
	LastRequestAt  time.Time
	LastStatusCode int
}

type logAvailabilityRangeSpec struct {
	Key            string
	Start          time.Time
	End            time.Time
	BucketDuration time.Duration
}

type logAvailabilityRequestLog struct {
	ProviderRef string
	Model       string
	HTTPCode    int
	DurationMs  int
	CreatedAt   time.Time
}

type logAvailabilityProviderBucketSet struct {
	Provider Provider
	Buckets  []logAvailabilityBucket
}

func (hcs *HealthCheckService) batchGetLogBasedHistories(platform string, providers []Provider, rangeSpec logAvailabilityRangeSpec, operationalThresholdMs int) (map[string]*HealthCheckHistory, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return nil, fmt.Errorf("获取数据库连接失败: %w", err)
	}

	if len(providers) == 0 {
		return map[string]*HealthCheckHistory{}, nil
	}

	bucketSetsByProviderRef := make(map[string]*logAvailabilityProviderBucketSet, len(providers)*2)
	bucketSetsByHistoryKey := make(map[string]*logAvailabilityProviderBucketSet, len(providers))
	for _, provider := range providers {
		providerRef := providerRefFromNumericID(provider.ID, provider.Name)
		if strings.TrimSpace(providerRef) == "" {
			continue
		}

		bucketSet := &logAvailabilityProviderBucketSet{
			Provider: provider,
			Buckets:  buildLogAvailabilityBuckets(provider, rangeSpec),
		}
		bucketSetsByHistoryKey[healthCheckHistoryKey(provider.ID, provider.Name)] = bucketSet
		bucketSetsByProviderRef[providerRef] = bucketSet
		trimmedName := strings.TrimSpace(provider.Name)
		if trimmedName != "" {
			bucketSetsByProviderRef[trimmedName] = bucketSet
		}
	}

	if err := hcs.applyLogAvailabilityRequests(db, platform, rangeSpec, bucketSetsByProviderRef, operationalThresholdMs); err != nil {
		return nil, err
	}

	historiesMap := make(map[string]*HealthCheckHistory, len(bucketSetsByHistoryKey))
	for key, bucketSet := range bucketSetsByHistoryKey {
		buckets := bucketSet.Buckets
		if len(buckets) == 0 {
			continue
		}
		finalizeLogAvailabilityBuckets(buckets)
		reverseLogAvailabilityBuckets(buckets)

		history := &HealthCheckHistory{
			ProviderID:   buckets[0].ProviderID,
			ProviderName: buckets[0].ProviderName,
			Platform:     platform,
			Items:        make([]HealthCheckResult, 0, len(buckets)),
		}

		var totalLatency int64
		var totalLatencySamples int
		var successBuckets int
		var sampledBuckets int
		latestIndex := -1
		for index, bucket := range buckets {
			status := resolveLogAvailabilityStatus(bucket)
			if status == HealthStatusOperational {
				successBuckets++
			}
			if status == HealthStatusOperational || status == HealthStatusDegraded {
				totalLatency += bucket.LatencyTotalMs
				totalLatencySamples += bucket.LatencySamples
			}
			if bucket.TotalRequests > 0 {
				sampledBuckets++
				if latestIndex < 0 {
					latestIndex = index
				}
			}

			history.Items = append(history.Items, HealthCheckResult{
				ID:             int64(index + 1),
				ProviderID:     bucket.ProviderID,
				ProviderName:   bucket.ProviderName,
				Platform:       platform,
				Model:          bucket.LatestModel,
				Endpoint:       "request_log",
				Status:         status,
				LatencyMs:      bucket.AvgLatencyMs,
				ErrorMessage:   formatLogAvailabilityError(bucket, operationalThresholdMs),
				CheckedAt:      bucket.BucketStart,
				TotalRequests:  bucket.TotalRequests,
				FailedRequests: bucket.FailedCount,
				SlowRequests:   bucket.WarningCount,
				ErrorRate:      logAvailabilityErrorRate(bucket),
			})
		}

		if len(history.Items) > 0 && latestIndex >= 0 {
			history.Latest = &history.Items[latestIndex]
		}
		if sampledBuckets > 0 {
			history.Uptime = float64(successBuckets) / float64(sampledBuckets) * 100
			if totalLatencySamples > 0 {
				history.AvgLatencyMs = int(totalLatency / int64(totalLatencySamples))
			}
		}
		historiesMap[key] = history
	}

	return historiesMap, nil
}

func (hcs *HealthCheckService) applyLogAvailabilityRequests(db *sql.DB, platform string, rangeSpec logAvailabilityRangeSpec, bucketSetsByProviderRef map[string]*logAvailabilityProviderBucketSet, operationalThresholdMs int) error {
	if len(bucketSetsByProviderRef) == 0 {
		return nil
	}

	query := `
		SELECT provider_id, provider, model, http_code, duration_sec, created_at
		FROM request_log
		WHERE platform = ?
		  AND created_at >= ?
		  AND created_at <= ?
		ORDER BY created_at ASC
	`

	rows, err := db.Query(
		query,
		platform,
		rangeSpec.Start.UTC().Format(timeLayout),
		rangeSpec.End.UTC().Format(timeLayout),
	)
	if err != nil {
		if isNoSuchTableErr(err) {
			return nil
		}
		return fmt.Errorf("聚合日志可用性失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		logItem, err := scanLogAvailabilityRequest(rows)
		if err != nil {
			return err
		}
		bucketSet := bucketSetsByProviderRef[logItem.ProviderRef]
		if bucketSet == nil {
			continue
		}
		applyLogAvailabilityRequest(bucketSet.Buckets, rangeSpec, logItem, operationalThresholdMs)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	return nil
}

func resolveLogAvailabilityRange(rangeKey string, now time.Time) logAvailabilityRangeSpec {
	key := strings.TrimSpace(rangeKey)
	duration := 24 * time.Hour
	switch key {
	case LogAvailabilityRange15Min:
		duration = 15 * time.Minute
	case LogAvailabilityRange1H:
		duration = time.Hour
	case LogAvailabilityRange6H:
		duration = 6 * time.Hour
	case LogAvailabilityRange24H:
		duration = 24 * time.Hour
	case LogAvailabilityRange7D:
		duration = 7 * 24 * time.Hour
	default:
		key = LogAvailabilityRangeDefault
	}

	end := now.UTC().Truncate(time.Second)
	start := end.Add(-duration)
	return logAvailabilityRangeSpec{
		Key:            key,
		Start:          start,
		End:            end,
		BucketDuration: duration / time.Duration(LogAvailabilityHistoryLimit),
	}
}

func buildLogAvailabilityBuckets(provider Provider, rangeSpec logAvailabilityRangeSpec) []logAvailabilityBucket {
	buckets := make([]logAvailabilityBucket, LogAvailabilityHistoryLimit)
	for index := range buckets {
		buckets[index] = logAvailabilityBucket{
			ProviderID:   provider.ID,
			ProviderName: provider.Name,
			BucketStart:  rangeSpec.Start.Add(time.Duration(index) * rangeSpec.BucketDuration),
		}
	}
	return buckets
}

func scanLogAvailabilityRequest(rows *sql.Rows) (logAvailabilityRequestLog, error) {
	var providerID, providerName sql.NullString
	var model sql.NullString
	var httpCode sql.NullInt64
	var durationSec sql.NullFloat64
	var createdAtRaw sql.NullString
	if err := rows.Scan(&providerID, &providerName, &model, &httpCode, &durationSec, &createdAtRaw); err != nil {
		return logAvailabilityRequestLog{}, err
	}

	createdAt, err := parseStoredRequestLogTime(createdAtRaw)
	if err != nil {
		return logAvailabilityRequestLog{}, err
	}

	durationMs := 0
	if durationSec.Valid && durationSec.Float64 > 0 {
		durationMs = int(durationSec.Float64*1000 + 0.5)
	}

	return logAvailabilityRequestLog{
		ProviderRef: providerRefFromStringID(providerID.String, providerName.String),
		Model:       strings.TrimSpace(model.String),
		HTTPCode:    int(httpCode.Int64),
		DurationMs:  durationMs,
		CreatedAt:   createdAt,
	}, nil
}

func applyLogAvailabilityRequest(buckets []logAvailabilityBucket, rangeSpec logAvailabilityRangeSpec, logItem logAvailabilityRequestLog, operationalThresholdMs int) {
	if logItem.CreatedAt.IsZero() || logItem.CreatedAt.Before(rangeSpec.Start) || logItem.CreatedAt.After(rangeSpec.End) {
		return
	}

	bucketIndex := int(logItem.CreatedAt.Sub(rangeSpec.Start) / rangeSpec.BucketDuration)
	if bucketIndex < 0 {
		bucketIndex = 0
	}
	if bucketIndex >= len(buckets) {
		bucketIndex = len(buckets) - 1
	}

	bucket := &buckets[bucketIndex]
	bucket.TotalRequests++
	if isLogAvailabilityFailure(logItem.HTTPCode) {
		bucket.FailedCount++
	} else if logItem.DurationMs > operationalThresholdMs {
		bucket.WarningCount++
	}
	if logItem.DurationMs > 0 {
		bucket.LatencyTotalMs += int64(logItem.DurationMs)
		bucket.LatencySamples++
	}
	if logItem.CreatedAt.After(bucket.LastRequestAt) || bucket.LastRequestAt.IsZero() {
		bucket.LastRequestAt = logItem.CreatedAt
		bucket.LatestModel = logItem.Model
		bucket.LastStatusCode = logItem.HTTPCode
	}
}

func finalizeLogAvailabilityBuckets(buckets []logAvailabilityBucket) {
	for index := range buckets {
		if buckets[index].LatencySamples > 0 {
			buckets[index].AvgLatencyMs = int(buckets[index].LatencyTotalMs / int64(buckets[index].LatencySamples))
		}
	}
}

func reverseLogAvailabilityBuckets(buckets []logAvailabilityBucket) {
	for left, right := 0, len(buckets)-1; left < right; left, right = left+1, right-1 {
		buckets[left], buckets[right] = buckets[right], buckets[left]
	}
}

func isLogAvailabilityFailure(httpCode int) bool {
	return httpCode <= 0 || httpCode >= 400
}

func logAvailabilityErrorRate(bucket logAvailabilityBucket) float64 {
	if bucket.TotalRequests <= 0 {
		return 0
	}
	return float64(bucket.FailedCount) / float64(bucket.TotalRequests) * 100
}

func resolveLogAvailabilityStatus(bucket logAvailabilityBucket) string {
	if bucket.TotalRequests <= 0 {
		return ""
	}
	errorRate := logAvailabilityErrorRate(bucket)
	if errorRate > LogAvailabilityFailedRate {
		return HealthStatusFailed
	}
	if errorRate > LogAvailabilityWarningRate {
		return HealthStatusDegraded
	}
	return HealthStatusOperational
}

func formatLogAvailabilityError(bucket logAvailabilityBucket, operationalThresholdMs int) string {
	if bucket.TotalRequests <= 0 {
		return ""
	}
	var parts []string
	if bucket.FailedCount > 0 {
		errorRate := logAvailabilityErrorRate(bucket)
		if bucket.LastStatusCode >= 400 {
			parts = append(parts, fmt.Sprintf("%d/%d 请求失败（%.2f%%），最近 HTTP %d", bucket.FailedCount, bucket.TotalRequests, errorRate, bucket.LastStatusCode))
		} else {
			parts = append(parts, fmt.Sprintf("%d/%d 请求失败（%.2f%%）", bucket.FailedCount, bucket.TotalRequests, errorRate))
		}
	}
	if bucket.WarningCount > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d 请求延迟超过 %dms", bucket.WarningCount, bucket.TotalRequests, operationalThresholdMs))
	}
	if len(parts) > 0 {
		return "日志聚合：" + strings.Join(parts, "；")
	}
	return ""
}

// batchGetHistories 批量获取某平台所有 Provider 的历史记录（避免 N+1 查询）
func (hcs *HealthCheckService) batchGetHistories(platform string) (map[string]*HealthCheckHistory, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return nil, fmt.Errorf("获取数据库连接失败: %w", err)
	}

	// 批量查询：按平台一次性拉取所有记录，按 checked_at 倒序排列
	// 限制最多 5000 条记录，避免全表扫描
	query := `
		SELECT id, provider_id, provider_name, platform, model, endpoint, status, latency_ms, error_message, checked_at
		FROM health_check_history
		WHERE platform = ?
		ORDER BY checked_at DESC
		LIMIT 5000
	`

	rows, err := db.Query(query, platform)
	if err != nil {
		return nil, fmt.Errorf("批量查询历史记录失败: %w", err)
	}
	defer rows.Close()

	// 分组收集：按 provider_id（回退 provider_name）分组，每个 provider 最多保留 MaxHistoryPerProvider 条
	historiesMap := make(map[string]*HealthCheckHistory)

	for rows.Next() {
		var r HealthCheckResult
		var model, endpoint, errorMsg sql.NullString
		var latencyMs sql.NullInt64

		if err := rows.Scan(
			&r.ID, &r.ProviderID, &r.ProviderName, &r.Platform,
			&model, &endpoint, &r.Status, &latencyMs, &errorMsg, &r.CheckedAt,
		); err != nil {
			log.Printf("[HealthCheck] 解析历史记录失败: %v", err)
			continue
		}

		if model.Valid {
			r.Model = model.String
		}
		if endpoint.Valid {
			r.Endpoint = endpoint.String
		}
		if latencyMs.Valid {
			r.LatencyMs = int(latencyMs.Int64)
		}
		if errorMsg.Valid {
			r.ErrorMessage = errorMsg.String
		}

		// 获取或创建该 provider 的 history
		key := healthCheckHistoryKey(r.ProviderID, r.ProviderName)
		history, ok := historiesMap[key]
		if !ok {
			history = &HealthCheckHistory{
				ProviderID:   r.ProviderID,
				ProviderName: r.ProviderName,
				Platform:     platform,
				Items:        make([]HealthCheckResult, 0, MaxHistoryPerProvider),
			}
			historiesMap[key] = history
		}

		// 限制每个 provider 最多保留 MaxHistoryPerProvider 条
		if len(history.Items) < MaxHistoryPerProvider {
			history.Items = append(history.Items, r)
		}
	}

	// 计算每个 provider 的 Uptime 和 AvgLatency
	for _, history := range historiesMap {
		if len(history.Items) == 0 {
			continue
		}

		var totalLatency int64
		var successCount int

		for _, item := range history.Items {
			if item.Status == HealthStatusOperational || item.Status == HealthStatusDegraded {
				successCount++
				totalLatency += int64(item.LatencyMs)
			}
		}

		history.Uptime = float64(successCount) / float64(len(history.Items)) * 100
		if successCount > 0 {
			history.AvgLatencyMs = int(totalLatency / int64(successCount))
		}
		history.Latest = &history.Items[0]
	}

	return historiesMap, nil
}

func healthCheckHistoryKey(providerID int64, providerName string) string {
	if providerID > 0 {
		return fmt.Sprintf("id:%d", providerID)
	}
	return "name:" + strings.ToLower(strings.TrimSpace(providerName))
}

// GetHistory 获取单个 Provider 的历史记录
func (hcs *HealthCheckService) GetHistory(platform, providerName string, limit int) (*HealthCheckHistory, error) {
	return hcs.getHistoryInternal(platform, 0, strings.TrimSpace(providerName), limit)
}

// GetHistoryByID 获取单个 Provider 的历史记录（按 provider_id 精确匹配）
func (hcs *HealthCheckService) GetHistoryByID(platform string, providerID int64, providerName string, limit int) (*HealthCheckHistory, error) {
	return hcs.getHistoryInternal(platform, providerID, strings.TrimSpace(providerName), limit)
}

func (hcs *HealthCheckService) getHistoryInternal(platform string, providerID int64, providerName string, limit int) (*HealthCheckHistory, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return nil, fmt.Errorf("获取数据库连接失败: %w", err)
	}

	if limit <= 0 {
		limit = MaxHistoryPerProvider
	}

	query := `
		SELECT id, provider_id, provider_name, platform, model, endpoint, status, latency_ms, error_message, checked_at
		FROM health_check_history
		WHERE platform = ?
	`
	args := []interface{}{platform}
	if providerID > 0 {
		query += ` AND provider_id = ?`
		args = append(args, providerID)
	} else {
		query += ` AND provider_name = ?`
		args = append(args, providerName)
	}
	query += `
		ORDER BY checked_at DESC
		LIMIT ?
	`
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询历史记录失败: %w", err)
	}
	defer rows.Close()

	history := &HealthCheckHistory{
		ProviderID:   providerID,
		ProviderName: providerName,
		Platform:     platform,
		Items:        make([]HealthCheckResult, 0),
	}

	var totalLatency int64
	var successCount int

	for rows.Next() {
		var r HealthCheckResult
		var model, endpoint, errorMsg sql.NullString
		var latencyMs sql.NullInt64

		if err := rows.Scan(
			&r.ID, &r.ProviderID, &r.ProviderName, &r.Platform,
			&model, &endpoint, &r.Status, &latencyMs, &errorMsg, &r.CheckedAt,
		); err != nil {
			continue
		}

		if model.Valid {
			r.Model = model.String
		}
		if endpoint.Valid {
			r.Endpoint = endpoint.String
		}
		if latencyMs.Valid {
			r.LatencyMs = int(latencyMs.Int64)
		}
		if errorMsg.Valid {
			r.ErrorMessage = errorMsg.String
		}

		history.Items = append(history.Items, r)
		history.ProviderID = r.ProviderID

		// 统计
		if r.Status == HealthStatusOperational || r.Status == HealthStatusDegraded {
			successCount++
			totalLatency += int64(r.LatencyMs)
		}
	}

	// 计算可用率和平均延迟
	if len(history.Items) > 0 {
		history.Uptime = float64(successCount) / float64(len(history.Items)) * 100
		if successCount > 0 {
			history.AvgLatencyMs = int(totalLatency / int64(successCount))
		}
		history.Latest = &history.Items[0]
	}

	return history, nil
}

// RunSingleCheck 手动触发单个 Provider 检测
func (hcs *HealthCheckService) RunSingleCheck(platform string, providerID int64) (*HealthCheckResult, error) {
	providers, err := hcs.providerService.LoadProviders(platform)
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

	// 执行检测（使用 Provider 配置的有效超时）
	timeout := hcs.getEffectiveTimeout(targetProvider)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)
	defer cancel()

	result := hcs.checkProvider(ctx, *targetProvider, platform)

	// 保存结果
	if err := hcs.saveResult(result); err != nil {
		log.Printf("[HealthCheck] 保存结果失败: %v", err)
	}

	// 更新缓存
	hcs.updateCache(result)

	// 处理拉黑联动
	hcs.handleBlacklistIntegration(targetProvider, result)

	return result, nil
}

// RunAllChecks 手动触发全部检测
func (hcs *HealthCheckService) RunAllChecks() (map[string][]HealthCheckResult, error) {
	results := make(map[string][]HealthCheckResult)

	for _, platform := range availabilityPlatforms {
		platformResults := hcs.checkAllProviders(platform)
		results[platform] = platformResults
	}

	return results, nil
}

// checkAllProviders 检测指定平台的所有启用监控的供应商
func (hcs *HealthCheckService) checkAllProviders(platform string) []HealthCheckResult {
	providers, err := hcs.providerService.LoadProviders(platform)
	if err != nil {
		log.Printf("[HealthCheck] 加载 %s 供应商失败: %v", platform, err)
		return nil
	}
	providers = filterAvailabilityProviders(platform, providers)

	var results []HealthCheckResult
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, MaxConcurrentChecks)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, provider := range providers {
		// 只检测启用了可用性监控的供应商
		if !provider.AvailabilityMonitorEnabled {
			continue
		}

		wg.Add(1)
		go func(p Provider) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := hcs.checkProvider(ctx, p, platform)

			// 保存结果
			if err := hcs.saveResult(result); err != nil {
				log.Printf("[HealthCheck] 保存结果失败: %v", err)
			}

			// 更新缓存
			hcs.updateCache(result)

			// 处理拉黑联动
			hcs.handleBlacklistIntegration(&p, result)

			mu.Lock()
			results = append(results, *result)
			mu.Unlock()

			log.Printf("[HealthCheck] %s/%s: status=%s, latency=%dms",
				platform, p.Name, result.Status, result.LatencyMs)
		}(provider)
	}

	wg.Wait()
	return results
}

// checkProvider 执行单个 Provider 的健康检查
func (hcs *HealthCheckService) checkProvider(ctx context.Context, provider Provider, platform string) *HealthCheckResult {
	result := &HealthCheckResult{
		ProviderID:   provider.ID,
		ProviderName: provider.Name,
		Platform:     platform,
		Status:       HealthStatusFailed,
		CheckedAt:    time.Now(),
	}

	// 获取有效的测试参数
	model := hcs.getEffectiveModel(&provider, platform)
	endpoint := hcs.getEffectiveEndpoint(&provider, platform)
	timeout := hcs.getEffectiveTimeout(&provider)
	result.Model = model
	result.Endpoint = endpoint

	// 执行共享探测逻辑
	probeResult, err := executeAvailabilityProbe(ctx, hcs.client, &provider, platform, model, endpoint, timeout)
	if probeResult.Plan.EffectiveModel != "" {
		result.Model = probeResult.Plan.EffectiveModel
	}
	if probeResult.Plan.EffectiveEndpoint != "" {
		result.Endpoint = probeResult.Plan.EffectiveEndpoint
	}
	result.LatencyMs = probeResult.LatencyMs

	if err != nil {
		var buildErr availabilityProbeBuildError
		if errors.As(err, &buildErr) {
			result.ErrorMessage = fmt.Sprintf("无法构建测试请求: %v", buildErr)
			return result
		}
		// 检测是否为超时错误
		if isTimeoutError(err) {
			result.Status = HealthStatusFailed
			result.ErrorMessage = fmt.Sprintf("响应超时 (>%dms)", timeout)
			log.Printf("[HealthCheck] [%s/%s] 请求超时: %dms (阈值: %dms)",
				platform, provider.Name, result.LatencyMs, timeout)
			return result
		}
		result.ErrorMessage = fmt.Sprintf("网络错误: %v", err)
		log.Printf("[HealthCheck] [%s/%s] 网络错误: %v", platform, provider.Name, err)
		return result
	}
	body := probeResult.ResponseBody

	// 判定状态
	result.Status, result.ErrorMessage = hcs.determineStatus(probeResult.HTTPStatusCode, result.LatencyMs, body)
	if (result.Status == HealthStatusOperational || result.Status == HealthStatusDegraded) &&
		!responseContainsExpectedText(body, probeResult.Plan.ResponseFormat, probeResult.Plan.ExpectedText) {
		result.Status = HealthStatusValidationError
		result.ErrorMessage = buildAvailabilityValidationError(body, probeResult.Plan.ResponseFormat, probeResult.Plan.ExpectedText)
	}

	return result
}

// determineStatus 根据 HTTP 状态码和延迟判定健康状态
func (hcs *HealthCheckService) determineStatus(statusCode, latencyMs int, body []byte) (string, string) {
	// 获取正常阈值（全局配置）
	operationalThresholdMs := hcs.getOperationalThresholdMs()

	// 2xx = 成功
	if statusCode >= 200 && statusCode < 300 {
		if latencyMs > operationalThresholdMs {
			return HealthStatusDegraded, fmt.Sprintf("响应成功但耗时 %dms", latencyMs)
		}
		return HealthStatusOperational, ""
	}

	// 特殊错误码
	switch statusCode {
	case 401, 403:
		return HealthStatusFailed, "认证失败"
	case 429:
		return HealthStatusFailed, "请求频率限制"
	case 400:
		return HealthStatusFailed, "请求无效"
	}

	// 5xx = 服务器错误
	if statusCode >= 500 {
		return HealthStatusFailed, fmt.Sprintf("服务器错误 (%d)", statusCode)
	}

	// 其他 4xx
	if statusCode >= 400 {
		return HealthStatusFailed, fmt.Sprintf("客户端错误 (%d)", statusCode)
	}

	return HealthStatusFailed, fmt.Sprintf("异常状态码 (%d)", statusCode)
}

// getEffectiveModel 获取有效的测试模型
func (hcs *HealthCheckService) getEffectiveModel(provider *Provider, platform string) string {
	return resolveProviderAvailabilityModel(provider, platform)
}

// getEffectiveEndpoint 获取有效的测试端点
func (hcs *HealthCheckService) getEffectiveEndpoint(provider *Provider, platform string) string {
	return resolveProviderAvailabilityEndpoint(provider, platform)
}

// getEffectiveTimeout 获取有效的超时时间（毫秒）
func (hcs *HealthCheckService) getEffectiveTimeout(provider *Provider) int {
	return resolveProviderAvailabilityTimeout(provider)
}

func (hcs *HealthCheckService) getOperationalThresholdMs() int {
	if hcs.settingsService != nil {
		if threshold := hcs.settingsService.GetIntSetting("availability_operational_threshold_ms"); threshold > 0 {
			return threshold
		}
	}
	return DefaultOperationalThresholdMs
}

// saveResult 保存检测结果到数据库
func (hcs *HealthCheckService) saveResult(result *HealthCheckResult) error {
	if GlobalDBQueue == nil {
		return fmt.Errorf("数据库写入队列未初始化")
	}

	const insertSQL = `
		INSERT INTO health_check_history (provider_id, provider_name, platform, model, endpoint, status, latency_ms, error_message, checked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	return GlobalDBQueue.Exec(insertSQL,
		result.ProviderID,
		result.ProviderName,
		result.Platform,
		result.Model,
		result.Endpoint,
		result.Status,
		result.LatencyMs,
		result.ErrorMessage,
		result.CheckedAt,
	)
}

// updateCache 更新内存缓存
func (hcs *HealthCheckService) updateCache(result *HealthCheckResult) {
	hcs.mu.Lock()
	defer hcs.mu.Unlock()

	if hcs.latestResults[result.Platform] == nil {
		hcs.latestResults[result.Platform] = make(map[int64]*HealthCheckResult)
	}
	hcs.latestResults[result.Platform][result.ProviderID] = result
}

// handleBlacklistIntegration 处理与拉黑服务的联动
func (hcs *HealthCheckService) handleBlacklistIntegration(provider *Provider, result *HealthCheckResult) {
	// 未启用自动拉黑则跳过
	if !provider.ConnectivityAutoBlacklist {
		return
	}

	// 获取失败阈值（全局配置）
	failureThreshold := DefaultFailureThreshold
	if hcs.settingsService != nil {
		if threshold := hcs.settingsService.GetIntSetting("availability_failure_threshold"); threshold > 0 {
			failureThreshold = threshold
		}
	}

	// 获取或创建失败计数器（统一与黑名单相同的 providerRef 规则）
	providerRef := providerRefFromNumericID(provider.ID, provider.Name)
	counterKey := fmt.Sprintf("%s:%s", result.Platform, providerRef)
	hcs.mu.Lock()
	counter, exists := hcs.failCounters[counterKey]
	if !exists {
		counter = &AvailabilityFailureCounter{
			Platform:     result.Platform,
			ProviderName: provider.Name,
		}
		hcs.failCounters[counterKey] = counter
	}

	// 在锁内更新计数器，避免并发竞态
	var shouldTriggerBlacklist bool
	var shouldRecordSuccess bool
	var prevFails int

	if result.Status == HealthStatusFailed || result.Status == HealthStatusValidationError {
		counter.ConsecutiveFails++
		counter.LastFailedAt = time.Now()
		prevFails = counter.ConsecutiveFails

		log.Printf("[HealthCheck] Provider %s 检测失败，连续失败: %d/%d",
			provider.Name, prevFails, failureThreshold)

		// 检查是否达到拉黑阈值
		if prevFails >= failureThreshold && hcs.blacklistService != nil {
			shouldTriggerBlacklist = true
		}
	} else if result.Status == HealthStatusOperational {
		// 成功，清零失败计数
		prevFails = counter.ConsecutiveFails
		counter.ConsecutiveFails = 0

		if prevFails > 0 {
			log.Printf("[HealthCheck] Provider %s 恢复正常，清零失败计数（之前: %d）",
				provider.Name, prevFails)
		}

		// 标记需要通知拉黑服务恢复
		if hcs.blacklistService != nil {
			shouldRecordSuccess = true
		}
	}
	hcs.mu.Unlock()

	// 在锁外执行耗时的 RPC 调用，避免阻塞其他检测
	if shouldTriggerBlacklist {
		if err := hcs.blacklistService.RecordFailureByID(result.Platform, providerRef, provider.Name); err != nil {
			log.Printf("[HealthCheck] 触发拉黑失败: %v", err)
		} else {
			log.Printf("[HealthCheck] Provider %s 连续失败 %d 次，已触发拉黑！", provider.Name, failureThreshold)
		}
	}

	if shouldRecordSuccess {
		if err := hcs.blacklistService.RecordSuccessByID(result.Platform, providerRef, provider.Name); err != nil {
			log.Printf("[HealthCheck] RecordSuccess 失败: %v", err)
		}
	}
	// degraded 状态不触发拉黑，也不清零计数
}

// StartBackgroundPolling 启动后台定时巡检
func (hcs *HealthCheckService) StartBackgroundPolling() {
	hcs.mu.Lock()
	defer hcs.mu.Unlock()

	if hcs.running {
		return
	}

	// 获取配置的轮询间隔
	pollIntervalSeconds := DefaultPollIntervalSeconds
	if hcs.settingsService != nil {
		if interval := hcs.settingsService.GetIntSetting("availability_poll_interval_seconds"); interval > 0 {
			pollIntervalSeconds = interval
		}
	}
	hcs.pollInterval = time.Duration(pollIntervalSeconds) * time.Second

	hcs.stopChan = make(chan struct{})
	hcs.running = true

	go func() {
		// 启动时延迟随机时间（0-10s），避免整点风暴
		jitter := time.Duration(rand.Intn(10000)) * time.Millisecond
		time.Sleep(jitter)

		// 立即执行一次
		hcs.runAllPlatformChecks()

		// 添加抖动（±10%）
		ticker := time.NewTicker(hcs.pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				hcs.runAllPlatformChecks()
			case <-hcs.stopChan:
				log.Println("[HealthCheck] 后台巡检已停止")
				return
			}
		}
	}()

	log.Printf("[HealthCheck] 后台巡检已启动（间隔: %v）", hcs.pollInterval)
}

// StopBackgroundPolling 停止后台巡检
func (hcs *HealthCheckService) StopBackgroundPolling() {
	hcs.mu.Lock()
	defer hcs.mu.Unlock()

	if !hcs.running {
		return
	}

	close(hcs.stopChan)
	hcs.running = false
}

// IsPollingRunning 检查后台巡检是否运行中
func (hcs *HealthCheckService) IsPollingRunning() bool {
	hcs.mu.RLock()
	defer hcs.mu.RUnlock()
	return hcs.running
}

// SetAutoAvailabilityPolling 设置是否自动轮询（立即生效）
func (hcs *HealthCheckService) SetAutoAvailabilityPolling(enabled bool) {
	if enabled {
		// 启动轮询（StartBackgroundPolling 内部有锁）
		hcs.StartBackgroundPolling()
		log.Println("[HealthCheck] 已启用自动可用性监控")
	} else {
		// 停止轮询（StopBackgroundPolling 内部有锁）
		hcs.StopBackgroundPolling()
		log.Println("[HealthCheck] 已禁用自动可用性监控")
	}
}

// runAllPlatformChecks 执行所有平台的检测
func (hcs *HealthCheckService) runAllPlatformChecks() {
	for _, platform := range availabilityPlatforms {
		hcs.checkAllProviders(platform)
	}
}

// SetAvailabilityMonitorEnabled 启用/禁用指定 Provider 的可用性监控
func (hcs *HealthCheckService) SetAvailabilityMonitorEnabled(platform string, providerID int64, enabled bool) error {
	providers, err := hcs.providerService.LoadProviders(platform)
	if err != nil {
		return fmt.Errorf("加载供应商失败: %w", err)
	}

	found := false
	for i := range providers {
		if providers[i].ID == providerID {
			providers[i].AvailabilityMonitorEnabled = enabled
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到供应商 ID: %d", providerID)
	}

	if err := hcs.providerService.SaveProviders(platform, providers); err != nil {
		return fmt.Errorf("保存供应商配置失败: %w", err)
	}

	log.Printf("[HealthCheck] Provider %d 可用性监控已%s", providerID, map[bool]string{true: "启用", false: "禁用"}[enabled])
	return nil
}

// SetConnectivityAutoBlacklist 启用/禁用指定 Provider 的连通性自动拉黑
func (hcs *HealthCheckService) SetConnectivityAutoBlacklist(platform string, providerID int64, enabled bool) error {
	providers, err := hcs.providerService.LoadProviders(platform)
	if err != nil {
		return fmt.Errorf("加载供应商失败: %w", err)
	}

	found := false
	for i := range providers {
		if providers[i].ID == providerID {
			// 前置条件检查：必须先启用可用性监控
			if enabled && !providers[i].AvailabilityMonitorEnabled {
				return fmt.Errorf("请先在可用性页面启用监控")
			}
			providers[i].ConnectivityAutoBlacklist = enabled
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到供应商 ID: %d", providerID)
	}

	if err := hcs.providerService.SaveProviders(platform, providers); err != nil {
		return fmt.Errorf("保存供应商配置失败: %w", err)
	}

	log.Printf("[HealthCheck] Provider %d 自动拉黑已%s", providerID, map[bool]string{true: "启用", false: "禁用"}[enabled])
	return nil
}

// SaveAvailabilityConfig 保存 Provider 的可用性高级配置
func (hcs *HealthCheckService) SaveAvailabilityConfig(platform string, providerID int64, config *AvailabilityConfig) error {
	providers, err := hcs.providerService.LoadProviders(platform)
	if err != nil {
		return fmt.Errorf("加载供应商失败: %w", err)
	}

	found := false
	for i := range providers {
		if providers[i].ID == providerID {
			providers[i].AvailabilityConfig = normalizeAvailabilityConfig(config, &providers[i], platform)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到供应商 ID: %d", providerID)
	}

	if err := hcs.providerService.SaveProviders(platform, providers); err != nil {
		return fmt.Errorf("保存供应商配置失败: %w", err)
	}

	log.Printf("[HealthCheck] Provider %d 高级配置已保存", providerID)
	return nil
}

// CleanupOldRecords 清理过期的历史记录（保留最近 N 天）
func (hcs *HealthCheckService) CleanupOldRecords(daysToKeep int) (int64, error) {
	if daysToKeep <= 0 {
		daysToKeep = 7 // 默认保留 7 天
	}

	db, err := xdb.DB("default")
	if err != nil {
		return 0, fmt.Errorf("获取数据库连接失败: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -daysToKeep)

	result, err := db.Exec(`DELETE FROM health_check_history WHERE checked_at < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("清理历史记录失败: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("[HealthCheck] 已清理 %d 条过期历史记录", rowsAffected)
	}

	return rowsAffected, nil
}
