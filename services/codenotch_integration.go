/**
 * @name: Codenotch 按需联动
 * @Descripttion: 根据本地订阅发布托管且启用的供应商并回收额外查询资源。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-09-10 16:30:00
 * @LastEditTime: 2026-09-10 16:30:00
 * @FilePath: services/codenotch_integration.go
 */
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"
)

const codenotchLeaseFile = "codenotch-subscription-v1.json"
const codenotchDataFile = "codenotch-providers-v1.json"
const codenotchMaxDataBytes = 2_000_000

type CodenotchSnapshotInfo struct {
	Version         int    `json:"version"`
	Mode            string `json:"mode"`
	ConsumerSession string `json:"consumerSession"`
	Revision        uint64 `json:"revision"`
	Error           bool   `json:"error"`
}

type CodenotchProviderSnapshot struct {
	Version   int                    `json:"version"`
	Session   string                 `json:"session"`
	Revision  uint64                 `json:"revision"`
	Platforms []TraySnapshotPlatform `json:"platforms"`
}

type codenotchSubscription struct {
	Version     int     `json:"version"`
	Session     string  `json:"session"`
	Mode        string  `json:"mode"`
	HeartbeatAt float64 `json:"heartbeatAt"`
}

type codenotchIntegration struct {
	proxyStatus func(string) (bool, error)
	info        *CodenotchSnapshotInfo
	next        time.Time
	wanted      map[string]trayProviderInput
	platforms   []TraySnapshotPlatform
	revision    uint64
	fileInfo    os.FileInfo
}

// BindCodenotchProxyStatus reuses the same read-only hosting checks as the UI.
func (s *TraySnapshotService) BindCodenotchProxyStatus(status func(string) (bool, error)) {
	s.integration.proxyStatus = status
}

func readCodenotchSubscription(path string, now time.Time) (codenotchSubscription, error) {
	var result codenotchSubscription
	f, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil || len(data) > 4096 {
		return result, fmt.Errorf("invalid subscription size")
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	// Codenotch 1.6.12 encoded milliseconds as a fractional JSON number.
	// Compare in milliseconds without narrowing untrusted numbers to int64.
	ageMillis := float64(now.UnixMicro())/1000 - result.HeartbeatAt
	if result.Version != 1 || result.Mode != "enabled" || len(result.Session) == 0 || len(result.Session) > 128 || !(ageMillis >= -1000 && ageMillis <= 15000) {
		return result, fmt.Errorf("inactive subscription")
	}
	return result, nil
}

func (s *TraySnapshotService) enabledPlatformOrder(tray []TraySnapshotPlatform) ([]TraySnapshotPlatform, map[string]CustomCliTool) {
	customError := false
	customTools := make(map[string]CustomCliTool)
	known := []TraySnapshotPlatform{
		{Platform: "claude", Name: "Claude Code", Icon: "claude"},
		{Platform: "codex", Name: "Codex", Icon: "openai"},
		{Platform: "gemini", Name: "Gemini", Icon: "gemini"},
		{Platform: "grokbuild", Name: "Grok Build", Icon: "grok"},
		{Platform: "claude-desktop", Name: "Claude Desktop", Icon: "claude"},
	}
	if s.custom != nil {
		tools, err := s.custom.ListTools()
		customError = err != nil
		if err == nil {
			sort.SliceStable(tools, func(i, j int) bool { return tools[i].ID < tools[j].ID })
			for _, tool := range tools {
				if tool.ID != "" {
					customTools["custom:"+tool.ID] = tool
					known = append(known, TraySnapshotPlatform{Platform: "custom:" + tool.ID, Name: tool.Name, Icon: "others"})
				}
			}
		}
	}
	byID := make(map[string]TraySnapshotPlatform, len(known))
	for _, p := range known {
		byID[p.Platform] = p
	}
	result := make([]TraySnapshotPlatform, 0, len(known))
	for _, p := range append(tray, known...) {
		if value, ok := byID[p.Platform]; ok {
			value.Providers = []TraySnapshotProvider{}
			result = append(result, value)
			delete(byID, p.Platform)
		}
	}
	if customError {
		result = append(result, TraySnapshotPlatform{Platform: "custom", Name: "Custom CLI", Icon: "others", Error: true, Providers: []TraySnapshotProvider{}})
	}
	return result, customTools
}

func (s *TraySnapshotService) collectCodenotch(ctx context.Context, now time.Time, tray []TraySnapshotPlatform) {
	i := &s.integration
	if now.Before(i.next) {
		return
	}
	i.next = now.Add(time.Second)
	var lease codenotchSubscription
	var err error
	if s.path != "" && i.proxyStatus != nil {
		lease, err = readCodenotchSubscription(filepath.Join(filepath.Dir(s.path), codenotchLeaseFile), now)
	}
	if s.path == "" || i.proxyStatus == nil || err != nil {
		if i.info == nil || i.info.Mode != "tray" {
			if i.platforms != nil {
				s.removeCodenotchData()
			}
			i.info = &CodenotchSnapshotInfo{Version: 1, Mode: "tray"}
			i.wanted, i.platforms = nil, nil
			i.fileInfo = nil
		}
		return
	}
	platforms, customTools := s.enabledPlatformOrder(tray)
	wanted := make(map[string]trayProviderInput)
	for index := range platforms {
		p := &platforms[index]
		if p.Error {
			continue
		}
		p.SessionBindings = s.concurrency.hookSessionBindings(p.Platform, now)
		var enabled bool
		var err error
		if tool, ok := customTools[p.Platform]; ok {
			enabled = s.custom.proxyStatusForTool(tool).Enabled
		} else {
			enabled, err = i.proxyStatus(p.Platform)
		}
		if err != nil {
			p.Error = true
			continue
		}
		if !enabled {
			continue
		}
		inputs, err := s.inputs(p.Platform)
		if err != nil {
			p.Error = true
			continue
		}
		for _, input := range inputs {
			if !input.provider.Enabled {
				continue
			}
			item := TraySnapshotProvider{ProviderID: input.ref, ProviderName: input.provider.Name, Icon: input.provider.Icon, Status: "enabled", Quotas: []TraySnapshotQuota{}}
			if s.concurrency != nil && s.concurrency.relay != nil {
				item.ActiveRequests = s.concurrency.relay.providerConcurrencyCount(p.Platform, input.ref)
			}
			if item.ActiveRequests > 0 {
				item.Status = "active"
			}
			s.decorateTrayProvider(p.Platform, &item, input, now, wanted)
			p.Providers = append(p.Providers, item)
		}
	}
	i.wanted = wanted
	info := &CodenotchSnapshotInfo{Version: 1, Mode: "enabled", ConsumerSession: lease.Session, Revision: i.revision}
	dataPath := filepath.Join(filepath.Dir(s.path), codenotchDataFile)
	fileInfo, fileErr := os.Stat(dataPath)
	fileChanged := fileErr != nil || i.fileInfo == nil || !os.SameFile(i.fileInfo, fileInfo) ||
		i.fileInfo.Size() != fileInfo.Size() || !i.fileInfo.ModTime().Equal(fileInfo.ModTime())
	if i.platforms == nil || fileChanged || !reflect.DeepEqual(i.platforms, platforms) {
		payload := CodenotchProviderSnapshot{Version: 1, Session: s.snapshot.Session, Revision: i.revision + 1, Platforms: platforms}
		data, err := json.Marshal(payload)
		if err != nil || len(data) > codenotchMaxDataBytes {
			info.Error = true
		} else if err := writeCodenotchData(dataPath, data); err != nil {
			info.Error = true
		} else {
			i.revision++
			i.platforms = platforms
			info.Revision = i.revision
			i.fileInfo, err = os.Stat(dataPath)
			info.Error = err != nil
		}
	}
	i.info = info
}

func (s *TraySnapshotService) removeCodenotchData() {
	if s.path == "" {
		return
	}
	path := filepath.Join(filepath.Dir(s.path), codenotchDataFile)
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	var snapshot CodenotchProviderSnapshot
	if json.NewDecoder(io.LimitReader(f, codenotchMaxDataBytes+1)).Decode(&snapshot) == nil && snapshot.Session == s.snapshot.Session {
		_ = os.Remove(path)
	}
}
