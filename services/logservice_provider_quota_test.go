package services

import (
	"database/sql"
	"testing"
	"time"

	"github.com/daodao97/xgo/xdb"
)

func TestResolveFiveHourQuotaStatusByProviderAt_KeepsPersistedCycleAfterDeletingHistoryRows(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForProviderQuotaTest(t, db, providerQuotaLogEntry{
		Platform:   "codex",
		ProviderID: "pid-1",
		Provider:   "Acme",
		CreatedAt:  "2026-02-25 10:00:00",
		TotalCost:  1.2,
	})
	insertRequestLogForProviderQuotaTest(t, db, providerQuotaLogEntry{
		Platform:   "codex",
		ProviderID: "pid-1",
		Provider:   "Acme",
		CreatedAt:  "2026-02-25 12:30:00",
		TotalCost:  0.8,
	})

	if _, err := db.Exec(`DELETE FROM request_log WHERE platform = ? AND provider_id = ? AND created_at = ?`, "codex", "pid-1", "2026-02-25 10:00:00"); err != nil {
		t.Fatalf("删除 request_log 失败: %v", err)
	}

	ls := NewLogService(nil)
	now := time.Date(2026, time.February, 25, 13, 15, 0, 0, time.UTC)
	status, err := ls.resolveFiveHourQuotaStatusByProviderAt("codex", "pid-1", "Acme", now)
	if err != nil {
		t.Fatalf("resolveFiveHourQuotaStatusByProviderAt 调用失败: %v", err)
	}

	if !status.Active {
		t.Fatalf("删除历史日志后，provider 级 5 小时周期仍应保持活跃")
	}

	expectedStart := time.Date(2026, time.February, 25, 10, 0, 0, 0, time.UTC)
	if status.WindowStart != expectedStart.In(time.Local).Format(timeLayout) {
		t.Fatalf("期望周期开始时间保持为 %s，实际 %s", expectedStart.In(time.Local).Format(timeLayout), status.WindowStart)
	}
	if !almostEqualFiveHourQuotaFloat(status.Used, 2.0) {
		t.Fatalf("期望已用金额保持为 2.0，实际 %f", status.Used)
	}
}

func TestSyncProviderIdentityRename_UpdatesProviderQuotaCycleStateKey(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForProviderQuotaTest(t, db, providerQuotaLogEntry{
		Platform:   "codex",
		ProviderID: "",
		Provider:   "Legacy Acme",
		CreatedAt:  "2026-02-25 10:00:00",
		TotalCost:  1.5,
	})
	insertRequestLogForProviderQuotaTest(t, db, providerQuotaLogEntry{
		Platform:   "codex",
		ProviderID: "",
		Provider:   "Legacy Acme",
		CreatedAt:  "2026-02-25 12:00:00",
		TotalCost:  0.5,
	})

	if err := syncProviderIdentityRename("codex", "pid-9", "Legacy Acme", "Renamed Acme"); err != nil {
		t.Fatalf("syncProviderIdentityRename 调用失败: %v", err)
	}

	state, err := queryFiveHourQuotaCycleStateByProvider(db, "codex", "pid-9")
	if err != nil {
		t.Fatalf("queryFiveHourQuotaCycleStateByProvider 调用失败: %v", err)
	}

	expectedStart := time.Date(2026, time.February, 25, 10, 0, 0, 0, time.UTC)
	expectedReset := expectedStart.Add(fiveHourQuotaWindowDuration)
	if !state.WindowStart.Equal(expectedStart) {
		t.Fatalf("期望迁移后的开始时间为 %s，实际 %s", expectedStart.Format(timeLayout), state.WindowStart.Format(timeLayout))
	}
	if !state.NextReset.Equal(expectedReset) {
		t.Fatalf("期望迁移后的重置时间为 %s，实际 %s", expectedReset.Format(timeLayout), state.NextReset.Format(timeLayout))
	}
	if !almostEqualFiveHourQuotaFloat(state.Used, 2.0) {
		t.Fatalf("期望迁移后的已用金额为 2.0，实际 %f", state.Used)
	}

	legacyState, err := queryFiveHourQuotaCycleStateByProvider(db, "codex", "Legacy Acme")
	if err != nil {
		t.Fatalf("查询 legacy provider quota state 失败: %v", err)
	}
	if !legacyState.WindowStart.IsZero() || !legacyState.NextReset.IsZero() || legacyState.Used != 0 {
		t.Fatalf("legacy provider_ref 的 quota state 应已清理，实际 %+v", legacyState)
	}
}

type providerQuotaLogEntry struct {
	Platform   string
	ProviderID string
	Provider   string
	CreatedAt  string
	TotalCost  float64
}

func insertRequestLogForProviderQuotaTest(t *testing.T, db *sql.DB, entry providerQuotaLogEntry) {
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
		"gpt-5-codex",
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
		t.Fatalf("插入 provider quota request_log 失败: %v", err)
	}
}
