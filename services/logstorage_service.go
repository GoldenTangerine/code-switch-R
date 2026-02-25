package services

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

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
	// 防止历史遗留的 request_log DELETE 触发器误伤统计表
	if err := cleanupRequestLogStatsTriggersWithDB(db); err != nil {
		return err
	}
	if _, err := db.Exec("DELETE FROM request_log"); err != nil {
		return err
	}
	// 确保统计触发器仍然存在（有些环境可能被用户手动删过）
	if err := ensureRequestLogStatsTriggersWithDB(db); err != nil {
		fmt.Printf("[WARN] ensureRequestLogStatsTriggersWithDB failed after clearing request_log: %v\n", err)
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
