package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompareStructureSameSurface(t *testing.T) {
	refPath := filepath.Join("..", "sample_apps", "notes", "prompt.jsonl")
	if _, err := os.Stat(refPath); err != nil {
		t.Skipf("fixture not found: %s", refPath)
	}

	sess, err := loadSession(refPath)
	if err != nil {
		t.Fatalf("loadSession error: %v", err)
	}

	surf := firstSurface(sess)
	if surf == nil {
		t.Fatal("no surface in session")
	}

	// Comparing a surface to itself should yield perfect similarity
	diff := CompareStructure(surf, surf)
	if diff == nil {
		t.Fatal("expected non-nil diff")
	}
	if diff.Similarity != 1.0 {
		t.Errorf("self-comparison Similarity = %.2f, want 1.0", diff.Similarity)
	}
	if len(diff.MissingComponents) > 0 {
		t.Errorf("self-comparison has %d missing components", len(diff.MissingComponents))
	}
	if len(diff.ExtraComponents) > 0 {
		t.Errorf("self-comparison has %d extra components", len(diff.ExtraComponents))
	}
	if len(diff.TypeMismatches) > 0 {
		t.Errorf("self-comparison has %d type mismatches", len(diff.TypeMismatches))
	}
}

func TestCompareStructureRefVsGen(t *testing.T) {
	refPath := filepath.Join("..", "sample_apps", "notes", "prompt.jsonl")
	genPath := filepath.Join("..", "sample_apps", "notes_llm", "prompt.jsonl")
	if _, err := os.Stat(refPath); err != nil {
		t.Skipf("reference fixture not found: %s", refPath)
	}
	if _, err := os.Stat(genPath); err != nil {
		t.Skipf("generated fixture not found: %s", genPath)
	}

	refSess, err := loadSession(refPath)
	if err != nil {
		t.Fatalf("load ref: %v", err)
	}
	genSess, err := loadSession(genPath)
	if err != nil {
		t.Fatalf("load gen: %v", err)
	}

	refSurf := firstSurface(refSess)
	genSurf := firstSurface(genSess)
	if refSurf == nil || genSurf == nil {
		t.Fatal("missing surface")
	}

	diff := CompareStructure(refSurf, genSurf)
	if diff == nil {
		t.Fatal("expected non-nil diff")
	}

	// Should have some similarity (both are notes apps)
	if diff.Similarity <= 0 {
		t.Errorf("Similarity = %.2f, want > 0", diff.Similarity)
	}

	t.Logf("Structural diff: similarity=%.2f, missing=%d, extra=%d, typeMismatches=%d",
		diff.Similarity, len(diff.MissingComponents), len(diff.ExtraComponents), len(diff.TypeMismatches))
}

func TestCompareStructureNilInputs(t *testing.T) {
	if diff := CompareStructure(nil, nil); diff != nil {
		t.Error("expected nil diff for nil inputs")
	}
}

func TestCompareDataModelSameSession(t *testing.T) {
	refPath := filepath.Join("..", "sample_apps", "notes", "prompt.jsonl")
	if _, err := os.Stat(refPath); err != nil {
		t.Skipf("fixture not found: %s", refPath)
	}

	sess, err := loadSession(refPath)
	if err != nil {
		t.Fatalf("loadSession error: %v", err)
	}

	surf := firstSurface(sess)
	if surf == nil {
		t.Fatal("no surface")
	}

	paths := []string{"/selectedFolderId", "/selectedNoteId", "/searchQuery"}
	score := CompareDataModel(surf, surf, paths)
	if score != 1.0 {
		t.Errorf("self-comparison DataModel score = %.2f, want 1.0", score)
	}
}

func TestValuesMatch(t *testing.T) {
	tests := []struct {
		a, b interface{}
		want bool
	}{
		{"hello", "hello", true},
		{"hello", "world", false},
		{42.0, 42.0, true},
		{42.0, 43.0, false},
		{true, true, true},
		{true, false, false},
		{[]interface{}{1, 2}, []interface{}{3, 4}, true},  // same length
		{[]interface{}{1, 2}, []interface{}{3}, false},     // different length
		{map[string]interface{}{"a": 1}, map[string]interface{}{"a": 2}, true},  // same keys
		{map[string]interface{}{"a": 1}, map[string]interface{}{"b": 2}, false}, // different keys
	}

	for _, tt := range tests {
		got := valuesMatch(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("valuesMatch(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
