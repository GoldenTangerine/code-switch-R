/**
 * @name: DeepSeek Harness 会话用量测试
 * @Descripttion: 验证 DSH 会话日志的 token 提取、耗时与首字估算、跨批次配对、分叉去重及 zstd 跳过。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_dsh_test.go
 */

package services

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func dshTestStepStart(turn int, step int, timeMs int64) string {
	return fmt.Sprintf(`{"type":"step/start","seq":1,"time":%d,"data":{"turn":%d,"step":%d}}`, timeMs, turn, step)
}

func dshTestAssistant(turn int, step int, id string, timeMs int64, provider string, model string, input int, output int, cacheRead int, firstOutput int64) string {
	return fmt.Sprintf(`{"type":"assistant/message","seq":2,"time":%d,"data":{"turn":%d,"step":%d,"message":{"role":"assistant","id":%q,"source":{"kind":"model","provider":%q,"model":%q},"content":[{"type":"text","text":"ok"}]},"usage":{"inputTokens":%d,"outputTokens":%d,"cacheReadTokens":%d,"cacheWriteTokens":0},"stream":[{"type":"chunk","time":%d,"chunk":{"type":"block-start","index":0}}]}}`,
		timeMs, turn, step, id, provider, model, input, output, cacheRead, firstOutput)
}

const dshTestHeader = `{"type":"session","version":4,"id":"session-test","createdAt":1792746632526,"cwd":"D:\\proj","isSeeded":false,"delegationDepth":0,"agentPreset":"standard"}`

func TestDeepSeekHarnessSessionUsageImportsTokensTimingAndDedup(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	sessionDir := filepath.Join(home, ".dsh", "sessions", "--proj--", "session-test")
	path := filepath.Join(sessionDir, "session.v4.jsonl")
	piSessionTestWrite(t, path,
		dshTestHeader,
		dshTestStepStart(1, 1, 1_700_000_000_000),
		dshTestAssistant(1, 1, "m1", 1_700_000_003_000, "deepseek-account", "deepseek-flash", 100, 50, 900, 1_700_000_000_200),
		dshTestStepStart(1, 2, 1_700_000_010_000),
		// 路由前缀的模型 id 去掉前缀后才能按真实模型名计价
		dshTestAssistant(1, 2, "m2", 1_700_000_012_500, "gateway", "global:deepseek-pro", 20, 10, 1_000, 1_700_000_010_300),
		// token 全为 0 的事件不入库
		dshTestAssistant(1, 3, "m0", 1_700_000_020_000, "gateway", "deepseek-pro", 0, 0, 0, 1_700_000_019_000),
	)
	// 压缩日志当前不解码，只计入 Skipped
	zstdDir := filepath.Join(home, ".dsh", "sessions", "--proj--", "session-compressed")
	if err := os.MkdirAll(zstdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zstdDir, "session.v4.jsonl.zstd"), []byte{0x28, 0xb5, 0x2f, 0xfd, 0x00}, 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := syncDeepSeekHarnessSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, result, err)
	if result.Platform != sessionPlatformDeepSeekHarness || result.FilesScanned != 1 || result.Imported != 2 || result.Skipped != 1 {
		t.Fatalf("首次同步结果异常: %+v", result)
	}
	rows := piSessionTestRows(t, db, requestLogDataSourceDeepSeekHarnessSession)
	if len(rows) != 2 {
		t.Fatalf("行数 = %d, want 2", len(rows))
	}
	first, second := rows[0], rows[1]
	if first.Platform != "deepseek-harness" || first.Model != "deepseek-flash" || first.SessionID != "session-test" {
		t.Fatalf("第一行归属异常: %+v", first)
	}
	// inputTokens 本身不含缓存，直接作为新增输入
	if first.Input != 100 || first.Output != 50 || first.CacheRead != 900 || first.CacheCreate != 0 {
		t.Fatalf("第一行 token 异常: %+v", first)
	}
	// step/start 到 assistant/message 3.0s，最早的流块在起点后 0.2s
	if math.Abs(first.Duration-3.0) > 0.0005 || math.Abs(first.FirstToken-0.2) > 0.0005 {
		t.Fatalf("第一行计时异常: duration=%v first=%v", first.Duration, first.FirstToken)
	}
	if second.Model != "deepseek-pro" || second.Input != 20 || second.CacheRead != 1000 ||
		math.Abs(second.Duration-2.5) > 0.0005 || math.Abs(second.FirstToken-0.3) > 0.0005 {
		t.Fatalf("第二行异常: %+v", second)
	}

	// 分叉会话把父会话前缀原样抄进新日志：同一 message id 只记一次
	forkDir := filepath.Join(home, ".dsh", "sessions", "--proj--", "session-fork")
	piSessionTestWrite(t, filepath.Join(forkDir, "session.v4.jsonl"),
		dshTestHeader,
		dshTestStepStart(1, 1, 1_700_000_000_000),
		dshTestAssistant(1, 1, "m1", 1_700_000_003_000, "deepseek-account", "deepseek-flash", 100, 50, 900, 1_700_000_000_200),
	)
	if _, err := syncDeepSeekHarnessSessionUsage(ls, scope, home); err != nil {
		t.Fatal(err)
	}
	if rows := piSessionTestRows(t, db, requestLogDataSourceDeepSeekHarnessSession); len(rows) != 2 {
		t.Fatalf("分叉会话不应产生新行: %d", len(rows))
	}

	// 游标丢失后全量重扫：幂等且来源 ID 唯一
	piSessionTestResetCursors(t, db)
	again, err := syncDeepSeekHarnessSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, again, err)
	if again.Imported != 0 {
		t.Fatalf("重扫不应再导入: %+v", again)
	}
	var total, distinct int
	if err := db.QueryRow("SELECT COUNT(*), COUNT(DISTINCT source_record_id) FROM request_log WHERE data_source = ?", requestLogDataSourceDeepSeekHarnessSession).Scan(&total, &distinct); err != nil {
		t.Fatal(err)
	}
	if total != 2 || distinct != 2 {
		t.Fatalf("重扫后 total=%d distinct=%d, want 2/2", total, distinct)
	}
}

func TestDeepSeekHarnessSessionUsagePairsStepAcrossSyncs(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	path := filepath.Join(home, ".dsh", "sessions", "--proj--", "session-live", "session.v4.jsonl")
	// 第一轮只有 step/start，assistant/message 还没写出
	piSessionTestWrite(t, path, dshTestHeader, dshTestStepStart(2, 1, 1_700_000_100_000))
	result, err := syncDeepSeekHarnessSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, result, err)
	if result.Imported != 0 {
		t.Fatalf("尚无用量，不应导入: %+v", result)
	}

	// 第二轮追加 assistant/message：起点来自上一轮保存的解析状态
	piSessionTestAppend(t, path, dshTestAssistant(2, 1, "live-1", 1_700_000_104_000, "deepseek-account", "deepseek-flash", 8, 4, 0, 1_700_000_100_500))
	result, err = syncDeepSeekHarnessSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, result, err)
	if result.Imported != 1 {
		t.Fatalf("追加后导入数 = %d, want 1", result.Imported)
	}
	rows := piSessionTestRows(t, db, requestLogDataSourceDeepSeekHarnessSession)
	if len(rows) != 1 || math.Abs(rows[0].Duration-4.0) > 0.0005 || math.Abs(rows[0].FirstToken-0.5) > 0.0005 {
		t.Fatalf("跨批次计时异常: %+v", rows)
	}
}

func TestDeepSeekHarnessSessionUsageMissingDirectoryAndEnvOverride(t *testing.T) {
	ls, db, scope := prepareSessionUsageTest(t)
	home := t.TempDir()
	result, err := syncDeepSeekHarnessSessionUsage(ls, scope, home)
	piSessionTestAssertSyncOK(t, result, err)
	if result.FilesScanned != 0 || result.Imported != 0 || result.Platform != sessionPlatformDeepSeekHarness {
		t.Fatalf("目录缺失应返回空结果: %+v", result)
	}

	dshHome := t.TempDir()
	t.Setenv("DSH_HOME", dshHome)
	piSessionTestWrite(t, filepath.Join(dshHome, "sessions", "p", "session-env", "session.v4.jsonl"),
		dshTestHeader,
		dshTestAssistant(1, 1, "env-1", 1_700_000_003_000, "gateway", "deepseek-flash", 5, 5, 0, 1_700_000_000_200),
	)
	// native 范围读取 DSH_HOME；WSL 范围不读
	if result, err = syncDeepSeekHarnessSessionUsage(ls, "wsl:Ubuntu", home); err != nil || result.FilesScanned != 0 {
		t.Fatalf("WSL 范围不应读取 DSH_HOME: %+v err=%v", result, err)
	}
	result, err = syncDeepSeekHarnessSessionUsage(ls, "native", home)
	piSessionTestAssertSyncOK(t, result, err)
	if result.Imported != 1 {
		t.Fatalf("DSH_HOME 覆盖未生效: %+v", result)
	}
	if rows := piSessionTestRows(t, db, requestLogDataSourceDeepSeekHarnessSession); len(rows) != 1 || rows[0].SessionID != "session-env" {
		t.Fatalf("DSH_HOME 行异常: %+v", rows)
	}
}
