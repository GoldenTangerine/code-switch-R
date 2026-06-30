/**
 * @name: 会话隔离服务
 * @Descripttion: 仅向前端暴露会话隔离运行态查询与释放能力
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-06-30 14:30:00
 * @LastEditTime: 2026-06-30 14:30:00
 * @FilePath: services/sessionaffinityservice.go
 */
package services

type SessionAffinityService struct {
	relay *ProviderRelayService
}

func NewSessionAffinityService(relay *ProviderRelayService) *SessionAffinityService {
	return &SessionAffinityService{relay: relay}
}

func (s *SessionAffinityService) GetSessionAffinityStatuses(platform string) []ProviderSessionStatus {
	if s == nil || s.relay == nil {
		return []ProviderSessionStatus{}
	}
	return s.relay.GetSessionAffinityStatuses(platform)
}

func (s *SessionAffinityService) ReleaseProviderSessions(platform string, providerID string) {
	if s == nil || s.relay == nil {
		return
	}
	s.relay.ReleaseProviderSessions(platform, providerID)
}
