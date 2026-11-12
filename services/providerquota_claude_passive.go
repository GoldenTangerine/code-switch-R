/**
 * @name: Claude 被动额度采样
 * @Descripttion: 从代理响应采集 Claude 套餐窗口并通过数据库队列保存最近样本。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-06 16:35:58
 * @LastEditTime: 2026-11-06 16:35:58
 * @FilePath: services/providerquota_claude_passive.go
 */
package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/daodao97/xgo/xdb"
)

const (
	claudePassiveQuotaPending = "claude_passive_pending"
	claudePassiveQuotaStale   = "claude_passive_stale"

	passiveQuotaEstimateAged      = "aged"
	passiveQuotaEstimateProjected = "projected"
	passiveQuotaEstimateReset     = "reset"
	// 短于该周期的会话窗口（如 5 小时）在下一次请求时才开始计时，越过重置时间后不推算下一次重置。
	passiveQuotaSessionWindowLimit = 24 * time.Hour
	// 超过该时长没有新样本时，上游可能已被其他客户端继续消耗，用量只能作为参考。
	passiveQuotaAgedAfter = 5 * time.Minute
)

var claudePassiveQuotaWindows = []struct {
	header string
	key    string
	label  string
	period time.Duration
}{
	{"5h", "five_hour", "5h", 5 * time.Hour},
	{"7d", "weekly", "7d", 7 * 24 * time.Hour},
	{"7d_oi", "weekly_fable", "7d F", 7 * 24 * time.Hour},
}

type claudePassiveQuotaWindow struct {
	Utilization float64 `json:"utilization"`
	HasUsage    bool    `json:"has_usage"`
	ResetAt     int64   `json:"reset_at,omitempty"`
	SampledAt   int64   `json:"sampled_at"`
}

type claudePassiveQuotaSnapshot map[string]claudePassiveQuotaWindow

type claudePassiveQuotaStore struct {
	mu        sync.RWMutex
	snapshots map[string]claudePassiveQuotaSnapshot
	queue     *DBWriteQueue
}

func (s *ProviderQuotaQueryService) initDB() error {
	if GlobalDBQueue == nil {
		return nil
	}
	db, err := xdb.DB("default")
	if err != nil {
		return err
	}
	if err := GlobalDBQueue.Exec(`CREATE TABLE IF NOT EXISTS provider_claude_quota_samples (
		cache_key TEXT PRIMARY KEY,
		snapshot TEXT NOT NULL
	)`); err != nil {
		return err
	}
	rows, err := db.Query("SELECT cache_key, snapshot FROM provider_claude_quota_samples")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			return err
		}
		var snapshot claudePassiveQuotaSnapshot
		if json.Unmarshal([]byte(raw), &snapshot) == nil && snapshot != nil {
			s.claudePassive.snapshots[key] = snapshot
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.claudePassive.queue = GlobalDBQueue
	return s.initCodexPassiveQuotaDB()
}

// applyPassiveQuotaWindowEstimate 基于最后一次样本补全读数，样本超过 5 分钟时标记为可能不准确。
// 越过重置时间后：会话窗口只归零并清空倒计时，等待下一次请求开始计时；固定周期窗口（如 7 天）按周期顺延。
// period 为已知的窗口周期（未知传 0，此时不做推算）；staleAfter 用于缺少重置时间的样本判断过期。
// 仅在读取时推算，不回写样本，避免影响采样时的新旧窗口判断。返回 false 表示样本已无法继续使用，应视为过期。
func applyPassiveQuotaWindowEstimate(item *ProviderQuotaQueryItem, resetAt, sampledAt int64, period, staleAfter time.Duration, now time.Time) bool {
	periodSeconds, nowSeconds := int64(period/time.Second), now.Unix()
	if sampledAt > 0 {
		item.SampledAt = sampledAt * 1000
	}
	if resetAt > 0 {
		item.NextReset = time.Unix(resetAt, 0).UTC().Format(time.RFC3339)
	}
	switch {
	case resetAt <= 0:
		if nowSeconds >= sampledAt+int64(staleAfter/time.Second) {
			return false
		}
	case resetAt > nowSeconds:
	case periodSeconds > 0 && period < passiveQuotaSessionWindowLimit:
		item.NextReset, item.Used, item.Estimate = "", 0, passiveQuotaEstimateReset
	case periodSeconds > 0:
		resetAt += ((nowSeconds-resetAt)/periodSeconds + 1) * periodSeconds
		item.NextReset = time.Unix(resetAt, 0).UTC().Format(time.RFC3339)
		item.Used, item.Estimate = 0, passiveQuotaEstimateProjected
	default:
		// 已越过重置时间但不知道窗口周期，无法推算下一次重置。
		return false
	}
	if item.Estimate == "" && sampledAt > 0 && now.Sub(time.Unix(sampledAt, 0)) > passiveQuotaAgedAfter {
		item.Estimate = passiveQuotaEstimateAged
	}
	return true
}

func passiveQuotaCacheKey(baseURL, apiKey string) string {
	// 只保存摘要，避免把供应商地址中的私有路径或认证内容写入采样表。
	identity, _ := json.Marshal([]string{strings.TrimRight(strings.TrimSpace(baseURL), "/"), strings.TrimSpace(apiKey)})
	digest := sha256.Sum256(identity)
	return hex.EncodeToString(digest[:])
}

func parseClaudePassiveReset(raw string, now time.Time, period time.Duration) (int64, bool) {
	ts, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || ts <= 0 {
		return 0, false
	}
	if ts > 1e11 {
		ts /= 1000
	}
	reset := time.Unix(ts, 0)
	if reset.Before(now.Add(-period)) || reset.After(now.Add(period+24*time.Hour)) {
		return 0, false
	}
	return ts, true
}

func (s *ProviderQuotaQueryService) sampleClaudePassiveQuota(baseURL, apiKey string, headers http.Header, now time.Time) {
	if s == nil || s.claudePassive == nil || strings.TrimSpace(baseURL) == "" || strings.TrimSpace(apiKey) == "" {
		return
	}
	store := s.claudePassive
	store.mu.Lock()
	defer store.mu.Unlock()
	key := passiveQuotaCacheKey(baseURL, apiKey)
	snapshot := store.snapshots[key]
	if snapshot == nil {
		snapshot = make(claudePassiveQuotaSnapshot)
	}
	changed := false
	for _, spec := range claudePassiveQuotaWindows {
		prefix := "anthropic-ratelimit-unified-" + spec.header + "-"
		window := snapshot[spec.key]
		resetAt, hasReset := parseClaudePassiveReset(headers.Get(prefix+"reset"), now, spec.period)
		utilization, err := strconv.ParseFloat(strings.TrimSpace(headers.Get(prefix+"utilization")), 64)
		hasUsage := err == nil && !math.IsNaN(utilization) && !math.IsInf(utilization, 0) && utilization >= 0
		if !hasReset && !hasUsage {
			continue
		}
		// 新窗口只有 reset 时不能沿用旧窗口的使用率；只有使用率时也不能沿用已过期的 reset。
		if (hasReset && resetAt != window.ResetAt) || (hasUsage && !hasReset && window.ResetAt <= now.Unix()) {
			window = claudePassiveQuotaWindow{}
		}
		if hasReset {
			window.ResetAt = resetAt
		}
		if hasUsage {
			window.Utilization, window.HasUsage, window.SampledAt = utilization, true, now.Unix()
		}
		snapshot[spec.key] = window
		changed = true
	}
	if !changed {
		return
	}
	store.snapshots[key] = snapshot
	if store.queue != nil {
		raw, err := json.Marshal(snapshot)
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancel()
		if err := store.queue.ExecCtx(ctx, `INSERT INTO provider_claude_quota_samples (cache_key, snapshot) VALUES (?, ?)
			ON CONFLICT(cache_key) DO UPDATE SET snapshot = excluded.snapshot`, key, string(raw)); err != nil {
			fmt.Printf("Claude 被动额度样本保存失败: %v\n", err)
		}
	}
}

func (s *ProviderQuotaQueryService) queryClaudePassiveQuota(baseURL, apiKey string, now time.Time) ([]ProviderQuotaQueryItem, error) {
	items := []ProviderQuotaQueryItem{}
	if s.claudePassive != nil {
		store := s.claudePassive
		store.mu.RLock()
		defer store.mu.RUnlock()
		snapshot := store.snapshots[passiveQuotaCacheKey(baseURL, apiKey)]
		for _, spec := range claudePassiveQuotaWindows {
			window, ok := snapshot[spec.key]
			if !ok || !window.HasUsage {
				continue
			}
			item := ProviderQuotaQueryItem{Key: spec.key, Label: spec.label, Used: window.Utilization * 100,
				Total: 100, Active: true, ValueMode: "count", Unit: "%"}
			if !applyPassiveQuotaWindowEstimate(&item, window.ResetAt, window.SampledAt, spec.period, spec.period, now) {
				item.Active, item.InvalidMessage = false, claudePassiveQuotaStale
			}
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		items = append(items, ProviderQuotaQueryItem{Key: claudePassiveQuotaPending, Label: "Claude", ValueMode: "count",
			Unit: "%", InvalidMessage: claudePassiveQuotaPending})
	}
	return items, nil
}

func (prs *ProviderRelayService) sampleClaudePassiveQuota(provider Provider, headers http.Header) {
	if prs == nil || prs.providerQuotaAutomation == nil ||
		!hasRemoteProviderQuotaConfig(provider.ProviderQuotaQueryType, provider.ProviderQuotaQueryConfig) ||
		normalizeProviderQuotaQueryType(provider.ProviderQuotaQueryType) != ProviderQuotaQueryTypeTokenPlanClaudePassive {
		return
	}
	prs.providerQuotaAutomation.queryService.sampleClaudePassiveQuota(provider.APIURL, provider.APIKey, headers, time.Now())
}
