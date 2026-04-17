package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveProviderQuotaQueryTargetBaseURL(t *testing.T) {
	tests := []struct {
		name      string
		queryType ProviderQuotaQueryType
		apiURL    string
		expected  string
	}{
		{
			name:      "glm routes to official international endpoint",
			queryType: ProviderQuotaQueryTypeTokenPlanGLM,
			apiURL:    "https://open.bigmodel.cn/api/paas/v4/chat/completions",
			expected:  "https://api.z.ai",
		},
		{
			name:      "kimi routes to official quota endpoint even with moonshot anthropic url",
			queryType: ProviderQuotaQueryTypeTokenPlanKimi,
			apiURL:    "https://api.moonshot.cn/anthropic",
			expected:  "https://api.kimi.com",
		},
		{
			name:      "minimax cn stays on minimaxi domain",
			queryType: ProviderQuotaQueryTypeTokenPlanMiniMax,
			apiURL:    "https://api.minimaxi.com/anthropic",
			expected:  "https://api.minimaxi.com",
		},
		{
			name:      "minimax io routes to minimax io domain",
			queryType: ProviderQuotaQueryTypeTokenPlanMiniMax,
			apiURL:    "https://api.minimax.io/v1/text/chatcompletion_v2",
			expected:  "https://api.minimax.io",
		},
		{
			name:      "proxy subpath no longer hijacks kimi quota endpoint",
			queryType: ProviderQuotaQueryTypeTokenPlanKimi,
			apiURL:    "https://proxy.example.com/providers/kimi/v1",
			expected:  "https://api.kimi.com",
		},
		{
			name:      "local test origin stays local for integration tests",
			queryType: ProviderQuotaQueryTypeTokenPlanKimi,
			apiURL:    "http://127.0.0.1:39201/v1/messages",
			expected:  "http://127.0.0.1:39201",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actual := resolveProviderQuotaQueryTargetBaseURL(testCase.queryType, testCase.apiURL)
			if actual != testCase.expected {
				t.Fatalf("期望目标查询地址为 %s，实际为 %s", testCase.expected, actual)
			}
		})
	}
}

func TestProviderQuotaQueryService_QueryQuotaParsesGLMTokenPlan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/monitor/usage/quota/limit" {
			t.Fatalf("期望请求 GLM 额度路径，实际为 %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "glm-key" {
			t.Fatalf("期望 Authorization 为 glm-key，实际为 %s", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"limits": [
					{
						"type": "TOKENS_LIMIT",
						"currentValue": 25,
						"usage": 100,
						"percentage": 25,
						"nextResetTime": 1760000000000
					}
				]
			}
		}`))
	}))
	defer server.Close()

	service := NewProviderQuotaQueryService()
	result := service.QueryQuota(string(ProviderQuotaQueryTypeTokenPlanGLM), server.URL+"/api/paas/v4/chat/completions", "glm-key")

	if !result.Success {
		t.Fatalf("期望 GLM 查询成功，实际失败：%s", result.Error)
	}
	if len(result.Items) != 1 {
		t.Fatalf("期望 GLM 返回 1 个额度窗口，实际为 %d", len(result.Items))
	}
	if result.Items[0].Key != "five_hour" {
		t.Fatalf("期望额度 key 为 five_hour，实际为 %s", result.Items[0].Key)
	}
	if result.Items[0].Used != 25 || result.Items[0].Total != 100 {
		t.Fatalf("期望 GLM 已用 / 总量为 25 / 100，实际为 %f / %f", result.Items[0].Used, result.Items[0].Total)
	}
	if result.Items[0].NextReset == "" {
		t.Fatal("期望 GLM 返回 nextReset")
	}
}

func TestProviderQuotaQueryService_QueryQuotaParsesKimiAndMiniMaxTokenPlan(t *testing.T) {
	tests := []struct {
		name         string
		queryType    string
		expectedPath string
		responseBody string
		expectedKeys []string
		authHeader   string
	}{
		{
			name:         "kimi",
			queryType:    string(ProviderQuotaQueryTypeTokenPlanKimi),
			expectedPath: "/coding/v1/usages",
			authHeader:   "Bearer kimi-key",
			responseBody: `{
				"limits": [
					{
						"detail": {
							"limit": 200,
							"remaining": 150,
							"resetTime": "2026-04-12T00:00:00Z"
						}
					}
				],
				"usage": {
					"limit": 1000,
					"remaining": 850,
					"resetTime": "2026-04-14T00:00:00Z"
				}
			}`,
			expectedKeys: []string{"five_hour", "weekly"},
		},
		{
			name:         "minimax",
			queryType:    string(ProviderQuotaQueryTypeTokenPlanMiniMax),
			expectedPath: "/v1/api/openplatform/coding_plan/remains",
			authHeader:   "Bearer minimax-key",
			responseBody: `{
				"base_resp": {
					"status_code": 0
				},
				"model_remains": [
					{
						"current_interval_total_count": 500,
						"current_interval_usage_count": 320,
						"end_time": 1760000000000,
						"current_weekly_total_count": 2000,
						"current_weekly_usage_count": 1500,
						"weekly_end_time": 1760200000000
					}
				]
			}`,
			expectedKeys: []string{"five_hour", "weekly"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != testCase.expectedPath {
					t.Fatalf("期望请求路径为 %s，实际为 %s", testCase.expectedPath, r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); got != testCase.authHeader {
					t.Fatalf("期望 Authorization 为 %s，实际为 %s", testCase.authHeader, got)
				}

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(testCase.responseBody))
			}))
			defer server.Close()

			service := NewProviderQuotaQueryService()
			apiKey := "kimi-key"
			if testCase.name == "minimax" {
				apiKey = "minimax-key"
			}
			result := service.QueryQuota(testCase.queryType, server.URL+"/v1/messages", apiKey)

			if !result.Success {
				t.Fatalf("期望 %s 查询成功，实际失败：%s", testCase.name, result.Error)
			}
			if len(result.Items) != len(testCase.expectedKeys) {
				t.Fatalf("期望 %s 返回 %d 个额度窗口，实际为 %d", testCase.name, len(testCase.expectedKeys), len(result.Items))
			}
			for index, expectedKey := range testCase.expectedKeys {
				if result.Items[index].Key != expectedKey {
					t.Fatalf("期望第 %d 个额度 key 为 %s，实际为 %s", index, expectedKey, result.Items[index].Key)
				}
				if result.Items[index].Total <= 0 {
					t.Fatalf("期望第 %d 个额度 total > 0，实际为 %f", index, result.Items[index].Total)
				}
			}
		})
	}
}
