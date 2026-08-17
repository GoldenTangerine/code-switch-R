package services

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/dop251/goja"
)

type rewriteHostTransport struct {
	target    *url.URL
	transport http.RoundTripper
}

func (t rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = t.target.Scheme
	cloned.URL.Host = t.target.Host
	cloned.Host = t.target.Host
	transport := t.transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	return transport.RoundTrip(cloned)
}

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
						"unit": 6,
						"number": 7,
						"currentValue": 1,
						"usage": 100,
						"percentage": 1,
						"nextResetTime": 1760600000000
					},
					{
						"type": "TOKENS_LIMIT",
						"unit": 3,
						"number": 5,
						"currentValue": 25,
						"usage": 100,
						"percentage": 25,
						"nextResetTime": 1760000000000
					},
					{
						"type": "TOKENS_LIMIT",
						"window": "5 小时额度",
						"currentValue": 88,
						"usage": 100,
						"percentage": 88
					}
				]
			}
		}`))
	}))
	defer server.Close()

	service := NewProviderQuotaQueryService()
	result := service.QueryQuota(string(ProviderQuotaQueryTypeTokenPlanGLM), server.URL+"/api/paas/v4/chat/completions", "glm-key", nil)

	if !result.Success {
		t.Fatalf("期望 GLM 查询成功，实际失败：%s", result.Error)
	}
	if len(result.Items) != 2 {
		t.Fatalf("期望 GLM 返回 2 个额度窗口，实际为 %d", len(result.Items))
	}
	if result.Items[0].Key != "five_hour" {
		t.Fatalf("期望第 1 个额度 key 为 five_hour，实际为 %s", result.Items[0].Key)
	}
	if result.Items[0].Used != 25 || result.Items[0].Total != 100 {
		t.Fatalf("期望 GLM 已用 / 总量为 25 / 100，实际为 %f / %f", result.Items[0].Used, result.Items[0].Total)
	}
	if result.Items[0].NextReset == "" {
		t.Fatal("期望 GLM 5 小时额度返回 nextReset")
	}
	if result.Items[1].Key != "weekly" {
		t.Fatalf("期望第 2 个额度 key 为 weekly，实际为 %s", result.Items[1].Key)
	}
	if result.Items[1].Used != 1 || result.Items[1].Total != 100 {
		t.Fatalf("期望 GLM 周额度已用 / 总量为 1 / 100，实际为 %f / %f", result.Items[1].Used, result.Items[1].Total)
	}
	if result.Items[1].NextReset == "" {
		t.Fatal("期望 GLM 周额度返回 nextReset")
	}
}

func TestResolveGLMTokenPlanQuotaKey(t *testing.T) {
	tests := []struct {
		name      string
		limitItem map[string]any
		expected  string
	}{
		{
			name: "unit has highest priority for five hour",
			limitItem: map[string]any{
				"unit":   3,
				"window": "每周额度",
			},
			expected: "five_hour",
		},
		{
			name: "recognizes weekly from chinese labels without unit number",
			limitItem: map[string]any{
				"window": "周额度",
				"cycle":  "每周",
			},
			expected: "weekly",
		},
		{
			name: "recognizes five hour from chinese labels without unit number",
			limitItem: map[string]any{
				"name": "5 小时窗口",
			},
			expected: "five_hour",
		},
		{
			name: "does not fallback unknown localized labels to five hour",
			limitItem: map[string]any{
				"window": "额度窗口",
				"cycle":  "重置周期",
			},
			expected: "",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actual := resolveGLMTokenPlanQuotaKey(testCase.limitItem)
			if actual != testCase.expected {
				t.Fatalf("期望 key 为 %s，实际为 %s", testCase.expected, actual)
			}
		})
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
			result := service.QueryQuota(testCase.queryType, server.URL+"/v1/messages", apiKey, nil)

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

func TestProviderQuotaQueryService_QueryQuotaParsesGeneralScriptTemplate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/balance" {
			t.Fatalf("期望请求 /user/balance，实际为 %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer general-key" {
			t.Fatalf("期望 Authorization 为 Bearer general-key，实际为 %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"balance": 12.5}`))
	}))
	defer server.Close()

	service := NewProviderQuotaQueryService()
	result := service.QueryQuota(string(ProviderQuotaQueryTypeGeneral), server.URL, "", &ProviderQuotaQueryConfig{
		Enabled:      true,
		TemplateType: string(ProviderQuotaTemplateTypeGeneral),
		APIKey:       "general-key",
		Timeout:      6,
		Code: `({
  request: {
    url: '{{baseUrl}}/user/balance',
    method: 'GET',
    headers: {
      'Authorization': 'Bearer {{apiKey}}'
    }
  },
  extractor: function(response) {
    return {
      label: 'Balance',
      remaining: response.balance,
      unit: 'USD',
      valueMode: 'currency'
    };
  }
})`,
	})

	if !result.Success {
		t.Fatalf("期望通用模版查询成功，实际失败：%s", result.Error)
	}
	if len(result.Items) != 1 {
		t.Fatalf("期望返回 1 条余额数据，实际为 %d", len(result.Items))
	}
	if result.Items[0].Label != "Balance" {
		t.Fatalf("期望标签为 Balance，实际为 %s", result.Items[0].Label)
	}
	if result.Items[0].Used != 0 || result.Items[0].Total != 12.5 {
		t.Fatalf("期望已用/总量为 0 / 12.5，实际为 %f / %f", result.Items[0].Used, result.Items[0].Total)
	}
	if result.Items[0].ValueMode != string(ProviderQuotaValueModeCurrency) || result.Items[0].Unit != "USD" {
		t.Fatalf("期望金额模式为 currency/USD，实际为 %s / %s", result.Items[0].ValueMode, result.Items[0].Unit)
	}
}

func TestProviderQuotaQueryService_QueryQuotaParsesNewAPIScriptTemplate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user/self" {
			t.Fatalf("期望请求 /api/user/self，实际为 %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("期望 Authorization 为 Bearer access-token，实际为 %s", got)
		}
		if got := r.Header.Get("New-Api-User"); got != "user-42" {
			t.Fatalf("期望 New-Api-User 为 user-42，实际为 %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"group": "Pro",
				"quota": 3000000,
				"used_quota": 2000000
			}
		}`))
	}))
	defer server.Close()

	service := NewProviderQuotaQueryService()
	result := service.QueryQuota(string(ProviderQuotaQueryTypeNewAPI), server.URL, "", &ProviderQuotaQueryConfig{
		Enabled:      true,
		TemplateType: string(ProviderQuotaTemplateTypeNewAPI),
		AccessToken:  "access-token",
		UserID:       "user-42",
		Code: `({
  request: {
    url: '{{baseUrl}}/api/user/self',
    method: 'GET',
    headers: {
      'Authorization': 'Bearer {{accessToken}}',
      'New-Api-User': '{{userId}}'
    }
  },
  extractor: function(response) {
    return {
      label: response.data.group,
      remaining: response.data.quota / 500000,
      used: response.data.used_quota / 500000,
      total: (response.data.quota + response.data.used_quota) / 500000,
      unit: 'USD',
      valueMode: 'currency'
    };
  }
})`,
	})

	if !result.Success {
		t.Fatalf("期望 NewAPI 模版查询成功，实际失败：%s", result.Error)
	}
	if len(result.Items) != 1 {
		t.Fatalf("期望返回 1 条额度数据，实际为 %d", len(result.Items))
	}
	if result.Items[0].Label != "Pro" {
		t.Fatalf("期望标签为 Pro，实际为 %s", result.Items[0].Label)
	}
	if result.Items[0].Used != 4 || result.Items[0].Total != 10 {
		t.Fatalf("期望已用/总量为 4 / 10，实际为 %f / %f", result.Items[0].Used, result.Items[0].Total)
	}
}

func TestProviderQuotaQueryService_QueryQuotaParsesSub2APIScriptTemplate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/usage" {
			t.Fatalf("期望请求 /v1/usage，实际为 %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sub2api-key" {
			t.Fatalf("期望 Authorization 为 Bearer sub2api-key，实际为 %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"isValid": true,
			"remaining": 195.16573,
			"unit": "USD",
			"subscription": {
				"daily_limit_usd": 0,
				"daily_usage_usd": 4.83427,
				"weekly_limit_usd": 200,
				"weekly_usage_usd": 4.83427,
				"weekly_window_start": "2026-08-17T15:58:35.941105+08:00",
				"monthly_limit_usd": 800,
				"monthly_usage_usd": 4.83427,
				"expires_at": "2026-09-16T15:53:00.193234+08:00"
			}
		}`))
	}))
	defer server.Close()

	service := NewProviderQuotaQueryService()
	result := service.QueryQuota(string(ProviderQuotaQueryTypeSub2API), server.URL, "", &ProviderQuotaQueryConfig{
		Enabled:      true,
		TemplateType: string(ProviderQuotaTemplateTypeSub2API),
		APIKey:       "sub2api-key",
		Code: `({
  request: {
    url: "{{baseUrl}}/v1/usage",
    method: "GET",
    headers: { "Authorization": "Bearer {{apiKey}}" }
  },
  extractor: function(response) {
    const subscription = response?.subscription ?? {};
    const isValid = response?.is_active ?? response?.isValid ?? true;
    const unit = response?.unit ?? response?.quota?.unit ?? "USD";
    const invalidMessage = isValid ? "" : (response?.message || "Invalid subscription");
    const items = [];
    if (Number(subscription.daily_limit_usd) > 0) {
      const nextReset = new Date();
      nextReset.setHours(24, 0, 0, 0);
      items.push({ key: "daily", used: Number(subscription.daily_usage_usd) || 0, total: Number(subscription.daily_limit_usd), nextReset: nextReset.toISOString(), isValid, invalidMessage, unit, valueMode: "currency" });
    }
    if (Number(subscription.weekly_limit_usd) > 0) {
      const windowStart = new Date(subscription.weekly_window_start);
      const nextReset = Number.isNaN(windowStart.getTime()) ? undefined : new Date(windowStart.getTime() + 7 * 24 * 60 * 60 * 1000).toISOString();
      items.push({ key: "weekly", used: Number(subscription.weekly_usage_usd) || 0, total: Number(subscription.weekly_limit_usd), nextReset, isValid, invalidMessage, unit, valueMode: "currency" });
    }
    if (Number(subscription.monthly_limit_usd) > 0) {
      items.push({ key: "monthly", used: Number(subscription.monthly_usage_usd) || 0, total: Number(subscription.monthly_limit_usd), nextReset: subscription.expires_at, isValid, invalidMessage, unit, valueMode: "currency" });
    }
    if (items.length > 0) {
      return items;
    }
    const remaining = response?.remaining ?? response?.quota?.remaining ?? response?.balance;
    return { key: "balance", label: response?.planName || "Sub2API", isValid, invalidMessage, remaining, unit, valueMode: "currency" };
  }
})`,
	})

	if !result.Success {
		t.Fatalf("期望 Sub2API 模版查询成功，实际失败：%s", result.Error)
	}
	if len(result.Items) != 2 {
		t.Fatalf("期望过滤无限制日额度后返回 2 项，实际为 %d", len(result.Items))
	}
	if result.Items[0].Key != "weekly" || result.Items[0].Used != 4.83427 || result.Items[0].Total != 200 {
		t.Fatalf("Sub2API 周额度解析异常：%+v", result.Items[0])
	}
	if result.Items[0].NextReset != "2026-08-24T07:58:35.941Z" {
		t.Fatalf("Sub2API 周额度刷新时间异常：%s", result.Items[0].NextReset)
	}
	if result.Items[1].Key != "monthly" || result.Items[1].Used != 4.83427 || result.Items[1].Total != 800 {
		t.Fatalf("Sub2API 月额度解析异常：%+v", result.Items[1])
	}
	if result.Items[1].NextReset != "2026-09-16T15:53:00.193234+08:00" {
		t.Fatalf("Sub2API 月额度刷新时间异常：%s", result.Items[1].NextReset)
	}
}

func TestProviderQuotaQueryService_QueryQuotaSub2APIFallsBackToBalance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"isValid": true,
			"planName": "Unlimited",
			"remaining": -1,
			"unit": "USD",
			"subscription": {
				"daily_limit_usd": 0,
				"weekly_limit_usd": 0,
				"monthly_limit_usd": 0
			}
		}`))
	}))
	defer server.Close()

	service := NewProviderQuotaQueryService()
	result := service.QueryQuota(string(ProviderQuotaQueryTypeSub2API), server.URL, "sub2api-key", &ProviderQuotaQueryConfig{
		Enabled:      true,
		TemplateType: string(ProviderQuotaTemplateTypeSub2API),
		Code: `({
  request: { url: "{{baseUrl}}/v1/usage", method: "GET" },
  extractor: function(response) {
    return { key: "balance", label: response.planName, remaining: response.remaining, unlimited: response.remaining < 0, unit: response.unit, valueMode: "currency", isValid: response.isValid };
  }
})`,
	})

	if !result.Success || len(result.Items) != 1 {
		t.Fatalf("期望 Sub2API 无限订阅回退为余额，实际：%+v", result)
	}
	item := result.Items[0]
	if item.Key != "balance" || item.Label != "Unlimited" || item.Total != 0 || item.Used != 0 || !item.Unlimited {
		t.Fatalf("Sub2API 余额回退异常：%+v", item)
	}
}

func TestProviderQuotaQueryService_QueryQuotaPreservesInvalidMessageForNewAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user/self" {
			t.Fatalf("期望请求 /api/user/self，实际为 %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer expired-token" {
			t.Fatalf("期望 Authorization 为 Bearer expired-token，实际为 %s", got)
		}
		if got := r.Header.Get("New-Api-User"); got != "user-42" {
			t.Fatalf("期望 New-Api-User 为 user-42，实际为 %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": false,
			"message": "Access token expired"
		}`))
	}))
	defer server.Close()

	service := NewProviderQuotaQueryService()
	result := service.QueryQuota(string(ProviderQuotaQueryTypeNewAPI), server.URL, "", &ProviderQuotaQueryConfig{
		Enabled:      true,
		TemplateType: string(ProviderQuotaTemplateTypeNewAPI),
		AccessToken:  "expired-token",
		UserID:       "user-42",
		Code: `({
  request: {
    url: '{{baseUrl}}/api/user/self',
    method: 'GET',
    headers: {
      'Authorization': 'Bearer {{accessToken}}',
      'New-Api-User': '{{userId}}'
    }
  },
  extractor: function(response) {
    return {
      label: 'NewAPI',
      isValid: false,
      invalidMessage: response.message || 'Query failed',
      extra: '请检查 Access Token 和 User ID'
    };
  }
})`,
	})

	if !result.Success {
		t.Fatalf("期望 NewAPI 失败态仍能返回结果，实际失败：%s", result.Error)
	}
	if len(result.Items) != 1 {
		t.Fatalf("期望返回 1 条额度数据，实际为 %d", len(result.Items))
	}
	item := result.Items[0]
	if item.Active {
		t.Fatal("期望 NewAPI 失败态为 inactive")
	}
	if item.InvalidMessage != "Access token expired" {
		t.Fatalf("期望 invalidMessage 为 Access token expired，实际为 %s", item.InvalidMessage)
	}
	if item.Extra != "请检查 Access Token 和 User ID" {
		t.Fatalf("期望 extra 被透传，实际为 %s", item.Extra)
	}
	if item.Used != 0 || item.Total != 0 {
		t.Fatalf("期望失败态已用/总量为 0 / 0，实际为 %f / %f", item.Used, item.Total)
	}
}

func TestBuildProviderQuotaScriptWithVarsEscapesSpecialCharacters(t *testing.T) {
	vm := goja.New()
	script := buildProviderQuotaScriptWithVars(`({
  request: {
    url: '{{baseUrl}}/user/balance',
    method: 'GET',
    headers: {
      'Authorization': 'Bearer {{apiKey}}',
      'X-Access-Token': '{{accessToken}}',
      'X-User-Id': '{{userId}}'
    }
  },
  extractor: function(response) {
    return response;
  }
})`, `quota'"key\slash`, "https://quota.example.com", "token\nline", "user'42")

	value, err := vm.RunString(script)
	if err != nil {
		t.Fatalf("期望替换后的脚本仍可执行，实际失败：%v", err)
	}

	configObject := value.ToObject(vm)
	requestObject := configObject.Get("request").ToObject(vm)
	headersObject := requestObject.Get("headers").ToObject(vm)

	if got := requestObject.Get("url").String(); got != "https://quota.example.com/user/balance" {
		t.Fatalf("期望 URL 正确替换，实际为 %s", got)
	}
	if got := headersObject.Get("Authorization").String(); got != `Bearer quota'"key\slash` {
		t.Fatalf("期望 Authorization 保留特殊字符，实际为 %s", got)
	}
	if got := headersObject.Get("X-Access-Token").String(); got != "token\nline" {
		t.Fatalf("期望 access token 保留换行，实际为 %q", got)
	}
	if got := headersObject.Get("X-User-Id").String(); got != "user'42" {
		t.Fatalf("期望 user id 保留单引号，实际为 %s", got)
	}
}

func TestNormalizeProviderQuotaQueryConfigStripsHiddenFieldsForBuiltinTemplates(t *testing.T) {
	balanceConfig := normalizeProviderQuotaQueryConfig(&ProviderQuotaQueryConfig{
		Enabled:      true,
		TemplateType: string(ProviderQuotaTemplateTypeBalance),
		Code:         "stale-code",
		APIKey:       "dedicated-key",
		BaseURL:      "https://stale.example.com",
		AccessToken:  "stale-token",
		UserID:       "stale-user",
	}, ProviderQuotaQueryTypeBalance)

	if balanceConfig == nil {
		t.Fatal("期望 balance 配置被标准化")
	}
	if balanceConfig.Code != "" || balanceConfig.APIKey != "" || balanceConfig.BaseURL != "" || balanceConfig.AccessToken != "" || balanceConfig.UserID != "" {
		t.Fatalf("期望 balance 模板隐藏字段被清空，实际为 %+v", *balanceConfig)
	}

	tokenPlanConfig := normalizeProviderQuotaQueryConfig(&ProviderQuotaQueryConfig{
		Enabled:           true,
		TemplateType:      string(ProviderQuotaTemplateTypeTokenPlan),
		Code:              "stale-code",
		APIKey:            "dedicated-key",
		BaseURL:           "https://stale.example.com",
		AccessToken:       "stale-token",
		UserID:            "stale-user",
		TokenPlanProvider: "",
	}, ProviderQuotaQueryTypeTokenPlanKimi)

	if tokenPlanConfig == nil {
		t.Fatal("期望 token plan 配置被标准化")
	}
	if tokenPlanConfig.Code != "" || tokenPlanConfig.APIKey != "" || tokenPlanConfig.BaseURL != "" || tokenPlanConfig.AccessToken != "" || tokenPlanConfig.UserID != "" {
		t.Fatalf("期望 token plan 模板隐藏字段被清空，实际为 %+v", *tokenPlanConfig)
	}
	if tokenPlanConfig.TokenPlanProvider != "kimi" {
		t.Fatalf("期望 token plan 默认 provider 为 kimi，实际为 %s", tokenPlanConfig.TokenPlanProvider)
	}
}

func TestProviderQuotaQueryService_QueryQuotaAllowsCustomScriptWithoutBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/quota" {
			t.Fatalf("期望请求 /custom/quota，实际为 %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"remaining": 88}`))
	}))
	defer server.Close()

	service := NewProviderQuotaQueryService()
	result := service.QueryQuota(string(ProviderQuotaQueryTypeCustom), "", "", &ProviderQuotaQueryConfig{
		Enabled:      true,
		TemplateType: string(ProviderQuotaTemplateTypeCustom),
		Code: `({
  request: {
    url: '` + server.URL + `/custom/quota',
    method: 'GET'
  },
  extractor: function(response) {
    return {
      label: 'Quota',
      remaining: response.remaining,
      unit: 'requests',
      valueMode: 'count'
    };
  }
})`,
	})

	if !result.Success {
		t.Fatalf("期望自定义模版查询成功，实际失败：%s", result.Error)
	}
	if len(result.Items) != 1 || result.Items[0].Total != 88 {
		t.Fatalf("期望自定义模版返回 total=88，实际为 %+v", result.Items)
	}
}

func TestProviderQuotaQueryService_QueryQuotaRejectsNonCustomCrossOriginScript(t *testing.T) {
	service := NewProviderQuotaQueryService()
	result := service.QueryQuota(string(ProviderQuotaQueryTypeGeneral), "https://api.example.com", "quota-key", &ProviderQuotaQueryConfig{
		Enabled:      true,
		TemplateType: string(ProviderQuotaTemplateTypeGeneral),
		Code: `({
  request: {
    url: 'https://evil.example.com/user/balance',
    method: 'GET'
  },
  extractor: function(response) {
    return {
      remaining: response.balance,
      unit: 'USD',
      valueMode: 'currency'
    };
  }
})`,
	})

	if result.Success {
		t.Fatal("期望同源校验失败，但查询成功了")
	}
	if result.Error == "" {
		t.Fatal("期望返回同源校验错误")
	}
}

func TestProviderQuotaQueryService_QueryQuotaParsesOpenRouterBalance(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/credits" {
			t.Fatalf("期望请求 /api/v1/credits，实际为 %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer balance-key" {
			t.Fatalf("期望 Authorization 为 Bearer balance-key，实际为 %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"total_credits": 100,
				"total_usage": 30
			}
		}`))
	}))
	defer server.Close()

	parsedServerURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("解析测试服务地址失败: %v", err)
	}

	service := NewProviderQuotaQueryService()
	service.client = server.Client()
	service.client.Transport = rewriteHostTransport{target: parsedServerURL, transport: server.Client().Transport}

	result := service.QueryQuota(string(ProviderQuotaQueryTypeBalance), "https://openrouter.ai/api/v1/chat/completions", "balance-key", nil)
	if !result.Success {
		t.Fatalf("期望官方余额查询成功，实际失败：%s", result.Error)
	}
	if len(result.Items) != 1 {
		t.Fatalf("期望返回 1 条余额数据，实际为 %d", len(result.Items))
	}
	item := result.Items[0]
	if item.Label != "OpenRouter" {
		t.Fatalf("期望标签为 OpenRouter，实际为 %s", item.Label)
	}
	if item.Used != 30 || item.Total != 100 {
		t.Fatalf("期望已用/总量为 30 / 100，实际为 %f / %f", item.Used, item.Total)
	}
	if item.ValueMode != string(ProviderQuotaValueModeCurrency) || item.Unit != "USD" {
		t.Fatalf("期望金额模式为 currency/USD，实际为 %s / %s", item.ValueMode, item.Unit)
	}
}

func TestProviderQuotaQueryService_QueryQuotaReturnsInvalidItemForOfficialBalanceAuthFailure(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/credits" {
			t.Fatalf("期望请求 /api/v1/credits，实际为 %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer server.Close()

	parsedServerURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("解析测试服务地址失败: %v", err)
	}

	service := NewProviderQuotaQueryService()
	service.client = server.Client()
	service.client.Transport = rewriteHostTransport{target: parsedServerURL, transport: server.Client().Transport}

	result := service.QueryQuota(string(ProviderQuotaQueryTypeBalance), "https://openrouter.ai/api/v1/chat/completions", "balance-key", nil)
	if !result.Success {
		t.Fatalf("期望官方余额认证失败时仍返回失败项，实际失败：%s", result.Error)
	}
	if len(result.Items) != 1 {
		t.Fatalf("期望返回 1 条失败态数据，实际为 %d", len(result.Items))
	}
	item := result.Items[0]
	if item.Active {
		t.Fatal("期望失败态余额项为 inactive")
	}
	if item.Label != "OpenRouter" {
		t.Fatalf("期望标签为 OpenRouter，实际为 %s", item.Label)
	}
	if item.InvalidMessage != "官方余额查询认证失败 (HTTP 401)" {
		t.Fatalf("期望 invalidMessage 为认证失败提示，实际为 %s", item.InvalidMessage)
	}
	if item.Used != 0 || item.Total != 0 {
		t.Fatalf("期望失败态已用/总量为 0 / 0，实际为 %f / %f", item.Used, item.Total)
	}
}

func TestProviderQuotaQueryService_ValidateScriptPreset(t *testing.T) {
	service := NewProviderQuotaQueryService()
	result := service.ValidateScriptPreset(string(ProviderQuotaTemplateTypeGeneral), `({
  request: {
    url: "{{baseUrl}}/v1/usage",
    method: "GET",
    headers: { "Authorization": "Bearer {{apiKey}}" }
  },
  extractor: function(response) {
    return {
      remaining: response?.remaining ?? 0,
      unit: response?.unit ?? "USD"
    };
  }
})`)

	if !result.Valid {
		t.Fatalf("期望脚本预设校验通过，实际失败：%s", result.Error)
	}
}

func TestProviderQuotaQueryService_ValidateSub2APIScriptPreset(t *testing.T) {
	service := NewProviderQuotaQueryService()
	result := service.ValidateScriptPreset(string(ProviderQuotaTemplateTypeSub2API), `({
  request: {
    url: "{{baseUrl}}/v1/usage",
    method: "GET",
    headers: { "Authorization": "Bearer {{apiKey}}" }
  },
  extractor: function(response) {
    return response;
  }
})`)

	if !result.Valid {
		t.Fatalf("期望 Sub2API 脚本预设校验通过，实际失败：%s", result.Error)
	}
}

func TestProviderQuotaQueryService_ValidateScriptPresetRejectsMissingExtractor(t *testing.T) {
	service := NewProviderQuotaQueryService()
	result := service.ValidateScriptPreset(string(ProviderQuotaTemplateTypeGeneral), `({
  request: {
    url: "{{baseUrl}}/v1/usage",
    method: "GET"
  }
})`)

	if result.Valid {
		t.Fatal("缺少 extractor 时不应校验通过")
	}
	if result.Error == "" {
		t.Fatal("缺少 extractor 时应返回错误信息")
	}
}

func TestQuotaItemsExhaustedIgnoresInvalidItems(t *testing.T) {
	exhausted, valid := quotaItemsExhausted([]ProviderQuotaQueryItem{
		{Key: "invalid", Active: true, Total: 0, InvalidMessage: "expired"},
	})
	if exhausted || valid {
		t.Fatalf("无有效额度项时不应改变状态: exhausted=%v valid=%v", exhausted, valid)
	}
}

func TestQuotaItemsExhaustedRecognizesInactiveZeroBalance(t *testing.T) {
	exhausted, valid := quotaItemsExhausted([]ProviderQuotaQueryItem{
		{Key: "balance", Active: false, Total: 0},
	})
	if !exhausted || !valid {
		t.Fatalf("官方零余额项应判定为额度耗尽: exhausted=%v valid=%v", exhausted, valid)
	}
}

func TestQuotaItemsExhaustedSkipsUnlimitedSubscription(t *testing.T) {
	exhausted, valid := quotaItemsExhausted([]ProviderQuotaQueryItem{
		{Key: "balance", Active: true, Total: 0, Unlimited: true},
	})
	if exhausted || !valid {
		t.Fatalf("无限订阅不应判定为额度耗尽: exhausted=%v valid=%v", exhausted, valid)
	}
}

func TestQuotaItemsExhaustedWhenAnyValidItemHasNoRemainingQuota(t *testing.T) {
	exhausted, valid := quotaItemsExhausted([]ProviderQuotaQueryItem{
		{Key: "daily", Active: true, Used: 1, Total: 10},
		{Key: "monthly", Active: true, Used: 20, Total: 20},
	})
	if !exhausted || !valid {
		t.Fatalf("任一有效额度项耗尽时应判定耗尽: exhausted=%v valid=%v", exhausted, valid)
	}
}

func TestResolveProviderQuotaAutomationState(t *testing.T) {
	tests := []struct {
		name                                  string
		enabled, autoDisabled, paused         bool
		exhausted                             bool
		wantEnabled, wantDisabled, wantPaused bool
	}{
		{name: "enabled provider is auto disabled", enabled: true, exhausted: true, wantDisabled: true},
		{name: "manually disabled provider stays disabled", exhausted: true},
		{name: "auto disabled provider recovers", autoDisabled: true, wantEnabled: true},
		{name: "temporary enable stays enabled while exhausted", enabled: true, paused: true, exhausted: true, wantEnabled: true, wantPaused: true},
		{name: "temporary enable clears after recovery", enabled: true, paused: true, wantEnabled: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			enabled, autoDisabled, paused := resolveProviderQuotaAutomationState(testCase.enabled, testCase.autoDisabled, testCase.paused, testCase.exhausted)
			if enabled != testCase.wantEnabled || autoDisabled != testCase.wantDisabled || paused != testCase.wantPaused {
				t.Fatalf("状态不符合预期: got=(%v,%v,%v) want=(%v,%v,%v)", enabled, autoDisabled, paused, testCase.wantEnabled, testCase.wantDisabled, testCase.wantPaused)
			}
		})
	}
}

func TestNormalizeProviderQuotaAutomationOnSaveClearsStateWhenConfigRemoved(t *testing.T) {
	enabled := false
	autoDisabled := true
	paused := false
	normalizeProviderQuotaAutomationOnSave(&enabled, &autoDisabled, &paused, "none", nil)
	if !enabled || autoDisabled || paused {
		t.Fatalf("移除额度配置后应恢复自动停用项: enabled=%v autoDisabled=%v paused=%v", enabled, autoDisabled, paused)
	}

	enabled = false
	normalizeProviderQuotaAutomationOnSave(&enabled, &autoDisabled, &paused, "none", nil)
	if enabled {
		t.Fatal("手动关闭的供应商不应因移除额度配置而自动启用")
	}
}
