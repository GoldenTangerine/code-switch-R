/**
 * @name: Codex 被动额度采样
 * @Descripttion: 从代理响应采集 Codex 套餐窗口并缓存用量和重置时间。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-06 17:48:46
 * @LastEditTime: 2026-11-06 17:48:46
 * @FilePath: services/providerquota_codex_passive.go
 */
package services

import (
	"context"
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
	codexPassiveQuotaPending   = "codex_passive_pending"
	codexPassiveQuotaStale     = "codex_passive_stale"
	codexPassiveQuotaMaxPeriod = 366 * 24 * time.Hour
)

type codexPassiveQuotaWindow struct {
	UsedPercent   float64 `json:"used_percent"`
	HasUsage      bool    `json:"has_usage"`
	WindowMinutes int64   `json:"window_minutes,omitempty"`
	ResetAt       int64   `json:"reset_at,omitempty"`
	SampledAt     int64   `json:"sampled_at"`
}

type codexPassiveQuotaSnapshot map[string]codexPassiveQuotaWindow

type codexPassiveQuotaStore struct {
	mu        sync.RWMutex
	snapshots map[string]codexPassiveQuotaSnapshot
	queue     *DBWriteQueue
}

func (s *ProviderQuotaQueryService) initCodexPassiveQuotaDB() error {
	if GlobalDBQueue == nil || s.codexPassive == nil {
		return nil
	}
	db, err := xdb.DB("default")
	if err != nil {
		return err
	}
	if err := GlobalDBQueue.Exec(`CREATE TABLE IF NOT EXISTS provider_codex_quota_samples (
		cache_key TEXT PRIMARY KEY,
		snapshot TEXT NOT NULL
	)`); err != nil {
		return err
	}
	rows, err := db.Query("SELECT cache_key, snapshot FROM provider_codex_quota_samples")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			return err
		}
		var snapshot codexPassiveQuotaSnapshot
		if json.Unmarshal([]byte(raw), &snapshot) == nil && snapshot != nil {
			s.codexPassive.snapshots[key] = snapshot
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.codexPassive.queue = GlobalDBQueue
	return nil
}

func parseCodexPassiveQuotaInt(raw string, min, max int64) (int64, bool) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return value, err == nil && value >= min && value <= max
}

func (s *ProviderQuotaQueryService) sampleCodexPassiveQuota(baseURL, apiKey string, headers http.Header, now time.Time) {
	if s == nil || s.codexPassive == nil || strings.TrimSpace(baseURL) == "" || strings.TrimSpace(apiKey) == "" {
		return
	}
	store := s.codexPassive
	store.mu.Lock()
	defer store.mu.Unlock()
	key := passiveQuotaCacheKey(baseURL, apiKey)
	snapshot := store.snapshots[key]
	if snapshot == nil {
		snapshot = make(codexPassiveQuotaSnapshot)
	}
	changed := false
	for _, slot := range []string{"primary", "secondary"} {
		prefix := "x-codex-" + slot + "-"
		window := snapshot[slot]
		minutes, hasMinutes := parseCodexPassiveQuotaInt(headers.Get(prefix+"window-minutes"), 1, int64(codexPassiveQuotaMaxPeriod/time.Minute))
		resetAfter, hasReset := parseCodexPassiveQuotaInt(headers.Get(prefix+"reset-after-seconds"), 0, int64(codexPassiveQuotaMaxPeriod/time.Second))
		used, err := strconv.ParseFloat(strings.TrimSpace(headers.Get(prefix+"used-percent")), 64)
		hasUsage := err == nil && !math.IsNaN(used) && !math.IsInf(used, 0) && used >= 0
		if !hasMinutes && !hasReset && !hasUsage {
			continue
		}
		resetAt := now.Unix() + resetAfter
		windowChanged := hasMinutes && window.WindowMinutes > 0 && minutes != window.WindowMinutes
		// 倒计时取整可能使同一窗口的重置时间略有偏移；明显变化时不能复用旧使用率。
		resetChanged := hasReset && (window.ResetAt == 0 || resetAt > window.ResetAt+60 || resetAt < window.ResetAt-60)
		if windowChanged || resetChanged || (hasUsage && !hasReset && window.ResetAt > 0 && window.ResetAt <= now.Unix()) {
			window = codexPassiveQuotaWindow{WindowMinutes: window.WindowMinutes}
		}
		if hasMinutes {
			window.WindowMinutes = minutes
		}
		if hasReset {
			window.ResetAt = resetAt
		}
		if hasUsage {
			window.UsedPercent, window.HasUsage, window.SampledAt = used, true, now.Unix()
		}
		snapshot[slot] = window
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
		if err := store.queue.ExecCtx(ctx, `INSERT INTO provider_codex_quota_samples (cache_key, snapshot) VALUES (?, ?)
			ON CONFLICT(cache_key) DO UPDATE SET snapshot = excluded.snapshot`, key, string(raw)); err != nil {
			fmt.Printf("Codex 被动额度样本保存失败: %v\n", err)
		}
	}
}

func codexPassiveQuotaWindowLabel(minutes int64) string {
	switch {
	case minutes <= 0:
		return ""
	case minutes%(24*60) == 0:
		return fmt.Sprintf("%dd", minutes/(24*60))
	case minutes%60 == 0:
		return fmt.Sprintf("%dh", minutes/60)
	default:
		return fmt.Sprintf("%dm", minutes)
	}
}

func (s *ProviderQuotaQueryService) queryCodexPassiveQuota(baseURL, apiKey string, now time.Time) ([]ProviderQuotaQueryItem, error) {
	items := []ProviderQuotaQueryItem{}
	if s.codexPassive != nil {
		store := s.codexPassive
		store.mu.RLock()
		defer store.mu.RUnlock()
		snapshot := store.snapshots[passiveQuotaCacheKey(baseURL, apiKey)]
		for _, slot := range []string{"primary", "secondary"} {
			window, ok := snapshot[slot]
			if !ok || !window.HasUsage {
				continue
			}
			item := ProviderQuotaQueryItem{Key: "codex_" + slot, Label: codexPassiveQuotaWindowLabel(window.WindowMinutes),
				Used: window.UsedPercent, Total: 100, Active: true, ValueMode: "count", Unit: "%"}
			// 未返回窗口长度时只按 5 小时判断缺少重置时间的样本是否过期，不据此顺延重置时间。
			period, staleAfter := time.Duration(0), 5*time.Hour
			if window.WindowMinutes > 0 {
				period = time.Duration(window.WindowMinutes) * time.Minute
				staleAfter = period
			}
			if !applyPassiveQuotaWindowEstimate(&item, window.ResetAt, window.SampledAt, period, staleAfter, now) {
				item.Active, item.InvalidMessage = false, codexPassiveQuotaStale
			}
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		items = append(items, ProviderQuotaQueryItem{Key: codexPassiveQuotaPending, Label: "Codex", ValueMode: "count",
			Unit: "%", InvalidMessage: codexPassiveQuotaPending})
	}
	return items, nil
}

func (prs *ProviderRelayService) sampleCodexPassiveQuota(provider Provider, headers http.Header) {
	if prs == nil || prs.providerQuotaAutomation == nil || prs.providerQuotaAutomation.queryService == nil ||
		!hasRemoteProviderQuotaConfig(provider.ProviderQuotaQueryType, provider.ProviderQuotaQueryConfig) ||
		normalizeProviderQuotaQueryType(provider.ProviderQuotaQueryType) != ProviderQuotaQueryTypeTokenPlanCodexPassive {
		return
	}
	prs.providerQuotaAutomation.queryService.sampleCodexPassiveQuota(provider.APIURL, provider.APIKey, headers, time.Now())
}
