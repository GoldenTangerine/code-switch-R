package services

import "testing"

func TestDiffProviderCandidates_AllowsSameNameWithDifferentIdentity(t *testing.T) {
	existing := []Provider{
		{
			ID:     1,
			Name:   "Shared Name",
			APIURL: "https://existing.example.com",
			APIKey: "key-existing",
		},
	}

	entries := map[string]ccProviderEntry{
		"candidate": {
			Name: "Shared Name",
			Settings: ccProviderSetting{
				Env: stringMap{
					"ANTHROPIC_BASE_URL":   "https://new.example.com",
					"ANTHROPIC_AUTH_TOKEN": "key-new",
				},
			},
		},
	}

	candidates := diffProviderCandidates("claude", entries, existing)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].Name != "Shared Name" {
		t.Fatalf("expected same name to be kept, got %q", candidates[0].Name)
	}
}

func TestDiffProviderCandidates_DedupByURLAndKey(t *testing.T) {
	existing := []Provider{
		{
			ID:     1,
			Name:   "Existing",
			APIURL: "https://same.example.com/",
			APIKey: "same-key",
		},
	}

	entries := map[string]ccProviderEntry{
		"same_identity": {
			Name: "Any Name",
			Settings: ccProviderSetting{
				Env: stringMap{
					"ANTHROPIC_BASE_URL":   "https://same.example.com",
					"ANTHROPIC_AUTH_TOKEN": "same-key",
				},
			},
		},
		"same_url_diff_key": {
			Name: "Any Name 2",
			Settings: ccProviderSetting{
				Env: stringMap{
					"ANTHROPIC_BASE_URL":   "https://same.example.com",
					"ANTHROPIC_AUTH_TOKEN": "another-key",
				},
			},
		},
	}

	candidates := diffProviderCandidates("claude", entries, existing)
	if len(candidates) != 1 {
		t.Fatalf("expected only 1 new candidate, got %d", len(candidates))
	}
	if candidates[0].APIKey != "another-key" {
		t.Fatalf("expected candidate with different key to remain, got key=%q", candidates[0].APIKey)
	}
}

func TestDiffProviderCandidates_DedupWithinImportBatchByURLAndKey(t *testing.T) {
	entries := map[string]ccProviderEntry{
		"candidate_a": {
			Name: "A",
			Settings: ccProviderSetting{
				Env: stringMap{
					"ANTHROPIC_BASE_URL":   "https://dup.example.com",
					"ANTHROPIC_AUTH_TOKEN": "dup-key",
				},
			},
		},
		"candidate_b": {
			Name: "B",
			Settings: ccProviderSetting{
				Env: stringMap{
					"ANTHROPIC_BASE_URL":   "https://dup.example.com/",
					"ANTHROPIC_AUTH_TOKEN": "dup-key",
				},
			},
		},
	}

	candidates := diffProviderCandidates("claude", entries, nil)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 deduplicated candidate, got %d", len(candidates))
	}
}
