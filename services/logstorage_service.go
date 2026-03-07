package services

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/daodao97/xgo/xdb"
)

type LogTableStorageStat struct {
	Name  string `json:"name"`
	Rows  int64  `json:"rows"`
	Bytes int64  `json:"bytes"`
}

type LogDatabaseStorageStat struct {
	FileBytes  int64 `json:"file_bytes"`
	WalBytes   int64 `json:"wal_bytes"`
	ShmBytes   int64 `json:"shm_bytes"`
	TotalBytes int64 `json:"total_bytes"`
	UsedBytes  int64 `json:"used_bytes"`
	FreeBytes  int64 `json:"free_bytes"`
}

type LogStorageStats struct {
	Database   LogDatabaseStorageStat `json:"database"`
	RequestLog LogTableStorageStat    `json:"request_log"`
	StatsHour  LogTableStorageStat    `json:"stats_hour"`
	StatsDay   LogTableStorageStat    `json:"stats_day"`
}

type DeleteRequestLogsByDateResult struct {
	DeletedRequestLogs int64 `json:"deleted_request_logs"`
	DeletedStatsHour   int64 `json:"deleted_stats_hour"`
	DeletedStatsDay    int64 `json:"deleted_stats_day"`
}

func (ls *LogService) GetLogStorageStats() (LogStorageStats, error) {
	stats := LogStorageStats{
		RequestLog: LogTableStorageStat{Name: "request_log"},
		StatsHour:  LogTableStorageStat{Name: requestLogStatsHourlyTable},
		StatsDay:   LogTableStorageStat{Name: requestLogStatsDailyTable},
	}

	db, err := xdb.DB("default")
	if err != nil {
		return stats, err
	}

	dbFile, err := resolveSQLiteMainDBFile(db)
	if err == nil && dbFile != "" {
		stats.Database.FileBytes = fileSize(dbFile)
		stats.Database.WalBytes = fileSize(dbFile + "-wal")
		stats.Database.ShmBytes = fileSize(dbFile + "-shm")
	}

	pageSize, pageCount, freelistCount, err := querySQLitePageStats(db)
	if err == nil && pageSize > 0 && pageCount >= 0 && freelistCount >= 0 {
		stats.Database.TotalBytes = pageSize * pageCount
		stats.Database.FreeBytes = pageSize * freelistCount
		stats.Database.UsedBytes = stats.Database.TotalBytes - stats.Database.FreeBytes
	}

	stats.RequestLog.Rows = countTableRows(db, "request_log")
	stats.StatsHour.Rows = countTableRows(db, requestLogStatsHourlyTable)
	stats.StatsDay.Rows = countTableRows(db, requestLogStatsDailyTable)

	tableBytes, err := querySQLiteDbstatBytes(db)
	if err == nil && len(tableBytes) > 0 {
		stats.RequestLog.Bytes = sumBytes(tableBytes,
			"request_log",
			"idx_request_log_created_at",
			"idx_request_log_platform_created_at",
			"idx_request_log_platform_provider_created_at",
			"idx_request_log_platform_provider_id_created_at",
		)
		stats.StatsHour.Bytes = tableBytes[requestLogStatsHourlyTable]
		stats.StatsDay.Bytes = tableBytes[requestLogStatsDailyTable]
	}

	return stats, nil
}

func (ls *LogService) ClearRequestLogs() error {
	db, err := xdb.DB("default")
	if err != nil {
		return err
	}
	triggersCleaned := false
	defer func() {
		if !triggersCleaned {
			return
		}
		if err := ensureRequestLogStatsTriggersWithDB(db); err != nil {
			fmt.Printf("[WARN] ensureRequestLogStatsTriggersWithDB failed after clearing request_log: %v\n", err)
		}
	}()
	// 防止历史遗留的 request_log DELETE 触发器误伤统计表
	if err := cleanupRequestLogStatsTriggersWithDB(db); err != nil {
		return err
	}
	triggersCleaned = true
	if _, err := db.Exec("DELETE FROM request_log"); err != nil {
		return err
	}
	_, _ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return nil
}

func (ls *LogService) ClearLogStats() error {
	db, err := xdb.DB("default")
	if err != nil {
		return err
	}
	if _, err := db.Exec(fmt.Sprintf("DELETE FROM %s", requestLogStatsHourlyTable)); err != nil {
		return err
	}
	if _, err := db.Exec(fmt.Sprintf("DELETE FROM %s", requestLogStatsDailyTable)); err != nil {
		return err
	}
	_, _ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return nil
}

func (ls *LogService) RequestLogDailyHeatmapStats(days int) ([]HeatmapStat, error) {
	if days <= 0 {
		days = 365
	}
	startDay := startOfDay(time.Now()).AddDate(0, 0, -(days - 1))
	endDay := startOfDay(time.Now()).AddDate(0, 0, 1)

	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT
			strftime('%Y-%m-%d', datetime(created_at, 'localtime')) AS day,
			COUNT(*) AS total_requests
		FROM request_log
		WHERE created_at >= ? AND created_at < ?
		GROUP BY day
		ORDER BY day DESC
	`, startDay.UTC().Format(timeLayout), endDay.UTC().Format(timeLayout))
	if err != nil {
		if isNoSuchTableErr(err) {
			return []HeatmapStat{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	stats := make([]HeatmapStat, 0, days)
	for rows.Next() {
		stat := HeatmapStat{}
		if err := rows.Scan(&stat.Day, &stat.TotalRequests); err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}

func (ls *LogService) DeleteRequestLogsByDate(day string) (DeleteRequestLogsByDateResult, error) {
	result := DeleteRequestLogsByDateResult{}
	dayStart, err := parseLogStorageDay(day)
	if err != nil {
		return result, err
	}
	dayEnd := dayStart.AddDate(0, 0, 1)

	db, err := xdb.DB("default")
	if err != nil {
		return result, err
	}
	triggersCleaned := false
	defer func() {
		if !triggersCleaned {
			return
		}
		if err := ensureRequestLogStatsTriggersWithDB(db); err != nil {
			fmt.Printf("[WARN] ensureRequestLogStatsTriggersWithDB failed after deleting request_log by date: %v\n", err)
		}
	}()
	if err := cleanupRequestLogStatsTriggersWithDB(db); err != nil {
		return result, err
	}
	triggersCleaned = true

	tx, err := db.Begin()
	if err != nil {
		return result, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	requestLogExec, err := tx.Exec(
		"DELETE FROM request_log WHERE created_at >= ? AND created_at < ?",
		dayStart.UTC().Format(timeLayout),
		dayEnd.UTC().Format(timeLayout),
	)
	if err != nil {
		return result, err
	}
	result.DeletedRequestLogs = rowsAffected(requestLogExec)

	dayStartKey := dayStart.Format(timeLayout)
	dayEndKey := dayEnd.Format(timeLayout)
	deletedHourly, err := deleteRequestLogStatsRangeTx(tx, requestLogStatsHourlyTable, dayStartKey, dayEndKey)
	if err != nil {
		return result, err
	}
	result.DeletedStatsHour = deletedHourly

	deletedDaily, err := deleteRequestLogStatsRangeTx(tx, requestLogStatsDailyTable, dayStartKey, dayEndKey)
	if err != nil {
		return result, err
	}
	result.DeletedStatsDay = deletedDaily

	if err := tx.Commit(); err != nil {
		return result, err
	}
	committed = true

	_, _ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return result, nil
}

func parseLogStorageDay(value string) (time.Time, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return time.Time{}, fmt.Errorf("invalid date")
	}
	if len(raw) > len("2006-01-02") {
		raw = raw[:len("2006-01-02")]
	}
	parsed, err := time.ParseInLocation("2006-01-02", raw, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date: %s", value)
	}
	return startOfDay(parsed), nil
}

func deleteRequestLogStatsRangeTx(tx *sql.Tx, table string, startKey string, endKey string) (int64, error) {
	if tx == nil {
		return 0, fmt.Errorf("nil tx")
	}
	if strings.TrimSpace(table) == "" {
		return 0, fmt.Errorf("empty table")
	}
	execResult, err := tx.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE bucket_start >= ? AND bucket_start < ?", table),
		startKey,
		endKey,
	)
	if err != nil && !isNoSuchTableErr(err) {
		return 0, err
	}
	if err != nil {
		return 0, nil
	}
	return rowsAffected(execResult), nil
}

func rowsAffected(result sql.Result) int64 {
	if result == nil {
		return 0
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0
	}
	return count
}

func resolveSQLiteMainDBFile(db *sql.DB) (string, error) {
	if db == nil {
		return "", errors.New("nil db")
	}
	rows, err := db.Query("PRAGMA database_list")
	if err != nil {
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var seq int
		var name string
		var file string
		if err := rows.Scan(&seq, &name, &file); err != nil {
			return "", err
		}
		if name == "main" {
			return file, nil
		}
	}
	return "", sql.ErrNoRows
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func querySQLitePageStats(db *sql.DB) (pageSize int64, pageCount int64, freelistCount int64, err error) {
	if db == nil {
		return 0, 0, 0, errors.New("nil db")
	}
	var ps, pc, fc int64
	if err := db.QueryRow("PRAGMA page_size").Scan(&ps); err != nil {
		return 0, 0, 0, err
	}
	if err := db.QueryRow("PRAGMA page_count").Scan(&pc); err != nil {
		return 0, 0, 0, err
	}
	if err := db.QueryRow("PRAGMA freelist_count").Scan(&fc); err != nil {
		return 0, 0, 0, err
	}
	return ps, pc, fc, nil
}

func countTableRows(db *sql.DB, table string) int64 {
	if db == nil || strings.TrimSpace(table) == "" {
		return 0
	}
	var count int64
	err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

func querySQLiteDbstatBytes(db *sql.DB) (map[string]int64, error) {
	if db == nil {
		return nil, errors.New("nil db")
	}
	rows, err := db.Query("SELECT name, SUM(pgsize) AS bytes FROM dbstat GROUP BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]int64{}
	for rows.Next() {
		var name string
		var bytes int64
		if err := rows.Scan(&name, &bytes); err != nil {
			return nil, err
		}
		out[name] = bytes
	}
	return out, nil
}

func sumBytes(m map[string]int64, keys ...string) int64 {
	var total int64
	for _, key := range keys {
		total += m[key]
	}
	return total
}
