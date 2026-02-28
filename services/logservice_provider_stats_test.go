package services

import (
	"database/sql"
	"math"
	"testing"

	"github.com/daodao97/xgo/xdb"
)

func TestProviderStatsRangeV2_PerformanceAggregatesByStableProviderKey(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		ProviderID:    "pid-1",
		Provider:      "Acme",
		IsStream:      1,
		DurationSec:   2.0,
		FirstTokenSec: 0.2,
		OutputTokens:  100,
		CreatedAt:     "2026-02-25 10:00:00",
	})
	// 同 provider_id 使用不同 provider 名称，不应覆盖性能均值。
	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		ProviderID:    "pid-1",
		Provider:      "Acme-Renamed",
		IsStream:      1,
		DurationSec:   1.4,
		FirstTokenSec: 0.4,
		OutputTokens:  60,
		CreatedAt:     "2026-02-25 11:00:00",
	})
	// first_token_sec 无效，不应参与性能均值。
	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		ProviderID:    "pid-1",
		Provider:      "Acme-Renamed",
		IsStream:      1,
		DurationSec:   1.2,
		FirstTokenSec: 0,
		OutputTokens:  30,
		CreatedAt:     "2026-02-25 11:30:00",
	})
	// 非流式，不应参与性能均值。
	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		ProviderID:    "pid-1",
		Provider:      "Acme",
		IsStream:      0,
		DurationSec:   1.0,
		FirstTokenSec: 0.1,
		OutputTokens:  80,
		CreatedAt:     "2026-02-25 12:00:00",
	})

	ls := NewLogService(nil)
	stats, err := ls.ProviderStatsRangeV2("codex", "", "2026-02-25 00:00:00", "2026-02-26 00:00:00")
	if err != nil {
		t.Fatalf("ProviderStatsRangeV2 调用失败: %v", err)
	}

	stat := findProviderStatByID(stats, "pid-1")
	if stat == nil {
		t.Fatalf("未找到 provider_id=pid-1 的统计")
	}
	if stat.TotalRequests != 4 {
		t.Fatalf("期望 total_requests=4，实际 %d", stat.TotalRequests)
	}

	expectedTTFT := 0.3 // (0.2 + 0.4) / 2
	if !almostEqualFloatProviderStat(stat.AvgFirstTokenSec, expectedTTFT) {
		t.Fatalf("期望 avg_first_token_sec=%f，实际 %f", expectedTTFT, stat.AvgFirstTokenSec)
	}

	expectedTPS := (100.0/(2.0-0.2) + 60.0/(1.4-0.4)) / 2.0
	if !almostEqualFloatProviderStat(stat.AvgTokensPerSec, expectedTPS) {
		t.Fatalf("期望 avg_tokens_per_sec=%f，实际 %f", expectedTPS, stat.AvgTokensPerSec)
	}
}

func TestProviderStatsRangeV2_PerformanceReturnsZeroWhenNoValidStreamingSamples(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		ProviderID:    "pid-2",
		Provider:      "NoStreamA",
		IsStream:      1,
		DurationSec:   2.0,
		FirstTokenSec: 0,
		OutputTokens:  50,
		CreatedAt:     "2026-02-25 13:00:00",
	})
	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		ProviderID:    "pid-2",
		Provider:      "NoStreamB",
		IsStream:      0,
		DurationSec:   1.0,
		FirstTokenSec: 0.1,
		OutputTokens:  40,
		CreatedAt:     "2026-02-25 14:00:00",
	})

	ls := NewLogService(nil)
	stats, err := ls.ProviderStatsRangeV2("codex", "", "2026-02-25 00:00:00", "2026-02-26 00:00:00")
	if err != nil {
		t.Fatalf("ProviderStatsRangeV2 调用失败: %v", err)
	}

	stat := findProviderStatByID(stats, "pid-2")
	if stat == nil {
		t.Fatalf("未找到 provider_id=pid-2 的统计")
	}
	if stat.AvgFirstTokenSec != 0 {
		t.Fatalf("无有效样本时 avg_first_token_sec 应为 0，实际 %f", stat.AvgFirstTokenSec)
	}
	if stat.AvgTokensPerSec != 0 {
		t.Fatalf("无有效样本时 avg_tokens_per_sec 应为 0，实际 %f", stat.AvgTokensPerSec)
	}
}

func TestProviderStatsRangeV2_FallbackToProviderNameWhenProviderIDNotFound(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForProviderStats(t, db, providerStatsLogEntry{
		Platform:      "codex",
		ProviderID:    "",
		Provider:      "legacy-provider",
		IsStream:      1,
		DurationSec:   1.0,
		FirstTokenSec: 0.2,
		OutputTokens:  40,
		CreatedAt:     "2026-02-25 15:00:00",
	})

	ls := NewLogService(nil)
	stats, err := ls.ProviderStatsRangeV2("codex", "legacy-provider", "2026-02-25 00:00:00", "2026-02-26 00:00:00")
	if err != nil {
		t.Fatalf("ProviderStatsRangeV2 调用失败: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("期望返回 1 条 provider 统计，实际 %d", len(stats))
	}
	if stats[0].Provider != "legacy-provider" {
		t.Fatalf("期望 provider=legacy-provider，实际 %s", stats[0].Provider)
	}
	if !almostEqualFloatProviderStat(stats[0].AvgFirstTokenSec, 0.2) {
		t.Fatalf("期望 avg_first_token_sec=0.2，实际 %f", stats[0].AvgFirstTokenSec)
	}
	if !almostEqualFloatProviderStat(stats[0].AvgTokensPerSec, 50.0) {
		t.Fatalf("期望 avg_tokens_per_sec=50，实际 %f", stats[0].AvgTokensPerSec)
	}
}

type providerStatsLogEntry struct {
	Platform      string
	ProviderID    string
	Provider      string
	IsStream      int
	DurationSec   float64
	FirstTokenSec float64
	OutputTokens  int
	CreatedAt     string
}

func insertRequestLogForProviderStats(t *testing.T, db *sql.DB, entry providerStatsLogEntry) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO request_log (
			platform,
			provider_id,
			provider,
			http_code,
			input_tokens,
			output_tokens,
			cache_create_tokens,
			cache_read_tokens,
			reasoning_tokens,
			is_stream,
			duration_sec,
			first_token_sec,
			total_cost,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		entry.Platform,
		entry.ProviderID,
		entry.Provider,
		200,
		0,
		entry.OutputTokens,
		0,
		0,
		0,
		entry.IsStream,
		entry.DurationSec,
		entry.FirstTokenSec,
		0,
		entry.CreatedAt,
	)
	if err != nil {
		t.Fatalf("插入 request_log 失败: %v", err)
	}
}

func findProviderStatByID(stats []ProviderDailyStat, providerID string) *ProviderDailyStat {
	for i := range stats {
		if stats[i].ProviderID == providerID {
			return &stats[i]
		}
	}
	return nil
}

func almostEqualFloatProviderStat(left, right float64) bool {
	return math.Abs(left-right) < 1e-9
}
