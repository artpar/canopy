package engine

import (
	"canopy/renderer"
	"math"
	"os"
	"os/exec"
	"testing"
)

const testDylibPath = "/tmp/jview_test_ffi_lib.dylib"
const testFFIConfigPath = "../testdata/ffi_lib.json"

func buildTestDylib(t *testing.T) {
	t.Helper()
	cmd := exec.Command("cc", "-shared", "-o", testDylibPath, "../testdata/ffi_lib.c")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build test dylib: %v\n%s", err, out)
	}
}

func TestFFIRegistryLoadAndCall(t *testing.T) {
	buildTestDylib(t)
	defer os.Remove(testDylibPath)

	cfg, err := LoadFFIConfig(testFFIConfigPath)
	if err != nil {
		t.Fatalf("LoadFFIConfig: %v", err)
	}

	reg := NewFFIRegistry()
	defer reg.Close()

	if err := reg.LoadFromConfig(cfg); err != nil {
		t.Fatalf("LoadFromConfig: %v", err)
	}

	// test.add: double math_add(double, double)
	result, err := reg.Call("test.add", []interface{}{float64(3), float64(4)})
	if err != nil {
		t.Fatalf("test.add: %v", err)
	}
	if f, ok := result.(float64); !ok || f != 7 {
		t.Errorf("test.add(3,4) = %v, want 7", result)
	}

	// test.reverse: const char* string_reverse(const char*)
	result, err = reg.Call("test.reverse", []interface{}{"hello"})
	if err != nil {
		t.Fatalf("test.reverse: %v", err)
	}
	if s, ok := result.(string); !ok || s != "olleh" {
		t.Errorf("test.reverse(\"hello\") = %v, want \"olleh\"", result)
	}

	// test.strlen: int string_length(const char*)
	result, err = reg.Call("test.strlen", []interface{}{"hello"})
	if err != nil {
		t.Fatalf("test.strlen: %v", err)
	}
	if f, ok := result.(float64); !ok || f != 5 {
		t.Errorf("test.strlen(\"hello\") = %v, want 5", result)
	}

	// test.upper: const char* string_upper(const char*)
	result, err = reg.Call("test.upper", []interface{}{"hello"})
	if err != nil {
		t.Fatalf("test.upper: %v", err)
	}
	if s, ok := result.(string); !ok || s != "HELLO" {
		t.Errorf("test.upper(\"hello\") = %v, want \"HELLO\"", result)
	}

	// test.echo: const char* echo(const char*)
	result, err = reg.Call("test.echo", []interface{}{"test string"})
	if err != nil {
		t.Fatalf("test.echo: %v", err)
	}
	if s, ok := result.(string); !ok || s != "test string" {
		t.Errorf("test.echo(\"test string\") = %v, want \"test string\"", result)
	}

	// test.intadd: int int_add(int, int)
	result, err = reg.Call("test.intadd", []interface{}{float64(10), float64(20)})
	if err != nil {
		t.Fatalf("test.intadd: %v", err)
	}
	if f, ok := result.(float64); !ok || f != 30 {
		t.Errorf("test.intadd(10,20) = %v, want 30", result)
	}

	// test.floatadd: float float_add(float, float)
	result, err = reg.Call("test.floatadd", []interface{}{float64(1.5), float64(2.5)})
	if err != nil {
		t.Fatalf("test.floatadd: %v", err)
	}
	if f, ok := result.(float64); !ok || math.Abs(f-4.0) > 0.001 {
		t.Errorf("test.floatadd(1.5,2.5) = %v, want 4.0", result)
	}
}

func TestFFIHandleTable(t *testing.T) {
	buildTestDylib(t)
	defer os.Remove(testDylibPath)

	cfg, err := LoadFFIConfig(testFFIConfigPath)
	if err != nil {
		t.Fatalf("LoadFFIConfig: %v", err)
	}

	reg := NewFFIRegistry()
	defer reg.Close()

	if err := reg.LoadFromConfig(cfg); err != nil {
		t.Fatalf("LoadFromConfig: %v", err)
	}

	// test.alloc: void* alloc_buffer(int) → returns a handle ID
	result, err := reg.Call("test.alloc", []interface{}{float64(64)})
	if err != nil {
		t.Fatalf("test.alloc: %v", err)
	}
	handleID, ok := result.(float64)
	if !ok || handleID < 1 {
		t.Fatalf("test.alloc(64) = %v, want handle ID >= 1", result)
	}

	// test.free: void free_buffer(void*) — pass handle back
	_, err = reg.Call("test.free", []interface{}{handleID})
	if err != nil {
		t.Fatalf("test.free: %v", err)
	}
}

func TestFFIRegistryUnknownFunc(t *testing.T) {
	reg := NewFFIRegistry()
	defer reg.Close()

	_, err := reg.Call("nonexistent.func", nil)
	if err == nil {
		t.Error("expected error for unknown function")
	}
}

func TestFFIRegistryBadPath(t *testing.T) {
	reg := NewFFIRegistry()
	defer reg.Close()

	err := reg.LoadLibrary("/nonexistent/path.dylib", "bad", nil)
	if err == nil {
		t.Error("expected error for bad library path")
	}
}

func TestFFIRegistryBadSymbol(t *testing.T) {
	buildTestDylib(t)
	defer os.Remove(testDylibPath)

	reg := NewFFIRegistry()
	defer reg.Close()

	err := reg.LoadLibrary(testDylibPath, "test", []FuncConfig{
		{Name: "bad", Symbol: "nonexistent_symbol", ReturnType: "void"},
	})
	if err == nil {
		t.Error("expected error for bad symbol")
	}
}

func TestFFIRegistryHas(t *testing.T) {
	buildTestDylib(t)
	defer os.Remove(testDylibPath)

	cfg, err := LoadFFIConfig(testFFIConfigPath)
	if err != nil {
		t.Fatalf("LoadFFIConfig: %v", err)
	}

	reg := NewFFIRegistry()
	defer reg.Close()

	if err := reg.LoadFromConfig(cfg); err != nil {
		t.Fatalf("LoadFromConfig: %v", err)
	}

	if !reg.Has("test.add") {
		t.Error("Has(test.add) = false, want true")
	}
	if reg.Has("test.nonexistent") {
		t.Error("Has(test.nonexistent) = true, want false")
	}
}

func TestFFIRegistryArgCountMismatch(t *testing.T) {
	buildTestDylib(t)
	defer os.Remove(testDylibPath)

	reg := NewFFIRegistry()
	defer reg.Close()

	if err := reg.LoadLibrary(testDylibPath, "test", []FuncConfig{
		{Name: "add", Symbol: "math_add", ReturnType: "double", ParamTypes: []string{"double", "double"}},
	}); err != nil {
		t.Fatalf("LoadLibrary: %v", err)
	}

	// Too few args
	_, err := reg.Call("test.add", []interface{}{float64(1)})
	if err == nil {
		t.Error("expected error for wrong arg count")
	}

	// Too many args
	_, err = reg.Call("test.add", []interface{}{float64(1), float64(2), float64(3)})
	if err == nil {
		t.Error("expected error for wrong arg count")
	}
}

func TestFFIRegistryBadArgType(t *testing.T) {
	buildTestDylib(t)
	defer os.Remove(testDylibPath)

	reg := NewFFIRegistry()
	defer reg.Close()

	if err := reg.LoadLibrary(testDylibPath, "test", []FuncConfig{
		{Name: "add", Symbol: "math_add", ReturnType: "double", ParamTypes: []string{"double", "double"}},
	}); err != nil {
		t.Fatalf("LoadLibrary: %v", err)
	}

	// String where double expected
	_, err := reg.Call("test.add", []interface{}{"not a number", float64(2)})
	if err == nil {
		t.Error("expected error for wrong arg type")
	}
}

func TestEvaluatorFFIFallthrough(t *testing.T) {
	buildTestDylib(t)
	defer os.Remove(testDylibPath)

	cfg, err := LoadFFIConfig(testFFIConfigPath)
	if err != nil {
		t.Fatalf("LoadFFIConfig: %v", err)
	}

	reg := NewFFIRegistry()
	defer reg.Close()

	if err := reg.LoadFromConfig(cfg); err != nil {
		t.Fatalf("LoadFromConfig: %v", err)
	}

	dm := NewDataModel()
	eval := NewEvaluator(dm)
	eval.FFI = reg

	// Built-in function still works
	result, err := eval.Eval("add", []interface{}{float64(1), float64(2)})
	if err != nil {
		t.Fatalf("built-in add: %v", err)
	}
	if f, ok := result.(float64); !ok || f != 3 {
		t.Errorf("built-in add(1,2) = %v, want 3", result)
	}

	// FFI function works through evaluator
	result, err = eval.Eval("test.add", []interface{}{float64(10), float64(20)})
	if err != nil {
		t.Fatalf("ffi test.add: %v", err)
	}
	if f, ok := result.(float64); !ok || f != 30 {
		t.Errorf("ffi test.add(10,20) = %v, want 30", result)
	}

	// Unknown function still errors
	_, err = eval.Eval("totally.unknown", nil)
	if err == nil {
		t.Error("expected error for totally unknown function")
	}
}

func TestEvaluatorFFIWithPathArgs(t *testing.T) {
	buildTestDylib(t)
	defer os.Remove(testDylibPath)

	cfg, err := LoadFFIConfig(testFFIConfigPath)
	if err != nil {
		t.Fatalf("LoadFFIConfig: %v", err)
	}

	reg := NewFFIRegistry()
	defer reg.Close()

	if err := reg.LoadFromConfig(cfg); err != nil {
		t.Fatalf("LoadFromConfig: %v", err)
	}

	dm := NewDataModel()
	dm.Set("/a", float64(5))
	dm.Set("/b", float64(3))

	eval := NewEvaluator(dm)
	eval.FFI = reg

	// FFI call with data model path refs as args
	result, err := eval.Eval("test.add", []interface{}{
		map[string]interface{}{"path": "/a"},
		map[string]interface{}{"path": "/b"},
	})
	if err != nil {
		t.Fatalf("ffi test.add with paths: %v", err)
	}
	if f, ok := result.(float64); !ok || f != 8 {
		t.Errorf("ffi test.add(/a, /b) = %v, want 8", result)
	}
}

func TestSessionRuntimeLoadLibrary(t *testing.T) {
	buildTestDylib(t)
	defer os.Remove(testDylibPath)

	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"Runtime FFI","width":400,"height":300}`)

	// Load with typed function declarations
	feedMessages(t, sess, `{"type":"loadLibrary","path":"`+testDylibPath+`","prefix":"rt","functions":[{"name":"add","symbol":"math_add","returnType":"double","paramTypes":["double","double"]},{"name":"reverse","symbol":"string_reverse","returnType":"string","paramTypes":["string"]}]}`)

	feedMessages(t, sess, `{"type":"updateComponents","surfaceId":"main","components":[{"componentId":"result","type":"Text","props":{"content":{"functionCall":{"name":"rt.add","args":[10,20]}},"variant":"body"}},{"componentId":"rev","type":"Text","props":{"content":{"functionCall":{"name":"rt.reverse","args":["world"]}},"variant":"body"}}]}`)

	foundAdd := false
	foundRev := false
	for _, c := range mock.Created {
		if c.Node.ComponentID == "result" {
			foundAdd = true
			if c.Node.Props.Content != "30" {
				t.Errorf("result content = %q, want \"30\"", c.Node.Props.Content)
			}
		}
		if c.Node.ComponentID == "rev" {
			foundRev = true
			if c.Node.Props.Content != "dlrow" {
				t.Errorf("rev content = %q, want \"dlrow\"", c.Node.Props.Content)
			}
		}
	}
	if !foundAdd {
		t.Error("result component not created")
	}
	if !foundRev {
		t.Error("rev component not created")
	}
}

func TestSessionRuntimeLoadLibraryPropagates(t *testing.T) {
	buildTestDylib(t)
	defer os.Remove(testDylibPath)

	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"Surface 1","width":400,"height":300}
{"type":"createSurface","surfaceId":"s2","title":"Surface 2","width":400,"height":300}`)

	feedMessages(t, sess, `{"type":"loadLibrary","path":"`+testDylibPath+`","prefix":"rt","functions":[{"name":"add","symbol":"math_add","returnType":"double","paramTypes":["double","double"]}]}`)

	feedMessages(t, sess, `{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"r1","type":"Text","props":{"content":{"functionCall":{"name":"rt.add","args":[1,2]}},"variant":"body"}}]}
{"type":"updateComponents","surfaceId":"s2","components":[{"componentId":"r2","type":"Text","props":{"content":{"functionCall":{"name":"rt.add","args":[100,200]}},"variant":"body"}}]}`)

	foundS1 := false
	foundS2 := false
	for _, c := range mock.Created {
		if c.Node.ComponentID == "r1" {
			foundS1 = true
			if c.Node.Props.Content != "3" {
				t.Errorf("s1 result content = %q, want \"3\"", c.Node.Props.Content)
			}
		}
		if c.Node.ComponentID == "r2" {
			foundS2 = true
			if c.Node.Props.Content != "300" {
				t.Errorf("s2 result content = %q, want \"300\"", c.Node.Props.Content)
			}
		}
	}
	if !foundS1 {
		t.Error("s1 result component not created")
	}
	if !foundS2 {
		t.Error("s2 result component not created")
	}
}

func TestSessionRuntimeLoadMultipleLibraries(t *testing.T) {
	buildTestDylib(t)
	defer os.Remove(testDylibPath)

	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"Multi-lib","width":400,"height":300}`)

	feedMessages(t, sess, `{"type":"loadLibrary","path":"`+testDylibPath+`","prefix":"math","functions":[{"name":"add","symbol":"math_add","returnType":"double","paramTypes":["double","double"]}]}`)

	feedMessages(t, sess, `{"type":"loadLibrary","path":"`+testDylibPath+`","prefix":"str","functions":[{"name":"reverse","symbol":"string_reverse","returnType":"string","paramTypes":["string"]}]}`)

	feedMessages(t, sess, `{"type":"updateComponents","surfaceId":"main","components":[{"componentId":"sum","type":"Text","props":{"content":{"functionCall":{"name":"math.add","args":[5,5]}},"variant":"body"}},{"componentId":"rev","type":"Text","props":{"content":{"functionCall":{"name":"str.reverse","args":["abcde"]}},"variant":"body"}}]}`)

	foundSum := false
	foundRev := false
	for _, c := range mock.Created {
		if c.Node.ComponentID == "sum" {
			foundSum = true
			if c.Node.Props.Content != "10" {
				t.Errorf("sum content = %q, want \"10\"", c.Node.Props.Content)
			}
		}
		if c.Node.ComponentID == "rev" {
			foundRev = true
			if c.Node.Props.Content != "edcba" {
				t.Errorf("rev content = %q, want \"edcba\"", c.Node.Props.Content)
			}
		}
	}
	if !foundSum {
		t.Error("sum component not created")
	}
	if !foundRev {
		t.Error("rev component not created")
	}
}

func TestSessionRuntimeLoadBadPath(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"Bad","width":400,"height":300}`)

	feedMessages(t, sess, `{"type":"loadLibrary","path":"/nonexistent/lib.dylib","prefix":"bad","functions":[{"name":"x","symbol":"y","returnType":"void"}]}`)

	feedMessages(t, sess, `{"type":"updateComponents","surfaceId":"main","components":[{"componentId":"t1","type":"Text","props":{"content":"still works","variant":"body"}}]}`)

	found := false
	for _, c := range mock.Created {
		if c.Node.ComponentID == "t1" {
			found = true
			if c.Node.Props.Content != "still works" {
				t.Errorf("content = %q, want \"still works\"", c.Node.Props.Content)
			}
		}
	}
	if !found {
		t.Error("t1 not created after bad loadLibrary")
	}
}

func TestFFIIntegrationWithSession(t *testing.T) {
	buildTestDylib(t)
	defer os.Remove(testDylibPath)

	cfg, err := LoadFFIConfig(testFFIConfigPath)
	if err != nil {
		t.Fatalf("LoadFFIConfig: %v", err)
	}

	reg := NewFFIRegistry()
	defer reg.Close()

	if err := reg.LoadFromConfig(cfg); err != nil {
		t.Fatalf("LoadFromConfig: %v", err)
	}

	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)
	sess.SetFFI(reg)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"FFI","width":400,"height":300}
{"type":"updateComponents","surfaceId":"main","components":[{"componentId":"result","type":"Text","props":{"content":{"functionCall":{"name":"test.add","args":[3,4]}},"variant":"body"}}]}`)

	found := false
	for _, c := range mock.Created {
		if c.Node.ComponentID == "result" {
			found = true
			if c.Node.Props.Content != "7" {
				t.Errorf("result content = %q, want \"7\"", c.Node.Props.Content)
			}
		}
	}
	if !found {
		t.Error("result component not created")
	}
}
