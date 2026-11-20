/**
 * @name: Pi 会话用量测试
 * @Descripttion: 验证 Pi 会话 JSONL 的发现、token 提取、耗时估算、子代理与投影去重及增量续读。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_pi_test.go
 */

package services

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// piSessionTestRow request_log 中会话用量行的断言视图
type piSessionTestRow struct {
	Platform       string
	DataSource     string
	Model          string
	SessionID      string
	SourceRecordID string
	Input          int
	Output         int
	CacheCreate    int
	CacheRead      int
	Reasoning      int
	Duration       float64
	FirstToken     float64
}

// piSessionTestRows 读取指定数据来源的全部行（按创建时间、id 排序）
func piSessionTestRows(t *testing.T, db *sql.DB, dataSource string) []piSessionTestRow {
	t.Helper()
	rows, err := db.Query(`
		SELECT platform, data_source, model, COALESCE(session_id, ''), source_record_id,
		       input_tokens, output_tokens, cache_create_tokens, cache_read_tokens, reasoning_tokens,
		       COALESCE(duration_sec, 0), COALESCE(first_token_sec, 0)
		FROM request_log WHERE data_source = ? ORDER BY created_at, id`, dataSource)
	if err != nil {
		t.Fatalf("查询 request_log 失败: %v", err)
	}
	defer rows.Close()
	var result []piSessionTestRow
	for rows.Next() {
		var row piSessionTestRow
		if err := rows.Scan(&row.Platform, &row.DataSource, &row.Model, &row.SessionID, &row.SourceRecordID,
			&row.Input, &row.Output, &row.CacheCreate, &row.CacheRead, &row.Reasoning, &row.Duration, &row.FirstToken); err != nil {
			t.Fatal(err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

// piSessionTestResetCursors 清空文件游标，模拟游标丢失后的全量重扫（用于验证来源 ID 稳定、同步幂等）
func piSessionTestResetCursors(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("DELETE FROM session_log_sync"); err != nil {
		t.Fatalf("清空游标失败: %v", err)
	}
}

// piSessionTestWrite 写入 JSONL 夹具（自动创建父目录）
func piSessionTestWrite(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// piSessionTestAppend 向 JSONL 追加行
func piSessionTestAppend(t *testing.T, path string, lines ...string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		t.Fatal(err)
	}
}

func piSessionTestHeader(id string) string {
	return fmt.Sprintf(`{"type":"session","version":3,"id":%q,"timestamp":"2023-11-14T22:13:20Z","cwd":"/work"}`, id)
}

func piSessionTestAssistant(id string, timestamp string, input int) string {
	return fmt.Sprintf(`{"type":"message","id":%q,"parentId":null,"timestamp":%q,"message":{"role":"assistant","content":[{"type":"text","text":"ok"}],"provider":"fixture-provider","model":"fixture-model","responseId":"reused-response-id","timestamp":1700000000000,"usage":{"input":%d,"output":2,"cacheRead":5,"cacheWrite":2,"totalTokens":999,"cost":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0,"total":0}},"stopReason":"stop"}}`, id, timestamp, input)
}

func piSessionTestAssertSyncOK(t *testing.T, result SessionSyncSourceResult, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("同步返回错误: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("同步结果带错误: %s", result.Error)
	}
}

func TestPiSessionUsageImportsTokensTimingAndResumes(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	path := filepath.Join(home, ".pi", "agent", "sessions", "project-a", "2023-11-14_pi-s1.jsonl")
	piSessionTestWrite(t, path,
		piSessionTestHeader("pi-s1"),
		piSessionTestAssistant("a1", "2023-11-14T22:13:21.500Z", 7),
		// toolResult 带用量但没有模型归属
		`{"type":"message","id":"tr1","parentId":"a1","timestamp":"2023-11-14T22:13:22Z","message":{"role":"toolResult","toolCallId":"c1","toolName":"bash","content":[],"usage":{"input":3,"output":1,"cacheRead":0,"cacheWrite":0}}}`,
		// token 全为 0 的条目不入库
		`{"type":"message","id":"a0","timestamp":"2023-11-14T22:13:23Z","message":{"role":"assistant","model":"fixture-model","usage":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0},"stopReason":"stop"}}`,
	)

	result, err := syncPiSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, result, err)
	if result.Platform != sessionPlatformPi || result.FilesScanned != 1 || result.Imported != 2 {
		t.Fatalf("首次同步结果异常: %+v", result)
	}
	rows := piSessionTestRows(t, db, requestLogDataSourcePiSession)
	if len(rows) != 2 {
		t.Fatalf("行数 = %d, want 2", len(rows))
	}
	assistant := rows[0]
	if assistant.Platform != "pi" || assistant.Model != "fixture-model" || assistant.SessionID != "pi-s1" {
		t.Fatalf("assistant 行归属异常: %+v", assistant)
	}
	if assistant.Input != 7 || assistant.Output != 2 || assistant.CacheRead != 5 || assistant.CacheCreate != 2 {
		t.Fatalf("assistant token 异常: %+v", assistant)
	}
	// message.timestamp(…000) 到条目落盘(…1.5s) 估算耗时 1.5s，Pi 没有首字数据
	if math.Abs(assistant.Duration-1.5) > 0.001 || assistant.FirstToken != 0 {
		t.Fatalf("assistant 计时异常: duration=%v first=%v", assistant.Duration, assistant.FirstToken)
	}
	if tool := rows[1]; tool.Model != "unknown" || tool.Input != 3 || tool.Duration != 0 {
		t.Fatalf("toolResult 行异常: %+v", tool)
	}

	// 追加一行：只增量导入新行（续读依赖游标里保存的会话头上下文）
	piSessionTestAppend(t, path, piSessionTestAssistant("a2", "2023-11-14T22:14:00Z", 11))
	result, err = syncPiSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, result, err)
	if result.Imported != 1 {
		t.Fatalf("追加后导入数 = %d, want 1", result.Imported)
	}
	if rows := piSessionTestRows(t, db, requestLogDataSourcePiSession); len(rows) != 3 || rows[2].SessionID != "pi-s1" {
		t.Fatalf("追加后行异常: %+v", rows)
	}

	// 游标丢失后全量重扫：行数不变、来源 ID 唯一、全部判定为已存在
	piSessionTestResetCursors(t, db)
	result, err = syncPiSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, result, err)
	if result.Imported != 0 || result.Skipped != 3 {
		t.Fatalf("重扫结果异常: %+v", result)
	}
	var total, distinct int
	if err := db.QueryRow("SELECT COUNT(*), COUNT(DISTINCT source_record_id) FROM request_log WHERE data_source = ?", requestLogDataSourcePiSession).Scan(&total, &distinct); err != nil {
		t.Fatal(err)
	}
	if total != 3 || distinct != 3 {
		t.Fatalf("重扫后 total=%d distinct=%d, want 3/3", total, distinct)
	}
}

func TestPiSessionUsageSubagentToolResultsAndProjectionDedup(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	sessions := filepath.Join(home, ".pi", "agent", "sessions", "project-a")
	parent := filepath.Join(sessions, "parent.jsonl")
	child := filepath.Join(sessions, "parent", "run-0", "session.jsonl")

	// 官方 subagent 扩展：子代理用量只在父会话 details.results 里（逐请求 + 仅汇总各一个）
	subagent := `{"type":"message","id":"sub1","parentId":null,"timestamp":"2023-11-14T22:20:00Z","message":{"role":"toolResult","toolCallId":"c1","toolName":"subagent","content":[],"details":{"mode":"parallel","results":[` +
		`{"model":"m-a","usage":{"input":999,"output":999},"messages":[{"role":"assistant","model":"child-model","timestamp":"2023-11-14T22:15:00Z","usage":{"input":40,"output":4,"cacheRead":6,"cacheWrite":1}}]},` +
		`{"model":"m-summary","usage":{"input":50,"output":5,"cacheRead":0,"cacheWrite":0,"cost":0.01}}]}}}`
	// pi-subagents 投影：汇总了所链接子会话的用量，子会话已导入，投影本身必须跳过
	projection := fmt.Sprintf(`{"type":"message","id":"proj1","parentId":"sub1","timestamp":"2023-11-14T22:21:00Z","message":{"role":"toolResult","toolCallId":"c2","toolName":"delegate","content":[],"details":{"results":[{"sessionFile":%q}]},"usage":{"input":500,"output":50,"cacheRead":0,"cacheWrite":0}}}`, child)
	piSessionTestWrite(t, parent,
		piSessionTestHeader("parent"),
		piSessionTestAssistant("p1", "2023-11-14T22:13:21Z", 7),
		subagent,
		projection,
	)
	piSessionTestWrite(t, child,
		`{"type":"session","version":3,"id":"child","timestamp":"2023-11-14T22:14:00Z","cwd":"/work","parentSession":"parent"}`,
		piSessionTestAssistant("c1", "2023-11-14T22:14:30Z", 500),
	)
	// subagent-artifacts 没有会话头，不应被当作会话
	piSessionTestWrite(t, filepath.Join(sessions, "parent", "subagent-artifacts", "transcript.jsonl"), `{"type":"note"}`)

	result, err := syncPiSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, result, err)
	if result.FilesScanned != 2 {
		t.Fatalf("FilesScanned = %d, want 2（父会话 + 子会话）", result.FilesScanned)
	}
	rows := piSessionTestRows(t, db, requestLogDataSourcePiSession)
	// 父 assistant、子代理逐请求、子代理汇总、子会话 assistant；投影被跳过
	if len(rows) != 4 {
		t.Fatalf("行数 = %d, want 4: %+v", len(rows), rows)
	}
	var inputSum int
	models := map[string]int{}
	for _, row := range rows {
		inputSum += row.Input
		models[row.Model]++
	}
	if inputSum != 7+40+50+500 {
		t.Fatalf("input 合计 = %d, want %d", inputSum, 7+40+50+500)
	}
	if models["child-model"] != 1 || models["m-summary"] != 1 || models["fixture-model"] != 2 {
		t.Fatalf("模型分布异常: %v", models)
	}
}

func TestPiSessionUsageNativeSessionDirOverrideAndSharedRoot(t *testing.T) {
	ls, db, _ := prepareSessionUsageTest(t)
	home := t.TempDir()
	sessionDir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_SESSION_DIR", sessionDir)
	t.Setenv("PI_CODING_AGENT_DIR", "")

	piSessionTestWrite(t, filepath.Join(sessionDir, "pi.jsonl"),
		piSessionTestHeader("pi-flat"),
		piSessionTestAssistant("a1", "2023-11-14T22:13:21Z", 9),
	)
	// Oh My Pi 的会话（标题槽开头）与 Pi 共用目录时，Pi 不应收下
	piSessionTestWrite(t, filepath.Join(sessionDir, "omp.jsonl"),
		`{"type":"title","v":1,"title":"","updatedAt":"2026-08-22T02:12:51.066Z","pad":""}`,
		`{"type":"session","version":3,"id":"omp-flat","timestamp":"2026-08-22T02:12:51.066Z","cwd":"/work"}`,
		piSessionTestAssistant("o1", "2026-08-22T02:13:00Z", 100),
	)

	result, err := syncPiSessionUsage(ls, "native", home)
	piSessionTestAssertSyncOK(t, result, err)
	if result.FilesScanned != 1 || result.Imported != 1 {
		t.Fatalf("共享目录下 Pi 结果异常: %+v", result)
	}
	rows := piSessionTestRows(t, db, requestLogDataSourcePiSession)
	if len(rows) != 1 || rows[0].SessionID != "pi-flat" || rows[0].Input != 9 {
		t.Fatalf("Pi 行异常: %+v", rows)
	}

	// 非 native 范围（WSL）不读取环境变量，默认路径下什么都没有
	empty, err := syncPiSessionUsage(ls, "wsl:test", home)
	piSessionTestAssertSyncOK(t, empty, err)
	if empty.FilesScanned != 0 || empty.Imported != 0 {
		t.Fatalf("非 native 范围不应读取环境变量: %+v", empty)
	}
}

func TestPiSessionUsageMissingDirectoryIsEmptyResult(t *testing.T) {
	ls, _, scope := prepareSessionUsageTest(t)
	result, err := syncPiSessionUsage(ls, scope, t.TempDir())
	piSessionTestAssertSyncOK(t, result, err)
	if result.FilesScanned != 0 || result.Imported != 0 || result.Platform != sessionPlatformPi {
		t.Fatalf("目录缺失应返回空结果: %+v", result)
	}
}

func TestPiSessionUsageRootFromSettingsSessionDir(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	custom := filepath.Join(home, "pi-sessions")
	piSessionTestWrite(t, filepath.Join(custom, "s.jsonl"),
		piSessionTestHeader("custom-1"),
		piSessionTestAssistant("a1", "2023-11-14T22:13:21Z", 4),
	)
	settings := filepath.Join(home, ".pi", "agent", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte(`{"sessionDir":"~/pi-sessions"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := syncPiSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, result, err)
	if rows := piSessionTestRows(t, db, requestLogDataSourcePiSession); len(rows) != 1 || rows[0].Input != 4 {
		t.Fatalf("settings.json sessionDir 未生效: %+v", rows)
	}
}
