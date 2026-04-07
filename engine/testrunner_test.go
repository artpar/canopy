package engine

import (
	"canopy/renderer"
	"strings"
	"testing"
)

func runTestHelper(t *testing.T, jsonl string) []TestResult {
	t.Helper()
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	results, err := RunTests(strings.NewReader(jsonl), mock, disp)
	if err != nil {
		t.Fatalf("RunTests error: %v", err)
	}
	return results
}

// runTestWithMock returns both results and mock so tests can set layout/style.
func runTestWithMock(t *testing.T, jsonl string, setup func(*renderer.MockRenderer)) []TestResult {
	t.Helper()
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	if setup != nil {
		setup(mock)
	}
	results, err := RunTests(strings.NewReader(jsonl), mock, disp)
	if err != nil {
		t.Fatalf("RunTests error: %v", err)
	}
	return results
}

// expectPass asserts a single test passed.
func expectPass(t *testing.T, results []TestResult, idx int) {
	t.Helper()
	if idx >= len(results) {
		t.Fatalf("expected result at index %d but only got %d results", idx, len(results))
	}
	if !results[idx].Passed {
		t.Errorf("expected test %d %q to pass, got error: %s", idx, results[idx].Name, results[idx].Error)
	}
}

// expectFail asserts a single test failed and error contains substr.
func expectFail(t *testing.T, results []TestResult, idx int, substr string) {
	t.Helper()
	if idx >= len(results) {
		t.Fatalf("expected result at index %d but only got %d results", idx, len(results))
	}
	if results[idx].Passed {
		t.Errorf("expected test %d %q to fail", idx, results[idx].Name)
		return
	}
	if substr != "" && !strings.Contains(results[idx].Error, substr) {
		t.Errorf("test %d error = %q, want substring %q", idx, results[idx].Error, substr)
	}
}

func TestRunnerPassingTest(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/name","value":"Alice"}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":{"path":"/name"},"variant":"h1"}}]}
{"type":"test","surfaceId":"s1","name":"check text","steps":[{"assert":"component","componentId":"t1","props":{"content":"Alice","variant":"h1"}}]}`

	results := runTestHelper(t, jsonl)
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if !results[0].Passed {
		t.Errorf("test failed: %s", results[0].Error)
	}
	if results[0].Assertions != 1 {
		t.Errorf("assertions = %d, want 1", results[0].Assertions)
	}
}

func TestRunnerFailingTest(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"Hello"}}]}
{"type":"test","surfaceId":"s1","name":"wrong content","steps":[{"assert":"component","componentId":"t1","props":{"content":"Goodbye"}}]}`

	results := runTestHelper(t, jsonl)
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if results[0].Passed {
		t.Error("expected test to fail")
	}
	if !strings.Contains(results[0].Error, "assertComponent") {
		t.Errorf("error = %q, expected assertComponent message", results[0].Error)
	}
}

func TestRunnerSimulateAndAssert(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/val","value":""}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"field","type":"TextField","props":{"value":{"path":"/val"},"dataBinding":"/val"}},{"componentId":"display","type":"Text","props":{"content":{"path":"/val"}}}]}
{"type":"test","surfaceId":"s1","name":"simulate change","steps":[{"simulate":"event","componentId":"field","event":"change","eventData":"Bob"},{"assert":"dataModel","path":"/val","value":"Bob"},{"assert":"component","componentId":"display","props":{"content":"Bob"}}]}`

	results := runTestHelper(t, jsonl)
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if !results[0].Passed {
		t.Errorf("test failed: %s", results[0].Error)
	}
	if results[0].Assertions != 2 {
		t.Errorf("assertions = %d, want 2", results[0].Assertions)
	}
}

func TestRunnerAssertAction(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/x","value":"hello"}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Go","onClick":{"action":{"event":{"name":"doIt","dataRefs":["/x"]}}}}}]}
{"type":"test","surfaceId":"s1","name":"action test","steps":[{"simulate":"event","componentId":"btn","event":"click"},{"assert":"action","name":"doIt","data":{"/x":"hello"}}]}`

	results := runTestHelper(t, jsonl)
	if !results[0].Passed {
		t.Errorf("test failed: %s", results[0].Error)
	}
}

func TestRunnerAssertNotExists(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"hi"}}]}
{"type":"test","surfaceId":"s1","name":"not exists","steps":[{"assert":"notExists","componentId":"ghost"}]}`

	results := runTestHelper(t, jsonl)
	if !results[0].Passed {
		t.Errorf("test failed: %s", results[0].Error)
	}
}

func TestRunnerAssertNotExistsFails(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"hi"}}]}
{"type":"test","surfaceId":"s1","name":"should fail","steps":[{"assert":"notExists","componentId":"t1"}]}`

	results := runTestHelper(t, jsonl)
	if results[0].Passed {
		t.Error("expected test to fail")
	}
}

func TestRunnerAssertCount(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"col","type":"Column","children":["a","b","c"]},{"componentId":"a","type":"Text","props":{"content":"A"}},{"componentId":"b","type":"Text","props":{"content":"B"}},{"componentId":"c","type":"Text","props":{"content":"C"}}]}
{"type":"test","surfaceId":"s1","name":"count children","steps":[{"assert":"count","componentId":"col","count":3}]}`

	results := runTestHelper(t, jsonl)
	if !results[0].Passed {
		t.Errorf("test failed: %s", results[0].Error)
	}
}

func TestRunnerAssertChildren(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"col","type":"Column","children":["a","b"]},{"componentId":"a","type":"Text","props":{"content":"A"}},{"componentId":"b","type":"Text","props":{"content":"B"}}]}
{"type":"test","surfaceId":"s1","name":"children order","steps":[{"assert":"children","componentId":"col","children":["a","b"]}]}`

	results := runTestHelper(t, jsonl)
	if !results[0].Passed {
		t.Errorf("test failed: %s", results[0].Error)
	}
}

func TestRunnerAssertComponentType(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Click"}}]}
{"type":"test","surfaceId":"s1","name":"check type","steps":[{"assert":"component","componentId":"btn","componentType":"Button","props":{"label":"Click"}}]}`

	results := runTestHelper(t, jsonl)
	if !results[0].Passed {
		t.Errorf("test failed: %s", results[0].Error)
	}
}

func TestRunnerAssertComponentTypeWrong(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Click"}}]}
{"type":"test","surfaceId":"s1","name":"wrong type","steps":[{"assert":"component","componentId":"btn","componentType":"Text"}]}`

	results := runTestHelper(t, jsonl)
	if results[0].Passed {
		t.Error("expected test to fail for wrong component type")
	}
}

func TestRunnerSideEffectsPersist(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/val","value":""}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"field","type":"TextField","props":{"value":{"path":"/val"},"dataBinding":"/val"}}]}
{"type":"test","surfaceId":"s1","name":"set value","steps":[{"simulate":"event","componentId":"field","event":"change","eventData":"first"}]}
{"type":"test","surfaceId":"s1","name":"value persists","steps":[{"assert":"dataModel","path":"/val","value":"first"}]}`

	results := runTestHelper(t, jsonl)
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("test %q failed: %s", r.Name, r.Error)
		}
	}
}

func TestRunnerActionsClearBetweenTests(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Go","onClick":{"action":{"event":{"name":"doIt"}}}}}]}
{"type":"test","surfaceId":"s1","name":"fire action","steps":[{"simulate":"event","componentId":"btn","event":"click"},{"assert":"action","name":"doIt"}]}
{"type":"test","surfaceId":"s1","name":"action cleared","steps":[{"assert":"action","name":"doIt"}]}`

	results := runTestHelper(t, jsonl)
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if !results[0].Passed {
		t.Errorf("first test failed: %s", results[0].Error)
	}
	// Second test should fail because actions were cleared
	if results[1].Passed {
		t.Error("second test should fail — actions should be cleared between tests")
	}
}

func TestRunnerLayoutAssertMock(t *testing.T) {
	// With MockRenderer, QueryLayout returns zero values (no real views)
	// This tests that assertLayout works with zero/empty layout
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"box","type":"Column","children":["t1"]},{"componentId":"t1","type":"Text","props":{"content":"hi"}}]}
{"type":"test","surfaceId":"s1","name":"layout zero","steps":[{"assert":"layout","componentId":"box","layout":{}}]}`

	results := runTestHelper(t, jsonl)
	if !results[0].Passed {
		t.Errorf("test failed: %s", results[0].Error)
	}
}

func TestRunnerContactFormFixture(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	results, err := RunTestFile("../testdata/contact_form_test.jsonl", mock, disp)
	if err != nil {
		t.Fatalf("RunTestFile error: %v", err)
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("FAIL %s: %s", r.Name, r.Error)
		}
	}
}

func TestRunnerDataModelMissing(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"test","surfaceId":"s1","name":"missing path","steps":[{"assert":"dataModel","path":"/nope","value":"x"}]}`

	results := runTestHelper(t, jsonl)
	if results[0].Passed {
		t.Error("expected test to fail for missing path")
	}
}

func TestRunnerComponentNotFound(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"test","surfaceId":"s1","name":"no component","steps":[{"assert":"component","componentId":"ghost","props":{"content":"x"}}]}`

	results := runTestHelper(t, jsonl)
	if results[0].Passed {
		t.Error("expected test to fail for missing component")
	}
}

// ===== Helper function tests =====

func TestJsonEqualIntVsFloat(t *testing.T) {
	// JSON decodes numbers as float64; jsonEqual must handle int vs float64
	if !jsonEqual(42, 42.0) {
		t.Error("jsonEqual(42, 42.0) should be true")
	}
	if !jsonEqual(0, 0.0) {
		t.Error("jsonEqual(0, 0.0) should be true")
	}
}

func TestJsonEqualStrings(t *testing.T) {
	if !jsonEqual("hello", "hello") {
		t.Error("same strings should be equal")
	}
	if jsonEqual("hello", "world") {
		t.Error("different strings should not be equal")
	}
}

func TestJsonEqualNested(t *testing.T) {
	a := map[string]interface{}{"x": 1.0, "y": "z"}
	b := map[string]interface{}{"x": 1.0, "y": "z"}
	if !jsonEqual(a, b) {
		t.Error("equal nested objects should be equal")
	}
	c := map[string]interface{}{"x": 2.0, "y": "z"}
	if jsonEqual(a, c) {
		t.Error("different nested objects should not be equal")
	}
}

func TestJsonEqualNulls(t *testing.T) {
	if !jsonEqual(nil, nil) {
		t.Error("nil == nil should be true")
	}
	if jsonEqual(nil, "x") {
		t.Error("nil != string should be false")
	}
}

func TestJsonEqualBooleans(t *testing.T) {
	if !jsonEqual(true, true) {
		t.Error("true == true")
	}
	if jsonEqual(true, false) {
		t.Error("true != false")
	}
}

func TestIsZeroValue(t *testing.T) {
	cases := []struct {
		val  interface{}
		want bool
	}{
		{nil, true},
		{0, true},
		{0.0, true},
		{"", true},
		{false, true},
		{1, false},
		{0.1, false},
		{"x", false},
		{true, false},
	}
	for _, tc := range cases {
		got := isZeroValue(tc.val)
		if got != tc.want {
			t.Errorf("isZeroValue(%v) = %v, want %v", tc.val, got, tc.want)
		}
	}
}

// ===== Test runner mechanics =====

func TestRunnerEmptyFile(t *testing.T) {
	results := runTestHelper(t, "")
	if len(results) != 0 {
		t.Errorf("empty file should produce 0 results, got %d", len(results))
	}
}

func TestRunnerNoTests(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"hi"}}]}`
	results := runTestHelper(t, jsonl)
	if len(results) != 0 {
		t.Errorf("file with no tests should produce 0 results, got %d", len(results))
	}
}

func TestRunnerEmptySteps(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"test","surfaceId":"s1","name":"empty","steps":[]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
	if results[0].Assertions != 0 {
		t.Errorf("assertions = %d, want 0", results[0].Assertions)
	}
}

func TestRunnerStepNoDiscriminator(t *testing.T) {
	// Step with neither assert nor simulate is silently skipped
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"test","surfaceId":"s1","name":"no op","steps":[{"componentId":"t1"}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
	if results[0].Assertions != 0 {
		t.Errorf("assertions = %d, want 0 (no-op step)", results[0].Assertions)
	}
}

func TestRunnerUnknownAssertType(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"test","surfaceId":"s1","name":"bad assert","steps":[{"assert":"bogus","componentId":"t1"}]}`
	results := runTestHelper(t, jsonl)
	expectFail(t, results, 0, "unknown assert type")
}

func TestRunnerUnknownSimulateType(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"test","surfaceId":"s1","name":"bad simulate","steps":[{"simulate":"bogus","componentId":"t1"}]}`
	results := runTestHelper(t, jsonl)
	expectFail(t, results, 0, "unknown simulate type")
}

func TestRunnerMalformedJSONL(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	_, err := RunTests(strings.NewReader("not valid json"), mock, disp)
	if err == nil {
		t.Error("expected parse error for malformed JSONL")
	}
}

func TestRunnerFileNotFound(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	_, err := RunTestFile("/nonexistent/path.jsonl", mock, disp)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestRunnerShortCircuitOnFailure(t *testing.T) {
	// First step fails, second step should not run (assertion count = 1)
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"Hello"}}]}
{"type":"test","surfaceId":"s1","name":"short circuit","steps":[{"assert":"component","componentId":"t1","props":{"content":"Wrong"}},{"assert":"component","componentId":"t1","props":{"content":"Hello"}}]}`
	results := runTestHelper(t, jsonl)
	expectFail(t, results, 0, "assertComponent")
	if results[0].Assertions != 1 {
		t.Errorf("assertions = %d, want 1 (short circuit after first failure)", results[0].Assertions)
	}
}

func TestRunnerStepNumberInError(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"Hello"}}]}
{"type":"test","surfaceId":"s1","name":"step num","steps":[{"assert":"component","componentId":"t1","props":{"content":"Hello"}},{"assert":"component","componentId":"t1","props":{"content":"Wrong"}}]}`
	results := runTestHelper(t, jsonl)
	expectFail(t, results, 0, "step 2:")
}

func TestRunnerTestsInterleavedWithAppMessages(t *testing.T) {
	// Test messages should all execute AFTER all app messages, regardless of interleaving
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"test","surfaceId":"s1","name":"check component","steps":[{"assert":"component","componentId":"t1","props":{"content":"Hello"}}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"Hello"}}]}`
	results := runTestHelper(t, jsonl)
	// The test message comes before updateComponents in the file,
	// but the runner processes all non-test messages first, then runs tests.
	expectPass(t, results, 0)
}

// ===== assertComponent edge cases =====

func TestRunnerComponentWrongSurface(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"hi"}}]}
{"type":"test","surfaceId":"s999","name":"wrong surface","steps":[{"assert":"component","componentId":"t1","props":{"content":"hi"}}]}`
	results := runTestHelper(t, jsonl)
	expectFail(t, results, 0, "surface")
}

func TestRunnerComponentSubsetMatch(t *testing.T) {
	// Assert only some props — extra resolved props should not cause failure
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"Hello","variant":"h1"}}]}
{"type":"test","surfaceId":"s1","name":"subset","steps":[{"assert":"component","componentId":"t1","props":{"content":"Hello"}}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
}

func TestRunnerComponentTypeOnlyNoProps(t *testing.T) {
	// Assert only component type, no props
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"hi"}}]}
{"type":"test","surfaceId":"s1","name":"type only","steps":[{"assert":"component","componentId":"t1","componentType":"Text"}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
}

func TestRunnerComponentExistsOnly(t *testing.T) {
	// Assert component exists with no type check and no props
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"hi"}}]}
{"type":"test","surfaceId":"s1","name":"exists only","steps":[{"assert":"component","componentId":"t1"}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
}

func TestRunnerComponentAfterDataBindingUpdate(t *testing.T) {
	// Component re-resolves to current data model values
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/v","value":"before"}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"field","type":"TextField","props":{"value":{"path":"/v"},"dataBinding":"/v"}},{"componentId":"t1","type":"Text","props":{"content":{"path":"/v"}}}]}
{"type":"test","surfaceId":"s1","name":"before","steps":[{"assert":"component","componentId":"t1","props":{"content":"before"}}]}
{"type":"test","surfaceId":"s1","name":"after simulate","steps":[{"simulate":"event","componentId":"field","event":"change","eventData":"after"},{"assert":"component","componentId":"t1","props":{"content":"after"}}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
	expectPass(t, results, 1)
}

// ===== assertDataModel edge cases =====

func TestRunnerDataModelNestedPath(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/user","value":{"name":"Alice","age":30}}]}
{"type":"test","surfaceId":"s1","name":"nested","steps":[{"assert":"dataModel","path":"/user/name","value":"Alice"},{"assert":"dataModel","path":"/user/age","value":30}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
}

func TestRunnerDataModelNullValue(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/x","value":null}]}
{"type":"test","surfaceId":"s1","name":"null","steps":[{"assert":"dataModel","path":"/x","value":null}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
}

func TestRunnerDataModelBooleanValue(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/flag","value":true}]}
{"type":"test","surfaceId":"s1","name":"bool","steps":[{"assert":"dataModel","path":"/flag","value":true}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
}

func TestRunnerDataModelWrongValue(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/x","value":"actual"}]}
{"type":"test","surfaceId":"s1","name":"mismatch","steps":[{"assert":"dataModel","path":"/x","value":"expected"}]}`
	results := runTestHelper(t, jsonl)
	expectFail(t, results, 0, "assertDataModel")
}

func TestRunnerDataModelArrayValue(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/items","value":["a","b","c"]}]}
{"type":"test","surfaceId":"s1","name":"array","steps":[{"assert":"dataModel","path":"/items","value":["a","b","c"]}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
}

func TestRunnerDataModelEmptyString(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/x","value":""}]}
{"type":"test","surfaceId":"s1","name":"empty str","steps":[{"assert":"dataModel","path":"/x","value":""}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
}

func TestRunnerDataModelWrongSurface(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/x","value":"y"}]}
{"type":"test","surfaceId":"s999","name":"bad surface","steps":[{"assert":"dataModel","path":"/x","value":"y"}]}`
	results := runTestHelper(t, jsonl)
	expectFail(t, results, 0, "surface")
}

// ===== assertChildren edge cases =====

func TestRunnerChildrenWrongOrder(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"col","type":"Column","children":["a","b"]},{"componentId":"a","type":"Text","props":{"content":"A"}},{"componentId":"b","type":"Text","props":{"content":"B"}}]}
{"type":"test","surfaceId":"s1","name":"wrong order","steps":[{"assert":"children","componentId":"col","children":["b","a"]}]}`
	results := runTestHelper(t, jsonl)
	expectFail(t, results, 0, "assertChildren")
}

func TestRunnerChildrenLengthMismatch(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"col","type":"Column","children":["a"]},{"componentId":"a","type":"Text","props":{"content":"A"}}]}
{"type":"test","surfaceId":"s1","name":"length mismatch","steps":[{"assert":"children","componentId":"col","children":["a","b"]}]}`
	results := runTestHelper(t, jsonl)
	expectFail(t, results, 0, "assertChildren")
}

func TestRunnerChildrenEmptyExpected(t *testing.T) {
	// Leaf component has no children — asserting empty children should pass
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"hi"}}]}
{"type":"test","surfaceId":"s1","name":"empty children","steps":[{"assert":"children","componentId":"t1","children":[]}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
}

func TestRunnerChildrenComponentNotFound(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"test","surfaceId":"s1","name":"ghost children","steps":[{"assert":"children","componentId":"ghost","children":["a"]}]}`
	results := runTestHelper(t, jsonl)
	expectFail(t, results, 0, "not found")
}

// ===== assertCount edge cases =====

func TestRunnerCountZero(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"hi"}}]}
{"type":"test","surfaceId":"s1","name":"count zero","steps":[{"assert":"count","componentId":"t1","count":0}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
}

func TestRunnerCountWrong(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"col","type":"Column","children":["a"]},{"componentId":"a","type":"Text","props":{"content":"A"}}]}
{"type":"test","surfaceId":"s1","name":"wrong count","steps":[{"assert":"count","componentId":"col","count":5}]}`
	results := runTestHelper(t, jsonl)
	expectFail(t, results, 0, "assertCount")
}

func TestRunnerCountComponentNotFound(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"test","surfaceId":"s1","name":"ghost count","steps":[{"assert":"count","componentId":"ghost","count":0}]}`
	results := runTestHelper(t, jsonl)
	expectFail(t, results, 0, "not found")
}

// ===== assertNotExists edge cases =====

func TestRunnerNotExistsWrongSurface(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"test","surfaceId":"s999","name":"bad surface","steps":[{"assert":"notExists","componentId":"t1"}]}`
	results := runTestHelper(t, jsonl)
	expectFail(t, results, 0, "surface")
}

// ===== assertAction edge cases =====

func TestRunnerActionNameOnly(t *testing.T) {
	// Assert action by name only, no data check
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Go","onClick":{"action":{"event":{"name":"doIt"}}}}}]}
{"type":"test","surfaceId":"s1","name":"name only","steps":[{"simulate":"event","componentId":"btn","event":"click"},{"assert":"action","name":"doIt"}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
}

func TestRunnerActionWrongName(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Go","onClick":{"action":{"event":{"name":"doIt"}}}}}]}
{"type":"test","surfaceId":"s1","name":"wrong name","steps":[{"simulate":"event","componentId":"btn","event":"click"},{"assert":"action","name":"otherAction"}]}`
	results := runTestHelper(t, jsonl)
	expectFail(t, results, 0, "no action")
}

func TestRunnerActionDataMismatch(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/x","value":"actual"}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Go","onClick":{"action":{"event":{"name":"doIt","dataRefs":["/x"]}}}}}]}
{"type":"test","surfaceId":"s1","name":"wrong data","steps":[{"simulate":"event","componentId":"btn","event":"click"},{"assert":"action","name":"doIt","data":{"/x":"expected"}}]}`
	results := runTestHelper(t, jsonl)
	expectFail(t, results, 0, "assertAction")
}

func TestRunnerActionDataKeyMissing(t *testing.T) {
	// Action has no data, but test asserts on data key → should fail
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Go","onClick":{"action":{"event":{"name":"doIt"}}}}}]}
{"type":"test","surfaceId":"s1","name":"missing key","steps":[{"simulate":"event","componentId":"btn","event":"click"},{"assert":"action","name":"doIt","data":{"/x":"hello"}}]}`
	results := runTestHelper(t, jsonl)
	expectFail(t, results, 0, "not present")
}

func TestRunnerActionNoActionsFired(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"test","surfaceId":"s1","name":"no actions","steps":[{"assert":"action","name":"anything"}]}`
	results := runTestHelper(t, jsonl)
	expectFail(t, results, 0, "no action")
}

func TestRunnerActionExtraDataInActual(t *testing.T) {
	// Action has extra keys beyond what test asserts — should pass (subset)
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/x","value":"hello"},{"op":"add","path":"/y","value":"world"}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Go","onClick":{"action":{"event":{"name":"doIt","dataRefs":["/x","/y"]}}}}}]}
{"type":"test","surfaceId":"s1","name":"subset data","steps":[{"simulate":"event","componentId":"btn","event":"click"},{"assert":"action","name":"doIt","data":{"/x":"hello"}}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
}

// ===== assertLayout with values =====

func TestRunnerLayoutWithValues(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"box","type":"Column","children":["t1"]},{"componentId":"t1","type":"Text","props":{"content":"hi"}}]}
{"type":"test","surfaceId":"s1","name":"layout vals","steps":[{"assert":"layout","componentId":"box","layout":{"width":300,"height":200}}]}`
	results := runTestWithMock(t, jsonl, func(m *renderer.MockRenderer) {
		m.SetLayout("s1", "box", renderer.LayoutInfo{Width: 300, Height: 200})
	})
	expectPass(t, results, 0)
}

func TestRunnerLayoutMismatch(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"box","type":"Column"}]}
{"type":"test","surfaceId":"s1","name":"layout wrong","steps":[{"assert":"layout","componentId":"box","layout":{"width":500}}]}`
	results := runTestWithMock(t, jsonl, func(m *renderer.MockRenderer) {
		m.SetLayout("s1", "box", renderer.LayoutInfo{Width: 300})
	})
	expectFail(t, results, 0, "assertLayout")
}

func TestRunnerLayoutSubsetMatch(t *testing.T) {
	// Assert only width — other fields don't matter
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"box","type":"Column"}]}
{"type":"test","surfaceId":"s1","name":"subset layout","steps":[{"assert":"layout","componentId":"box","layout":{"width":300}}]}`
	results := runTestWithMock(t, jsonl, func(m *renderer.MockRenderer) {
		m.SetLayout("s1", "box", renderer.LayoutInfo{X: 10, Y: 20, Width: 300, Height: 200})
	})
	expectPass(t, results, 0)
}

// ===== assertStyle with values =====

func TestRunnerStyleWithValues(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"hi"}}]}
{"type":"test","surfaceId":"s1","name":"style vals","steps":[{"assert":"style","componentId":"t1","style":{"fontSize":24,"bold":true}}]}`
	results := runTestWithMock(t, jsonl, func(m *renderer.MockRenderer) {
		m.SetStyle("s1", "t1", renderer.StyleInfo{FontSize: 24, Bold: true})
	})
	expectPass(t, results, 0)
}

func TestRunnerStyleMismatch(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"hi"}}]}
{"type":"test","surfaceId":"s1","name":"style wrong","steps":[{"assert":"style","componentId":"t1","style":{"fontSize":24}}]}`
	results := runTestWithMock(t, jsonl, func(m *renderer.MockRenderer) {
		m.SetStyle("s1", "t1", renderer.StyleInfo{FontSize: 13})
	})
	expectFail(t, results, 0, "assertStyle")
}

func TestRunnerStyleSubsetMatch(t *testing.T) {
	// Assert only fontSize — other style fields don't matter
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"hi"}}]}
{"type":"test","surfaceId":"s1","name":"subset style","steps":[{"assert":"style","componentId":"t1","style":{"bold":true}}]}`
	results := runTestWithMock(t, jsonl, func(m *renderer.MockRenderer) {
		m.SetStyle("s1", "t1", renderer.StyleInfo{FontName: "Helvetica", FontSize: 24, Bold: true})
	})
	expectPass(t, results, 0)
}

func TestRunnerStyleFontName(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"hi"}}]}
{"type":"test","surfaceId":"s1","name":"font name","steps":[{"assert":"style","componentId":"t1","style":{"fontName":"Helvetica"}}]}`
	results := runTestWithMock(t, jsonl, func(m *renderer.MockRenderer) {
		m.SetStyle("s1", "t1", renderer.StyleInfo{FontName: "Helvetica", FontSize: 13})
	})
	expectPass(t, results, 0)
}

func TestRunnerStyleTextColor(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"hi"}}]}
{"type":"test","surfaceId":"s1","name":"text color","steps":[{"assert":"style","componentId":"t1","style":{"textColor":"#FF0000"}}]}`
	results := runTestWithMock(t, jsonl, func(m *renderer.MockRenderer) {
		m.SetStyle("s1", "t1", renderer.StyleInfo{TextColor: "#FF0000"})
	})
	expectPass(t, results, 0)
}

func TestRunnerStyleEmptyExpected(t *testing.T) {
	// Empty style map means "just check the component can be queried"
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"hi"}}]}
{"type":"test","surfaceId":"s1","name":"style empty","steps":[{"assert":"style","componentId":"t1","style":{}}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
}

// ===== Simulate edge cases =====

func TestRunnerSimulateToggle(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/on","value":false}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"cb","type":"CheckBox","props":{"label":"Toggle","checked":{"path":"/on"},"dataBinding":"/on"}}]}
{"type":"test","surfaceId":"s1","name":"toggle","steps":[{"simulate":"event","componentId":"cb","event":"toggle","eventData":"true"},{"assert":"dataModel","path":"/on","value":true}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
}

func TestRunnerSimulateSlide(t *testing.T) {
	// Slider callback converts eventData string to float64 via Sscanf
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/val","value":0}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"sl","type":"Slider","props":{"min":0,"max":100,"sliderValue":{"path":"/val"},"dataBinding":"/val"}}]}
{"type":"test","surfaceId":"s1","name":"slide","steps":[{"simulate":"event","componentId":"sl","event":"slide","eventData":"50"},{"assert":"dataModel","path":"/val","value":50}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
}

func TestRunnerSimulateNoCallback(t *testing.T) {
	// Simulate on a component that has no callback registered — silent no-op
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"hi"}}]}
{"type":"test","surfaceId":"s1","name":"no callback","steps":[{"simulate":"event","componentId":"t1","event":"click"}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
}

// ===== Multiple surfaces =====

func TestRunnerMultipleSurfaces(t *testing.T) {
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"Surface 1"}
{"type":"createSurface","surfaceId":"s2","title":"Surface 2"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/x","value":"one"}]}
{"type":"updateDataModel","surfaceId":"s2","ops":[{"op":"add","path":"/x","value":"two"}]}
{"type":"test","surfaceId":"s1","name":"s1 data","steps":[{"assert":"dataModel","path":"/x","value":"one"}]}
{"type":"test","surfaceId":"s2","name":"s2 data","steps":[{"assert":"dataModel","path":"/x","value":"two"}]}`
	results := runTestHelper(t, jsonl)
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	expectPass(t, results, 0)
	expectPass(t, results, 1)
}

// ===== Full integration flows =====

func TestRunnerFullDataBindingCycle(t *testing.T) {
	// TextField → DataModel → bound Text: complete cycle
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/name","value":""}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"field","type":"TextField","props":{"placeholder":"Name","value":{"path":"/name"},"dataBinding":"/name"}},{"componentId":"label","type":"Text","props":{"content":{"path":"/name"}}}]}
{"type":"test","surfaceId":"s1","name":"initial empty","steps":[{"assert":"dataModel","path":"/name","value":""},{"assert":"component","componentId":"label","props":{"content":""}},{"assert":"component","componentId":"field","componentType":"TextField"}]}
{"type":"test","surfaceId":"s1","name":"type name","steps":[{"simulate":"event","componentId":"field","event":"change","eventData":"Alice"},{"assert":"dataModel","path":"/name","value":"Alice"},{"assert":"component","componentId":"label","props":{"content":"Alice"}}]}
{"type":"test","surfaceId":"s1","name":"update name","steps":[{"simulate":"event","componentId":"field","event":"change","eventData":"Bob"},{"assert":"dataModel","path":"/name","value":"Bob"},{"assert":"component","componentId":"label","props":{"content":"Bob"}}]}`
	results := runTestHelper(t, jsonl)
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	for i, r := range results {
		expectPass(t, results, i)
		if !r.Passed {
			break
		}
	}
}

func TestRunnerButtonActionWithDataRefs(t *testing.T) {
	// Full flow: set data, click button, assert action carries resolved data
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/name","value":""},{"op":"add","path":"/email","value":""}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"nameField","type":"TextField","props":{"value":{"path":"/name"},"dataBinding":"/name"}},{"componentId":"emailField","type":"TextField","props":{"value":{"path":"/email"},"dataBinding":"/email"}},{"componentId":"btn","type":"Button","props":{"label":"Submit","onClick":{"action":{"event":{"name":"submit","dataRefs":["/name","/email"]}}}}}]}
{"type":"test","surfaceId":"s1","name":"fill and submit","steps":[{"simulate":"event","componentId":"nameField","event":"change","eventData":"Alice"},{"simulate":"event","componentId":"emailField","event":"change","eventData":"alice@test.com"},{"simulate":"event","componentId":"btn","event":"click"},{"assert":"action","name":"submit","data":{"/name":"Alice","/email":"alice@test.com"}}]}`
	results := runTestHelper(t, jsonl)
	expectPass(t, results, 0)
}

func TestRunnerMultipleTestsProgressiveState(t *testing.T) {
	// 3 tests building on shared state
	jsonl := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/count","value":0}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"field","type":"TextField","props":{"value":{"path":"/count"},"dataBinding":"/count"}},{"componentId":"display","type":"Text","props":{"content":{"path":"/count"}}}]}
{"type":"test","surfaceId":"s1","name":"initial","steps":[{"assert":"dataModel","path":"/count","value":0}]}
{"type":"test","surfaceId":"s1","name":"set to 10","steps":[{"simulate":"event","componentId":"field","event":"change","eventData":"10"},{"assert":"dataModel","path":"/count","value":"10"}]}
{"type":"test","surfaceId":"s1","name":"still 10","steps":[{"assert":"dataModel","path":"/count","value":"10"},{"assert":"component","componentId":"display","props":{"content":"10"}}]}`
	results := runTestHelper(t, jsonl)
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	for i := range results {
		expectPass(t, results, i)
	}
}
