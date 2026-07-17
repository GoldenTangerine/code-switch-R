package services

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	requestLogProviderQuotaCycleStateTable   = "request_log_provider_quota_cycle_state"
	requestLogProviderQuotaCycleTriggerName  = "request_log_provider_quota_cycle_ai"
	requestLogProviderQuotaCycleMigrationKey = "request_log_provider_quota_cycle_provider_v1_backfill"
)

type providerFiveHourQuotaCycleState struct {
	Platform    string
	ProviderRef string
	WindowStart time.Time
	NextReset   time.Time
	Used        float64
}

type requestLogProviderRefRecord struct {
	Platform    string
	ProviderRef string
}

type requestLogQueryRower interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

func ensureRequestLogProviderQuotaCycleStorageWithDB(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}

	if err := ensureRequestLogProviderQuotaCycleTableWithDB(db); err != nil {
		return err
	}
	if err := ensureRequestLogProviderQuotaCycleTriggerWithDB(db); err != nil {
		return err
	}
	if err := ensureSchemaMigrationsTable(db); err != nil {
		return err
	}

	applied, err := isSchemaMigrationApplied(db, requestLogProviderQuotaCycleMigrationKey)
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	if err := backfillRequestLogProviderQuotaCycleStateWithDB(db); err != nil {
		return err
	}
	if err := markSchemaMigrationApplied(db, requestLogProviderQuotaCycleMigrationKey, time.Now().Format(timeLayout)); err != nil {
		return err
	}
	return nil
}

func ensureRequestLogProviderQuotaCycleTableWithDB(db *sql.DB) error {
	createTableSQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		platform TEXT NOT NULL,
		provider_ref TEXT NOT NULL,
		five_hour_window_start TEXT NOT NULL DEFAULT '',
		five_hour_next_reset TEXT NOT NULL DEFAULT '',
		five_hour_used REAL NOT NULL DEFAULT 0,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (platform, provider_ref)
	) WITHOUT ROWID`, requestLogProviderQuotaCycleStateTable)

	if _, err := db.Exec(createTableSQL); err != nil {
		return err
	}
	if err := ensureRequestLogProviderQuotaCycleColumnWithDB(db, "provider_ref", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureRequestLogProviderQuotaCycleColumnWithDB(db, "five_hour_window_start", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureRequestLogProviderQuotaCycleColumnWithDB(db, "five_hour_next_reset", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureRequestLogProviderQuotaCycleColumnWithDB(db, "five_hour_used", "REAL NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogProviderQuotaCycleColumnWithDB(db, "updated_at", "TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP"); err != nil {
		return err
	}
	return nil
}

func ensureRequestLogProviderQuotaCycleColumnWithDB(db *sql.DB, column string, definition string) error {
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", requestLogProviderQuotaCycleStateTable)
	var count int
	if err := db.QueryRow(query, column).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	alter := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", requestLogProviderQuotaCycleStateTable, column, definition)
	_, err := db.Exec(alter)
	return err
}

func ensureRequestLogProviderQuotaCycleTriggerWithDB(db *sql.DB) error {
	if _, err := db.Exec(fmt.Sprintf(`DROP TRIGGER IF EXISTS "%s"`, requestLogProviderQuotaCycleTriggerName)); err != nil {
		return err
	}

	providerRefExpr := `
		CASE
			WHEN TRIM(COALESCE(NEW.provider_id, '')) <> '' THEN TRIM(NEW.provider_id)
			ELSE TRIM(COALESCE(NEW.provider, ''))
		END
	`
	triggerSQL := fmt.Sprintf(`
CREATE TRIGGER IF NOT EXISTS %[1]s
AFTER INSERT ON request_log
WHEN TRIM(COALESCE(NEW.platform, '')) <> ''
  AND (TRIM(COALESCE(NEW.provider_id, '')) <> '' OR TRIM(COALESCE(NEW.provider, '')) <> '')
  AND COALESCE(NULLIF(TRIM(NEW.data_source), ''), 'proxy') = 'proxy'
BEGIN
  INSERT INTO %[2]s (
    platform,
    provider_ref,
    five_hour_window_start,
    five_hour_next_reset,
    five_hour_used,
    updated_at
  ) VALUES (
    TRIM(COALESCE(NEW.platform, '')),
    %[3]s,
    NEW.created_at,
    datetime(NEW.created_at, '+5 hours'),
    COALESCE(NEW.total_cost, 0),
    CURRENT_TIMESTAMP
  )
  ON CONFLICT(platform, provider_ref) DO UPDATE SET
    five_hour_window_start = CASE
      WHEN TRIM(COALESCE(%[2]s.five_hour_next_reset, '')) = ''
        OR excluded.five_hour_window_start >= %[2]s.five_hour_next_reset
      THEN excluded.five_hour_window_start
      ELSE %[2]s.five_hour_window_start
    END,
    five_hour_next_reset = CASE
      WHEN TRIM(COALESCE(%[2]s.five_hour_next_reset, '')) = ''
        OR excluded.five_hour_window_start >= %[2]s.five_hour_next_reset
      THEN excluded.five_hour_next_reset
      ELSE %[2]s.five_hour_next_reset
    END,
    five_hour_used = CASE
      WHEN TRIM(COALESCE(%[2]s.five_hour_next_reset, '')) = ''
        OR excluded.five_hour_window_start >= %[2]s.five_hour_next_reset
      THEN excluded.five_hour_used
      ELSE %[2]s.five_hour_used + excluded.five_hour_used
    END,
    updated_at = CURRENT_TIMESTAMP;
END;`,
		requestLogProviderQuotaCycleTriggerName,
		requestLogProviderQuotaCycleStateTable,
		providerRefExpr,
	)

	_, err := db.Exec(triggerSQL)
	return err
}

func queryFiveHourQuotaCycleStateByProvider(queryer requestLogQueryRower, platform string, providerRef string) (providerFiveHourQuotaCycleState, error) {
	state := providerFiveHourQuotaCycleState{
		Platform:    strings.TrimSpace(platform),
		ProviderRef: strings.TrimSpace(providerRef),
	}
	if queryer == nil || state.Platform == "" || state.ProviderRef == "" {
		return state, nil
	}

	var rawWindowStart sql.NullString
	var rawNextReset sql.NullString
	if err := queryer.QueryRow(
		fmt.Sprintf(`
			SELECT five_hour_window_start, five_hour_next_reset, five_hour_used
			FROM %s
			WHERE platform = ? AND provider_ref = ?
			LIMIT 1
		`, requestLogProviderQuotaCycleStateTable),
		state.Platform,
		state.ProviderRef,
	).Scan(&rawWindowStart, &rawNextReset, &state.Used); err != nil {
		if err == sql.ErrNoRows {
			return providerFiveHourQuotaCycleState{}, nil
		}
		return providerFiveHourQuotaCycleState{}, err
	}

	windowStart, err := parseStoredRequestLogTime(rawWindowStart)
	if err != nil {
		return providerFiveHourQuotaCycleState{}, err
	}
	nextReset, err := parseStoredRequestLogTime(rawNextReset)
	if err != nil {
		return providerFiveHourQuotaCycleState{}, err
	}

	state.WindowStart = windowStart
	state.NextReset = nextReset
	state.Used = normalizeBudgetRawUsed(state.Used)
	return state, nil
}

func upsertFiveHourQuotaCycleStateByProviderWithExec(exec providerIdentitySyncExecer, state providerFiveHourQuotaCycleState) error {
	if exec == nil {
		return fmt.Errorf("nil exec")
	}

	platform := strings.TrimSpace(state.Platform)
	providerRef := strings.TrimSpace(state.ProviderRef)
	if platform == "" || providerRef == "" || state.WindowStart.IsZero() || state.NextReset.IsZero() {
		return nil
	}

	_, err := exec.Exec(
		fmt.Sprintf(`
			INSERT INTO %s (
				platform,
				provider_ref,
				five_hour_window_start,
				five_hour_next_reset,
				five_hour_used,
				updated_at
			) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(platform, provider_ref) DO UPDATE SET
				five_hour_window_start = excluded.five_hour_window_start,
				five_hour_next_reset = excluded.five_hour_next_reset,
				five_hour_used = excluded.five_hour_used,
				updated_at = CURRENT_TIMESTAMP
		`, requestLogProviderQuotaCycleStateTable),
		platform,
		providerRef,
		state.WindowStart.UTC().Format(timeLayout),
		state.NextReset.UTC().Format(timeLayout),
		normalizeBudgetRawUsed(state.Used),
	)
	return err
}

func deleteFiveHourQuotaCycleStateByProviderWithExec(exec providerIdentitySyncExecer, platform string, providerRef string) error {
	if exec == nil {
		return fmt.Errorf("nil exec")
	}
	platform = strings.TrimSpace(platform)
	providerRef = strings.TrimSpace(providerRef)
	if platform == "" || providerRef == "" {
		return nil
	}
	_, err := exec.Exec(
		fmt.Sprintf(`DELETE FROM %s WHERE platform = ? AND provider_ref = ?`, requestLogProviderQuotaCycleStateTable),
		platform,
		providerRef,
	)
	return err
}

func backfillRequestLogProviderQuotaCycleStateWithDB(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}

	refs, err := listRequestLogProviderRefsWithDB(db)
	if err != nil {
		if isNoSuchTableErr(err) {
			return nil
		}
		return err
	}

	for _, item := range refs {
		state, err := buildFiveHourQuotaCycleStateFromHistoryByProvider(db, item.Platform, item.ProviderRef)
		if err != nil {
			return err
		}
		if err := upsertFiveHourQuotaCycleStateByProviderWithExec(db, state); err != nil {
			return err
		}
	}
	return nil
}

func listRequestLogProviderRefsWithDB(db *sql.DB) ([]requestLogProviderRefRecord, error) {
	rows, err := db.Query(`
		SELECT DISTINCT
			TRIM(COALESCE(platform, '')) AS platform,
			CASE
				WHEN TRIM(COALESCE(provider_id, '')) <> '' THEN TRIM(provider_id)
				WHEN TRIM(COALESCE(provider, '')) <> '' THEN TRIM(provider)
				ELSE ''
			END AS provider_ref
		FROM request_log
		WHERE TRIM(COALESCE(platform, '')) <> ''
		  AND (TRIM(COALESCE(provider_id, '')) <> '' OR TRIM(COALESCE(provider, '')) <> '')
		  AND COALESCE(NULLIF(TRIM(data_source), ''), 'proxy') = 'proxy'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]requestLogProviderRefRecord, 0, 16)
	for rows.Next() {
		var item requestLogProviderRefRecord
		if err := rows.Scan(&item.Platform, &item.ProviderRef); err != nil {
			return nil, err
		}
		item.Platform = strings.TrimSpace(item.Platform)
		item.ProviderRef = strings.TrimSpace(item.ProviderRef)
		if item.Platform == "" || item.ProviderRef == "" {
			continue
		}
		records = append(records, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func buildFiveHourQuotaCycleStateFromHistoryByProvider(queryer requestLogQueryRower, platform string, providerRef string) (providerFiveHourQuotaCycleState, error) {
	state := providerFiveHourQuotaCycleState{
		Platform:    strings.TrimSpace(platform),
		ProviderRef: strings.TrimSpace(providerRef),
	}
	if queryer == nil || state.Platform == "" || state.ProviderRef == "" {
		return state, nil
	}

	windowStart, err := queryLatestFiveHourQuotaWindowStartByProvider(queryer, state.Platform, state.ProviderRef)
	if err != nil {
		return state, err
	}
	if windowStart.IsZero() {
		return state, nil
	}

	nextReset := windowStart.Add(fiveHourQuotaWindowDuration)
	used, err := queryRequestLogCostBetweenByProvider(queryer, state.Platform, state.ProviderRef, windowStart, nextReset)
	if err != nil {
		return state, err
	}

	state.WindowStart = windowStart
	state.NextReset = nextReset
	state.Used = normalizeBudgetRawUsed(used)
	return state, nil
}

// queryLatestFiveHourQuotaWindowStartByProvider 使用递归 CTE 查找指定供应商当前 5 小时窗口的起始时间。
// 该查询主要用于一次性回填或身份同步时重建持久化状态，运行时展示优先读取持久化表。
func queryLatestFiveHourQuotaWindowStartByProvider(queryer requestLogQueryRower, platform string, providerRef string) (time.Time, error) {
	providerRef = strings.TrimSpace(providerRef)
	if queryer == nil || providerRef == "" {
		return time.Time{}, nil
	}

	providerWhere := ` AND (TRIM(COALESCE(provider_id,'')) = ? OR TRIM(COALESCE(provider,'')) = ?)`
	proxyWhere := ` WHERE ` + requestLogSourceWhereClause(LogDataSourceModeProxy, "request_log")
	proxyAnd := ` AND ` + requestLogSourceWhereClause(LogDataSourceModeProxy, "request_log")
	initialWhere := proxyWhere + providerWhere
	nextWhere := proxyAnd + providerWhere
	args := make([]interface{}, 0, 6)
	args = append(args, providerRef, providerRef)

	if strings.TrimSpace(platform) != "" {
		initialWhere = proxyWhere + ` AND platform = ?` + providerWhere
		nextWhere = proxyAnd + ` AND platform = ?` + providerWhere
		args = []interface{}{platform, providerRef, providerRef, platform, providerRef, providerRef}
	} else {
		args = []interface{}{providerRef, providerRef, providerRef, providerRef}
	}

	query := `
		WITH RECURSIVE cycle_starts(start_at) AS (
			SELECT MIN(created_at) FROM request_log` + initialWhere + `
			UNION ALL
			SELECT (
				SELECT MIN(created_at)
				FROM request_log
				WHERE created_at >= datetime(cycle_starts.start_at, '+5 hours')` + nextWhere + `
			)
			FROM cycle_starts
			WHERE cycle_starts.start_at IS NOT NULL
		)
		SELECT start_at
		FROM cycle_starts
		WHERE start_at IS NOT NULL
		ORDER BY start_at DESC
		LIMIT 1
	`

	var raw sql.NullString
	if err := queryer.QueryRow(query, args...).Scan(&raw); err != nil {
		return time.Time{}, err
	}
	return parseStoredRequestLogTime(raw)
}

func queryRequestLogCostBetweenByProvider(queryer requestLogQueryRower, platform string, providerRef string, start time.Time, end time.Time) (float64, error) {
	providerRef = strings.TrimSpace(providerRef)
	if queryer == nil || providerRef == "" {
		return 0, nil
	}

	query := `SELECT COALESCE(SUM(total_cost), 0) FROM request_log WHERE created_at >= ? AND created_at < ? AND ` + requestLogSourceWhereClause(LogDataSourceModeProxy, "request_log")
	args := []interface{}{start.UTC().Format(timeLayout), end.UTC().Format(timeLayout)}

	if strings.TrimSpace(platform) != "" {
		query += ` AND platform = ?`
		args = append(args, platform)
	}

	query += ` AND (TRIM(COALESCE(provider_id,'')) = ? OR TRIM(COALESCE(provider,'')) = ?)`
	args = append(args, providerRef, providerRef)

	total := 0.0
	if err := queryer.QueryRow(query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}
