package engine

import (
	"canopy/protocol"
	"canopy/renderer"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRecorderCapturesUIMessages verifies that the recorder captures
// createSurface and updateComponents (UI-definition messages) when
// messages flow through Session.HandleMessage.
func TestRecorderCapturesUIMessages(t *testing.T) {
	dir := t.TempDir()
	recFile := filepath.Join(dir, "out.jsonl")
	f, err := os.Create(recFile)
	if err != nil {
		t.Fatal(err)
	}

	rec := NewRecorder(f)
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)
	sess.SetRecorder(rec)

	input := `{"type":"createSurface","surfaceId":"s1","title":"Test","width":400,"height":300}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"Hello"}}]}`

	feedMessages(t, sess, input)
	rec.Close()

	// Read recorded output
	data, err := os.ReadFile(recFile)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("recorded %d lines, want 2", len(lines))
	}

	// Verify recorded lines are valid JSONL that can be replayed
	p := protocol.NewParser(strings.NewReader(string(data)))
	msg1, err := p.Next()
	if err != nil {
		t.Fatalf("parse line 1: %v", err)
	}
	if msg1.Type != protocol.MsgCreateSurface {
		t.Errorf("line 1 type = %s, want createSurface", msg1.Type)
	}
	msg2, err := p.Next()
	if err != nil {
		t.Fatalf("parse line 2: %v", err)
	}
	if msg2.Type != protocol.MsgUpdateComponents {
		t.Errorf("line 2 type = %s, want updateComponents", msg2.Type)
	}
}

// TestRecorderSkipsNonUIMessages verifies that test messages and other
// runtime-only messages are NOT recorded.
func TestRecorderSkipsNonUIMessages(t *testing.T) {
	dir := t.TempDir()
	recFile := filepath.Join(dir, "out.jsonl")
	f, err := os.Create(recFile)
	if err != nil {
		t.Fatal(err)
	}

	rec := NewRecorder(f)
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)
	sess.SetRecorder(rec)

	// Feed a mix: one UI message, one test message
	input := `{"type":"createSurface","surfaceId":"s1","title":"Test","width":400,"height":300}
{"type":"test","surfaceId":"s1","assert":"component","componentId":"t1","expect":{"props":{"content":"x"}}}`

	feedMessages(t, sess, input)
	rec.Close()

	data, err := os.ReadFile(recFile)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("recorded %d lines, want 1 (test message should be skipped)", len(lines))
	}
}

// TestRecordThenReplay is the full roundtrip: feed messages through a session
// with recorder active, then replay the recorded file through a new session
// and verify the same components are created.
func TestRecordThenReplay(t *testing.T) {
	dir := t.TempDir()
	recFile := filepath.Join(dir, "out.jsonl")
	f, err := os.Create(recFile)
	if err != nil {
		t.Fatal(err)
	}

	rec := NewRecorder(f)
	mock1 := renderer.NewMockRenderer()
	disp1 := &renderer.MockDispatcher{}
	sess1 := NewSession(mock1, disp1)
	sess1.SetRecorder(rec)

	input := `{"type":"createSurface","surfaceId":"s1","title":"Test","width":400,"height":300}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"card1","type":"Card","props":{"title":"Welcome"},"children":["t1"]},{"componentId":"t1","type":"Text","props":{"content":"Hello"}}]}`

	feedMessages(t, sess1, input)
	rec.Close()

	// Replay recorded file into a fresh session
	data, err := os.ReadFile(recFile)
	if err != nil {
		t.Fatal(err)
	}

	mock2 := renderer.NewMockRenderer()
	disp2 := &renderer.MockDispatcher{}
	sess2 := NewSession(mock2, disp2)
	feedMessages(t, sess2, string(data))

	// Both sessions should have created the same components
	if len(mock1.Windows) != len(mock2.Windows) {
		t.Errorf("windows: session1=%d, session2=%d", len(mock1.Windows), len(mock2.Windows))
	}
	if len(mock1.Created) != len(mock2.Created) {
		t.Errorf("created: session1=%d, session2=%d", len(mock1.Created), len(mock2.Created))
	}

	// Verify specific component exists in replay
	found := false
	for _, c := range mock2.Created {
		if c.Node.ComponentID == "t1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("component t1 not found in replayed session")
	}
}

// TestCacheRoundTripWithRecorder exercises the full cache lifecycle:
// record → write cache hash → verify valid → replay from cache.
func TestCacheRoundTripWithRecorder(t *testing.T) {
	dir := t.TempDir()
	promptFile := filepath.Join(dir, "prompt.txt")
	os.WriteFile(promptFile, []byte("Build a test UI"), 0644)

	jsonlPath, hashPath, tmpPath := CachePathsForFile(promptFile)
	prompt := "Build a test UI"
	componentRef := "test-ref-v1"

	// 1. No cache yet
	if CacheValid(jsonlPath, hashPath, prompt, componentRef) {
		t.Fatal("expected cache invalid before recording")
	}

	// 2. Record through session
	f, err := os.Create(tmpPath)
	if err != nil {
		t.Fatal(err)
	}
	rec := NewRecorder(f)
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)
	sess.SetRecorder(rec)

	input := `{"type":"createSurface","surfaceId":"s1","title":"Test","width":400,"height":300}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"Hello"}}]}`
	feedMessages(t, sess, input)

	// 3. Finalize cache (close recorder, rename tmp → jsonl, write hash)
	rec.Close()
	if err := os.Rename(tmpPath, jsonlPath); err != nil {
		t.Fatal(err)
	}
	if err := WriteCacheHash(hashPath, prompt, componentRef); err != nil {
		t.Fatal(err)
	}

	// 4. Cache should be valid
	if !CacheValid(jsonlPath, hashPath, prompt, componentRef) {
		t.Fatal("expected cache valid after recording + hash write")
	}

	// 5. Different prompt → invalid
	if CacheValid(jsonlPath, hashPath, "Build something else", componentRef) {
		t.Error("expected cache invalid with different prompt")
	}

	// 6. Different componentRef → invalid
	if CacheValid(jsonlPath, hashPath, prompt, "test-ref-v2") {
		t.Error("expected cache invalid with different componentRef")
	}

	// 7. Replay from cache
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatal(err)
	}
	mock2 := renderer.NewMockRenderer()
	disp2 := &renderer.MockDispatcher{}
	sess2 := NewSession(mock2, disp2)
	feedMessages(t, sess2, string(data))

	if len(mock2.Created) == 0 {
		t.Error("replay created no components")
	}
}

// TestLibrarySaveAndLoad verifies that defineComponent messages are
// persisted to the library directory when recorder is active, and that
// a new session can load them.
func TestLibrarySaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	// Create library backed by temp dir
	lib := &Library{
		dir:  dir,
		defs: make(map[string]*protocol.DefineComponent),
	}

	recFile := filepath.Join(t.TempDir(), "out.jsonl")
	f, err := os.Create(recFile)
	if err != nil {
		t.Fatal(err)
	}
	rec := NewRecorder(f)

	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)
	sess.SetRecorder(rec)
	sess.SetLibrary(lib)

	// Feed a defineComponent message
	input := `{"type":"defineComponent","name":"MyButton","params":["label","color"],"components":[{"componentId":"_root","type":"Button","props":{"label":{"param":"label"}}}]}`
	feedMessages(t, sess, input)
	rec.Close()

	// Verify component was saved to library dir
	libFile := filepath.Join(dir, "MyButton.jsonl")
	if _, err := os.Stat(libFile); os.IsNotExist(err) {
		t.Fatal("MyButton.jsonl not saved to library dir")
	}

	// Verify library has the def
	if _, ok := lib.Defs()["MyButton"]; !ok {
		t.Fatal("MyButton not in library defs after save")
	}

	// Create a new library from same dir and load
	lib2 := &Library{
		dir:  dir,
		defs: make(map[string]*protocol.DefineComponent),
	}
	lib2.Load()

	if _, ok := lib2.Defs()["MyButton"]; !ok {
		t.Fatal("MyButton not found after library reload")
	}
	if len(lib2.Defs()["MyButton"].Params) != 2 {
		t.Errorf("params count = %d, want 2", len(lib2.Defs()["MyButton"].Params))
	}

	// New session with loaded library should have the compDef
	mock2 := renderer.NewMockRenderer()
	disp2 := &renderer.MockDispatcher{}
	sess2 := NewSession(mock2, disp2)
	sess2.SetLibrary(lib2)

	// Verify the compDef is available (use useComponent to exercise it)
	input2 := `{"type":"createSurface","surfaceId":"s1","title":"Lib Test","width":400,"height":300}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"u1","useComponent":"MyButton","args":{"label":"Click me","color":"blue"}}]}`
	feedMessages(t, sess2, input2)

	// The useComponent should have expanded — check that the template's
	// button component was created (with scoped ID)
	found := false
	for _, c := range mock2.Created {
		if c.Node.Type == "Button" {
			found = true
			break
		}
	}
	if !found {
		t.Error("useComponent did not expand MyButton into a Button in the new session")
	}
}

// TestLibraryNotSavedWithoutRecorder verifies that defineComponent
// messages are NOT persisted to library when there is no recorder
// (i.e., during cache replay, not generation).
func TestLibraryNotSavedWithoutRecorder(t *testing.T) {
	dir := t.TempDir()
	lib := &Library{
		dir:  dir,
		defs: make(map[string]*protocol.DefineComponent),
	}

	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)
	// No recorder set — simulates cache replay
	sess.SetLibrary(lib)

	input := `{"type":"defineComponent","name":"ReplayComp","params":["x"],"template":[{"componentId":"t","type":"Text","props":{"content":"hi"}}]}`
	feedMessages(t, sess, input)

	// Component should be in session compDefs (for current session use)
	if _, ok := sess.compDefs["ReplayComp"]; !ok {
		t.Error("component not in session compDefs")
	}

	// But NOT saved to library dir (no recorder = replay mode)
	libFile := filepath.Join(dir, "ReplayComp.jsonl")
	if _, err := os.Stat(libFile); !os.IsNotExist(err) {
		t.Error("component should NOT be saved to library during replay (no recorder)")
	}
}

// TestCachePathsForPromptConsistency verifies that the same prompt+ref
// always produces the same cache paths, and different prompts produce
// different paths.
func TestCachePathsForPromptConsistency(t *testing.T) {
	j1, h1, _ := CachePathsForPrompt("Build a counter", "ref-v1")
	j2, h2, _ := CachePathsForPrompt("Build a counter", "ref-v1")
	j3, _, _ := CachePathsForPrompt("Build a calculator", "ref-v1")
	j4, _, _ := CachePathsForPrompt("Build a counter", "ref-v2")

	if j1 != j2 || h1 != h2 {
		t.Error("same inputs should produce same paths")
	}
	if j1 == j3 {
		t.Error("different prompt should produce different paths")
	}
	if j1 == j4 {
		t.Error("different ref should produce different paths")
	}
}

// TestComponentListForPrompt verifies library prompt text generation.
func TestComponentListForPrompt(t *testing.T) {
	lib := &Library{
		dir:  t.TempDir(),
		defs: make(map[string]*protocol.DefineComponent),
	}

	// Empty library → empty string
	if lib.ComponentListForPrompt() != "" {
		t.Error("empty library should produce empty string")
	}

	// Add a component
	lib.defs["StatusBadge"] = &protocol.DefineComponent{
		Name:   "StatusBadge",
		Params: []string{"status", "color"},
	}

	prompt := lib.ComponentListForPrompt()
	if !strings.Contains(prompt, "StatusBadge") {
		t.Error("prompt should contain component name")
	}
	if !strings.Contains(prompt, "status, color") {
		t.Error("prompt should contain params")
	}
	if !strings.Contains(prompt, "COMPONENT LIBRARY") {
		t.Error("prompt should contain header")
	}
}
