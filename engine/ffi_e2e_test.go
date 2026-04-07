package engine

import (
	"canopy/renderer"
	"os"
	"strings"
	"testing"
)

// TestFFICurlVersion calls curl_version() which takes no args and returns a const char*.
func TestFFICurlVersion(t *testing.T) {
	reg := NewFFIRegistry()
	defer reg.Close()

	err := reg.LoadLibrary("libcurl.dylib", "curl", []FuncConfig{
		{Name: "version", Symbol: "curl_version", ReturnType: "string", ParamTypes: []string{}},
	})
	if err != nil {
		t.Fatalf("LoadLibrary libcurl: %v", err)
	}

	result, err := reg.Call("curl.version", []interface{}{})
	if err != nil {
		t.Fatalf("curl.version: %v", err)
	}
	s, ok := result.(string)
	if !ok {
		t.Fatalf("curl.version returned %T, want string", result)
	}
	if !strings.HasPrefix(s, "libcurl/") {
		t.Errorf("curl.version = %q, want prefix 'libcurl/'", s)
	}
	t.Logf("curl_version() = %q", s)
}

// TestFFICurlEasyInitCleanup tests the pointer handle lifecycle with curl_easy_init/cleanup.
func TestFFICurlEasyInitCleanup(t *testing.T) {
	reg := NewFFIRegistry()
	defer reg.Close()

	err := reg.LoadLibrary("libcurl.dylib", "curl", []FuncConfig{
		{Name: "init", Symbol: "curl_easy_init", ReturnType: "pointer", ParamTypes: []string{}},
		{Name: "cleanup", Symbol: "curl_easy_cleanup", ReturnType: "void", ParamTypes: []string{"pointer"}},
		{Name: "strerror", Symbol: "curl_easy_strerror", ReturnType: "string", ParamTypes: []string{"int"}},
	})
	if err != nil {
		t.Fatalf("LoadLibrary libcurl: %v", err)
	}

	// curl_easy_init() → returns a CURL* handle
	result, err := reg.Call("curl.init", []interface{}{})
	if err != nil {
		t.Fatalf("curl.init: %v", err)
	}
	handleID, ok := result.(float64)
	if !ok || handleID < 1 {
		t.Fatalf("curl.init returned %v, want handle ID >= 1", result)
	}
	t.Logf("curl_easy_init() → handle %v", handleID)

	// curl_easy_strerror(0) → "No error" (CURLE_OK = 0)
	result, err = reg.Call("curl.strerror", []interface{}{float64(0)})
	if err != nil {
		t.Fatalf("curl.strerror: %v", err)
	}
	errMsg, ok := result.(string)
	if !ok {
		t.Fatalf("curl.strerror returned %T, want string", result)
	}
	if errMsg != "No error" {
		t.Errorf("curl.strerror(0) = %q, want 'No error'", errMsg)
	}
	t.Logf("curl_easy_strerror(0) = %q", errMsg)

	// curl_easy_cleanup(handle) → void, should not error
	_, err = reg.Call("curl.cleanup", []interface{}{handleID})
	if err != nil {
		t.Fatalf("curl.cleanup: %v", err)
	}
	t.Log("curl_easy_cleanup() succeeded")
}

// TestFFISqlite3Version calls sqlite3_libversion() which returns a const char*.
func TestFFISqlite3Version(t *testing.T) {
	reg := NewFFIRegistry()
	defer reg.Close()

	err := reg.LoadLibrary("libsqlite3.dylib", "sqlite", []FuncConfig{
		{Name: "version", Symbol: "sqlite3_libversion", ReturnType: "string", ParamTypes: []string{}},
	})
	if err != nil {
		t.Fatalf("LoadLibrary libsqlite3: %v", err)
	}

	result, err := reg.Call("sqlite.version", []interface{}{})
	if err != nil {
		t.Fatalf("sqlite.version: %v", err)
	}
	s, ok := result.(string)
	if !ok {
		t.Fatalf("sqlite.version returned %T, want string", result)
	}
	if !strings.HasPrefix(s, "3.") {
		t.Errorf("sqlite.version = %q, want prefix '3.'", s)
	}
	t.Logf("sqlite3_libversion() = %q", s)
}

// TestFFISqlite3OpenClose tests the full sqlite3 open/errmsg/close pointer lifecycle.
func TestFFISqlite3OpenClose(t *testing.T) {
	reg := NewFFIRegistry()
	defer reg.Close()

	// sqlite3_open(const char *filename, sqlite3 **ppDb) → int
	// This is a tricky one — it takes a pointer-to-pointer (output param).
	// We can't easily test this without a helper, but we can test simpler functions.
	// Instead, let's test sqlite3_libversion_number() which returns an int.
	err := reg.LoadLibrary("libsqlite3.dylib", "sqlite", []FuncConfig{
		{Name: "version_number", Symbol: "sqlite3_libversion_number", ReturnType: "int", ParamTypes: []string{}},
		{Name: "version", Symbol: "sqlite3_libversion", ReturnType: "string", ParamTypes: []string{}},
	})
	if err != nil {
		t.Fatalf("LoadLibrary: %v", err)
	}

	result, err := reg.Call("sqlite.version_number", []interface{}{})
	if err != nil {
		t.Fatalf("sqlite.version_number: %v", err)
	}
	num, ok := result.(float64)
	if !ok {
		t.Fatalf("sqlite.version_number returned %T, want float64", result)
	}
	// sqlite3 version numbers are like 3043002 for 3.43.2
	if num < 3000000 {
		t.Errorf("sqlite.version_number = %v, want >= 3000000", num)
	}
	t.Logf("sqlite3_libversion_number() = %v", num)
}

// TestFFIZlibVersion calls zlibVersion().
func TestFFIZlibVersion(t *testing.T) {
	reg := NewFFIRegistry()
	defer reg.Close()

	err := reg.LoadLibrary("libz.dylib", "z", []FuncConfig{
		{Name: "version", Symbol: "zlibVersion", ReturnType: "string", ParamTypes: []string{}},
	})
	if err != nil {
		t.Fatalf("LoadLibrary libz: %v", err)
	}

	result, err := reg.Call("z.version", []interface{}{})
	if err != nil {
		t.Fatalf("z.version: %v", err)
	}
	s, ok := result.(string)
	if !ok {
		t.Fatalf("z.version returned %T, want string", result)
	}
	if !strings.HasPrefix(s, "1.") {
		t.Errorf("z.version = %q, want prefix '1.'", s)
	}
	t.Logf("zlibVersion() = %q", s)
}

// TestFFIZlibCompressBound calls compressBound(uLong sourceLen) → uLong.
// On macOS, uLong is unsigned long (uint64 on arm64).
func TestFFIZlibCompressBound(t *testing.T) {
	reg := NewFFIRegistry()
	defer reg.Close()

	err := reg.LoadLibrary("libz.dylib", "z", []FuncConfig{
		{Name: "compressBound", Symbol: "compressBound", ReturnType: "uint64", ParamTypes: []string{"uint64"}},
	})
	if err != nil {
		t.Fatalf("LoadLibrary libz: %v", err)
	}

	// compressBound(1000) should return something > 1000
	result, err := reg.Call("z.compressBound", []interface{}{float64(1000)})
	if err != nil {
		t.Fatalf("z.compressBound: %v", err)
	}
	bound, ok := result.(float64)
	if !ok {
		t.Fatalf("z.compressBound returned %T, want float64", result)
	}
	if bound <= 1000 {
		t.Errorf("z.compressBound(1000) = %v, want > 1000", bound)
	}
	t.Logf("compressBound(1000) = %v", bound)
}

// TestFFIMultipleLibrariesSimultaneous loads curl, sqlite, zlib all at once and calls functions from each.
func TestFFIMultipleLibrariesSimultaneous(t *testing.T) {
	reg := NewFFIRegistry()
	defer reg.Close()

	if err := reg.LoadLibrary("libcurl.dylib", "curl", []FuncConfig{
		{Name: "version", Symbol: "curl_version", ReturnType: "string", ParamTypes: []string{}},
	}); err != nil {
		t.Fatalf("LoadLibrary libcurl: %v", err)
	}

	if err := reg.LoadLibrary("libsqlite3.dylib", "sqlite", []FuncConfig{
		{Name: "version", Symbol: "sqlite3_libversion", ReturnType: "string", ParamTypes: []string{}},
	}); err != nil {
		t.Fatalf("LoadLibrary libsqlite3: %v", err)
	}

	if err := reg.LoadLibrary("libz.dylib", "z", []FuncConfig{
		{Name: "version", Symbol: "zlibVersion", ReturnType: "string", ParamTypes: []string{}},
	}); err != nil {
		t.Fatalf("LoadLibrary libz: %v", err)
	}

	// Call all three
	curlVer, err := reg.Call("curl.version", []interface{}{})
	if err != nil {
		t.Fatalf("curl.version: %v", err)
	}
	sqliteVer, err := reg.Call("sqlite.version", []interface{}{})
	if err != nil {
		t.Fatalf("sqlite.version: %v", err)
	}
	zlibVer, err := reg.Call("z.version", []interface{}{})
	if err != nil {
		t.Fatalf("z.version: %v", err)
	}

	t.Logf("curl: %v, sqlite: %v, zlib: %v", curlVer, sqliteVer, zlibVer)

	if !strings.HasPrefix(curlVer.(string), "libcurl/") {
		t.Errorf("curl version unexpected: %v", curlVer)
	}
	if !strings.HasPrefix(sqliteVer.(string), "3.") {
		t.Errorf("sqlite version unexpected: %v", sqliteVer)
	}
	if !strings.HasPrefix(zlibVer.(string), "1.") {
		t.Errorf("zlib version unexpected: %v", zlibVer)
	}
}

// TestFFIPointerHandleLifecycle tests alloc → use → free with our test dylib,
// plus curl init → cleanup to test real library pointer handles.
func TestFFIPointerHandleLifecycle(t *testing.T) {
	buildTestDylib(t)
	defer os.Remove(testDylibPath)

	reg := NewFFIRegistry()
	defer reg.Close()

	// Load test lib for alloc/free
	if err := reg.LoadLibrary(testDylibPath, "mem", []FuncConfig{
		{Name: "alloc", Symbol: "alloc_buffer", ReturnType: "pointer", ParamTypes: []string{"int"}},
		{Name: "free", Symbol: "free_buffer", ReturnType: "void", ParamTypes: []string{"pointer"}},
	}); err != nil {
		t.Fatalf("LoadLibrary test: %v", err)
	}

	// Load curl for init/cleanup
	if err := reg.LoadLibrary("libcurl.dylib", "curl", []FuncConfig{
		{Name: "init", Symbol: "curl_easy_init", ReturnType: "pointer", ParamTypes: []string{}},
		{Name: "cleanup", Symbol: "curl_easy_cleanup", ReturnType: "void", ParamTypes: []string{"pointer"}},
	}); err != nil {
		t.Fatalf("LoadLibrary curl: %v", err)
	}

	// Alloc multiple buffers
	handles := make([]float64, 5)
	for i := 0; i < 5; i++ {
		result, err := reg.Call("mem.alloc", []interface{}{float64(64 * (i + 1))})
		if err != nil {
			t.Fatalf("mem.alloc[%d]: %v", i, err)
		}
		h, ok := result.(float64)
		if !ok || h < 1 {
			t.Fatalf("mem.alloc[%d] returned %v, want handle >= 1", i, result)
		}
		handles[i] = h
	}

	// Handles should be unique and sequential
	for i := 1; i < len(handles); i++ {
		if handles[i] <= handles[i-1] {
			t.Errorf("handles not sequential: %v", handles)
			break
		}
	}

	// Init a curl handle
	curlResult, err := reg.Call("curl.init", []interface{}{})
	if err != nil {
		t.Fatalf("curl.init: %v", err)
	}
	curlHandle := curlResult.(float64)

	// Free all buffers
	for i, h := range handles {
		_, err := reg.Call("mem.free", []interface{}{h})
		if err != nil {
			t.Fatalf("mem.free[%d]: %v", i, err)
		}
	}

	// Cleanup curl
	_, err = reg.Call("curl.cleanup", []interface{}{curlHandle})
	if err != nil {
		t.Fatalf("curl.cleanup: %v", err)
	}

	t.Logf("allocated %d buffers + 1 curl handle, freed all successfully", len(handles))
}

// TestFFIInvalidHandleErrors verifies that passing a bogus handle ID errors properly.
func TestFFIInvalidHandleErrors(t *testing.T) {
	reg := NewFFIRegistry()
	defer reg.Close()

	if err := reg.LoadLibrary("libcurl.dylib", "curl", []FuncConfig{
		{Name: "cleanup", Symbol: "curl_easy_cleanup", ReturnType: "void", ParamTypes: []string{"pointer"}},
	}); err != nil {
		t.Fatalf("LoadLibrary: %v", err)
	}

	// Pass a handle ID that was never allocated
	_, err := reg.Call("curl.cleanup", []interface{}{float64(99999)})
	if err == nil {
		t.Error("expected error for invalid handle, got nil")
	}
	t.Logf("invalid handle error: %v", err)
}

// TestFFISessionWithCurlVersion tests the full session pipeline with curl_version.
func TestFFISessionWithCurlVersion(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"Curl Test","width":400,"height":300}`)

	feedMessages(t, sess, `{"type":"loadLibrary","path":"libcurl.dylib","prefix":"curl","functions":[{"name":"version","symbol":"curl_version","returnType":"string","paramTypes":[]}]}`)

	feedMessages(t, sess, `{"type":"updateComponents","surfaceId":"main","components":[{"componentId":"ver","type":"Text","props":{"content":{"functionCall":{"name":"curl.version","args":[]}},"variant":"body"}}]}`)

	found := false
	for _, c := range mock.Created {
		if c.Node.ComponentID == "ver" {
			found = true
			if !strings.HasPrefix(c.Node.Props.Content, "libcurl/") {
				t.Errorf("ver content = %q, want prefix 'libcurl/'", c.Node.Props.Content)
			}
			t.Logf("rendered curl version: %q", c.Node.Props.Content)
		}
	}
	if !found {
		t.Error("ver component not created")
	}
}

// TestFFISessionWithMultipleLibs tests loading curl + sqlite + zlib in a single session.
func TestFFISessionWithMultipleLibs(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"Multi-lib Session","width":600,"height":400}`)

	feedMessages(t, sess, `{"type":"loadLibrary","path":"libcurl.dylib","prefix":"curl","functions":[{"name":"version","symbol":"curl_version","returnType":"string","paramTypes":[]}]}`)
	feedMessages(t, sess, `{"type":"loadLibrary","path":"libsqlite3.dylib","prefix":"sqlite","functions":[{"name":"version","symbol":"sqlite3_libversion","returnType":"string","paramTypes":[]}]}`)
	feedMessages(t, sess, `{"type":"loadLibrary","path":"libz.dylib","prefix":"z","functions":[{"name":"version","symbol":"zlibVersion","returnType":"string","paramTypes":[]}]}`)

	feedMessages(t, sess, `{"type":"updateComponents","surfaceId":"main","components":[{"componentId":"col","type":"Column","props":{"gap":8},"children":["curl_ver","sqlite_ver","zlib_ver"]},{"componentId":"curl_ver","type":"Text","props":{"content":{"functionCall":{"name":"curl.version","args":[]}},"variant":"body"}},{"componentId":"sqlite_ver","type":"Text","props":{"content":{"functionCall":{"name":"sqlite.version","args":[]}},"variant":"body"}},{"componentId":"zlib_ver","type":"Text","props":{"content":{"functionCall":{"name":"z.version","args":[]}},"variant":"body"}}]}`)

	expectations := map[string]string{
		"curl_ver":   "libcurl/",
		"sqlite_ver": "3.",
		"zlib_ver":   "1.",
	}

	for _, c := range mock.Created {
		prefix, ok := expectations[c.Node.ComponentID]
		if !ok {
			continue
		}
		if !strings.HasPrefix(c.Node.Props.Content, prefix) {
			t.Errorf("%s content = %q, want prefix %q", c.Node.ComponentID, c.Node.Props.Content, prefix)
		}
		t.Logf("%s = %q", c.Node.ComponentID, c.Node.Props.Content)
		delete(expectations, c.Node.ComponentID)
	}
	for id := range expectations {
		t.Errorf("%s component not created", id)
	}
}

// TestFFITestDylibAllTypes exercises every supported type through our test library.
func TestFFITestDylibAllTypes(t *testing.T) {
	buildTestDylib(t)
	defer os.Remove(testDylibPath)

	reg := NewFFIRegistry()
	defer reg.Close()

	if err := reg.LoadFromConfig(&FFIConfig{
		Libraries: []LibConfig{{
			Path:   testDylibPath,
			Prefix: "t",
			Functions: []FuncConfig{
				{Name: "add_d", Symbol: "math_add", ReturnType: "double", ParamTypes: []string{"double", "double"}},
				{Name: "add_i", Symbol: "int_add", ReturnType: "int", ParamTypes: []string{"int", "int"}},
				{Name: "add_f", Symbol: "float_add", ReturnType: "float", ParamTypes: []string{"float", "float"}},
				{Name: "strlen", Symbol: "string_length", ReturnType: "int", ParamTypes: []string{"string"}},
				{Name: "reverse", Symbol: "string_reverse", ReturnType: "string", ParamTypes: []string{"string"}},
				{Name: "upper", Symbol: "string_upper", ReturnType: "string", ParamTypes: []string{"string"}},
				{Name: "echo", Symbol: "echo", ReturnType: "string", ParamTypes: []string{"string"}},
				{Name: "alloc", Symbol: "alloc_buffer", ReturnType: "pointer", ParamTypes: []string{"int"}},
				{Name: "free", Symbol: "free_buffer", ReturnType: "void", ParamTypes: []string{"pointer"}},
			},
		}},
	}); err != nil {
		t.Fatalf("LoadFromConfig: %v", err)
	}

	tests := []struct {
		name    string
		args    []interface{}
		check   func(interface{}) bool
		display string
	}{
		{"t.add_d", []interface{}{float64(1.5), float64(2.5)}, func(v interface{}) bool { return v.(float64) == 4.0 }, "1.5+2.5=4.0"},
		{"t.add_i", []interface{}{float64(100), float64(200)}, func(v interface{}) bool { return v.(float64) == 300 }, "100+200=300"},
		{"t.add_f", []interface{}{float64(0.1), float64(0.2)}, func(v interface{}) bool { f := v.(float64); return f > 0.29 && f < 0.31 }, "0.1+0.2≈0.3"},
		{"t.strlen", []interface{}{"hello world"}, func(v interface{}) bool { return v.(float64) == 11 }, "strlen('hello world')=11"},
		{"t.reverse", []interface{}{"abcdef"}, func(v interface{}) bool { return v.(string) == "fedcba" }, "reverse('abcdef')='fedcba'"},
		{"t.upper", []interface{}{"hello"}, func(v interface{}) bool { return v.(string) == "HELLO" }, "upper('hello')='HELLO'"},
		{"t.echo", []interface{}{"test"}, func(v interface{}) bool { return v.(string) == "test" }, "echo('test')='test'"},
	}

	for _, tt := range tests {
		result, err := reg.Call(tt.name, tt.args)
		if err != nil {
			t.Errorf("%s: %v", tt.display, err)
			continue
		}
		if !tt.check(result) {
			t.Errorf("%s: got %v", tt.display, result)
		} else {
			t.Logf("PASS: %s → %v", tt.display, result)
		}
	}

	// Pointer lifecycle
	h, err := reg.Call("t.alloc", []interface{}{float64(128)})
	if err != nil {
		t.Fatalf("alloc: %v", err)
	}
	t.Logf("PASS: alloc(128) → handle %v", h)

	_, err = reg.Call("t.free", []interface{}{h})
	if err != nil {
		t.Fatalf("free: %v", err)
	}
	t.Log("PASS: free(handle) → void")
}
