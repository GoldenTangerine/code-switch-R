package services

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/daodao97/xgo/xdb"
)

// SettingsService 管理全局配置
type SettingsService struct{}

// BlacklistSettings 黑名单配置
type BlacklistSettings struct {
	FailureThreshold int `json:"failureThreshold"` // 失败次数阈值
	DurationMinutes  int `json:"durationMinutes"`  // 拉黑时长（分钟）
}

func NewSettingsService() *SettingsService {
	return &SettingsService{}
}

// GetBlacklistSettings 获取黑名单配置
func (ss *SettingsService) GetBlacklistSettings() (threshold int, duration int, err error) {
	db, err := xdb.DB("default")
	if err != nil {
		return 0, 0, fmt.Errorf("获取数据库连接失败: %w", err)
	}

	// 获取失败阈值
	var thresholdStr string
	err = db.QueryRow(`
		SELECT value FROM app_settings WHERE key = 'blacklist_failure_threshold'
	`).Scan(&thresholdStr)

	if err != nil {
		return 0, 0, fmt.Errorf("获取失败阈值失败: %w", err)
	}

	threshold, err = strconv.Atoi(thresholdStr)
	if err != nil {
		return 0, 0, fmt.Errorf("失败阈值格式错误: %w", err)
	}

	// 获取拉黑时长
	var durationStr string
	err = db.QueryRow(`
		SELECT value FROM app_settings WHERE key = 'blacklist_duration_minutes'
	`).Scan(&durationStr)

	if err != nil {
		return 0, 0, fmt.Errorf("获取拉黑时长失败: %w", err)
	}

	duration, err = strconv.Atoi(durationStr)
	if err != nil {
		return 0, 0, fmt.Errorf("拉黑时长格式错误: %w", err)
	}

	return threshold, duration, nil
}

// UpdateBlacklistSettings 更新黑名单配置
func (ss *SettingsService) UpdateBlacklistSettings(threshold int, duration int) error {
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	// 验证参数
	if threshold < 1 || threshold > 10 {
		return fmt.Errorf("失败阈值必须在 1-10 之间")
	}

	if duration != 15 && duration != 30 && duration != 60 {
		return fmt.Errorf("拉黑时长只支持 15/30/60 分钟")
	}

	// 开启事务
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	// 更新失败阈值
	_, err = tx.Exec(`
		UPDATE app_settings SET value = ? WHERE key = 'blacklist_failure_threshold'
	`, strconv.Itoa(threshold))

	if err != nil {
		return fmt.Errorf("更新失败阈值失败: %w", err)
	}

	// 更新拉黑时长
	_, err = tx.Exec(`
		UPDATE app_settings SET value = ? WHERE key = 'blacklist_duration_minutes'
	`, strconv.Itoa(duration))

	if err != nil {
		return fmt.Errorf("更新拉黑时长失败: %w", err)
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
}

// GetBlacklistSettingsStruct 获取黑名单配置（结构体形式，用于前端）
func (ss *SettingsService) GetBlacklistSettingsStruct() (*BlacklistSettings, error) {
	threshold, duration, err := ss.GetBlacklistSettings()
	if err != nil {
		return nil, err
	}

	return &BlacklistSettings{
		FailureThreshold: threshold,
		DurationMinutes:  duration,
	}, nil
}
