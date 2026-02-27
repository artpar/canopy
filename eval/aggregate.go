package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AggregateReports loads all eval reports from sample_apps subdirectories
// and produces a cross-prompt analysis identifying systemic issues.
func AggregateReports(sampleAppsDir string) (*AggregateAnalysis, error) {
	entries, err := os.ReadDir(sampleAppsDir)
	if err != nil {
		return nil, fmt.Errorf("read sample_apps dir: %w", err)
	}

	analysis := &AggregateAnalysis{
		FailureCounts: make(map[string]int),
	}

	var allFailures []appFailure

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		appName := entry.Name()
		evalDir := filepath.Join(sampleAppsDir, appName, "eval")

		reports, err := loadReportsFromDir(evalDir)
		if err != nil {
			continue // skip apps without eval data
		}
		if len(reports) == 0 {
			continue
		}

		analysis.TotalApps++

		// Use the best attempt (highest overall score) for aggregation
		var best *EvalReport
		for i := range reports {
			analysis.TotalReports++
			if best == nil || reports[i].Overall > best.Overall {
				best = &reports[i]
			}
		}

		analysis.AvgOverall += best.Overall
		analysis.AvgScores.TestPassRate += best.Scores.TestPassRate
		analysis.AvgScores.StructuralSimilarity += best.Scores.StructuralSimilarity
		analysis.AvgScores.FeatureCompleteness += best.Scores.FeatureCompleteness
		analysis.AvgScores.DataModelCorrectness += best.Scores.DataModelCorrectness
		analysis.AvgScores.ComponentCoverage += best.Scores.ComponentCoverage

		for _, f := range best.Failures {
			analysis.FailureCounts[f.Category]++
			allFailures = append(allFailures, appFailure{
				app:         appName,
				category:    f.Category,
				description: f.Description,
			})
		}
	}

	if analysis.TotalApps > 0 {
		n := float64(analysis.TotalApps)
		analysis.AvgOverall /= n
		analysis.AvgScores.TestPassRate /= n
		analysis.AvgScores.StructuralSimilarity /= n
		analysis.AvgScores.FeatureCompleteness /= n
		analysis.AvgScores.DataModelCorrectness /= n
		analysis.AvgScores.ComponentCoverage /= n
	}

	// Identify patterns appearing in 2+ apps
	analysis.Patterns = findPatterns(allFailures)

	// Generate patch suggestions from patterns
	analysis.Patches = generatePatches(analysis.Patterns)

	return analysis, nil
}

func loadReportsFromDir(dir string) ([]EvalReport, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var reports []EvalReport
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "attempt_") || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var report EvalReport
		if err := json.Unmarshal(data, &report); err != nil {
			continue
		}
		reports = append(reports, report)
	}
	return reports, nil
}

type appFailure struct {
	app         string
	category    string
	description string
}

func findPatterns(failures []appFailure) []FailurePattern {
	// Group by normalized description
	type patternKey struct {
		category    string
		description string
	}
	groups := make(map[patternKey]map[string]bool) // key → set of apps

	for _, f := range failures {
		key := patternKey{category: f.category, description: normalizeDescription(f.description)}
		if groups[key] == nil {
			groups[key] = make(map[string]bool)
		}
		groups[key][f.app] = true
	}

	var patterns []FailurePattern
	for key, apps := range groups {
		if len(apps) < 2 {
			continue
		}
		appList := make([]string, 0, len(apps))
		for app := range apps {
			appList = append(appList, app)
		}
		sort.Strings(appList)
		patterns = append(patterns, FailurePattern{
			Category:    key.category,
			Description: key.description,
			Apps:        appList,
			Count:       len(appList),
		})
	}

	// Sort by count descending
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Count > patterns[j].Count
	})

	return patterns
}

// normalizeDescription strips component-specific details to find common patterns.
func normalizeDescription(desc string) string {
	// Remove component IDs (alphanumeric_underscore patterns after "component")
	// Keep the general structure
	return strings.ToLower(strings.TrimSpace(desc))
}

func generatePatches(patterns []FailurePattern) []PromptPatch {
	var patches []PromptPatch

	for _, p := range patterns {
		var patch PromptPatch
		switch p.Category {
		case "wrong_function":
			patch = PromptPatch{
				Section:     "COMPONENT PATTERNS / Functions",
				Description: fmt.Sprintf("Pattern in %d apps: %s — add explicit correct function usage example", p.Count, p.Description),
				Priority:    "high",
			}
		case "missing_component":
			patch = PromptPatch{
				Section:     "COMPONENT PATTERNS / Required Components",
				Description: fmt.Sprintf("Pattern in %d apps: %s — add required component checklist", p.Count, p.Description),
				Priority:    "high",
			}
		case "bad_structure":
			patch = PromptPatch{
				Section:     "COMPONENT PATTERNS / Layout",
				Description: fmt.Sprintf("Pattern in %d apps: %s — add structural constraint", p.Count, p.Description),
				Priority:    "medium",
			}
		case "data_model":
			patch = PromptPatch{
				Section:     "DATA MODEL PATTERNS",
				Description: fmt.Sprintf("Pattern in %d apps: %s — add data model path convention", p.Count, p.Description),
				Priority:    "medium",
			}
		case "test_logic":
			patch = PromptPatch{
				Section:     "WORKFLOW / Testing",
				Description: fmt.Sprintf("Pattern in %d apps: %s — add test writing guidance", p.Count, p.Description),
				Priority:    "low",
			}
		default:
			patch = PromptPatch{
				Section:     "General",
				Description: fmt.Sprintf("Pattern in %d apps [%s]: %s", p.Count, p.Category, p.Description),
				Priority:    "medium",
			}
		}
		patches = append(patches, patch)
	}

	return patches
}

// FormatAggregate produces a human-readable summary of the aggregate analysis.
func FormatAggregate(a *AggregateAnalysis) string {
	var b strings.Builder

	fmt.Fprintf(&b, "AGGREGATE ANALYSIS: %d apps, %d total reports\n\n", a.TotalApps, a.TotalReports)
	fmt.Fprintf(&b, "Average Scores:\n")
	fmt.Fprintf(&b, "  Overall:     %.2f\n", a.AvgOverall)
	fmt.Fprintf(&b, "  Tests:       %.2f\n", a.AvgScores.TestPassRate)
	fmt.Fprintf(&b, "  Structural:  %.2f\n", a.AvgScores.StructuralSimilarity)
	fmt.Fprintf(&b, "  Features:    %.2f\n", a.AvgScores.FeatureCompleteness)
	fmt.Fprintf(&b, "  DataModel:   %.2f\n", a.AvgScores.DataModelCorrectness)
	fmt.Fprintf(&b, "  Coverage:    %.2f\n", a.AvgScores.ComponentCoverage)

	if len(a.FailureCounts) > 0 {
		fmt.Fprintf(&b, "\nFailure Counts by Category:\n")
		for cat, count := range a.FailureCounts {
			fmt.Fprintf(&b, "  %-20s %d\n", cat, count)
		}
	}

	if len(a.Patterns) > 0 {
		fmt.Fprintf(&b, "\nRecurring Patterns (2+ apps):\n")
		for _, p := range a.Patterns {
			fmt.Fprintf(&b, "  [%s] %s (in %s)\n", p.Category, p.Description, strings.Join(p.Apps, ", "))
		}
	}

	if len(a.Patches) > 0 {
		fmt.Fprintf(&b, "\nSuggested System Prompt Patches:\n")
		for i, p := range a.Patches {
			fmt.Fprintf(&b, "  %d. [%s] Section: %s\n     %s\n", i+1, p.Priority, p.Section, p.Description)
		}
	}

	return b.String()
}
