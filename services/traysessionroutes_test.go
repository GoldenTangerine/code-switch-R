/*
@name: 会话供应商关联回归
@Descripttion: 验证明确会话键、并发供应商路由和快照兼容行为。
@version: 1.0.0
@Author: sm
@Date: 2026-09-09 12:20:00
@LastEditTime: 2026-09-09 12:20:00
@FilePath: services/traysessionroutes_test.go
*/
package services

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHookSessionKeyCrossLanguage(t *testing.T) {
	for _, tc := range []struct{ tool, expected string }{
		{"codex", "d3f691a7763732a761396db8b1f3ca194c77142d6f19bceb567038f2d2c388ac"},
		{"claude", "b51f4913a4cc284f55d86d9c7dec107dbde637d85c8c94f6d873634b3194200a"},
	} {
		if got := hookSessionKey(tc.tool, "session-1"); got != tc.expected {
			t.Fatalf("%s key %s != %s", tc.tool, got, tc.expected)
		}
	}
	if hookSessionKey("cursor", "session-1") != "" || hookSessionKey("codex", " ") != "" {
		t.Fatal("unsupported or empty identity was accepted")
	}
}

func TestHookSessionIDExplicitIdentityOnly(t *testing.T) {
	for _, tc := range []struct {
		name, tool, body, expected string
		headers                    map[string]string
	}{
		{"claude header", "claude", `{}`, "s1", map[string]string{"X-Claude-Code-Session-Id": "s1"}},
		{"claude metadata", "claude", `{"metadata":{"user_id":"{\"device_id\":\"device\",\"session_id\":\"s2\"}"}}`, "s2", nil},
		{"claude child", "claude", `{}`, "", map[string]string{"x-claude-code-session-id": "parent", "x-claude-code-agent-id": "child"}},
		{"codex header", "codex", `{}`, "s3", map[string]string{"X-Codex-Thread-Id": "s3"}},
		{"codex native header", "codex", `{}`, "s4", map[string]string{"Session_Id": "s4"}},
		{"codex turn metadata", "codex", `{}`, "s5", map[string]string{"X-Codex-Turn-Metadata": `{"thread_id":"s5"}`}},
		{"codex body", "codex", `{"thread_id":"s6"}`, "s6", nil},
		{"cache key is not identity", "codex", `{"prompt_cache_key":"not-session"}`, "", nil},
		{"parent alone is not identity", "codex", `{"parent_thread_id":"parent"}`, "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := hookSessionID(tc.tool, []byte(tc.body), tc.headers); got != tc.expected {
				t.Fatalf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestTrayHookRoutesParallelAndFallback(t *testing.T) {
	var routes traySessionRoutes
	now := time.Now()
	a, b := hookSessionKey("codex", "a"), hookSessionKey("codex", "b")
	routes.record("codex", a, Provider{ID: 1, Name: "first", APIKey: "never-export"}, now)
	routes.record("codex", b, Provider{ID: 2, Name: "second"}, now)
	routes.record("codex", a, Provider{ID: 3, Name: "fallback"}, now.Add(time.Second))
	rows := routes.snapshot("codex", now.Add(time.Minute))
	if len(rows) != 2 {
		t.Fatalf("lost completed-request associations: %d", len(rows))
	}
	for _, row := range rows {
		if row.SessionKey == a && (row.ProviderName != "fallback" || row.Sequence != 3) {
			t.Fatal(row)
		}
		if row.SessionKey == b && row.ProviderName != "second" {
			t.Fatal(row)
		}
	}
	data, _ := json.Marshal(rows)
	if strings.Contains(string(data), "never-export") || strings.Contains(string(data), "apiKey") {
		t.Fatal("exported private fields")
	}
	if len(routes.snapshot("claude", now)) != 0 {
		t.Fatal("cross-platform association")
	}
	if len(routes.snapshot("codex", now.Add(25*time.Hour))) != 0 {
		t.Fatal("unbounded old associations")
	}
}

func TestTrayHookRoutesConcurrentPublication(t *testing.T) {
	var routes traySessionRoutes
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes.record("codex", hookSessionKey("codex", "same"), Provider{ID: 1}, time.Now())
			_ = routes.snapshot("codex", time.Now())
		}()
	}
	wg.Wait()
	rows := routes.snapshot("codex", time.Now())
	if len(rows) != 1 || rows[0].Sequence != 20 {
		t.Fatal(rows)
	}
}
