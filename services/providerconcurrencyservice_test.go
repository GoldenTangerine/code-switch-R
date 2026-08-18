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

import "testing"

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
