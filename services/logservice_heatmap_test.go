package services

import (
	"database/sql"
	"testing"
	"time"

	"github.com/daodao97/xgo/xdb"
)

func TestHeatmapStats_AggregatesFromHourlyStatsTable(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	nowHour := startOfHour(time.Now())
	nowBucket := nowHour.Format(timeLayout)
	prevBucket := nowHour.Add(-time.Hour).Format(timeLayout)

	insertHeatmapStatsBucketRow(t, db, nowBucket, "pid-a", 3, 30, 40, 5, 1.5)
	insertHeatmapStatsBucketRow(t, db, nowBucket, "pid-b", 2, 12, 18, 2, 0.5)
	insertHeatmapStatsBucketRow(t, db, prevBucket, "pid-c", 7, 70, 80, 9, 2.0)

	ls := NewLogService(nil)
	stats, err := ls.HeatmapStats(1)
	if err != nil {
		t.Fatalf("HeatmapStats 调用失败: %v", err)
	}
	if len(stats) == 0 {
		t.Fatalf("期望返回热力墙数据，实际为空")
	}

	currentLabel := nowHour.Format("2006-01-02 15")
	current := findHeatmapStatByDay(stats, currentLabel)
	if current == nil {
		t.Fatalf("未找到当前小时桶: %s", currentLabel)
	}
	if current.TotalRequests != 5 {
		t.Fatalf("期望当前小时请求数为 5，实际 %d", current.TotalRequests)
	}
	if current.InputTokens != 42 {
		t.Fatalf("期望当前小时 input_tokens 为 42，实际 %d", current.InputTokens)
	}
	if current.OutputTokens != 58 {
		t.Fatalf("期望当前小时 output_tokens 为 58，实际 %d", current.OutputTokens)
	}
	if current.ReasoningTokens != 7 {
		t.Fatalf("期望当前小时 reasoning_tokens 为 7，实际 %d", current.ReasoningTokens)
	}
	if !almostEqualFloat(current.TotalCost, 2.0) {
		t.Fatalf("期望当前小时 total_cost 为 2.0，实际 %f", current.TotalCost)
	}
}

func TestHeatmapStats_FallsBackToRequestLogWhenHourlyStatsTableMissing(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	if _, err := db.Exec(`DROP TRIGGER IF EXISTS request_log_stats_hourly_ai`); err != nil {
		t.Fatalf("删除 hourly trigger 失败: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS request_log_stats_hourly`); err != nil {
		t.Fatalf("删除 hourly stats table 失败: %v", err)
	}

	createdAtUTC := time.Now().UTC().Format(timeLayout)
	insertRequestLogForHeatmap(t, db, createdAtUTC, 13, 21, 3, 0.9)

	parsedUTC, parseErr := time.Parse(timeLayout, createdAtUTC)
	if parseErr != nil {
		t.Fatalf("解析 createdAtUTC 失败: %v", parseErr)
	}
	expectedDay := parsedUTC.In(time.Local).Format("2006-01-02 15")

	ls := NewLogService(nil)
	stats, err := ls.HeatmapStats(1)
	if err != nil {
		t.Fatalf("HeatmapStats 调用失败: %v", err)
	}
	target := findHeatmapStatByDay(stats, expectedDay)
	if target == nil {
		t.Fatalf("fallback 未返回预期小时桶: %s", expectedDay)
	}
	if target.TotalRequests != 1 {
		t.Fatalf("期望 fallback 请求数为 1，实际 %d", target.TotalRequests)
	}
	if target.InputTokens != 13 || target.OutputTokens != 21 || target.ReasoningTokens != 3 {
		t.Fatalf("fallback token 聚合异常: input=%d output=%d reasoning=%d", target.InputTokens, target.OutputTokens, target.ReasoningTokens)
	}
	if !almostEqualFloat(target.TotalCost, 0.9) {
		t.Fatalf("fallback total_cost 异常，期望 0.9，实际 %f", target.TotalCost)
	}
}

func insertHeatmapStatsBucketRow(
	t *testing.T,
	db *sql.DB,
	bucketStart string,
	providerID string,
	requests int64,
	input int64,
	output int64,
	reasoning int64,
	totalCost float64,
) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO request_log_stats_hourly (
			bucket_start,
			platform,
			provider_id,
			provider,
			total_requests,
			successful_requests,
			failed_requests,
			input_tokens,
			output_tokens,
			reasoning_tokens,
			cache_create_tokens,
			cache_read_tokens,
			total_cost
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		bucketStart,
		"codex",
		providerID,
		providerID,
		requests,
		requests,
		int64(0),
		input,
		output,
		reasoning,
		int64(0),
		int64(0),
		totalCost,
	)
	if err != nil {
		t.Fatalf("插入 request_log_stats_hourly 失败: %v", err)
	}
}

func insertRequestLogForHeatmap(
	t *testing.T,
	db *sql.DB,
	createdAt string,
	inputTokens int,
	outputTokens int,
	reasoningTokens int,
	totalCost float64,
) {
	t.Helper()
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
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"codex",
		"gpt-5",
		"pid-1",
		"provider-1",
		200,
		inputTokens,
		outputTokens,
		0,
		0,
		reasoningTokens,
		totalCost,
		createdAt,
	)
	if err != nil {
		t.Fatalf("插入 request_log 失败: %v", err)
	}
}

func findHeatmapStatByDay(stats []HeatmapStat, day string) *HeatmapStat {
	for i := range stats {
		if stats[i].Day == day {
			return &stats[i]
		}
	}
	return nil
}
