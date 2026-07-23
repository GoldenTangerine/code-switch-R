package services

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
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

type ProviderLogStorageStat struct {
	Platform        string `json:"platform"`
	ProviderID      string `json:"provider_id,omitempty"`
	Provider        string `json:"provider"`
	RequestLogRows  int64  `json:"request_log_rows"`
	RequestLogBytes int64  `json:"request_log_bytes"`
	StatsRows       int64  `json:"stats_rows"`
	StatsBytes      int64  `json:"stats_bytes"`
	TotalBytes      int64  `json:"total_bytes"`
	LatestAt        string `json:"latest_at"`
}

type DeleteRequestLogsByDateResult struct {
	DeletedRequestLogs int64 `json:"deleted_request_logs"`
	DeletedStatsHour   int64 `json:"deleted_stats_hour"`
	DeletedStatsDay    int64 `json:"deleted_stats_day"`
}

type MarkRequestLogsReadResult struct {
	MarkedRequestLogs int64 `json:"marked_request_logs"`
}

type deleteProviderLogsTarget struct {
	Platform   string
	ProviderID string
	Provider   string
}

type providerLogStorageRow struct {
	Platform   string
	ProviderID string
	Provider   string
	Rows       int64
	Bytes      int64
	LatestAt   string
}

const providerLogStorageUnknownIdentity = "__unknown__"

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
			"idx_request_log_platform_provider_id_error_read_at",
			"idx_request_log_platform_provider_error_read_at",
		)
		stats.StatsHour.Bytes = tableBytes[requestLogStatsHourlyTable]
		stats.StatsDay.Bytes = tableBytes[requestLogStatsDailyTable]
	}

	return stats, nil
}

func (ls *LogService) ClearRequestLogs() error {
	return ls.ClearRequestLogsV2(false)
}

func (ls *LogService) ClearRequestLogsV2(reimportSessions bool) error {
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
	if GlobalDBQueue == nil {
		return fmt.Errorf("database write queue is not initialized")
	}
	if err := GlobalDBQueue.Exec("DELETE FROM request_log"); err != nil {
		return err
	}
	if reimportSessions {
		if err := resetSessionUsageSyncState(); err != nil {
			return err
		}
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

func (ls *LogService) ListProviderLogStorageStats() ([]ProviderLogStorageStat, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}

	merged := make(map[string]*ProviderLogStorageStat)

	requestRows, err := queryProviderRequestLogStorageRows(db)
	if err != nil {
		return nil, err
	}
	for _, row := range requestRows {
		entry := mergeProviderLogStorageRow(merged, row)
		entry.RequestLogRows += row.Rows
		entry.RequestLogBytes += row.Bytes
		entry.TotalBytes = entry.RequestLogBytes + entry.StatsBytes
	}

	statsHourlyRows, err := queryProviderStatsStorageRows(db, requestLogStatsHourlyTable)
	if err != nil {
		return nil, err
	}
	for _, row := range statsHourlyRows {
		entry := mergeProviderLogStorageRow(merged, row)
		entry.StatsRows += row.Rows
		entry.StatsBytes += row.Bytes
		entry.TotalBytes = entry.RequestLogBytes + entry.StatsBytes
	}

	statsDailyRows, err := queryProviderStatsStorageRows(db, requestLogStatsDailyTable)
	if err != nil {
		return nil, err
	}
	for _, row := range statsDailyRows {
		entry := mergeProviderLogStorageRow(merged, row)
		entry.StatsRows += row.Rows
		entry.StatsBytes += row.Bytes
		entry.TotalBytes = entry.RequestLogBytes + entry.StatsBytes
	}

	items := make([]ProviderLogStorageStat, 0, len(merged))
	for _, item := range merged {
		if item == nil {
			continue
		}
		item.TotalBytes = item.RequestLogBytes + item.StatsBytes
		items = append(items, *item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].TotalBytes != items[j].TotalBytes {
			return items[i].TotalBytes > items[j].TotalBytes
		}
		if items[i].RequestLogRows != items[j].RequestLogRows {
			return items[i].RequestLogRows > items[j].RequestLogRows
		}
		if items[i].StatsRows != items[j].StatsRows {
			return items[i].StatsRows > items[j].StatsRows
		}
		if items[i].LatestAt != items[j].LatestAt {
			return items[i].LatestAt > items[j].LatestAt
		}
		if items[i].Platform != items[j].Platform {
			return items[i].Platform < items[j].Platform
		}
		return items[i].Provider < items[j].Provider
	})

	return items, nil
}

func (ls *LogService) ClearProviderLogStorage(platform string, providerID string, provider string) (DeleteRequestLogsByDateResult, error) {
	result := DeleteRequestLogsByDateResult{}
	target, err := normalizeProviderLogStorageTarget(platform, providerID, provider)
	if err != nil {
		return result, err
	}

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
			fmt.Printf("[WARN] ensureRequestLogStatsTriggersWithDB failed after deleting request_log by provider: %v\n", err)
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

	whereClause, args := buildProviderLogStorageWhereClause(target)
	requestLogExec, err := tx.Exec(
		"DELETE FROM request_log WHERE "+whereClause,
		args...,
	)
	if err != nil {
		return result, err
	}
	result.DeletedRequestLogs = rowsAffected(requestLogExec)

	deletedHourly, err := deleteRequestLogStatsByProviderTx(tx, requestLogStatsHourlyTable, target)
	if err != nil {
		return result, err
	}
	result.DeletedStatsHour = deletedHourly

	deletedDaily, err := deleteRequestLogStatsByProviderTx(tx, requestLogStatsDailyTable, target)
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

func (ls *LogService) MarkProviderFailedRequestLogsRead(platform string, providerID string, provider string) (MarkRequestLogsReadResult, error) {
	result := MarkRequestLogsReadResult{}
	target, err := normalizeProviderLogStorageTarget(platform, providerID, provider)
	if err != nil {
		return result, err
	}

	whereClause, args := buildProviderUnreadFailedRequestLogWhereClause(target)
	return ls.markProviderFailedRequestLogsRead(whereClause, args)
}

func (ls *LogService) markProviderFailedRequestLogsRead(whereClause string, args []any) (MarkRequestLogsReadResult, error) {
	result := MarkRequestLogsReadResult{}
	if strings.TrimSpace(whereClause) == "" {
		return result, fmt.Errorf("empty where clause")
	}

	db, err := xdb.DB("default")
	if err != nil {
		return result, err
	}

	var matched int64
	if err := db.QueryRow("SELECT COUNT(*) FROM request_log WHERE "+whereClause, args...).Scan(&matched); err != nil {
		if isNoSuchTableErr(err) {
			return result, nil
		}
		return result, err
	}
	if matched == 0 {
		return result, nil
	}

	now := time.Now().UTC().Format(timeLayout)
	updateArgs := make([]any, 0, len(args)+1)
	updateArgs = append(updateArgs, now)
	updateArgs = append(updateArgs, args...)
	updateSQL := "UPDATE request_log SET error_read_at = ? WHERE " + whereClause

	if GlobalDBQueue != nil {
		if err := GlobalDBQueue.Exec(updateSQL, updateArgs...); err != nil {
			return result, err
		}
	} else {
		execResult, err := db.Exec(updateSQL, updateArgs...)
		if err != nil {
			return result, err
		}
		matched = rowsAffected(execResult)
	}

	result.MarkedRequestLogs = matched
	return result, nil
}

func (ls *LogService) RequestLogDailyHeatmapStats(days int) ([]HeatmapStat, error) {
	if days <= 0 {
		days = 365
	}
	startDay := startOfDay(time.Now()).AddDate(0, 0, -(days - 1))
	endDay := startOfDay(time.Now()).AddDate(0, 0, 1)
	return ls.requestLogDailyHeatmapStatsBetween(startDay, endDay)
}

func (ls *LogService) RequestLogDailyHeatmapStatsByYear(year int) ([]HeatmapStat, error) {
	return ls.RequestLogDailyHeatmapStatsByYearV2(year, string(LogDataSourceModeProxy))
}

func (ls *LogService) RequestLogDailyHeatmapStatsByYearV2(year int, sourceMode string) ([]HeatmapStat, error) {
	if year <= 0 {
		year = time.Now().Year()
	}
	startDay := time.Date(year, time.January, 1, 0, 0, 0, 0, time.Local)
	endDay := startDay.AddDate(1, 0, 0)
	return ls.requestLogDailyHeatmapStatsBetweenV2(startDay, endDay, normalizeLogDataSourceMode(sourceMode))
}

func (ls *LogService) ListRequestLogHeatmapYears() ([]int, error) {
	return ls.ListRequestLogHeatmapYearsV2(string(LogDataSourceModeProxy))
}

func (ls *LogService) ListRequestLogHeatmapYearsV2(sourceMode string) ([]int, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}

	query := `
		SELECT DISTINCT CAST(strftime('%Y', datetime(created_at, 'localtime')) AS INTEGER) AS year
		FROM request_log
		WHERE TRIM(COALESCE(created_at, '')) != '' AND ` +
		requestLogSourceWhereClause(normalizeLogDataSourceMode(sourceMode), "request_log") + `
		ORDER BY year DESC
	`
	rows, err := db.Query(query)
	if err != nil {
		if isNoSuchTableErr(err) {
			return []int{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	years := make([]int, 0, 8)
	for rows.Next() {
		var year int
		if err := rows.Scan(&year); err != nil {
			return nil, err
		}
		if year > 0 {
			years = append(years, year)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return years, nil
}

func (ls *LogService) requestLogDailyHeatmapStatsBetween(startDay time.Time, endDay time.Time) ([]HeatmapStat, error) {
	return ls.requestLogDailyHeatmapStatsBetweenV2(startDay, endDay, LogDataSourceModeProxy)
}

func (ls *LogService) requestLogDailyHeatmapStatsBetweenV2(startDay time.Time, endDay time.Time, sourceMode LogDataSourceMode) ([]HeatmapStat, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}

	sourceWhere := requestLogSourceWhereClause(sourceMode, "request_log")
	queryWithStoredPayloadColumns := `
			SELECT
				strftime('%Y-%m-%d', datetime(created_at, 'localtime')) AS day,
				COUNT(*) AS total_requests,
				SUM(COALESCE(payload_bytes, 0)) AS payload_bytes,
				SUM(CASE WHEN payload_captured != 0 THEN 1 ELSE 0 END) AS payload_captured_requests
			FROM request_log
			WHERE created_at >= ? AND created_at < ? AND ` + sourceWhere + `
			GROUP BY day
			ORDER BY day DESC
	`
	queryLegacyPayloadColumns := `
			SELECT
				strftime('%Y-%m-%d', datetime(created_at, 'localtime')) AS day,
				COUNT(*) AS total_requests,
				SUM(
					COALESCE(LENGTH(CAST(request_body AS BLOB)), 0)
					+ COALESCE(LENGTH(CAST(response_body AS BLOB)), 0)
				) AS payload_bytes,
				SUM(
					CASE
						WHEN request_body != '' OR response_body != '' OR request_body_truncated != 0 OR response_body_truncated != 0 THEN 1
						ELSE 0
					END
				) AS payload_captured_requests
			FROM request_log
			WHERE created_at >= ? AND created_at < ? AND ` + sourceWhere + `
			GROUP BY day
			ORDER BY day DESC
	`
	rows, err := db.Query(queryWithStoredPayloadColumns, startDay.UTC().Format(timeLayout), endDay.UTC().Format(timeLayout))
	if err != nil && strings.Contains(err.Error(), "no such column") {
		rows, err = db.Query(queryLegacyPayloadColumns, startDay.UTC().Format(timeLayout), endDay.UTC().Format(timeLayout))
	}
	if err != nil {
		if isNoSuchTableErr(err) {
			return []HeatmapStat{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	stats := make([]HeatmapStat, 0, 64)
	for rows.Next() {
		stat := HeatmapStat{}
		if err := rows.Scan(&stat.Day, &stat.TotalRequests, &stat.PayloadBytes, &stat.PayloadCapturedRequests); err != nil {
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

func normalizeProviderLogStorageTarget(platform string, providerID string, provider string) (deleteProviderLogsTarget, error) {
	normalizedProviderID := normalizeProviderLogStorageIdentity(providerID, provider)
	normalizedProvider := normalizeProviderLogStorageName(providerID, provider)
	target := deleteProviderLogsTarget{
		Platform:   strings.TrimSpace(platform),
		ProviderID: normalizedProviderID,
		Provider:   normalizedProvider,
	}
	if target.Platform == "" {
		return target, fmt.Errorf("invalid platform")
	}
	if target.ProviderID == "" || target.ProviderID == providerLogStorageUnknownIdentity {
		return target, fmt.Errorf("invalid provider")
	}
	return target, nil
}

func providerLogStorageIdentity(platform string, providerID string, provider string) string {
	normalizedPlatform := strings.TrimSpace(platform)
	normalizedProviderID := normalizeProviderLogStorageIdentity(providerID, provider)
	return normalizedPlatform + "\x00id:" + normalizedProviderID
}

func normalizeProviderLogStorageIdentity(providerID string, provider string) string {
	normalizedProviderID := strings.TrimSpace(providerID)
	if normalizedProviderID != "" {
		return normalizedProviderID
	}
	normalizedProvider := strings.TrimSpace(provider)
	if normalizedProvider != "" {
		return normalizedProvider
	}
	return providerLogStorageUnknownIdentity
}

func normalizeProviderLogStorageName(providerID string, provider string) string {
	normalizedProvider := strings.TrimSpace(provider)
	if normalizedProvider != "" {
		return normalizedProvider
	}
	normalizedProviderID := strings.TrimSpace(providerID)
	if normalizedProviderID != "" {
		return normalizedProviderID
	}
	return "unknown"
}

func mergeProviderLogStorageRow(entries map[string]*ProviderLogStorageStat, row providerLogStorageRow) *ProviderLogStorageStat {
	if entries == nil {
		return nil
	}
	key := providerLogStorageIdentity(row.Platform, row.ProviderID, row.Provider)
	entry, ok := entries[key]
	if !ok || entry == nil {
		entry = &ProviderLogStorageStat{
			Platform:   strings.TrimSpace(row.Platform),
			ProviderID: strings.TrimSpace(row.ProviderID),
			Provider:   normalizeProviderLogStorageName(row.ProviderID, row.Provider),
			LatestAt:   strings.TrimSpace(row.LatestAt),
		}
		entries[key] = entry
		return entry
	}
	if entry.ProviderID == "" && strings.TrimSpace(row.ProviderID) != "" {
		entry.ProviderID = strings.TrimSpace(row.ProviderID)
	}
	if (strings.TrimSpace(row.Provider) != "" && entry.Provider == "unknown") ||
		strings.TrimSpace(row.LatestAt) > strings.TrimSpace(entry.LatestAt) {
		entry.Provider = normalizeProviderLogStorageName(row.ProviderID, row.Provider)
	}
	if strings.TrimSpace(row.LatestAt) > strings.TrimSpace(entry.LatestAt) {
		entry.LatestAt = strings.TrimSpace(row.LatestAt)
	}
	return entry
}

func buildProviderLogStorageWhereClause(target deleteProviderLogsTarget) (string, []any) {
	matchName := normalizeProviderLogStorageName(target.ProviderID, target.Provider)
	return strings.Join([]string{
		"platform = ?",
		"(TRIM(COALESCE(provider_id, '')) = ? OR (TRIM(COALESCE(provider_id, '')) = '' AND TRIM(COALESCE(provider, '')) = ?))",
	}, " AND "), []any{target.Platform, target.ProviderID, matchName}
}

func buildProviderFailedRequestLogWhereClause(target deleteProviderLogsTarget) (string, []any) {
	baseWhereClause, baseArgs := buildProviderLogStorageWhereClause(target)
	whereClause := strings.Join([]string{
		baseWhereClause,
		requestLogFailureWhereClause(""),
	}, " AND ")
	return whereClause, baseArgs
}

func buildProviderUnreadFailedRequestLogWhereClause(target deleteProviderLogsTarget) (string, []any) {
	baseWhereClause, baseArgs := buildProviderFailedRequestLogWhereClause(target)
	whereClause := strings.Join([]string{
		baseWhereClause,
		requestLogUnreadWhereClause,
	}, " AND ")
	return whereClause, baseArgs
}

func deleteRequestLogStatsByProviderTx(tx *sql.Tx, table string, target deleteProviderLogsTarget) (int64, error) {
	if tx == nil {
		return 0, fmt.Errorf("nil tx")
	}
	if strings.TrimSpace(table) == "" {
		return 0, fmt.Errorf("empty table")
	}
	whereClause := strings.Join([]string{
		"platform = ?",
		"(TRIM(COALESCE(provider_id, '')) = ? OR (TRIM(COALESCE(provider_id, '')) = '' AND TRIM(COALESCE(provider, '')) = ?))",
	}, " AND ")
	args := []any{target.Platform, target.ProviderID, normalizeProviderLogStorageName(target.ProviderID, target.Provider)}
	execResult, err := tx.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE %s", table, whereClause),
		args...,
	)
	if err != nil && !isNoSuchTableErr(err) {
		return 0, err
	}
	if err != nil {
		return 0, nil
	}
	return rowsAffected(execResult), nil
}

func queryProviderRequestLogStorageRows(db *sql.DB) ([]providerLogStorageRow, error) {
	if db == nil {
		return nil, errors.New("nil db")
	}

	queryWithPayloadBytes := `
		SELECT
			TRIM(COALESCE(platform, '')) AS platform,
			CASE
				WHEN TRIM(COALESCE(provider_id, '')) <> '' THEN TRIM(provider_id)
				WHEN TRIM(COALESCE(provider, '')) <> '' THEN TRIM(provider)
				ELSE '` + providerLogStorageUnknownIdentity + `'
			END AS provider_id,
			MAX(
				CASE
					WHEN TRIM(COALESCE(provider, '')) <> '' THEN TRIM(provider)
					WHEN TRIM(COALESCE(provider_id, '')) <> '' THEN TRIM(provider_id)
					ELSE 'unknown'
				END
			) AS provider,
			COUNT(*) AS rows,
			COALESCE(SUM(
				COALESCE(LENGTH(CAST(platform AS BLOB)), 0) +
				COALESCE(LENGTH(CAST(model AS BLOB)), 0) +
				COALESCE(LENGTH(CAST(requested_model AS BLOB)), 0) +
				COALESCE(LENGTH(CAST(response_model AS BLOB)), 0) +
				COALESCE(LENGTH(CAST(provider_id AS BLOB)), 0) +
				COALESCE(LENGTH(CAST(provider AS BLOB)), 0) +
				COALESCE(LENGTH(CAST(price_source AS BLOB)), 0) +
				COALESCE(LENGTH(CAST(matched_pricing_model AS BLOB)), 0) +
				CASE
					WHEN COALESCE(payload_bytes, 0) > 0 THEN COALESCE(payload_bytes, 0)
					ELSE COALESCE(LENGTH(CAST(request_body AS BLOB)), 0) + COALESCE(LENGTH(CAST(response_body AS BLOB)), 0)
				END +
				176
			), 0) AS bytes,
			MAX(created_at) AS latest_at
		FROM request_log
		WHERE TRIM(COALESCE(provider_id, '')) <> '' OR TRIM(COALESCE(provider, '')) <> ''
		GROUP BY platform,
			CASE
				WHEN TRIM(COALESCE(provider_id, '')) <> '' THEN TRIM(provider_id)
				WHEN TRIM(COALESCE(provider, '')) <> '' THEN TRIM(provider)
				ELSE '` + providerLogStorageUnknownIdentity + `'
			END
	`
	queryLegacy := `
		SELECT
			TRIM(COALESCE(platform, '')) AS platform,
			CASE
				WHEN TRIM(COALESCE(provider_id, '')) <> '' THEN TRIM(provider_id)
				WHEN TRIM(COALESCE(provider, '')) <> '' THEN TRIM(provider)
				ELSE '` + providerLogStorageUnknownIdentity + `'
			END AS provider_id,
			MAX(
				CASE
					WHEN TRIM(COALESCE(provider, '')) <> '' THEN TRIM(provider)
					WHEN TRIM(COALESCE(provider_id, '')) <> '' THEN TRIM(provider_id)
					ELSE 'unknown'
				END
			) AS provider,
			COUNT(*) AS rows,
			COALESCE(SUM(
				COALESCE(LENGTH(CAST(platform AS BLOB)), 0) +
				COALESCE(LENGTH(CAST(model AS BLOB)), 0) +
				COALESCE(LENGTH(CAST(provider_id AS BLOB)), 0) +
				COALESCE(LENGTH(CAST(provider AS BLOB)), 0) +
				COALESCE(LENGTH(CAST(request_body AS BLOB)), 0) +
				COALESCE(LENGTH(CAST(response_body AS BLOB)), 0) +
				160
			), 0) AS bytes,
			MAX(created_at) AS latest_at
		FROM request_log
		WHERE TRIM(COALESCE(provider_id, '')) <> '' OR TRIM(COALESCE(provider, '')) <> ''
		GROUP BY platform,
			CASE
				WHEN TRIM(COALESCE(provider_id, '')) <> '' THEN TRIM(provider_id)
				WHEN TRIM(COALESCE(provider, '')) <> '' THEN TRIM(provider)
				ELSE '` + providerLogStorageUnknownIdentity + `'
			END
	`
	return queryProviderLogStorageRows(db, queryWithPayloadBytes, queryLegacy)
}

func queryProviderStatsStorageRows(db *sql.DB, table string) ([]providerLogStorageRow, error) {
	if db == nil {
		return nil, errors.New("nil db")
	}
	if strings.TrimSpace(table) == "" {
		return nil, fmt.Errorf("empty table")
	}

	query := fmt.Sprintf(`
		SELECT
			TRIM(COALESCE(platform, '')) AS platform,
			CASE
				WHEN TRIM(COALESCE(provider_id, '')) <> '' THEN TRIM(provider_id)
				WHEN TRIM(COALESCE(provider, '')) <> '' THEN TRIM(provider)
				ELSE '%s'
			END AS provider_id,
			MAX(
				CASE
					WHEN TRIM(COALESCE(provider, '')) <> '' THEN TRIM(provider)
					WHEN TRIM(COALESCE(provider_id, '')) <> '' THEN TRIM(provider_id)
					ELSE 'unknown'
				END
			) AS provider,
			COUNT(*) AS rows,
			COALESCE(SUM(
				COALESCE(LENGTH(CAST(bucket_start AS BLOB)), 0) +
				COALESCE(LENGTH(CAST(platform AS BLOB)), 0) +
				COALESCE(LENGTH(CAST(provider_id AS BLOB)), 0) +
				COALESCE(LENGTH(CAST(provider AS BLOB)), 0) +
				96
			), 0) AS bytes,
			MAX(bucket_start) AS latest_at
		FROM %s
		WHERE TRIM(COALESCE(provider_id, '')) <> '' OR TRIM(COALESCE(provider, '')) <> ''
		GROUP BY platform,
			CASE
				WHEN TRIM(COALESCE(provider_id, '')) <> '' THEN TRIM(provider_id)
				WHEN TRIM(COALESCE(provider, '')) <> '' THEN TRIM(provider)
				ELSE '%s'
			END
	`, providerLogStorageUnknownIdentity, table, providerLogStorageUnknownIdentity)
	return queryProviderLogStorageRows(db, query, "")
}

func queryProviderLogStorageRows(db *sql.DB, primaryQuery string, fallbackQuery string) ([]providerLogStorageRow, error) {
	rows, err := db.Query(primaryQuery)
	if err != nil && fallbackQuery != "" && strings.Contains(err.Error(), "no such column") {
		rows, err = db.Query(fallbackQuery)
	}
	if err != nil {
		if isNoSuchTableErr(err) {
			return []providerLogStorageRow{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	items := make([]providerLogStorageRow, 0, 64)
	for rows.Next() {
		var platform sql.NullString
		var providerID sql.NullString
		var provider sql.NullString
		var latestAt sql.NullString
		var row providerLogStorageRow
		if err := rows.Scan(&platform, &providerID, &provider, &row.Rows, &row.Bytes, &latestAt); err != nil {
			return nil, err
		}
		row.Platform = strings.TrimSpace(platform.String)
		row.ProviderID = strings.TrimSpace(providerID.String)
		row.Provider = normalizeProviderLogStorageName(row.ProviderID, provider.String)
		row.LatestAt = strings.TrimSpace(latestAt.String)
		if row.Platform == "" || row.ProviderID == "" || row.ProviderID == providerLogStorageUnknownIdentity {
			continue
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
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
