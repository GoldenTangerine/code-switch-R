package services

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"sync"

	"github.com/daodao97/xgo/xdb"
)

// SettingsService 管理全局配置
type SettingsService struct {
	blacklistInvalidatorMu sync.RWMutex
	blacklistInvalidator   func()
}

// BlacklistSettings 黑名单配置（基础配置，向后兼容）
type BlacklistSettings struct {
	FailureThreshold int `json:"failureThreshold"` // 失败次数阈值
	DurationSeconds  int `json:"durationSeconds"`  // 拉黑时长（秒）
}

// BlacklistLevelConfig 等级拉黑配置（v0.4.0 新增）
type BlacklistLevelConfig struct {
	// 功能开关
	EnableLevelBlacklist bool `json:"enableLevelBlacklist"` // 是否启用等级拉黑

	// 基础配置
	FailureThreshold    int `json:"failureThreshold"`    // 失败阈值（连续失败次数）
	DedupeWindowSeconds int `json:"dedupeWindowSeconds"` // 去重窗口（秒）
	RetryWaitSeconds    int `json:"retryWaitSeconds"`    // 同 Provider 重试等待时间（秒），必须 > DedupeWindowSeconds

	// 降级配置
	NormalDegradeIntervalHours float64 `json:"normalDegradeIntervalHours"` // 正常降级间隔（小时）
	ForgivenessHours           float64 `json:"forgivenessHours"`           // 宽恕触发时间（小时）
	JumpPenaltyWindowHours     float64 `json:"jumpPenaltyWindowHours"`     // 跳级惩罚窗口（小时）

	// 等级时长配置（分钟）
	L1DurationMinutes int `json:"l1DurationMinutes"` // L1 拉黑时长
	L2DurationMinutes int `json:"l2DurationMinutes"` // L2 拉黑时长
	L3DurationMinutes int `json:"l3DurationMinutes"` // L3 拉黑时长
	L4DurationMinutes int `json:"l4DurationMinutes"` // L4 拉黑时长
	L5DurationMinutes int `json:"l5DurationMinutes"` // L5 拉黑时长

	// 开关关闭时的行为
	FallbackMode            string `json:"fallbackMode"`            // fixed=固定拉黑, none=不拉黑
	FallbackDurationMinutes int    `json:"fallbackDurationMinutes"` // 固定拉黑时长（分钟）
}

// DefaultBlacklistLevelConfig 返回默认的等级拉黑配置
func DefaultBlacklistLevelConfig() *BlacklistLevelConfig {
	return &BlacklistLevelConfig{
		EnableLevelBlacklist:       false, // 默认关闭，向后兼容
		FailureThreshold:           5,
		DedupeWindowSeconds:        2,
		RetryWaitSeconds:           3, // 必须 > DedupeWindowSeconds，否则重试不会计入失败次数
		NormalDegradeIntervalHours: 1.0,
		ForgivenessHours:           3.0,
		JumpPenaltyWindowHours:     2.5,
		L1DurationMinutes:          5,
		L2DurationMinutes:          15,
		L3DurationMinutes:          60,
		L4DurationMinutes:          360,  // 6小时
		L5DurationMinutes:          1440, // 24小时
		FallbackMode:               "fixed",
		FallbackDurationMinutes:    30,
	}
}

func NewSettingsService() *SettingsService {
	// 确保数据库表存在
	if err := ensureBlacklistTables(); err != nil {
		// 记录错误但不阻止服务创建
		fmt.Printf("[SettingsService] 初始化数据库表失败: %v\n", err)
	}
	return &SettingsService{}
}

func (ss *SettingsService) BindBlacklistInvalidator(invalidator func()) {
	ss.blacklistInvalidatorMu.Lock()
	ss.blacklistInvalidator = invalidator
	ss.blacklistInvalidatorMu.Unlock()
}

func (ss *SettingsService) notifyBlacklistChanged() {
	ss.blacklistInvalidatorMu.RLock()
	invalidator := ss.blacklistInvalidator
	ss.blacklistInvalidatorMu.RUnlock()
	if invalidator != nil {
		invalidator()
	}
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

	// 获取拉黑时长（秒）
	var durationSecondsStr string
	err = db.QueryRow(`
		SELECT value FROM app_settings WHERE key = 'blacklist_duration_seconds'
	`).Scan(&durationSecondsStr)
	if err != nil {
		if err != sql.ErrNoRows {
			return 0, 0, fmt.Errorf("获取拉黑时长失败: %w", err)
		}
		// 向后兼容：旧版存的是分钟
		var durationMinutesStr string
		err = db.QueryRow(`
			SELECT value FROM app_settings WHERE key = 'blacklist_duration_minutes'
		`).Scan(&durationMinutesStr)
		if err != nil {
			return 0, 0, fmt.Errorf("获取拉黑时长失败: %w", err)
		}
		minutes, err := strconv.Atoi(durationMinutesStr)
		if err != nil {
			return 0, 0, fmt.Errorf("拉黑时长格式错误: %w", err)
		}
		return threshold, minutes * 60, nil
	}

	durationSeconds, err := strconv.Atoi(durationSecondsStr)
	if err != nil {
		return 0, 0, fmt.Errorf("拉黑时长格式错误: %w", err)
	}
	duration = durationSeconds

	return threshold, duration, nil
}

// IsBlacklistEnabled 检查拉黑功能是否启用
func (ss *SettingsService) IsBlacklistEnabled() bool {
	db, err := xdb.DB("default")
	if err != nil {
		log.Printf("⚠️  获取数据库连接失败: %v，默认启用拉黑", err)
		return true
	}

	var enabledStr string
	err = db.QueryRow(`
		SELECT value FROM app_settings WHERE key = 'enable_blacklist'
	`).Scan(&enabledStr)

	if err != nil {
		log.Printf("⚠️  获取拉黑开关失败: %v，默认启用", err)
		return true
	}

	return enabledStr == "true"
}

// UpdateBlacklistEnabled 更新拉黑功能开关
func (ss *SettingsService) UpdateBlacklistEnabled(enabled bool) error {
	enabledStr := "false"
	if enabled {
		enabledStr = "true"
	}

	err := GlobalDBQueue.Exec(`
		UPDATE app_settings SET value = ? WHERE key = 'enable_blacklist'
	`, enabledStr)

	if err != nil {
		return fmt.Errorf("更新拉黑开关失败: %w", err)
	}

	log.Printf("✅ 拉黑功能开关已更新: %v", enabled)
	ss.notifyBlacklistChanged()
	return nil
}

// UpdateBlacklistSettings 更新黑名单配置
// 使用 Saga 模式保证数据一致性（因队列无法使用事务）
func (ss *SettingsService) UpdateBlacklistSettings(threshold int, durationSeconds int) error {
	// 验证参数
	if threshold < 1 || threshold > 9 {
		return fmt.Errorf("失败阈值必须在 1-9 之间")
	}

	switch durationSeconds {
	case 30, 60, 180, 300, 900, 1800, 3600:
	default:
		return fmt.Errorf("拉黑时长只支持 30秒/1/3/5/15/30/60 分钟")
	}

	durationMinutes := (durationSeconds + 59) / 60
	if durationMinutes < 1 {
		durationMinutes = 1
	}

	// Saga 步骤 1：读取旧值（用于回滚）
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	var oldThresholdStr string
	err = db.QueryRow(`SELECT value FROM app_settings WHERE key = 'blacklist_failure_threshold'`).Scan(&oldThresholdStr)
	if err != nil {
		return fmt.Errorf("读取旧失败阈值失败: %w", err)
	}

	var oldDurationSecondsStr string
	durationSecondsErr := db.QueryRow(`SELECT value FROM app_settings WHERE key = 'blacklist_duration_seconds'`).Scan(&oldDurationSecondsStr)
	if durationSecondsErr != nil && durationSecondsErr != sql.ErrNoRows {
		return fmt.Errorf("读取旧拉黑时长失败: %w", durationSecondsErr)
	}
	var oldDurationMinutesStr string
	durationMinutesErr := db.QueryRow(`SELECT value FROM app_settings WHERE key = 'blacklist_duration_minutes'`).Scan(&oldDurationMinutesStr)
	if durationMinutesErr != nil && durationMinutesErr != sql.ErrNoRows {
		return fmt.Errorf("读取旧拉黑时长失败: %w", durationMinutesErr)
	}

	if durationSecondsErr == sql.ErrNoRows {
		// 旧版没有 seconds，尽量从 minutes 补
		minutes, err := strconv.Atoi(oldDurationMinutesStr)
		if err != nil {
			minutes = 30
			oldDurationMinutesStr = "30"
		}
		oldDurationSecondsStr = strconv.Itoa(minutes * 60)
	}
	if durationMinutesErr == sql.ErrNoRows {
		// 旧版没有 minutes，尽量从 seconds 补
		seconds, err := strconv.Atoi(oldDurationSecondsStr)
		if err != nil {
			seconds = 1800
			oldDurationSecondsStr = "1800"
		}
		minutes := (seconds + 59) / 60
		if minutes < 1 {
			minutes = 1
		}
		oldDurationMinutesStr = strconv.Itoa(minutes)
	}

	upsert := func(key string, value string) error {
		return GlobalDBQueue.Exec(`
			INSERT INTO app_settings (key, value) VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value
		`, key, value)
	}

	// Saga 步骤 2：写入失败阈值
	if err := upsert("blacklist_failure_threshold", strconv.Itoa(threshold)); err != nil {
		return fmt.Errorf("更新失败阈值失败: %w", err)
	}

	// Saga 步骤 3：写入拉黑时长（秒）
	if err := upsert("blacklist_duration_seconds", strconv.Itoa(durationSeconds)); err != nil {
		_ = upsert("blacklist_failure_threshold", oldThresholdStr)
		return fmt.Errorf("更新拉黑时长失败，已回滚失败阈值: %w", err)
	}

	// Saga 步骤 4：写入拉黑时长（分钟，向后兼容）
	if err := upsert("blacklist_duration_minutes", strconv.Itoa(durationMinutes)); err != nil {
		_ = upsert("blacklist_duration_seconds", oldDurationSecondsStr)
		_ = upsert("blacklist_failure_threshold", oldThresholdStr)
		return fmt.Errorf("更新拉黑时长失败，已回滚: %w", err)
	}

	ss.notifyBlacklistChanged()
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
		DurationSeconds:  duration,
	}, nil
}

// GetLevelBlacklistEnabled 获取等级拉黑开关状态
func (ss *SettingsService) GetLevelBlacklistEnabled() (bool, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return false, fmt.Errorf("获取数据库连接失败: %w", err)
	}

	var enabledStr string
	err = db.QueryRow(`
		SELECT value FROM app_settings WHERE key = 'blacklist_level_enabled'
	`).Scan(&enabledStr)

	if err != nil {
		// 如果找不到记录，返回默认值 false（向后兼容）
		return false, nil
	}

	return enabledStr == "true", nil
}

// SetLevelBlacklistEnabled 设置等级拉黑开关状态
func (ss *SettingsService) SetLevelBlacklistEnabled(enabled bool) error {
	enabledStr := "false"
	if enabled {
		enabledStr = "true"
	}

	// 使用 UPSERT 模式：如果存在则更新，不存在则插入
	err := GlobalDBQueue.Exec(`
		INSERT INTO app_settings (key, value) VALUES ('blacklist_level_enabled', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, enabledStr)

	if err != nil {
		return fmt.Errorf("设置等级拉黑开关失败: %w", err)
	}

	ss.notifyBlacklistChanged()
	return nil
}

// GetIntSetting 获取整数类型的配置值（通用方法）
// 如果找不到或解析失败，返回 0
func (ss *SettingsService) GetIntSetting(key string) int {
	db, err := xdb.DB("default")
	if err != nil {
		return 0
	}

	var valueStr string
	err = db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&valueStr)
	if err != nil {
		return 0
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0
	}

	return value
}

// SetIntSetting 设置整数类型的配置值（通用方法）
func (ss *SettingsService) SetIntSetting(key string, value int) error {
	err := GlobalDBQueue.Exec(`
		INSERT INTO app_settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, strconv.Itoa(value))

	if err != nil {
		return fmt.Errorf("设置 %s 失败: %w", key, err)
	}

	return nil
}
