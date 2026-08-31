/**
 * @name: 供应商快照性能基线
 * @Descripttion: 验证 Provider 深拷贝隔离并测量快照命中、指纹与稳定刷新成本
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-31 16:15:03
 * @LastEditTime: 2026-08-31 16:15:03
 * @FilePath: services/provider_snapshot_benchmark_test.go
 */

package services

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

var (
	providerBenchmarkProvidersSink   []Provider
	providerBenchmarkFingerprintSink [sha256.Size]byte
)

func buildProviderSnapshotBenchmarkProviders(count int) []Provider {
	providers := make([]Provider, count)
	for index := range providers {
		sequence := index + 1
		concurrencyLimit := sequence + 3
		modelPrefix := fmt.Sprintf("model-%d", sequence)
		providers[index] = Provider{
			ID:      int64(sequence),
			Name:    fmt.Sprintf("Provider %03d", sequence),
			APIURL:  fmt.Sprintf("https://provider-%03d.example.com", sequence),
			APIKey:  fmt.Sprintf("benchmark-key-%03d", sequence),
			Enabled: true,
			Level:   sequence%10 + 1,
			CLIConfig: map[string]interface{}{
				"env": map[string]interface{}{
					"PROFILE": modelPrefix,
					"RETRIES": float64(3),
				},
				"args": []interface{}{"--model", modelPrefix, "--stream"},
			},
			ClaudeDesktopModelRoutes: []ClaudeDesktopModelRoute{
				{Name: modelPrefix, LabelOverride: "Primary", Supports1M: true},
				{Name: modelPrefix + "-fast", LabelOverride: "Fast"},
			},
			SupportedModels: map[string]bool{
				modelPrefix:           true,
				modelPrefix + "-fast": true,
				modelPrefix + "-long": true,
			},
			ModelMapping: map[string]string{
				"claude-*": modelPrefix + "-*",
				"gpt-*":    modelPrefix + "-gpt-*",
			},
			ModelMappingDisabled: map[string]bool{
				"gpt-*": true,
			},
			ModelMappingReasoningEfforts: map[string]string{
				"claude-*": "high",
			},
			ModelMappingSupports1M: map[string]bool{
				"claude-*": true,
			},
			ModelPassthroughPatterns: []string{"vendor-*", modelPrefix + "-*"},
			RequestBodyOverrides: map[string]interface{}{
				"metadata": map[string]interface{}{
					"region": "benchmark",
					"tags":   []interface{}{"snapshot", modelPrefix},
				},
				"temperature": float64(0.2),
			},
			ProviderConcurrencyLimit: &concurrencyLimit,
			AvailabilityConfig: &AvailabilityConfig{
				TestModel:    modelPrefix,
				TestEndpoint: "/v1/models",
				Timeout:      5000,
			},
			BudgetQuotaSettings: &BudgetQuotaSettings{
				Daily: BudgetQuotaSetting{Total: 10, RefreshTime: "00:00"},
				Total: BudgetQuotaSetting{Total: 100},
			},
			BudgetQuotaUsedAdjustments: &BudgetQuotaAdjustments{
				Daily: float64(sequence),
				Total: float64(sequence * 2),
			},
			ProviderQuotaQueryConfig: &ProviderQuotaQueryConfig{
				Enabled:           true,
				TemplateType:      "new-api",
				BaseURL:           fmt.Sprintf("https://quota-%03d.example.com", sequence),
				AutoQueryInterval: 60,
			},
			configErrors: []string{"benchmark warning", modelPrefix},
		}
	}
	return providers
}

func benchmarkProviderStoredRowsFingerprint(kind string) ([sha256.Size]byte, bool, error) {
	return providerStoreFingerprint(kind)
}

func benchmarkProviderSemanticFingerprint(kind string) ([sha256.Size]byte, bool, error) {
	providers, err := LoadProvidersFromStore(kind)
	if err != nil {
		return [sha256.Size]byte{}, false, err
	}
	if providers == nil {
		return [sha256.Size]byte{}, false, nil
	}
	payload, err := json.Marshal(providers)
	if err != nil {
		return [sha256.Size]byte{}, false, err
	}
	return sha256.Sum256(payload), true, nil
}

func providerBenchmarkJSONSize(tb testing.TB, value interface{}) int {
	tb.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		tb.Fatalf("序列化 Provider 基准数据失败: %v", err)
	}
	return len(payload)
}

func TestProviderSnapshotCloneCoversAllMutableFields(t *testing.T) {
	original := buildProviderSnapshotBenchmarkProviders(1)[0]
	cloned := cloneProvider(original)
	if !reflect.DeepEqual(cloned, original) {
		t.Fatalf("克隆内容与原值不一致:\nclone=%#v\noriginal=%#v", cloned, original)
	}
	if cloned.ProviderConcurrencyLimit == original.ProviderConcurrencyLimit ||
		cloned.AvailabilityConfig == original.AvailabilityConfig ||
		cloned.BudgetQuotaSettings == original.BudgetQuotaSettings ||
		cloned.BudgetQuotaUsedAdjustments == original.BudgetQuotaUsedAdjustments ||
		cloned.ProviderQuotaQueryConfig == original.ProviderQuotaQueryConfig {
		t.Fatal("Provider 指针字段仍与原值共享地址")
	}

	cloned.CLIConfig["env"].(map[string]interface{})["PROFILE"] = "changed"
	cloned.CLIConfig["args"].([]interface{})[0] = "changed"
	cloned.ClaudeDesktopModelRoutes[0].Name = "changed"
	cloned.SupportedModels["model-1"] = false
	cloned.ModelMapping["claude-*"] = "changed"
	cloned.ModelMappingDisabled["gpt-*"] = false
	cloned.ModelMappingReasoningEfforts["claude-*"] = "low"
	cloned.ModelMappingSupports1M["claude-*"] = false
	cloned.ModelPassthroughPatterns[0] = "changed"
	cloned.RequestBodyOverrides["metadata"].(map[string]interface{})["region"] = "changed"
	cloned.RequestBodyOverrides["metadata"].(map[string]interface{})["tags"].([]interface{})[0] = "changed"
	*cloned.ProviderConcurrencyLimit = 999
	cloned.AvailabilityConfig.TestModel = "changed"
	cloned.BudgetQuotaSettings.Daily.Total = 999
	cloned.BudgetQuotaUsedAdjustments.Daily = 999
	cloned.ProviderQuotaQueryConfig.BaseURL = "https://changed.example.com"
	cloned.configErrors[0] = "changed"

	if original.CLIConfig["env"].(map[string]interface{})["PROFILE"] != "model-1" ||
		original.CLIConfig["args"].([]interface{})[0] != "--model" ||
		original.ClaudeDesktopModelRoutes[0].Name != "model-1" ||
		!original.SupportedModels["model-1"] ||
		original.ModelMapping["claude-*"] != "model-1-*" ||
		!original.ModelMappingDisabled["gpt-*"] ||
		original.ModelMappingReasoningEfforts["claude-*"] != "high" ||
		!original.ModelMappingSupports1M["claude-*"] ||
		original.ModelPassthroughPatterns[0] != "vendor-*" {
		t.Fatalf("克隆修改污染 Map 或 Slice 字段: %#v", original)
	}
	metadata := original.RequestBodyOverrides["metadata"].(map[string]interface{})
	if metadata["region"] != "benchmark" || metadata["tags"].([]interface{})[0] != "snapshot" ||
		*original.ProviderConcurrencyLimit != 4 || original.AvailabilityConfig.TestModel != "model-1" ||
		original.BudgetQuotaSettings.Daily.Total != 10 || original.BudgetQuotaUsedAdjustments.Daily != 1 ||
		original.ProviderQuotaQueryConfig.BaseURL != "https://quota-001.example.com" ||
		original.configErrors[0] != "benchmark warning" {
		t.Fatalf("克隆修改污染嵌套或指针字段: %#v", original)
	}
}

func TestProviderSnapshotStoredRowsFingerprintDetectsChanges(t *testing.T) {
	resetProviderStoreForTest(t)
	providers := buildProviderSnapshotBenchmarkProviders(1)
	if err := SaveProvidersToStore("codex", providers); err != nil {
		t.Fatal(err)
	}
	semanticBefore, semanticExists, err := benchmarkProviderSemanticFingerprint("codex")
	if err != nil || !semanticExists {
		t.Fatalf("读取当前语义指纹失败: exists=%v err=%v", semanticExists, err)
	}
	rowsBefore, rowsExist, err := benchmarkProviderStoredRowsFingerprint("codex")
	if err != nil || !rowsExist {
		t.Fatalf("读取存储行候选指纹失败: exists=%v err=%v", rowsExist, err)
	}

	providers[0].RequestBodyOverrides["metadata"].(map[string]interface{})["region"] = "changed"
	if err := SaveProvidersToStore("codex", providers); err != nil {
		t.Fatal(err)
	}
	semanticAfter, _, err := benchmarkProviderSemanticFingerprint("codex")
	if err != nil {
		t.Fatal(err)
	}
	rowsAfter, _, err := benchmarkProviderStoredRowsFingerprint("codex")
	if err != nil {
		t.Fatal(err)
	}
	if semanticBefore == semanticAfter || rowsBefore == rowsAfter {
		t.Fatal("当前语义指纹和存储行候选指纹均应检测 payload 变化")
	}
}

func TestProviderServiceSavedSnapshotUsesCurrentStoreFingerprint(t *testing.T) {
	tests := []struct {
		name      string
		providers []Provider
	}{
		{name: "non-empty", providers: buildProviderSnapshotBenchmarkProviders(2)},
		{name: "empty", providers: []Provider{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetProviderStoreForTest(t)
			service := NewProviderService()
			defer service.Stop()
			if err := service.SaveProviders("codex", tt.providers); err != nil {
				t.Fatal(err)
			}

			wantFingerprint, wantExists, err := providerStoreFingerprint("codex")
			if err != nil {
				t.Fatal(err)
			}
			service.snapshotMu.RLock()
			snapshot := service.snapshots["codex"]
			service.snapshotMu.RUnlock()
			if snapshot.exists != wantExists || snapshot.fingerprint != wantFingerprint {
				t.Fatalf("保存后快照指纹未与存储同步: snapshot=%x/%v store=%x/%v", snapshot.fingerprint, snapshot.exists, wantFingerprint, wantExists)
			}
		})
	}
}

func TestSaveProvidersToStoreWithFingerprintMatchesStoredRows(t *testing.T) {
	tests := []struct {
		name      string
		providers []Provider
	}{
		{name: "non-empty", providers: buildProviderSnapshotBenchmarkProviders(2)},
		{name: "empty", providers: []Provider{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetProviderStoreForTest(t)
			fingerprint, exists, err := saveProvidersToStoreWithFingerprint("codex", tt.providers)
			if err != nil {
				t.Fatal(err)
			}
			storedFingerprint, storedExists, err := providerStoreFingerprint("codex")
			if err != nil {
				t.Fatal(err)
			}
			if exists != storedExists || fingerprint != storedFingerprint {
				t.Fatalf("保存返回指纹与落库内容不一致: save=%x/%v store=%x/%v", fingerprint, exists, storedFingerprint, storedExists)
			}
		})
	}
}

func TestProviderStoreFingerprintTracksOrderedRawRows(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, rows []providerStoreRow)
		mutate  func(t *testing.T, rows []providerStoreRow)
	}{
		{
			name: "public column",
			mutate: func(t *testing.T, rows []providerStoreRow) {
				rows[0].Name = "column-only-change"
			},
		},
		{
			name: "payload formatting",
			mutate: func(t *testing.T, rows []providerStoreRow) {
				rows[0].Payload = " \n" + rows[0].Payload
			},
		},
		{
			name: "different corrupt payload",
			prepare: func(t *testing.T, rows []providerStoreRow) {
				rows[0].Payload = "{broken-a"
			},
			mutate: func(t *testing.T, rows []providerStoreRow) {
				rows[0].Payload = "{broken-b"
			},
		},
		{
			name: "stored order",
			mutate: func(t *testing.T, rows []providerStoreRow) {
				rows[0].SortIndex, rows[1].SortIndex = rows[1].SortIndex, rows[0].SortIndex
			},
		},
		{
			name: "row id fallback",
			prepare: func(t *testing.T, rows []providerStoreRow) {
				var provider Provider
				if err := json.Unmarshal([]byte(rows[0].Payload), &provider); err != nil {
					t.Fatal(err)
				}
				provider.ID = 0
				payload, err := json.Marshal(provider)
				if err != nil {
					t.Fatal(err)
				}
				rows[0].Payload = string(payload)
			},
			mutate: func(t *testing.T, rows []providerStoreRow) {
				rows[0].ID = "9001"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetProviderStoreForTest(t)
			if err := SaveProvidersToStore("codex", buildProviderSnapshotBenchmarkProviders(2)); err != nil {
				t.Fatal(err)
			}
			rows, err := loadProviderStoreRows("codex")
			if err != nil {
				t.Fatal(err)
			}
			if tt.prepare != nil {
				tt.prepare(t, rows)
				if err := replaceProviderStoreRows("codex", rows); err != nil {
					t.Fatal(err)
				}
			}
			before, exists, err := providerStoreFingerprint("codex")
			if err != nil || !exists {
				t.Fatalf("读取修改前指纹失败: exists=%v err=%v", exists, err)
			}
			rows, err = loadProviderStoreRows("codex")
			if err != nil {
				t.Fatal(err)
			}
			tt.mutate(t, rows)
			if err := replaceProviderStoreRows("codex", rows); err != nil {
				t.Fatal(err)
			}
			after, exists, err := providerStoreFingerprint("codex")
			if err != nil || !exists {
				t.Fatalf("读取修改后指纹失败: exists=%v err=%v", exists, err)
			}
			if before == after {
				t.Fatal("有序存储内容变化后指纹未变化")
			}
		})
	}
}

func TestProviderStoreFingerprintPreservesNilAndEmptySentinel(t *testing.T) {
	resetProviderStoreForTest(t)
	if _, exists, err := providerStoreFingerprint("codex"); err != nil || exists {
		t.Fatalf("未初始化存储指纹应不存在: exists=%v err=%v", exists, err)
	}
	if err := SaveProvidersToStore("codex", []Provider{}); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := providerStoreFingerprint("codex"); err != nil || !exists {
		t.Fatalf("空哨兵存储指纹应存在: exists=%v err=%v", exists, err)
	}
}

func TestProviderStoreFingerprintPreservesCustomSemanticFormatting(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	kind := "custom:fingerprint-format"
	providers := buildProviderSnapshotBenchmarkProviders(1)
	if err := SaveProvidersToStore(kind, providers); err != nil {
		t.Fatal(err)
	}
	before, exists, err := providerStoreFingerprint(kind)
	if err != nil || !exists {
		t.Fatalf("读取 custom 修改前指纹失败: exists=%v err=%v", exists, err)
	}

	path, err := providerConfigPath(kind, false)
	if err != nil {
		t.Fatal(err)
	}
	compact, err := json.Marshal(providerEnvelope{Providers: providers})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, compact, 0o644); err != nil {
		t.Fatal(err)
	}
	after, exists, err := providerStoreFingerprint(kind)
	if err != nil || !exists {
		t.Fatalf("读取 custom 修改后指纹失败: exists=%v err=%v", exists, err)
	}
	if before != after {
		t.Fatal("custom 文件格式变化不应改变 Provider 语义指纹")
	}
}

func TestProviderServiceRefreshesRawFingerprintWithoutChangingProviderSemantics(t *testing.T) {
	resetProviderStoreForTest(t)
	providers := buildProviderSnapshotBenchmarkProviders(1)
	service := NewProviderService()
	defer service.Stop()
	if err := service.SaveProviders("codex", providers); err != nil {
		t.Fatal(err)
	}
	service.snapshotMu.RLock()
	before := service.snapshots["codex"]
	service.snapshotMu.RUnlock()

	rows, err := loadProviderStoreRows("codex")
	if err != nil {
		t.Fatal(err)
	}
	rows[0].Name = "redundant-column-change"
	rows[0].Payload = " \n" + rows[0].Payload
	if err := replaceProviderStoreRows("codex", rows); err != nil {
		t.Fatal(err)
	}
	wantFingerprint, exists, err := providerStoreFingerprint("codex")
	if err != nil || !exists {
		t.Fatalf("读取 raw 修改后指纹失败: exists=%v err=%v", exists, err)
	}

	service.refreshProviderSnapshots()
	service.snapshotMu.RLock()
	snapshot := service.snapshots["codex"]
	service.snapshotMu.RUnlock()
	if snapshot.fingerprint != wantFingerprint || !snapshot.exists {
		t.Fatalf("raw 修改后快照指纹未同步: snapshot=%x/%v store=%x/%v", snapshot.fingerprint, snapshot.exists, wantFingerprint, exists)
	}
	if !reflect.DeepEqual(snapshot.providers, before.providers) {
		t.Fatalf("raw 非语义变化不应改变 Provider 快照: got=%#v want=%#v", snapshot.providers, before.providers)
	}
	service.refreshProviderSnapshots()
	service.snapshotMu.RLock()
	stable := service.snapshots["codex"]
	service.snapshotMu.RUnlock()
	if stable.fingerprint != snapshot.fingerprint || !reflect.DeepEqual(stable.providers, snapshot.providers) {
		t.Fatal("raw 指纹同步后的下一轮刷新应保持稳定")
	}
}

func TestSaveProvidersToStoreWithFingerprintMatchesCustomStore(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	kind := "custom:fingerprint-save"
	fingerprint, exists, err := saveProvidersToStoreWithFingerprint(kind, buildProviderSnapshotBenchmarkProviders(1))
	if err != nil {
		t.Fatal(err)
	}
	storedFingerprint, storedExists, err := providerStoreFingerprint(kind)
	if err != nil {
		t.Fatal(err)
	}
	if exists != storedExists || fingerprint != storedFingerprint {
		t.Fatalf("custom 保存返回指纹与文件语义不一致: save=%x/%v store=%x/%v", fingerprint, exists, storedFingerprint, storedExists)
	}
}

func BenchmarkProviderClone(b *testing.B) {
	for _, count := range []int{1, 10, 100} {
		providers := buildProviderSnapshotBenchmarkProviders(count)
		semanticSize := providerBenchmarkJSONSize(b, providers)
		if cloned := cloneProviders(providers); !reflect.DeepEqual(cloned, providers) {
			b.Fatal("Provider 克隆预检失败")
		}
		b.Run(fmt.Sprintf("%d_providers", count), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(semanticSize))
			b.ResetTimer()
			b.ReportMetric(float64(semanticSize), "semantic-bytes/op")
			for i := 0; i < b.N; i++ {
				providerBenchmarkProvidersSink = cloneProviders(providers)
			}
		})
	}
}

func BenchmarkProviderSnapshotLoad(b *testing.B) {
	for _, count := range []int{1, 10, 100} {
		providers := buildProviderSnapshotBenchmarkProviders(count)
		semanticSize := providerBenchmarkJSONSize(b, providers)
		service := &ProviderService{snapshots: make(map[string]providerConfigSnapshot)}
		service.storeProviderSnapshot("benchmark", providers, sha256.Sum256([]byte("benchmark")), true)
		loaded, err := service.LoadProviders("benchmark")
		if err != nil || !reflect.DeepEqual(loaded, providers) {
			b.Fatalf("Provider 快照命中预检失败: %v", err)
		}

		b.Run(fmt.Sprintf("%d_providers", count), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(semanticSize))
			b.ResetTimer()
			b.ReportMetric(float64(semanticSize), "semantic-bytes/op")
			for i := 0; i < b.N; i++ {
				loaded, err := service.LoadProviders("benchmark")
				if err != nil {
					b.Fatal(err)
				}
				providerBenchmarkProvidersSink = loaded
			}
		})
	}
}

func BenchmarkProviderSnapshotFingerprint(b *testing.B) {
	for _, count := range []int{1, 10, 100} {
		providers := buildProviderSnapshotBenchmarkProviders(count)
		if err := SaveProvidersToStore("codex", providers); err != nil {
			b.Fatal(err)
		}
		semanticSize := providerBenchmarkJSONSize(b, providers)
		rows, err := loadProviderStoreRows("codex")
		if err != nil {
			b.Fatal(err)
		}
		storedRowsSize := providerBenchmarkJSONSize(b, rows)

		b.Run(fmt.Sprintf("%d_providers/current_semantic", count), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(semanticSize))
			b.ResetTimer()
			b.ReportMetric(1, "queries/op")
			b.ReportMetric(float64(count), "rows/op")
			b.ReportMetric(float64(semanticSize), "semantic-bytes/op")
			for i := 0; i < b.N; i++ {
				fingerprint, exists, err := benchmarkProviderSemanticFingerprint("codex")
				if err != nil || !exists {
					b.Fatalf("当前语义指纹失败: exists=%v err=%v", exists, err)
				}
				providerBenchmarkFingerprintSink = fingerprint
			}
		})

		b.Run(fmt.Sprintf("%d_providers/stored_rows_candidate", count), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(storedRowsSize))
			b.ResetTimer()
			b.ReportMetric(1, "queries/op")
			b.ReportMetric(float64(count), "rows/op")
			b.ReportMetric(float64(storedRowsSize), "stored-row-bytes/op")
			for i := 0; i < b.N; i++ {
				fingerprint, exists, err := benchmarkProviderStoredRowsFingerprint("codex")
				if err != nil || !exists {
					b.Fatalf("存储行候选指纹失败: exists=%v err=%v", exists, err)
				}
				providerBenchmarkFingerprintSink = fingerprint
			}
		})
	}
}

func BenchmarkProviderSnapshotRefresh(b *testing.B) {
	for _, count := range []int{1, 10, 100} {
		b.Run(fmt.Sprintf("%d_providers", count), func(b *testing.B) {
			providers := buildProviderSnapshotBenchmarkProviders(count)
			if err := SaveProvidersToStore("codex", providers); err != nil {
				b.Fatal(err)
			}
			semanticSize := providerBenchmarkJSONSize(b, providers)
			fingerprint, exists, err := providerStoreFingerprint("codex")
			if err != nil || !exists {
				b.Fatalf("初始化稳定快照指纹失败: exists=%v err=%v", exists, err)
			}
			service := &ProviderService{snapshots: make(map[string]providerConfigSnapshot)}
			service.storeProviderSnapshot("codex", providers, fingerprint, true)

			b.ReportAllocs()
			b.SetBytes(int64(semanticSize))
			b.ResetTimer()
			b.ReportMetric(1, "queries/op")
			b.ReportMetric(float64(count), "rows/op")
			b.ReportMetric(float64(semanticSize), "semantic-bytes/op")
			for i := 0; i < b.N; i++ {
				service.refreshProviderSnapshots()
			}
			b.StopTimer()
			service.snapshotMu.RLock()
			current := service.snapshots["codex"]
			service.snapshotMu.RUnlock()
			if current.fingerprint != fingerprint || !current.exists {
				b.Fatal("稳定刷新不应替换快照")
			}
		})
	}
}
