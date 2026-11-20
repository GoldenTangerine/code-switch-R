/**
 * @name: Oh My Pi 会话用量测试
 * @Descripttion: 验证 Oh My Pi 会话的标题槽、model_usage、实测耗时与首字、子代理会话及数据目录解析。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_omp_test.go
 */

package services

import (
	"math"
	"path/filepath"
	"strconv"
	"testing"
)

const ompTestTitleSlot = `{"type":"title","v":1,"title":"Fix the parser","updatedAt":"2026-10-29T13:17:48.750Z","pad":""}`

func ompTestHeader(id string, timestamp string) string {
	return `{"type":"session","version":3,"id":"` + id + `","timestamp":"` + timestamp + `","cwd":"/work"}`
}

func TestOhMyPiSessionUsageImportsOwnStoreCarriersAndTiming(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	path := filepath.Join(home, ".omp", "agent", "sessions", "project-a", "omp-session.jsonl")
	piSessionTestWrite(t, path,
		ompTestTitleSlot,
		ompTestHeader("01a187ce-171e-7335-946a-3114ab4dcbe7", "2026-10-29T13:16:32.926Z"),
		`{"type":"message","id":"call-1","parentId":null,"timestamp":"2026-10-29T13:17:42.984Z","message":{"role":"assistant","content":[{"type":"text","text":"ok"}],"api":"openai-completions","provider":"workbuddy","model":"deepseek-v4.1-flash","responseId":"chatcmpl-1","duration":6744.172099999996,"ttft":6192.651100000003,"usage":{"input":282,"output":194,"cacheRead":12800,"cacheWrite":0,"totalTokens":13276,"reasoningTokens":41,"cost":{"input":0.0000846,"output":0.0002328,"cacheRead":0.0000768,"cacheWrite":0,"total":0.0003942}},"stopReason":"toolUse"}}`,
		// model_usage 的请求元数据在条目顶层，同样要归属到模型
		`{"type":"model_usage","id":"judge-1","parentId":"call-1","timestamp":"2026-10-29T13:26:53.160Z","purpose":"job-widget","api":"openai-completions","provider":"workbuddy","model":"deepseek-v4.1-flash","usage":{"input":312,"output":110,"cacheRead":2432,"cacheWrite":0,"totalTokens":2854},"stopReason":"toolUse"}`,
		// 被中止且无 token 的请求不入库
		`{"type":"message","id":"call-2","parentId":"call-1","timestamp":"2026-10-29T13:28:00.000Z","message":{"role":"assistant","content":[],"provider":"workbuddy","model":"deepseek-v4.1-flash","usage":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0,"totalTokens":0},"stopReason":"aborted","errorMessage":"Request was aborted"}}`,
		// compaction 没有 provider / model，落到 unknown
		`{"type":"compaction","id":"compact-1","parentId":"call-2","timestamp":"2026-10-29T13:29:00.000Z","summary":"summary","usage":{"input":11,"output":12,"cacheRead":2,"cacheWrite":3,"totalTokens":28}}`,
	)

	result, err := syncOhMyPiSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, result, err)
	if result.Platform != sessionPlatformOhMyPi || result.FilesScanned != 1 || result.Imported != 3 {
		t.Fatalf("同步结果异常: %+v", result)
	}
	rows := piSessionTestRows(t, db, requestLogDataSourceOhMyPiSession)
	if len(rows) != 3 {
		t.Fatalf("行数 = %d, want 3", len(rows))
	}
	for _, row := range rows {
		if row.Platform != "omp" || row.SessionID != "01a187ce-171e-7335-946a-3114ab4dcbe7" {
			t.Fatalf("行归属异常: %+v", row)
		}
	}
	call := rows[0]
	if call.Model != "deepseek-v4.1-flash" || call.Input != 282 || call.Output != 194 || call.CacheRead != 12800 || call.CacheCreate != 0 {
		t.Fatalf("assistant 行异常: %+v", call)
	}
	// 实测 duration 6744.17ms、ttft 6192.65ms 取整到毫秒
	if math.Abs(call.Duration-6.744) > 0.0005 || math.Abs(call.FirstToken-6.193) > 0.0005 {
		t.Fatalf("assistant 计时异常: duration=%v first=%v", call.Duration, call.FirstToken)
	}
	if judge := rows[1]; judge.Model != "deepseek-v4.1-flash" || judge.Input != 312 || judge.CacheRead != 2432 || judge.Duration != 0 {
		t.Fatalf("model_usage 行异常: %+v", judge)
	}
	if compaction := rows[2]; compaction.Model != "unknown" || compaction.Input != 11 || compaction.Output != 12 || compaction.CacheRead != 2 || compaction.CacheCreate != 3 {
		t.Fatalf("compaction 行异常: %+v", compaction)
	}

	// 游标丢失后重扫：幂等
	piSessionTestResetCursors(t, db)
	again, err := syncOhMyPiSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, again, err)
	if again.Imported != 0 || again.Skipped != 3 {
		t.Fatalf("重扫结果异常: %+v", again)
	}
	var total, distinct int
	if err := db.QueryRow("SELECT COUNT(*), COUNT(DISTINCT source_record_id) FROM request_log WHERE data_source = ?", requestLogDataSourceOhMyPiSession).Scan(&total, &distinct); err != nil {
		t.Fatal(err)
	}
	if total != 3 || distinct != 3 {
		t.Fatalf("重扫后 total=%d distinct=%d, want 3/3", total, distinct)
	}
}

func TestOhMyPiSessionUsageSubagentSessionsAlwaysCount(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	sessions := filepath.Join(home, ".omp", "agent", "sessions", "project-a")
	parent := filepath.Join(sessions, "2026-08-22T02-12-51-066Z_parent.jsonl")
	child := filepath.Join(sessions, "2026-08-22T02-12-51-066Z_parent", "PageScout.jsonl")
	assistant := func(id string, timestamp string, input int) string {
		return `{"type":"message","id":"` + id + `","parentId":null,"timestamp":"` + timestamp + `","message":{"role":"assistant","content":[],"provider":"workbuddy","model":"deepseek-v4.1-flash","usage":{"input":` + strconv.Itoa(input) + `,"output":1,"cacheRead":0,"cacheWrite":0,"totalTokens":1},"stopReason":"stop"}}`
	}
	piSessionTestWrite(t, parent,
		ompTestTitleSlot,
		ompTestHeader("parent", "2026-08-22T02:12:51.066Z"),
		assistant("a1", "2026-08-22T02:13:00.000Z", 100),
		// task 的 toolResult 只在 details 里带用量，不是消息用量，不能读取
		`{"type":"message","id":"task-1","parentId":"a1","timestamp":"2026-08-22T02:20:00.000Z","message":{"role":"toolResult","toolCallId":"call","toolName":"task","content":[],"details":{"usage":{"input":500,"output":1,"cacheRead":0,"cacheWrite":0}}}}`,
	)
	piSessionTestWrite(t, child,
		ompTestTitleSlot,
		`{"type":"session","version":3,"id":"child","timestamp":"2026-08-22T02:14:00.000Z","cwd":"/work","parentSession":"parent"}`,
		assistant("c1", "2026-08-22T02:15:00.000Z", 500),
	)

	result, err := syncOhMyPiSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, result, err)
	if result.FilesScanned != 2 || result.Imported != 2 {
		t.Fatalf("同步结果异常: %+v", result)
	}
	var input int
	if err := db.QueryRow("SELECT SUM(input_tokens) FROM request_log WHERE data_source = ?", requestLogDataSourceOhMyPiSession).Scan(&input); err != nil {
		t.Fatal(err)
	}
	if input != 600 {
		t.Fatalf("input 合计 = %d, want 600", input)
	}
}

func TestOhMyPiSessionUsageSharedRootKeepsOnlyTitleSlotFiles(t *testing.T) {
	ls, db, _ := prepareSessionUsageTest(t)
	home := t.TempDir()
	sessionDir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_SESSION_DIR", sessionDir)

	piSessionTestWrite(t, filepath.Join(sessionDir, "omp.jsonl"),
		ompTestTitleSlot,
		ompTestHeader("omp-1", "2026-10-29T13:16:32.926Z"),
		piSessionTestAssistant("o1", "2026-10-29T13:17:00Z", 21),
	)
	piSessionTestWrite(t, filepath.Join(sessionDir, "pi.jsonl"),
		piSessionTestHeader("pi-1"),
		piSessionTestAssistant("p1", "2023-11-14T22:13:21Z", 5),
	)

	result, err := syncOhMyPiSessionUsage(ls, "native", home)
	piSessionTestAssertSyncOK(t, result, err)
	if result.FilesScanned != 1 || result.Imported != 1 {
		t.Fatalf("共享目录下 Oh My Pi 结果异常: %+v", result)
	}
	rows := piSessionTestRows(t, db, requestLogDataSourceOhMyPiSession)
	if len(rows) != 1 || rows[0].SessionID != "omp-1" || rows[0].Input != 21 {
		t.Fatalf("Oh My Pi 行异常: %+v", rows)
	}
	if piRows := piSessionTestRows(t, db, requestLogDataSourcePiSession); len(piRows) != 0 {
		t.Fatalf("Oh My Pi 不应写入 Pi 数据来源: %+v", piRows)
	}
}

func TestOhMyPiSessionUsageRootResolutionFromEnvironment(t *testing.T) {
	home := t.TempDir()
	for _, key := range []string{"PI_CODING_AGENT_SESSION_DIR", "PI_CODING_AGENT_DIR", "PI_CONFIG_DIR", "OMP_PROFILE", "PI_PROFILE", "XDG_DATA_HOME"} {
		t.Setenv(key, "")
	}

	root, err := ompResolveSessionRoot(home, "native")
	if err != nil || root.Path != filepath.Join(home, ".omp", "agent", "sessions") || !root.ProjectLayout {
		t.Fatalf("默认根目录异常: %+v err=%v", root, err)
	}

	t.Setenv("OMP_PROFILE", "work")
	root, err = ompResolveSessionRoot(home, "native")
	if err != nil || root.Path != filepath.Join(home, ".omp", "profiles", "work", "agent", "sessions") {
		t.Fatalf("profile 根目录异常: %+v err=%v", root, err)
	}

	t.Setenv("OMP_PROFILE", "")
	agent := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", agent)
	root, err = ompResolveSessionRoot(home, "native")
	if err != nil || root.Path != filepath.Join(agent, "sessions") {
		t.Fatalf("PI_CODING_AGENT_DIR 根目录异常: %+v err=%v", root, err)
	}

	// 非 native 范围忽略全部环境变量
	root, err = ompResolveSessionRoot(home, "wsl:Ubuntu")
	if err != nil || root.Path != filepath.Join(home, ".omp", "agent", "sessions") {
		t.Fatalf("WSL 范围应忽略环境变量: %+v err=%v", root, err)
	}

	t.Setenv("OMP_PROFILE", "Bad Name")
	if _, err := ompResolveSessionRoot(home, "native"); err == nil {
		t.Fatal("非法 profile 应返回错误")
	}
}
