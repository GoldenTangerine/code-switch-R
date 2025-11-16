package services

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/daodao97/xgo/xdb"
)

// BlacklistService 管理供应商黑名单
type BlacklistService struct {
	settingsService *SettingsService
}

// BlacklistStatus 黑名单状态（用于前端展示）
type BlacklistStatus struct {
	Platform         string     `json:"platform"`
	ProviderName     string     `json:"providerName"`
	FailureCount     int        `json:"failureCount"`
	BlacklistedAt    *time.Time `json:"blacklistedAt"`
	BlacklistedUntil *time.Time `json:"blacklistedUntil"`
	LastFailureAt    *time.Time `json:"lastFailureAt"`
	IsBlacklisted    bool       `json:"isBlacklisted"`
	RemainingSeconds int        `json:"remainingSeconds"` // 剩余拉黑时间（秒）

	// v0.4.0 新增：等级拉黑相关字段
	BlacklistLevel       int        `json:"blacklistLevel"`       // 当前黑名单等级 (0-5)
	LastRecoveredAt      *time.Time `json:"lastRecoveredAt"`      // 最后恢复时间
	ForgivenessRemaining int        `json:"forgivenessRemaining"` // 距离宽恕还剩多少秒（3小时倒计时）
}

func NewBlacklistService(settingsService *SettingsService) *BlacklistService {
	return &BlacklistService{
		settingsService: settingsService,
	}
}

// RecordSuccess 记录 provider 成功，清零连续失败计数，执行降级和宽恕逻辑
func (bs *BlacklistService) RecordSuccess(platform string, providerName string) error {
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	// 获取等级拉黑配置
	levelConfig, err := bs.settingsService.GetBlacklistLevelConfig()
	if err != nil {
		log.Printf("⚠️  获取等级拉黑配置失败: %v", err)
		levelConfig = DefaultBlacklistLevelConfig()
	}

	// 查询现有记录
	var id int
	var blacklistLevel int
	var lastRecoveredAt sql.NullTime
	var lastDegradeHour int
	var blacklistedUntil sql.NullTime

	err = db.QueryRow(`
		SELECT id, blacklist_level, last_recovered_at, last_degrade_hour, blacklisted_until
		FROM provider_blacklist
		WHERE platform = ? AND provider_name = ?
	`, platform, providerName).Scan(&id, &blacklistLevel, &lastRecoveredAt, &lastDegradeHour, &blacklistedUntil)

	if err == sql.ErrNoRows {
		// 没有失败记录，无需操作
		return nil
	} else if err != nil {
		return fmt.Errorf("查询黑名单记录失败: %w", err)
	}

	now := time.Now()

	// 检查是否刚从拉黑中恢复（blacklisted_until 刚过期且 last_recovered_at 未设置）
	justRecovered := false
	if blacklistedUntil.Valid && blacklistedUntil.Time.Before(now) && !lastRecoveredAt.Valid {
		justRecovered = true
		lastRecoveredAt = sql.NullTime{Time: now, Valid: true}
		log.Printf("🔓 Provider %s/%s 从黑名单恢复（L%d），开始降级计时", platform, providerName, blacklistLevel)
	}

	// 如果功能关闭，只清零失败计数
	if !levelConfig.EnableLevelBlacklist {
		_, err = db.Exec(`
			UPDATE provider_blacklist
			SET failure_count = 0
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
			blacklist_level = ?,
			last_recovered_at = ?,
			last_degrade_hour = ?
		WHERE id = ?
	`

	var lastRecoveredTime interface{}
	if lastRecoveredAt.Valid {
		lastRecoveredTime = lastRecoveredAt.Time
	} else {
		lastRecoveredTime = nil
	}

	_, err = db.Exec(updateSQL, newLevel, lastRecoveredTime, newLastDegradeHour, id)

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
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	// 获取等级拉黑配置
	levelConfig, err := bs.settingsService.GetBlacklistLevelConfig()
	if err != nil {
		log.Printf("⚠️  获取等级拉黑配置失败: %v", err)
		levelConfig = DefaultBlacklistLevelConfig()
	}

	// 如果功能关闭，使用旧的固定拉黑模式
	if !levelConfig.EnableLevelBlacklist {
		return bs.recordFailureFixedMode(platform, providerName, levelConfig.FallbackMode, levelConfig.FallbackDurationMinutes)
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
		WHERE platform = ? AND provider_name = ?
	`, platform, providerName).Scan(&id, &failureCount, &blacklistedUntil, &blacklistLevel, &lastRecoveredAt, &lastFailureWindowStart)

	if err == sql.ErrNoRows {
		// 首次失败，插入新记录
		_, err = db.Exec(`
			INSERT INTO provider_blacklist
				(platform, provider_name, failure_count, last_failure_at, last_failure_window_start, blacklist_level)
			VALUES (?, ?, 1, ?, ?, 0)
		`, platform, providerName, now, now)

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

		_, err = db.Exec(`
			UPDATE provider_blacklist
			SET failure_count = 0,
				last_failure_at = ?,
				blacklisted_at = ?,
				blacklisted_until = ?,
				blacklist_level = ?,
				auto_recovered = 0,
				last_failure_window_start = ?
			WHERE id = ?
		`, now, blacklistedAt, blacklistedUntil, newLevel, now, id)

		if err != nil {
			return fmt.Errorf("更新拉黑状态失败: %w", err)
		}

		log.Printf("⛔ Provider %s/%s 已拉黑（L%d → L%d，%d 分钟），过期时间: %s",
			platform, providerName, blacklistLevel, newLevel, duration, blacklistedUntil.Format("15:04:05"))

	} else {
		// 未达到阈值，仅更新失败计数和窗口起始时间
		_, err = db.Exec(`
			UPDATE provider_blacklist
			SET failure_count = ?, last_failure_at = ?, last_failure_window_start = ?
			WHERE id = ?
		`, failureCount, now, now, id)

		if err != nil {
			return fmt.Errorf("更新失败计数失败: %w", err)
		}

		log.Printf("📊 Provider %s/%s 失败计数: %d/%d（当前等级: L%d）",
			platform, providerName, failureCount, levelConfig.FailureThreshold, blacklistLevel)
	}

	return nil
}

// recordFailureFixedMode 固定拉黑模式（向后兼容）
func (bs *BlacklistService) recordFailureFixedMode(platform string, providerName string, fallbackMode string, fallbackDuration int) error {
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
		WHERE platform = ? AND provider_name = ?
	`, platform, providerName).Scan(&id, &failureCount, &blacklistedUntil)

	if err == sql.ErrNoRows {
		// 首次失败，插入新记录
		_, err = db.Exec(`
			INSERT INTO provider_blacklist
				(platform, provider_name, failure_count, last_failure_at)
			VALUES (?, ?, 1, ?)
		`, platform, providerName, now)

		if err != nil {
			return fmt.Errorf("插入失败记录失败: %w", err)
		}

		log.Printf("📊 Provider %s/%s 失败计数: 1/3（固定拉黑模式）", platform, providerName)
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

	// 检查是否达到拉黑阈值（固定3次）
	if failureCount >= 3 {
		blacklistedAt := now
		blacklistedUntil := now.Add(time.Duration(fallbackDuration) * time.Minute)

		_, err = db.Exec(`
			UPDATE provider_blacklist
			SET failure_count = ?,
				last_failure_at = ?,
				blacklisted_at = ?,
				blacklisted_until = ?,
				auto_recovered = 0
			WHERE id = ?
		`, failureCount, now, blacklistedAt, blacklistedUntil, id)

		if err != nil {
			return fmt.Errorf("更新拉黑状态失败: %w", err)
		}

		log.Printf("⛔ Provider %s/%s 已拉黑 %d 分钟（固定模式，失败 %d 次），过期时间: %s",
			platform, providerName, fallbackDuration, failureCount, blacklistedUntil.Format("15:04:05"))

	} else {
		// 更新失败计数
		_, err = db.Exec(`
			UPDATE provider_blacklist
			SET failure_count = ?, last_failure_at = ?
			WHERE id = ?
		`, failureCount, now, id)

		if err != nil {
			return fmt.Errorf("更新失败计数失败: %w", err)
		}

		log.Printf("📊 Provider %s/%s 失败计数: %d/3（固定模式）", platform, providerName, failureCount)
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
	db, err := xdb.DB("default")
	if err != nil {
		log.Printf("⚠️  获取数据库连接失败: %v", err)
		return false, nil
	}

	var blacklistedUntil sql.NullTime

	// 移除 SQL 时间比较，改为 Go 代码判断（修复时区 bug）
	err = db.QueryRow(`
		SELECT blacklisted_until
		FROM provider_blacklist
		WHERE platform = ? AND provider_name = ? AND blacklisted_until IS NOT NULL
	`, platform, providerName).Scan(&blacklistedUntil)

	if err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		log.Printf("⚠️  查询黑名单状态失败: %v", err)
		return false, nil
	}

	if blacklistedUntil.Valid {
		// 使用 Go 代码比较时间（正确处理时区）
		if blacklistedUntil.Time.After(time.Now()) {
			return true, &blacklistedUntil.Time
		}
	}

	return false, nil
}

// ManualUnblockAndReset 手动解除拉黑并重置等级（完全重置）
func (bs *BlacklistService) ManualUnblockAndReset(platform string, providerName string) error {
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	now := time.Now()

	result, err := db.Exec(`
		UPDATE provider_blacklist
		SET blacklisted_at = NULL,
			blacklisted_until = NULL,
			failure_count = 0,
			blacklist_level = 0,
			last_recovered_at = ?,
			last_degrade_hour = 0,
			auto_recovered = 0
		WHERE platform = ? AND provider_name = ?
	`, now, platform, providerName)

	if err != nil {
		return fmt.Errorf("手动解除拉黑失败: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("provider %s/%s 不在黑名单中", platform, providerName)
	}

	log.Printf("✅ 手动解除拉黑并重置: %s/%s（等级清零，重新开始降级计时）", platform, providerName)
	return nil
}

// ManualUnblock 手动解除拉黑（向后兼容，调用 ManualUnblockAndReset）
func (bs *BlacklistService) ManualUnblock(platform string, providerName string) error {
	return bs.ManualUnblockAndReset(platform, providerName)
}

// ManualResetLevel 手动清零等级（不解除拉黑，仅重置等级）
func (bs *BlacklistService) ManualResetLevel(platform string, providerName string) error {
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	result, err := db.Exec(`
		UPDATE provider_blacklist
		SET blacklist_level = 0,
			last_degrade_hour = 0
		WHERE platform = ? AND provider_name = ?
	`, platform, providerName)

	if err != nil {
		return fmt.Errorf("手动清零等级失败: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("provider %s/%s 不存在", platform, providerName)
	}

	log.Printf("✅ 手动清零等级: %s/%s（等级 → L0，拉黑状态保留）", platform, providerName)
	return nil
}

// AutoRecoverExpired 自动恢复过期的黑名单（由定时器调用）
func (bs *BlacklistService) AutoRecoverExpired() error {
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	// 查询需要恢复的 provider（移除 SQL 时间比较，改为 Go 代码判断）
	rows, err := db.Query(`
		SELECT platform, provider_name, blacklisted_until
		FROM provider_blacklist
		WHERE blacklisted_until IS NOT NULL
			AND auto_recovered = 0
	`)

	if err != nil {
		return fmt.Errorf("查询过期黑名单失败: %w", err)
	}
	defer rows.Close()

	now := time.Now()
	var recovered []string

	for rows.Next() {
		var platform, providerName string
		var blacklistedUntil sql.NullTime

		if err := rows.Scan(&platform, &providerName, &blacklistedUntil); err != nil {
			log.Printf("⚠️  读取恢复记录失败: %v", err)
			continue
		}

		// 使用 Go 代码判断是否过期（正确处理时区）
		if !blacklistedUntil.Valid || blacklistedUntil.Time.After(now) {
			continue // 未过期，跳过
		}

		// 标记为已恢复（保留历史记录）
		_, err = db.Exec(`
			UPDATE provider_blacklist
			SET auto_recovered = 1, failure_count = 0
			WHERE platform = ? AND provider_name = ?
		`, platform, providerName)

		if err != nil {
			log.Printf("⚠️  标记恢复状态失败: %s/%s - %v", platform, providerName, err)
			continue
		}

		recovered = append(recovered, fmt.Sprintf("%s/%s", platform, providerName))
	}

	if len(recovered) > 0 {
		log.Printf("✅ 自动恢复 %d 个过期拉黑: %v", len(recovered), recovered)
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
	levelConfig, err := bs.settingsService.GetBlacklistLevelConfig()
	if err != nil {
		levelConfig = DefaultBlacklistLevelConfig()
	}

	rows, err := db.Query(`
		SELECT
			platform,
			provider_name,
			failure_count,
			blacklisted_at,
			blacklisted_until,
			last_failure_at,
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
			&s.ProviderName,
			&s.FailureCount,
			&blacklistedAt,
			&blacklistedUntil,
			&lastFailureAt,
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
