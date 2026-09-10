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
		{"codex packed metadata", "codex", `{"client_metadata":{"x-codex-turn-metadata":"{\"thread_id\":\"s7\"}"}}`, "s7", nil},
		{"codex underscored metadata", "codex", `{"client_metadata":{"x_codex_turn_metadata":"{\"sessionId\":\"s8\"}"}}`, "s8", nil},
		{"explicit header wins", "codex", `{"client_metadata":{"x-codex-turn-metadata":"{\"thread_id\":\"packed\"}"}}`, "header", map[string]string{"session_id": "header"}},
		{"header metadata wins", "codex", `{"client_metadata":{"x-codex-turn-metadata":"{\"thread_id\":\"packed\"}"}}`, "header", map[string]string{"x-codex-turn-metadata": `{"thread_id":"header"}`}},
		{"direct body wins", "codex", `{"thread_id":"direct","client_metadata":{"x-codex-turn-metadata":"{\"thread_id\":\"packed\"}"}}`, "direct", nil},
		{"direct client metadata wins", "codex", `{"client_metadata":{"session_id":"direct","x-codex-turn-metadata":"{\"thread_id\":\"packed\"}"}}`, "direct", nil},
		{"invalid packed metadata", "codex", `{"client_metadata":{"x-codex-turn-metadata":"{\"thread_id\":\"partial\""}}`, "", nil},
		{"packed parent is not identity", "codex", `{"client_metadata":{"x-codex-turn-metadata":"{\"parent_thread_id\":\"parent\",\"prompt_cache_key\":\"cache\"}"}}`, "", nil},
		{"cursor is not codex", "codex", `{"client_metadata":{"x-codex-turn-metadata":"{\"thread_id\":\"packed\"}"}}`, "", map[string]string{"x-cursor-conversation-id": "cursor"}},
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

func TestTrayHookSnapshotOneSupplierManySessions(t *testing.T) {
	var routes traySessionRoutes
	now := time.Unix(1800000000, 0)
	provider := Provider{ID: 42, Name: "Fixture supplier", Icon: "openai"}
	for _, id := range []string{"session-1", "session-2", "session-3"} {
		routes.record("codex", hookSessionKey("codex", id), provider, now)
	}
	providers := trayActivities(TrayProviderRuntimeState{Statuses: []TrayProviderActivityStatus{
		{ProviderID: "42", ProviderName: provider.Name, ActiveRequests: 3},
	}})
	providers[0].Icon = provider.Icon
	bindings := routes.snapshot("codex", now)
	if len(providers) != 1 || len(bindings) != 3 {
		t.Fatalf("session bindings must not create providers: providers=%d bindings=%d", len(providers), len(bindings))
	}
	seen := make(map[string]bool)
	for _, binding := range bindings {
		if seen[binding.SessionKey] || binding.ProviderID != providers[0].ProviderID {
			t.Fatal("duplicate session or mismatched provider identity")
		}
		seen[binding.SessionKey] = true
	}
	fixture := TraySnapshot{Version: 1, Session: "routing-fixture", Sequence: 1, HeartbeatAt: now.UnixMilli(),
		Platforms: []TraySnapshotPlatform{{Platform: "codex", Name: "Codex", Icon: "openai", Providers: providers, SessionBindings: bindings}}}
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	// Codenotch consumes this generated, credential-free fixture in its routing regression.
	t.Logf("CODENOTCH_ROUTING_FIXTURE=%s", data)
}
