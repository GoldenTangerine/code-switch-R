/**
 * @name: Codenotch 按需联动回归
 * @Descripttion: 验证订阅边界、启用筛选、修订复用及任务回收并测量扩展成本。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-09-10 18:00:00
 * @LastEditTime: 2026-09-10 18:00:00
 * @FilePath: services/codenotch_integration_test.go
 */
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func codenotchTestService(t testing.TB, count int) *TraySnapshotService {
	t.Helper()
	p := &ProviderService{snapshots: make(map[string]providerConfigSnapshot)}
	providers := make([]Provider, count)
	for i := range providers {
		providers[i] = Provider{ID: int64(i + 1), Name: fmt.Sprintf("Fixture %d", i), Enabled: true}
	}
	p.storeProviderSnapshot("codex", providers, [32]byte{1}, true)
	s := NewTraySnapshotService(p, nil, nil, nil, nil, nil, nil)
	s.path = filepath.Join(t.TempDir(), "tray-snapshot-v1.json")
	s.BindCodenotchProxyStatus(func(platform string) (bool, error) { return platform == "codex", nil })
	return s
}

func codenotchLease(t testing.TB, s *TraySnapshotService, now time.Time) {
	t.Helper()
	if err := writeCodenotchJSON(filepath.Join(filepath.Dir(s.path), codenotchLeaseFile), codenotchSubscription{1, "consumer", "enabled", now.UnixMilli()}); err != nil {
		t.Fatal(err)
	}
}

func TestCodenotchSubscriptionBounds(t *testing.T) {
	now := time.Now()
	path := filepath.Join(t.TempDir(), "lease")
	for _, age := range []time.Duration{-2 * time.Second, -time.Second, 0, 15 * time.Second, 16 * time.Second} {
		if err := writeCodenotchJSON(path, codenotchSubscription{1, "consumer", "enabled", now.Add(-age).UnixMilli()}); err != nil {
			t.Fatal(err)
		}
		_, err := readCodenotchSubscription(path, time.UnixMilli(now.UnixMilli()))
		valid := age >= -time.Second && age <= 15*time.Second
		if (err == nil) != valid {
			t.Fatalf("age %s: %v", age, err)
		}
	}
	for _, bad := range []string{`{}`, `{"version":2,"session":"a","mode":"enabled"}`, strings.Repeat(" ", 4097)} {
		if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readCodenotchSubscription(path, now); err == nil {
			t.Fatal("invalid lease accepted")
		}
	}
}

func TestCodenotchEnabledHiddenPlatformsRevisionAndRevoke(t *testing.T) {
	s := codenotchTestService(t, 3)
	now := time.Now()
	calls := 0
	s.BindCodenotchProxyStatus(func(platform string) (bool, error) { calls++; return platform == "codex", nil })
	s.collectCodenotch(context.Background(), now, nil)
	if calls != 0 || len(s.cache) != 0 || s.integration.info.Mode != "tray" {
		t.Fatal("work without subscription")
	}
	codenotchLease(t, s, now)
	s.providers.storeProviderSnapshot("codex", []Provider{{ID: 1, Name: "Same", Enabled: true}, {ID: 2, Name: "Same", Enabled: false}, {ID: 3, Name: "Idle", Enabled: true}}, [32]byte{2}, true)
	tray := []TraySnapshotPlatform{{Platform: "claude"}, {Platform: "opencode"}}
	s.collectCodenotch(context.Background(), now.Add(time.Second), tray)
	if len(s.integration.wanted) != 2 || s.integration.info.Revision != 1 {
		t.Fatalf("unexpected selection: %+v", s.integration.info)
	}
	if s.integration.platforms[0].Platform != "claude" || s.integration.platforms[1].Platform != "codex" {
		t.Fatal("unstable hidden platform order")
	}
	for _, p := range s.integration.platforms {
		if p.Platform == "opencode" {
			t.Fatal("additive platform selected")
		}
		if p.Platform == "codex" && (len(p.Providers) != 2 || p.Providers[1].Status != "enabled") {
			t.Fatal("idle enabled supplier missing")
		}
	}
	path := filepath.Join(filepath.Dir(s.path), codenotchDataFile)
	first, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	s.collectCodenotch(context.Background(), now.Add(2*time.Second), tray)
	second, _ := os.Stat(path)
	if !os.SameFile(first, second) || s.integration.info.Revision != 1 {
		t.Fatal("unchanged full list was rewritten")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	s.collectCodenotch(context.Background(), now.Add(3*time.Second), tray)
	if _, err := os.Stat(path); err != nil || s.integration.info.Revision != 2 {
		t.Fatal("removed sidecar not recreated")
	}
	if err := os.Remove(filepath.Join(filepath.Dir(s.path), codenotchLeaseFile)); err != nil {
		t.Fatal(err)
	}
	s.collectCodenotch(context.Background(), now.Add(4*time.Second), tray)
	s.scheduleTrayDetails(context.Background(), now, nil)
	if s.integration.wanted != nil || s.integration.platforms != nil || len(s.cache) != 0 || len(s.inputsCache) != 0 {
		t.Fatal("extra state retained after revocation")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("owned data not removed")
	}
}

func TestCodenotchHostingAndOversizeRecovery(t *testing.T) {
	s := codenotchTestService(t, 1)
	now := time.Now()
	codenotchLease(t, s, now)
	s.BindCodenotchProxyStatus(func(string) (bool, error) { return false, nil })
	s.collectCodenotch(context.Background(), now, nil)
	if len(s.integration.wanted) != 0 {
		t.Fatal("nonhosted suppliers selected")
	}
	s.BindCodenotchProxyStatus(func(p string) (bool, error) { return p == "codex", nil })
	s.providers.storeProviderSnapshot("codex", []Provider{{ID: 1, Name: strings.Repeat("x", codenotchMaxDataBytes), Enabled: true}}, [32]byte{2}, true)
	s.collectCodenotch(context.Background(), now.Add(time.Second), nil)
	if !s.integration.info.Error {
		t.Fatal("oversized payload silently accepted")
	}
	s.providers.storeProviderSnapshot("codex", []Provider{{ID: 1, Name: "Recovered", Enabled: true}}, [32]byte{3}, true)
	s.collectCodenotch(context.Background(), now.Add(2*time.Second), nil)
	if s.integration.info.Error || len(s.integration.wanted) != 1 {
		t.Fatal("publish did not recover")
	}
}

func TestCodenotchCancelsRemovedJobsAndKeepsSharedTrayWork(t *testing.T) {
	s := codenotchTestService(t, 1)
	removed, cancelRemoved := context.WithCancel(context.Background())
	defer cancelRemoved()
	shared, cancelShared := context.WithCancel(context.Background())
	defer cancelShared()
	s.cache["codex/removed"] = &trayDetailCache{cancel: cancelRemoved, inflight: true}
	s.cache["codex/shared"] = &trayDetailCache{cancel: cancelShared, inflight: true}
	s.scheduleTrayDetails(context.Background(), time.Now(), map[string]trayProviderInput{"codex/shared": {ref: "shared"}})
	if removed.Err() == nil || shared.Err() != nil || len(s.cache) != 1 {
		t.Fatal("incorrect shared-job cancellation")
	}
}

func BenchmarkCodenotchCollection(b *testing.B) {
	for _, count := range []int{10, 100, 500} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			s := codenotchTestService(b, count)
			now := time.Now()
			codenotchLease(b, s, now)
			s.collectCodenotch(context.Background(), now, nil)
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				s.integration.next = time.Time{}
				s.collectCodenotch(context.Background(), now, nil)
			}
			b.StopTimer()
			if s.integration.revision != 1 {
				b.Fatal("unchanged collection rewrote sidecar")
			}
			data, _ := json.Marshal(CodenotchProviderSnapshot{Version: 1, Platforms: s.integration.platforms})
			b.ReportMetric(float64(len(data)), "snapshot-bytes")
		})
	}
}

func TestCodenotchQueryGenerationAndIdentityIsolation(t *testing.T) {
	s := codenotchTestService(t, 1)
	wanted := make(map[string]trayProviderInput)
	for _, pair := range [][2]string{{"custom:a/b", "c"}, {"custom:a", "b/c"}} {
		item := TraySnapshotProvider{}
		s.decorateTrayProvider(pair[0], &item, trayProviderInput{ref: pair[1]}, time.Now(), wanted)
	}
	if len(s.cache) != 2 {
		t.Fatal("supplier key collision")
	}
	for key, entry := range s.cache {
		entry.inflight, entry.requestID = true, 2
		s.acceptTrayDetails(trayDetailResult{key: key, requestID: 1, updated: time.Now()})
		if !entry.inflight || !entry.result.updated.IsZero() {
			t.Fatal("late result replaced new query")
		}
		s.acceptTrayDetails(trayDetailResult{key: key, requestID: 2, updated: time.Now()})
		if entry.inflight || entry.result.updated.IsZero() {
			t.Fatal("current result rejected")
		}
	}
}

func BenchmarkCodenotchChurn(b *testing.B) {
	for _, count := range []int{10, 100, 500} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			s := codenotchTestService(b, count)
			now := time.Now()
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				codenotchLease(b, s, now)
				s.integration.next = time.Time{}
				s.collectCodenotch(context.Background(), now, nil)
				if err := os.Remove(filepath.Join(filepath.Dir(s.path), codenotchLeaseFile)); err != nil {
					b.Fatal(err)
				}
				s.integration.next = time.Time{}
				s.collectCodenotch(context.Background(), now, nil)
				s.scheduleTrayDetails(context.Background(), now, nil)
			}
			b.StopTimer()
			if len(s.cache) != 0 || len(s.inputsCache) != 0 || len(s.integration.wanted) != 0 {
				b.Fatal("churn retained extra state")
			}
		})
	}
}

func TestCodenotchSharedWorkerLimitFairnessAndCancellation(t *testing.T) {
	started := make(chan string, 16)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- r.URL.Path
		<-r.Context().Done()
	}))
	defer server.Close()
	s := codenotchTestService(t, 1)
	s.query = NewProviderQuotaQueryService()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wanted := make(map[string]trayProviderInput)
	for n := 0; n < 8; n++ {
		input := trayProviderInput{ref: fmt.Sprint(n), provider: Provider{ProviderQuotaQueryType: "custom", ProviderQuotaQueryConfig: &ProviderQuotaQueryConfig{
			Enabled: true, TemplateType: "custom", Timeout: 30, Code: `({request:{url:"` + server.URL + `/` + fmt.Sprint(n) + `"},extractor:()=>[]})`,
		}}}
		item := TraySnapshotProvider{}
		s.decorateTrayProvider("codex", &item, input, time.Now(), wanted)
	}
	s.cache["5:codex:0"].scheduled = 50
	s.scheduleTrayDetails(ctx, time.Now(), wanted)
	for n := 0; n < 4; n++ {
		select {
		case path := <-started:
			if path == "/0" {
				t.Fatal("previously queried supplier starved fresh suppliers")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("worker did not start")
		}
	}
	if len(s.workers) != 4 {
		t.Fatal("unexpected worker bound")
	}
	s.scheduleTrayDetails(ctx, time.Now(), wanted)
	if len(started) != 0 {
		t.Fatal("duplicate or extra query")
	}
	s.scheduleTrayDetails(ctx, time.Now(), nil)
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("removed queries still running")
	}
	if len(s.workers) != 0 || len(s.cache) != 0 {
		t.Fatal("workers or details leaked")
	}
}

func TestCodenotchTrayReentryReusesQuotaUntilExpiryOrConfigChange(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	s := codenotchTestService(t, 1)
	s.query = NewProviderQuotaQueryService()
	inputs := []trayProviderInput{{ref: "one", provider: Provider{ProviderQuotaQueryType: "custom", ProviderQuotaQueryConfig: &ProviderQuotaQueryConfig{
		Enabled: true, TemplateType: "custom", Timeout: 2, Code: `({request:{url:"` + server.URL + `"},extractor:()=>[]})`,
	}}}}
	fingerprintTrayInputs(inputs)
	now := time.Now()
	key := trayDetailKey("codex", "one")
	for step := 0; step < 4; step++ {
		if step == 2 {
			now = s.cache[key].result.updated.Add(time.Minute)
		}
		if step == 3 {
			inputs[0].provider.APIURL = "https://changed.invalid"
			fingerprintTrayInputs(inputs)
		}
		wanted := make(map[string]trayProviderInput)
		item := TraySnapshotProvider{}
		s.decorateTrayProvider("codex", &item, inputs[0], now, wanted)
		s.cache[key].lastTraySeen = now
		if step == 1 && item.Loading {
			t.Fatal("fresh quota lost on reentry")
		}
		if step == 3 && !item.Loading {
			t.Fatal("changed query reused old result")
		}
		s.scheduleTrayDetails(context.Background(), now, wanted)
		s.wg.Wait()
		if step != 1 {
			select {
			case result := <-s.results:
				s.acceptTrayDetails(result)
			default:
				t.Fatalf("step %d: expected query result", step)
			}
		}
		expected := []int32{1, 1, 2, 3}[step]
		if calls.Load() != expected {
			t.Fatalf("step %d: queries=%d, want %d", step, calls.Load(), expected)
		}
		s.scheduleTrayDetails(context.Background(), now, nil)
	}
}

func TestCodenotchInactiveTrayRetentionIsBoundedAndRejectsCancelledResults(t *testing.T) {
	s := codenotchTestService(t, 1)
	now := time.Now()
	for n := 0; n < 150; n++ {
		seen := now.Add(-time.Duration(n) * time.Millisecond)
		s.cache[fmt.Sprint(n)] = &trayDetailCache{lastTraySeen: seen, result: trayDetailResult{updated: now}}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	s.cache["0"].cancel, s.cache["0"].inflight, s.cache["0"].requestID = cancel, true, 42
	s.cache["extra-only"] = &trayDetailCache{result: trayDetailResult{updated: now}}
	s.cache["expired"] = &trayDetailCache{lastTraySeen: now, result: trayDetailResult{updated: now.Add(-time.Minute)}}
	s.scheduleTrayDetails(context.Background(), now, nil)
	if len(s.cache) != 128 || s.cache["149"] != nil || s.cache["extra-only"] != nil || s.cache["expired"] != nil {
		t.Fatal("inactive result retention exceeded its time, origin or count bound")
	}
	if cancelled.Err() == nil || s.cache["0"].inflight {
		t.Fatal("removed job not cancelled")
	}
	s.acceptTrayDetails(trayDetailResult{key: "0", requestID: 42, updated: now.Add(time.Second)})
	if !s.cache["0"].result.updated.Equal(now) {
		t.Fatal("cancelled result replaced retained quota")
	}
	s.scheduleTrayDetails(context.Background(), now.Add(time.Minute), nil)
	if len(s.cache) != 0 {
		t.Fatal("expired results retained")
	}
}

func TestCodenotchSidecarExternalChangesRecover(t *testing.T) {
	s := codenotchTestService(t, 1)
	now := time.Now()
	codenotchLease(t, s, now)
	s.collectCodenotch(context.Background(), now, nil)
	path := filepath.Join(filepath.Dir(s.path), codenotchDataFile)
	for step := 0; step < 3; step++ {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		switch step {
		case 0:
			err = os.WriteFile(path, []byte(`{}`), 0600)
		case 1:
			err = os.WriteFile(path, []byte(strings.Repeat("x", len(data))), 0600)
			if err == nil {
				err = os.Chtimes(path, now.Add(-time.Hour), now.Add(-time.Hour))
			}
		case 2:
			err = writeCodenotchData(path, []byte(strings.Repeat("x", len(data))))
		}
		if err != nil {
			t.Fatal(err)
		}
		s.collectCodenotch(context.Background(), now.Add(time.Duration(step+1)*time.Second), nil)
		data, err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var recovered CodenotchProviderSnapshot
		if err := json.Unmarshal(data, &recovered); err != nil {
			t.Fatal(err)
		}
		if recovered.Session != s.snapshot.Session || recovered.Revision != uint64(step+2) || s.integration.info.Error {
			t.Fatal("damaged file did not recover with a new revision")
		}
	}
}

func codenotchCustomTools(t testing.TB, count int) (*CustomCliService, []CustomCliTool) {
	t.Helper()
	s := NewCustomCliService(":18100")
	// TestMain points the store at an isolated temporary home.
	previous, err := s.loadStore()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.saveStore(previous); err != nil {
			t.Error(err)
		}
	})
	tools := make([]CustomCliTool, count)
	dir := t.TempDir()
	for n := range tools {
		id := fmt.Sprintf("fixture-%03d", n)
		path := filepath.Join(dir, id+".json")
		tools[n] = CustomCliTool{ID: id, Name: id,
			ConfigFiles:    []ConfigFile{{ID: "config", Path: path, Format: "json"}},
			ProxyInjection: []ProxyInjection{{TargetFileID: "config", BaseUrlField: "base", AuthTokenField: "auth"}}}
		if err := writeCodenotchJSON(path, map[string]string{"base": s.baseURLWithToolPath(id), "auth": "code-switch-r"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.saveStore(&customCliStore{Tools: tools}); err != nil {
		t.Fatal(err)
	}
	return s, tools
}

func TestCodenotchCustomHostingSharesToolSnapshotAndReflectsEdits(t *testing.T) {
	custom, tools := codenotchCustomTools(t, 2)
	s := codenotchTestService(t, 0)
	s.custom = custom
	for _, tool := range tools {
		s.providers.storeProviderSnapshot("custom:"+tool.ID, []Provider{{ID: 1, Name: tool.Name, Enabled: true}}, [32]byte{1}, true)
	}
	s.BindCodenotchProxyStatus(func(platform string) (bool, error) {
		if strings.HasPrefix(platform, "custom:") {
			t.Fatal("custom tool configuration reparsed through resolver")
		}
		return false, nil
	})
	now := time.Now()
	codenotchLease(t, s, now)
	s.collectCodenotch(context.Background(), now, nil)
	if len(s.integration.wanted) != 2 {
		t.Fatal("hosted custom suppliers missing")
	}
	for _, token := range []string{"code-switch", "invalid"} {
		if err := writeCodenotchJSON(tools[0].ConfigFiles[0].Path, map[string]string{"base": custom.baseURLWithToolPath(tools[0].ID), "auth": token}); err != nil {
			t.Fatal(err)
		}
		status, err := custom.ProxyStatus(tools[0].ID)
		if err != nil || status.Enabled != (token == "code-switch") {
			t.Fatal("public proxy check changed semantics")
		}
		now = now.Add(time.Second)
		s.collectCodenotch(context.Background(), now, nil)
		want := 1
		if token == "code-switch" {
			want = 2
		}
		if len(s.integration.wanted) != want {
			t.Fatal("target configuration change not reflected")
		}
	}
	tools[1].ProxyInjection = nil
	if err := custom.saveStore(&customCliStore{Tools: tools}); err != nil {
		t.Fatal(err)
	}
	s.collectCodenotch(context.Background(), now.Add(time.Second), nil)
	if len(s.integration.wanted) != 0 {
		t.Fatal("tool-list change not reflected")
	}
	if err := custom.saveStore(&customCliStore{}); err != nil {
		t.Fatal(err)
	}
	s.collectCodenotch(context.Background(), now.Add(2*time.Second), nil)
	for _, p := range s.integration.platforms {
		if strings.HasPrefix(p.Platform, "custom:") {
			t.Fatal("deleted custom tool retained")
		}
	}
}

func BenchmarkCodenotchCustomHosting(b *testing.B) {
	for _, count := range []int{10, 100} {
		for _, shared := range []bool{false, true} {
			b.Run(fmt.Sprintf("%d/shared=%t", count, shared), func(b *testing.B) {
				custom, _ := codenotchCustomTools(b, count)
				b.ReportAllocs()
				b.ResetTimer()
				for n := 0; n < b.N; n++ {
					tools, err := custom.ListTools()
					if err != nil {
						b.Fatal(err)
					}
					for _, tool := range tools {
						if shared {
							if !custom.proxyStatusForTool(tool).Enabled {
								b.Fatal("hosting lost")
							}
						} else {
							status, err := custom.ProxyStatus(tool.ID)
							if err != nil || !status.Enabled {
								b.Fatal("hosting lost")
							}
						}
					}
				}
			})
		}
	}
}

func BenchmarkCodenotchDynamicCollection(b *testing.B) {
	for _, count := range []int{10, 100, 500} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			s := codenotchTestService(b, count)
			custom, tools := codenotchCustomTools(b, 10)
			s.custom = custom
			for _, tool := range tools {
				s.providers.storeProviderSnapshot("custom:"+tool.ID, []Provider{{ID: 1, Name: tool.Name, Enabled: true}}, [32]byte{1}, true)
			}
			relay := &ProviderRelayService{providerConcurrency: make(map[string]int)}
			s.concurrency = &ProviderConcurrencyService{relay: relay}
			now := time.Now()
			codenotchLease(b, s, now)
			s.collectCodenotch(context.Background(), now, nil)
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				id := int64(n%count + 1)
				provider := Provider{ID: id, Name: fmt.Sprintf("Fixture %d", id-1), Enabled: true}
				if n > 0 {
					delete(relay.providerConcurrency, providerConcurrencyStateKey("codex", fmt.Sprint((n-1)%count+1)))
				}
				relay.providerConcurrency[providerConcurrencyStateKey("codex", fmt.Sprint(id))] = n%2 + 1
				relay.trayRoutes.record("codex", hookSessionKey("codex", "benchmark"), provider, now)
				if n%10 == 0 {
					tools[0].Name = fmt.Sprintf("Edited %d", n)
					if err := custom.saveStore(&customCliStore{Tools: tools}); err != nil {
						b.Fatal(err)
					}
				}
				s.integration.next = time.Time{}
				s.collectCodenotch(context.Background(), now, nil)
			}
			b.StopTimer()
			if s.integration.info.Error || s.integration.revision != uint64(b.N+1) {
				b.Fatal("dynamic changes not published")
			}
			b.ReportMetric(float64(s.integration.revision-1)/float64(b.N), "writes/op")
		})
	}
}
