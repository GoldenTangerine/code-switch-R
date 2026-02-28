package services

import (
	"testing"

	"github.com/daodao97/xgo/xdb"
)

func TestGetRequestLogPayload_ReturnsStoredBodies(t *testing.T) {
	useIsolatedHomeDir(t)

	if err := InitDatabase(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取数据库连接失败: %v", err)
	}

	result, err := db.Exec(`
		INSERT INTO request_log (
			platform, model, provider, http_code, input_tokens, output_tokens, cache_create_tokens, cache_read_tokens, reasoning_tokens,
			request_body, response_body, request_body_truncated, response_body_truncated, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"codex",
		"gpt-5.3-codex",
		"payload-provider",
		200,
		10,
		20,
		0,
		0,
		0,
		`{"model":"gpt-5.3-codex","stream":true}`,
		`{"id":"resp_123","output":[{"type":"output_text","text":"hello"}]}`,
		0,
		1,
		"2026-02-28 12:00:00",
	)
	if err != nil {
		t.Fatalf("插入 request_log 失败: %v", err)
	}
	logID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("获取插入 id 失败: %v", err)
	}

	ls := NewLogService(nil)
	detail, err := ls.GetRequestLogPayload(logID)
	if err != nil {
		t.Fatalf("GetRequestLogPayload 调用失败: %v", err)
	}

	if detail.ID != logID {
		t.Fatalf("日志 id 不匹配，期望 %d，实际 %d", logID, detail.ID)
	}
	if detail.RequestBody != `{"model":"gpt-5.3-codex","stream":true}` {
		t.Fatalf("request_body 不匹配: %s", detail.RequestBody)
	}
	if detail.ResponseBody == "" {
		t.Fatalf("response_body 为空")
	}
	if detail.RequestBodyTruncated {
		t.Fatalf("request_body_truncated 期望 false，实际 true")
	}
	if !detail.ResponseBodyTruncated {
		t.Fatalf("response_body_truncated 期望 true，实际 false")
	}
}

func TestGetRequestLogPayload_InvalidID(t *testing.T) {
	ls := NewLogService(nil)
	if _, err := ls.GetRequestLogPayload(0); err == nil {
		t.Fatalf("非法 id 应返回错误")
	}
}
