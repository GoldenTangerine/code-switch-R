/*
@name: 会话供应商活动关联
@Descripttion: 将实际转发供应商与 CLI hooks 会话键关联并发布无正文快照。
@version: 1.0.0
@Author: sm
@Date: 2026-09-09 12:08:00
@LastEditTime: 2026-09-09 12:08:00
@FilePath: services/traysessionroutes.go
*/
package services

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
)

type TraySessionBinding struct {
	SessionKey   string `json:"sessionKey"`
	ProviderID   string `json:"providerId"`
	ProviderName string `json:"providerName"`
	Icon         string `json:"icon"`
	Sequence     uint64 `json:"sequence"`
	UpdatedAt    int64  `json:"updatedAt"`
}

type traySessionRoute struct {
	platform string
	binding  TraySessionBinding
}

type traySessionRoutes struct {
	mu       sync.Mutex
	sequence uint64
	routes   map[string]traySessionRoute
}

func hookSessionKey(platform, id string) string {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 512 || (platform != "claude" && platform != "codex") {
		return ""
	}
	sum := sha256.Sum256([]byte(platform + "\n" + id))
	return hex.EncodeToString(sum[:])
}

func hookSessionID(platform string, body []byte, headers map[string]string) string {
	if platform == "claude" {
		// Native subagents share the parent CLI id. Their different routes must
		// not move the parent's waiting indicator to a child's supplier.
		if getHeaderValueCaseInsensitive(headers, "x-claude-code-agent-id") != "" {
			return ""
		}
		return claudeSessionIDFromRequest(body, headers)
	}
	if platform != "codex" || getHeaderValueCaseInsensitive(headers, "x-cursor-conversation-id") != "" {
		return ""
	}
	for _, name := range []string{"x-codex-thread-id", "x-codex-session-id", "session_id"} {
		if value := getHeaderValueCaseInsensitive(headers, name); value != "" {
			return value
		}
	}
	metadata := getHeaderValueCaseInsensitive(headers, "x-codex-turn-metadata")
	if value := firstNonEmptyGJSON(gjson.Parse(metadata), "thread_id", "threadId", "session_id", "sessionId"); value != "" {
		return value
	}
	if value := relaySessionField(body, "thread_id", "threadId", "session_id", "sessionId", "client_metadata.thread_id", "client_metadata.session_id"); value != "" {
		return value
	}
	clientMetadata := gjson.GetBytes(body, "client_metadata")
	for _, field := range []string{"x-codex-turn-metadata", "x_codex_turn_metadata"} {
		metadata := clientMetadata.Get(field)
		if metadata.Type != gjson.String || !gjson.Valid(metadata.String()) {
			continue
		}
		if value := firstNonEmptyGJSON(gjson.Parse(metadata.String()), "thread_id", "threadId", "session_id", "sessionId"); value != "" {
			return value
		}
	}
	return ""
}

func (r *traySessionRoutes) record(platform, sessionKey string, provider Provider, now time.Time) {
	if sessionKey == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.routes == nil {
		r.routes = make(map[string]traySessionRoute)
	}
	r.prune(now)
	r.sequence++
	icon := provider.Icon
	if icon == "" {
		icon = "openai"
	}
	r.routes[sessionKey] = traySessionRoute{platform: platform, binding: TraySessionBinding{
		SessionKey: sessionKey, ProviderID: providerRefFromProvider(provider), ProviderName: provider.Name,
		Icon: icon, Sequence: r.sequence, UpdatedAt: now.UnixMilli(),
	}}
	if len(r.routes) > 4096 {
		oldestKey := ""
		oldest := r.sequence
		for key, route := range r.routes {
			if route.binding.Sequence < oldest {
				oldest, oldestKey = route.binding.Sequence, key
			}
		}
		delete(r.routes, oldestKey)
	}
}

func (r *traySessionRoutes) prune(now time.Time) {
	for key, route := range r.routes {
		if now.Sub(time.UnixMilli(route.binding.UpdatedAt)) > 24*time.Hour {
			delete(r.routes, key)
		}
	}
}

func (r *traySessionRoutes) snapshot(platform string, now time.Time) []TraySessionBinding {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prune(now)
	var result []TraySessionBinding
	for _, route := range r.routes {
		if route.platform == platform {
			result = append(result, route.binding)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].SessionKey < result[j].SessionKey })
	return result
}

func (s *ProviderConcurrencyService) hookSessionBindings(platform string, now time.Time) []TraySessionBinding {
	if s == nil || s.relay == nil {
		return nil
	}
	return s.relay.trayRoutes.snapshot(platform, now)
}
