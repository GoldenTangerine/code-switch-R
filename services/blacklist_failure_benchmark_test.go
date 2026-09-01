/**
 * @name: 黑名单失败路径并发基线
 * @Descripttion: 测量同一与不同 Provider 失败记录在全局锁和数据库队列下的等待与分配
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-09-01 02:16:38
 * @LastEditTime: 2026-09-01 02:16:38
 * @FilePath: services/blacklist_failure_benchmark_test.go
 */

package services

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/daodao97/xgo/xdb"
	"github.com/daodao97/xgo/xlog"
)

const blacklistFailureBenchmarkPlatform = "codex"

type blacklistFailureBenchmarkProvider struct {
	id   string
	name string
}

type blacklistFailureBenchmarkScenario struct {
	name                 string
	initialFailureCount  int
	windowState          string
	initiallyBlacklisted bool
	identityBound        bool
	wantFailureCount     int
	wantQueueWrites      int64
	wantBlacklisted      bool
	wantReason           bool
}

type blacklistFailureBenchmarkState struct {
	rows            int
	totalFailures   int
	blacklistedRows int
	reasonRows      int
}

type blacklistFailureBenchmarkSetting struct {
	key    string
	value  string
	exists bool
}

func blacklistFailureBenchmarkScenarios() []blacklistFailureBenchmarkScenario {
	return []blacklistFailureBenchmarkScenario{
		{name: "first_bound", identityBound: true, wantFailureCount: 1, wantQueueWrites: 1},
		{name: "first_unbound", wantFailureCount: 1, wantQueueWrites: 3},
		{name: "increment", initialFailureCount: 1, windowState: "expired", identityBound: true, wantFailureCount: 2, wantQueueWrites: 1},
		{name: "threshold", initialFailureCount: 4, windowState: "expired", identityBound: true, wantFailureCount: 5, wantQueueWrites: 1, wantBlacklisted: true, wantReason: true},
		{name: "dedupe", initialFailureCount: 1, windowState: "active", identityBound: true, wantFailureCount: 1},
		{name: "blacklisted", initialFailureCount: 5, windowState: "active", initiallyBlacklisted: true, identityBound: true, wantFailureCount: 5, wantBlacklisted: true},
	}
}

func configureBlacklistFailureBenchmark(tb testing.TB) func() {
	tb.Helper()
	db, err := xdb.DB("default")
	if err != nil {
		tb.Fatal(err)
	}
	if GlobalDBQueue == nil {
		tb.Fatal("GlobalDBQueue 未初始化")
	}
	settings := []blacklistFailureBenchmarkSetting{
		{key: "enable_blacklist"},
		{key: "blacklist_level_enabled"},
		{key: "blacklist_failure_threshold"},
	}
	for index := range settings {
		err := db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, settings[index].key).Scan(&settings[index].value)
		if err == nil {
			settings[index].exists = true
		} else if err != sql.ErrNoRows {
			tb.Fatal(err)
		}
	}
	for _, setting := range []blacklistFailureBenchmarkSetting{
		{key: "enable_blacklist", value: "true"},
		{key: "blacklist_level_enabled", value: "true"},
		{key: "blacklist_failure_threshold", value: "5"},
	} {
		if _, err := db.Exec(`
			INSERT INTO app_settings (key, value) VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value
		`, setting.key, setting.value); err != nil {
			tb.Fatal(err)
		}
	}
	if _, err := db.Exec(`DELETE FROM provider_blacklist`); err != nil {
		tb.Fatal(err)
	}

	previousLogWriter := log.Writer()
	log.SetOutput(io.Discard)
	originalLogger := xlog.GetLogger()
	xlog.SetLogger(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})))

	return func() {
		log.SetOutput(previousLogWriter)
		xlog.SetLogger(originalLogger)
		_, _ = db.Exec(`DELETE FROM provider_blacklist`)
		for _, setting := range settings {
			if setting.exists {
				_, _ = db.Exec(`UPDATE app_settings SET value = ? WHERE key = ?`, setting.value, setting.key)
			} else {
				_, _ = db.Exec(`DELETE FROM app_settings WHERE key = ?`, setting.key)
			}
		}
	}
}

func blacklistFailureBenchmarkProviders(concurrency int, sameProvider bool) []blacklistFailureBenchmarkProvider {
	providers := make([]blacklistFailureBenchmarkProvider, concurrency)
	for index := range providers {
		providerIndex := index
		if sameProvider {
			providerIndex = 0
		}
		providers[index] = blacklistFailureBenchmarkProvider{
			id:   fmt.Sprintf("blacklist-benchmark-%03d", providerIndex+1),
			name: fmt.Sprintf("Blacklist Benchmark %03d", providerIndex+1),
		}
	}
	return providers
}

func seedBlacklistFailureBenchmarkRow(tb testing.TB, db *sql.DB, provider blacklistFailureBenchmarkProvider, scenario blacklistFailureBenchmarkScenario) {
	tb.Helper()
	if scenario.initialFailureCount == 0 {
		return
	}
	now := time.Now()
	var lastFailureWindowStart any
	switch scenario.windowState {
	case "expired":
		lastFailureWindowStart = now.Add(-10 * time.Second)
	case "active":
		lastFailureWindowStart = now
	}
	var blacklistedAt any
	var blacklistedUntil any
	blacklistLevel := 0
	if scenario.initiallyBlacklisted {
		blacklistedAt = now
		blacklistedUntil = now.Add(time.Hour)
		blacklistLevel = 1
	}
	if _, err := db.Exec(`
		INSERT INTO provider_blacklist (
			platform, provider_id, provider_name, failure_count,
			last_failure_at, last_failure_window_start,
			blacklisted_at, blacklisted_until, blacklist_level
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, blacklistFailureBenchmarkPlatform, provider.id, provider.name, scenario.initialFailureCount,
		now, lastFailureWindowStart, blacklistedAt, blacklistedUntil, blacklistLevel); err != nil {
		tb.Fatal(err)
	}
}

func prepareBlacklistFailureBenchmark(
	tb testing.TB,
	scenario blacklistFailureBenchmarkScenario,
	concurrency int,
	sameProvider bool,
) (*BlacklistService, []blacklistFailureBenchmarkProvider) {
	tb.Helper()
	db, err := xdb.DB("default")
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM provider_blacklist`); err != nil {
		tb.Fatal(err)
	}
	providers := blacklistFailureBenchmarkProviders(concurrency, sameProvider)
	seeded := make(map[string]bool, concurrency)
	for _, provider := range providers {
		if seeded[provider.id] {
			continue
		}
		seeded[provider.id] = true
		seedBlacklistFailureBenchmarkRow(tb, db, provider, scenario)
	}
	service := NewBlacklistService(NewSettingsService(), nil)
	if scenario.identityBound {
		for _, provider := range providers {
			if service.isProviderIdentityBound(blacklistFailureBenchmarkPlatform, provider.id, provider.name) {
				continue
			}
			service.bindProviderIdentity(blacklistFailureBenchmarkPlatform, provider.id, provider.name)
		}
	}
	return service, providers
}

func readBlacklistFailureBenchmarkState(tb testing.TB) blacklistFailureBenchmarkState {
	tb.Helper()
	db, err := xdb.DB("default")
	if err != nil {
		tb.Fatal(err)
	}
	var state blacklistFailureBenchmarkState
	if err := db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(failure_count), 0),
			COALESCE(SUM(CASE WHEN blacklisted_until > ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN blacklist_reason = 'benchmark failure' THEN 1 ELSE 0 END), 0)
		FROM provider_blacklist
	`, time.Now()).Scan(&state.rows, &state.totalFailures, &state.blacklistedRows, &state.reasonRows); err != nil {
		tb.Fatal(err)
	}
	return state
}

func assertBlacklistFailureBenchmarkState(
	tb testing.TB,
	service *BlacklistService,
	providers []blacklistFailureBenchmarkProvider,
	wantRows int,
	wantTotalFailures int,
	wantBlacklistedRows int,
	wantReasonRows int,
	wantQueueWrites int64,
	writesBefore int64,
) {
	tb.Helper()
	state := readBlacklistFailureBenchmarkState(tb)
	if state.rows != wantRows || state.totalFailures != wantTotalFailures || state.blacklistedRows != wantBlacklistedRows {
		tb.Fatalf("黑名单状态异常: got=%+v wantRows=%d wantFailures=%d wantBlacklisted=%d", state, wantRows, wantTotalFailures, wantBlacklistedRows)
	}
	if state.reasonRows != wantReasonRows {
		tb.Fatalf("拉黑原因记录数=%d，期望=%d", state.reasonRows, wantReasonRows)
	}
	if writes := GetGlobalDBQueueStats().TotalWrites - writesBefore; writes != wantQueueWrites {
		tb.Fatalf("队列写入数=%d，期望=%d", writes, wantQueueWrites)
	}
	checked := make(map[string]bool, len(providers))
	for _, provider := range providers {
		if checked[provider.id] {
			continue
		}
		checked[provider.id] = true
		blacklisted, _ := service.IsBlacklistedByID(blacklistFailureBenchmarkPlatform, provider.id, provider.name)
		if blacklisted != (wantBlacklistedRows > 0) {
			tb.Fatalf("Provider %s 运行时拉黑状态=%v", provider.id, blacklisted)
		}
	}
}

func runBlacklistFailureBenchmarkCalls(
	service *BlacklistService,
	providers []blacklistFailureBenchmarkProvider,
	durations []time.Duration,
	errors []error,
) time.Duration {
	var ready sync.WaitGroup
	var done sync.WaitGroup
	start := make(chan struct{})
	ready.Add(len(providers))
	done.Add(len(providers))
	for index, provider := range providers {
		go func(index int, provider blacklistFailureBenchmarkProvider) {
			defer done.Done()
			ready.Done()
			<-start
			startedAt := time.Now()
			errors[index] = service.RecordFailureWithReasonByID(
				blacklistFailureBenchmarkPlatform,
				provider.id,
				provider.name,
				"benchmark failure",
			)
			durations[index] = time.Since(startedAt)
		}(index, provider)
	}
	ready.Wait()
	startedAt := time.Now()
	close(start)
	done.Wait()
	return time.Since(startedAt)
}

func assertBlacklistFailureBenchmarkErrors(tb testing.TB, errors []error) {
	tb.Helper()
	for index, err := range errors {
		if err != nil {
			tb.Fatalf("第 %d 个失败记录调用异常: %v", index, err)
		}
	}
}

func blacklistFailureBenchmarkPercentile(samples []time.Duration, percentile float64) time.Duration {
	ordered := append([]time.Duration(nil), samples...)
	sort.Slice(ordered, func(left, right int) bool { return ordered[left] < ordered[right] })
	index := int(math.Ceil(float64(len(ordered))*percentile)) - 1
	if index < 0 {
		index = 0
	}
	return ordered[index]
}

func TestBlacklistFailureBenchmarkFixture(t *testing.T) {
	restore := configureBlacklistFailureBenchmark(t)
	defer restore()
	for _, scenario := range blacklistFailureBenchmarkScenarios() {
		t.Run(scenario.name, func(t *testing.T) {
			service, providers := prepareBlacklistFailureBenchmark(t, scenario, 1, true)
			writesBefore := GetGlobalDBQueueStats().TotalWrites
			err := service.RecordFailureWithReasonByID(
				blacklistFailureBenchmarkPlatform,
				providers[0].id,
				providers[0].name,
				"benchmark failure",
			)
			if err != nil {
				t.Fatal(err)
			}
			wantBlacklistedRows := 0
			if scenario.wantBlacklisted {
				wantBlacklistedRows = 1
			}
			wantReasonRows := 0
			if scenario.wantReason {
				wantReasonRows = 1
			}
			assertBlacklistFailureBenchmarkState(
				t,
				service,
				providers,
				1,
				scenario.wantFailureCount,
				wantBlacklistedRows,
				wantReasonRows,
				scenario.wantQueueWrites,
				writesBefore,
			)
		})
	}
}

func BenchmarkBlacklistFailureScenarios(b *testing.B) {
	restore := configureBlacklistFailureBenchmark(b)
	defer restore()
	for _, scenario := range blacklistFailureBenchmarkScenarios() {
		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			b.StopTimer()
			latencies := make([]time.Duration, b.N)
			var totalWrites int64
			for index := 0; index < b.N; index++ {
				service, providers := prepareBlacklistFailureBenchmark(b, scenario, 1, true)
				writesBefore := GetGlobalDBQueueStats().TotalWrites
				b.StartTimer()
				startedAt := time.Now()
				err := service.RecordFailureWithReasonByID(
					blacklistFailureBenchmarkPlatform,
					providers[0].id,
					providers[0].name,
					"benchmark failure",
				)
				latencies[index] = time.Since(startedAt)
				b.StopTimer()
				if err != nil {
					b.Fatal(err)
				}
				writes := GetGlobalDBQueueStats().TotalWrites - writesBefore
				totalWrites += writes
				wantBlacklistedRows := 0
				if scenario.wantBlacklisted {
					wantBlacklistedRows = 1
				}
				wantReasonRows := 0
				if scenario.wantReason {
					wantReasonRows = 1
				}
				assertBlacklistFailureBenchmarkState(
					b,
					service,
					providers,
					1,
					scenario.wantFailureCount,
					wantBlacklistedRows,
					wantReasonRows,
					scenario.wantQueueWrites,
					writesBefore,
				)
			}
			b.ReportMetric(float64(totalWrites)/float64(b.N), "queue-writes/op")
			b.ReportMetric(float64(blacklistFailureBenchmarkPercentile(latencies, 0.50).Nanoseconds()), "p50-ns/call")
			b.ReportMetric(float64(blacklistFailureBenchmarkPercentile(latencies, 0.95).Nanoseconds()), "p95-ns/call")
		})
	}
}

func BenchmarkBlacklistFailureContention(b *testing.B) {
	restore := configureBlacklistFailureBenchmark(b)
	defer restore()
	scenario := blacklistFailureBenchmarkScenario{
		name:                "increment",
		initialFailureCount: 1,
		windowState:         "expired",
		identityBound:       true,
	}
	for _, sameProvider := range []bool{true, false} {
		mode := "different_provider"
		if sameProvider {
			mode = "same_provider"
		}
		for _, concurrency := range []int{1, 8, 32} {
			b.Run(fmt.Sprintf("%s/%d", mode, concurrency), func(b *testing.B) {
				b.ReportAllocs()
				b.StopTimer()
				latencies := make([]time.Duration, b.N*concurrency)
				var totalCalls int
				var totalWrites int64
				for operation := 0; operation < b.N; operation++ {
					service, providers := prepareBlacklistFailureBenchmark(b, scenario, concurrency, sameProvider)
					errors := make([]error, concurrency)
					operationLatencies := latencies[operation*concurrency : (operation+1)*concurrency]
					writesBefore := GetGlobalDBQueueStats().TotalWrites
					b.StartTimer()
					runBlacklistFailureBenchmarkCalls(service, providers, operationLatencies, errors)
					b.StopTimer()
					assertBlacklistFailureBenchmarkErrors(b, errors)
					wantRows := concurrency
					wantFailures := concurrency * 2
					wantWrites := int64(concurrency)
					if sameProvider {
						wantRows = 1
						wantFailures = 2
						wantWrites = 1
					}
					assertBlacklistFailureBenchmarkState(
						b,
						service,
						providers,
						wantRows,
						wantFailures,
						0,
						0,
						wantWrites,
						writesBefore,
					)
					totalCalls += concurrency
					totalWrites += wantWrites
				}
				b.ReportMetric(float64(concurrency), "calls/op")
				b.ReportMetric(float64(totalCalls)/b.Elapsed().Seconds(), "calls/s")
				b.ReportMetric(float64(totalWrites)/float64(b.N), "queue-writes/op")
				b.ReportMetric(float64(blacklistFailureBenchmarkPercentile(latencies, 0.50).Nanoseconds()), "p50-ns/call")
				b.ReportMetric(float64(blacklistFailureBenchmarkPercentile(latencies, 0.95).Nanoseconds()), "p95-ns/call")
			})
		}
	}
}
