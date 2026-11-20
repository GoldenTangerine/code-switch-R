/**
 * @name: MiniMax Code 会话用量测试
 * @Descripttion: 验证 MiniMax Code 运行时库的 token 映射、模型解析、消息行计时回填、水位及裁剪行不复活。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_mcode_test.go
 */

package services

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mcodeTestOpen 在假 home 下创建 MiniMax Code 运行时库并执行建表 / 初始数据语句
func mcodeTestOpen(t *testing.T, home string, statements ...string) *sql.DB {
	t.Helper()
	path := mcodeDatabasePath(home, "test")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	source, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = source.Close() })
	for _, statement := range statements {
		if _, err := source.Exec(statement); err != nil {
			t.Fatalf("执行夹具语句失败: %v\n%s", err, statement)
		}
	}
	return source
}

const mcodeTestUsageTable = `CREATE TABLE local_runtime_token_usage (
	id INTEGER PRIMARY KEY, session_id TEXT, turn_id TEXT, model TEXT, ts INTEGER,
	input_tokens INTEGER, output_tokens INTEGER, reasoning_tokens INTEGER,
	cache_read_tokens INTEGER, cache_write_tokens INTEGER, cost_usd REAL)`

func TestMiniMaxCodeSessionUsageMapsTokensAndModelNamespaces(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	mcodeTestOpen(t, home, mcodeTestUsageTable,
		`INSERT INTO local_runtime_token_usage VALUES
		 (1,'s1','t1','custom_provider:router/vendor/model',100000,10,20,3,40,5,0),
		 (2,'s1','t2','plain-model',101000,1,2,0,0,0,0.25),
		 (3,'s2','t1','custom_provider:router/another/model',102000,1,2,0,0,0,NULL)`)

	result, err := syncMiniMaxCodeSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, result, err)
	if result.Platform != sessionPlatformMiniMaxCode || result.FilesScanned != 1 || result.Imported != 3 {
		t.Fatalf("首次同步结果异常: %+v", result)
	}
	rows := piSessionTestRows(t, db, requestLogDataSourceMiniMaxCodeSession)
	if len(rows) != 3 {
		t.Fatalf("行数 = %d, want 3", len(rows))
	}
	first := rows[0]
	if first.Platform != "minimax-code" || first.Model != "vendor/model" || first.SessionID != "s1" {
		t.Fatalf("第一行归属异常: %+v", first)
	}
	// 推理 token 单独成列，输出不含推理；缓存读写分别映射
	if first.Input != 10 || first.Output != 20 || first.Reasoning != 3 || first.CacheRead != 40 || first.CacheCreate != 5 {
		t.Fatalf("第一行 token 异常: %+v", first)
	}
	if rows[1].Model != "plain-model" || rows[2].Model != "another/model" {
		t.Fatalf("模型命名空间处理异常: %q / %q", rows[1].Model, rows[2].Model)
	}

	// 同一 session 下的不同用量行 id 生成不同来源 ID；水位使未变化的库整体跳过
	again, err := syncMiniMaxCodeSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, again, err)
	if again.Imported != 0 {
		t.Fatalf("库未变化不应再导入: %+v", again)
	}

	// 游标丢失后重扫：幂等
	piSessionTestResetCursors(t, db)
	rescan, err := syncMiniMaxCodeSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, rescan, err)
	if rescan.Imported != 0 || rescan.Skipped != 3 {
		t.Fatalf("重扫结果异常: %+v", rescan)
	}
	var total, distinct int
	if err := db.QueryRow("SELECT COUNT(*), COUNT(DISTINCT source_record_id) FROM request_log WHERE data_source = ?", requestLogDataSourceMiniMaxCodeSession).Scan(&total, &distinct); err != nil {
		t.Fatal(err)
	}
	if total != 3 || distinct != 3 {
		t.Fatalf("重扫后 total=%d distinct=%d, want 3/3", total, distinct)
	}
}

func TestMiniMaxCodeSessionUsageRecoversMissingModelFromHistory(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	mcodeTestOpen(t, home, mcodeTestUsageTable,
		`CREATE TABLE local_runtime_pi_history_rows (id INTEGER PRIMARY KEY, session_id TEXT, role TEXT, created_at_ms INTEGER, data_json TEXT)`,
		`CREATE TABLE local_runtime_messages (session_id TEXT PRIMARY KEY, display_messages_json TEXT, pi_history_json TEXT)`,
		`INSERT INTO local_runtime_pi_history_rows VALUES
		 (1,'s1','user',100000,'{"role":"user","timestamp":100000}'),
		 (2,'s1','assistant',100000,'{"role":"assistant","provider":"minimax","model":"MiniMax-M3","timestamp":100000}')`,
		`INSERT INTO local_runtime_messages VALUES ('s2','[]',
		 '[{"role":"user","timestamp":200000},
		   {"role":"assistant","model":"vendor/legacy-model","timestamp":200000},
		   {"role":"assistant","model":"legacy-next","timestamp":250000}]')`,
		`INSERT INTO local_runtime_token_usage VALUES
		 (1,'s1','t1',NULL,100000,1,2,0,0,0,NULL),
		 (2,'s2','t1',NULL,200000,1,2,0,0,0,NULL),
		 (3,'s2','t2',NULL,250000,1,2,0,0,0,NULL),
		 (4,'s1','t2',NULL,300000,1,2,0,0,0,NULL)`)

	result, err := syncMiniMaxCodeSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, result, err)
	if result.Imported != 4 {
		t.Fatalf("导入数 = %d, want 4", result.Imported)
	}
	rows := piSessionTestRows(t, db, requestLogDataSourceMiniMaxCodeSession)
	var models []string
	for _, row := range rows {
		models = append(models, row.Model)
	}
	// 旧版 JSON 里的模型名原样保留（不剥命名空间）；查不到仍记 unknown
	want := []string{"MiniMax-M3", "vendor/legacy-model", "legacy-next", "unknown"}
	if fmt.Sprint(models) != fmt.Sprint(want) {
		t.Fatalf("模型 = %v, want %v", models, want)
	}
}

func TestMiniMaxCodeSessionUsageTimingFromMessageRowsAndLateBackfill(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	now := time.Now().UnixMilli()
	old := now - 3_600_000
	source := mcodeTestOpen(t, home, mcodeTestUsageTable,
		`CREATE TABLE local_runtime_message_rows (id INTEGER PRIMARY KEY, session_id TEXT, role TEXT, turn_id TEXT, created_at_ms INTEGER, data_json TEXT)`)
	usage := func(id int64, turn string, ts int64, input int, output int) {
		t.Helper()
		if _, err := source.Exec(`INSERT INTO local_runtime_token_usage VALUES (?,'s1',?,'custom_provider:p/k3',?,?,?,0,0,0,0)`, id, turn, ts, input, output); err != nil {
			t.Fatal(err)
		}
	}
	message := func(turn string, end int64, input int, output int, durations string) {
		t.Helper()
		data := fmt.Sprintf(`{"usage":{"input_tokens":%d,"output_tokens":%d%s}}`, input, output, durations)
		if _, err := source.Exec(`INSERT INTO local_runtime_message_rows (session_id,role,turn_id,created_at_ms,data_json) VALUES ('s1','assistant',?,?,?)`, turn, end, data); err != nil {
			t.Fatal(err)
		}
	}
	// 数值取自真实 mcode exec 运行：一次长回复，随后一次工具调用轮
	usage(1, "t1", old, 12828, 384)
	message("t1", old+16_610, 12828, 384, `,"decode_duration_ms":14595,"request_duration_ms":16616`)
	usage(2, "t2", old+20_000, 797, 62)
	// 另一轮里 token 相同的消息不能被误选
	message("t9", old+21_000, 797, 62, `,"decode_duration_ms":1,"request_duration_ms":1000`)
	message("t2", old+28_880, 797, 62, `,"decode_duration_ms":797,"request_duration_ms":8880`)
	// 没有记录时长（旧版本）：无计时，且视为已结算
	usage(3, "t3", old+40_000, 354, 160)
	message("t3", old+43_000, 354, 160, "")
	// 最近的行，消息行还没写出
	usage(4, "t4", now, 238, 130)

	result, err := syncMiniMaxCodeSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, result, err)
	if result.Imported != 4 {
		t.Fatalf("导入数 = %d, want 4", result.Imported)
	}
	check := func(rows []piSessionTestRow, index int, duration float64, firstToken float64) {
		t.Helper()
		if math.Abs(rows[index].Duration-duration) > 0.0005 || math.Abs(rows[index].FirstToken-firstToken) > 0.0005 {
			t.Fatalf("第 %d 行计时 = (%v, %v), want (%v, %v)", index, rows[index].Duration, rows[index].FirstToken, duration, firstToken)
		}
	}
	rows := piSessionTestRows(t, db, requestLogDataSourceMiniMaxCodeSession)
	if len(rows) != 4 {
		t.Fatalf("行数 = %d, want 4", len(rows))
	}
	check(rows, 0, 16.616, 2.021) // 首字 = 请求时长 - 解码时长
	check(rows, 1, 8.880, 8.083)
	check(rows, 2, 0, 0)
	check(rows, 3, 0, 0)

	// 用户裁剪掉旧明细：之后新增数据触发再次同步，被裁剪的行不能复活
	if _, err := db.Exec("DELETE FROM request_log WHERE data_source = ? AND session_id = 's1' AND input_tokens = 12828", requestLogDataSourceMiniMaxCodeSession); err != nil {
		t.Fatal(err)
	}
	// 迟到的消息行：下一次同步只回填计时，不重复导入
	message("t4", now+14_250, 238, 130, `,"decode_duration_ms":1626,"request_duration_ms":14250`)
	again, err := syncMiniMaxCodeSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, again, err)
	rows = piSessionTestRows(t, db, requestLogDataSourceMiniMaxCodeSession)
	if len(rows) != 3 {
		t.Fatalf("被裁剪的行被重新导入: %d 行", len(rows))
	}
	check(rows, 2, 14.250, 12.624)
	if again.Imported != 1 {
		t.Fatalf("回填计时应只更新 1 行: %+v", again)
	}
}

func TestMiniMaxCodeSessionUsageNewRowsAfterWatermarkAndMissingDatabase(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()

	missing, err := syncMiniMaxCodeSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, missing, err)
	if missing.FilesScanned != 0 || missing.Imported != 0 || missing.Platform != sessionPlatformMiniMaxCode {
		t.Fatalf("库缺失应返回空结果: %+v", missing)
	}

	source := mcodeTestOpen(t, home, mcodeTestUsageTable,
		`INSERT INTO local_runtime_token_usage VALUES (1,'s1','t1','custom_provider:test/model',100000,10,20,0,0,0,0)`)
	if result, err := syncMiniMaxCodeSessionUsage(ls, scope, home); err != nil || result.Imported != 1 {
		t.Fatalf("首次同步 = %+v err=%v", result, err)
	}
	// 用户清掉明细后再次同步不能复活旧行，只导入水位之后的新行
	if _, err := db.Exec("DELETE FROM request_log WHERE data_source = ?", requestLogDataSourceMiniMaxCodeSession); err != nil {
		t.Fatal(err)
	}
	if _, err := source.Exec(`INSERT INTO local_runtime_token_usage VALUES (2,'s1','t2','custom_provider:test/model',101000,1,2,0,0,0,0.25)`); err != nil {
		t.Fatal(err)
	}
	result, err := syncMiniMaxCodeSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, result, err)
	if result.Imported != 1 {
		t.Fatalf("水位之后应只导入 1 行: %+v", result)
	}
	rows := piSessionTestRows(t, db, requestLogDataSourceMiniMaxCodeSession)
	if len(rows) != 1 || rows[0].Input != 1 || rows[0].Model != "model" {
		t.Fatalf("行异常: %+v", rows)
	}
}

func TestMiniMaxCodeSessionUsageDataDirEnvOverride(t *testing.T) {
	home := t.TempDir()
	override := t.TempDir()
	for _, key := range []string{"MINIMAX_DATA_DIR", "MAVIS_DATA_DIR"} {
		t.Setenv(key, "")
	}
	if got := mcodeDatabasePath(home, "native"); got != filepath.Join(home, ".minimax", "v2", "sqlite", "runtime-state.sqlite") {
		t.Fatalf("默认路径异常: %s", got)
	}
	t.Setenv("MAVIS_DATA_DIR", override)
	if got := mcodeDatabasePath(home, "native"); got != filepath.Join(override, "v2", "sqlite", "runtime-state.sqlite") {
		t.Fatalf("MAVIS_DATA_DIR 未生效: %s", got)
	}
	otherOverride := t.TempDir()
	t.Setenv("MINIMAX_DATA_DIR", otherOverride)
	if got := mcodeDatabasePath(home, "native"); got != filepath.Join(otherOverride, "v2", "sqlite", "runtime-state.sqlite") {
		t.Fatalf("MINIMAX_DATA_DIR 应优先: %s", got)
	}
	if got := mcodeDatabasePath(home, "wsl:Ubuntu"); got != filepath.Join(home, ".minimax", "v2", "sqlite", "runtime-state.sqlite") {
		t.Fatalf("WSL 范围应忽略环境变量: %s", got)
	}
}
