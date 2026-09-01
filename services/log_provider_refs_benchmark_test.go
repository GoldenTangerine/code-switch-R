/**
 * @name: 日志 ProviderRefs 查询性能基线
 * @Descripttion: 测量 ProviderRefs 在不同日志规模、来源、平台和 Provider 基数下的查询成本与计划
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-09-01 02:30:19
 * @LastEditTime: 2026-09-01 02:30:19
 * @FilePath: services/log_provider_refs_benchmark_test.go
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

	"github.com/daodao97/xgo/xlog"
)

type logProviderRefsBenchmarkScenario struct {
	name       string
	platform   string
	sourceMode LogDataSourceMode
}

var logProviderRefsBenchmarkSink []LogProviderRef

func logProviderRefsBenchmarkScenarios() []logProviderRefsBenchmarkScenario {
	return []logProviderRefsBenchmarkScenario{
		{name: "ProxyAllPlatforms", sourceMode: LogDataSourceModeProxy},
		{name: "ProxyCodex", platform: "codex", sourceMode: LogDataSourceModeProxy},
		{name: "SessionAllPlatforms", sourceMode: LogDataSourceModeSession},
		{name: "SessionCodex", platform: "codex", sourceMode: LogDataSourceModeSession},
		{name: "AllPlatforms", sourceMode: LogDataSourceModeAll},
		{name: "AllCodex", platform: "codex", sourceMode: LogDataSourceModeAll},
	}
}

func buildLogProviderRefsBenchmarkQuery(scenario logProviderRefsBenchmarkScenario) (string, []interface{}) {
	query := `
		SELECT provider_id, provider, MAX(created_at) AS latest_at
		FROM request_log
		WHERE TRIM(COALESCE(provider, '')) <> ''
	`
	query += " AND " + requestLogSourceWhereClause(scenario.sourceMode, "request_log")
	args := make([]interface{}, 0, 1)
	if strings.TrimSpace(scenario.platform) != "" {
		query += " AND platform = ?"
		args = append(args, strings.TrimSpace(scenario.platform))
	}
	query += " GROUP BY provider_id, provider"
	return query, args
}

func countLogProviderRefsBenchmarkRows(tb testing.TB, db *sql.DB, scenario logProviderRefsBenchmarkScenario) (int64, int64) {
	tb.Helper()
	query, args := buildLogProviderRefsBenchmarkQuery(scenario)
	whereIndex := strings.Index(query, "WHERE")
	groupIndex := strings.LastIndex(query, "GROUP BY")
	if whereIndex < 0 || groupIndex < 0 {
		tb.Fatal("ProviderRefs 基准查询结构异常")
	}
	whereClause := strings.TrimSpace(query[whereIndex:groupIndex])
	var matchedRows int64
	if err := db.QueryRow("SELECT COUNT(*) FROM request_log "+whereClause, args...).Scan(&matchedRows); err != nil {
		tb.Fatal(err)
	}
	var candidateRows int64
	if err := db.QueryRow("SELECT COUNT(*) FROM ("+query+")", args...).Scan(&candidateRows); err != nil {
		tb.Fatal(err)
	}
	return matchedRows, candidateRows
}

func explainLogProviderRefsBenchmarkQuery(tb testing.TB, db *sql.DB, scenario logProviderRefsBenchmarkScenario) string {
	tb.Helper()
	query, args := buildLogProviderRefsBenchmarkQuery(scenario)
	if strings.TrimSpace(scenario.platform) != "" {
		query = strings.Replace(query, "FROM request_log", "FROM request_log INDEXED BY "+requestLogProviderRefsIndexName, 1)
	}
	return explainLogProviderRefsBenchmarkSQL(tb, db, query, args)
}

func explainLogProviderRefsBenchmarkSQL(tb testing.TB, db *sql.DB, query string, args []interface{}) string {
	tb.Helper()
	rows, err := db.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		tb.Fatal(err)
	}
	defer rows.Close()
	details := make([]string, 0, 4)
	for rows.Next() {
		var id int
		var parent int
		var notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			tb.Fatal(err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		tb.Fatal(err)
	}
	return strings.Join(details, " | ")
}

func listLogProviderRefsBenchmarkWithIndex(
	db *sql.DB,
	scenario logProviderRefsBenchmarkScenario,
	indexName string,
) ([]LogProviderRef, error) {
	query, args := buildLogProviderRefsBenchmarkQuery(scenario)
	if strings.TrimSpace(indexName) != "" {
		query = strings.Replace(query, "FROM request_log", "FROM request_log INDEXED BY "+indexName, 1)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	candidates := make([]logProviderRefCandidate, 0, 64)
	for rows.Next() {
		var providerID sql.NullString
		var providerName sql.NullString
		var latestAt sql.NullString
		if err := rows.Scan(&providerID, &providerName, &latestAt); err != nil {
			return nil, err
		}
		candidates = append(candidates, logProviderRefCandidate{
			ProviderID: strings.TrimSpace(providerID.String),
			Provider:   strings.TrimSpace(providerName.String),
			LatestAt:   strings.TrimSpace(latestAt.String),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return mergeProviderRefsFromCandidates(candidates), nil
}

func applyHighCardinalityLogProviderRefsFixture(tb testing.TB, db *sql.DB) {
	tb.Helper()
	if _, err := db.Exec(`
		UPDATE request_log
		SET provider_id = printf('provider-%03d', ((id - 1) % 64) + 1),
			provider = CASE
				WHEN id % 11 = 0 THEN printf('Provider %03d Renamed', ((id - 1) % 64) + 1)
				ELSE printf('Provider %03d', ((id - 1) % 64) + 1)
			END
	`); err != nil {
		tb.Fatal(err)
	}
}

func TestLogProviderRefsQueryPlans(t *testing.T) {
	useIsolatedHomeDir(t)
	db := prepareLogDashboardFixture(t, 1440)
	for _, scenario := range logProviderRefsBenchmarkScenarios() {
		t.Run(scenario.name, func(t *testing.T) {
			plan := explainLogProviderRefsBenchmarkQuery(t, db, scenario)
			t.Logf("query plan: %s", plan)
			if !strings.Contains(plan, requestLogProviderRefsIndexName) {
				t.Fatalf("查询计划未使用 ProviderRefs 索引: %s", plan)
			}
			if strings.Contains(plan, "USE TEMP B-TREE FOR GROUP BY") {
				t.Fatalf("查询计划仍使用临时分组 B-tree: %s", plan)
			}
		})
	}
}

func TestLogProviderRefsIndexPreservesResultsAndPlan(t *testing.T) {
	useIsolatedHomeDir(t)
	db := prepareLogDashboardFixture(t, 1440)
	applyHighCardinalityLogProviderRefsFixture(t, db)
	if _, err := db.Exec(`DROP INDEX IF EXISTS ` + requestLogProviderRefsIndexName); err != nil {
		t.Fatal(err)
	}
	baseline := make(map[string][]LogProviderRef, len(logProviderRefsBenchmarkScenarios()))
	for _, scenario := range logProviderRefsBenchmarkScenarios() {
		refs, err := listLogProviderRefsBenchmarkWithIndex(db, scenario, "")
		if err != nil {
			t.Fatal(err)
		}
		baseline[scenario.name] = refs
	}
	if _, err := db.Exec(requestLogProviderRefsIndexSQL); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(requestLogProviderRefsIndexSQL); err != nil {
		t.Fatalf("ProviderRefs 索引重复初始化失败: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(requestLogProviderRefsIndexSQL)
	})
	service := NewLogService(nil)
	for _, scenario := range logProviderRefsBenchmarkScenarios() {
		t.Run(scenario.name, func(t *testing.T) {
			refs, err := service.ListProviderRefsV2(scenario.platform, string(scenario.sourceMode))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(refs, baseline[scenario.name]) {
				t.Fatalf("候选索引改变 ProviderRefs:\ngot=%#v\nwant=%#v", refs, baseline[scenario.name])
			}
			plan := explainLogProviderRefsBenchmarkQuery(t, db, scenario)
			t.Logf("production query plan: %s", plan)
			if !strings.Contains(plan, requestLogProviderRefsIndexName) {
				t.Fatalf("生产查询未使用 ProviderRefs 索引: %s", plan)
			}
			if strings.Contains(plan, "USE TEMP B-TREE FOR GROUP BY") {
				t.Fatalf("生产查询仍使用临时分组 B-tree: %s", plan)
			}
		})
	}
}

func benchmarkLogProviderRefsForcedCandidate(
	b *testing.B,
	db *sql.DB,
	scenario logProviderRefsBenchmarkScenario,
) {
	b.Helper()
	matchedRows, candidateRows := countLogProviderRefsBenchmarkRows(b, db, scenario)
	wantRefs, err := listLogProviderRefsBenchmarkWithIndex(db, scenario, requestLogProviderRefsIndexName)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		logProviderRefsBenchmarkSink, err = listLogProviderRefsBenchmarkWithIndex(db, scenario, requestLogProviderRefsIndexName)
		if err != nil {
			b.Fatal(err)
		}
		if !reflect.DeepEqual(logProviderRefsBenchmarkSink, wantRefs) {
			b.Fatal("强制候选索引结果不稳定")
		}
	}
	b.ReportMetric(float64(matchedRows), "matched-rows/op")
	b.ReportMetric(float64(candidateRows), "candidates/op")
	b.ReportMetric(float64(len(wantRefs)), "refs/op")
}

func benchmarkLogProviderRefs(
	b *testing.B,
	db *sql.DB,
	service *LogService,
	scenario logProviderRefsBenchmarkScenario,
	cold bool,
) {
	b.Helper()
	matchedRows, candidateRows := countLogProviderRefsBenchmarkRows(b, db, scenario)
	warmRefs, err := service.ListProviderRefsV2(scenario.platform, string(scenario.sourceMode))
	if err != nil {
		b.Fatal(err)
	}
	wantRefs := len(warmRefs)
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		if cold {
			b.StopTimer()
			if _, err := db.Exec(`PRAGMA shrink_memory`); err != nil {
				b.Fatal(err)
			}
			b.StartTimer()
		}
		logProviderRefsBenchmarkSink, err = service.ListProviderRefsV2(scenario.platform, string(scenario.sourceMode))
		if err != nil {
			b.Fatal(err)
		}
		if len(logProviderRefsBenchmarkSink) != wantRefs {
			b.Fatalf("ProviderRefs 数量=%d，期望=%d", len(logProviderRefsBenchmarkSink), wantRefs)
		}
	}
	b.ReportMetric(float64(matchedRows), "matched-rows/op")
	b.ReportMetric(float64(candidateRows), "candidates/op")
	b.ReportMetric(float64(wantRefs), "refs/op")
}

func BenchmarkLogProviderRefs(b *testing.B) {
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
	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			for _, highCardinality := range []bool{false, true} {
				cardinality := "low-cardinality"
				if highCardinality {
					cardinality = "high-cardinality"
				}
				b.Run(cardinality, func(b *testing.B) {
					b.StopTimer()
					db := prepareLogDashboardFixture(b, size.count)
					if highCardinality {
						applyHighCardinalityLogProviderRefsFixture(b, db)
					}
					previousMaxOpenConns := db.Stats().MaxOpenConnections
					db.SetMaxOpenConns(1)
					b.Cleanup(func() {
						db.SetMaxOpenConns(previousMaxOpenConns)
					})
					service := NewLogService(nil)
					for _, scenario := range logProviderRefsBenchmarkScenarios() {
						b.Run(fmt.Sprintf("%s/Cold", scenario.name), func(b *testing.B) {
							benchmarkLogProviderRefs(b, db, service, scenario, true)
						})
						b.Run(fmt.Sprintf("%s/Warm", scenario.name), func(b *testing.B) {
							benchmarkLogProviderRefs(b, db, service, scenario, false)
						})
					}
				})
			}
		})
	}
}

func BenchmarkLogProviderRefsCandidateIndex(b *testing.B) {
	originalLogger := xlog.GetLogger()
	xlog.SetLogger(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})))
	b.Cleanup(func() {
		xlog.SetLogger(originalLogger)
	})
	b.StopTimer()
	db := prepareLogDashboardFixture(b, 100_000)
	applyHighCardinalityLogProviderRefsFixture(b, db)
	if _, err := db.Exec(`DROP INDEX IF EXISTS ` + requestLogProviderRefsIndexName); err != nil {
		b.Fatal(err)
	}
	previousMaxOpenConns := db.Stats().MaxOpenConnections
	db.SetMaxOpenConns(1)
	b.Cleanup(func() {
		db.SetMaxOpenConns(previousMaxOpenConns)
		_, _ = db.Exec(requestLogProviderRefsIndexSQL)
	})
	for _, scenario := range logProviderRefsBenchmarkScenarios() {
		b.Run(scenario.name+"/Baseline", func(b *testing.B) {
			matchedRows, candidateRows := countLogProviderRefsBenchmarkRows(b, db, scenario)
			wantRefs, err := listLogProviderRefsBenchmarkWithIndex(db, scenario, "")
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for iteration := 0; iteration < b.N; iteration++ {
				logProviderRefsBenchmarkSink, err = listLogProviderRefsBenchmarkWithIndex(db, scenario, "")
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(matchedRows), "matched-rows/op")
			b.ReportMetric(float64(candidateRows), "candidates/op")
			b.ReportMetric(float64(len(wantRefs)), "refs/op")
		})
	}
	if _, err := db.Exec(requestLogProviderRefsIndexSQL); err != nil {
		b.Fatal(err)
	}
	service := NewLogService(nil)
	for _, scenario := range logProviderRefsBenchmarkScenarios() {
		b.Run(scenario.name+"/Candidate", func(b *testing.B) {
			if scenario.platform == "" {
				benchmarkLogProviderRefs(b, db, service, scenario, false)
				return
			}
			benchmarkLogProviderRefsForcedCandidate(b, db, scenario)
		})
	}
}

func logProviderRefsBenchmarkUsedBytes(tb testing.TB, db *sql.DB) int64 {
	tb.Helper()
	var pageCount int64
	var pageSize int64
	var freePages int64
	if err := db.QueryRow(`PRAGMA page_count`).Scan(&pageCount); err != nil {
		tb.Fatal(err)
	}
	if err := db.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
		tb.Fatal(err)
	}
	if err := db.QueryRow(`PRAGMA freelist_count`).Scan(&freePages); err != nil {
		tb.Fatal(err)
	}
	return (pageCount - freePages) * pageSize
}

func insertLogProviderRefsBenchmarkRows(tb testing.TB, db *sql.DB, offset int, count int) {
	tb.Helper()
	tx, err := db.Begin()
	if err != nil {
		tb.Fatal(err)
	}
	stmt, err := tx.Prepare(`
		INSERT INTO request_log (
			platform, provider_id, provider, http_code, request_outcome, data_source, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		tb.Fatal(err)
	}
	platforms := []string{"codex", "claude", "gemini"}
	sources := []string{requestLogDataSourceProxy, requestLogDataSourceClaudeSession, requestLogDataSourceCodexSession}
	for index := 0; index < count; index++ {
		rowIndex := offset + index
		providerIndex := rowIndex % 64
		if _, err := stmt.Exec(
			platforms[rowIndex%len(platforms)],
			fmt.Sprintf("provider-%03d", providerIndex+1),
			fmt.Sprintf("Provider %03d", providerIndex+1),
			200,
			requestOutcomeSuccess,
			sources[rowIndex%len(sources)],
			logDashboardRangeStart.Add(time.Duration(rowIndex)*time.Second).UTC().Format(timeLayout),
		); err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			tb.Fatal(err)
		}
	}
	if err := stmt.Close(); err != nil {
		_ = tx.Rollback()
		tb.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		tb.Fatal(err)
	}
}

func TestLogProviderRefsCandidateIndexStorageCost(t *testing.T) {
	useIsolatedHomeDir(t)
	db := prepareLogDashboardFixture(t, 100_000)
	applyHighCardinalityLogProviderRefsFixture(t, db)
	if _, err := db.Exec(`DROP INDEX IF EXISTS ` + requestLogProviderRefsIndexName); err != nil {
		t.Fatal(err)
	}
	baselineBytes := logProviderRefsBenchmarkUsedBytes(t, db)
	if _, err := db.Exec(requestLogProviderRefsIndexSQL); err != nil {
		t.Fatal(err)
	}
	indexBytes := logProviderRefsBenchmarkUsedBytes(t, db) - baselineBytes
	if indexBytes <= 0 {
		t.Fatalf("候选索引未产生可测量存储占用: %d bytes", indexBytes)
	}
	t.Logf("candidate index storage: %d bytes, %.2f bytes/row", indexBytes, float64(indexBytes)/100_000)
	t.Cleanup(func() {
		_, _ = db.Exec(requestLogProviderRefsIndexSQL)
	})
}

func BenchmarkLogProviderRefsCandidateIndexWriteCost(b *testing.B) {
	const fixtureRows = 100_000
	const batchRows = 10_000
	for _, candidate := range []bool{false, true} {
		name := "Baseline"
		if candidate {
			name = "Candidate"
		}
		b.Run(name, func(b *testing.B) {
			b.StopTimer()
			db := prepareLogDashboardFixture(b, fixtureRows)
			applyHighCardinalityLogProviderRefsFixture(b, db)
			if _, err := db.Exec(`DROP INDEX IF EXISTS ` + requestLogProviderRefsIndexName); err != nil {
				b.Fatal(err)
			}
			if candidate {
				if _, err := db.Exec(requestLogProviderRefsIndexSQL); err != nil {
					b.Fatal(err)
				}
			}
			b.Cleanup(func() {
				_, _ = db.Exec(requestLogProviderRefsIndexSQL)
			})
			b.ReportAllocs()
			b.ResetTimer()
			b.StartTimer()
			for iteration := 0; iteration < b.N; iteration++ {
				insertLogProviderRefsBenchmarkRows(b, db, fixtureRows+iteration*batchRows, batchRows)
			}
			b.ReportMetric(batchRows, "rows/op")
		})
	}
}
