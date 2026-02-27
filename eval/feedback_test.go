package eval

import (
	"strings"
	"testing"
)

func TestGenerateFeedback(t *testing.T) {
	report := &EvalReport{
		Scores: Scores{TestPassRate: 0.75},
		TestResults: []TestDetail{
			{Name: "layout test", Passed: true, Assertions: 3},
			{Name: "search test", Passed: false, Error: "uses filterContains instead of filterContainsAny"},
			{Name: "data test", Passed: true, Assertions: 2},
			{Name: "count test", Passed: false, Error: "component noteCount not found"},
		},
		StructuralDiff: &StructuralDiff{
			MissingComponents: []string{"noteCount"},
			TypeMismatches: []TypeMismatch{
				{ComponentID: "root", RefType: "SplitView", GenType: "Column"},
			},
		},
		FeatureChecks: []FeatureResult{
			{Feature: "SearchField", Satisfied: true, Evidence: "found"},
			{Feature: "Toolbar", Satisfied: false, Evidence: "not found"},
		},
		Failures: []FailureEntry{
			{Category: "wrong_function", Description: "uses filterContains", Suggestion: "use filterContainsAny"},
		},
	}
	report.ComputeOverall()

	feedback := GenerateFeedback(report)

	// Should contain key sections
	if !strings.Contains(feedback, "POST-GENERATION EVALUATION") {
		t.Error("missing header")
	}
	if !strings.Contains(feedback, "2/4 tests passed") {
		t.Error("wrong test count summary")
	}
	if !strings.Contains(feedback, "FAILED TESTS") {
		t.Error("missing failed tests section")
	}
	if !strings.Contains(feedback, "MISSING COMPONENTS") {
		t.Error("missing components section")
	}
	if !strings.Contains(feedback, "TYPE MISMATCHES") {
		t.Error("missing type mismatches section")
	}
	if !strings.Contains(feedback, "MISSING FEATURES") {
		t.Error("missing features section")
	}
	if !strings.Contains(feedback, "SPECIFIC FIXES") {
		t.Error("missing fixes section")
	}
	if !strings.Contains(feedback, "FIX THESE") {
		t.Error("missing instruction footer")
	}
}

func TestGenerateFeedbackAllPassed(t *testing.T) {
	report := &EvalReport{
		Scores: Scores{TestPassRate: 1.0},
		TestResults: []TestDetail{
			{Name: "test1", Passed: true, Assertions: 5},
		},
	}
	report.ComputeOverall()

	feedback := GenerateFeedback(report)

	if !strings.Contains(feedback, "1/1 tests passed") {
		t.Error("should show all tests passed")
	}
	// Should NOT have failed tests section
	if strings.Contains(feedback, "FAILED TESTS") {
		t.Error("should not have FAILED TESTS section when all pass")
	}
}
