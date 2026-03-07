package services

import (
	"database/sql"
	"testing"

	"github.com/daodao97/xgo/xdb"
)

func TestListRequestLogsPageV2_UsesProviderIDFirstAndReturnsPagedItems(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForPageTest(t, db, requestLogPageEntry{
		Platform:   "codex",
		Model:      "gpt-5.3-codex",
		ProviderID: "pid-1",
		Provider:   "Acme",
		CreatedAt:  "2026-02-25 10:00:00",
		TotalCost:  1.2,
	})
	insertRequestLogForPageTest(t, db, requestLogPageEntry{
		Platform:   "codex",
		Model:      "gpt-5.3-codex-mini",
		ProviderID: "pid-1",
		Provider:   "Acme",
		CreatedAt:  "2026-02-25 11:00:00",
		TotalCost:  0.8,
	})
	insertRequestLogForPageTest(t, db, requestLogPageEntry{
		Platform:   "codex",
		Model:      "name-only-should-be-ignored",
		ProviderID: "",
		Provider:   "pid-1",
		CreatedAt:  "2026-02-25 12:00:00",
		TotalCost:  9.9,
	})

	ls := NewLogService(nil)
	page, err := ls.ListRequestLogsPageV2("codex", "pid-1", 1, 0, "2026-02-25 00:00:00", "2026-02-26 00:00:00")
	if err != nil {
		t.Fatalf("ListRequestLogsPageV2 调用失败: %v", err)
	}

	if page.Total != 2 {
		t.Fatalf("期望 total=2，实际 %d", page.Total)
	}
	if len(page.Items) != 1 {
		t.Fatalf("期望当前页 1 条数据，实际 %d", len(page.Items))
	}
	if page.Items[0].Model != "gpt-5.3-codex-mini" {
		t.Fatalf("期望按 created_at 倒序返回最新一条，实际 %s", page.Items[0].Model)
	}

	nextPage, err := ls.ListRequestLogsPageV2("codex", "pid-1", 1, 1, "2026-02-25 00:00:00", "2026-02-26 00:00:00")
	if err != nil {
		t.Fatalf("第二页查询失败: %v", err)
	}
	if nextPage.Total != 2 {
		t.Fatalf("期望第二页 total 仍为 2，实际 %d", nextPage.Total)
	}
	if len(nextPage.Items) != 1 || nextPage.Items[0].Model != "gpt-5.3-codex" {
		t.Fatalf("期望第二页命中较早一条记录，实际 %+v", nextPage.Items)
	}
}

func TestListRequestLogsPageV2_FallbackToProviderNameWhenProviderIDMissing(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForPageTest(t, db, requestLogPageEntry{
		Platform:   "codex",
		Model:      "legacy-model",
		ProviderID: "",
		Provider:   "legacy-provider",
		CreatedAt:  "2026-02-25 13:00:00",
		TotalCost:  2.5,
	})

	ls := NewLogService(nil)
	page, err := ls.ListRequestLogsPageV2("codex", "legacy-provider", 20, 0, "2026-02-25 00:00:00", "2026-02-26 00:00:00")
	if err != nil {
		t.Fatalf("ListRequestLogsPageV2 回退 provider 名称查询失败: %v", err)
	}

	if page.Total != 1 {
		t.Fatalf("期望 total=1，实际 %d", page.Total)
	}
	if len(page.Items) != 1 {
		t.Fatalf("期望返回 1 条记录，实际 %d", len(page.Items))
	}
	if page.Items[0].Model != "legacy-model" {
		t.Fatalf("期望命中 legacy-model，实际 %s", page.Items[0].Model)
	}
}

type requestLogPageEntry struct {
	Platform   string
	Model      string
	ProviderID string
	Provider   string
	CreatedAt  string
	TotalCost  float64
}

func insertRequestLogForPageTest(t *testing.T, db *sql.DB, entry requestLogPageEntry) {
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
		10,
		5,
		0,
		1,
		0,
		entry.TotalCost,
		entry.CreatedAt,
	)
	if err != nil {
		t.Fatalf("插入 request_log 失败: %v", err)
	}
}
