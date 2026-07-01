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
