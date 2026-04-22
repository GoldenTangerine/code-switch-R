package services

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConnectivityTestServiceUsesSharedAvailabilityProbe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("请求路径 = %q, 期望 /responses", r.URL.Path)
		}
		if got := r.Header.Get("openai-beta"); got != "responses=experimental" {
			t.Fatalf("openai-beta = %q, 期望 responses=experimental", got)
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("读取请求体失败: %v", err)
		}
		body := string(bodyBytes)
		if !strings.Contains(body, `"instructions":"You are an echo bot. Reply with exactly pong."`) {
			t.Fatalf("请求体缺少共享 probe 指令: %s", body)
		}
		if !strings.Contains(body, `"type":"input_text"`) {
			t.Fatalf("连通性测试没有复用 Responses 探测体: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"pong"}]}]}`))
	}))
	defer server.Close()

	cts := NewConnectivityTestService(nil, nil, nil)
	result := cts.TestProvider(context.Background(), Provider{
		ID:     12,
		Name:   "SharedProbeProvider",
		APIURL: server.URL,
		APIKey: "sk-test",
		AvailabilityConfig: &AvailabilityConfig{
			TestModel:    "gpt-5.3-codex",
			TestEndpoint: " /responses ",
			Timeout:      5000,
		},
	}, "codex")

	if result.Status != StatusAvailable {
		t.Fatalf("Status = %d, 期望 %d, subStatus=%q, message=%q", result.Status, StatusAvailable, result.SubStatus, result.Message)
	}
	if result.HTTPCode != 200 {
		t.Fatalf("HTTPCode = %d, 期望 200", result.HTTPCode)
	}
}
