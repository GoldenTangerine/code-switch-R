package services

import (
	"reflect"
	"testing"

	"github.com/daodao97/xgo/xdb"
)

func TestMergeProviderRefsFromCandidates_MergeLegacyNameToUniqueID(t *testing.T) {
	refs := mergeProviderRefsFromCandidates([]logProviderRefCandidate{
		{ProviderID: "", Provider: "Foo", LatestAt: "2026-02-01 00:00:00"},
		{ProviderID: "123", Provider: "Foo", LatestAt: "2026-02-02 00:00:00"},
	})

	if len(refs) != 1 {
		t.Fatalf("expected 1 merged ref, got %d", len(refs))
	}
	if refs[0].ProviderID != "123" {
		t.Fatalf("expected provider_id=123, got %q", refs[0].ProviderID)
	}
	if refs[0].Provider != "Foo" {
		t.Fatalf("expected provider name Foo, got %q", refs[0].Provider)
	}
}

func TestMergeProviderRefsFromCandidates_KeepLegacyNameWhenAmbiguous(t *testing.T) {
	refs := mergeProviderRefsFromCandidates([]logProviderRefCandidate{
		{ProviderID: "111", Provider: "Foo", LatestAt: "2026-02-01 00:00:00"},
		{ProviderID: "222", Provider: "Foo", LatestAt: "2026-02-02 00:00:00"},
		{ProviderID: "", Provider: "Foo", LatestAt: "2026-02-03 00:00:00"},
	})

	if len(refs) != 3 {
		t.Fatalf("expected 3 refs when name is ambiguous, got %d", len(refs))
	}

	if !hasRef(refs, "", "Foo") {
		t.Fatalf("expected legacy name-only ref to remain when ambiguous")
	}
	if !hasRef(refs, "111", "Foo") {
		t.Fatalf("expected id ref 111 to remain")
	}
	if !hasRef(refs, "222", "Foo") {
		t.Fatalf("expected id ref 222 to remain")
	}
}

func TestMergeProviderRefsFromCandidates_UseLatestNamePerRef(t *testing.T) {
	refs := mergeProviderRefsFromCandidates([]logProviderRefCandidate{
		{ProviderID: "123", Provider: "Old Name", LatestAt: "2026-02-01 00:00:00"},
		{ProviderID: "123", Provider: "New Name", LatestAt: "2026-02-03 00:00:00"},
	})

	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].Provider != "New Name" {
		t.Fatalf("expected latest provider name, got %q", refs[0].Provider)
	}
}

func TestListProviderRefsV2FiltersSourcesAndMergesLegacyRows(t *testing.T) {
	useIsolatedHomeDir(t)
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM request_log`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM request_log`)
	})

	rows := []struct {
		platform   string
		providerID string
		provider   string
		source     string
		createdAt  string
	}{
		{platform: "codex", providerID: "id-1", provider: "Old Name", source: requestLogDataSourceProxy, createdAt: "2026-09-01 00:00:01"},
		{platform: "codex", providerID: "id-1", provider: "New Name", source: "", createdAt: "2026-09-01 00:00:02"},
		{platform: "codex", provider: "New Name", source: requestLogDataSourceProxy, createdAt: "2026-09-01 00:00:03"},
		{platform: "codex", providerID: "id-2", provider: "Shared", source: requestLogDataSourceClaudeSession, createdAt: "2026-09-01 00:00:04"},
		{platform: "codex", providerID: "id-3", provider: "Shared", source: requestLogDataSourceCodexSession, createdAt: "2026-09-01 00:00:05"},
		{platform: "codex", provider: "Shared", source: requestLogDataSourceGeminiSession, createdAt: "2026-09-01 00:00:06"},
		{platform: "claude", providerID: "id-4", provider: "Other", source: requestLogDataSourceClaudeSession, createdAt: "2026-09-01 00:00:07"},
		{platform: "codex", providerID: "ignored", provider: "   ", source: requestLogDataSourceProxy, createdAt: "2026-09-01 00:00:08"},
	}
	for _, row := range rows {
		if _, err := db.Exec(`
			INSERT INTO request_log (platform, provider_id, provider, data_source, created_at)
			VALUES (?, ?, ?, ?, ?)
		`, row.platform, row.providerID, row.provider, row.source, row.createdAt); err != nil {
			t.Fatal(err)
		}
	}

	testCases := []struct {
		name       string
		platform   string
		sourceMode LogDataSourceMode
		want       []LogProviderRef
	}{
		{
			name:       "ProxyCodex",
			platform:   "codex",
			sourceMode: LogDataSourceModeProxy,
			want:       []LogProviderRef{{ProviderID: "id-1", Provider: "New Name"}},
		},
		{
			name:       "SessionCodex",
			platform:   "codex",
			sourceMode: LogDataSourceModeSession,
			want: []LogProviderRef{
				{Provider: "Shared"},
				{ProviderID: "id-2", Provider: "Shared"},
				{ProviderID: "id-3", Provider: "Shared"},
			},
		},
		{
			name:       "AllPlatforms",
			sourceMode: LogDataSourceModeAll,
			want: []LogProviderRef{
				{ProviderID: "id-1", Provider: "New Name"},
				{ProviderID: "id-4", Provider: "Other"},
				{Provider: "Shared"},
				{ProviderID: "id-2", Provider: "Shared"},
				{ProviderID: "id-3", Provider: "Shared"},
			},
		},
	}

	service := NewLogService(nil)
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			refs, err := service.ListProviderRefsV2(testCase.platform, string(testCase.sourceMode))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(refs, testCase.want) {
				t.Fatalf("ProviderRefs 不等价:\ngot=%#v\nwant=%#v", refs, testCase.want)
			}
		})
	}
}

func hasRef(refs []LogProviderRef, providerID, providerName string) bool {
	for _, ref := range refs {
		if ref.ProviderID == providerID && ref.Provider == providerName {
			return true
		}
	}
	return false
}
