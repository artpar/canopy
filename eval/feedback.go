package eval

import (
	"fmt"
	"strings"
)

// GenerateFeedback produces actionable correction text from an EvalReport.
// This is deterministic (no LLM call) and designed to be appended as a user
// message in the inner loop for retry.
func GenerateFeedback(report *EvalReport) string {
	var b strings.Builder

	// Header
	passed, total := 0, len(report.TestResults)
	for _, t := range report.TestResults {
		if t.Passed {
			passed++
		}
	}
	fmt.Fprintf(&b, "POST-GENERATION EVALUATION: %d/%d tests passed, overall score %.2f/1.00\n\n", passed, total, report.Overall)

	// Failed tests
	var failedTests []TestDetail
	for _, t := range report.TestResults {
		if !t.Passed {
			failedTests = append(failedTests, t)
		}
	}
	if len(failedTests) > 0 {
		b.WriteString("FAILED TESTS:\n")
		for _, t := range failedTests {
			fmt.Fprintf(&b, "- %q: %s\n", t.Name, t.Error)
		}
		b.WriteString("\n")
	}

	// Structural issues
	if report.StructuralDiff != nil {
		if len(report.StructuralDiff.MissingComponents) > 0 {
			fmt.Fprintf(&b, "MISSING COMPONENTS (present in reference but not generated):\n")
			for _, id := range report.StructuralDiff.MissingComponents {
				fmt.Fprintf(&b, "- %s\n", id)
			}
			b.WriteString("\n")
		}
		if len(report.StructuralDiff.TypeMismatches) > 0 {
			b.WriteString("TYPE MISMATCHES:\n")
			for _, tm := range report.StructuralDiff.TypeMismatches {
				fmt.Fprintf(&b, "- %s: generated %s, should be %s\n", tm.ComponentID, tm.GenType, tm.RefType)
			}
			b.WriteString("\n")
		}
	}

	// Feature gaps
	var missingFeatures []FeatureResult
	for _, f := range report.FeatureChecks {
		if !f.Satisfied {
			missingFeatures = append(missingFeatures, f)
		}
	}
	if len(missingFeatures) > 0 {
		b.WriteString("MISSING FEATURES:\n")
		for _, f := range missingFeatures {
			fmt.Fprintf(&b, "- %s: %s\n", f.Feature, f.Evidence)
		}
		b.WriteString("\n")
	}

	// Categorized failure suggestions
	if len(report.Failures) > 0 {
		b.WriteString("SPECIFIC FIXES:\n")
		for _, f := range report.Failures {
			fmt.Fprintf(&b, "- [%s] %s", f.Category, f.Description)
			if f.Suggestion != "" {
				fmt.Fprintf(&b, " → %s", f.Suggestion)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("FIX THESE by calling the appropriate a2ui_ tools. Only fix what's broken — do not rebuild the entire app.\n")

	return b.String()
}
