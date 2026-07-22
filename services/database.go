package services

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/daodao97/xgo/xdb"
	_ "modernc.org/sqlite"
)

// InitDatabase 初始化数据库连接（必须在所有服务构造之前调用）
// 【修复】解决数据库初始化时序问题：
// 1. 确保配置目录存在
// 2. 初始化 xdb 连接池
// 3. 显式设置 PRAGMA（WAL 模式 + busy_timeout）
// 4. 确保表结构存在
// 5. 预热连接池
func InitDatabase() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %w", err)
	}

	// 1. 确保配置目录存在（SQLite 不会自动创建父目录）
	configDir := filepath.Join(home, ".code-switch")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	// 2. 初始化 xdb 连接池
	// 【修复】移除 DSN 中的 PRAGMA 参数，modernc.org/sqlite 需要显式执行 PRAGMA
	dbPath := filepath.Join(configDir, "app.db?cache=shared&mode=rwc")
	if err := xdb.Inits([]xdb.Config{
		{
			Name:   "default",
			Driver: "sqlite",
			DSN:    dbPath,
		},
	}); err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}

	// 3. 显式设置 PRAGMA（解决 SQLITE_BUSY 问题）
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// 3.1 设置 busy_timeout（30秒，确保高并发下有足够等待时间）
	if _, err := db.Exec("PRAGMA busy_timeout = 30000"); err != nil {
		return fmt.Errorf("设置 busy_timeout 失败: %w", err)
	}

	// 3.2 设置 WAL 模式（允许读写并发）
	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode = WAL").Scan(&journalMode); err != nil {
		return fmt.Errorf("设置 WAL 模式失败: %w", err)
	}
	if _, err := db.Exec("PRAGMA cache_size = -1000"); err != nil {
		return fmt.Errorf("设置 cache_size 失败: %w", err)
	}
	if _, err := db.Exec("PRAGMA temp_store = FILE"); err != nil {
		return fmt.Errorf("设置 temp_store 失败: %w", err)
	}
	fmt.Printf("✅ SQLite PRAGMA 已设置: journal_mode=%s, busy_timeout=30000ms\n", journalMode)

	// 4. 确保表结构存在
	if err := ensureRequestLogTable(); err != nil {
		return fmt.Errorf("初始化 request_log 表失败: %w", err)
	}
	if err := ensureBlacklistTables(); err != nil {
		return fmt.Errorf("初始化黑名单表失败: %w", err)
	}

	// 5. 预热连接池：强制建立数据库连接，避免首次写入时失败
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM request_log").Scan(&count); err != nil {
		fmt.Printf("⚠️  连接池预热查询失败: %v\n", err)
	} else {
		fmt.Printf("✅ 数据库连接已预热（request_log 记录数: %d）\n", count)
	}

	return nil
}

// ensureBlacklistTables 确保黑名单相关表存在
func ensureBlacklistTables() error {
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	// 1. 创建 app_settings 表
	const createAppSettingsSQL = `CREATE TABLE IF NOT EXISTS app_settings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key TEXT UNIQUE NOT NULL,
		value TEXT
	)`
	if _, err := db.Exec(createAppSettingsSQL); err != nil {
		return fmt.Errorf("创建 app_settings 表失败: %w", err)
	}

	// 2. 创建 provider_blacklist 表
	const createBlacklistSQL = `CREATE TABLE IF NOT EXISTS provider_blacklist (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		platform TEXT NOT NULL,
		provider_id TEXT NOT NULL DEFAULT '',
		provider_name TEXT NOT NULL,
		failure_count INTEGER DEFAULT 0,
		blacklisted_at DATETIME,
		blacklisted_until DATETIME,
		last_failure_at DATETIME,
		blacklist_level INTEGER DEFAULT 0,
		last_recovered_at DATETIME,
		last_degrade_hour INTEGER DEFAULT 0,
		last_failure_window_start DATETIME,
		auto_recovered INTEGER DEFAULT 0,
		UNIQUE(platform, provider_id)
	)`
	if _, err := db.Exec(createBlacklistSQL); err != nil {
		return fmt.Errorf("创建 provider_blacklist 表失败: %w", err)
	}
	var providerIDColCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('provider_blacklist') WHERE name = 'provider_id'").Scan(&providerIDColCount); err != nil {
		return fmt.Errorf("检查 provider_blacklist.provider_id 字段失败: %w", err)
	}
	if providerIDColCount == 0 {
		if _, err := db.Exec("ALTER TABLE provider_blacklist ADD COLUMN provider_id TEXT NOT NULL DEFAULT ''"); err != nil {
			return fmt.Errorf("新增 provider_blacklist.provider_id 字段失败: %w", err)
		}
	}

	// 迁移旧结构（UNIQUE(platform, provider_name)）到 id 作为唯一标识
	if err := migrateBlacklistIdentityKeyWithDB(db); err != nil {
		return fmt.Errorf("迁移 provider_blacklist 标识结构失败: %w", err)
	}

	if _, err := db.Exec("UPDATE provider_blacklist SET provider_id = CASE WHEN TRIM(COALESCE(provider_name,'')) <> '' THEN TRIM(provider_name) ELSE printf('legacy-%d', id) END WHERE TRIM(COALESCE(provider_id,'')) = ''"); err != nil {
		return fmt.Errorf("回填 provider_blacklist.provider_id 失败: %w", err)
	}
	if _, err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_provider_blacklist_platform_provider_id_unique ON provider_blacklist(platform, provider_id)"); err != nil {
		return fmt.Errorf("创建 provider_blacklist provider_id 唯一索引失败: %w", err)
	}
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_provider_blacklist_platform_provider_name ON provider_blacklist(platform, provider_name)"); err != nil {
		return fmt.Errorf("创建 provider_blacklist provider_name 索引失败: %w", err)
	}
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_provider_blacklist_platform_provider_id ON provider_blacklist(platform, provider_id)"); err != nil {
		return fmt.Errorf("创建 provider_blacklist provider_id 索引失败: %w", err)
	}

	// 3. 确保 app_settings 中有默认的黑名单配置
	defaultSettings := []struct {
		key   string
		value string
	}{
		{"enable_blacklist", "true"},
		{"blacklist_failure_threshold", "5"},
		{"blacklist_duration_minutes", "30"},
	}

	for _, s := range defaultSettings {
		_, err := db.Exec(`
			INSERT OR IGNORE INTO app_settings (key, value) VALUES (?, ?)
		`, s.key, s.value)
		if err != nil {
			return fmt.Errorf("插入默认设置 %s 失败: %w", s.key, err)
		}
	}

	// 4. 迁移/补齐 blacklist_duration_seconds（优先从旧 minutes 值推导，避免覆盖用户已有配置）
	// 如果 seconds 不存在，则用 minutes * 60 初始化
	if _, err := db.Exec(`
		INSERT OR IGNORE INTO app_settings (key, value)
		SELECT 'blacklist_duration_seconds', CAST(value AS INTEGER) * 60
		FROM app_settings
		WHERE key = 'blacklist_duration_minutes'
		LIMIT 1
	`); err != nil {
		return fmt.Errorf("初始化 blacklist_duration_seconds 失败: %w", err)
	}
	// 万一 minutes 也不存在，兜底写默认 30 分钟
	if _, err := db.Exec(`
		INSERT OR IGNORE INTO app_settings (key, value) VALUES ('blacklist_duration_seconds', '1800')
	`); err != nil {
		return fmt.Errorf("初始化 blacklist_duration_seconds 失败: %w", err)
	}

	return nil
}

func migrateBlacklistIdentityKeyWithDB(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}

	var createSQL sql.NullString
	if err := db.QueryRow(`
		SELECT sql
		FROM sqlite_master
		WHERE type = 'table' AND name = 'provider_blacklist'
		LIMIT 1
	`).Scan(&createSQL); err != nil {
		return err
	}

	definition := strings.ToLower(strings.TrimSpace(createSQL.String))
	if definition == "" {
		return nil
	}
	normalizedDefinition := strings.NewReplacer(
		" ", "",
		"\n", "",
		"\r", "",
		"\t", "",
	).Replace(definition)
	if !strings.Contains(normalizedDefinition, "unique(platform,provider_name)") {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.Exec(`DROP TABLE IF EXISTS provider_blacklist_v2`); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		CREATE TABLE provider_blacklist_v2 (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			platform TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			provider_name TEXT NOT NULL,
			failure_count INTEGER DEFAULT 0,
			blacklisted_at DATETIME,
			blacklisted_until DATETIME,
			last_failure_at DATETIME,
			blacklist_level INTEGER DEFAULT 0,
			last_recovered_at DATETIME,
			last_degrade_hour INTEGER DEFAULT 0,
			last_failure_window_start DATETIME,
			auto_recovered INTEGER DEFAULT 0
		)
	`); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO provider_blacklist_v2 (
			id,
			platform,
			provider_id,
			provider_name,
			failure_count,
			blacklisted_at,
			blacklisted_until,
			last_failure_at,
			blacklist_level,
			last_recovered_at,
			last_degrade_hour,
			last_failure_window_start,
			auto_recovered
		)
		SELECT
			id,
			platform,
			CASE
				WHEN TRIM(COALESCE(provider_id, '')) <> '' THEN TRIM(provider_id)
				WHEN TRIM(COALESCE(provider_name, '')) <> '' THEN TRIM(provider_name)
				ELSE printf('legacy-%d', id)
			END AS provider_id,
			CASE
				WHEN TRIM(COALESCE(provider_name, '')) <> '' THEN provider_name
				WHEN TRIM(COALESCE(provider_id, '')) <> '' THEN provider_id
				ELSE printf('provider-%d', id)
			END AS provider_name,
			failure_count,
			blacklisted_at,
			blacklisted_until,
			last_failure_at,
			blacklist_level,
			last_recovered_at,
			last_degrade_hour,
			last_failure_window_start,
			auto_recovered
		FROM provider_blacklist
	`); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		DELETE FROM provider_blacklist_v2
		WHERE id NOT IN (
			SELECT MAX(id)
			FROM provider_blacklist_v2
			GROUP BY platform, provider_id
		)
	`); err != nil {
		return err
	}

	if _, err := tx.Exec(`DROP TABLE provider_blacklist`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE provider_blacklist_v2 RENAME TO provider_blacklist`); err != nil {
		return err
	}

	if _, err := tx.Exec(`CREATE UNIQUE INDEX idx_provider_blacklist_platform_provider_id_unique ON provider_blacklist(platform, provider_id)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE INDEX idx_provider_blacklist_platform_provider_name ON provider_blacklist(platform, provider_name)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE INDEX idx_provider_blacklist_platform_provider_id ON provider_blacklist(platform, provider_id)`); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}
