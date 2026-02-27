package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScorerBasicEval(t *testing.T) {
	// Use the hand-crafted notes fixture (has inline tests)
	refPath := filepath.Join("..", "sample_apps", "notes", "prompt.jsonl")
	if _, err := os.Stat(refPath); err != nil {
		t.Skipf("reference fixture not found: %s", refPath)
	}

	scorer := NewScorer(refPath, "", "")
	report, err := scorer.Evaluate()
	if err != nil {
		t.Fatalf("Evaluate() error: %v", err)
	}

	// Reference should pass all its own tests
	if report.Scores.TestPassRate < 1.0 {
		t.Errorf("reference TestPassRate = %.2f, want 1.0", report.Scores.TestPassRate)
		for _, tr := range report.TestResults {
			if !tr.Passed {
				t.Logf("  FAIL: %s — %s", tr.Name, tr.Error)
			}
		}
	}

	// No reference = structural similarity should be 1.0
	if report.Scores.StructuralSimilarity != 1.0 {
		t.Errorf("no-ref StructuralSimilarity = %.2f, want 1.0", report.Scores.StructuralSimilarity)
	}

	// Overall should be reasonable
	if report.Overall < 0.5 {
		t.Errorf("Overall = %.2f, want >= 0.5", report.Overall)
	}
}

func TestScorerWithReference(t *testing.T) {
	refPath := filepath.Join("..", "sample_apps", "notes", "prompt.jsonl")
	genPath := filepath.Join("..", "sample_apps", "notes_llm", "prompt.jsonl")
	if _, err := os.Stat(refPath); err != nil {
		t.Skipf("reference fixture not found: %s", refPath)
	}
	if _, err := os.Stat(genPath); err != nil {
		t.Skipf("generated fixture not found: %s", genPath)
	}

	scorer := NewScorer(genPath, refPath, "Apple Notes clone with SplitView, OutlineView, SearchField, RichTextEditor, toolbar, context menu")
	report, err := scorer.Evaluate()
	if err != nil {
		t.Fatalf("Evaluate() error: %v", err)
	}

	// Structural similarity should be between 0 and 1
	if report.Scores.StructuralSimilarity < 0 || report.Scores.StructuralSimilarity > 1 {
		t.Errorf("StructuralSimilarity = %.2f, want [0, 1]", report.Scores.StructuralSimilarity)
	}

	// Feature completeness should be > 0 (we have matching keywords)
	if report.Scores.FeatureCompleteness <= 0 {
		t.Errorf("FeatureCompleteness = %.2f, want > 0", report.Scores.FeatureCompleteness)
	}

	// Should have some test results
	if len(report.TestResults) == 0 {
		t.Error("expected test results from LLM-generated JSONL")
	}

	// Structural diff should exist since we have a reference
	if report.StructuralDiff == nil {
		t.Error("expected StructuralDiff when reference is provided")
	}

	t.Logf("Scores: tests=%.2f structural=%.2f features=%.2f dm=%.2f coverage=%.2f overall=%.2f",
		report.Scores.TestPassRate,
		report.Scores.StructuralSimilarity,
		report.Scores.FeatureCompleteness,
		report.Scores.DataModelCorrectness,
		report.Scores.ComponentCoverage,
		report.Overall)
	t.Logf("Tests: %d total, %d failures", len(report.TestResults), len(report.Failures))
}

func TestScorerNoTests(t *testing.T) {
	// hello.jsonl has no inline tests — should get TestPassRate = 1.0
	helloPath := filepath.Join("..", "testdata", "hello.jsonl")
	if _, err := os.Stat(helloPath); err != nil {
		t.Skipf("hello fixture not found: %s", helloPath)
	}

	scorer := NewScorer(helloPath, "", "")
	report, err := scorer.Evaluate()
	if err != nil {
		t.Fatalf("Evaluate() error: %v", err)
	}

	if report.Scores.TestPassRate != 1.0 {
		t.Errorf("TestPassRate for no-test file = %.2f, want 1.0", report.Scores.TestPassRate)
	}
}

func TestWriteAndFormatReport(t *testing.T) {
	report := &EvalReport{
		PromptFile: "test.jsonl",
		Attempt:    1,
		Scores: Scores{
			TestPassRate:         0.75,
			StructuralSimilarity: 0.80,
			FeatureCompleteness:  0.60,
			DataModelCorrectness: 1.0,
			ComponentCoverage:    0.50,
		},
		TestResults: []TestDetail{
			{Name: "test1", Passed: true, Assertions: 3},
			{Name: "test2", Passed: false, Error: "component not found", Assertions: 1},
		},
		Failures: []FailureEntry{
			{Category: "missing_component", Description: "component X not found"},
		},
	}
	report.ComputeOverall()

	// Test FormatReport
	text := FormatReport(report)
	if text == "" {
		t.Error("FormatReport returned empty string")
	}
	if !containsStr(text, "test.jsonl") {
		t.Error("format should contain prompt file")
	}
	if !containsStr(text, "FAIL") {
		t.Error("format should contain FAIL for failed test")
	}

	// Test WriteReport
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "report.json")
	if err := WriteReport(report, path); err != nil {
		t.Fatalf("WriteReport error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if len(data) == 0 {
		t.Error("written report is empty")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findStr(s, substr))
}

func findStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
