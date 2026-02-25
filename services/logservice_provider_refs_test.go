package services

import "testing"

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

func hasRef(refs []LogProviderRef, providerID, providerName string) bool {
	for _, ref := range refs {
		if ref.ProviderID == providerID && ref.Provider == providerName {
			return true
		}
	}
	return false
}
