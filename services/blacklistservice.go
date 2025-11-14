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
}

func NewBlacklistService(settingsService *SettingsService) *BlacklistService {
	return &BlacklistService{
		settingsService: settingsService,
	}
}

// RecordFailure 记录 provider 失败，失败次数达到阈值时自动拉黑
func (bs *BlacklistService) RecordFailure(platform string, providerName string) error {
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	// 获取配置
	threshold, duration, err := bs.settingsService.GetBlacklistSettings()
	if err != nil {
		log.Printf("⚠️  获取黑名单配置失败，使用默认值: %v", err)
		threshold, duration = 3, 30
	}

	now := time.Now()

	// 检查是否已存在记录
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

		log.Printf("📊 Provider %s/%s 失败计数: 1/%d", platform, providerName, threshold)
		return nil
	} else if err != nil {
		return fmt.Errorf("查询黑名单记录失败: %w", err)
	}

	// 如果已经拉黑且未过期，不重复计数
	if blacklistedUntil.Valid && blacklistedUntil.Time.After(now) {
		log.Printf("⛔ Provider %s/%s 已在黑名单中，过期时间: %s", platform, providerName, blacklistedUntil.Time.Format("15:04:05"))
		return nil
	}

	// 失败计数 +1
	failureCount++

	// 检查是否达到拉黑阈值
	if failureCount >= threshold {
		blacklistedAt := now
		blacklistedUntil := now.Add(time.Duration(duration) * time.Minute)

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

		log.Printf("⛔ Provider %s/%s 已拉黑 %d 分钟（失败 %d 次），过期时间: %s",
			platform, providerName, duration, failureCount, blacklistedUntil.Format("15:04:05"))

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

		log.Printf("📊 Provider %s/%s 失败计数: %d/%d", platform, providerName, failureCount, threshold)
	}

	return nil
}

// IsBlacklisted 检查 provider 是否在黑名单中
func (bs *BlacklistService) IsBlacklisted(platform string, providerName string) (bool, *time.Time) {
	db, err := xdb.DB("default")
	if err != nil {
		log.Printf("⚠️  获取数据库连接失败: %v", err)
		return false, nil
	}

	var blacklistedUntil sql.NullTime

	err = db.QueryRow(`
		SELECT blacklisted_until
		FROM provider_blacklist
		WHERE platform = ? AND provider_name = ? AND blacklisted_until > datetime('now')
	`, platform, providerName).Scan(&blacklistedUntil)

	if err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		log.Printf("⚠️  查询黑名单状态失败: %v", err)
		return false, nil
	}

	if blacklistedUntil.Valid {
		return true, &blacklistedUntil.Time
	}

	return false, nil
}

// ManualUnblock 手动解除拉黑
func (bs *BlacklistService) ManualUnblock(platform string, providerName string) error {
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	result, err := db.Exec(`
		UPDATE provider_blacklist
		SET blacklisted_at = NULL,
			blacklisted_until = NULL,
			failure_count = 0,
			auto_recovered = 0
		WHERE platform = ? AND provider_name = ?
	`, platform, providerName)

	if err != nil {
		return fmt.Errorf("手动解除拉黑失败: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("provider %s/%s 不在黑名单中", platform, providerName)
	}

	log.Printf("✅ 手动解除拉黑: %s/%s", platform, providerName)
	return nil
}

// AutoRecoverExpired 自动恢复过期的黑名单（由定时器调用）
func (bs *BlacklistService) AutoRecoverExpired() error {
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	// 查询需要恢复的 provider
	rows, err := db.Query(`
		SELECT platform, provider_name
		FROM provider_blacklist
		WHERE blacklisted_until IS NOT NULL
			AND blacklisted_until <= datetime('now')
			AND auto_recovered = 0
	`)

	if err != nil {
		return fmt.Errorf("查询过期黑名单失败: %w", err)
	}
	defer rows.Close()

	var recovered []string
	for rows.Next() {
		var platform, providerName string
		if err := rows.Scan(&platform, &providerName); err != nil {
			log.Printf("⚠️  读取恢复记录失败: %v", err)
			continue
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

// GetBlacklistStatus 获取所有黑名单状态（用于前端展示）
func (bs *BlacklistService) GetBlacklistStatus(platform string) ([]BlacklistStatus, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return nil, fmt.Errorf("获取数据库连接失败: %w", err)
	}

	rows, err := db.Query(`
		SELECT
			platform,
			provider_name,
			failure_count,
			blacklisted_at,
			blacklisted_until,
			last_failure_at
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
		var blacklistedAt, blacklistedUntil, lastFailureAt sql.NullTime

		err := rows.Scan(
			&s.Platform,
			&s.ProviderName,
			&s.FailureCount,
			&blacklistedAt,
			&blacklistedUntil,
			&lastFailureAt,
		)

		if err != nil {
			log.Printf("⚠️  读取黑名单状态失败: %v", err)
			continue
		}

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

		statuses = append(statuses, s)
	}

	return statuses, nil
}
