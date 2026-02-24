package engine

import (
	"jview/renderer"
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
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Go","onClick":{"action":{"type":"serverAction","name":"doIt","dataRefs":["/x"]}}}}]}
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
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Go","onClick":{"action":{"type":"serverAction","name":"doIt"}}}}]}
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
