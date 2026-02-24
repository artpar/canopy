package engine

import (
	"jview/renderer"
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

	// test.add
	result, err := reg.Call("test.add", []interface{}{float64(3), float64(4)})
	if err != nil {
		t.Fatalf("test.add: %v", err)
	}
	if f, ok := result.(float64); !ok || f != 7 {
		t.Errorf("test.add(3,4) = %v, want 7", result)
	}

	// test.reverse
	result, err = reg.Call("test.reverse", []interface{}{"hello"})
	if err != nil {
		t.Fatalf("test.reverse: %v", err)
	}
	if s, ok := result.(string); !ok || s != "olleh" {
		t.Errorf("test.reverse(\"hello\") = %v, want \"olleh\"", result)
	}

	// test.echo
	result, err = reg.Call("test.echo", []interface{}{float64(1), "two", float64(3)})
	if err != nil {
		t.Fatalf("test.echo: %v", err)
	}
	arr, ok := result.([]interface{})
	if !ok || len(arr) != 3 {
		t.Errorf("test.echo = %v, want [1,\"two\",3]", result)
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
		{Name: "bad", Symbol: "nonexistent_symbol"},
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

	// Find the created text component and check its resolved content
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
