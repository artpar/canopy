package registry

import (
	"sort"
	"testing"
)

func TestValidateManifest(t *testing.T) {
	valid := &Manifest{
		Name:    "test",
		Version: "1.0.0",
		Type:    TypeApp,
		Entry:   "prompt.jsonl",
	}
	if err := validateManifest(valid); err != nil {
		t.Errorf("expected valid, got %v", err)
	}

	tests := []struct {
		name string
		m    Manifest
	}{
		{"missing name", Manifest{Version: "1.0.0", Type: TypeApp, Entry: "e"}},
		{"missing version", Manifest{Name: "n", Type: TypeApp, Entry: "e"}},
		{"invalid version", Manifest{Name: "n", Version: "bad", Type: TypeApp, Entry: "e"}},
		{"missing type", Manifest{Name: "n", Version: "1.0.0", Entry: "e"}},
		{"unknown type", Manifest{Name: "n", Version: "1.0.0", Type: "bad", Entry: "e"}},
		{"missing entry", Manifest{Name: "n", Version: "1.0.0", Type: TypeApp}},
	}
	for _, tt := range tests {
		if err := validateManifest(&tt.m); err == nil {
			t.Errorf("case %q: expected error", tt.name)
		}
	}
}

func TestValidateManifestAllTypes(t *testing.T) {
	for _, typ := range []PackageType{TypeApp, TypeComponent, TypeTheme, TypeFFIConfig} {
		m := &Manifest{Name: "n", Version: "1.0.0", Type: typ, Entry: "e"}
		if err := validateManifest(m); err != nil {
			t.Errorf("type %s: %v", typ, err)
		}
	}
}

func TestEnsureTopics(t *testing.T) {
	existing := []string{"go", "macos"}
	result := ensureTopics(existing, TypeApp)
	sort.Strings(result)

	expected := []string{"canopy-app", "canopy-package", "go", "macos"}
	if len(result) != len(expected) {
		t.Fatalf("got %v, want %v", result, expected)
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("result[%d] = %q, want %q", i, result[i], v)
		}
	}
}

func TestEnsureTopicsDedup(t *testing.T) {
	existing := []string{"canopy-package", "canopy-app"}
	result := ensureTopics(existing, TypeApp)
	if len(result) != 2 {
		t.Errorf("expected 2, got %d: %v", len(result), result)
	}
}

func TestEnsureTopicsEmpty(t *testing.T) {
	result := ensureTopics(nil, TypeComponent)
	sort.Strings(result)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d: %v", len(result), result)
	}
	if result[0] != "canopy-component" || result[1] != "canopy-package" {
		t.Errorf("got %v", result)
	}
}
