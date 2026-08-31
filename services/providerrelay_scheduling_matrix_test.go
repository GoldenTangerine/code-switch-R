/**
 * @name: 通用代理调度行为矩阵测试
 * @Descripttion: 固定 Codex 通用代理在降级、轮询、黑名单、强制优先和会话亲和组合下的调度语义
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-31 15:13:53
 * @LastEditTime: 2026-08-31 15:13:53
 * @FilePath: services/providerrelay_scheduling_matrix_test.go
 */

package services

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/daodao97/xgo/xdb"
	"github.com/gin-gonic/gin"
)

func TestProxyHandlerSchedulingBehaviorMatrix(t *testing.T) {
	type testCase struct {
		name                   string
		blacklistEnabled       bool
		roundRobinEnabled      bool
		sessionAffinityEnabled bool
		forcedProvider         string
		blacklistedProvider    string
		roundRobinLastProvider string
		boundProvider          string
		failedProviders        map[string]bool
		wantCalls              []string
		wantSuccessProvider    string
		wantFailureCounts      map[string]int
		wantBoundProvider      string
	}

	tests := []testCase{
		{
			name:                "sequential fallback keeps level and saved order",
			failedProviders:     map[string]bool{"A": true, "B": true, "C": true},
			wantCalls:           []string{"A", "B", "C", "D"},
			wantSuccessProvider: "D",
		},
		{
			name:                   "round robin rotates only within the same level",
			roundRobinEnabled:      true,
			roundRobinLastProvider: "A",
			failedProviders:        map[string]bool{"A": true, "B": true, "C": true},
			wantCalls:              []string{"B", "C", "A", "D"},
			wantSuccessProvider:    "D",
		},
		{
			name:                   "fixed blacklist skips blocked provider and overrides round robin",
			blacklistEnabled:       true,
			roundRobinEnabled:      true,
			blacklistedProvider:    "A",
			roundRobinLastProvider: "B",
			failedProviders:        map[string]bool{"B": true, "C": true},
			wantCalls:              []string{"B", "C", "D"},
			wantSuccessProvider:    "D",
			wantFailureCounts:      map[string]int{"B": 1, "C": 1},
		},
		{
			name:                "forced provider runs before lower numeric levels",
			forcedProvider:      "B",
			failedProviders:     map[string]bool{"A": true, "B": true, "C": true},
			wantCalls:           []string{"B", "A", "C", "D"},
			wantSuccessProvider: "D",
		},
		{
			name:                   "confirmed session provider runs first and migrates after failover",
			sessionAffinityEnabled: true,
			boundProvider:          "B",
			failedProviders:        map[string]bool{"B": true},
			wantCalls:              []string{"B", "A"},
			wantSuccessProvider:    "A",
			wantBoundProvider:      "A",
		},
		{
			name:                   "fixed blacklist keeps confirmed session provider first and migrates after failover",
			blacklistEnabled:       true,
			sessionAffinityEnabled: true,
			boundProvider:          "B",
			failedProviders:        map[string]bool{"B": true},
			wantCalls:              []string{"B", "A"},
			wantSuccessProvider:    "A",
			wantFailureCounts:      map[string]int{"B": 1},
			wantBoundProvider:      "A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useIsolatedHomeDir(t)
			gin.SetMode(gin.TestMode)

			db, err := xdb.DB("default")
			if err != nil {
				t.Fatalf("获取测试数据库失败: %v", err)
			}
			blacklistEnabled := "false"
			if tt.blacklistEnabled {
				blacklistEnabled = "true"
			}
			if _, err := db.Exec(`UPDATE app_settings SET value = ? WHERE key = 'enable_blacklist'`, blacklistEnabled); err != nil {
				t.Fatalf("设置黑名单开关失败: %v", err)
			}

			var callsMu sync.Mutex
			calls := make([]string, 0, 4)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				providerName := strings.ToUpper(strings.Split(strings.Trim(r.URL.Path, "/"), "/")[0])
				callsMu.Lock()
				calls = append(calls, providerName)
				callsMu.Unlock()

				w.Header().Set("Content-Type", "application/json")
				if tt.failedProviders[providerName] {
					w.WriteHeader(http.StatusBadGateway)
					_, _ = fmt.Fprintf(w, `{"error":"%s failed"}`, providerName)
					return
				}
				_, _ = fmt.Fprintf(w, `{"id":"resp_%s","status":"completed","model":"gpt-5.3-codex","output":[],"usage":{"input_tokens":1,"output_tokens":1}}`, providerName)
			}))
			defer upstream.Close()

			providers := []Provider{
				{ID: 1, Name: "A", APIURL: upstream.URL + "/a", APIKey: "key-a", Enabled: true, Level: 1},
				{ID: 2, Name: "B", APIURL: upstream.URL + "/b", APIKey: "key-b", Enabled: true, Level: 1},
				{ID: 3, Name: "C", APIURL: upstream.URL + "/c", APIKey: "key-c", Enabled: true, Level: 1},
				{ID: 4, Name: "D", APIURL: upstream.URL + "/d", APIKey: "key-d", Enabled: true, Level: 2},
			}
			providerByName := make(map[string]Provider, len(providers))
			for i := range providers {
				if providers[i].Name == tt.forcedProvider {
					providers[i].Level = 5
					providers[i].ForcedPriority = true
				}
				if providers[i].Name == tt.boundProvider {
					providers[i].Level = 5
				}
				providerByName[providers[i].Name] = providers[i]
			}

			providerService := NewProviderService()
			if err := providerService.SaveProviders("codex", providers); err != nil {
				t.Fatalf("保存测试 Provider 失败: %v", err)
			}

			if tt.blacklistedProvider != "" {
				provider := providerByName[tt.blacklistedProvider]
				now := time.Now()
				if _, err := db.Exec(`
					INSERT INTO provider_blacklist (
						platform, provider_id, provider_name, failure_count, blacklisted_at, blacklisted_until
					) VALUES (?, ?, ?, 5, ?, ?)
				`, "codex", providerRefFromProvider(provider), provider.Name, now, now.Add(time.Hour)); err != nil {
					t.Fatalf("预置黑名单失败: %v", err)
				}
			}

			appSettings := NewAppSettingsService(nil)
			settings, err := appSettings.GetAppSettings()
			if err != nil {
				t.Fatalf("读取应用设置失败: %v", err)
			}
			settings.EnableRoundRobin = tt.roundRobinEnabled
			settings.SessionAffinityEnabled["codex"] = tt.sessionAffinityEnabled
			if _, err := appSettings.SaveAppSettings(settings); err != nil {
				t.Fatalf("保存应用设置失败: %v", err)
			}

			blacklistService := NewBlacklistService(NewSettingsService(), nil)
			relay := NewProviderRelayService(providerService, nil, blacklistService, nil, appSettings, nil, "")
			if tt.roundRobinLastProvider != "" {
				relay.markRoundRobinProviderAttempt("codex", providerByName[tt.roundRobinLastProvider])
			}

			requestBody := `{"model":"gpt-5.3-codex","input":"hello","thread_id":"matrix-session"}`
			sessionHash := deriveRelaySessionIdentity("codex", []byte(requestBody)).NodeHash
			if tt.boundProvider != "" {
				provider := providerByName[tt.boundProvider]
				relay.rememberSessionRelation("codex", deriveRelaySessionIdentity("codex", []byte(requestBody)))
				attemptID := relay.beginSessionProviderRequest(
					"codex",
					sessionHash,
					providerRefFromProvider(provider),
					provider.Name,
					"codex-test",
					providerSessionMaxSessions(provider),
					providerSessionTTLMinutes(provider),
					false,
					false,
					false,
				)
				relay.finishSessionProviderRequest("codex", sessionHash, providerRefFromProvider(provider))
				relay.confirmSessionProviderBinding("codex", sessionHash, attemptID)
			}

			router := gin.New()
			relay.registerRoutes(router)
			req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("代理状态码=%d body=%s", w.Code, w.Body.String())
			}
			callsMu.Lock()
			gotCalls := append([]string(nil), calls...)
			callsMu.Unlock()
			if !reflect.DeepEqual(gotCalls, tt.wantCalls) {
				t.Fatalf("Provider 调用顺序=%v，期望=%v", gotCalls, tt.wantCalls)
			}
			if !strings.Contains(w.Body.String(), `"id":"resp_`+tt.wantSuccessProvider+`"`) {
				t.Fatalf("响应未来自期望 Provider %s: %s", tt.wantSuccessProvider, w.Body.String())
			}

			lastUsed := relay.GetLastUsedProvider("codex")
			wantSuccess := providerByName[tt.wantSuccessProvider]
			if lastUsed == nil || lastUsed.ProviderID != providerRefFromProvider(wantSuccess) || lastUsed.ProviderName != wantSuccess.Name {
				t.Fatalf("最后使用 Provider=%#v，期望=%s/%s", lastUsed, providerRefFromProvider(wantSuccess), wantSuccess.Name)
			}

			for providerName, wantFailureCount := range tt.wantFailureCounts {
				provider := providerByName[providerName]
				var gotFailureCount int
				if err := db.QueryRow(`
					SELECT failure_count FROM provider_blacklist
					WHERE platform = ? AND provider_id = ?
				`, "codex", providerRefFromProvider(provider)).Scan(&gotFailureCount); err != nil {
					t.Fatalf("读取 %s 失败计数失败: %v", providerName, err)
				}
				if gotFailureCount != wantFailureCount {
					t.Fatalf("%s 失败计数=%d，期望=%d", providerName, gotFailureCount, wantFailureCount)
				}
			}

			if tt.wantBoundProvider != "" {
				binding := relay.getSessionBindingSnapshot("codex", sessionHash)
				wantProvider := providerByName[tt.wantBoundProvider]
				if binding == nil || binding.ProviderID != providerRefFromProvider(wantProvider) {
					t.Fatalf("会话绑定=%#v，期望迁移到 %s", binding, tt.wantBoundProvider)
				}
			}
		})
	}
}
