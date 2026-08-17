/**
 * @name: 供应商额度自动停用服务
 * @Descripttion: 根据远端额度查询结果维护供应商的自动停用与恢复状态
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-30 17:23:40
 * @LastEditTime: 2026-07-30 17:23:40
 * @FilePath: services/providerquotaautomationservice.go
 */
package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

const providerQuotaErrorTriggerCooldown = 30 * time.Second

type ProviderQuotaAutomationResult struct {
	Success                bool                     `json:"success"`
	QueryType              string                   `json:"queryType"`
	Items                  []ProviderQuotaQueryItem `json:"items"`
	Error                  string                   `json:"error,omitempty"`
	QueriedAt              int64                    `json:"queriedAt,omitempty"`
	ProviderEnabled        bool                     `json:"providerEnabled"`
	QuotaAutoDisabled      bool                     `json:"quotaAutoDisabled"`
	QuotaAutoDisablePaused bool                     `json:"quotaAutoDisablePaused"`
	StateChanged           bool                     `json:"stateChanged"`
}

type providerQuotaAutomationSnapshot struct {
	Kind                   string
	ProviderID             string
	ProviderName           string
	APIURL                 string
	APIKey                 string
	QueryType              string
	QueryConfig            *ProviderQuotaQueryConfig
	ConfigFingerprint      string
	Enabled                bool
	QuotaAutoDisabled      bool
	QuotaAutoDisablePaused bool
}

type providerQuotaAutomationCall struct {
	done   chan struct{}
	result *ProviderQuotaAutomationResult
}

type ProviderQuotaAutomationService struct {
	queryService       *ProviderQuotaQueryService
	providerService    *ProviderService
	geminiService      *GeminiService
	openCodeService    *OpenCodeService
	appSettings        *AppSettingsService
	notification       *NotificationService
	customCliService   *CustomCliService
	callMu             sync.Mutex
	calls              map[string]*providerQuotaAutomationCall
	triggerMu          sync.Mutex
	lastErrorTriggered map[string]time.Time
	recoveryMu         sync.Mutex
	recoveryStarted    bool
	recoveryStopped    bool
	recoveryWake       chan struct{}
	recoveryStop       chan struct{}
	recoveryDone       chan struct{}
}

func NewProviderQuotaAutomationService(
	queryService *ProviderQuotaQueryService,
	providerService *ProviderService,
	geminiService *GeminiService,
	openCodeService *OpenCodeService,
	appSettings *AppSettingsService,
	notification *NotificationService,
	customCliService *CustomCliService,
) *ProviderQuotaAutomationService {
	return &ProviderQuotaAutomationService{
		queryService:       queryService,
		providerService:    providerService,
		geminiService:      geminiService,
		openCodeService:    openCodeService,
		appSettings:        appSettings,
		notification:       notification,
		customCliService:   customCliService,
		calls:              make(map[string]*providerQuotaAutomationCall),
		lastErrorTriggered: make(map[string]time.Time),
	}
}

func (s *ProviderQuotaAutomationService) Start() error {
	if s == nil {
		return nil
	}
	s.recoveryMu.Lock()
	defer s.recoveryMu.Unlock()
	if s.recoveryStarted || s.recoveryStopped {
		return nil
	}
	s.recoveryStarted = true
	s.recoveryWake = make(chan struct{}, 1)
	s.recoveryStop = make(chan struct{})
	s.recoveryDone = make(chan struct{})
	go s.runRecoveryScheduler()
	return nil
}

func (s *ProviderQuotaAutomationService) Stop() error {
	if s == nil {
		return nil
	}
	s.recoveryMu.Lock()
	if !s.recoveryStarted || s.recoveryStopped {
		s.recoveryMu.Unlock()
		return nil
	}
	s.recoveryStopped = true
	close(s.recoveryStop)
	done := s.recoveryDone
	s.recoveryMu.Unlock()
	<-done
	return nil
}

func (s *ProviderQuotaAutomationService) HandleSettingsChanged(previous AppSettings, next AppSettings) {
	if previous.ProviderQuotaAutoDisableEnabled && !next.ProviderQuotaAutoDisableEnabled {
		s.restoreAllProviders()
	}
	s.signalRecoveryScheduler()
}

func (s *ProviderQuotaAutomationService) runRecoveryScheduler() {
	defer close(s.recoveryDone)
	for {
		settings, err := s.appSettings.GetAppSettings()
		if err != nil || !settings.ProviderQuotaAutoDisableEnabled {
			if !s.waitForRecoverySignal() {
				return
			}
			continue
		}

		snapshots, scanErr := s.listManagedProviderSnapshots()
		if scanErr != nil {
			fmt.Printf("[ProviderQuotaRecovery] 扫描额度状态失败: %v\n", scanErr)
		}
		managedCount := len(snapshots)
		snapshots = nil
		if managedCount == 0 && scanErr == nil {
			if !s.waitForRecoverySignal() {
				return
			}
			continue
		}

		interval := time.Duration(normalizeProviderQuotaRecoveryIntervalSeconds(settings.QuotaRecoveryIntervalSeconds)) * time.Second
		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
			currentSnapshots, currentScanErr := s.listManagedProviderSnapshots()
			if currentScanErr != nil {
				fmt.Printf("[ProviderQuotaRecovery] 刷新额度状态失败: %v\n", currentScanErr)
			}
			recoveredNames := s.runRecoveryPass(currentSnapshots)
			if len(recoveredNames) > 0 && s.notification != nil {
				s.notification.NotifyProviderQuotaRecovered(recoveredNames)
			}
		case <-s.recoveryWake:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-s.recoveryStop:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
	}
}

func (s *ProviderQuotaAutomationService) waitForRecoverySignal() bool {
	select {
	case <-s.recoveryWake:
		return true
	case <-s.recoveryStop:
		return false
	}
}

func (s *ProviderQuotaAutomationService) runRecoveryPass(snapshots []providerQuotaAutomationSnapshot) []string {
	recoveredNames := make([]string, 0, len(snapshots))
	for _, snapshot := range snapshots {
		select {
		case <-s.recoveryStop:
			return nil
		default:
		}
		result := s.CheckProviderQuota(snapshot.Kind, snapshot.ProviderID)
		if result.Success && result.StateChanged && !result.QuotaAutoDisabled && !result.QuotaAutoDisablePaused {
			recoveredNames = append(recoveredNames, snapshot.ProviderName)
		}
	}
	select {
	case <-s.recoveryStop:
		return nil
	default:
	}
	return recoveredNames
}

func (s *ProviderQuotaAutomationService) listManagedProviderSnapshots() ([]providerQuotaAutomationSnapshot, error) {
	result := make([]providerQuotaAutomationSnapshot, 0)
	var firstErr error
	for _, kind := range s.allProviderKinds() {
		snapshots, err := s.listProviderSnapshots(kind)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, snapshot := range snapshots {
			if snapshot.QuotaAutoDisabled || snapshot.QuotaAutoDisablePaused {
				result = append(result, snapshot)
			}
		}
	}
	return result, firstErr
}

func (s *ProviderQuotaAutomationService) signalRecoveryScheduler() {
	s.recoveryMu.Lock()
	wake := s.recoveryWake
	started := s.recoveryStarted && !s.recoveryStopped
	s.recoveryMu.Unlock()
	if !started {
		return
	}
	select {
	case wake <- struct{}{}:
	default:
	}
}

func (s *ProviderQuotaAutomationService) CheckProviderQuota(kind string, providerID string) *ProviderQuotaAutomationResult {
	kind = normalizeQuotaAutomationKind(kind)
	providerID = strings.TrimSpace(providerID)
	key := kind + "\x00" + providerID

	s.callMu.Lock()
	if current := s.calls[key]; current != nil {
		s.callMu.Unlock()
		<-current.done
		return cloneProviderQuotaAutomationResult(current.result)
	}
	call := &providerQuotaAutomationCall{done: make(chan struct{})}
	s.calls[key] = call
	s.callMu.Unlock()

	result := s.checkProviderQuota(kind, providerID)

	s.callMu.Lock()
	call.result = result
	close(call.done)
	delete(s.calls, key)
	s.callMu.Unlock()
	return cloneProviderQuotaAutomationResult(result)
}

func (s *ProviderQuotaAutomationService) triggerProviderQuotaCheck(kind string, providerID string) {
	kind = normalizeQuotaAutomationKind(kind)
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return
	}
	settings, err := s.appSettings.GetAppSettings()
	if err != nil || !settings.ProviderQuotaAutoDisableEnabled {
		return
	}
	key := kind + "\x00" + providerID
	now := time.Now()

	s.triggerMu.Lock()
	if last := s.lastErrorTriggered[key]; !last.IsZero() && now.Sub(last) < providerQuotaErrorTriggerCooldown {
		s.triggerMu.Unlock()
		return
	}
	s.lastErrorTriggered[key] = now
	s.triggerMu.Unlock()

	go s.CheckProviderQuota(kind, providerID)
}

func (s *ProviderQuotaAutomationService) TemporarilyEnableProvider(kind string, providerID string) (*ProviderQuotaAutomationResult, error) {
	snapshot, err := s.loadProviderSnapshot(kind, providerID)
	if err != nil {
		return nil, err
	}
	if !snapshot.QuotaAutoDisabled {
		return resultFromSnapshot(snapshot, false), nil
	}

	updated, changed, err := s.updateProviderState(snapshot.Kind, snapshot.ProviderID, "", func(enabled, autoDisabled, paused bool) (bool, bool, bool) {
		return true, false, true
	})
	if err != nil {
		return nil, err
	}
	if changed {
		s.emitStateChanged(updated)
		s.signalRecoveryScheduler()
	}
	return resultFromSnapshot(updated, changed), nil
}

func (s *ProviderQuotaAutomationService) ResumeProviderQuotaAutomation(kind string, providerID string) (*ProviderQuotaAutomationResult, error) {
	snapshot, err := s.loadProviderSnapshot(kind, providerID)
	if err != nil {
		return nil, err
	}
	updated, changed, err := s.updateProviderState(snapshot.Kind, snapshot.ProviderID, "", func(enabled, autoDisabled, paused bool) (bool, bool, bool) {
		return enabled, autoDisabled, false
	})
	if err != nil {
		return nil, err
	}
	if changed {
		s.emitStateChanged(updated)
		s.signalRecoveryScheduler()
	}
	return s.CheckProviderQuota(snapshot.Kind, snapshot.ProviderID), nil
}

func (s *ProviderQuotaAutomationService) restoreAllProviders() {
	for _, kind := range s.allProviderKinds() {
		snapshots, err := s.listProviderSnapshots(kind)
		if err != nil {
			continue
		}
		for _, snapshot := range snapshots {
			if !snapshot.QuotaAutoDisabled && !snapshot.QuotaAutoDisablePaused {
				continue
			}
			updated, changed, updateErr := s.updateProviderState(kind, snapshot.ProviderID, "", func(enabled, autoDisabled, paused bool) (bool, bool, bool) {
				if autoDisabled {
					enabled = true
				}
				return enabled, false, false
			})
			if updateErr == nil && changed {
				s.emitStateChanged(updated)
			}
		}
	}
	s.signalRecoveryScheduler()
}

func (s *ProviderQuotaAutomationService) checkProviderQuota(kind string, providerID string) *ProviderQuotaAutomationResult {
	snapshot, err := s.loadProviderSnapshot(kind, providerID)
	if err != nil {
		return &ProviderQuotaAutomationResult{Items: []ProviderQuotaQueryItem{}, Error: err.Error()}
	}
	queryResult := s.queryService.QueryQuota(snapshot.QueryType, snapshot.APIURL, snapshot.APIKey, snapshot.QueryConfig)
	result := automationResultFromQuery(queryResult, snapshot)
	if !queryResult.Success {
		return result
	}

	exhausted, valid := quotaItemsExhausted(queryResult.Items)
	if !valid {
		return result
	}
	settings, settingsErr := s.appSettings.GetAppSettings()
	if settingsErr != nil || !settings.ProviderQuotaAutoDisableEnabled {
		return result
	}

	updated, changed, updateErr := s.updateProviderState(snapshot.Kind, snapshot.ProviderID, snapshot.ConfigFingerprint, func(enabled, autoDisabled, paused bool) (bool, bool, bool) {
		return resolveProviderQuotaAutomationState(enabled, autoDisabled, paused, exhausted)
	})
	if updateErr != nil {
		result.Error = updateErr.Error()
		return result
	}
	result.ProviderEnabled = updated.Enabled
	result.QuotaAutoDisabled = updated.QuotaAutoDisabled
	result.QuotaAutoDisablePaused = updated.QuotaAutoDisablePaused
	result.StateChanged = changed
	if changed {
		s.emitStateChanged(updated)
		s.signalRecoveryScheduler()
	}
	return result
}

func resolveProviderQuotaAutomationState(enabled, autoDisabled, paused, exhausted bool) (bool, bool, bool) {
	if exhausted {
		if paused || (!enabled && !autoDisabled) {
			return enabled, autoDisabled, paused
		}
		return false, true, false
	}
	if autoDisabled || paused {
		return true, false, false
	}
	return enabled, autoDisabled, paused
}

func quotaItemsExhausted(items []ProviderQuotaQueryItem) (bool, bool) {
	valid := false
	for _, item := range items {
		if strings.TrimSpace(item.InvalidMessage) != "" {
			continue
		}
		// 官方余额查询也会用 Active=false 表示余额为零，不能将其当作无效数据跳过。
		valid = true
		if item.Unlimited {
			continue
		}
		if item.Total-item.Used <= 0 {
			return true, true
		}
	}
	return false, valid
}

func hasRemoteProviderQuotaConfig(queryType string, config *ProviderQuotaQueryConfig) bool {
	normalizedType := normalizeProviderQuotaQueryType(queryType)
	if normalizedType == ProviderQuotaQueryTypeNone {
		return false
	}
	normalizedConfig := normalizeProviderQuotaQueryConfig(config, normalizedType)
	return normalizedConfig == nil || normalizedConfig.Enabled
}

func normalizeProviderQuotaAutomationOnSave(
	enabled *bool,
	autoDisabled *bool,
	paused *bool,
	queryType string,
	config *ProviderQuotaQueryConfig,
) {
	if hasRemoteProviderQuotaConfig(queryType, config) {
		return
	}
	if *autoDisabled {
		*enabled = true
	}
	*autoDisabled = false
	*paused = false
}

func normalizeQuotaAutomationKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "claude-code", "claude_code":
		return "claude"
	default:
		return strings.ToLower(strings.TrimSpace(kind))
	}
}

func quotaAutomationConfigFingerprint(queryType, apiURL, apiKey string, config *ProviderQuotaQueryConfig) string {
	payload, _ := json.Marshal(struct {
		QueryType string                    `json:"queryType"`
		APIURL    string                    `json:"apiUrl"`
		APIKey    string                    `json:"apiKey"`
		Config    *ProviderQuotaQueryConfig `json:"config"`
	}{queryType, apiURL, apiKey, config})
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func (s *ProviderQuotaAutomationService) loadProviderSnapshot(kind string, providerID string) (providerQuotaAutomationSnapshot, error) {
	kind = normalizeQuotaAutomationKind(kind)
	providerID = strings.TrimSpace(providerID)
	snapshots, err := s.listProviderSnapshots(kind)
	if err != nil {
		return providerQuotaAutomationSnapshot{}, err
	}
	for _, snapshot := range snapshots {
		if snapshot.ProviderID == providerID {
			return snapshot, nil
		}
	}
	return providerQuotaAutomationSnapshot{}, fmt.Errorf("未找到供应商 (kind=%s, id=%s)", kind, providerID)
}

func (s *ProviderQuotaAutomationService) listProviderSnapshots(kind string) ([]providerQuotaAutomationSnapshot, error) {
	kind = normalizeQuotaAutomationKind(kind)
	result := make([]providerQuotaAutomationSnapshot, 0)
	switch kind {
	case "gemini":
		for _, provider := range s.geminiService.GetProviders() {
			result = append(result, newQuotaSnapshot(kind, provider.ID, provider.Name, provider.BaseURL, provider.APIKey, provider.ProviderQuotaQueryType, provider.ProviderQuotaQueryConfig, provider.Enabled, provider.QuotaAutoDisabled, provider.QuotaAutoDisablePaused))
		}
	case "opencode":
		for _, provider := range s.openCodeService.GetProviders() {
			result = append(result, newQuotaSnapshot(kind, provider.ID, provider.Name, provider.BaseURL, provider.APIKey, provider.ProviderQuotaQueryType, provider.ProviderQuotaQueryConfig, provider.Enabled, provider.QuotaAutoDisabled, provider.QuotaAutoDisablePaused))
		}
	default:
		providers, err := s.providerService.LoadProviders(kind)
		if err != nil {
			return nil, err
		}
		for _, provider := range providers {
			result = append(result, newQuotaSnapshot(kind, fmt.Sprintf("%d", provider.ID), provider.Name, provider.APIURL, provider.APIKey, provider.ProviderQuotaQueryType, provider.ProviderQuotaQueryConfig, provider.Enabled, provider.QuotaAutoDisabled, provider.QuotaAutoDisablePaused))
		}
	}
	return result, nil
}

func newQuotaSnapshot(kind, providerID, providerName, apiURL, apiKey, queryType string, config *ProviderQuotaQueryConfig, enabled, autoDisabled, paused bool) providerQuotaAutomationSnapshot {
	return providerQuotaAutomationSnapshot{
		Kind:                   kind,
		ProviderID:             providerID,
		ProviderName:           providerName,
		APIURL:                 apiURL,
		APIKey:                 apiKey,
		QueryType:              queryType,
		QueryConfig:            config,
		ConfigFingerprint:      quotaAutomationConfigFingerprint(queryType, apiURL, apiKey, config),
		Enabled:                enabled,
		QuotaAutoDisabled:      autoDisabled,
		QuotaAutoDisablePaused: paused,
	}
}

func (s *ProviderQuotaAutomationService) updateProviderState(
	kind string,
	providerID string,
	expectedFingerprint string,
	transition func(bool, bool, bool) (bool, bool, bool),
) (providerQuotaAutomationSnapshot, bool, error) {
	kind = normalizeQuotaAutomationKind(kind)
	switch kind {
	case "gemini":
		return s.updateGeminiProviderState(providerID, expectedFingerprint, transition)
	case "opencode":
		return s.updateOpenCodeProviderState(providerID, expectedFingerprint, transition)
	default:
		return s.updateFileProviderState(kind, providerID, expectedFingerprint, transition)
	}
}

func (s *ProviderQuotaAutomationService) updateFileProviderState(kind, providerID, expectedFingerprint string, transition func(bool, bool, bool) (bool, bool, bool)) (providerQuotaAutomationSnapshot, bool, error) {
	s.providerService.mu.Lock()
	providers, err := s.providerService.loadProvidersNoLock(kind)
	if err != nil {
		s.providerService.mu.Unlock()
		return providerQuotaAutomationSnapshot{}, false, err
	}
	for i := range providers {
		provider := &providers[i]
		if fmt.Sprintf("%d", provider.ID) != providerID {
			continue
		}
		current := newQuotaSnapshot(kind, providerID, provider.Name, provider.APIURL, provider.APIKey, provider.ProviderQuotaQueryType, provider.ProviderQuotaQueryConfig, provider.Enabled, provider.QuotaAutoDisabled, provider.QuotaAutoDisablePaused)
		if expectedFingerprint != "" && current.ConfigFingerprint != expectedFingerprint {
			s.providerService.mu.Unlock()
			return current, false, nil
		}
		nextEnabled, nextAutoDisabled, nextPaused := transition(provider.Enabled, provider.QuotaAutoDisabled, provider.QuotaAutoDisablePaused)
		changed := provider.Enabled != nextEnabled || provider.QuotaAutoDisabled != nextAutoDisabled || provider.QuotaAutoDisablePaused != nextPaused
		if !changed {
			s.providerService.mu.Unlock()
			return current, false, nil
		}
		provider.Enabled = nextEnabled
		provider.QuotaAutoDisabled = nextAutoDisabled
		provider.QuotaAutoDisablePaused = nextPaused
		if err := s.providerService.saveProvidersLocked(kind, providers); err != nil {
			s.providerService.mu.Unlock()
			return current, false, err
		}
		updated := newQuotaSnapshot(kind, providerID, provider.Name, provider.APIURL, provider.APIKey, provider.ProviderQuotaQueryType, provider.ProviderQuotaQueryConfig, provider.Enabled, provider.QuotaAutoDisabled, provider.QuotaAutoDisablePaused)
		s.providerService.mu.Unlock()
		return updated, true, nil
	}
	s.providerService.mu.Unlock()
	return providerQuotaAutomationSnapshot{}, false, fmt.Errorf("未找到供应商 (kind=%s, id=%s)", kind, providerID)
}

func (s *ProviderQuotaAutomationService) updateGeminiProviderState(providerID, expectedFingerprint string, transition func(bool, bool, bool) (bool, bool, bool)) (providerQuotaAutomationSnapshot, bool, error) {
	s.geminiService.mu.Lock()
	defer s.geminiService.mu.Unlock()
	for i := range s.geminiService.providers {
		provider := &s.geminiService.providers[i]
		if provider.ID != providerID {
			continue
		}
		current := newQuotaSnapshot("gemini", providerID, provider.Name, provider.BaseURL, provider.APIKey, provider.ProviderQuotaQueryType, provider.ProviderQuotaQueryConfig, provider.Enabled, provider.QuotaAutoDisabled, provider.QuotaAutoDisablePaused)
		if expectedFingerprint != "" && current.ConfigFingerprint != expectedFingerprint {
			return current, false, nil
		}
		nextEnabled, nextAutoDisabled, nextPaused := transition(provider.Enabled, provider.QuotaAutoDisabled, provider.QuotaAutoDisablePaused)
		changed := provider.Enabled != nextEnabled || provider.QuotaAutoDisabled != nextAutoDisabled || provider.QuotaAutoDisablePaused != nextPaused
		if !changed {
			return current, false, nil
		}
		previous := *provider
		provider.Enabled = nextEnabled
		provider.QuotaAutoDisabled = nextAutoDisabled
		provider.QuotaAutoDisablePaused = nextPaused
		if err := s.geminiService.saveProviders(); err != nil {
			*provider = previous
			return current, false, err
		}
		return newQuotaSnapshot("gemini", providerID, provider.Name, provider.BaseURL, provider.APIKey, provider.ProviderQuotaQueryType, provider.ProviderQuotaQueryConfig, provider.Enabled, provider.QuotaAutoDisabled, provider.QuotaAutoDisablePaused), true, nil
	}
	return providerQuotaAutomationSnapshot{}, false, fmt.Errorf("未找到 Gemini 供应商 (id=%s)", providerID)
}

func (s *ProviderQuotaAutomationService) updateOpenCodeProviderState(providerID, expectedFingerprint string, transition func(bool, bool, bool) (bool, bool, bool)) (providerQuotaAutomationSnapshot, bool, error) {
	s.openCodeService.mu.Lock()
	defer s.openCodeService.mu.Unlock()
	for i := range s.openCodeService.providers {
		provider := &s.openCodeService.providers[i]
		if provider.ID != providerID {
			continue
		}
		current := newQuotaSnapshot("opencode", providerID, provider.Name, provider.BaseURL, provider.APIKey, provider.ProviderQuotaQueryType, provider.ProviderQuotaQueryConfig, provider.Enabled, provider.QuotaAutoDisabled, provider.QuotaAutoDisablePaused)
		if expectedFingerprint != "" && current.ConfigFingerprint != expectedFingerprint {
			return current, false, nil
		}
		nextEnabled, nextAutoDisabled, nextPaused := transition(provider.Enabled, provider.QuotaAutoDisabled, provider.QuotaAutoDisablePaused)
		changed := provider.Enabled != nextEnabled || provider.QuotaAutoDisabled != nextAutoDisabled || provider.QuotaAutoDisablePaused != nextPaused
		if !changed {
			return current, false, nil
		}
		previousProviders := cloneOpenCodeProviders(s.openCodeService.providers)
		provider.Enabled = nextEnabled
		provider.QuotaAutoDisabled = nextAutoDisabled
		provider.QuotaAutoDisablePaused = nextPaused
		provider.LiveConfigManaged = boolPtr(nextEnabled)
		provider.IsInConfig = boolPtr(nextEnabled)
		if err := s.openCodeService.saveProvidersAndSyncLive(previousProviders); err != nil {
			s.openCodeService.providers = previousProviders
			return current, false, err
		}
		return newQuotaSnapshot("opencode", providerID, provider.Name, provider.BaseURL, provider.APIKey, provider.ProviderQuotaQueryType, provider.ProviderQuotaQueryConfig, provider.Enabled, provider.QuotaAutoDisabled, provider.QuotaAutoDisablePaused), true, nil
	}
	return providerQuotaAutomationSnapshot{}, false, fmt.Errorf("未找到 OpenCode 供应商 (id=%s)", providerID)
}

func (s *ProviderQuotaAutomationService) allProviderKinds() []string {
	kinds := []string{"claude", "codex", "gemini", "opencode"}
	if s.customCliService == nil {
		return kinds
	}
	tools, err := s.customCliService.ListTools()
	if err != nil {
		return kinds
	}
	for _, tool := range tools {
		if id := strings.TrimSpace(tool.ID); id != "" {
			kinds = append(kinds, "custom:"+id)
		}
	}
	return kinds
}

func (s *ProviderQuotaAutomationService) emitStateChanged(snapshot providerQuotaAutomationSnapshot) {
	if s.notification == nil {
		return
	}
	s.notification.EmitProviderQuotaStateChanged(snapshot.Kind, snapshot.ProviderID, snapshot.ProviderName, snapshot.Enabled, snapshot.QuotaAutoDisabled, snapshot.QuotaAutoDisablePaused)
}

func automationResultFromQuery(query *ProviderQuotaQueryResult, snapshot providerQuotaAutomationSnapshot) *ProviderQuotaAutomationResult {
	return &ProviderQuotaAutomationResult{
		Success:                query.Success,
		QueryType:              query.QueryType,
		Items:                  append([]ProviderQuotaQueryItem(nil), query.Items...),
		Error:                  query.Error,
		QueriedAt:              query.QueriedAt,
		ProviderEnabled:        snapshot.Enabled,
		QuotaAutoDisabled:      snapshot.QuotaAutoDisabled,
		QuotaAutoDisablePaused: snapshot.QuotaAutoDisablePaused,
	}
}

func resultFromSnapshot(snapshot providerQuotaAutomationSnapshot, changed bool) *ProviderQuotaAutomationResult {
	return &ProviderQuotaAutomationResult{
		Success:                true,
		QueryType:              snapshot.QueryType,
		Items:                  []ProviderQuotaQueryItem{},
		ProviderEnabled:        snapshot.Enabled,
		QuotaAutoDisabled:      snapshot.QuotaAutoDisabled,
		QuotaAutoDisablePaused: snapshot.QuotaAutoDisablePaused,
		StateChanged:           changed,
	}
}

func cloneProviderQuotaAutomationResult(result *ProviderQuotaAutomationResult) *ProviderQuotaAutomationResult {
	if result == nil {
		return nil
	}
	cloned := *result
	cloned.Items = append([]ProviderQuotaQueryItem(nil), result.Items...)
	return &cloned
}
