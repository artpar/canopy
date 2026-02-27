package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractFeatures(t *testing.T) {
	tests := []struct {
		prompt   string
		expected []string
	}{
		{
			"Build a notes app with SplitView and SearchField",
			[]string{"SplitView layout", "SearchField"},
		},
		{
			"Create an OutlineView for folder tree navigation",
			[]string{"OutlineView"},
		},
		{
			"Add a RichTextEditor and toolbar",
			[]string{"RichTextEditor", "Toolbar"},
		},
		{
			"Simple todo list", // no matching features
			nil,
		},
		{
			"three-pane layout with search and context menu",
			[]string{"SplitView layout", "SearchField", "Context menu"},
		},
	}

	for _, tt := range tests {
		features := ExtractFeatures(tt.prompt)
		names := make(map[string]bool)
		for _, f := range features {
			names[f.Name] = true
		}
		for _, want := range tt.expected {
			if !names[want] {
				t.Errorf("ExtractFeatures(%q): missing %q", tt.prompt, want)
			}
		}
	}
}

func TestVerifyFeaturesOnReference(t *testing.T) {
	refPath := filepath.Join("..", "sample_apps", "notes", "prompt.jsonl")
	if _, err := os.Stat(refPath); err != nil {
		t.Skipf("fixture not found: %s", refPath)
	}

	sess, err := loadSession(refPath)
	if err != nil {
		t.Fatalf("loadSession error: %v", err)
	}

	features := ExtractFeatures("Apple Notes clone with SplitView, OutlineView, SearchField, RichTextEditor")
	results := VerifyFeatures(features, sess)

	for _, r := range results {
		t.Logf("Feature %q: satisfied=%v evidence=%s", r.Feature, r.Satisfied, r.Evidence)
	}

	// The reference app should satisfy SplitView, OutlineView, RichTextEditor
	// Note: SearchField is a toolbar item, not a component — verified via JSONL scanning in scorer
	for _, r := range results {
		switch r.Feature {
		case "SplitView layout", "OutlineView", "RichTextEditor":
			if !r.Satisfied {
				t.Errorf("reference app should satisfy %q but didn't: %s", r.Feature, r.Evidence)
			}
		}
	}
}

func TestExtractFeaturesFromJSONL(t *testing.T) {
	// Test with some sample JSONL content
	content := `{"type":"updateToolbar","surfaceId":"main","items":[]}
{"type":"updateMenu","surfaceId":"main","items":[{"keyEquivalent":"\b"}]}
{"type":"updateComponents","surfaceId":"main","components":[{"props":{"contextMenu":[{"id":"x"}]}}]}`

	features := ExtractFeaturesFromJSONL(content)

	if !features["toolbar"] {
		t.Error("expected toolbar feature from updateToolbar message")
	}
	if !features["menu"] {
		t.Error("expected menu feature from updateMenu message")
	}
	if !features["context_menu"] {
		t.Error("expected context_menu feature")
	}
}
