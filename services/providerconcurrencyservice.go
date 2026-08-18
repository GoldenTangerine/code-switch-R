/**
 * @name: 供应商并发服务
 * @Descripttion: 向前端暴露供应商实时并发运行态查询能力
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-01 11:20:00
 * @LastEditTime: 2026-07-01 11:20:00
 * @FilePath: services/providerconcurrencyservice.go
 */
package services

import "fmt"

type ProviderConcurrencyService struct {
	relay *ProviderRelayService
}

func NewProviderConcurrencyService(relay *ProviderRelayService) *ProviderConcurrencyService {
	return &ProviderConcurrencyService{relay: relay}
}

func (s *ProviderConcurrencyService) GetProviderConcurrencyStatuses(platform string) []ProviderConcurrencyStatus {
	if s == nil || s.relay == nil {
		return []ProviderConcurrencyStatus{}
	}
	return s.relay.GetProviderConcurrencyStatuses(platform)
}

func (s *ProviderConcurrencyService) GetSessionSwitchCandidates(platform string, sessionNumber int64) []ProviderSessionSwitchCandidate {
	if s == nil || s.relay == nil {
		return []ProviderSessionSwitchCandidate{}
	}
	return s.relay.GetSessionSwitchCandidates(platform, sessionNumber)
}

func (s *ProviderConcurrencyService) SwitchSessionProvider(platform string, sessionNumber int64, targetProviderID string) (SessionSwitchResult, error) {
	if s == nil || s.relay == nil {
		return SessionSwitchResult{}, fmt.Errorf("供应商服务未初始化")
	}
	return s.relay.SwitchSessionProvider(platform, sessionNumber, targetProviderID)
}

func (s *ProviderConcurrencyService) ClearSessionAffinity(platform string) {
	if s == nil || s.relay == nil {
		return
	}
	s.relay.ClearSessionAffinity(platform)
}
