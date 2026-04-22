package services

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthCheckServiceCheckProviderUsesModelProbeForCodex(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("请求路径 = %q, 期望 /responses", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("Authorization = %q, 期望 Bearer sk-test", got)
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
			t.Fatalf("请求体缺少 pong 指令: %s", body)
		}
		if !strings.Contains(body, `"type":"input_text"`) || !strings.Contains(body, `"text":"ping"`) {
			t.Fatalf("请求体不是 Responses API 探测格式: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gpt-5.3-codex","output":[{"type":"message","content":[{"type":"output_text","text":"pong"}]}]}`))
	}))
	defer server.Close()

	hcs := NewHealthCheckService(nil, nil, nil)
	result := hcs.checkProvider(context.Background(), Provider{
		ID:     1,
		Name:   "CodexTest",
		APIURL: server.URL,
		APIKey: "sk-test",
		AvailabilityConfig: &AvailabilityConfig{
			TestModel:    "gpt-5.3-codex",
			TestEndpoint: "/responses",
			Timeout:      5000,
		},
	}, "codex")

	if result.Status != HealthStatusOperational {
		t.Fatalf("Status = %q, 期望 %q, error=%q", result.Status, HealthStatusOperational, result.ErrorMessage)
	}
	if result.Model != "gpt-5.3-codex" {
		t.Fatalf("Model = %q, 期望 gpt-5.3-codex", result.Model)
	}
	if result.Endpoint != "/responses" {
		t.Fatalf("Endpoint = %q, 期望 /responses", result.Endpoint)
	}
}

func TestHealthCheckServiceCheckProviderMarksValidationFailureWhenResponseIsNotPong(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hello"}}]}`))
	}))
	defer server.Close()

	hcs := NewHealthCheckService(nil, nil, nil)
	result := hcs.checkProvider(context.Background(), Provider{
		ID:     2,
		Name:   "ChatProxy",
		APIURL: server.URL,
		APIKey: "sk-test",
		AvailabilityConfig: &AvailabilityConfig{
			TestModel:    "gpt-4.1-mini",
			TestEndpoint: "/v1/chat/completions",
			Timeout:      5000,
		},
	}, "codex")

	if result.Status != HealthStatusValidationError {
		t.Fatalf("Status = %q, 期望 %q, error=%q", result.Status, HealthStatusValidationError, result.ErrorMessage)
	}
	if !strings.Contains(result.ErrorMessage, availabilityProbeExpectedText) {
		t.Fatalf("ErrorMessage = %q, 应包含 %q", result.ErrorMessage, availabilityProbeExpectedText)
	}
}

func TestHealthCheckServiceHandleBlacklistIntegrationTreatsValidationFailureAsFailure(t *testing.T) {
	hcs := NewHealthCheckService(nil, nil, nil)
	provider := &Provider{
		ID:                        3,
		Name:                      "ValidationFailProvider",
		ConnectivityAutoBlacklist: true,
	}
	result := &HealthCheckResult{
		ProviderID:   provider.ID,
		ProviderName: provider.Name,
		Platform:     "codex",
		Status:       HealthStatusValidationError,
	}

	hcs.handleBlacklistIntegration(provider, result)

	counterKey := "codex:" + providerRefFromNumericID(provider.ID, provider.Name)
	counter := hcs.failCounters[counterKey]
	if counter == nil {
		t.Fatalf("应创建失败计数器")
	}
	if counter.ConsecutiveFails != 1 {
		t.Fatalf("ConsecutiveFails = %d, 期望 1", counter.ConsecutiveFails)
	}
}
