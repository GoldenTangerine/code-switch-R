/**
 * @name: 托盘共享快照
 * @Descripttion: 在窗口之外维护托盘供应商展示数据并向 CodeNotch 发布只读快照。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-09-08 16:50:00
 * @LastEditTime: 2026-09-08 16:50:00
 * @FilePath: services/traysnapshotservice.go
 */
package services

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type TraySnapshot struct {
	Codenotch   *CodenotchSnapshotInfo `json:"codenotch,omitempty"`
	Version     int                    `json:"version"`
	Session     string                 `json:"session"`
	Sequence    uint64                 `json:"sequence"`
	HeartbeatAt int64                  `json:"heartbeatAt"`
	Platforms   []TraySnapshotPlatform `json:"platforms"`
}

type TraySnapshotPlatform struct {
	SessionBindings []TraySessionBinding   `json:"sessionBindings,omitempty"`
	Platform        string                 `json:"platform"`
	Name            string                 `json:"name"`
	Icon            string                 `json:"icon"`
	Error           bool                   `json:"error"`
	Providers       []TraySnapshotProvider `json:"providers"`
}

type TraySnapshotProvider struct {
	ProviderID        string              `json:"providerId"`
	ProviderName      string              `json:"providerName"`
	Icon              string              `json:"icon"`
	ActiveRequests    int                 `json:"activeRequests"`
	Status            string              `json:"status"`
	QuotaState        string              `json:"quotaState,omitempty"`
	QuotaAutoDisabled bool                `json:"quotaAutoDisabled,omitempty"`
	Loading           bool                `json:"loading"`
	UpdatedAt         int64               `json:"updatedAt"`
	Quotas            []TraySnapshotQuota `json:"quotas"`
	Stats             *ProviderDailyStat  `json:"stats"`
}

type TraySnapshotQuota struct {
	ProviderQuotaQueryItem
	DisplayKind string `json:"displayKind"`
}

// This internal type must never be marshalled into the public snapshot.
type trayProviderInput struct {
	ref         string
	provider    Provider
	fingerprint [32]byte
	uniqueName  bool
}

type trayInputCache struct {
	fingerprint [32]byte
	inputs      []trayProviderInput
}

type trayStatsCache struct {
	mu      sync.Mutex
	updated time.Time
	stats   []ProviderDailyStat
	err     error
	byID    map[string]ProviderDailyStat
	byName  map[string]ProviderDailyStat
}

type trayDetailResult struct {
	requestID   uint64
	key         string
	fingerprint [32]byte
	quotas      []TraySnapshotQuota
	stats       *ProviderDailyStat
	updated     time.Time
}

type trayDetailCache struct {
	platform     string
	lastTraySeen time.Time
	cancel       context.CancelFunc
	requestID    uint64
	scheduled    uint64
	fingerprint  [32]byte
	inflight     bool
	result       trayDetailResult
}

type TraySnapshotService struct {
	integration codenotchIntegration
	requestID   uint64
	providers   *ProviderService
	gemini      *GeminiService
	settings    *AppSettingsService
	custom      *CustomCliService
	concurrency *ProviderConcurrencyService
	query       *ProviderQuotaQueryService
	logs        *LogService
	mu          sync.RWMutex
	snapshot    TraySnapshot
	cancel      context.CancelFunc
	done        chan struct{}
	path        string
	cache       map[string]*trayDetailCache
	results     chan trayDetailResult
	workers     chan struct{}
	wg          sync.WaitGroup
	inputsCache map[string]trayInputCache
	statsMu     sync.Mutex
	statsCache  map[string]*trayStatsCache
}

func NewTraySnapshotService(providers *ProviderService, gemini *GeminiService, settings *AppSettingsService,
	custom *CustomCliService, concurrency *ProviderConcurrencyService, query *ProviderQuotaQueryService, logs *LogService) *TraySnapshotService {
	return &TraySnapshotService{providers: providers, gemini: gemini, settings: settings, custom: custom,
		concurrency: concurrency, query: query, logs: logs,
		snapshot: TraySnapshot{Version: 1, Session: uuid.NewString(), Platforms: []TraySnapshotPlatform{}},
		cache:    make(map[string]*trayDetailCache), results: make(chan trayDetailResult, 64), workers: make(chan struct{}, 4),
		inputsCache: make(map[string]trayInputCache), statsCache: make(map[string]*trayStatsCache)}
}

func (s *TraySnapshotService) Start() error {
	var publishErr error
	if runtime.GOOS == "darwin" {
		dir, err := os.UserCacheDir()
		if err == nil {
			s.path = filepath.Join(dir, "code-switch", "tray-snapshot-v1.json")
			err = os.MkdirAll(filepath.Dir(s.path), 0700)
			if err == nil {
				err = os.Chmod(filepath.Dir(s.path), 0700)
			}
		}
		publishErr = err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel, s.done = cancel, make(chan struct{})
	go s.run(ctx, s.collect)
	return publishErr
}

func (s *TraySnapshotService) Stop() {
	if s.cancel == nil {
		return
	}
	s.cancel()
	<-s.done
	if s.path != "" {
		// A newer process may already own the shared path.
		data, err := os.ReadFile(s.path)
		var current TraySnapshot
		if err == nil && json.Unmarshal(data, &current) == nil && current.Session == s.snapshot.Session {
			_ = os.Remove(s.path)
		}
	}
	s.wg.Wait()
	s.removeCodenotchData()
	s.cache = make(map[string]*trayDetailCache)
	s.inputsCache = make(map[string]trayInputCache)
	s.statsMu.Lock()
	s.statsCache = make(map[string]*trayStatsCache)
	s.statsMu.Unlock()
	s.integration.wanted, s.integration.platforms = nil, nil
	s.results = make(chan trayDetailResult, 64)
}

func (s *TraySnapshotService) GetSnapshot() TraySnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Published slices are immutable; each tick allocates its next snapshot.
	return s.snapshot
}

func (s *TraySnapshotService) run(ctx context.Context, collect func(context.Context, time.Time) []TraySnapshotPlatform) {
	defer close(s.done)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	var previous [32]byte
	var published time.Time
	var lastWriteError time.Time
	for {
		now := time.Now()
		platforms := collect(ctx, now)
		data, err := json.Marshal(platforms)
		if err == nil {
			metadata, _ := json.Marshal(s.integration.info)
			fingerprint := sha256.Sum256(append(data, metadata...))
			if fingerprint != previous || now.Sub(published) >= time.Second {
				s.mu.Lock()
				s.snapshot.Sequence++
				s.snapshot.HeartbeatAt = now.UnixMilli()
				s.snapshot.Platforms = platforms
				s.snapshot.Codenotch = s.integration.info
				next := s.snapshot
				s.mu.Unlock()
				if s.path != "" {
					if err := writeTraySnapshot(s.path, next); err != nil {
						if now.Sub(lastWriteError) >= time.Minute || lastWriteError.IsZero() {
							log.Print("托盘共享快照写入失败")
							lastWriteError = now
						}
					}
				}
				previous, published = fingerprint, now
			}
		}
		select {
		case <-ctx.Done():
			return
		case result := <-s.results:
			s.acceptTrayDetails(result)
		case <-ticker.C:
		}
	}
}

func (s *TraySnapshotService) acceptTrayDetails(result trayDetailResult) {
	if entry := s.cache[result.key]; entry != nil && entry.fingerprint == result.fingerprint && entry.requestID == result.requestID {
		entry.inflight, entry.result = false, result
		if entry.cancel != nil {
			entry.cancel()
			entry.cancel = nil
		}
	}
}

func writeTraySnapshot(path string, snapshot TraySnapshot) error {
	return writeCodenotchJSON(path, snapshot)
}

func writeCodenotchJSON(path string, snapshot any) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return writeCodenotchData(path, data)
}

func writeCodenotchData(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".tray-snapshot-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func (s *TraySnapshotService) platforms() []TraySnapshotPlatform {
	settings, err := s.settings.GetAppSettings()
	if err != nil {
		return []TraySnapshotPlatform{}
	}
	result := []TraySnapshotPlatform{}
	for _, platform := range settings.HomeProviderTabs {
		name, icon := "", ""
		switch platform {
		case "claude":
			name, icon = "Claude Code", "claude"
		case "codex":
			name, icon = "Codex", "openai"
		case "gemini":
			name, icon = "Gemini", "gemini"
		case "grokbuild":
			name, icon = "Grok Build", "grok"
		}
		if name != "" {
			result = append(result, TraySnapshotPlatform{Platform: platform, Name: name, Icon: icon, Providers: []TraySnapshotProvider{}})
		}
	}
	for _, platform := range settings.HomeProviderTabs {
		if platform != "others" {
			continue
		}
		tools, err := s.custom.ListTools()
		if err != nil {
			break
		}
		for _, tool := range tools {
			if tool.ID == "" {
				continue
			}
			name := tool.Name
			if name == "" {
				name = tool.ID
			}
			result = append(result, TraySnapshotPlatform{Platform: "custom:" + tool.ID, Name: name, Icon: "others", Providers: []TraySnapshotProvider{}})
		}
	}
	return result
}

func (s *TraySnapshotService) inputs(platform string) ([]trayProviderInput, error) {
	result := []trayProviderInput{}
	if platform == "gemini" {
		if s.gemini == nil {
			return result, nil
		}
		s.gemini.mu.Lock()
		for _, p := range s.gemini.providers {
			result = append(result, trayProviderInput{ref: providerRefFromStringID(p.ID, p.Name), provider: cloneProvider(Provider{
				Name: p.Name, Icon: "gemini", APIURL: p.BaseURL, APIKey: p.APIKey, Enabled: p.Enabled, QuotaAutoDisabled: p.QuotaAutoDisabled,
				BudgetQuotaSettings: p.BudgetQuotaSettings, BudgetQuotaUsedAdjustments: p.BudgetQuotaUsedAdjustments,
				ProviderQuotaQueryType: p.ProviderQuotaQueryType, ProviderQuotaQueryConfig: p.ProviderQuotaQueryConfig})})
		}
		s.gemini.mu.Unlock()
		fingerprintTrayInputs(result)
		return result, nil
	}
	s.providers.snapshotMu.RLock()
	config, exists := s.providers.snapshots[platform]
	s.providers.snapshotMu.RUnlock()
	if cached, ok := s.inputsCache[platform]; exists && ok && cached.fingerprint == config.fingerprint {
		return cached.inputs, nil
	}
	providers, err := s.providers.LoadProviders(platform)
	if err != nil {
		return nil, err
	}
	for _, p := range providers {
		result = append(result, trayProviderInput{ref: providerRefFromNumericID(p.ID, p.Name), provider: p})
	}
	fingerprintTrayInputs(result)
	if exists {
		s.inputsCache[platform] = trayInputCache{fingerprint: config.fingerprint, inputs: result}
	}
	return result, nil
}

func fingerprintTrayInputs(inputs []trayProviderInput) {
	names := make(map[string]int)
	for _, input := range inputs {
		names[strings.ToLower(strings.TrimSpace(input.provider.Name))]++
	}
	for i := range inputs {
		p := inputs[i].provider
		inputs[i].uniqueName = names[strings.ToLower(strings.TrimSpace(p.Name))] == 1
		// Only query inputs invalidate details; display and relay settings do not.
		encoded, _ := json.Marshal([]any{p.Name, inputs[i].uniqueName, p.APIURL, p.APIKey, p.BudgetQuotaSettings,
			p.BudgetQuotaUsedAdjustments, p.ProviderQuotaQueryType, p.ProviderQuotaQueryConfig})
		inputs[i].fingerprint = sha256.Sum256(encoded)
	}
}

func matchTrayProvider(inputs []trayProviderInput, item TraySnapshotProvider) *trayProviderInput {
	var match *trayProviderInput
	for i := range inputs {
		if item.ProviderID != "" {
			if inputs[i].ref == item.ProviderID {
				return &inputs[i]
			}
		} else if inputs[i].provider.Name == item.ProviderName {
			if match != nil {
				return nil
			}
			match = &inputs[i]
		}
	}
	return match
}

func trayActivities(state TrayProviderRuntimeState) []TraySnapshotProvider {
	result := []TraySnapshotProvider{}
	if state.Error {
		return result
	}
	for _, status := range state.Statuses {
		if (strings.TrimSpace(status.ProviderID) == "" && strings.TrimSpace(status.ProviderName) == "") || status.ActiveRequests <= 0 {
			continue
		}
		result = append(result, TraySnapshotProvider{ProviderID: strings.TrimSpace(status.ProviderID), ProviderName: status.ProviderName,
			ActiveRequests: status.ActiveRequests, Status: "active", Quotas: []TraySnapshotQuota{}})
	}
	if len(result) == 0 && state.DefaultProvider != nil {
		p := state.DefaultProvider
		if p.ProviderID != "" || p.ProviderName != "" {
			result = append(result, TraySnapshotProvider{ProviderID: p.ProviderID, ProviderName: p.ProviderName, Status: "default", Quotas: []TraySnapshotQuota{}})
		}
	}
	return result
}

func (s *TraySnapshotService) collect(ctx context.Context, now time.Time) []TraySnapshotPlatform {
	platforms := s.platforms()
	ids := make([]string, len(platforms))
	for i := range platforms {
		ids[i] = platforms[i].Platform
	}
	states := s.concurrency.GetTrayProviderRuntimeStatesBatch(ids)
	wanted := make(map[string]trayProviderInput)
	for i := range platforms {
		platform := &platforms[i]
		state, exists := states[platform.Platform]
		platform.Error = !exists || state.Error
		if platform.Error {
			continue
		}
		inputs, err := s.inputs(platform.Platform)
		if err != nil {
			platform.Error = true
			continue
		}
		platform.Providers = trayActivities(state)
		platform.SessionBindings = s.concurrency.hookSessionBindings(platform.Platform, now)
		byID, byName := make(map[string]trayProviderInput, len(inputs)), make(map[string]trayProviderInput)
		for _, input := range inputs {
			byID[input.ref] = input
			if input.uniqueName {
				byName[input.provider.Name] = input
			}
		}
		for j := range platform.Providers {
			item := &platform.Providers[j]
			input, exists := byID[item.ProviderID]
			if item.ProviderID == "" {
				input, exists = byName[item.ProviderName]
			}
			if !exists {
				item.Icon = "openai"
				continue
			}
			s.decorateTrayProvider(platform.Platform, item, input, now, wanted)
			s.cache[trayDetailKey(platform.Platform, input.ref)].lastTraySeen = now
		}
	}
	s.collectCodenotch(ctx, now, platforms)
	for key, input := range s.integration.wanted {
		// A configuration edit can land between tray and full collection.
		// Query the inputs matching the latest decorated cache generation.
		if entry := s.cache[key]; entry != nil && entry.fingerprint == input.fingerprint {
			wanted[key] = input
		}
	}
	s.scheduleTrayDetails(ctx, now, wanted)
	return platforms
}

func trayDetailKey(platform, ref string) string {
	return fmt.Sprintf("%d:%s:%s", len(platform), platform, ref)
}

func (s *TraySnapshotService) decorateTrayProvider(platform string, item *TraySnapshotProvider, input trayProviderInput, now time.Time, wanted map[string]trayProviderInput) {
	item.ProviderID, item.ProviderName, item.Icon = input.ref, input.provider.Name, input.provider.Icon
	if item.Icon == "" {
		item.Icon = "openai"
	}
	item.QuotaAutoDisabled = input.provider.QuotaAutoDisabled
	key := trayDetailKey(platform, input.ref)
	wanted[key] = input
	entry := s.cache[key]
	if entry == nil || entry.fingerprint != input.fingerprint {
		if entry != nil && entry.cancel != nil {
			entry.cancel()
		}
		entry = &trayDetailCache{platform: platform, fingerprint: input.fingerprint}
		s.cache[key] = entry
	}
	item.Loading = entry.result.updated.IsZero()
	if !item.Loading {
		item.UpdatedAt, item.Quotas, item.Stats = entry.result.updated.UnixMilli(), entry.result.quotas, entry.result.stats
	}
	item.QuotaState = codenotchQuotaState(item.QuotaAutoDisabled, item.Quotas)
}

func (s *TraySnapshotService) scheduleTrayDetails(ctx context.Context, now time.Time, wanted map[string]trayProviderInput) {
	usedPlatforms := make(map[string]bool)
	for key := range wanted {
		if entry := s.cache[key]; entry != nil {
			usedPlatforms[entry.platform] = true
		}
	}
	retained := make([]string, 0)
	for key, entry := range s.cache {
		if _, exists := wanted[key]; !exists {
			if entry.cancel != nil {
				entry.cancel()
			}
			// Cancelled workers may already have queued a result. Retain only
			// recent tray results, never the inputs or full-mode-only demand.
			entry.cancel, entry.inflight, entry.requestID = nil, false, 0
			if now.Sub(entry.lastTraySeen) < time.Minute && !entry.result.updated.IsZero() && now.Sub(entry.result.updated) < time.Minute {
				retained = append(retained, key)
			} else {
				delete(s.cache, key)
			}
		}
	}
	const maxInactiveTrayDetails = 128
	if len(retained) > maxInactiveTrayDetails {
		sort.Slice(retained, func(a, b int) bool {
			x, y := s.cache[retained[a]].lastTraySeen, s.cache[retained[b]].lastTraySeen
			if x.Equal(y) {
				return retained[a] < retained[b]
			}
			return x.After(y)
		})
		for _, key := range retained[maxInactiveTrayDetails:] {
			delete(s.cache, key)
		}
	}
	for platform := range s.inputsCache {
		if !usedPlatforms[platform] {
			delete(s.inputsCache, platform)
		}
	}
	s.statsMu.Lock()
	for platform := range s.statsCache {
		if !usedPlatforms[platform] {
			delete(s.statsCache, platform)
		}
	}
	s.statsMu.Unlock()
	if len(s.workers) >= cap(s.workers) {
		return
	}
	pending := make([]string, 0, len(wanted))
	for key := range wanted {
		entry := s.cache[key]
		if entry != nil && !entry.inflight && (entry.result.updated.IsZero() || now.Sub(entry.result.updated) >= time.Minute) {
			pending = append(pending, key)
		}
	}
	// Oldest scheduled work goes first, including providers beyond the first
	// worker batch. The queue holds identities, not copies of account secrets.
	sort.Slice(pending, func(a, b int) bool {
		x, y := s.cache[pending[a]].scheduled, s.cache[pending[b]].scheduled
		if x == y {
			return pending[a] < pending[b]
		}
		return x < y
	})
	for _, key := range pending {
		select {
		case s.workers <- struct{}{}:
			entry := s.cache[key]
			s.requestID++
			entry.requestID, entry.scheduled, entry.inflight = s.requestID, s.requestID, true
			requestCtx, cancel := context.WithCancel(ctx)
			entry.cancel = cancel
			s.wg.Add(1)
			go s.loadDetails(requestCtx, entry.platform, wanted[key], key, entry.fingerprint, entry.requestID)
		default:
			return
		}
	}
}

func (s *TraySnapshotService) loadDetails(ctx context.Context, platform string, input trayProviderInput, key string, fingerprint [32]byte, requestIDs ...uint64) {
	defer s.wg.Done()
	defer func() { <-s.workers }()
	if ctx.Err() != nil {
		return
	}
	result := trayDetailResult{key: key, fingerprint: fingerprint, quotas: s.loadQuotas(ctx, platform, input, time.Now())}
	if len(requestIDs) > 0 {
		result.requestID = requestIDs[0]
	}
	if ctx.Err() != nil {
		return
	}
	stats, err := s.dailyStat(ctx, platform, input)
	if err == nil {
		result.stats = &stats
	}
	result.updated = time.Now()
	select {
	case s.results <- result:
	case <-ctx.Done():
	}
}

func (s *TraySnapshotService) dailyStats(ctx context.Context, platform string) ([]ProviderDailyStat, error) {
	if s.logs == nil {
		return nil, fmt.Errorf("statistics unavailable")
	}
	s.statsMu.Lock()
	cached := s.statsCache[platform]
	if cached == nil {
		cached = &trayStatsCache{}
		s.statsCache[platform] = cached
	}
	s.statsMu.Unlock()
	cached.mu.Lock()
	defer cached.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	now := time.Now()
	if now.Sub(cached.updated) < time.Minute &&
		cached.updated.Format("2006-01-02") == now.Format("2006-01-02") {
		return cached.stats, cached.err
	}
	stats, err := s.logs.ProviderDailyStats(platform)
	cached.updated, cached.stats, cached.err = now, stats, err
	cached.byID, cached.byName = make(map[string]ProviderDailyStat, len(stats)), make(map[string]ProviderDailyStat)
	for _, stat := range stats {
		if stat.ProviderID != "" {
			cached.byID[stat.ProviderID] = stat
		} else {
			cached.byName[strings.ToLower(strings.TrimSpace(stat.Provider))] = stat
		}
	}
	return stats, err
}

func (s *TraySnapshotService) dailyStat(ctx context.Context, platform string, input trayProviderInput) (ProviderDailyStat, error) {
	result := ProviderDailyStat{ProviderID: input.ref, Provider: input.provider.Name}
	if _, err := s.dailyStats(ctx, platform); err != nil {
		return result, err
	}
	s.statsMu.Lock()
	cached := s.statsCache[platform]
	s.statsMu.Unlock()
	if cached == nil {
		return result, nil
	}
	cached.mu.Lock()
	defer cached.mu.Unlock()
	if stat, ok := cached.byID[input.ref]; ok {
		return stat, nil
	}
	if input.uniqueName {
		if stat, ok := cached.byName[strings.ToLower(strings.TrimSpace(input.provider.Name))]; ok {
			return stat, nil
		}
	}
	return result, nil
}

func trayQuotaError(key string) TraySnapshotQuota {
	return TraySnapshotQuota{ProviderQuotaQueryItem: ProviderQuotaQueryItem{Key: key, InvalidMessage: "Quota unavailable", ValueMode: "currency"}, DisplayKind: "error"}
}

func (s *TraySnapshotService) loadQuotas(ctx context.Context, platform string, input trayProviderInput, now time.Time) []TraySnapshotQuota {
	p := input.provider
	if hasRemoteProviderQuotaConfig(p.ProviderQuotaQueryType, p.ProviderQuotaQueryConfig) {
		result := s.query.queryQuotaContext(ctx, p.ProviderQuotaQueryType, p.APIURL, p.APIKey, p.ProviderQuotaQueryConfig)
		quotas := []TraySnapshotQuota{}
		for _, raw := range result.Items {
			item := raw
			item.Used, item.Total = normalizeBudgetRawUsed(item.Used), normalizeBudgetRawUsed(item.Total)
			item.Unlimited = result.QueryType == "sub2api" && item.Unlimited
			kind := "progress"
			if item.ValueMode != "count" {
				item.ValueMode = "currency"
			}
			if item.ValueMode == "currency" && item.NextReset == "" {
				kind = "balance"
			}
			if item.InvalidMessage != "" {
				item.InvalidMessage, kind = "Quota unavailable", "error"
			}
			// Script diagnostics and arbitrary response extras can include private request data.
			item.Extra = ""
			quotas = append(quotas, TraySnapshotQuota{ProviderQuotaQueryItem: item, DisplayKind: kind})
		}
		if len(quotas) > 0 {
			return quotas
		}
		return []TraySnapshotQuota{trayQuotaError("provider_quota_error")}
	}
	return s.loadBudgetQuotas(platform, input, now)
}

func (s *TraySnapshotService) loadBudgetQuotas(platform string, input trayProviderInput, now time.Time) []TraySnapshotQuota {
	result := []TraySnapshotQuota{}
	p := input.provider
	if p.BudgetQuotaSettings == nil {
		return result
	}
	settings := *p.BudgetQuotaSettings
	adjust := BudgetQuotaAdjustments{}
	if p.BudgetQuotaUsedAdjustments != nil {
		adjust = *p.BudgetQuotaUsedAdjustments
	}
	for _, quota := range []struct {
		key        string
		setting    BudgetQuotaSetting
		adjustment float64
	}{
		{"five_hour", settings.FiveHour, adjust.FiveHour}, {"daily", settings.Daily, adjust.Daily},
		{"weekly", settings.Weekly, adjust.Weekly}, {"monthly", settings.Monthly, adjust.Monthly}, {"total", settings.Total, adjust.Total},
	} {
		if quota.setting.Total <= 0 {
			continue
		}
		item := ProviderQuotaQueryItem{Key: quota.key, Total: quota.setting.Total, Active: true, ValueMode: "currency"}
		var used float64
		var err error
		switch quota.key {
		case "total":
			used, err = s.logs.CostByProvider(platform, input.ref, p.Name)
		case "five_hour":
			var status FiveHourQuotaStatus
			status, err = s.logs.ResolveFiveHourQuotaStatusByProvider(platform, input.ref, p.Name)
			used, item.Active, item.NextReset = status.Used, status.Active, status.NextReset
		default:
			start, next := trayBudgetWindow(quota.key, quota.setting, now)
			used, err = s.logs.CostSinceByProvider(start.Format(time.RFC3339), platform, input.ref, p.Name)
			item.NextReset = next.Format(time.RFC3339)
		}
		if err != nil {
			result = append(result, trayQuotaError(quota.key))
			continue
		}
		if item.Active {
			item.Used = trayBudgetUsed(used, quota.adjustment)
		}
		result = append(result, TraySnapshotQuota{ProviderQuotaQueryItem: item, DisplayKind: "progress"})
	}
	return result
}

func trayBudgetUsed(used, adjustment float64) float64 {
	// Match the UI's cents rounding before and after calibration.
	return roundTrayBudget(ComputeBudgetUsed(roundTrayBudget(normalizeBudgetRawUsed(used)), adjustment))
}

func roundTrayBudget(value float64) float64 {
	return math.Floor((value+math.Copysign(2.220446049250313e-16, value))*100+0.5) / 100
}

func trayBudgetWindow(key string, setting BudgetQuotaSetting, now time.Time) (time.Time, time.Time) {
	config := BuildBudgetUsageConfig(true, key, setting.RefreshTime, setting.RefreshDay, setting.RefreshMonthDay)
	start := ResolveBudgetCycleStart(config, now)
	next := start.AddDate(0, 0, 1)
	if key == "weekly" {
		next = start.AddDate(0, 0, 7)
	}
	if key == "monthly" {
		hour, minute := parseBudgetRefreshTime(setting.RefreshTime)
		month := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location()).AddDate(0, 1, 0)
		next = resolveBudgetMonthlyRefreshPoint(month.Year(), month.Month(), config.RefreshMonthDay, hour, minute, start.Location())
	}
	return start, next
}

func codenotchQuotaState(autoDisabled bool, quotas []TraySnapshotQuota) string {
	if autoDisabled {
		return "exhausted"
	}
	items := make([]ProviderQuotaQueryItem, 0, len(quotas))
	for _, quota := range quotas {
		if math.IsNaN(quota.Used) || math.IsInf(quota.Used, 0) || math.IsNaN(quota.Total) || math.IsInf(quota.Total, 0) || quota.Used < 0 || quota.Total < 0 {
			continue
		}
		if quota.DisplayKind != "progress" && quota.DisplayKind != "balance" {
			continue
		}
		items = append(items, quota.ProviderQuotaQueryItem)
	}
	exhausted, valid := quotaItemsExhausted(items)
	if exhausted {
		return "exhausted"
	}
	if valid {
		return "available"
	}
	return "unknown"
}
