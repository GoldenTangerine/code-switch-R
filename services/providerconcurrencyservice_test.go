/**
 * @name: 供应商并发服务测试
 * @Descripttion: 验证供应商并发状态批量查询的平台归一化与空服务行为
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-18 15:32:07
 * @LastEditTime: 2026-08-18 15:32:07
 * @FilePath: services/providerconcurrencyservice_test.go
 */
package services

import (
	"testing"
	"time"
)

func TestProviderConcurrencyServiceGetProviderConcurrencyStatusesBatch(t *testing.T) {
	service := NewProviderConcurrencyService(&ProviderRelayService{})
	result := service.GetProviderConcurrencyStatusesBatch([]string{" claude ", "claude", "", "codex"})

	if len(result) != 2 {
		t.Fatalf("批量查询平台数量 = %d，期望 2", len(result))
	}
	if _, exists := result["claude"]; !exists {
		t.Fatal("批量查询结果缺少 claude")
	}
	if _, exists := result["codex"]; !exists {
		t.Fatal("批量查询结果缺少 codex")
	}
}

func TestProviderConcurrencyServiceGetProviderConcurrencyStatusesBatchHandlesNilService(t *testing.T) {
	var service *ProviderConcurrencyService
	result := service.GetProviderConcurrencyStatusesBatch([]string{"claude"})
	if len(result) != 0 {
		t.Fatalf("空服务批量查询结果数量 = %d，期望 0", len(result))
	}
}

func TestProviderConcurrencyServiceGetTrayProviderRuntimeStatesBatchNormalizesPlatforms(t *testing.T) {
	service := NewProviderConcurrencyService(&ProviderRelayService{})
	result := service.GetTrayProviderRuntimeStatesBatch([]string{" claude ", "claude", "", "gemini"})

	if len(result) != 2 {
		t.Fatalf("托盘运行态平台数量 = %d，期望 2", len(result))
	}
	if _, exists := result["claude"]; !exists {
		t.Fatal("托盘运行态缺少 claude")
	}
	if _, exists := result["gemini"]; !exists {
		t.Fatal("托盘运行态缺少 gemini")
	}
}

func TestProviderConcurrencyServiceGetTrayProviderRuntimeStatesBatchHandlesNilService(t *testing.T) {
	var service *ProviderConcurrencyService
	result := service.GetTrayProviderRuntimeStatesBatch([]string{"claude"})
	if len(result) != 0 {
		t.Fatalf("空服务托盘运行态数量 = %d，期望 0", len(result))
	}
}

func TestPreviewNextProviderUsesRoundRobinWithoutAdvancingCursor(t *testing.T) {
	relay := &ProviderRelayService{
		rrLastStart:         map[string]string{"claude:1": "1"},
		providerConcurrency: map[string]int{},
		sessionAffinity:     map[string]*providerSessionBinding{},
	}
	providers := []Provider{
		{ID: 1, Name: "First", APIURL: "https://first.example.com", APIKey: "key", Enabled: true, Level: 1},
		{ID: 2, Name: "Second", APIURL: "https://second.example.com", APIKey: "key", Enabled: true, Level: 1},
	}

	preview := relay.previewNextProviderFromProviders("claude", providers, trayProviderPreviewOptions{
		roundRobinEnabled:      true,
		sessionAffinityEnabled: true,
	})

	if preview == nil || preview.ProviderID != "2" {
		t.Fatalf("默认供应商 = %+v，期望 Second", preview)
	}
	if relay.rrLastStart["claude:1"] != "1" {
		t.Fatalf("只读预览推进了轮询游标：%q", relay.rrLastStart["claude:1"])
	}
}

func TestPreviewNextGeminiProviderUsesRoundRobinWhenNoSessionLoad(t *testing.T) {
	relay := &ProviderRelayService{
		rrLastStart:         map[string]string{"gemini:1": "gemini-1"},
		providerConcurrency: map[string]int{},
		sessionAffinity:     map[string]*providerSessionBinding{},
	}
	providers := []GeminiProvider{
		{ID: "gemini-1", Name: "First", BaseURL: "https://one.example.com", Enabled: true, Level: 1},
		{ID: "gemini-2", Name: "Second", BaseURL: "https://two.example.com", Enabled: true, Level: 1},
	}

	preview := relay.previewNextGeminiProvider(providers, trayProviderPreviewOptions{
		roundRobinEnabled:      true,
		sessionAffinityEnabled: true,
	})

	if preview == nil || preview.ProviderID != "gemini-2" {
		t.Fatalf("Gemini 默认供应商 = %+v，期望 Second", preview)
	}
}

func TestPreviewNextProviderSkipsBlacklistedAndConcurrencyFullProviders(t *testing.T) {
	limit := 1
	blacklist := &BlacklistService{}
	blacklist.runtimeSnapshot.Store(blacklistRuntimeSnapshot{
		enabled: true,
		blacklistedUntil: map[string]time.Time{
			blacklistRuntimeKey("codex", "1"): time.Now().Add(time.Hour),
		},
	})
	relay := &ProviderRelayService{
		blacklistService: blacklist,
		rrLastStart:      map[string]string{},
		providerConcurrency: map[string]int{
			providerConcurrencyStateKey("codex", "2"): 1,
		},
		sessionAffinity: map[string]*providerSessionBinding{},
	}
	providers := []Provider{
		{ID: 1, Name: "Blacklisted", APIURL: "https://one.example.com", APIKey: "key", Enabled: true, Level: 1},
		{ID: 2, Name: "Busy", APIURL: "https://two.example.com", APIKey: "key", Enabled: true, Level: 1, ProviderConcurrencyLimit: &limit},
		{ID: 3, Name: "Available", APIURL: "https://three.example.com", APIKey: "key", Enabled: true, Level: 2},
	}

	preview := relay.previewNextProviderFromProviders("codex", providers, trayProviderPreviewOptions{
		concurrencyLimitEnabled: true,
	})

	if preview == nil || preview.ProviderID != "3" {
		t.Fatalf("默认供应商 = %+v，期望 Available", preview)
	}
}

func TestPreviewNextProviderReturnsNilWhenNoProviderIsAvailable(t *testing.T) {
	zeroLimit := 0
	relay := &ProviderRelayService{
		rrLastStart:         map[string]string{},
		providerConcurrency: map[string]int{},
		sessionAffinity:     map[string]*providerSessionBinding{},
	}
	providers := []Provider{
		{ID: 1, Name: "Disabled", APIURL: "https://one.example.com", APIKey: "key", Enabled: false},
		{ID: 2, Name: "Missing auth", APIURL: "https://two.example.com", Enabled: true},
		{ID: 3, Name: "Full", APIURL: "https://three.example.com", APIKey: "key", Enabled: true, ProviderConcurrencyLimit: &zeroLimit},
	}

	preview := relay.previewNextProviderFromProviders("claude", providers, trayProviderPreviewOptions{
		concurrencyLimitEnabled: true,
	})

	if preview != nil {
		t.Fatalf("默认供应商 = %+v，期望无可用供应商", preview)
	}
}

func TestPreviewNextProviderUsesSessionCapacityAndLoad(t *testing.T) {
	now := time.Now()
	relay := &ProviderRelayService{
		rrLastStart:         map[string]string{},
		providerConcurrency: map[string]int{},
		sessionAffinity: map[string]*providerSessionBinding{
			sessionAffinityStateKey("claude", "session-1"): {
				Platform:    "claude",
				SessionHash: "session-1",
				ProviderID:  "1",
				MaxSessions: 1,
				LastSeen:    now,
				Confirmed:   true,
			},
		},
	}
	providers := []Provider{
		{ID: 1, Name: "Full", APIURL: "https://one.example.com", APIKey: "key", Enabled: true, Level: 1, SessionMaxSessions: 1},
		{ID: 2, Name: "Free", APIURL: "https://two.example.com", APIKey: "key", Enabled: true, Level: 1, SessionMaxSessions: 1},
	}

	preview := relay.previewNextProviderFromProviders("claude", providers, trayProviderPreviewOptions{
		sessionAffinityEnabled: true,
	})

	if preview == nil || preview.ProviderID != "2" {
		t.Fatalf("默认供应商 = %+v，期望 Free", preview)
	}
}

func TestPreviewNextGeminiProviderUsesRuntimeOrder(t *testing.T) {
	relay := &ProviderRelayService{
		rrLastStart:         map[string]string{"gemini:1": "gemini-1"},
		providerConcurrency: map[string]int{},
		sessionAffinity:     map[string]*providerSessionBinding{},
	}
	providers := []GeminiProvider{
		{ID: "gemini-1", Name: "First", BaseURL: "https://one.example.com", Enabled: true, Level: 1},
		{ID: "gemini-2", Name: "Second", BaseURL: "https://two.example.com", Enabled: true, Level: 1},
	}

	preview := relay.previewNextGeminiProvider(providers, trayProviderPreviewOptions{
		roundRobinEnabled: true,
	})

	if preview == nil || preview.ProviderID != "gemini-2" {
		t.Fatalf("Gemini 默认供应商 = %+v，期望 Second", preview)
	}
	if relay.rrLastStart["gemini:1"] != "gemini-1" {
		t.Fatalf("Gemini 只读预览推进了轮询游标：%q", relay.rrLastStart["gemini:1"])
	}
}
