package services

import (
	"database/sql"
	"math"
	"testing"

	"github.com/daodao97/xgo/xdb"
)

func TestModelStatsRangeV2_UsesProviderIDFirstAndKeepsCostSign(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForModelStats(t, db, modelStatsLogEntry{
		Platform:     "codex",
		Model:        "gpt-5.3-codex",
		ProviderID:   "pid-1",
		Provider:     "Acme",
		InputTokens:  100,
		OutputTokens: 50,
		CacheRead:    25,
		Reasoning:    999,
		TotalCost:    10,
		CreatedAt:    "2026-02-25 10:00:00",
	})
	insertRequestLogForModelStats(t, db, modelStatsLogEntry{
		Platform:     "codex",
		Model:        "gpt-5.3-codex",
		ProviderID:   "pid-1",
		Provider:     "Acme",
		InputTokens:  10,
		OutputTokens: 5,
		CacheRead:    0,
		Reasoning:    500,
		TotalCost:    -2,
		CreatedAt:    "2026-02-25 11:00:00",
	})
	insertRequestLogForModelStats(t, db, modelStatsLogEntry{
		Platform:     "codex",
		Model:        "gpt-5-mini",
		ProviderID:   "pid-1",
		Provider:     "Acme",
		InputTokens:  1,
		OutputTokens: 1,
		CacheRead:    1,
		Reasoning:    100,
		TotalCost:    1,
		CreatedAt:    "2026-02-25 11:30:00",
	})
	// provider 名称等于过滤值，但 provider_id 为空；当 provider_id 命中时不应回退到 name。
	insertRequestLogForModelStats(t, db, modelStatsLogEntry{
		Platform:     "codex",
		Model:        "name-only-provider-row",
		ProviderID:   "",
		Provider:     "pid-1",
		InputTokens:  999,
		OutputTokens: 999,
		CacheRead:    999,
		Reasoning:    999,
		TotalCost:    99,
		CreatedAt:    "2026-02-25 12:00:00",
	})
	// 超出范围，不应进入统计。
	insertRequestLogForModelStats(t, db, modelStatsLogEntry{
		Platform:     "codex",
		Model:        "out-of-range-model",
		ProviderID:   "pid-1",
		Provider:     "Acme",
		InputTokens:  500,
		OutputTokens: 500,
		CacheRead:    500,
		Reasoning:    500,
		TotalCost:    50,
		CreatedAt:    "2026-02-26 00:00:00",
	})

	ls := NewLogService(nil)
	stats, err := ls.ModelStatsRangeV2("codex", "pid-1", "2026-02-25 00:00:00", "2026-02-26 00:00:00")
	if err != nil {
		t.Fatalf("ModelStatsRangeV2 调用失败: %v", err)
	}

	if len(stats) != 2 {
		t.Fatalf("期望 2 条模型统计，实际 %d", len(stats))
	}

	if stats[0].Model != "gpt-5.3-codex" {
		t.Fatalf("期望首条模型为 gpt-5.3-codex，实际 %s", stats[0].Model)
	}
	if stats[0].TotalRequests != 2 {
		t.Fatalf("期望 gpt-5.3-codex 请求数为 2，实际 %d", stats[0].TotalRequests)
	}
	// token 口径应为 input + output + cache_read，不包含 reasoning。
	if stats[0].TotalTokens != 190 {
		t.Fatalf("期望 gpt-5.3-codex token 总数为 190，实际 %d", stats[0].TotalTokens)
	}
	if !almostEqualFloat(stats[0].CostTotal, 8) {
		t.Fatalf("期望 gpt-5.3-codex 金额为 8，实际 %f", stats[0].CostTotal)
	}

	if stats[1].Model != "gpt-5-mini" {
		t.Fatalf("期望第二条模型为 gpt-5-mini，实际 %s", stats[1].Model)
	}
	if stats[1].TotalTokens != 3 {
		t.Fatalf("期望 gpt-5-mini token 总数为 3，实际 %d", stats[1].TotalTokens)
	}

	for _, stat := range stats {
		if stat.Model == "name-only-provider-row" {
			t.Fatalf("provider_id 命中时不应混入 provider 名称回退数据")
		}
	}
}

func TestModelStatsRangeV2_FallbackToProviderNameWhenIDNotFound(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForModelStats(t, db, modelStatsLogEntry{
		Platform:     "codex",
		Model:        "legacy-model",
		ProviderID:   "",
		Provider:     "legacy-provider",
		InputTokens:  7,
		OutputTokens: 8,
		CacheRead:    9,
		Reasoning:    11,
		TotalCost:    3.5,
		CreatedAt:    "2026-02-25 13:00:00",
	})

	ls := NewLogService(nil)
	stats, err := ls.ModelStatsRangeV2("codex", "legacy-provider", "2026-02-25 00:00:00", "2026-02-26 00:00:00")
	if err != nil {
		t.Fatalf("ModelStatsRangeV2 调用失败: %v", err)
	}

	if len(stats) != 1 {
		t.Fatalf("期望 1 条模型统计，实际 %d", len(stats))
	}
	if stats[0].Model != "legacy-model" {
		t.Fatalf("期望模型为 legacy-model，实际 %s", stats[0].Model)
	}
	if stats[0].TotalTokens != 24 {
		t.Fatalf("期望 token 总数为 24，实际 %d", stats[0].TotalTokens)
	}
	if !almostEqualFloat(stats[0].CostTotal, 3.5) {
		t.Fatalf("期望金额为 3.5，实际 %f", stats[0].CostTotal)
	}
}

type modelStatsLogEntry struct {
	Platform     string
	Model        string
	ProviderID   string
	Provider     string
	InputTokens  int
	OutputTokens int
	CacheRead    int
	Reasoning    int
	TotalCost    float64
	CreatedAt    string
}

func insertRequestLogForModelStats(t *testing.T, db *sql.DB, entry modelStatsLogEntry) {
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
		entry.Platform,
		entry.Model,
		entry.ProviderID,
		entry.Provider,
		200,
		entry.InputTokens,
		entry.OutputTokens,
		0,
		entry.CacheRead,
		entry.Reasoning,
		entry.TotalCost,
		entry.CreatedAt,
	)
	if err != nil {
		t.Fatalf("插入 request_log 失败: %v", err)
	}
}

func almostEqualFloat(left, right float64) bool {
	return math.Abs(left-right) < 1e-9
}
