/**
 * @name: 托盘共享快照测试
 * @Descripttion: 验证实时发布、额度口径、只读协议与退出清理。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-09-08 17:05:00
 * @LastEditTime: 2026-09-08 17:05:00
 * @FilePath: services/traysnapshotservice_test.go
 */
package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/daodao97/xgo/xdb"
)

func TestTraySnapshotActivitiesMatchTraySelection(t *testing.T) {
	state := TrayProviderRuntimeState{DefaultProvider: &TrayDefaultProvider{ProviderID: "default", ProviderName: "Default"},
		Statuses: []TrayProviderActivityStatus{{ProviderID: "one", ActiveRequests: 2}, {ProviderID: "two", ActiveRequests: 1}, {ProviderID: "idle"}}}
	active := trayActivities(state)
	if len(active) != 2 || active[0].ProviderID != "one" || active[1].ProviderID != "two" || active[0].Status != "active" {
		t.Fatalf("unexpected active selection: %+v", active)
	}
	state.Statuses = nil
	if got := trayActivities(state); len(got) != 1 || got[0].ProviderID != "default" || got[0].Status != "default" {
		t.Fatalf("unexpected fallback: %+v", got)
	}
	state.Error = true
	if len(trayActivities(state)) != 0 {
		t.Fatal("runtime error must not retain providers")
	}
}

func TestTraySnapshotIdentityNeverFallsBackFromID(t *testing.T) {
	inputs := []trayProviderInput{
		{ref: "a", provider: Provider{Name: "Same", Enabled: false}},
		{ref: "b", provider: Provider{Name: "Same", Enabled: true}},
	}
	for _, id := range []string{"a", "b"} {
		got := matchTrayProvider(inputs, TraySnapshotProvider{ProviderID: id, ProviderName: "Same"})
		if got == nil || got.ref != id {
			t.Fatalf("identity mismatch for %s", id)
		}
	}
	for _, id := range []string{"missing", ""} {
		if matchTrayProvider(inputs, TraySnapshotProvider{ProviderID: id, ProviderName: "Same"}) != nil {
			t.Fatal("missing ID or ambiguous name must not borrow another provider")
		}
	}
	if matchTrayProvider(inputs[:1], TraySnapshotProvider{ProviderName: "Same"}) == nil {
		t.Fatal("unique legacy name should resolve")
	}
}

func TestTraySnapshotInputCacheTracksConfigurationChanges(t *testing.T) {
	providers := &ProviderService{snapshots: make(map[string]providerConfigSnapshot)}
	providers.storeProviderSnapshot("codex", []Provider{{ID: 1, Name: "Same", Enabled: false}, {ID: 2, Name: "Same", Enabled: true}}, [32]byte{1}, true)
	s := NewTraySnapshotService(providers, nil, nil, nil, nil, nil, nil)
	first, err := s.inputs("codex")
	if err != nil || len(first) != 2 || first[0].provider.Enabled {
		t.Fatal("disabled provider metadata lost")
	}
	second, err := s.inputs("codex")
	if err != nil || &first[0] != &second[0] {
		t.Fatal("unchanged configuration was cloned again")
	}
	providers.storeProviderSnapshot("codex", []Provider{{ID: 2, Name: "Updated", Enabled: true}}, [32]byte{2}, true)
	third, err := s.inputs("codex")
	if err != nil || len(third) != 1 || third[0].provider.Name != "Updated" {
		t.Fatal("configuration update was not observed")
	}
}

func TestTraySnapshotSharesPlatformStatsAcrossWorkers(t *testing.T) {
	useIsolatedHomeDir(t)
	if err := InitDatabase(); err != nil {
		t.Fatal(err)
	}
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatal(err)
	}
	insertRequestLogForProviderQuotaTest(t, db, providerQuotaLogEntry{Platform: "codex", ProviderID: "one", Provider: "One", CreatedAt: time.Now().UTC().Format(timeLayout), TotalCost: 1})
	s := NewTraySnapshotService(nil, nil, nil, nil, nil, nil, NewLogService(nil))
	results := make([][]ProviderDailyStat, 8)
	var workers sync.WaitGroup
	for i := range results {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			var err error
			results[i], err = s.dailyStats(context.Background(), "codex")
			if err != nil {
				t.Error(err)
			}
		}(i)
	}
	workers.Wait()
	for _, stats := range results {
		if len(stats) == 0 || len(results[0]) == 0 || &stats[0] != &results[0][0] {
			t.Fatal("concurrent suppliers did not share immutable platform statistics")
		}
	}
	cached := s.statsCache["codex"]
	cached.updated = time.Now().Add(-2 * time.Minute)
	next, err := s.dailyStats(context.Background(), "codex")
	if err != nil || len(next) == 0 || &next[0] == &results[0][0] {
		t.Fatal("expired statistics were not refreshed")
	}
}

func TestTrayQuotaCancelsJSONResponseBody(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	query := NewProviderQuotaQueryService()
	query.ctx = ctx
	done := make(chan error, 1)
	go func() { _, _, err := query.sendJSONRequest(http.MethodGet, server.URL, nil, nil); done <- err }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request never started")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("response body should be cancelled")
		}
	case <-time.After(time.Second):
		t.Fatal("response body ignored cancellation")
	}
}

func TestTraySnapshotRecreatesRemovedCacheDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	path := filepath.Join(dir, "snapshot.json")
	for i := 0; i < 2; i++ {
		if err := writeTraySnapshot(path, TraySnapshot{Version: 1}); err != nil {
			t.Fatal(err)
		}
		for name, mode := range map[string]os.FileMode{dir: 0700, path: 0600} {
			info, err := os.Stat(name)
			if err != nil || info.Mode().Perm() != mode {
				t.Fatalf("incorrect permissions for %s", name)
			}
		}
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTraySnapshotStopCancelsSlowQuotaRequest(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	s := NewTraySnapshotService(nil, nil, nil, nil, nil, NewProviderQuotaQueryService(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.cancel, s.done = cancel, make(chan struct{})
	go s.run(ctx, func(context.Context, time.Time) []TraySnapshotPlatform { return nil })
	input := trayProviderInput{ref: "a", provider: Provider{ProviderQuotaQueryType: "custom", ProviderQuotaQueryConfig: &ProviderQuotaQueryConfig{
		Enabled: true, TemplateType: "custom", Timeout: 30,
		Code: `({request:{url:"` + server.URL + `"},extractor:()=>[]})`,
	}}}
	s.workers <- struct{}{}
	s.wg.Add(1)
	go s.loadDetails(ctx, "codex", input, "a", [32]byte{})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request never started")
	}
	stopped := make(chan struct{})
	go func() { s.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("stop did not cancel HTTP worker")
	}
}

func TestTrayQuotaCancellationInterruptsScript(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	query := NewProviderQuotaQueryService()
	query.ctx = ctx
	done := make(chan error, 1)
	go func() {
		_, err := query.executeScriptQuotaQuery(`while (true) {}`, "", "", "", "", "custom", 30)
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("script should be interrupted")
		}
	case <-time.After(time.Second):
		t.Fatal("script ignored cancellation")
	}
}

func TestTraySnapshotPublishesWithoutWindowsAndRemovesOnStop(t *testing.T) {
	s := NewTraySnapshotService(nil, nil, nil, nil, nil, nil, nil)
	s.path = filepath.Join(t.TempDir(), "tray-snapshot-v1.json")
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel, s.done = cancel, make(chan struct{})
	var active atomic.Bool
	go s.run(ctx, func(context.Context, time.Time) []TraySnapshotPlatform {
		id := "default"
		if active.Load() {
			id = "active"
		}
		return []TraySnapshotPlatform{{Platform: "codex", Providers: []TraySnapshotProvider{{ProviderID: id, Quotas: []TraySnapshotQuota{}}}}}
	})
	t.Cleanup(s.Stop)
	waitFor := func(id string, after uint64) TraySnapshot {
		t.Helper()
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			data, err := os.ReadFile(s.path)
			var snapshot TraySnapshot
			if err == nil && json.Unmarshal(data, &snapshot) == nil && snapshot.Sequence > after && snapshot.Platforms[0].Providers[0].ProviderID == id {
				return snapshot
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("snapshot did not update within one second")
		return TraySnapshot{}
	}
	first := waitFor("default", 0)
	active.Store(true)
	next := waitFor("active", first.Sequence)
	if next.Session != first.Session || next.HeartbeatAt < first.HeartbeatAt {
		t.Fatal("session or heartbeat regressed")
	}
	info, err := os.Stat(s.path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("unexpected snapshot permissions: %v %v", info, err)
	}
	s.Stop()
	if _, err := os.Stat(s.path); !os.IsNotExist(err) {
		t.Fatal("normal exit must remove snapshot")
	}
}

func TestTraySnapshotBudgetUsesProviderIdentityAndCalibration(t *testing.T) {
	useIsolatedHomeDir(t)
	if err := InitDatabase(); err != nil {
		t.Fatal(err)
	}
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatal(err)
	}
	insertRequestLogForProviderQuotaTest(t, db, providerQuotaLogEntry{Platform: "codex", ProviderID: "one", Provider: "Same", CreatedAt: "2026-09-08 02:00:00", TotalCost: 1.235})
	insertRequestLogForProviderQuotaTest(t, db, providerQuotaLogEntry{Platform: "codex", ProviderID: "two", Provider: "Same", CreatedAt: "2026-09-08 02:00:00", TotalCost: 99})
	s := NewTraySnapshotService(nil, nil, nil, nil, nil, nil, NewLogService(nil))
	input := trayProviderInput{ref: "one", provider: Provider{Name: "Same", APIKey: "PRIVATE_TEST_KEY",
		BudgetQuotaSettings:        &BudgetQuotaSettings{Daily: BudgetQuotaSetting{Total: 10, RefreshTime: "00:00"}},
		BudgetQuotaUsedAdjustments: &BudgetQuotaAdjustments{Daily: 0.111111}}}
	quotas := s.loadBudgetQuotas("codex", input, time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC))
	if len(quotas) != 1 || quotas[0].Used != 1.35 || quotas[0].Total != 10 || quotas[0].DisplayKind != "progress" {
		t.Fatalf("incorrect budget: %+v", quotas)
	}
	data, err := json.Marshal(TraySnapshotProvider{ProviderID: input.ref, ProviderName: input.provider.Name, Quotas: quotas})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"PRIVATE_TEST_KEY", "apiKey", "apiUrl", "queryConfig"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("private field exported: %s", forbidden)
		}
	}
}

func TestTraySnapshotBudgetWindowsAndRounding(t *testing.T) {
	now := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	start, next := trayBudgetWindow("monthly", BudgetQuotaSetting{RefreshTime: "09:30", RefreshMonthDay: 31}, now)
	if start.Day() != 28 || start.Month() != time.February || next.Day() != 31 || next.Month() != time.March || next.Hour() != 9 {
		t.Fatalf("month-end window %s -> %s", start, next)
	}
	for _, test := range []struct{ used, adjustment, want float64 }{{1.005, 0, 1.01}, {1.235, .111111, 1.35}, {1, -2, 0}, {0, 0, 0}} {
		if got := trayBudgetUsed(test.used, test.adjustment); got != test.want {
			t.Errorf("used(%v, %v) = %v; want %v", test.used, test.adjustment, got, test.want)
		}
	}
}
