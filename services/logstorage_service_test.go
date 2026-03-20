package services

import (
	"database/sql"
	"testing"
	"time"

	"github.com/daodao97/xgo/xdb"
)

func TestDeleteRequestLogsByDate_RemovesRequestLogsAndStatsBuckets(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	targetDay := startOfDay(time.Now()).AddDate(0, 0, -2)
	otherDay := targetDay.AddDate(0, 0, 1)

	insertRequestLogForHeatmap(t, db, targetDay.Add(2*time.Hour).UTC().Format(timeLayout), 10, 20, 3, 1, 0.2)
	insertRequestLogForHeatmap(t, db, targetDay.Add(10*time.Hour).UTC().Format(timeLayout), 12, 24, 5, 2, 0.4)
	insertRequestLogForHeatmap(t, db, otherDay.Add(3*time.Hour).UTC().Format(timeLayout), 8, 16, 2, 1, 0.1)

	ls := NewLogService(nil)
	result, err := ls.DeleteRequestLogsByDate(targetDay.Format("2006-01-02"))
	if err != nil {
		t.Fatalf("DeleteRequestLogsByDate 调用失败: %v", err)
	}
	if result.DeletedRequestLogs != 2 {
		t.Fatalf("期望删除 2 条 request_log，实际 %d", result.DeletedRequestLogs)
	}
	if result.DeletedStatsHour != 2 {
		t.Fatalf("期望删除 2 条 hourly stats，实际 %d", result.DeletedStatsHour)
	}
	if result.DeletedStatsDay != 1 {
		t.Fatalf("期望删除 1 条 daily stats，实际 %d", result.DeletedStatsDay)
	}

	assertTableCount(t, db,
		"SELECT COUNT(*) FROM request_log WHERE created_at >= ? AND created_at < ?",
		0,
		targetDay.UTC().Format(timeLayout),
		targetDay.AddDate(0, 0, 1).UTC().Format(timeLayout),
	)
	assertTableCount(t, db,
		"SELECT COUNT(*) FROM request_log WHERE created_at >= ? AND created_at < ?",
		1,
		otherDay.UTC().Format(timeLayout),
		otherDay.AddDate(0, 0, 1).UTC().Format(timeLayout),
	)

	assertTableCount(t, db,
		"SELECT COUNT(*) FROM request_log_stats_hourly WHERE bucket_start >= ? AND bucket_start < ?",
		0,
		targetDay.Format(timeLayout),
		targetDay.AddDate(0, 0, 1).Format(timeLayout),
	)
	assertTableCount(t, db,
		"SELECT COUNT(*) FROM request_log_stats_daily WHERE bucket_start >= ? AND bucket_start < ?",
		0,
		targetDay.Format(timeLayout),
		targetDay.AddDate(0, 0, 1).Format(timeLayout),
	)
	assertTableCount(t, db,
		"SELECT COUNT(*) FROM request_log_stats_hourly WHERE bucket_start >= ? AND bucket_start < ?",
		1,
		otherDay.Format(timeLayout),
		otherDay.AddDate(0, 0, 1).Format(timeLayout),
	)
	assertTableCount(t, db,
		"SELECT COUNT(*) FROM request_log_stats_daily WHERE bucket_start >= ? AND bucket_start < ?",
		1,
		otherDay.Format(timeLayout),
		otherDay.AddDate(0, 0, 1).Format(timeLayout),
	)
}

func TestDeleteRequestLogsByDate_InvalidDate(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	ls := NewLogService(nil)
	if _, err := ls.DeleteRequestLogsByDate("2026-13-40"); err == nil {
		t.Fatalf("期望非法日期返回错误，实际为 nil")
	}
}

func TestRequestLogDailyHeatmapStats_UsesExistingRequestLogsOnly(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	targetDay := startOfDay(time.Now()).AddDate(0, 0, -1)
	insertRequestLogForHeatmap(t, db, targetDay.Add(2*time.Hour).UTC().Format(timeLayout), 10, 20, 3, 1, 0.2)
	insertRequestLogForHeatmap(t, db, targetDay.Add(10*time.Hour).UTC().Format(timeLayout), 12, 24, 5, 2, 0.4)

	ls := NewLogService(nil)
	statsBeforeClear, err := ls.RequestLogDailyHeatmapStats(30)
	if err != nil {
		t.Fatalf("RequestLogDailyHeatmapStats 调用失败: %v", err)
	}
	targetBeforeClear := findHeatmapStatByDay(statsBeforeClear, targetDay.Format("2006-01-02"))
	if targetBeforeClear == nil || targetBeforeClear.TotalRequests != 2 {
		t.Fatalf("期望清理前按天聚合为 2，实际 %+v", targetBeforeClear)
	}

	if err := ls.ClearRequestLogs(); err != nil {
		t.Fatalf("ClearRequestLogs 调用失败: %v", err)
	}

	statsAfterClear, err := ls.RequestLogDailyHeatmapStats(30)
	if err != nil {
		t.Fatalf("清理后 RequestLogDailyHeatmapStats 调用失败: %v", err)
	}
	if targetAfterClear := findHeatmapStatByDay(statsAfterClear, targetDay.Format("2006-01-02")); targetAfterClear != nil {
		t.Fatalf("期望清理明细后不再返回该日 request_log 热力墙数据，实际 %+v", targetAfterClear)
	}

	assertTableCount(t, db,
		"SELECT COUNT(*) FROM request_log_stats_daily WHERE bucket_start >= ? AND bucket_start < ?",
		1,
		targetDay.Format(timeLayout),
		targetDay.AddDate(0, 0, 1).Format(timeLayout),
	)
}

func TestRequestLogDailyHeatmapStats_IncludesPayloadBytes(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	targetDay := startOfDay(time.Now()).AddDate(0, 0, -1)
	_, err = db.Exec(`
		INSERT INTO request_log (
			platform,
			model,
			provider_id,
			provider,
			http_code,
			input_tokens,
			output_tokens,
			cache_create_tokens,
			cache_read_tokens,
			reasoning_tokens,
			total_cost,
			request_body,
			response_body,
			payload_bytes,
			payload_captured,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"codex",
		"gpt-5",
		"pid-1",
		"provider-1",
		200,
		10,
		20,
		0,
		3,
		1,
		0.2,
		"abcd",
		"123456",
		10,
		1,
		targetDay.Add(2*time.Hour).UTC().Format(timeLayout),
	)
	if err != nil {
		t.Fatalf("插入第一条 request_log 失败: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO request_log (
			platform,
			model,
			provider_id,
			provider,
			http_code,
			input_tokens,
			output_tokens,
			cache_create_tokens,
			cache_read_tokens,
			reasoning_tokens,
			total_cost,
			request_body,
			response_body,
			payload_bytes,
			payload_captured,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"codex",
		"gpt-5",
		"pid-1",
		"provider-1",
		200,
		12,
		24,
		0,
		5,
		2,
		0.4,
		"xy",
		"z",
		3,
		1,
		targetDay.Add(10*time.Hour).UTC().Format(timeLayout),
	)
	if err != nil {
		t.Fatalf("插入第二条 request_log 失败: %v", err)
	}

	ls := NewLogService(nil)
	stats, err := ls.RequestLogDailyHeatmapStats(30)
	if err != nil {
		t.Fatalf("RequestLogDailyHeatmapStats 调用失败: %v", err)
	}
	target := findHeatmapStatByDay(stats, targetDay.Format("2006-01-02"))
	if target == nil {
		t.Fatalf("期望返回目标日期热力墙数据，实际为 nil")
	}
	if target.TotalRequests != 2 {
		t.Fatalf("期望目标日期请求数为 2，实际 %d", target.TotalRequests)
	}
	if target.PayloadBytes != 13 {
		t.Fatalf("期望目标日期 payload bytes 为 13，实际 %d", target.PayloadBytes)
	}
	if target.PayloadCapturedRequests != 2 {
		t.Fatalf("期望目标日期 payload captured requests 为 2，实际 %d", target.PayloadCapturedRequests)
	}
}

func TestRequestLogDailyHeatmapStatsByYear_ReturnsSelectedYearOnly(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	year2024Day := time.Date(2024, time.January, 15, 12, 0, 0, 0, time.Local)
	year2025Day := time.Date(2025, time.June, 20, 12, 0, 0, 0, time.Local)
	insertRequestLogForHeatmap(t, db, year2024Day.UTC().Format(timeLayout), 8, 16, 2, 1, 0.1)
	insertRequestLogForHeatmap(t, db, year2025Day.UTC().Format(timeLayout), 10, 20, 3, 1, 0.2)

	ls := NewLogService(nil)
	stats, err := ls.RequestLogDailyHeatmapStatsByYear(2025)
	if err != nil {
		t.Fatalf("RequestLogDailyHeatmapStatsByYear 调用失败: %v", err)
	}

	if findHeatmapStatByDay(stats, "2024-01-15") != nil {
		t.Fatalf("期望 2025 年热力墙不包含 2024 年数据，实际 %+v", stats)
	}
	target := findHeatmapStatByDay(stats, "2025-06-20")
	if target == nil || target.TotalRequests != 1 {
		t.Fatalf("期望 2025-06-20 返回 1 条聚合，实际 %+v", target)
	}
}

func TestListRequestLogHeatmapYears_ReturnsDistinctYearsDescending(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForHeatmap(
		t,
		db,
		time.Date(2024, time.January, 15, 12, 0, 0, 0, time.Local).UTC().Format(timeLayout),
		8,
		16,
		2,
		1,
		0.1,
	)
	insertRequestLogForHeatmap(
		t,
		db,
		time.Date(2025, time.June, 20, 12, 0, 0, 0, time.Local).UTC().Format(timeLayout),
		10,
		20,
		3,
		1,
		0.2,
	)
	insertRequestLogForHeatmap(
		t,
		db,
		time.Date(2025, time.December, 31, 12, 0, 0, 0, time.Local).UTC().Format(timeLayout),
		12,
		24,
		5,
		2,
		0.4,
	)

	ls := NewLogService(nil)
	years, err := ls.ListRequestLogHeatmapYears()
	if err != nil {
		t.Fatalf("ListRequestLogHeatmapYears 调用失败: %v", err)
	}
	if len(years) != 2 {
		t.Fatalf("期望返回 2 个年份，实际 %v", years)
	}
	if years[0] != 2025 || years[1] != 2024 {
		t.Fatalf("期望按降序返回 [2025 2024]，实际 %v", years)
	}
}

func TestListProviderLogStorageStats_ReturnsProviderAggregates(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	baseDay := startOfDay(time.Now()).AddDate(0, 0, -1)
	insertProviderRequestLogForStorage(t, db, "codex", "pid-a", "provider-a", baseDay.Add(2*time.Hour), "req-a", "resp-a")
	insertProviderRequestLogForStorage(t, db, "codex", "pid-a", "provider-a", baseDay.Add(5*time.Hour), "req-aaaa", "resp-aaaa")
	insertProviderRequestLogForStorage(t, db, "claude", "pid-b", "provider-b", baseDay.Add(6*time.Hour), "req-b", "resp-b")

	ls := NewLogService(nil)
	stats, err := ls.ListProviderLogStorageStats()
	if err != nil {
		t.Fatalf("ListProviderLogStorageStats 调用失败: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("期望返回 2 个供应商聚合，实际 %d，详情 %+v", len(stats), stats)
	}

	var codexProvider *ProviderLogStorageStat
	for i := range stats {
		if stats[i].Platform == "codex" && stats[i].ProviderID == "pid-a" {
			codexProvider = &stats[i]
			break
		}
	}
	if codexProvider == nil {
		t.Fatalf("期望返回 codex / pid-a 的供应商存储统计，实际 %+v", stats)
	}
	if codexProvider.RequestLogRows != 2 {
		t.Fatalf("期望 codex / pid-a 请求明细为 2，实际 %d", codexProvider.RequestLogRows)
	}
	if codexProvider.StatsRows != 3 {
		t.Fatalf("期望 codex / pid-a 统计汇总为 3（2 hourly + 1 daily），实际 %d", codexProvider.StatsRows)
	}
	if codexProvider.TotalBytes <= 0 || codexProvider.RequestLogBytes <= 0 || codexProvider.StatsBytes <= 0 {
		t.Fatalf("期望 codex / pid-a 占用估算均大于 0，实际 %+v", codexProvider)
	}
}

func TestListProviderLogStorageStats_MergesLegacyProviderNameIdentity(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	baseDay := startOfDay(time.Now()).AddDate(0, 0, -1)
	insertProviderRequestLogForStorage(t, db, "codex", "", "legacy-provider", baseDay.Add(1*time.Hour), "req-legacy", "resp-legacy")
	insertProviderRequestLogForStorage(t, db, "codex", "", "legacy-provider", baseDay.Add(3*time.Hour), "req-legacy-2", "resp-legacy-2")

	ls := NewLogService(nil)
	stats, err := ls.ListProviderLogStorageStats()
	if err != nil {
		t.Fatalf("ListProviderLogStorageStats 调用失败: %v", err)
	}

	if len(stats) != 1 {
		t.Fatalf("期望 legacy 供应商只聚合成 1 条，实际 %d，详情 %+v", len(stats), stats)
	}
	if stats[0].ProviderID != "legacy-provider" {
		t.Fatalf("期望 legacy 供应商统一后的 provider_id 为 legacy-provider，实际 %+v", stats[0])
	}
	if stats[0].RequestLogRows != 2 || stats[0].StatsRows != 3 {
		t.Fatalf("期望 legacy 供应商明细 / 统计分别为 2 / 3，实际 %+v", stats[0])
	}
}

func TestClearProviderLogStorage_RemovesProviderRequestLogsAndStats(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	targetDay := startOfDay(time.Now()).AddDate(0, 0, -2)
	insertProviderRequestLogForStorage(t, db, "codex", "pid-a", "provider-a", targetDay.Add(2*time.Hour), "req-a", "resp-a")
	insertProviderRequestLogForStorage(t, db, "codex", "pid-a", "provider-a", targetDay.Add(4*time.Hour), "req-aa", "resp-aa")
	insertProviderRequestLogForStorage(t, db, "codex", "pid-b", "provider-b", targetDay.Add(6*time.Hour), "req-b", "resp-b")

	ls := NewLogService(nil)
	result, err := ls.ClearProviderLogStorage("codex", "pid-a", "provider-a")
	if err != nil {
		t.Fatalf("ClearProviderLogStorage 调用失败: %v", err)
	}
	if result.DeletedRequestLogs != 2 {
		t.Fatalf("期望删除 2 条 request_log，实际 %d", result.DeletedRequestLogs)
	}
	if result.DeletedStatsHour != 2 {
		t.Fatalf("期望删除 2 条 hourly stats，实际 %d", result.DeletedStatsHour)
	}
	if result.DeletedStatsDay != 1 {
		t.Fatalf("期望删除 1 条 daily stats，实际 %d", result.DeletedStatsDay)
	}

	assertTableCount(t, db,
		"SELECT COUNT(*) FROM request_log WHERE platform = ? AND provider_id = ?",
		0,
		"codex",
		"pid-a",
	)
	assertTableCount(t, db,
		"SELECT COUNT(*) FROM request_log WHERE platform = ? AND provider_id = ?",
		1,
		"codex",
		"pid-b",
	)
	assertTableCount(t, db,
		"SELECT COUNT(*) FROM request_log_stats_hourly WHERE platform = ? AND provider_id = ?",
		0,
		"codex",
		"pid-a",
	)
	assertTableCount(t, db,
		"SELECT COUNT(*) FROM request_log_stats_daily WHERE platform = ? AND provider_id = ?",
		0,
		"codex",
		"pid-a",
	)
	assertTableCount(t, db,
		"SELECT COUNT(*) FROM request_log_stats_daily WHERE platform = ? AND provider_id = ?",
		1,
		"codex",
		"pid-b",
	)
}

func TestClearProviderLogStorage_RemovesLegacyProviderStats(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	targetDay := startOfDay(time.Now()).AddDate(0, 0, -1)
	insertProviderRequestLogForStorage(t, db, "codex", "", "legacy-provider", targetDay.Add(2*time.Hour), "req-legacy", "resp-legacy")
	insertProviderRequestLogForStorage(t, db, "codex", "", "legacy-provider", targetDay.Add(4*time.Hour), "req-legacy-2", "resp-legacy-2")
	insertProviderRequestLogForStorage(t, db, "codex", "pid-b", "provider-b", targetDay.Add(6*time.Hour), "req-b", "resp-b")

	ls := NewLogService(nil)
	result, err := ls.ClearProviderLogStorage("codex", "", "legacy-provider")
	if err != nil {
		t.Fatalf("ClearProviderLogStorage 调用失败: %v", err)
	}
	if result.DeletedRequestLogs != 2 || result.DeletedStatsHour != 2 || result.DeletedStatsDay != 1 {
		t.Fatalf("期望 legacy 供应商删除 2 条明细、2 条 hourly、1 条 daily，实际 %+v", result)
	}

	assertTableCount(t, db,
		"SELECT COUNT(*) FROM request_log WHERE platform = ? AND COALESCE(provider_id, '') = '' AND provider = ?",
		0,
		"codex",
		"legacy-provider",
	)
	assertTableCount(t, db,
		"SELECT COUNT(*) FROM request_log_stats_hourly WHERE platform = ? AND provider_id = ?",
		0,
		"codex",
		"legacy-provider",
	)
	assertTableCount(t, db,
		"SELECT COUNT(*) FROM request_log_stats_daily WHERE platform = ? AND provider_id = ?",
		0,
		"codex",
		"legacy-provider",
	)
}

func insertProviderRequestLogForStorage(
	t *testing.T,
	db *sql.DB,
	platform string,
	providerID string,
	provider string,
	createdAt time.Time,
	requestBody string,
	responseBody string,
) {
	t.Helper()
	payloadBytes := len(requestBody) + len(responseBody)
	_, err := db.Exec(`
		INSERT INTO request_log (
			platform,
			model,
			provider_id,
			provider,
			http_code,
			input_tokens,
			output_tokens,
			cache_create_tokens,
			cache_read_tokens,
			reasoning_tokens,
			total_cost,
			request_body,
			response_body,
			payload_bytes,
			payload_captured,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		platform,
		"test-model",
		providerID,
		provider,
		500,
		10,
		20,
		0,
		3,
		1,
		0.2,
		requestBody,
		responseBody,
		payloadBytes,
		1,
		createdAt.UTC().Format(timeLayout),
	)
	if err != nil {
		t.Fatalf("插入供应商 request_log 失败: %v", err)
	}
}

func assertTableCount(t *testing.T, db *sql.DB, query string, expected int64, args ...any) {
	t.Helper()
	var count int64
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatalf("查询数量失败: %v", err)
	}
	if count != expected {
		t.Fatalf("数量断言失败，期望 %d，实际 %d，query=%s", expected, count, query)
	}
}
