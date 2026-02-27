package eval

import "time"

// EvalReport holds the complete evaluation of a generated JSONL app.
type EvalReport struct {
	PromptFile string    `json:"promptFile"`
	RefFile    string    `json:"refFile,omitempty"`
	Attempt    int       `json:"attempt"`
	Timestamp  time.Time `json:"timestamp"`

	Scores        Scores          `json:"scores"`
	TestResults   []TestDetail    `json:"testResults"`
	StructuralDiff *StructuralDiff `json:"structuralDiff,omitempty"`
	FeatureChecks []FeatureResult `json:"featureChecks,omitempty"`
	Failures      []FailureEntry  `json:"failures"`
	Overall       float64         `json:"overall"`
}

// Scores holds the individual dimension scores (0.0 to 1.0).
type Scores struct {
	TestPassRate          float64 `json:"testPassRate"`
	StructuralSimilarity  float64 `json:"structuralSimilarity"`
	FeatureCompleteness   float64 `json:"featureCompleteness"`
	DataModelCorrectness  float64 `json:"dataModelCorrectness"`
	ComponentCoverage     float64 `json:"componentCoverage"`
}

// TestDetail records one test's outcome.
type TestDetail struct {
	Name       string `json:"name"`
	Passed     bool   `json:"passed"`
	Error      string `json:"error,omitempty"`
	Assertions int    `json:"assertions"`
	Skipped    int    `json:"skipped,omitempty"`
}

// StructuralDiff captures tree comparison between generated and reference.
type StructuralDiff struct {
	MissingComponents []string        `json:"missingComponents"`
	ExtraComponents   []string        `json:"extraComponents"`
	TypeMismatches    []TypeMismatch  `json:"typeMismatches,omitempty"`
	ChildrenDiffs     []ChildrenDiff  `json:"childrenDiffs,omitempty"`
	Similarity        float64         `json:"similarity"`
}

// TypeMismatch records a component present in both trees with different types.
type TypeMismatch struct {
	ComponentID string `json:"componentId"`
	RefType     string `json:"refType"`
	GenType     string `json:"genType"`
}

// ChildrenDiff records a component whose children differ between ref and gen.
type ChildrenDiff struct {
	ComponentID string   `json:"componentId"`
	RefChildren []string `json:"refChildren"`
	GenChildren []string `json:"genChildren"`
}

// FeatureResult records whether a feature extracted from the prompt is satisfied.
type FeatureResult struct {
	Feature   string `json:"feature"`
	Satisfied bool   `json:"satisfied"`
	Evidence  string `json:"evidence"`
}

// FailureEntry categorizes a single failure for feedback generation.
type FailureEntry struct {
	Category    string `json:"category"` // wrong_function, missing_component, bad_structure, data_model, test_logic
	Description string `json:"description"`
	ComponentID string `json:"componentId,omitempty"`
	Suggestion  string `json:"suggestion"`
}

// AggregateAnalysis summarizes patterns across multiple eval reports.
type AggregateAnalysis struct {
	TotalApps     int                `json:"totalApps"`
	TotalReports  int                `json:"totalReports"`
	AvgOverall    float64            `json:"avgOverall"`
	AvgScores     Scores             `json:"avgScores"`
	FailureCounts map[string]int     `json:"failureCounts"` // category → count
	Patterns      []FailurePattern   `json:"patterns"`      // patterns in 2+ apps
	Patches       []PromptPatch      `json:"patches"`
}

// FailurePattern is a recurring failure across multiple apps.
type FailurePattern struct {
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Apps        []string `json:"apps"` // which apps exhibit this
	Count       int      `json:"count"`
}

// PromptPatch suggests a system prompt change to address a recurring failure.
type PromptPatch struct {
	Section     string `json:"section"`
	Description string `json:"description"`
	Priority    string `json:"priority"` // high, medium, low
}

// ComputeOverall calculates the weighted overall score from individual dimensions.
// Weights: tests=0.40, structural=0.25, features=0.20, dataModel=0.10, coverage=0.05
func (r *EvalReport) ComputeOverall() float64 {
	r.Overall = 0.40*r.Scores.TestPassRate +
		0.25*r.Scores.StructuralSimilarity +
		0.20*r.Scores.FeatureCompleteness +
		0.10*r.Scores.DataModelCorrectness +
		0.05*r.Scores.ComponentCoverage
	return r.Overall
}
