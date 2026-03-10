package services

import (
	"database/sql"
	"math"
	"testing"
	"time"

	"github.com/daodao97/xgo/xdb"
)

func TestResolveFiveHourQuotaStatusAt_ReturnsActiveCycle(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForFiveHourQuotaTest(t, db, fiveHourQuotaLogEntry{
		Platform:  "codex",
		CreatedAt: "2026-02-25 10:00:00",
		TotalCost: 1.2,
	})
	insertRequestLogForFiveHourQuotaTest(t, db, fiveHourQuotaLogEntry{
		Platform:  "codex",
		CreatedAt: "2026-02-25 12:30:00",
		TotalCost: 0.8,
	})

	ls := NewLogService(nil)
	now := time.Date(2026, time.February, 25, 13, 15, 0, 0, time.UTC)
	status, err := ls.resolveFiveHourQuotaStatusAt("codex", now)
	if err != nil {
		t.Fatalf("resolveFiveHourQuotaStatusAt 调用失败: %v", err)
	}

	if !status.Active {
		t.Fatalf("期望存在活跃的 5 小时周期")
	}

	expectedStart := time.Date(2026, time.February, 25, 10, 0, 0, 0, time.UTC)
	expectedReset := expectedStart.Add(fiveHourQuotaWindowDuration)
	if status.WindowStart != expectedStart.In(time.Local).Format(timeLayout) {
		t.Fatalf("期望窗口开始时间 %s，实际 %s", expectedStart.In(time.Local).Format(timeLayout), status.WindowStart)
	}
	if status.NextReset != expectedReset.In(time.Local).Format(timeLayout) {
		t.Fatalf("期望重置时间 %s，实际 %s", expectedReset.In(time.Local).Format(timeLayout), status.NextReset)
	}
	if !almostEqualFiveHourQuotaFloat(status.Used, 2.0) {
		t.Fatalf("期望已用金额 2.0，实际 %f", status.Used)
	}
}

func TestResolveFiveHourQuotaStatusAt_BecomesInactiveAfterCycleExpiresWithoutNewRequest(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForFiveHourQuotaTest(t, db, fiveHourQuotaLogEntry{
		Platform:  "codex",
		CreatedAt: "2026-02-25 10:00:00",
		TotalCost: 1.2,
	})
	insertRequestLogForFiveHourQuotaTest(t, db, fiveHourQuotaLogEntry{
		Platform:  "codex",
		CreatedAt: "2026-02-25 14:30:00",
		TotalCost: 0.8,
	})

	ls := NewLogService(nil)
	now := time.Date(2026, time.February, 25, 15, 1, 0, 0, time.UTC)
	status, err := ls.resolveFiveHourQuotaStatusAt("codex", now)
	if err != nil {
		t.Fatalf("resolveFiveHourQuotaStatusAt 调用失败: %v", err)
	}

	if status.Active {
		t.Fatalf("周期已过期且没有新请求时，不应继续保持活跃状态")
	}
	if status.WindowStart != "" {
		t.Fatalf("非活跃周期不应返回窗口开始时间，实际 %s", status.WindowStart)
	}
	if status.NextReset != "" {
		t.Fatalf("非活跃周期不应返回重置时间，实际 %s", status.NextReset)
	}
	if status.Used != 0 {
		t.Fatalf("非活跃周期的已用金额应为 0，实际 %f", status.Used)
	}
}

func TestResolveFiveHourQuotaStatusAt_StartsNewCycleOnFirstRequestAfterReset(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForFiveHourQuotaTest(t, db, fiveHourQuotaLogEntry{
		Platform:  "codex",
		CreatedAt: "2026-02-25 10:00:00",
		TotalCost: 1.0,
	})
	insertRequestLogForFiveHourQuotaTest(t, db, fiveHourQuotaLogEntry{
		Platform:  "codex",
		CreatedAt: "2026-02-25 14:59:00",
		TotalCost: 0.5,
	})
	insertRequestLogForFiveHourQuotaTest(t, db, fiveHourQuotaLogEntry{
		Platform:  "codex",
		CreatedAt: "2026-02-25 15:10:00",
		TotalCost: 2.0,
	})

	ls := NewLogService(nil)
	now := time.Date(2026, time.February, 25, 15, 30, 0, 0, time.UTC)
	status, err := ls.resolveFiveHourQuotaStatusAt("codex", now)
	if err != nil {
		t.Fatalf("resolveFiveHourQuotaStatusAt 调用失败: %v", err)
	}

	if !status.Active {
		t.Fatalf("首个跨边界新请求后应开启新周期")
	}

	expectedStart := time.Date(2026, time.February, 25, 15, 10, 0, 0, time.UTC)
	expectedReset := expectedStart.Add(fiveHourQuotaWindowDuration)
	if status.WindowStart != expectedStart.In(time.Local).Format(timeLayout) {
		t.Fatalf("期望新周期开始时间 %s，实际 %s", expectedStart.In(time.Local).Format(timeLayout), status.WindowStart)
	}
	if status.NextReset != expectedReset.In(time.Local).Format(timeLayout) {
		t.Fatalf("期望新周期重置时间 %s，实际 %s", expectedReset.In(time.Local).Format(timeLayout), status.NextReset)
	}
	if !almostEqualFiveHourQuotaFloat(status.Used, 2.0) {
		t.Fatalf("期望新周期只统计 2.0，实际 %f", status.Used)
	}
}

func TestResolveFiveHourQuotaStatusAt_KeepsPersistedCycleAfterDeletingHistoryRows(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	insertRequestLogForFiveHourQuotaTest(t, db, fiveHourQuotaLogEntry{
		Platform:  "codex",
		CreatedAt: "2026-02-25 10:00:00",
		TotalCost: 1.2,
	})
	insertRequestLogForFiveHourQuotaTest(t, db, fiveHourQuotaLogEntry{
		Platform:  "codex",
		CreatedAt: "2026-02-25 12:30:00",
		TotalCost: 0.8,
	})

	if _, err := db.Exec(`DELETE FROM request_log WHERE platform = ? AND created_at = ?`, "codex", "2026-02-25 10:00:00"); err != nil {
		t.Fatalf("删除 request_log 失败: %v", err)
	}

	ls := NewLogService(nil)
	now := time.Date(2026, time.February, 25, 13, 15, 0, 0, time.UTC)
	status, err := ls.resolveFiveHourQuotaStatusAt("codex", now)
	if err != nil {
		t.Fatalf("resolveFiveHourQuotaStatusAt 调用失败: %v", err)
	}

	if !status.Active {
		t.Fatalf("删除历史日志后，当前 5 小时周期仍应保持活跃")
	}
	expectedStart := time.Date(2026, time.February, 25, 10, 0, 0, 0, time.UTC)
	if status.WindowStart != expectedStart.In(time.Local).Format(timeLayout) {
		t.Fatalf("期望周期开始时间保持为 %s，实际 %s", expectedStart.In(time.Local).Format(timeLayout), status.WindowStart)
	}
	if !almostEqualFiveHourQuotaFloat(status.Used, 2.0) {
		t.Fatalf("期望已用金额保持为 2.0，实际 %f", status.Used)
	}
}

func TestBackfillRequestLogQuotaCycleStateWithDB_RebuildsCurrentCycleFromLegacyLogs(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	if _, err := db.Exec(`DROP TRIGGER IF EXISTS request_log_quota_cycle_ai`); err != nil {
		t.Fatalf("删除 quota cycle trigger 失败: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM request_log_quota_cycle_state`); err != nil {
		t.Fatalf("清空 quota cycle state 失败: %v", err)
	}

	insertRequestLogForFiveHourQuotaTest(t, db, fiveHourQuotaLogEntry{
		Platform:  "codex",
		CreatedAt: "2026-02-25 10:00:00",
		TotalCost: 1.0,
	})
	insertRequestLogForFiveHourQuotaTest(t, db, fiveHourQuotaLogEntry{
		Platform:  "codex",
		CreatedAt: "2026-02-25 14:59:00",
		TotalCost: 0.5,
	})
	insertRequestLogForFiveHourQuotaTest(t, db, fiveHourQuotaLogEntry{
		Platform:  "codex",
		CreatedAt: "2026-02-25 15:10:00",
		TotalCost: 2.0,
	})

	if err := backfillRequestLogQuotaCycleStateWithDB(db); err != nil {
		t.Fatalf("backfillRequestLogQuotaCycleStateWithDB 调用失败: %v", err)
	}

	state, err := queryFiveHourQuotaCycleState(db, "codex")
	if err != nil {
		t.Fatalf("queryFiveHourQuotaCycleState 调用失败: %v", err)
	}

	expectedStart := time.Date(2026, time.February, 25, 15, 10, 0, 0, time.UTC)
	expectedReset := expectedStart.Add(fiveHourQuotaWindowDuration)
	if !state.WindowStart.Equal(expectedStart) {
		t.Fatalf("期望回填后的开始时间 %s，实际 %s", expectedStart.Format(timeLayout), state.WindowStart.Format(timeLayout))
	}
	if !state.NextReset.Equal(expectedReset) {
		t.Fatalf("期望回填后的重置时间 %s，实际 %s", expectedReset.Format(timeLayout), state.NextReset.Format(timeLayout))
	}
	if !almostEqualFiveHourQuotaFloat(state.Used, 2.0) {
		t.Fatalf("期望回填后的已用金额为 2.0，实际 %f", state.Used)
	}
}

type fiveHourQuotaLogEntry struct {
	Platform  string
	CreatedAt string
	TotalCost float64
}

func insertRequestLogForFiveHourQuotaTest(t *testing.T, db *sql.DB, entry fiveHourQuotaLogEntry) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO request_log (
			platform,
			model,
			provider,
			http_code,
			input_tokens,
			output_tokens,
			cache_create_tokens,
			cache_read_tokens,
			reasoning_tokens,
			total_cost,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		entry.Platform,
		"gpt-5-codex",
		"Acme",
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

func almostEqualFiveHourQuotaFloat(actual float64, expected float64) bool {
	return math.Abs(actual-expected) < 1e-9
}
