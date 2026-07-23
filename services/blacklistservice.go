package services

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/daodao97/xgo/xdb"
)

// BlacklistService 管理供应商黑名单
type BlacklistService struct {
	settingsService     *SettingsService
	notificationService *NotificationService
	runtimeSnapshot     atomic.Value
	snapshotStop        chan struct{}
	snapshotDone        chan struct{}
	snapshotLifecycleMu sync.Mutex
	snapshotStarted     bool
	snapshotStopped     bool
	identityMu          sync.Mutex
	boundIdentities     map[string]string
	refreshMu           sync.Mutex
	operationMu         sync.RWMutex
}

const blacklistRuntimeSnapshotRefreshInterval = time.Minute

const (
	BlacklistTriggerSourceRequest = "request"
	BlacklistTriggerSourceHealth  = "health"
	BlacklistTriggerSourceLegacy  = "legacy"
	blacklistReasonMaxBytes       = 2 * 1024
)

type blacklistRuntimeSnapshot struct {
	enabled                 bool
	levelConfig             BlacklistLevelConfig
	blacklistedUntil        map[string]time.Time
	recoveryPendingUntil    map[string]time.Time
	successNeedsWrite       map[string]bool
	healthSuccessNeedsWrite map[string]bool
}

// BlacklistStatus 黑名单状态（用于前端展示）
type BlacklistStatus struct {
	Platform               string     `json:"platform"`
	ProviderID             string     `json:"providerId"`
	ProviderName           string     `json:"providerName"`
	FailureCount           int        `json:"failureCount"`
	FailureThreshold       int        `json:"failureThreshold"`
	HealthFailureCount     int        `json:"healthFailureCount"`
	HealthFailureThreshold int        `json:"healthFailureThreshold"`
	BlacklistedAt          *time.Time `json:"blacklistedAt"`
	BlacklistedUntil       *time.Time `json:"blacklistedUntil"`
	LastFailureAt          *time.Time `json:"lastFailureAt"`
	IsBlacklisted          bool       `json:"isBlacklisted"`
	RemainingSeconds       int        `json:"remainingSeconds"` // 剩余拉黑时间（秒）

	// v0.4.0 新增：等级拉黑相关字段
	BlacklistLevel         int        `json:"blacklistLevel"` // 当前黑名单等级 (0-5)
	BlacklistTriggerSource string     `json:"blacklistTriggerSource,omitempty"`
	BlacklistReason        string     `json:"blacklistReason,omitempty"`
	LastRecoveredAt        *time.Time `json:"lastRecoveredAt"`      // 最后恢复时间
	ForgivenessRemaining   int        `json:"forgivenessRemaining"` // 距离宽恕还剩多少秒（3小时倒计时）
}

func NewBlacklistService(settingsService *SettingsService, notificationService *NotificationService) *BlacklistService {
	service := &BlacklistService{
		settingsService:     settingsService,
		notificationService: notificationService,
		snapshotStop:        make(chan struct{}),
		snapshotDone:        make(chan struct{}),
		boundIdentities:     make(map[string]string),
	}
	service.refreshRuntimeSnapshot()
	return service
}

func blacklistRuntimeKey(platform, providerID string) string {
	return strings.TrimSpace(platform) + "\x00" + strings.TrimSpace(providerID)
}

func (bs *BlacklistService) runtime() blacklistRuntimeSnapshot {
	if value := bs.runtimeSnapshot.Load(); value != nil {
		return value.(blacklistRuntimeSnapshot)
	}
	return blacklistRuntimeSnapshot{
		enabled:                 true,
		levelConfig:             *DefaultBlacklistLevelConfig(),
		blacklistedUntil:        map[string]time.Time{},
		recoveryPendingUntil:    map[string]time.Time{},
		successNeedsWrite:       map[string]bool{},
		healthSuccessNeedsWrite: map[string]bool{},
	}
}

func (bs *BlacklistService) refreshRuntimeSnapshot() {
	bs.refreshMu.Lock()
	defer bs.refreshMu.Unlock()
	previous := bs.runtime()
	next := blacklistRuntimeSnapshot{
		enabled:                 bs.settingsService.IsBlacklistEnabled(),
		levelConfig:             previous.levelConfig,
		blacklistedUntil:        make(map[string]time.Time),
		recoveryPendingUntil:    make(map[string]time.Time),
		successNeedsWrite:       make(map[string]bool),
		healthSuccessNeedsWrite: make(map[string]bool),
	}
	if config, err := bs.settingsService.GetBlacklistLevelConfig(); err == nil && config != nil {
		next.levelConfig = *config
	} else if err != nil {
		log.Printf("[BlacklistService] 刷新配置快照失败，保留上一份配置: %v", err)
	}
	db, err := xdb.DB("default")
	if err == nil {
		rows, queryErr := db.Query(`
			SELECT platform, provider_id, failure_count, health_failure_count, blacklist_level,
				last_recovered_at, last_degrade_hour, blacklisted_until
			FROM provider_blacklist
		`)
		if queryErr == nil {
			defer rows.Close()
			stateComplete := true
			now := time.Now()
			for rows.Next() {
				var platform string
				var providerID string
				var failureCount int
				var healthFailureCount int
				var blacklistLevel int
				var lastRecoveredAt sql.NullTime
				var lastDegradeHour int
				var until sql.NullTime
				if scanErr := rows.Scan(
					&platform,
					&providerID,
					&failureCount,
					&healthFailureCount,
					&blacklistLevel,
					&lastRecoveredAt,
					&lastDegradeHour,
					&until,
				); scanErr != nil {
					log.Printf("[BlacklistService] 读取黑名单快照失败，保留上一份状态: %v", scanErr)
					stateComplete = false
					break
				}
				key := blacklistRuntimeKey(platform, providerID)
				next.successNeedsWrite[key] = blacklistSuccessNeedsWrite(
					next.levelConfig,
					failureCount,
					healthFailureCount,
					blacklistLevel,
					lastRecoveredAt,
					lastDegradeHour,
					until,
					now,
				)
				next.healthSuccessNeedsWrite[key] = healthFailureCount != 0
				if until.Valid {
					next.blacklistedUntil[key] = until.Time
					if next.levelConfig.EnableLevelBlacklist && !lastRecoveredAt.Valid {
						next.recoveryPendingUntil[key] = until.Time
					}
				}
			}
			if rowsErr := rows.Err(); rowsErr != nil {
				log.Printf("[BlacklistService] 遍历黑名单快照失败，保留上一份状态: %v", rowsErr)
				stateComplete = false
			}
			if !stateComplete {
				next.blacklistedUntil = previous.blacklistedUntil
				next.recoveryPendingUntil = previous.recoveryPendingUntil
				next.successNeedsWrite = previous.successNeedsWrite
				next.healthSuccessNeedsWrite = previous.healthSuccessNeedsWrite
			}
		} else {
			next.blacklistedUntil = previous.blacklistedUntil
			next.recoveryPendingUntil = previous.recoveryPendingUntil
			next.successNeedsWrite = previous.successNeedsWrite
			next.healthSuccessNeedsWrite = previous.healthSuccessNeedsWrite
		}
	} else {
		next.blacklistedUntil = previous.blacklistedUntil
		next.recoveryPendingUntil = previous.recoveryPendingUntil
		next.successNeedsWrite = previous.successNeedsWrite
		next.healthSuccessNeedsWrite = previous.healthSuccessNeedsWrite
	}
	bs.runtimeSnapshot.Store(next)
}

func blacklistSuccessNeedsWrite(
	config BlacklistLevelConfig,
	failureCount int,
	healthFailureCount int,
	blacklistLevel int,
	lastRecoveredAt sql.NullTime,
	lastDegradeHour int,
	blacklistedUntil sql.NullTime,
	now time.Time,
) bool {
	if failureCount != 0 || healthFailureCount != 0 {
		return true
	}
	if !config.EnableLevelBlacklist {
		return false
	}
	if blacklistedUntil.Valid && blacklistedUntil.Time.Before(now) && !lastRecoveredAt.Valid {
		return true
	}
	if !lastRecoveredAt.Valid || blacklistLevel <= 0 {
		return false
	}
	timeSinceRecovery := now.Sub(lastRecoveredAt.Time)
	if timeSinceRecovery >= time.Duration(config.ForgivenessHours*float64(time.Hour)) && blacklistLevel >= 3 {
		return true
	}
	return int(timeSinceRecovery.Hours()) > lastDegradeHour
}

func (bs *BlacklistService) RefreshRuntimeSnapshot() {
	if bs != nil {
		bs.refreshRuntimeSnapshot()
	}
}

func (bs *BlacklistService) watchRuntimeSnapshot() {
	defer close(bs.snapshotDone)
	ticker := time.NewTicker(blacklistRuntimeSnapshotRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			bs.refreshRuntimeSnapshot()
		case <-bs.snapshotStop:
			return
		}
	}
}

func (bs *BlacklistService) Start() error {
	if bs == nil {
		return nil
	}
	bs.snapshotLifecycleMu.Lock()
	defer bs.snapshotLifecycleMu.Unlock()
	if bs.snapshotStarted || bs.snapshotStopped {
		return nil
	}
	bs.snapshotStarted = true
	go bs.watchRuntimeSnapshot()
	return nil
}

func (bs *BlacklistService) Stop() error {
	if bs == nil || bs.snapshotStop == nil {
		return nil
	}
	bs.snapshotLifecycleMu.Lock()
	started := bs.snapshotStarted
	if !bs.snapshotStopped {
		bs.snapshotStopped = true
		if started {
			close(bs.snapshotStop)
		}
	}
	bs.snapshotLifecycleMu.Unlock()
	if started {
		<-bs.snapshotDone
	}
	return nil
}

func (bs *BlacklistService) levelConfig() *BlacklistLevelConfig {
	config := bs.runtime().levelConfig
	return &config
}

func normalizeBlacklistIdentity(providerID, providerName string) (string, string) {
	providerID = strings.TrimSpace(providerID)
	providerName = strings.TrimSpace(providerName)
	if providerID == "" {
		providerID = providerName
	}
	if providerName == "" {
		providerName = providerID
	}
	return providerID, providerName
}

func normalizeBlacklistReason(reason string) string {
	reason = sanitizeRequestLogPayload(strings.TrimSpace(reason))
	reason, _ = truncateRequestLogPayload(reason, blacklistReasonMaxBytes)
	return reason
}

func (bs *BlacklistService) bindProviderIdentity(platform, providerID, providerName string) {
	providerID, providerName = normalizeBlacklistIdentity(providerID, providerName)
	if providerID == "" {
		return
	}
	identityKey := blacklistRuntimeKey(platform, providerID)
	bs.identityMu.Lock()
	if bs.boundIdentities[identityKey] == providerName {
		bs.identityMu.Unlock()
		return
	}
	bs.identityMu.Unlock()
	bindSucceeded := true
	execSQL := func(statement string, args ...interface{}) error {
		if GlobalDBQueue != nil {
			return GlobalDBQueue.Exec(statement, args...)
		}
		db, err := xdb.DB("default")
		if err != nil {
			return err
		}
		_, err = db.Exec(statement, args...)
		return err
	}
	// 先尝试按 provider_id 更新名称（改名后可保持同一条记录）
	if err := execSQL(`
		UPDATE provider_blacklist
		SET provider_name = ?
		WHERE platform = ? AND provider_id = ?
	`, providerName, platform, providerID); err != nil {
		bindSucceeded = false
		log.Printf("⚠️  绑定 provider_id（按 id 更新名称）失败: %v", err)
	}
	// 再补齐当前名称对应记录的 provider_id（历史数据兼容）
	if providerName != "" {
		if err := execSQL(`
			UPDATE provider_blacklist
			SET provider_id = ?
			WHERE platform = ? AND provider_name = ? AND (provider_id IS NULL OR provider_id = '')
		`, providerID, platform, providerName); err != nil {
			bindSucceeded = false
			log.Printf("⚠️  绑定 provider_id（按名称回填 id）失败: %v", err)
		}
	}
	if bindSucceeded {
		bs.identityMu.Lock()
		bs.boundIdentities[identityKey] = providerName
		bs.identityMu.Unlock()
	}
}

func (bs *BlacklistService) isProviderIdentityBound(platform, providerID, providerName string) bool {
	providerID, providerName = normalizeBlacklistIdentity(providerID, providerName)
	bs.identityMu.Lock()
	defer bs.identityMu.Unlock()
	return bs.boundIdentities[blacklistRuntimeKey(platform, providerID)] == providerName
}

func (bs *BlacklistService) RecordSuccessByID(platform, providerID, providerName string) error {
	providerID, providerName = normalizeBlacklistIdentity(providerID, providerName)
	bs.operationMu.RLock()
	snapshot := bs.runtime()
	key := blacklistRuntimeKey(platform, providerID)
	now := time.Now()
	if until, exists := snapshot.blacklistedUntil[key]; exists && until.After(now) {
		bs.operationMu.RUnlock()
		return nil
	}
	wasBound := bs.isProviderIdentityBound(platform, providerID, providerName)
	needsWrite := snapshot.successNeedsWrite[key]
	if until, exists := snapshot.recoveryPendingUntil[key]; snapshot.levelConfig.EnableLevelBlacklist && exists && !until.After(now) {
		needsWrite = true
	}
	if wasBound && !needsWrite {
		bs.operationMu.RUnlock()
		return nil
	}
	bs.operationMu.RUnlock()

	bs.operationMu.Lock()
	defer bs.operationMu.Unlock()

	snapshot = bs.runtime()
	if until, exists := snapshot.blacklistedUntil[key]; exists && until.After(time.Now()) {
		return nil
	}
	wasBound = bs.isProviderIdentityBound(platform, providerID, providerName)
	bs.bindProviderIdentity(platform, providerID, providerName)
	snapshot = bs.runtime()
	needsWrite = snapshot.successNeedsWrite[key]
	if until, exists := snapshot.recoveryPendingUntil[key]; snapshot.levelConfig.EnableLevelBlacklist && exists && !until.After(time.Now()) {
		needsWrite = true
	}
	if wasBound && !needsWrite {
		return nil
	}
	if err := bs.recordSuccessByIdentity(platform, providerID, providerName); err != nil {
		return err
	}
	bs.refreshRuntimeSnapshot()
	bs.bindProviderIdentity(platform, providerID, providerName)
	return nil
}

func (bs *BlacklistService) RecordFailureByID(platform, providerID, providerName string) error {
	return bs.RecordFailureWithReasonByID(platform, providerID, providerName, "")
}

func (bs *BlacklistService) RecordFailureWithReasonByID(platform, providerID, providerName, reason string) error {
	bs.operationMu.Lock()
	defer bs.operationMu.Unlock()

	providerID, providerName = normalizeBlacklistIdentity(providerID, providerName)
	bs.bindProviderIdentity(platform, providerID, providerName)
	if err := bs.recordFailureByIdentityWithReason(platform, providerID, providerName, reason); err != nil {
		return err
	}
	bs.refreshRuntimeSnapshot()
	bs.bindProviderIdentity(platform, providerID, providerName)
	return nil
}

func (bs *BlacklistService) RecordHealthCheckSuccessByID(platform, providerID, providerName string) error {
	providerID, providerName = normalizeBlacklistIdentity(providerID, providerName)
	key := blacklistRuntimeKey(platform, providerID)
	bs.operationMu.RLock()
	if !bs.runtime().healthSuccessNeedsWrite[key] {
		bs.operationMu.RUnlock()
		return nil
	}
	bs.operationMu.RUnlock()

	bs.operationMu.Lock()
	defer bs.operationMu.Unlock()

	if !bs.runtime().healthSuccessNeedsWrite[key] {
		return nil
	}
	bs.bindProviderIdentity(platform, providerID, providerName)
	if GlobalDBQueue == nil {
		return fmt.Errorf("数据库写入队列未初始化")
	}
	if err := GlobalDBQueue.Exec(`
		UPDATE provider_blacklist
		SET health_failure_count = 0,
			last_health_failure_at = NULL,
			provider_name = ?
		WHERE platform = ? AND provider_id = ?
	`, providerName, platform, providerID); err != nil {
		return fmt.Errorf("清零健康检查失败计数失败: %w", err)
	}
	bs.refreshRuntimeSnapshot()
	return nil
}

func (bs *BlacklistService) RecordHealthCheckFailureByID(platform, providerID, providerName, reason string, failureThreshold int) error {
	bs.operationMu.Lock()
	defer bs.operationMu.Unlock()

	providerID, providerName = normalizeBlacklistIdentity(providerID, providerName)
	bs.bindProviderIdentity(platform, providerID, providerName)
	if failureThreshold < 2 || failureThreshold > 9 {
		failureThreshold = DefaultHealthBlacklistThreshold
	}
	if err := bs.recordHealthCheckFailureByIdentity(platform, providerID, providerName, reason, failureThreshold); err != nil {
		return err
	}
	bs.refreshRuntimeSnapshot()
	bs.bindProviderIdentity(platform, providerID, providerName)
	return nil
}

func (bs *BlacklistService) recordHealthCheckFailureByIdentity(platform, providerID, providerName, reason string, failureThreshold int) error {
	if !bs.runtime().enabled {
		return nil
	}
	levelConfig := bs.levelConfig()
	if !levelConfig.EnableLevelBlacklist && levelConfig.FallbackMode == "none" {
		return nil
	}
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	now := time.Now()
	var id int
	var healthFailureCount int
	var blacklistedUntil sql.NullTime
	var blacklistLevel int
	var lastRecoveredAt sql.NullTime
	err = db.QueryRow(`
		SELECT id, health_failure_count, blacklisted_until, blacklist_level, last_recovered_at
		FROM provider_blacklist
		WHERE platform = ? AND provider_id = ?
	`, platform, providerID).Scan(&id, &healthFailureCount, &blacklistedUntil, &blacklistLevel, &lastRecoveredAt)
	if err == sql.ErrNoRows {
		if GlobalDBQueue == nil {
			return fmt.Errorf("数据库写入队列未初始化")
		}
		if err := GlobalDBQueue.Exec(`
			INSERT INTO provider_blacklist (
				platform, provider_name, provider_id, health_failure_count,
				last_failure_at, last_health_failure_at, blacklist_level
			) VALUES (?, ?, ?, 1, ?, ?, 0)
		`, platform, providerName, providerID, now, now); err != nil {
			return fmt.Errorf("插入健康检查失败记录失败: %w", err)
		}
		log.Printf("📊 Provider %s/%s 健康检查失败计数: 1/%d", platform, providerName, failureThreshold)
		return nil
	}
	if err != nil {
		return fmt.Errorf("查询健康检查失败记录失败: %w", err)
	}
	if blacklistedUntil.Valid && blacklistedUntil.Time.After(now) {
		return nil
	}

	healthFailureCount++
	if healthFailureCount < failureThreshold {
		if err := GlobalDBQueue.Exec(`
			UPDATE provider_blacklist
			SET health_failure_count = ?,
				provider_name = ?,
				last_failure_at = ?,
				last_health_failure_at = ?
			WHERE id = ?
		`, healthFailureCount, providerName, now, now, id); err != nil {
			return fmt.Errorf("更新健康检查失败计数失败: %w", err)
		}
		log.Printf("📊 Provider %s/%s 健康检查失败计数: %d/%d", platform, providerName, healthFailureCount, failureThreshold)
		return nil
	}

	blacklistedAt := now
	triggerReason := normalizeBlacklistReason(reason)
	if levelConfig.EnableLevelBlacklist {
		levelIncrease := 1
		if lastRecoveredAt.Valid {
			jumpPenaltyWindow := time.Duration(levelConfig.JumpPenaltyWindowHours * float64(time.Hour))
			if now.Sub(lastRecoveredAt.Time) <= jumpPenaltyWindow {
				levelIncrease = 2
			}
		}
		newLevel := blacklistLevel + levelIncrease
		if newLevel > 5 {
			newLevel = 5
		}
		durationMinutes := bs.getLevelDuration(newLevel, levelConfig)
		blacklistedUntil := now.Add(time.Duration(durationMinutes) * time.Minute)
		if err := GlobalDBQueue.Exec(`
			UPDATE provider_blacklist
			SET failure_count = 0,
				health_failure_count = ?,
				provider_name = ?,
				last_failure_at = ?,
				last_health_failure_at = ?,
				blacklisted_at = ?,
				blacklisted_until = ?,
				blacklist_level = ?,
				blacklist_trigger_source = ?,
				blacklist_reason = ?,
				auto_recovered = 0
			WHERE id = ?
		`, healthFailureCount, providerName, now, now, blacklistedAt, blacklistedUntil, newLevel, BlacklistTriggerSourceHealth, triggerReason, id); err != nil {
			return fmt.Errorf("更新健康检查拉黑状态失败: %w", err)
		}
		if bs.notificationService != nil {
			bs.notificationService.NotifyProviderBlacklisted(platform, providerID, providerName, newLevel, durationMinutes)
		}
		return nil
	}

	_, durationSeconds, err := bs.settingsService.GetBlacklistSettings()
	if err != nil {
		durationSeconds = levelConfig.FallbackDurationMinutes * 60
	}
	blacklistedUntil = sql.NullTime{Time: now.Add(time.Duration(durationSeconds) * time.Second), Valid: true}
	if err := GlobalDBQueue.Exec(`
		UPDATE provider_blacklist
		SET failure_count = 0,
			health_failure_count = ?,
			provider_name = ?,
			last_failure_at = ?,
			last_health_failure_at = ?,
			blacklisted_at = ?,
			blacklisted_until = ?,
			blacklist_trigger_source = ?,
			blacklist_reason = ?,
			auto_recovered = 0
		WHERE id = ?
	`, healthFailureCount, providerName, now, now, blacklistedAt, blacklistedUntil.Time, BlacklistTriggerSourceHealth, triggerReason, id); err != nil {
		return fmt.Errorf("更新健康检查拉黑状态失败: %w", err)
	}
	if bs.notificationService != nil {
		bs.notificationService.NotifyProviderBlacklisted(platform, providerID, providerName, blacklistLevel, (durationSeconds+59)/60)
	}
	return nil
}

func (bs *BlacklistService) IsBlacklistedByID(platform, providerID, providerName string) (bool, *time.Time) {
	snapshot := bs.runtime()
	if !snapshot.enabled {
		return false, nil
	}

	providerID, _ = normalizeBlacklistIdentity(providerID, providerName)
	if providerID == "" {
		return false, nil
	}

	if blacklistedUntil, ok := snapshot.blacklistedUntil[blacklistRuntimeKey(platform, providerID)]; ok && blacklistedUntil.After(time.Now()) {
		until := blacklistedUntil
		return true, &until
	}
	return false, nil
}

func (bs *BlacklistService) ManualUnblockAndResetByID(platform, providerID, providerName string) error {
	bs.operationMu.Lock()
	defer bs.operationMu.Unlock()

	providerID, providerName = normalizeBlacklistIdentity(providerID, providerName)
	bs.bindProviderIdentity(platform, providerID, providerName)
	if err := bs.manualUnblockAndResetByIdentity(platform, providerID, providerName); err != nil {
		return err
	}
	bs.refreshRuntimeSnapshot()
	bs.bindProviderIdentity(platform, providerID, providerName)
	return nil
}

func (bs *BlacklistService) ManualResetLevelByID(platform, providerID, providerName string) error {
	bs.operationMu.Lock()
	defer bs.operationMu.Unlock()

	providerID, providerName = normalizeBlacklistIdentity(providerID, providerName)
	bs.bindProviderIdentity(platform, providerID, providerName)
	if err := bs.manualResetLevelByIdentity(platform, providerID, providerName); err != nil {
		return err
	}
	bs.refreshRuntimeSnapshot()
	bs.bindProviderIdentity(platform, providerID, providerName)
	return nil
}

// RecordSuccess 记录 provider 成功，清零连续失败计数，执行降级和宽恕逻辑
func (bs *BlacklistService) RecordSuccess(platform string, providerName string) error {
	bs.operationMu.Lock()
	defer bs.operationMu.Unlock()

	providerID, resolvedName := normalizeBlacklistIdentity(providerName, providerName)
	bs.bindProviderIdentity(platform, providerID, resolvedName)
	return bs.recordSuccessByIdentity(platform, providerID, resolvedName)
}

func (bs *BlacklistService) recordSuccessByIdentity(platform, providerID, providerName string) error {
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	// 获取等级拉黑配置
	levelConfig := bs.levelConfig()

	// 查询现有记录
	var id int
	var blacklistLevel int
	var lastRecoveredAt sql.NullTime
	var lastDegradeHour int
	var blacklistedUntil sql.NullTime

	err = db.QueryRow(`
		SELECT id, blacklist_level, last_recovered_at, last_degrade_hour, blacklisted_until
		FROM provider_blacklist
		WHERE platform = ? AND provider_id = ?
	`, platform, providerID).Scan(&id, &blacklistLevel, &lastRecoveredAt, &lastDegradeHour, &blacklistedUntil)

	if err == sql.ErrNoRows {
		// 没有失败记录，无需操作
		return nil
	} else if err != nil {
		return fmt.Errorf("查询黑名单记录失败: %w", err)
	}

	now := time.Now()
	if blacklistedUntil.Valid && blacklistedUntil.Time.After(now) {
		return nil
	}

	// 检查是否刚从拉黑中恢复（blacklisted_until 刚过期且 last_recovered_at 未设置）
	justRecovered := false
	if blacklistedUntil.Valid && blacklistedUntil.Time.Before(now) && !lastRecoveredAt.Valid {
		justRecovered = true
		lastRecoveredAt = sql.NullTime{Time: now, Valid: true}
		log.Printf("🔓 Provider %s/%s 从黑名单恢复（L%d），开始降级计时", platform, providerName, blacklistLevel)
	}

	// 如果功能关闭，只清零失败计数
	if !levelConfig.EnableLevelBlacklist {
		err = GlobalDBQueue.Exec(`
			UPDATE provider_blacklist
			SET failure_count = 0,
				health_failure_count = 0,
				last_health_failure_at = NULL,
				blacklist_trigger_source = '',
				blacklist_reason = ''
			WHERE id = ?
		`, id)

		if err != nil {
			return fmt.Errorf("清零失败计数失败: %w", err)
		}

		log.Printf("✅ Provider %s/%s 成功，连续失败计数已清零（固定模式）", platform, providerName)
		return nil
	}

	// 执行降级和宽恕逻辑（仅在等级拉黑模式开启时）
	newLevel := blacklistLevel
	newLastDegradeHour := lastDegradeHour

	if lastRecoveredAt.Valid && blacklistLevel > 0 {
		timeSinceRecovery := now.Sub(lastRecoveredAt.Time)
		hoursSinceRecovery := int(timeSinceRecovery.Hours())

		// 宽恕机制：稳定 3 小时且等级 >= 3，直接清零到 L0
		if timeSinceRecovery >= time.Duration(levelConfig.ForgivenessHours*float64(time.Hour)) && blacklistLevel >= 3 {
			newLevel = 0
			newLastDegradeHour = 0
			log.Printf("🎉 Provider %s/%s 触发宽恕机制（稳定 %.1f 小时），等级清零（L%d → L0）",
				platform, providerName, timeSinceRecovery.Hours(), blacklistLevel)
		} else if hoursSinceRecovery > lastDegradeHour {
			// 正常降级：每小时 -1 等级（防止同一小时内重复降级）
			hoursPassed := hoursSinceRecovery - lastDegradeHour
			degradeCount := hoursPassed

			newLevel = blacklistLevel - degradeCount
			if newLevel < 0 {
				newLevel = 0
			}

			newLastDegradeHour = hoursSinceRecovery

			if degradeCount > 0 {
				log.Printf("📉 Provider %s/%s 降级（L%d → L%d，经过 %d 小时）",
					platform, providerName, blacklistLevel, newLevel, degradeCount)
			}
		}
	}

	// 更新数据库
	updateSQL := `
		UPDATE provider_blacklist
		SET failure_count = 0,
			health_failure_count = 0,
			provider_name = ?,
			blacklist_level = ?,
			last_recovered_at = ?,
			last_degrade_hour = ?,
			last_health_failure_at = NULL,
			blacklist_trigger_source = '',
			blacklist_reason = ''
		WHERE id = ?
	`

	var lastRecoveredTime interface{}
	if lastRecoveredAt.Valid {
		lastRecoveredTime = lastRecoveredAt.Time
	} else {
		lastRecoveredTime = nil
	}

	err = GlobalDBQueue.Exec(updateSQL, providerName, newLevel, lastRecoveredTime, newLastDegradeHour, id)

	if err != nil {
		return fmt.Errorf("更新成功记录失败: %w", err)
	}

	if justRecovered {
		log.Printf("✅ Provider %s/%s 成功（刚恢复），失败计数已清零，当前等级: L%d", platform, providerName, newLevel)
	} else if newLevel != blacklistLevel {
		log.Printf("✅ Provider %s/%s 成功，失败计数已清零，等级: L%d → L%d", platform, providerName, blacklistLevel, newLevel)
	} else {
		log.Printf("✅ Provider %s/%s 成功，失败计数已清零，当前等级: L%d", platform, providerName, newLevel)
	}

	return nil
}

// RecordFailure 记录 provider 失败，连续失败次数达到阈值时自动拉黑（支持等级拉黑）
func (bs *BlacklistService) RecordFailure(platform string, providerName string) error {
	bs.operationMu.Lock()
	defer bs.operationMu.Unlock()

	providerID, resolvedName := normalizeBlacklistIdentity(providerName, providerName)
	bs.bindProviderIdentity(platform, providerID, resolvedName)
	return bs.recordFailureByIdentity(platform, providerID, resolvedName)
}

func (bs *BlacklistService) recordFailureByIdentity(platform, providerID, providerName string) error {
	return bs.recordFailureByIdentityWithReason(platform, providerID, providerName, "")
}

func (bs *BlacklistService) recordFailureByIdentityWithReason(platform, providerID, providerName, reason string) error {
	// 检查拉黑功能是否启用
	if !bs.runtime().enabled {
		log.Printf("🚫 拉黑功能已关闭，跳过 provider %s/%s 的失败记录", platform, providerName)
		return nil
	}

	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	// 获取等级拉黑配置
	levelConfig := bs.levelConfig()

	// 如果功能关闭，使用旧的固定拉黑模式
	if !levelConfig.EnableLevelBlacklist {
		// 从数据库读取配置（优先使用数据库配置而非默认值）
		threshold, durationSeconds, err := bs.settingsService.GetBlacklistSettings()
		if err != nil {
			log.Printf("⚠️  获取数据库拉黑配置失败: %v，使用默认值", err)
			threshold = levelConfig.FailureThreshold
			durationSeconds = levelConfig.FallbackDurationMinutes * 60
		}
		return bs.recordFailureFixedModeByIdentityWithReason(platform, providerID, providerName, levelConfig.FallbackMode, durationSeconds, threshold, reason)
	}

	now := time.Now()

	// 查询现有记录
	var id int
	var failureCount int
	var blacklistedUntil sql.NullTime
	var blacklistLevel int
	var lastRecoveredAt sql.NullTime
	var lastFailureWindowStart sql.NullTime

	err = db.QueryRow(`
			SELECT id, failure_count, blacklisted_until, blacklist_level, last_recovered_at, last_failure_window_start
			FROM provider_blacklist
			WHERE platform = ? AND provider_id = ?
		`, platform, providerID).Scan(&id, &failureCount, &blacklistedUntil, &blacklistLevel, &lastRecoveredAt, &lastFailureWindowStart)

	if err == sql.ErrNoRows {
		// 首次失败，插入新记录
		err = GlobalDBQueue.Exec(`
				INSERT INTO provider_blacklist
					(platform, provider_name, provider_id, failure_count, last_failure_at, last_failure_window_start, blacklist_level)
				VALUES (?, ?, ?, 1, ?, ?, 0)
			`, platform, providerName, providerID, now, now)

		if err != nil {
			return fmt.Errorf("插入失败记录失败: %w", err)
		}

		log.Printf("📊 Provider %s/%s 失败计数: 1/%d（等级拉黑模式）", platform, providerName, levelConfig.FailureThreshold)
		return nil
	} else if err != nil {
		return fmt.Errorf("查询黑名单记录失败: %w", err)
	}

	// 如果已经拉黑且未过期，不重复计数
	if blacklistedUntil.Valid && blacklistedUntil.Time.After(now) {
		log.Printf("⛔ Provider %s/%s 已在黑名单中（L%d），过期时间: %s",
			platform, providerName, blacklistLevel, blacklistedUntil.Time.Format("15:04:05"))
		return nil
	}

	// 30秒去重窗口检测（防止客户端重试误判）
	if lastFailureWindowStart.Valid {
		timeSinceLastFailure := now.Sub(lastFailureWindowStart.Time)
		if timeSinceLastFailure < time.Duration(levelConfig.DedupeWindowSeconds)*time.Second {
			log.Printf("🔄 Provider %s/%s 在30秒去重窗口内，忽略此次失败", platform, providerName)
			return nil
		}
	}

	// 失败计数 +1，更新去重窗口起始时间
	failureCount++

	// 检查是否达到拉黑阈值
	if failureCount >= levelConfig.FailureThreshold {
		// 计算等级升级策略
		newLevel := blacklistLevel
		var levelIncrease int

		if lastRecoveredAt.Valid {
			timeSinceRecovery := now.Sub(lastRecoveredAt.Time)
			jumpPenaltyWindow := time.Duration(levelConfig.JumpPenaltyWindowHours * float64(time.Hour))

			if timeSinceRecovery <= jumpPenaltyWindow {
				// 跳级惩罚：恢复后短时间内再次失败
				levelIncrease = 2
				log.Printf("⚡ Provider %s/%s 触发跳级惩罚（恢复后 %.1f 小时内再次失败）",
					platform, providerName, timeSinceRecovery.Hours())
			} else {
				// 正常升级
				levelIncrease = 1
				log.Printf("📈 Provider %s/%s 正常升级（恢复后 %.1f 小时再次失败）",
					platform, providerName, timeSinceRecovery.Hours())
			}
		} else {
			// 首次拉黑，默认 L1
			levelIncrease = 1
		}

		newLevel = blacklistLevel + levelIncrease
		if newLevel > 5 {
			newLevel = 5 // 最高 L5
		}

		// 根据等级获取拉黑时长
		duration := bs.getLevelDuration(newLevel, levelConfig)
		blacklistedAt := now
		blacklistedUntil := now.Add(time.Duration(duration) * time.Minute)

		err = GlobalDBQueue.Exec(`
				UPDATE provider_blacklist
				SET failure_count = ?,
					health_failure_count = 0,
					provider_name = ?,
					last_failure_at = ?,
					blacklisted_at = ?,
					blacklisted_until = ?,
					blacklist_level = ?,
					blacklist_trigger_source = ?,
					blacklist_reason = ?,
					auto_recovered = 0,
					last_failure_window_start = ?
				WHERE id = ?
			`, failureCount, providerName, now, blacklistedAt, blacklistedUntil, newLevel, BlacklistTriggerSourceRequest, normalizeBlacklistReason(reason), now, id)

		if err != nil {
			return fmt.Errorf("更新拉黑状态失败: %w", err)
		}

		log.Printf("⛔ Provider %s/%s 已拉黑（L%d → L%d，%d 分钟），过期时间: %s",
			platform, providerName, blacklistLevel, newLevel, duration, blacklistedUntil.Format("15:04:05"))

		// 发送拉黑通知
		if bs.notificationService != nil {
			bs.notificationService.NotifyProviderBlacklisted(platform, providerID, providerName, newLevel, duration)
		}

	} else {
		// 未达到阈值，仅更新失败计数和窗口起始时间
		err = GlobalDBQueue.Exec(`
				UPDATE provider_blacklist
				SET failure_count = ?, provider_name = ?, last_failure_at = ?, last_failure_window_start = ?
				WHERE id = ?
			`, failureCount, providerName, now, now, id)

		if err != nil {
			return fmt.Errorf("更新失败计数失败: %w", err)
		}

		log.Printf("📊 Provider %s/%s 失败计数: %d/%d（当前等级: L%d）",
			platform, providerName, failureCount, levelConfig.FailureThreshold, blacklistLevel)
	}

	return nil
}

// recordFailureFixedMode 固定拉黑模式（向后兼容）
func (bs *BlacklistService) recordFailureFixedMode(platform string, providerName string, fallbackMode string, fallbackDurationSeconds int, failureThreshold int) error {
	providerID, resolvedName := normalizeBlacklistIdentity(providerName, providerName)
	bs.bindProviderIdentity(platform, providerID, resolvedName)
	return bs.recordFailureFixedModeByIdentity(platform, providerID, resolvedName, fallbackMode, fallbackDurationSeconds, failureThreshold)
}

func (bs *BlacklistService) recordFailureFixedModeByIdentity(platform, providerID, providerName string, fallbackMode string, fallbackDurationSeconds int, failureThreshold int) error {
	return bs.recordFailureFixedModeByIdentityWithReason(platform, providerID, providerName, fallbackMode, fallbackDurationSeconds, failureThreshold, "")
}

func (bs *BlacklistService) recordFailureFixedModeByIdentityWithReason(platform, providerID, providerName string, fallbackMode string, fallbackDurationSeconds int, failureThreshold int, reason string) error {
	if fallbackMode == "none" {
		log.Printf("🚫 Provider %s/%s 失败，但等级拉黑已关闭且 fallbackMode=none，不拉黑", platform, providerName)
		return nil
	}

	// 使用旧的固定拉黑逻辑
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	now := time.Now()

	// 查询现有记录
	var id int
	var failureCount int
	var blacklistedUntil sql.NullTime

	err = db.QueryRow(`
		SELECT id, failure_count, blacklisted_until
		FROM provider_blacklist
		WHERE platform = ? AND provider_id = ?
	`, platform, providerID).Scan(&id, &failureCount, &blacklistedUntil)

	if err == sql.ErrNoRows {
		// 首次失败，插入新记录
		err = GlobalDBQueue.Exec(`
			INSERT INTO provider_blacklist
				(platform, provider_id, provider_name, failure_count, last_failure_at)
			VALUES (?, ?, ?, 1, ?)
		`, platform, providerID, providerName, now)

		if err != nil {
			return fmt.Errorf("插入失败记录失败: %w", err)
		}

		log.Printf("📊 Provider %s/%s 失败计数: 1/%d（固定拉黑模式）", platform, providerName, failureThreshold)
		return nil
	} else if err != nil {
		return fmt.Errorf("查询黑名单记录失败: %w", err)
	}

	// 如果已经拉黑且未过期，不重复计数
	if blacklistedUntil.Valid && blacklistedUntil.Time.After(now) {
		log.Printf("⛔ Provider %s/%s 已在黑名单中（固定模式），过期时间: %s", platform, providerName, blacklistedUntil.Time.Format("15:04:05"))
		return nil
	}

	// 失败计数 +1
	failureCount++

	// 检查是否达到拉黑阈值
	if failureCount >= failureThreshold {
		blacklistedAt := now
		blacklistedUntil := now.Add(time.Duration(fallbackDurationSeconds) * time.Second)

		err = GlobalDBQueue.Exec(`
				UPDATE provider_blacklist
				SET failure_count = ?,
					health_failure_count = 0,
					provider_name = ?,
				last_failure_at = ?,
				blacklisted_at = ?,
					blacklisted_until = ?,
					blacklist_trigger_source = ?,
					blacklist_reason = ?,
					auto_recovered = 0
				WHERE id = ?
			`, failureCount, providerName, now, blacklistedAt, blacklistedUntil, BlacklistTriggerSourceRequest, normalizeBlacklistReason(reason), id)

		if err != nil {
			return fmt.Errorf("更新拉黑状态失败: %w", err)
		}

		if fallbackDurationSeconds < 60 {
			log.Printf("⛔ Provider %s/%s 已拉黑 %d 秒（固定模式，失败 %d 次），过期时间: %s",
				platform, providerName, fallbackDurationSeconds, failureCount, blacklistedUntil.Format("15:04:05"))
		} else {
			log.Printf("⛔ Provider %s/%s 已拉黑 %d 分钟（固定模式，失败 %d 次），过期时间: %s",
				platform, providerName, (fallbackDurationSeconds+59)/60, failureCount, blacklistedUntil.Format("15:04:05"))
		}

	} else {
		// 更新失败计数
		err = GlobalDBQueue.Exec(`
			UPDATE provider_blacklist
			SET failure_count = ?, provider_name = ?, last_failure_at = ?
			WHERE id = ?
		`, failureCount, providerName, now, id)

		if err != nil {
			return fmt.Errorf("更新失败计数失败: %w", err)
		}

		log.Printf("📊 Provider %s/%s 失败计数: %d/%d（固定模式）", platform, providerName, failureCount, failureThreshold)
	}

	return nil
}

// getLevelDuration 根据等级获取拉黑时长（分钟）
func (bs *BlacklistService) getLevelDuration(level int, config *BlacklistLevelConfig) int {
	switch level {
	case 1:
		return config.L1DurationMinutes
	case 2:
		return config.L2DurationMinutes
	case 3:
		return config.L3DurationMinutes
	case 4:
		return config.L4DurationMinutes
	case 5:
		return config.L5DurationMinutes
	default:
		return config.L1DurationMinutes // 默认 L1
	}
}

// IsBlacklisted 检查 provider 是否在黑名单中
func (bs *BlacklistService) IsBlacklisted(platform string, providerName string) (bool, *time.Time) {
	providerID, resolvedName := normalizeBlacklistIdentity(providerName, providerName)
	return bs.IsBlacklistedByID(platform, providerID, resolvedName)
}

// ManualUnblockAndReset 手动解除拉黑（保留等级，如需清零请调用 ManualResetLevel）
func (bs *BlacklistService) ManualUnblockAndReset(platform string, providerName string) error {
	bs.operationMu.Lock()
	defer bs.operationMu.Unlock()

	providerID, resolvedName := normalizeBlacklistIdentity(providerName, providerName)
	bs.bindProviderIdentity(platform, providerID, resolvedName)
	return bs.manualUnblockAndResetByIdentity(platform, providerID, resolvedName)
}

func (bs *BlacklistService) manualUnblockAndResetByIdentity(platform, providerID, providerName string) error {
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	now := time.Now()

	// 先检查记录是否存在
	var exists int
	err = db.QueryRow(`
		SELECT 1 FROM provider_blacklist
		WHERE platform = ? AND provider_id = ?
	`, platform, providerID).Scan(&exists)

	if err == sql.ErrNoRows {
		return fmt.Errorf("provider %s/%s 不在黑名单中", platform, providerName)
	} else if err != nil {
		return fmt.Errorf("查询黑名单记录失败: %w", err)
	}

	// 【重要】保留 blacklist_level，让降级/宽恕机制逐渐降低等级
	err = GlobalDBQueue.Exec(`
		UPDATE provider_blacklist
		SET blacklisted_at = NULL,
			blacklisted_until = NULL,
			failure_count = 0,
			health_failure_count = 0,
			provider_name = ?,
			last_recovered_at = ?,
			last_degrade_hour = 0,
			last_health_failure_at = NULL,
			blacklist_trigger_source = '',
			blacklist_reason = '',
			auto_recovered = 0
		WHERE platform = ? AND provider_id = ?
	`, providerName, now, platform, providerID)

	if err != nil {
		return fmt.Errorf("手动解除拉黑失败: %w", err)
	}

	log.Printf("✅ 手动解除拉黑: %s/%s（等级保留，重新开始降级计时）", platform, providerName)
	return nil
}

// ManualUnblock 手动解除拉黑（向后兼容，调用 ManualUnblockAndReset）
func (bs *BlacklistService) ManualUnblock(platform string, providerName string) error {
	return bs.ManualUnblockAndReset(platform, providerName)
}

// ManualResetLevel 手动清零等级（不解除拉黑，仅重置等级）
func (bs *BlacklistService) ManualResetLevel(platform string, providerName string) error {
	bs.operationMu.Lock()
	defer bs.operationMu.Unlock()

	providerID, resolvedName := normalizeBlacklistIdentity(providerName, providerName)
	bs.bindProviderIdentity(platform, providerID, resolvedName)
	return bs.manualResetLevelByIdentity(platform, providerID, resolvedName)
}

func (bs *BlacklistService) manualResetLevelByIdentity(platform, providerID, providerName string) error {
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	// 先检查记录是否存在
	var exists int
	err = db.QueryRow(`
		SELECT 1 FROM provider_blacklist
		WHERE platform = ? AND provider_id = ?
	`, platform, providerID).Scan(&exists)

	if err == sql.ErrNoRows {
		return fmt.Errorf("provider %s/%s 不存在", platform, providerName)
	} else if err != nil {
		return fmt.Errorf("查询黑名单记录失败: %w", err)
	}

	err = GlobalDBQueue.Exec(`
		UPDATE provider_blacklist
		SET blacklist_level = 0,
			provider_name = ?,
			last_degrade_hour = 0
		WHERE platform = ? AND provider_id = ?
	`, providerName, platform, providerID)

	if err != nil {
		return fmt.Errorf("手动清零等级失败: %w", err)
	}

	log.Printf("✅ 手动清零等级: %s/%s（等级 → L0，拉黑状态保留）", platform, providerName)
	return nil
}

// AutoRecoverExpired 自动恢复过期的黑名单（由定时器调用）
// 使用事务批量处理，避免多次单独写入导致的并发锁冲突
func (bs *BlacklistService) AutoRecoverExpired() error {
	bs.operationMu.Lock()
	defer bs.operationMu.Unlock()

	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	// 查询需要恢复的 provider（移除 SQL 时间比较，改为 Go 代码判断）
	rows, err := db.Query(`
		SELECT platform, provider_id, provider_name, blacklisted_until
		FROM provider_blacklist
		WHERE blacklisted_until IS NOT NULL
			AND auto_recovered = 0
	`)

	if err != nil {
		return fmt.Errorf("查询过期黑名单失败: %w", err)
	}
	defer rows.Close()

	now := time.Now()
	type RecoverItem struct {
		Platform     string
		ProviderID   string
		ProviderName string
	}
	var toRecover []RecoverItem

	// 收集所有需要恢复的 provider
	for rows.Next() {
		var platform, providerID, providerName string
		var blacklistedUntil sql.NullTime

		if err := rows.Scan(&platform, &providerID, &providerName, &blacklistedUntil); err != nil {
			log.Printf("⚠️  读取恢复记录失败: %v", err)
			continue
		}
		providerID, providerName = normalizeBlacklistIdentity(providerID, providerName)
		if providerID == "" {
			continue
		}

		// 使用 Go 代码判断是否过期（正确处理时区）
		if !blacklistedUntil.Valid || blacklistedUntil.Time.After(now) {
			continue // 未过期，跳过
		}

		toRecover = append(toRecover, RecoverItem{
			Platform:     platform,
			ProviderID:   providerID,
			ProviderName: providerName,
		})
	}

	// 如果没有需要恢复的，直接返回
	if len(toRecover) == 0 {
		return nil
	}

	var recovered []string
	var failed []string

	// 批量更新所有过期的 provider（使用队列）
	// 【重要】保留 blacklist_level，让 RecordSuccess 中的降级/宽恕机制逐渐降低等级
	for _, item := range toRecover {
		err := GlobalDBQueue.Exec(`
			UPDATE provider_blacklist
			SET auto_recovered = 1,
				failure_count = 0,
				health_failure_count = 0,
				provider_name = ?,
				last_recovered_at = ?,
				last_degrade_hour = 0,
				last_health_failure_at = NULL,
				blacklist_trigger_source = '',
				blacklist_reason = ''
			WHERE platform = ? AND provider_id = ?
		`, item.ProviderName, now, item.Platform, item.ProviderID)

		if err != nil {
			failed = append(failed, fmt.Sprintf("%s/%s", item.Platform, item.ProviderID))
			log.Printf("⚠️  标记恢复状态失败: %s/%s - %v", item.Platform, item.ProviderID, err)
		} else {
			recovered = append(recovered, fmt.Sprintf("%s/%s", item.Platform, item.ProviderID))
		}
	}

	if len(recovered) > 0 {
		log.Printf("✅ 自动恢复 %d 个过期拉黑（等级保留，等待降级）: %v", len(recovered), recovered)
		bs.refreshRuntimeSnapshot()
	}

	if len(failed) > 0 {
		log.Printf("⚠️  恢复失败 %d 个: %v", len(failed), failed)
	}

	return nil
}

// GetBlacklistStatus 获取所有黑名单状态（用于前端展示，支持等级拉黑）
func (bs *BlacklistService) GetBlacklistStatus(platform string) ([]BlacklistStatus, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return nil, fmt.Errorf("获取数据库连接失败: %w", err)
	}

	// 获取等级拉黑配置（用于计算宽恕倒计时）
	levelConfig := bs.levelConfig()
	healthFailureThreshold := bs.settingsService.GetHealthBlacklistThreshold()

	rows, err := db.Query(`
		SELECT
			platform,
			provider_id,
			provider_name,
			failure_count,
			health_failure_count,
			blacklisted_at,
			blacklisted_until,
			last_failure_at,
			blacklist_trigger_source,
			blacklist_reason,
			blacklist_level,
			last_recovered_at
		FROM provider_blacklist
		WHERE platform = ?
		ORDER BY last_failure_at DESC
	`, platform)

	if err != nil {
		return nil, fmt.Errorf("查询黑名单状态失败: %w", err)
	}
	defer rows.Close()

	var statuses []BlacklistStatus
	now := time.Now()

	for rows.Next() {
		var s BlacklistStatus
		var blacklistedAt, blacklistedUntil, lastFailureAt, lastRecoveredAt sql.NullTime

		err := rows.Scan(
			&s.Platform,
			&s.ProviderID,
			&s.ProviderName,
			&s.FailureCount,
			&s.HealthFailureCount,
			&blacklistedAt,
			&blacklistedUntil,
			&lastFailureAt,
			&s.BlacklistTriggerSource,
			&s.BlacklistReason,
			&s.BlacklistLevel,
			&lastRecoveredAt,
		)

		if err != nil {
			log.Printf("⚠️  读取黑名单状态失败: %v", err)
			continue
		}

		// 基础时间字段
		if blacklistedAt.Valid {
			s.BlacklistedAt = &blacklistedAt.Time
		}
		if blacklistedUntil.Valid {
			s.BlacklistedUntil = &blacklistedUntil.Time
			s.IsBlacklisted = blacklistedUntil.Time.After(now)
			if s.IsBlacklisted {
				s.RemainingSeconds = int(blacklistedUntil.Time.Sub(now).Seconds())
			}
		}
		if lastFailureAt.Valid {
			s.LastFailureAt = &lastFailureAt.Time
		}
		if lastRecoveredAt.Valid {
			s.LastRecoveredAt = &lastRecoveredAt.Time
		}
		s.FailureThreshold = levelConfig.FailureThreshold
		s.HealthFailureThreshold = healthFailureThreshold

		// 计算宽恕倒计时（如果正在降级计时中）
		if levelConfig.EnableLevelBlacklist && lastRecoveredAt.Valid && s.BlacklistLevel >= 3 {
			timeSinceRecovery := now.Sub(lastRecoveredAt.Time)
			forgivenessThreshold := time.Duration(levelConfig.ForgivenessHours * float64(time.Hour))

			if timeSinceRecovery < forgivenessThreshold {
				s.ForgivenessRemaining = int((forgivenessThreshold - timeSinceRecovery).Seconds())
			} else {
				s.ForgivenessRemaining = 0 // 已触发宽恕
			}
		}

		statuses = append(statuses, s)
	}

	return statuses, nil
}

// ShouldUseFixedMode 返回是否应该使用固定拉黑模式（禁用自动降级）
// 满足以下所有条件时返回 true：
// 1. 黑名单总开关已启用
// 2. 且满足以下任一：
//   - 等级拉黑开启
//   - 等级拉黑关闭但 fallbackMode="fixed"
func (bs *BlacklistService) ShouldUseFixedMode() bool {
	// 首先检查全局开关
	snapshot := bs.runtime()
	if !snapshot.enabled {
		return false // 全局拉黑关闭 → 始终降级
	}

	config := &snapshot.levelConfig

	// 等级拉黑开启 → 固定模式
	if config.EnableLevelBlacklist {
		return true
	}

	// 等级拉黑关闭 → 根据 fallbackMode 决定
	switch config.FallbackMode {
	case "fixed":
		return true
	case "none":
		return false
	default:
		// 未知值：记录警告并视为 none（保持降级）
		log.Printf("[BlacklistService] 未知的 fallbackMode: %s，视为 none", config.FallbackMode)
		return false
	}
}

// IsBlacklistEnabled 返回拉黑总开关状态（用于固定拉黑模式判断）
func (bs *BlacklistService) IsBlacklistEnabled() bool {
	return bs.runtime().enabled
}

// IsLevelBlacklistEnabled 返回等级拉黑功能是否开启
// 用于 proxyHandler 判断是否启用自动降级
func (bs *BlacklistService) IsLevelBlacklistEnabled() bool {
	return bs.runtime().levelConfig.EnableLevelBlacklist
}

// RetryConfig 重试配置（供 proxyHandler 使用）
type RetryConfig struct {
	FailureThreshold    int // 失败阈值（达到后触发拉黑）
	RetryWaitSeconds    int // 重试等待时间（秒）
	DedupeWindowSeconds int // 去重窗口（秒）
}

// GetRetryConfig 获取重试相关配置
// 用于 proxyHandler 实现同 Provider 重试机制
func (bs *BlacklistService) GetRetryConfig() *RetryConfig {
	config := bs.levelConfig()
	return &RetryConfig{
		FailureThreshold:    config.FailureThreshold,
		RetryWaitSeconds:    config.RetryWaitSeconds,
		DedupeWindowSeconds: config.DedupeWindowSeconds,
	}
}
