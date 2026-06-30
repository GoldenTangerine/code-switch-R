/**
 * @name: 托管运行态服务
 * @Descripttion: 仅向前端暴露托管运行态只读信息
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-06-30 15:20:00
 * @LastEditTime: 2026-06-30 15:20:00
 * @FilePath: services/providerrelaystateservice.go
 */
package services

type ProviderRelayStateService struct {
	relay *ProviderRelayService
}

func NewProviderRelayStateService(relay *ProviderRelayService) *ProviderRelayStateService {
	return &ProviderRelayStateService{relay: relay}
}

func (s *ProviderRelayStateService) GetAllLastUsedProviders() map[string]*LastUsedProvider {
	if s == nil || s.relay == nil {
		return map[string]*LastUsedProvider{}
	}
	return s.relay.GetAllLastUsedProviders()
}
