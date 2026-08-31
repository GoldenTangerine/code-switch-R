/**
 * @name: 日志仪表盘聚合性能基线
 * @Descripttion: 验证日志聚合筛选与查询计划，并测量固定规模数据集上的仪表盘刷新成本
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-31 16:43:30
 * @LastEditTime: 2026-08-31 16:43:30
 * @FilePath: services/log_dashboard_benchmark_test.go
 */

package services

import (
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/daodao97/xgo/xdb"
	"github.com/daodao97/xgo/xlog"
)

var (
	logDashboardStatsSink         LogStats
	logDashboardSummarySink       LogSummary
	logDashboardProviderStatsSink []ProviderDailyStat
	logDashboardModelStatsSink    []ModelUsageStat
	logDashboardPageSink          RequestLogPageResult
	logDashboardProviderRefsSink  []LogProviderRef
)

var logDashboardRangeStart = time.Date(2026, time.August, 1, 0, 0, 0, 0, time.Local)

type logDashboardBenchmarkScenario struct {
	name         string
	platform     string
	provider     string
	pricingModel string
	sourceMode   LogDataSourceMode
}

func logDashboardBenchmarkScenarios() []logDashboardBenchmarkScenario {
	return []logDashboardBenchmarkScenario{
		{name: "Proxy", sourceMode: LogDataSourceModeProxy},
		{name: "SessionProvider", platform: "codex", provider: "provider-1", sourceMode: LogDataSourceModeSession},
		{name: "AllProviderModel", platform: "codex", provider: "provider-1", pricingModel: "gpt-5", sourceMode: LogDataSourceModeAll},
	}
}

func resetLogDashboardFixture(tb testing.TB, db *sql.DB) {
	tb.Helper()
	if _, err := db.Exec("DELETE FROM request_log"); err != nil {
		tb.Fatalf("清空日志仪表盘基准数据失败: %v", err)
	}
	for _, table := range []string{requestLogStatsHourlyTable, requestLogStatsDailyTable} {
		if _, err := db.Exec("DELETE FROM " + table); err != nil {
			tb.Fatalf("清空日志统计表 %s 失败: %v", table, err)
		}
	}
}

func prepareLogDashboardFixture(tb testing.TB, count int) *sql.DB {
	tb.Helper()
	db, err := xdb.DB("default")
	if err != nil {
		tb.Fatalf("获取日志仪表盘基准数据库失败: %v", err)
	}
	resetLogDashboardFixture(tb, db)

	tx, err := db.Begin()
	if err != nil {
		tb.Fatalf("开启日志仪表盘基准事务失败: %v", err)
	}
	stmt, err := tx.Prepare(`
		INSERT INTO request_log (
			platform, model, requested_model, response_model,
			provider_id, provider, http_code, request_outcome,
			input_tokens, output_tokens, cache_create_tokens, cache_read_tokens, reasoning_tokens,
			is_stream, duration_sec, first_token_sec,
			input_cost, output_cost, cache_create_cost, cache_read_cost, total_cost,
			price_source, has_pricing, matched_pricing_model,
			error_read_at, data_source, source_record_id, session_id, dedup_core, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		tb.Fatalf("准备日志仪表盘基准写入失败: %v", err)
	}

	platforms := []string{"codex", "claude", "gemini"}
	providerIDs := []string{"provider-1", "provider-2", "provider-3"}
	providerNames := []string{"Provider One", "Provider Two", "Provider Three"}
	models := []string{"gpt-5", "claude-sonnet-4"}
	sources := []string{requestLogDataSourceProxy, requestLogDataSourceClaudeSession, "", requestLogDataSourceCodexSession}
	for index := 0; index < count; index++ {
		platformIndex := index % len(platforms)
		providerIndex := (index / len(platforms)) % len(providerIDs)
		modelIndex := (index / (len(platforms) * len(providerIDs))) % len(models)
		sourceIndex := (index / (len(platforms) * len(providerIDs) * len(models))) % len(sources)

		createdAt := logDashboardRangeStart
		if index%5 == 0 {
			createdAt = createdAt.Add(-24 * time.Hour)
		}
		createdAt = createdAt.Add(time.Duration((index*7919)%int((24*time.Hour)/time.Second)) * time.Second)

		requestOutcome := requestOutcomeSuccess
		httpCode := 200
		switch (index / 72) % 4 {
		case 1:
			requestOutcome = requestOutcomeFailure
			httpCode = 500
		case 2:
			requestOutcome = requestOutcomeExcluded
		case 3:
			requestOutcome = ""
			if index%2 == 0 {
				httpCode = 503
			}
		}

		matchedPricingModel := models[modelIndex]
		if index%7 == 0 {
			matchedPricingModel = ""
		}
		errorReadAt := ""
		if requestOutcome == requestOutcomeFailure && index%2 == 0 {
			errorReadAt = createdAt.Add(time.Minute).UTC().Format(timeLayout)
		}

		if _, err := stmt.Exec(
			platforms[platformIndex], models[modelIndex], models[modelIndex], models[modelIndex],
			providerIDs[providerIndex], providerNames[providerIndex], httpCode, requestOutcome,
			100+index%17, 40+index%11, index%13, index%19, index%7,
			1, 0.5+float64(index%10)/10, 0.05+float64(index%5)/100,
			0.001, 0.002, 0.0003, 0.0002, 0.0035,
			requestLogPriceSourceBuiltin, 1, matchedPricingModel,
			errorReadAt, sources[sourceIndex], fmt.Sprintf("dashboard-source-%d", index), fmt.Sprintf("dashboard-session-%d", index%41), fmt.Sprintf("dashboard-dedup-%d", index), createdAt.UTC().Format(timeLayout),
		); err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			tb.Fatalf("写入第 %d 条日志仪表盘基准数据失败: %v", index, err)
		}
	}
	if err := stmt.Close(); err != nil {
		_ = tx.Rollback()
		tb.Fatalf("关闭日志仪表盘基准语句失败: %v", err)
	}
	if err := tx.Commit(); err != nil {
		tb.Fatalf("提交日志仪表盘基准数据失败: %v", err)
	}
	tb.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM request_log")
		_, _ = db.Exec("DELETE FROM " + requestLogStatsHourlyTable)
		_, _ = db.Exec("DELETE FROM " + requestLogStatsDailyTable)
	})
	return db
}

func logDashboardRangeStrings() (string, string) {
	return logDashboardRangeStart.Format(timeLayout), logDashboardRangeStart.Add(24 * time.Hour).Format(timeLayout)
}

func buildLogDashboardCountQuery(scenario logDashboardBenchmarkScenario, start time.Time, end time.Time) (string, []interface{}) {
	query := `SELECT COUNT(*) FROM request_log WHERE created_at >= ? AND created_at < ? AND ` + requestLogSourceWhereClause(scenario.sourceMode, "request_log")
	args := []interface{}{start.UTC().Format(timeLayout), end.UTC().Format(timeLayout)}
	if strings.TrimSpace(scenario.platform) != "" {
		query += " AND platform = ?"
		args = append(args, strings.TrimSpace(scenario.platform))
	}
	if strings.TrimSpace(scenario.provider) != "" {
		query += " AND provider_id = ?"
		args = append(args, strings.TrimSpace(scenario.provider))
	}
	if strings.TrimSpace(scenario.pricingModel) != "" {
		query += " AND COALESCE(NULLIF(TRIM(matched_pricing_model), ''), TRIM(model)) = ?"
		args = append(args, strings.TrimSpace(scenario.pricingModel))
	}
	return query, args
}

func countLogDashboardFixtureRows(tb testing.TB, db *sql.DB, scenario logDashboardBenchmarkScenario, start time.Time, end time.Time) int64 {
	tb.Helper()
	query, args := buildLogDashboardCountQuery(scenario, start, end)
	var count int64
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		tb.Fatalf("统计日志仪表盘基准匹配行失败: %v", err)
	}
	return count
}

func sumProviderDashboardRequests(stats []ProviderDailyStat) int64 {
	var total int64
	for _, stat := range stats {
		total += stat.TotalRequests
	}
	return total
}

func sumModelDashboardRequests(stats []ModelUsageStat) int64 {
	var total int64
	for _, stat := range stats {
		total += stat.TotalRequests
	}
	return total
}

func TestLogDashboardAggregationFixtureCoversSourceProviderAndModelFilters(t *testing.T) {
	useIsolatedHomeDir(t)
	db := prepareLogDashboardFixture(t, 1440)
	startAt, endAt := logDashboardRangeStrings()
	service := NewLogService(nil)

	for _, scenario := range logDashboardBenchmarkScenarios() {
		t.Run(scenario.name, func(t *testing.T) {
			want := countLogDashboardFixtureRows(t, db, scenario, logDashboardRangeStart, logDashboardRangeStart.Add(24*time.Hour))
			if want == 0 {
				t.Fatal("基准场景未匹配任何当前范围日志")
			}

			stats, err := service.StatsRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt)
			if err != nil {
				t.Fatalf("StatsRangeV3 调用失败: %v", err)
			}
			summary, err := service.SummaryRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt)
			if err != nil {
				t.Fatalf("SummaryRangeV3 调用失败: %v", err)
			}
			providerStats, err := service.ProviderStatsRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt)
			if err != nil {
				t.Fatalf("ProviderStatsRangeV3 调用失败: %v", err)
			}
			modelStats, err := service.ModelStatsRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt)
			if err != nil {
				t.Fatalf("ModelStatsRangeV3 调用失败: %v", err)
			}

			if stats.TotalRequests != want || summary.TotalRequests != want || sumProviderDashboardRequests(providerStats) != want || sumModelDashboardRequests(modelStats) != want {
				t.Fatalf("四类聚合的筛选结果不一致: want=%d stats=%d summary=%d providers=%d models=%d", want, stats.TotalRequests, summary.TotalRequests, sumProviderDashboardRequests(providerStats), sumModelDashboardRequests(modelStats))
			}
			if !summary.ComparisonAvailable || summary.PreviousCostTotal <= 0 {
				t.Fatalf("摘要未覆盖历史比较区间: %#v", summary)
			}
		})
	}
}

func TestDashboardAggregateRangeV1MatchesLegacyAggregates(t *testing.T) {
	useIsolatedHomeDir(t)
	prepareLogDashboardFixture(t, 1440)
	startAt, endAt := logDashboardRangeStrings()

	for _, scenario := range logDashboardBenchmarkScenarios() {
		t.Run(scenario.name, func(t *testing.T) {
			legacyService := NewLogService(nil)
			stats, err := legacyService.StatsRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt)
			if err != nil {
				t.Fatalf("读取旧 Stats 失败: %v", err)
			}
			summary, err := legacyService.SummaryRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt)
			if err != nil {
				t.Fatalf("读取旧 Summary 失败: %v", err)
			}
			providerStats, err := legacyService.ProviderStatsRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt)
			if err != nil {
				t.Fatalf("读取旧 ProviderStats 失败: %v", err)
			}
			modelStats, err := legacyService.ModelStatsRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt)
			if err != nil {
				t.Fatalf("读取旧 ModelStats 失败: %v", err)
			}

			aggregate, err := NewLogService(nil).DashboardAggregateRangeV1(
				scenario.platform,
				scenario.provider,
				scenario.pricingModel,
				string(scenario.sourceMode),
				startAt,
				endAt,
				true,
			)
			if err != nil {
				t.Fatalf("读取新聚合结果失败: %v", err)
			}
			if !reflect.DeepEqual(aggregate.Stats, stats) {
				t.Fatalf("Stats 不等价:\nnew=%#v\nold=%#v", aggregate.Stats, stats)
			}
			if !reflect.DeepEqual(aggregate.Summary, summary) {
				t.Fatalf("Summary 不等价:\nnew=%#v\nold=%#v", aggregate.Summary, summary)
			}
			if !reflect.DeepEqual(aggregate.ProviderStats, providerStats) {
				t.Fatalf("ProviderStats 不等价:\nnew=%#v\nold=%#v", aggregate.ProviderStats, providerStats)
			}
			if !reflect.DeepEqual(aggregate.ModelStats, modelStats) {
				t.Fatalf("ModelStats 不等价:\nnew=%#v\nold=%#v", aggregate.ModelStats, modelStats)
			}

			withoutProviders, err := NewLogService(nil).DashboardAggregateRangeV1(
				scenario.platform,
				scenario.provider,
				scenario.pricingModel,
				string(scenario.sourceMode),
				startAt,
				endAt,
				false,
			)
			if err != nil {
				t.Fatalf("跳过 Provider 聚合失败: %v", err)
			}
			if len(withoutProviders.ProviderStats) != 0 {
				t.Fatalf("跳过 Provider 聚合时仍返回数据: %#v", withoutProviders.ProviderStats)
			}
			if !reflect.DeepEqual(withoutProviders.Stats, stats) ||
				!reflect.DeepEqual(withoutProviders.Summary, summary) ||
				!reflect.DeepEqual(withoutProviders.ModelStats, modelStats) {
				t.Fatal("跳过 Provider 聚合不应改变其他统计结果")
			}
		})
	}
}

func explainLogDashboardQueryPlan(t *testing.T, db *sql.DB, scenario logDashboardBenchmarkScenario) string {
	t.Helper()
	query, args := buildLogDashboardCountQuery(scenario, logDashboardRangeStart, logDashboardRangeStart.Add(24*time.Hour))
	rows, err := db.Query("EXPLAIN QUERY PLAN "+strings.Replace(query, "SELECT COUNT(*)", "SELECT platform, model, total_cost", 1), args...)
	if err != nil {
		t.Fatalf("读取日志仪表盘查询计划失败: %v", err)
	}
	defer rows.Close()

	details := make([]string, 0, 4)
	for rows.Next() {
		var id int
		var parent int
		var notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("解析日志仪表盘查询计划失败: %v", err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历日志仪表盘查询计划失败: %v", err)
	}
	return strings.Join(details, " | ")
}

func TestLogDashboardAggregationQueryPlansUseExistingRangeIndexes(t *testing.T) {
	useIsolatedHomeDir(t)
	db := prepareLogDashboardFixture(t, 1440)
	testCases := []struct {
		scenario      logDashboardBenchmarkScenario
		expectedIndex string
	}{
		{scenario: logDashboardBenchmarkScenario{name: "AllRange", sourceMode: LogDataSourceModeAll}, expectedIndex: "idx_request_log_created_at"},
		{scenario: logDashboardBenchmarkScenario{name: "ProxyRange", sourceMode: LogDataSourceModeProxy}, expectedIndex: "idx_request_log_created_at"},
		{scenario: logDashboardBenchmarkScenario{name: "SessionPlatform", platform: "codex", sourceMode: LogDataSourceModeSession}, expectedIndex: "idx_request_log_platform_created_at"},
		{scenario: logDashboardBenchmarkScenario{name: "AllProviderModel", platform: "codex", provider: "provider-1", pricingModel: "gpt-5", sourceMode: LogDataSourceModeAll}, expectedIndex: "idx_request_log_platform_provider_id_created_at"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.scenario.name, func(t *testing.T) {
			plan := explainLogDashboardQueryPlan(t, db, testCase.scenario)
			t.Logf("query plan: %s", plan)
			if !strings.Contains(plan, testCase.expectedIndex) {
				t.Fatalf("查询计划未使用预期范围索引 %s: %s", testCase.expectedIndex, plan)
			}
			if strings.Contains(plan, "SCAN request_log") {
				t.Fatalf("查询计划退化为 request_log 全表扫描: %s", plan)
			}
		})
	}
}

func benchmarkLogDashboardOperation(b *testing.B, queries float64, fullRows int64, run func() error) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	b.ReportMetric(queries, "queries/op")
	b.ReportMetric(float64(fullRows), "fullrows/op")
	for iteration := 0; iteration < b.N; iteration++ {
		if err := run(); err != nil {
			b.Fatal(err)
		}
	}
}

func executeLogDashboardRefresh(service *LogService, scenario logDashboardBenchmarkScenario, startAt string, endAt string) error {
	var err error
	logDashboardPageSink, err = service.ListRequestLogsPageV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), 15, 0, startAt, endAt)
	if err != nil {
		return err
	}
	logDashboardSummarySink, err = service.SummaryRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt)
	if err != nil {
		return err
	}
	logDashboardStatsSink, err = service.StatsRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt)
	if err != nil {
		return err
	}
	logDashboardModelStatsSink, err = service.ModelStatsRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt)
	if err != nil {
		return err
	}
	logDashboardProviderStatsSink, err = service.ProviderStatsRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt)
	if err != nil {
		return err
	}
	if scenario.pricingModel != "" {
		logDashboardModelStatsSink, err = service.ModelStatsRangeV3(scenario.platform, scenario.provider, "", string(scenario.sourceMode), startAt, endAt)
		if err != nil {
			return err
		}
	}
	logDashboardProviderRefsSink, err = service.ListProviderRefsV2(scenario.platform, string(scenario.sourceMode))
	return err
}

func executeAggregatedLogDashboardRefresh(service *LogService, scenario logDashboardBenchmarkScenario, startAt string, endAt string) error {
	var err error
	logDashboardPageSink, err = service.ListRequestLogsPageV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), 15, 0, startAt, endAt)
	if err != nil {
		return err
	}
	aggregate, err := service.DashboardAggregateRangeV1(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt, true)
	if err != nil {
		return err
	}
	logDashboardSummarySink = aggregate.Summary
	logDashboardStatsSink = aggregate.Stats
	logDashboardModelStatsSink = aggregate.ModelStats
	logDashboardProviderStatsSink = aggregate.ProviderStats
	if scenario.pricingModel != "" {
		logDashboardModelStatsSink, err = service.ModelStatsRangeV3(scenario.platform, scenario.provider, "", string(scenario.sourceMode), startAt, endAt)
		if err != nil {
			return err
		}
	}
	logDashboardProviderRefsSink, err = service.ListProviderRefsV2(scenario.platform, string(scenario.sourceMode))
	return err
}

func BenchmarkLogDashboardAggregation(b *testing.B) {
	originalLogger := xlog.GetLogger()
	xlog.SetLogger(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})))
	b.Cleanup(func() {
		xlog.SetLogger(originalLogger)
	})

	sizes := []struct {
		name  string
		count int
	}{
		{name: "1k", count: 1_000},
		{name: "10k", count: 10_000},
		{name: "100k", count: 100_000},
	}
	startAt, endAt := logDashboardRangeStrings()

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			b.StopTimer()
			db := prepareLogDashboardFixture(b, size.count)
			for _, scenario := range logDashboardBenchmarkScenarios() {
				b.Run(scenario.name, func(b *testing.B) {
					currentRows := countLogDashboardFixtureRows(b, db, scenario, logDashboardRangeStart, logDashboardRangeStart.Add(24*time.Hour))
					previousRows := countLogDashboardFixtureRows(b, db, scenario, logDashboardRangeStart.Add(-24*time.Hour), logDashboardRangeStart)
					service := NewLogService(nil)

					b.Run("Stats", func(b *testing.B) {
						benchmarkLogDashboardOperation(b, 1, currentRows, func() error {
							var err error
							logDashboardStatsSink, err = service.StatsRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt)
							return err
						})
					})
					b.Run("Summary", func(b *testing.B) {
						benchmarkLogDashboardOperation(b, 2, currentRows+previousRows, func() error {
							var err error
							logDashboardSummarySink, err = service.SummaryRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt)
							return err
						})
					})
					b.Run("ProviderCold", func(b *testing.B) {
						benchmarkLogDashboardOperation(b, 2, currentRows, func() error {
							coldService := NewLogService(nil)
							var err error
							logDashboardProviderStatsSink, err = coldService.ProviderStatsRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt)
							return err
						})
					})
					b.Run("ProviderWarm", func(b *testing.B) {
						warmService := NewLogService(nil)
						if _, err := warmService.ProviderStatsRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt); err != nil {
							b.Fatal(err)
						}
						benchmarkLogDashboardOperation(b, 1, currentRows, func() error {
							var err error
							logDashboardProviderStatsSink, err = warmService.ProviderStatsRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt)
							return err
						})
					})
					b.Run("Model", func(b *testing.B) {
						benchmarkLogDashboardOperation(b, 1, currentRows, func() error {
							var err error
							logDashboardModelStatsSink, err = service.ModelStatsRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt)
							return err
						})
					})

					dashboardCalls := 6.0
					dashboardQueriesCold := 9.0
					extraModelRows := int64(0)
					if scenario.pricingModel != "" {
						dashboardCalls++
						dashboardQueriesCold++
						modelOptionsScenario := scenario
						modelOptionsScenario.pricingModel = ""
						extraModelRows = countLogDashboardFixtureRows(b, db, modelOptionsScenario, logDashboardRangeStart, logDashboardRangeStart.Add(24*time.Hour))
					}
					listRows := currentRows
					if listRows > 15 {
						listRows = 15
					}
					dashboardFullRows := 4*currentRows + previousRows + extraModelRows + listRows

					b.Run("DashboardCold", func(b *testing.B) {
						benchmarkLogDashboardOperation(b, dashboardQueriesCold, dashboardFullRows, func() error {
							return executeLogDashboardRefresh(NewLogService(nil), scenario, startAt, endAt)
						})
						b.ReportMetric(dashboardCalls, "calls/op")
					})
					b.Run("DashboardWarm", func(b *testing.B) {
						warmService := NewLogService(nil)
						if _, err := warmService.ProviderStatsRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt); err != nil {
							b.Fatal(err)
						}
						benchmarkLogDashboardOperation(b, dashboardQueriesCold-1, dashboardFullRows, func() error {
							return executeLogDashboardRefresh(warmService, scenario, startAt, endAt)
						})
						b.ReportMetric(dashboardCalls, "calls/op")
					})

					aggregatedCalls := 3.0
					aggregatedQueriesCold := 6.0
					if scenario.pricingModel != "" {
						aggregatedCalls++
						aggregatedQueriesCold++
					}
					aggregatedFullRows := currentRows + previousRows + extraModelRows + listRows
					b.Run("DashboardAggregateCold", func(b *testing.B) {
						benchmarkLogDashboardOperation(b, aggregatedQueriesCold, aggregatedFullRows, func() error {
							return executeAggregatedLogDashboardRefresh(NewLogService(nil), scenario, startAt, endAt)
						})
						b.ReportMetric(aggregatedCalls, "calls/op")
					})
					b.Run("DashboardAggregateWarm", func(b *testing.B) {
						warmService := NewLogService(nil)
						if _, err := warmService.ProviderStatsRangeV3(scenario.platform, scenario.provider, scenario.pricingModel, string(scenario.sourceMode), startAt, endAt); err != nil {
							b.Fatal(err)
						}
						benchmarkLogDashboardOperation(b, aggregatedQueriesCold-1, aggregatedFullRows, func() error {
							return executeAggregatedLogDashboardRefresh(warmService, scenario, startAt, endAt)
						})
						b.ReportMetric(aggregatedCalls, "calls/op")
					})
				})
			}
		})
	}
}
