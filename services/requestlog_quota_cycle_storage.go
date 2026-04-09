package services

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	requestLogQuotaCycleStateTable   = "request_log_quota_cycle_state"
	requestLogQuotaCycleTriggerName  = "request_log_quota_cycle_ai"
	requestLogQuotaCycleMigrationKey = "request_log_quota_cycle_v1_backfill"
)

type fiveHourQuotaCycleState struct {
	Platform    string
	WindowStart time.Time
	NextReset   time.Time
	Used        float64
}

func ensureRequestLogQuotaCycleStorageWithDB(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}

	if err := ensureRequestLogQuotaCycleTableWithDB(db); err != nil {
		return err
	}
	if err := ensureRequestLogQuotaCycleTriggerWithDB(db); err != nil {
		return err
	}
	if err := ensureRequestLogProviderQuotaCycleStorageWithDB(db); err != nil {
		return err
	}
	if err := ensureSchemaMigrationsTable(db); err != nil {
		return err
	}

	applied, err := isSchemaMigrationApplied(db, requestLogQuotaCycleMigrationKey)
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	if err := backfillRequestLogQuotaCycleStateWithDB(db); err != nil {
		return err
	}
	if err := markSchemaMigrationApplied(db, requestLogQuotaCycleMigrationKey, time.Now().Format(timeLayout)); err != nil {
		return err
	}
	return nil
}

func ensureRequestLogQuotaCycleTableWithDB(db *sql.DB) error {
	createTableSQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		platform TEXT PRIMARY KEY,
		five_hour_window_start TEXT NOT NULL DEFAULT '',
		five_hour_next_reset TEXT NOT NULL DEFAULT '',
		five_hour_used REAL NOT NULL DEFAULT 0,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	) WITHOUT ROWID`, requestLogQuotaCycleStateTable)

	if _, err := db.Exec(createTableSQL); err != nil {
		return err
	}
	if err := ensureRequestLogQuotaCycleColumnWithDB(db, "five_hour_window_start", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureRequestLogQuotaCycleColumnWithDB(db, "five_hour_next_reset", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureRequestLogQuotaCycleColumnWithDB(db, "five_hour_used", "REAL NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureRequestLogQuotaCycleColumnWithDB(db, "updated_at", "TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP"); err != nil {
		return err
	}
	return nil
}

func ensureRequestLogQuotaCycleColumnWithDB(db *sql.DB, column string, definition string) error {
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", requestLogQuotaCycleStateTable)
	var count int
	if err := db.QueryRow(query, column).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	alter := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", requestLogQuotaCycleStateTable, column, definition)
	_, err := db.Exec(alter)
	return err
}

func ensureRequestLogQuotaCycleTriggerWithDB(db *sql.DB) error {
	if _, err := db.Exec(fmt.Sprintf(`DROP TRIGGER IF EXISTS "%s"`, requestLogQuotaCycleTriggerName)); err != nil {
		return err
	}

	triggerSQL := fmt.Sprintf(`
CREATE TRIGGER IF NOT EXISTS %[1]s
AFTER INSERT ON request_log
WHEN TRIM(COALESCE(NEW.platform, '')) <> ''
BEGIN
  INSERT INTO %[2]s (
    platform,
    five_hour_window_start,
    five_hour_next_reset,
    five_hour_used,
    updated_at
  ) VALUES (
    TRIM(COALESCE(NEW.platform, '')),
    NEW.created_at,
    datetime(NEW.created_at, '+5 hours'),
    COALESCE(NEW.total_cost, 0),
    CURRENT_TIMESTAMP
  )
  ON CONFLICT(platform) DO UPDATE SET
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
		requestLogQuotaCycleTriggerName,
		requestLogQuotaCycleStateTable,
	)

	_, err := db.Exec(triggerSQL)
	return err
}

func queryFiveHourQuotaCycleState(db *sql.DB, platform string) (fiveHourQuotaCycleState, error) {
	state := fiveHourQuotaCycleState{
		Platform: strings.TrimSpace(platform),
	}
	if db == nil || state.Platform == "" {
		return state, nil
	}

	var rawWindowStart sql.NullString
	var rawNextReset sql.NullString
	if err := db.QueryRow(
		fmt.Sprintf(`
			SELECT five_hour_window_start, five_hour_next_reset, five_hour_used
			FROM %s
			WHERE platform = ?
			LIMIT 1
		`, requestLogQuotaCycleStateTable),
		state.Platform,
	).Scan(&rawWindowStart, &rawNextReset, &state.Used); err != nil {
		if err == sql.ErrNoRows {
			return fiveHourQuotaCycleState{}, nil
		}
		return fiveHourQuotaCycleState{}, err
	}

	windowStart, err := parseStoredRequestLogTime(rawWindowStart)
	if err != nil {
		return fiveHourQuotaCycleState{}, err
	}
	nextReset, err := parseStoredRequestLogTime(rawNextReset)
	if err != nil {
		return fiveHourQuotaCycleState{}, err
	}

	state.WindowStart = windowStart
	state.NextReset = nextReset
	state.Used = normalizeBudgetRawUsed(state.Used)
	return state, nil
}

func upsertFiveHourQuotaCycleStateWithDB(db *sql.DB, state fiveHourQuotaCycleState) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}
	platform := strings.TrimSpace(state.Platform)
	if platform == "" || state.WindowStart.IsZero() || state.NextReset.IsZero() {
		return nil
	}

	_, err := db.Exec(
		fmt.Sprintf(`
			INSERT INTO %s (
				platform,
				five_hour_window_start,
				five_hour_next_reset,
				five_hour_used,
				updated_at
			) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(platform) DO UPDATE SET
				five_hour_window_start = excluded.five_hour_window_start,
				five_hour_next_reset = excluded.five_hour_next_reset,
				five_hour_used = excluded.five_hour_used,
				updated_at = CURRENT_TIMESTAMP
		`, requestLogQuotaCycleStateTable),
		platform,
		state.WindowStart.UTC().Format(timeLayout),
		state.NextReset.UTC().Format(timeLayout),
		normalizeBudgetRawUsed(state.Used),
	)
	return err
}

func backfillRequestLogQuotaCycleStateWithDB(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}

	platforms, err := listRequestLogPlatformsWithDB(db)
	if err != nil {
		if isNoSuchTableErr(err) {
			return nil
		}
		return err
	}
	for _, platform := range platforms {
		state, err := buildFiveHourQuotaCycleStateFromHistory(db, platform)
		if err != nil {
			return err
		}
		if err := upsertFiveHourQuotaCycleStateWithDB(db, state); err != nil {
			return err
		}
	}
	return nil
}

func listRequestLogPlatformsWithDB(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
		SELECT DISTINCT TRIM(COALESCE(platform, '')) AS platform
		FROM request_log
		WHERE TRIM(COALESCE(platform, '')) <> ''
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	platforms := make([]string, 0, 4)
	for rows.Next() {
		var platform string
		if err := rows.Scan(&platform); err != nil {
			return nil, err
		}
		platform = strings.TrimSpace(platform)
		if platform == "" {
			continue
		}
		platforms = append(platforms, platform)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return platforms, nil
}

func buildFiveHourQuotaCycleStateFromHistory(db *sql.DB, platform string) (fiveHourQuotaCycleState, error) {
	state := fiveHourQuotaCycleState{
		Platform: strings.TrimSpace(platform),
	}
	if db == nil || state.Platform == "" {
		return state, nil
	}

	latestCreatedAt, err := queryLatestRequestLogCreatedAt(db, state.Platform)
	if err != nil {
		return state, err
	}
	if latestCreatedAt.IsZero() {
		return state, nil
	}

	windowStart, err := queryLatestFiveHourQuotaWindowStart(db, state.Platform)
	if err != nil {
		return state, err
	}
	if windowStart.IsZero() {
		return state, nil
	}

	nextReset := windowStart.Add(fiveHourQuotaWindowDuration)
	used, err := queryRequestLogCostBetween(db, state.Platform, windowStart, nextReset)
	if err != nil {
		return state, err
	}

	state.WindowStart = windowStart
	state.NextReset = nextReset
	state.Used = normalizeBudgetRawUsed(used)
	return state, nil
}
