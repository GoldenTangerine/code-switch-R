/**
 * @name: SQLite 运行时写竞争测量
 * @Descripttion: 在临时数据库中测量双写队列与直连运行时事务并发时的等待、锁错误和数据一致性
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-31 18:04:42
 * @LastEditTime: 2026-08-31 18:04:42
 * @FilePath: services/sqlite_runtime_write_contention_test.go
 */

package services

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	sqliteContentionLogWrites          = 500
	sqliteContentionSingleQueueWrites  = 80
	sqliteContentionRenameTransactions = 12
	sqliteContentionRenameRows         = 250
	sqliteContentionCleanupOperations  = 4
	sqliteContentionCleanupRows        = 2000
)

type sqliteContentionSample struct {
	kind     string
	key      string
	duration time.Duration
	rows     int64
	err      error
}

type sqliteContentionMetrics struct {
	operations int
	successes  int
	busy       int
	locked     int
	p50        time.Duration
	p95        time.Duration
	successP50 time.Duration
	successP95 time.Duration
	firstError string
}

type sqliteContentionScenarioResult struct {
	elapsed      time.Duration
	operations   int
	successes    int
	busy         int
	locked       int
	waitCount    int64
	waitDuration time.Duration
	byKind       map[string]sqliteContentionMetrics
}

func TestDefaultSQLiteDSNAppliesBusyTimeoutToEveryConnection(t *testing.T) {
	db, err := sql.Open("sqlite", defaultSQLiteDSN(filepath.Join(t.TempDir(), "connection-pool.db")))
	if err != nil {
		t.Fatalf("打开临时 SQLite 失败: %v", err)
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("关闭临时 SQLite 失败: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), dbWriteQueueTestTimeout)
	defer cancel()
	connections := make([]*sql.Conn, 0, 5)
	t.Cleanup(func() {
		for _, connection := range connections {
			if err := connection.Close(); err != nil {
				t.Errorf("归还 SQLite 连接失败: %v", err)
			}
		}
	})
	for i := 0; i < 5; i++ {
		connection, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("获取第 %d 个 SQLite 连接失败: %v", i+1, err)
		}
		connections = append(connections, connection)
	}

	for i, connection := range connections {
		var timeout int
		if err := connection.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&timeout); err != nil {
			t.Fatalf("读取第 %d 个连接的 busy_timeout 失败: %v", i+1, err)
		}
		if timeout != 30000 {
			t.Errorf("第 %d 个连接 busy_timeout=%d，期望=30000", i+1, timeout)
		}
	}
}

func TestSQLiteRuntimeWriteContentionBaseline(t *testing.T) {
	var legacyQueuesOnly sqliteContentionScenarioResult
	var legacyWithDirect sqliteContentionScenarioResult
	var defaultConfig sqliteContentionScenarioResult

	t.Run("legacy_queues_only", func(t *testing.T) {
		legacyQueuesOnly = runSQLiteContentionScenario(t, "legacy_queues_only", false, false)
	})
	t.Run("legacy_queues_with_direct_runtime_writes", func(t *testing.T) {
		legacyWithDirect = runSQLiteContentionScenario(t, "legacy_queues_with_direct_runtime_writes", true, false)
	})
	t.Run("default_config_queues_with_direct_runtime_writes", func(t *testing.T) {
		defaultConfig = runSQLiteContentionScenario(t, "default_config_queues_with_direct_runtime_writes", true, true)
	})

	controlLogs := legacyQueuesOnly.byKind["log_batch"]
	contendedLogs := legacyWithDirect.byKind["log_batch"]
	t.Logf(
		"comparison log_batch_successes=%d/%d->%d/%d busy=%d->%d locked=%d->%d elapsed_ms=%.3f->%.3f",
		controlLogs.successes,
		controlLogs.operations,
		contendedLogs.successes,
		contendedLogs.operations,
		legacyQueuesOnly.busy,
		legacyWithDirect.busy,
		legacyQueuesOnly.locked,
		legacyWithDirect.locked,
		sqliteContentionMilliseconds(legacyQueuesOnly.elapsed),
		sqliteContentionMilliseconds(legacyWithDirect.elapsed),
	)
	t.Logf(
		"default_config successes=%d/%d->%d/%d busy=%d->%d locked=%d->%d elapsed_ms=%.3f->%.3f",
		legacyWithDirect.successes,
		legacyWithDirect.operations,
		defaultConfig.successes,
		defaultConfig.operations,
		legacyWithDirect.busy,
		defaultConfig.busy,
		legacyWithDirect.locked,
		defaultConfig.locked,
		sqliteContentionMilliseconds(legacyWithDirect.elapsed),
		sqliteContentionMilliseconds(defaultConfig.elapsed),
	)
}

func runSQLiteContentionScenario(t *testing.T, scenario string, includeDirect bool, useDefaultDSN bool) sqliteContentionScenarioResult {
	t.Helper()

	db := newSQLiteContentionFixture(t, useDefaultDSN)
	seedSQLiteContentionFixture(t, db)

	singleQueue := NewDBWriteQueue(db, 1024, false)
	logsQueue := NewDBWriteQueue(db, 1024, true)
	t.Cleanup(func() {
		if !singleQueue.closed.Load() {
			if err := singleQueue.Shutdown(dbWriteQueueTestTimeout); err != nil {
				t.Errorf("关闭单次写队列失败: %v", err)
			}
		}
		if !logsQueue.closed.Load() {
			if err := logsQueue.Shutdown(dbWriteQueueTestTimeout); err != nil {
				t.Errorf("关闭日志批写队列失败: %v", err)
			}
		}
	})

	operationCount := sqliteContentionLogWrites + sqliteContentionSingleQueueWrites
	if includeDirect {
		operationCount += sqliteContentionRenameTransactions + sqliteContentionCleanupOperations
	}

	start := make(chan struct{})
	samples := make(chan sqliteContentionSample, operationCount)
	var workers sync.WaitGroup
	launch := func(run func() sqliteContentionSample) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			samples <- run()
		}()
	}

	createdAt := time.Now().UTC().Format(timeLayout)
	for i := 0; i < sqliteContentionLogWrites; i++ {
		key := fmt.Sprintf("log-%03d", i)
		launch(func() sqliteContentionSample {
			started := time.Now()
			err := logsQueue.ExecBatch(`
				INSERT INTO request_log (
					platform, provider_id, provider, data_source, total_cost, created_at
				) VALUES (?, ?, ?, 'proxy', 0, ?)
			`, "codex", "traffic", key, createdAt)
			return sqliteContentionSample{kind: "log_batch", key: key, duration: time.Since(started), err: err}
		})
	}

	for i := 0; i < sqliteContentionSingleQueueWrites; i++ {
		key := fmt.Sprintf("state-%03d", i)
		launch(func() sqliteContentionSample {
			started := time.Now()
			err := singleQueue.Exec(`
				INSERT INTO runtime_state (key, value) VALUES (?, 1)
				ON CONFLICT(key) DO UPDATE SET value = value + 1
			`, key)
			return sqliteContentionSample{kind: "single_queue", key: key, duration: time.Since(started), err: err}
		})
	}

	if includeDirect {
		for i := 0; i < sqliteContentionRenameTransactions; i++ {
			providerID := fmt.Sprintf("%d", i+1)
			oldName := fmt.Sprintf("old-%02d", i)
			newName := fmt.Sprintf("new-%02d", i)
			launch(func() sqliteContentionSample {
				started := time.Now()
				err := executeSQLiteContentionProviderRename(db, providerID, oldName, newName)
				return sqliteContentionSample{kind: "provider_rename", key: providerID, duration: time.Since(started), err: err}
			})
		}

		for i := 0; i < sqliteContentionCleanupOperations; i++ {
			key := fmt.Sprintf("cleanup-%02d", i)
			launch(func() sqliteContentionSample {
				started := time.Now()
				result, err := db.Exec(`DELETE FROM health_check_history WHERE checked_at < ?`, "2026-01-01 00:00:00")
				var rows int64
				if err == nil {
					rows, err = result.RowsAffected()
				}
				return sqliteContentionSample{kind: "health_cleanup", key: key, duration: time.Since(started), rows: rows, err: err}
			})
		}
	}

	started := time.Now()
	close(start)
	go func() {
		workers.Wait()
		close(samples)
	}()

	collected := make([]sqliteContentionSample, 0, operationCount)
	for sample := range samples {
		collected = append(collected, sample)
	}
	elapsed := time.Since(started)
	if len(collected) != operationCount {
		t.Fatalf("完成操作数=%d，期望=%d", len(collected), operationCount)
	}

	assertSQLiteContentionErrors(t, collected)
	assertSQLiteContentionRows(t, db, collected, includeDirect)

	result := summarizeSQLiteContention(collected, elapsed, db.Stats())
	logSQLiteContentionResult(t, scenario, result)
	return result
}

func newSQLiteContentionFixture(t *testing.T, useDefaultDSN bool) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "runtime-contention.db")
	dsn := dbPath + "?cache=shared&mode=rwc"
	if useDefaultDSN {
		dsn = defaultSQLiteDSN(dbPath)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("打开临时 SQLite 失败: %v", err)
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("关闭临时 SQLite 失败: %v", err)
		}
	})

	if _, err := db.Exec("PRAGMA busy_timeout = 30000"); err != nil {
		t.Fatalf("设置 busy_timeout 失败: %v", err)
	}
	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode = WAL").Scan(&journalMode); err != nil {
		t.Fatalf("设置 WAL 模式失败: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode=%q，期望 wal", journalMode)
	}

	statements := []string{
		`CREATE TABLE request_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			platform TEXT NOT NULL,
			provider_id TEXT NOT NULL DEFAULT '',
			provider TEXT NOT NULL DEFAULT '',
			data_source TEXT NOT NULL DEFAULT 'proxy',
			total_cost REAL NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX idx_contention_request_log_provider_id ON request_log(platform, provider_id, created_at)`,
		`CREATE INDEX idx_contention_request_log_provider ON request_log(platform, provider, created_at)`,
		`CREATE TABLE provider_blacklist (
			platform TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			provider_name TEXT NOT NULL,
			PRIMARY KEY (platform, provider_id)
		)`,
		`CREATE TABLE health_check_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_id INTEGER NOT NULL,
			provider_name TEXT NOT NULL,
			platform TEXT NOT NULL,
			checked_at TEXT NOT NULL
		)`,
		`CREATE INDEX idx_contention_health_provider_id ON health_check_history(platform, provider_id)`,
		`CREATE INDEX idx_contention_health_checked_at ON health_check_history(checked_at)`,
		`CREATE TABLE request_log_stats_hourly (
			bucket_start TEXT NOT NULL,
			platform TEXT NOT NULL,
			provider_id TEXT NOT NULL DEFAULT '',
			provider TEXT NOT NULL DEFAULT '',
			total_requests INTEGER NOT NULL DEFAULT 0,
			successful_requests INTEGER NOT NULL DEFAULT 0,
			failed_requests INTEGER NOT NULL DEFAULT 0,
			excluded_requests INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			reasoning_tokens INTEGER NOT NULL DEFAULT 0,
			cache_create_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			total_cost REAL NOT NULL DEFAULT 0,
			PRIMARY KEY (bucket_start, platform, provider_id)
		) WITHOUT ROWID`,
		`CREATE TABLE request_log_stats_daily (
			bucket_start TEXT NOT NULL,
			platform TEXT NOT NULL,
			provider_id TEXT NOT NULL DEFAULT '',
			provider TEXT NOT NULL DEFAULT '',
			total_requests INTEGER NOT NULL DEFAULT 0,
			successful_requests INTEGER NOT NULL DEFAULT 0,
			failed_requests INTEGER NOT NULL DEFAULT 0,
			excluded_requests INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			reasoning_tokens INTEGER NOT NULL DEFAULT 0,
			cache_create_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			total_cost REAL NOT NULL DEFAULT 0,
			PRIMARY KEY (bucket_start, platform, provider_id)
		) WITHOUT ROWID`,
		`CREATE TABLE request_log_provider_quota_cycle_state (
			platform TEXT NOT NULL,
			provider_ref TEXT NOT NULL,
			five_hour_window_start TEXT NOT NULL DEFAULT '',
			five_hour_next_reset TEXT NOT NULL DEFAULT '',
			five_hour_used REAL NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (platform, provider_ref)
		) WITHOUT ROWID`,
		`CREATE TABLE runtime_state (
			key TEXT PRIMARY KEY,
			value INTEGER NOT NULL DEFAULT 0
		) WITHOUT ROWID`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("初始化临时 SQLite 失败: %v", err)
		}
	}
	return db
}

func seedSQLiteContentionFixture(t *testing.T, db *sql.DB) {
	t.Helper()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("启动测试数据事务失败: %v", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	now := time.Now().UTC()
	createdAt := now.Format(timeLayout)
	hourBucket := startOfHour(now).Format(timeLayout)
	dayBucket := startOfDay(now).Format(timeLayout)
	for i := 0; i < sqliteContentionRenameTransactions; i++ {
		providerID := fmt.Sprintf("%d", i+1)
		oldName := fmt.Sprintf("old-%02d", i)
		if _, err := tx.Exec(
			`INSERT INTO provider_blacklist (platform, provider_id, provider_name) VALUES ('codex', ?, ?)`,
			providerID,
			oldName,
		); err != nil {
			t.Fatalf("写入黑名单测试数据失败: %v", err)
		}
		if _, err := tx.Exec(
			`INSERT INTO health_check_history (provider_id, provider_name, platform, checked_at) VALUES (?, ?, 'codex', ?)`,
			i+1,
			oldName,
			createdAt,
		); err != nil {
			t.Fatalf("写入健康历史测试数据失败: %v", err)
		}
		for j := 0; j < sqliteContentionRenameRows; j++ {
			if _, err := tx.Exec(`
				INSERT INTO request_log (platform, provider_id, provider, data_source, total_cost, created_at)
				VALUES ('codex', ?, ?, 'proxy', 0.01, ?)
			`, providerID, oldName, createdAt); err != nil {
				t.Fatalf("写入请求日志测试数据失败: %v", err)
			}
		}
		for _, table := range []string{requestLogStatsHourlyTable, requestLogStatsDailyTable} {
			bucket := dayBucket
			if table == requestLogStatsHourlyTable {
				bucket = hourBucket
			}
			if _, err := tx.Exec(fmt.Sprintf(`
				INSERT INTO %s (
					bucket_start, platform, provider_id, provider, total_requests, successful_requests, total_cost
				) VALUES (?, 'codex', ?, ?, ?, ?, ?)
			`, table), bucket, providerID, oldName, sqliteContentionRenameRows, sqliteContentionRenameRows, 2.5); err != nil {
				t.Fatalf("写入聚合统计测试数据失败: %v", err)
			}
		}
	}

	for i := 0; i < sqliteContentionCleanupRows; i++ {
		if _, err := tx.Exec(`
			INSERT INTO health_check_history (provider_id, provider_name, platform, checked_at)
			VALUES (0, 'cleanup', 'cleanup', '2025-01-01 00:00:00')
		`); err != nil {
			t.Fatalf("写入待清理健康历史失败: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("提交测试数据失败: %v", err)
	}
	committed = true
}

func executeSQLiteContentionProviderRename(db *sql.DB, providerID, oldName, newName string) error {
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

	if err := syncProviderIdentityRenameTx(tx, "codex", providerID, oldName, newName); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func assertSQLiteContentionErrors(t *testing.T, samples []sqliteContentionSample) {
	t.Helper()
	for _, sample := range samples {
		if sample.err != nil && !isSQLiteContentionLockError(sample.err) {
			t.Errorf("%s %s 出现非锁竞争错误: %v", sample.kind, sample.key, sample.err)
		}
	}
}

func assertSQLiteContentionRows(t *testing.T, db *sql.DB, samples []sqliteContentionSample, includeDirect bool) {
	t.Helper()

	successByKind := make(map[string]int)
	resultByKey := make(map[string]sqliteContentionSample)
	var deletedHealthRows int64
	for _, sample := range samples {
		if sample.err == nil {
			successByKind[sample.kind]++
			deletedHealthRows += sample.rows
		}
		resultByKey[sample.kind+"|"+sample.key] = sample
	}

	assertSQLiteContentionCount(t, db,
		`SELECT COUNT(*) FROM request_log WHERE provider_id = 'traffic'`,
		successByKind["log_batch"],
		"日志批写行数",
	)
	assertSQLiteContentionCount(t, db,
		`SELECT COUNT(*) FROM runtime_state`,
		successByKind["single_queue"],
		"单次队列写入行数",
	)

	if !includeDirect {
		return
	}

	for i := 0; i < sqliteContentionRenameTransactions; i++ {
		providerID := fmt.Sprintf("%d", i+1)
		oldName := fmt.Sprintf("old-%02d", i)
		newName := fmt.Sprintf("new-%02d", i)
		sample := resultByKey["provider_rename|"+providerID]
		expectedName := oldName
		if sample.err == nil {
			expectedName = newName
		}

		assertSQLiteContentionName(t, db,
			`SELECT provider_name FROM provider_blacklist WHERE platform = 'codex' AND provider_id = ?`,
			providerID,
			expectedName,
			"黑名单 Provider 名称",
		)
		assertSQLiteContentionName(t, db,
			`SELECT MIN(provider) FROM request_log WHERE platform = 'codex' AND provider_id = ?`,
			providerID,
			expectedName,
			"请求日志 Provider 名称",
		)
		assertSQLiteContentionCount(t, db,
			`SELECT COUNT(*) FROM request_log WHERE platform = 'codex' AND provider_id = ? AND provider = ?`,
			sqliteContentionRenameRows,
			"请求日志 Provider 原子更新",
			providerID,
			expectedName,
		)
		assertSQLiteContentionName(t, db,
			`SELECT provider_name FROM health_check_history WHERE platform = 'codex' AND provider_id = ?`,
			providerID,
			expectedName,
			"健康历史 Provider 名称",
		)
		for _, table := range []string{requestLogStatsHourlyTable, requestLogStatsDailyTable} {
			assertSQLiteContentionName(t, db,
				fmt.Sprintf(`SELECT provider FROM %s WHERE platform = 'codex' AND provider_id = ?`, table),
				providerID,
				expectedName,
				"聚合统计 Provider 名称",
			)
		}
	}

	assertSQLiteContentionCount(t, db,
		`SELECT COUNT(*) FROM health_check_history WHERE platform = 'cleanup'`,
		sqliteContentionCleanupRows-int(deletedHealthRows),
		"健康历史清理行数",
	)
}

func assertSQLiteContentionCount(t *testing.T, db *sql.DB, query string, expected int, description string, args ...interface{}) {
	t.Helper()
	var count int
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatalf("读取%s失败: %v", description, err)
	}
	if count != expected {
		t.Errorf("%s=%d，期望=%d", description, count, expected)
	}
}

func assertSQLiteContentionName(t *testing.T, db *sql.DB, query string, arg interface{}, expected, description string) {
	t.Helper()
	var name string
	if err := db.QueryRow(query, arg).Scan(&name); err != nil {
		t.Fatalf("读取%s失败: %v", description, err)
	}
	if name != expected {
		t.Errorf("%s=%q，期望=%q", description, name, expected)
	}
}

func summarizeSQLiteContention(samples []sqliteContentionSample, elapsed time.Duration, stats sql.DBStats) sqliteContentionScenarioResult {
	result := sqliteContentionScenarioResult{
		elapsed:      elapsed,
		operations:   len(samples),
		waitCount:    stats.WaitCount,
		waitDuration: stats.WaitDuration,
		byKind:       make(map[string]sqliteContentionMetrics),
	}
	durations := make(map[string][]time.Duration)
	successDurations := make(map[string][]time.Duration)
	for _, sample := range samples {
		metrics := result.byKind[sample.kind]
		metrics.operations++
		if sample.err == nil {
			metrics.successes++
			result.successes++
			successDurations[sample.kind] = append(successDurations[sample.kind], sample.duration)
		}
		if isSQLiteContentionBusyError(sample.err) {
			metrics.busy++
			result.busy++
		}
		if isSQLiteContentionLockedError(sample.err) {
			metrics.locked++
			result.locked++
		}
		if sample.err != nil && metrics.firstError == "" {
			metrics.firstError = sample.err.Error()
		}
		result.byKind[sample.kind] = metrics
		durations[sample.kind] = append(durations[sample.kind], sample.duration)
	}
	for kind, values := range durations {
		metrics := result.byKind[kind]
		metrics.p50 = durationPercentile(values, 0.50)
		metrics.p95 = durationPercentile(values, 0.95)
		metrics.successP50 = durationPercentile(successDurations[kind], 0.50)
		metrics.successP95 = durationPercentile(successDurations[kind], 0.95)
		result.byKind[kind] = metrics
	}
	return result
}

func logSQLiteContentionResult(t *testing.T, scenario string, result sqliteContentionScenarioResult) {
	t.Helper()
	t.Logf(
		"scenario=%s operations=%d successes=%d busy=%d locked=%d elapsed_ms=%.3f throughput_ops_s=%.1f db_wait_count=%d db_wait_ms=%.3f",
		scenario,
		result.operations,
		result.successes,
		result.busy,
		result.locked,
		sqliteContentionMilliseconds(result.elapsed),
		float64(result.successes)/result.elapsed.Seconds(),
		result.waitCount,
		sqliteContentionMilliseconds(result.waitDuration),
	)

	kinds := make([]string, 0, len(result.byKind))
	for kind := range result.byKind {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		metrics := result.byKind[kind]
		t.Logf(
			"scenario=%s kind=%s operations=%d successes=%d busy=%d locked=%d attempt_p50_ms=%.3f attempt_p95_ms=%.3f success_p50_ms=%.3f success_p95_ms=%.3f first_error=%q",
			scenario,
			kind,
			metrics.operations,
			metrics.successes,
			metrics.busy,
			metrics.locked,
			sqliteContentionMilliseconds(metrics.p50),
			sqliteContentionMilliseconds(metrics.p95),
			sqliteContentionMilliseconds(metrics.successP50),
			sqliteContentionMilliseconds(metrics.successP95),
			metrics.firstError,
		)
	}
}

func durationPercentile(values []time.Duration, percentile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	ordered := append([]time.Duration(nil), values...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	index := int(float64(len(ordered)-1) * percentile)
	return ordered[index]
}

func sqliteContentionMilliseconds(value time.Duration) float64 {
	return float64(value) / float64(time.Millisecond)
}

func isSQLiteContentionBusyError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "sqlite_busy") ||
		(strings.Contains(message, "database is locked") && !strings.Contains(message, "database table is locked"))
}

func isSQLiteContentionLockError(err error) bool {
	return isSQLiteContentionBusyError(err) || isSQLiteContentionLockedError(err)
}

func isSQLiteContentionLockedError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "sqlite_locked") ||
		strings.Contains(message, "database table is locked") ||
		strings.Contains(message, "database is deadlocked")
}
