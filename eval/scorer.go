package eval

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"jview/engine"
	"jview/protocol"
	"jview/renderer"
)

// Scorer evaluates a generated JSONL app against optional reference and prompt text.
type Scorer struct {
	generatedPath string
	refPath       string
	promptText    string

	genSess *engine.Session
	refSess *engine.Session
}

// NewScorer creates a scorer. refPath and promptText may be empty.
func NewScorer(generatedPath, refPath, promptText string) *Scorer {
	return &Scorer{
		generatedPath: generatedPath,
		refPath:       refPath,
		promptText:    promptText,
	}
}

// Evaluate runs all scoring dimensions and returns a complete report.
func (s *Scorer) Evaluate() (*EvalReport, error) {
	report := &EvalReport{
		PromptFile: s.generatedPath,
		RefFile:    s.refPath,
		Attempt:    1,
		Timestamp:  time.Now(),
	}

	// Load generated JSONL into a session
	var err error
	s.genSess, err = loadSession(s.generatedPath)
	if err != nil {
		return nil, fmt.Errorf("load generated JSONL: %w", err)
	}

	// Load reference JSONL if provided
	if s.refPath != "" {
		s.refSess, err = loadSession(s.refPath)
		if err != nil {
			return nil, fmt.Errorf("load reference JSONL: %w", err)
		}
	}

	// Run all scoring dimensions
	s.scoreTests(report)
	s.scoreStructural(report)
	s.scoreDataModel(report)
	s.scoreFeatures(report)
	s.scoreCoverage(report)
	s.analyzeFailures(report)
	report.ComputeOverall()

	return report, nil
}

// scoreTests loads the JSONL, runs inline tests via MockRenderer+MockDispatcher.
func (s *Scorer) scoreTests(report *EvalReport) {
	f, err := os.Open(s.generatedPath)
	if err != nil {
		return
	}
	defer f.Close()

	rend := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	results, err := engine.RunTests(f, rend, disp)
	if err != nil {
		report.Failures = append(report.Failures, FailureEntry{
			Category:    "test_logic",
			Description: fmt.Sprintf("failed to run tests: %v", err),
		})
		return
	}

	passed := 0
	for _, r := range results {
		report.TestResults = append(report.TestResults, TestDetail{
			Name:       r.Name,
			Passed:     r.Passed,
			Error:      r.Error,
			Assertions: r.Assertions,
			Skipped:    r.Skipped,
		})
		if r.Passed {
			passed++
		}
	}

	if len(results) > 0 {
		report.Scores.TestPassRate = float64(passed) / float64(len(results))
	} else {
		report.Scores.TestPassRate = 1.0 // no tests = no penalty
	}
}

// scoreStructural compares component trees if reference is available.
func (s *Scorer) scoreStructural(report *EvalReport) {
	if s.refSess == nil {
		report.Scores.StructuralSimilarity = 1.0 // no reference = no penalty
		return
	}

	refSurf := firstSurface(s.refSess)
	genSurf := firstSurface(s.genSess)
	if refSurf == nil || genSurf == nil {
		report.Scores.StructuralSimilarity = 0
		return
	}

	diff := CompareStructure(refSurf, genSurf)
	report.StructuralDiff = diff
	if diff != nil {
		report.Scores.StructuralSimilarity = diff.Similarity
	}
}

// scoreDataModel compares key data model paths between ref and gen.
func (s *Scorer) scoreDataModel(report *EvalReport) {
	if s.refSess == nil {
		report.Scores.DataModelCorrectness = 1.0
		return
	}

	refSurf := firstSurface(s.refSess)
	genSurf := firstSurface(s.genSess)
	if refSurf == nil || genSurf == nil {
		report.Scores.DataModelCorrectness = 0
		return
	}

	// Extract key data model paths from the reference
	paths := extractDataModelPaths(refSurf)
	report.Scores.DataModelCorrectness = CompareDataModel(refSurf, genSurf, paths)
}

// scoreFeatures checks prompt-derived features against the generated session.
func (s *Scorer) scoreFeatures(report *EvalReport) {
	if s.promptText == "" {
		report.Scores.FeatureCompleteness = 1.0
		return
	}

	features := ExtractFeatures(s.promptText)
	if len(features) == 0 {
		report.Scores.FeatureCompleteness = 1.0
		return
	}

	// Supplement with JSONL-level checks
	jsonlContent, _ := os.ReadFile(s.generatedPath)
	jsonlFeatures := ExtractFeaturesFromJSONL(string(jsonlContent))

	results := VerifyFeatures(features, s.genSess)

	// Override session-only checks with JSONL evidence
	for i, r := range results {
		switch r.Feature {
		case "Toolbar":
			if jsonlFeatures["toolbar"] {
				results[i].Satisfied = true
				results[i].Evidence = "updateToolbar message found in JSONL"
			}
		case "Delete shortcut":
			if jsonlFeatures["backspace_key"] {
				results[i].Satisfied = true
				results[i].Evidence = "backspace keyEquivalent found in JSONL"
			}
		}
	}

	report.FeatureChecks = results

	satisfied := 0
	for _, r := range results {
		if r.Satisfied {
			satisfied++
		}
	}
	report.Scores.FeatureCompleteness = float64(satisfied) / float64(len(results))
}

// scoreCoverage measures what fraction of A2UI component types are used.
func (s *Scorer) scoreCoverage(report *EvalReport) {
	allTypes := []protocol.ComponentType{
		protocol.CompText, protocol.CompRow, protocol.CompColumn,
		protocol.CompCard, protocol.CompButton, protocol.CompTextField,
		protocol.CompCheckBox, protocol.CompSplitView, protocol.CompSearchField,
		protocol.CompOutlineView, protocol.CompRichTextEditor,
		protocol.CompSlider, protocol.CompImage, protocol.CompIcon,
		protocol.CompDivider, protocol.CompList, protocol.CompTabs,
		protocol.CompModal, protocol.CompChoicePicker, protocol.CompDateTimeInput,
	}

	used := make(map[protocol.ComponentType]bool)
	for _, sid := range s.genSess.SurfaceIDs() {
		surf := s.genSess.GetSurface(sid)
		if surf == nil {
			continue
		}
		for _, compID := range surf.Tree().All() {
			comp, ok := surf.Tree().Get(compID)
			if ok {
				used[comp.Type] = true
			}
		}
	}

	// Coverage = types used / types in reference (or all types if no ref)
	if s.refSess != nil {
		refTypes := make(map[protocol.ComponentType]bool)
		for _, sid := range s.refSess.SurfaceIDs() {
			surf := s.refSess.GetSurface(sid)
			if surf == nil {
				continue
			}
			for _, compID := range surf.Tree().All() {
				comp, ok := surf.Tree().Get(compID)
				if ok {
					refTypes[comp.Type] = true
				}
			}
		}
		if len(refTypes) > 0 {
			covered := 0
			for t := range refTypes {
				if used[t] {
					covered++
				}
			}
			report.Scores.ComponentCoverage = float64(covered) / float64(len(refTypes))
			return
		}
	}

	// No reference: coverage = used / total known types
	report.Scores.ComponentCoverage = float64(len(used)) / float64(len(allTypes))
}

// analyzeFailures categorizes test failures and structural issues into actionable entries.
func (s *Scorer) analyzeFailures(report *EvalReport) {
	// Categorize test failures
	for _, t := range report.TestResults {
		if t.Passed {
			continue
		}
		entry := FailureEntry{
			Description: fmt.Sprintf("test %q: %s", t.Name, t.Error),
		}

		errLower := strings.ToLower(t.Error)
		switch {
		case strings.Contains(errLower, "filtercontains") || strings.Contains(errLower, "filtercontainsany"):
			entry.Category = "wrong_function"
			entry.Suggestion = "use filterContainsAny(collection, [field1,field2], query) for multi-field search"
		case strings.Contains(errLower, "not found"):
			entry.Category = "missing_component"
			entry.Suggestion = "ensure the component is created via updateComponents"
		case strings.Contains(errLower, "children"):
			entry.Category = "bad_structure"
			entry.Suggestion = "check parent-child relationships in component tree"
		case strings.Contains(errLower, "data") || strings.Contains(errLower, "path"):
			entry.Category = "data_model"
			entry.Suggestion = "verify data model paths match what tests expect"
		default:
			entry.Category = "test_logic"
			entry.Suggestion = "review test assertion against actual component state"
		}

		report.Failures = append(report.Failures, entry)
	}

	// Structural failures
	if report.StructuralDiff != nil {
		for _, id := range report.StructuralDiff.MissingComponents {
			report.Failures = append(report.Failures, FailureEntry{
				Category:    "missing_component",
				ComponentID: id,
				Description: fmt.Sprintf("component %s present in reference but missing from generated output", id),
				Suggestion:  "add this component via updateComponents",
			})
		}
		for _, tm := range report.StructuralDiff.TypeMismatches {
			report.Failures = append(report.Failures, FailureEntry{
				Category:    "bad_structure",
				ComponentID: tm.ComponentID,
				Description: fmt.Sprintf("component %s has type %s but reference has %s", tm.ComponentID, tm.GenType, tm.RefType),
				Suggestion:  fmt.Sprintf("change component type to %s", tm.RefType),
			})
		}
	}

	// Feature failures
	for _, f := range report.FeatureChecks {
		if f.Satisfied {
			continue
		}
		report.Failures = append(report.Failures, FailureEntry{
			Category:    "missing_component",
			Description: fmt.Sprintf("feature %q not satisfied: %s", f.Feature, f.Evidence),
			Suggestion:  fmt.Sprintf("add %s support as described in the prompt", f.Feature),
		})
	}
}

// loadSession parses a JSONL file and builds a headless session.
func loadSession(path string) (*engine.Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rend := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := engine.NewSession(rend, disp)
	cm := engine.NewChannelManager(sess)
	sess.SetChannelManager(cm)

	parser := protocol.NewParser(f)
	for {
		msg, err := parser.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip unparseable lines
		}
		if msg.Type == protocol.MsgTest {
			continue // don't process test messages as app state
		}
		sess.HandleMessage(msg)
	}
	sess.FlushPendingComponents()

	return sess, nil
}

// firstSurface returns the first surface in a session, or nil.
func firstSurface(sess *engine.Session) *engine.Surface {
	ids := sess.SurfaceIDs()
	if len(ids) == 0 {
		return nil
	}
	return sess.GetSurface(ids[0])
}

// extractDataModelPaths walks the reference surface's tree and collects
// all dataBinding paths as key comparison points.
func extractDataModelPaths(surf *engine.Surface) []string {
	seen := make(map[string]bool)
	for _, compID := range surf.Tree().All() {
		comp, ok := surf.Tree().Get(compID)
		if !ok {
			continue
		}
		if comp.Props.DataBinding != "" {
			seen[comp.Props.DataBinding] = true
		}
	}
	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	return paths
}

// WriteReport writes an EvalReport as JSON to the given path.
func WriteReport(report *EvalReport, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// FormatReport produces a human-readable summary of an eval report.
func FormatReport(r *EvalReport) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Evaluation Report: %s\n", r.PromptFile)
	if r.RefFile != "" {
		fmt.Fprintf(&b, "Reference: %s\n", r.RefFile)
	}
	fmt.Fprintf(&b, "Attempt: %d  Time: %s\n\n", r.Attempt, r.Timestamp.Format(time.RFC3339))

	// Scores
	fmt.Fprintf(&b, "Scores:\n")
	fmt.Fprintf(&b, "  Tests:      %.2f\n", r.Scores.TestPassRate)
	fmt.Fprintf(&b, "  Structural: %.2f\n", r.Scores.StructuralSimilarity)
	fmt.Fprintf(&b, "  Features:   %.2f\n", r.Scores.FeatureCompleteness)
	fmt.Fprintf(&b, "  DataModel:  %.2f\n", r.Scores.DataModelCorrectness)
	fmt.Fprintf(&b, "  Coverage:   %.2f\n", r.Scores.ComponentCoverage)
	fmt.Fprintf(&b, "  OVERALL:    %.2f\n\n", r.Overall)

	// Test results
	if len(r.TestResults) > 0 {
		passed, failed := 0, 0
		for _, t := range r.TestResults {
			if t.Passed {
				passed++
			} else {
				failed++
			}
		}
		fmt.Fprintf(&b, "Tests: %d passed, %d failed, %d total\n", passed, failed, len(r.TestResults))
		for _, t := range r.TestResults {
			if t.Passed {
				fmt.Fprintf(&b, "  PASS  %s (%d assertions)\n", t.Name, t.Assertions)
			} else {
				fmt.Fprintf(&b, "  FAIL  %s: %s\n", t.Name, t.Error)
			}
		}
		b.WriteString("\n")
	}

	// Failures
	if len(r.Failures) > 0 {
		fmt.Fprintf(&b, "Failures (%d):\n", len(r.Failures))
		for _, f := range r.Failures {
			fmt.Fprintf(&b, "  [%s] %s\n", f.Category, f.Description)
			if f.Suggestion != "" {
				fmt.Fprintf(&b, "        → %s\n", f.Suggestion)
			}
		}
	}

	return b.String()
}
