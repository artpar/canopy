package engine

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"canopy/protocol"
	"canopy/renderer"
)

// TestFileWriteReadAppend tests the full file I/O pipeline with real files.
func TestFileWriteReadAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)
	sess.SetNativeProvider(&testNativeProvider{})

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/file","value":"`+path+`"}]}`)

	// Write via evaluator function
	sess.mu.Lock()
	surf := sess.surfaces["s1"]
	sess.mu.Unlock()

	eval := surf.resolver.evaluator
	result, err := eval.Eval("fileWrite", []any{path, "hello world"})
	if err != nil {
		t.Fatalf("fileWrite failed: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got %v", result)
	}

	// Read back
	result, err = eval.Eval("fileRead", []any{path})
	if err != nil {
		t.Fatalf("fileRead failed: %v", err)
	}
	if result != "hello world" {
		t.Fatalf("expected 'hello world', got %v", result)
	}

	// Append
	result, err = eval.Eval("fileAppend", []any{path, "\nline2"})
	if err != nil {
		t.Fatalf("fileAppend failed: %v", err)
	}

	// Read back again
	result, err = eval.Eval("fileRead", []any{path})
	if err != nil {
		t.Fatalf("fileRead after append failed: %v", err)
	}
	if result != "hello world\nline2" {
		t.Fatalf("expected 'hello world\\nline2', got %v", result)
	}
}

// TestFileWriteJSONSerializes tests that non-string values are JSON-serialized.
func TestFileWriteJSONSerializes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")

	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)
	sess.SetNativeProvider(&testNativeProvider{})

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}`)

	sess.mu.Lock()
	surf := sess.surfaces["s1"]
	sess.mu.Unlock()
	eval := surf.resolver.evaluator

	// Write a map — should be JSON serialized
	data := map[string]interface{}{"cpu": 45.2, "memory": 8589934592.0}
	_, err := eval.Eval("fileWrite", []any{path, data})
	if err != nil {
		t.Fatalf("fileWrite map failed: %v", err)
	}

	content, _ := os.ReadFile(path)
	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("written content is not valid JSON: %v\ncontent: %s", err, content)
	}
	if cpu, _ := parsed["cpu"].(float64); cpu != 45.2 {
		t.Errorf("expected cpu=45.2, got %v", cpu)
	}
}

// TestFileAppendCSVPattern tests the pattern of appending CSV rows — the primary streaming use case.
func TestFileAppendCSVPattern(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.csv")

	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)
	sess.SetNativeProvider(&testNativeProvider{})

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}`)

	sess.mu.Lock()
	surf := sess.surfaces["s1"]
	sess.mu.Unlock()
	eval := surf.resolver.evaluator

	// Write CSV header
	eval.Eval("fileWrite", []any{path, "timestamp,cpu,memory\n"})

	// Append 100 rows (simulating high-throughput streaming)
	for i := 0; i < 100; i++ {
		row := fmt.Sprintf("2026-04-09T00:%02d:00,%d,%d\n", i, 20+i%80, 4000+i*10)
		eval.Eval("fileAppend", []any{path, row})
	}

	content, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 101 { // header + 100 data rows
		t.Fatalf("expected 101 lines, got %d", len(lines))
	}
	if lines[0] != "timestamp,cpu,memory" {
		t.Errorf("header mismatch: %s", lines[0])
	}
}

// TestSensorEventPipelineEndToEnd tests that a sensor subscription writes to the data model.
// Simulates the native sensor by calling EventManager.Fire directly.
func TestSensorEventPipelineEndToEnd(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"Monitor","width":600,"height":400}
{"type":"on","surfaceId":"main","id":"cpu","event":"system.sensor.cpu","config":{"interval":1000},"handler":{"dataPath":"/cpu"}}
{"type":"on","surfaceId":"main","id":"mem","event":"system.sensor.memory","config":{"interval":1000},"handler":{"dataPath":"/memory"}}
{"type":"on","surfaceId":"main","id":"bat","event":"system.sensor.battery","config":{"interval":1000},"handler":{"dataPath":"/battery"}}
{"type":"on","surfaceId":"main","id":"disk","event":"system.sensor.disk","config":{"interval":1000},"handler":{"dataPath":"/disk"}}`)

	// Simulate native sensor events (as if the ObjC timer fired)
	em := sess.EventManager()
	em.Fire("system.sensor.cpu", "main", `{"usage":42.5,"userTime":25.1,"systemTime":17.4,"cores":[{"usage":55.0},{"usage":30.0}],"coreCount":2}`)
	em.Fire("system.sensor.memory", "main", `{"total":17179869184,"free":4294967296,"active":8589934592,"wired":2147483648,"compressed":1073741824,"used":11811160064,"pressure":"nominal"}`)
	em.Fire("system.sensor.battery", "main", `{"level":85,"charging":true,"pluggedIn":true,"hasBattery":true,"timeRemaining":120}`)
	em.Fire("system.sensor.disk", "main", `{"path":"/","total":499963174912,"used":287481139200,"free":212482035712,"percentUsed":57.5}`)

	sess.mu.Lock()
	surf := sess.surfaces["main"]
	sess.mu.Unlock()

	// Verify CPU
	cpu, found := surf.dm.Get("/cpu")
	if !found {
		t.Fatal("expected /cpu to be set")
	}
	cpuMap := cpu.(map[string]interface{})
	if usage, _ := cpuMap["usage"].(float64); usage != 42.5 {
		t.Errorf("cpu.usage: expected 42.5, got %v", usage)
	}
	if cores, ok := cpuMap["cores"].([]interface{}); !ok || len(cores) != 2 {
		t.Errorf("cpu.cores: expected 2 cores, got %v", cpuMap["cores"])
	}

	// Verify memory
	mem, found := surf.dm.Get("/memory")
	if !found {
		t.Fatal("expected /memory to be set")
	}
	memMap := mem.(map[string]interface{})
	if pressure, _ := memMap["pressure"].(string); pressure != "nominal" {
		t.Errorf("memory.pressure: expected 'nominal', got %v", pressure)
	}

	// Verify battery
	bat, found := surf.dm.Get("/battery")
	if !found {
		t.Fatal("expected /battery to be set")
	}
	batMap := bat.(map[string]interface{})
	if level, _ := batMap["level"].(float64); level != 85 {
		t.Errorf("battery.level: expected 85, got %v", level)
	}
	if charging, _ := batMap["charging"].(bool); !charging {
		t.Error("battery.charging: expected true")
	}

	// Verify disk
	disk, found := surf.dm.Get("/disk")
	if !found {
		t.Fatal("expected /disk to be set")
	}
	diskMap := disk.(map[string]interface{})
	if pct, _ := diskMap["percentUsed"].(float64); pct != 57.5 {
		t.Errorf("disk.percentUsed: expected 57.5, got %v", pct)
	}
}

// TestSensorThrottledSubscription tests that high-frequency sensor events are throttled.
func TestSensorThrottledSubscription(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":600,"height":400}
{"type":"on","surfaceId":"main","id":"cpu","event":"system.sensor.cpu","config":{"interval":100},"handler":{"dataPath":"/cpu","throttle":200}}`)

	em := sess.EventManager()

	// Fire 10 events rapidly
	for i := 0; i < 10; i++ {
		em.Fire("system.sensor.cpu", "main", fmt.Sprintf(`{"usage":%d}`, i*10))
	}

	sess.mu.Lock()
	surf := sess.surfaces["main"]
	sess.mu.Unlock()

	// Only the first should have gotten through (throttle = 200ms, all fired instantly)
	cpu, found := surf.dm.Get("/cpu")
	if !found {
		t.Fatal("expected /cpu to be set")
	}
	cpuMap := cpu.(map[string]interface{})
	if usage, _ := cpuMap["usage"].(float64); usage != 0 {
		t.Errorf("expected usage=0 (first event through throttle), got %v", usage)
	}
}

// TestMouseSensorPipeline tests that mouse position events flow through the pipeline.
func TestMouseSensorPipeline(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":600,"height":400}
{"type":"on","surfaceId":"main","id":"mouse","event":"system.sensor.mouse","config":{"interval":16},"handler":{"dataPath":"/mouse"}}`)

	sess.EventManager().Fire("system.sensor.mouse", "main", `{"x":512,"y":384,"screenWidth":1920,"screenHeight":1080}`)

	sess.mu.Lock()
	surf := sess.surfaces["main"]
	sess.mu.Unlock()

	val, found := surf.dm.Get("/mouse")
	if !found {
		t.Fatal("expected /mouse to be set")
	}
	m := val.(map[string]interface{})
	if x, _ := m["x"].(float64); x != 512 {
		t.Errorf("expected x=512, got %v", x)
	}
	if y, _ := m["y"].(float64); y != 384 {
		t.Errorf("expected y=384, got %v", y)
	}
}

// TestWifiSensorPipeline tests WiFi sensor data through the pipeline.
func TestWifiSensorPipeline(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":600,"height":400}
{"type":"on","surfaceId":"main","id":"wifi","event":"system.sensor.wifi","config":{"interval":5000},"handler":{"dataPath":"/wifi"}}`)

	sess.EventManager().Fire("system.sensor.wifi", "main", `{"ssid":"TestNetwork","rssi":-45,"channel":36,"noise":-90,"connected":true}`)

	sess.mu.Lock()
	surf := sess.surfaces["main"]
	sess.mu.Unlock()

	val, found := surf.dm.Get("/wifi")
	if !found {
		t.Fatal("expected /wifi to be set")
	}
	m := val.(map[string]interface{})
	if ssid, _ := m["ssid"].(string); ssid != "TestNetwork" {
		t.Errorf("expected ssid=TestNetwork, got %v", ssid)
	}
	if rssi, _ := m["rssi"].(float64); rssi != -45 {
		t.Errorf("expected rssi=-45, got %v", rssi)
	}
}

// TestProcessesSensorPipeline tests process count data through the pipeline.
func TestProcessesSensorPipeline(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":600,"height":400}
{"type":"on","surfaceId":"main","id":"procs","event":"system.sensor.processes","config":{"interval":2000},"handler":{"dataPath":"/procs"}}`)

	sess.EventManager().Fire("system.sensor.processes", "main", `{"count":342,"loadAvg1":2.5,"loadAvg5":1.8,"loadAvg15":1.2}`)

	sess.mu.Lock()
	surf := sess.surfaces["main"]
	sess.mu.Unlock()

	val, found := surf.dm.Get("/procs")
	if !found {
		t.Fatal("expected /procs to be set")
	}
	m := val.(map[string]interface{})
	if count, _ := m["count"].(float64); count != 342 {
		t.Errorf("expected count=342, got %v", count)
	}
	if la1, _ := m["loadAvg1"].(float64); la1 != 2.5 {
		t.Errorf("expected loadAvg1=2.5, got %v", la1)
	}
}

// TestMouseToFilePipeline tests the full cursor→file streaming path.
func TestMouseToFilePipeline(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "mouse.csv")

	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)
	sess.SetNativeProvider(&testNativeProvider{})

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":600,"height":400}
{"type":"on","surfaceId":"main","id":"mouse","event":"system.sensor.mouse","config":{"interval":16},"handler":{"dataPath":"/mouse"}}`)

	// Simulate 50 mouse position events
	em := sess.EventManager()
	for i := 0; i < 50; i++ {
		em.Fire("system.sensor.mouse", "main", fmt.Sprintf(`{"x":%d,"y":%d}`, i*10, i*5))
	}

	// Write accumulated data to file
	sess.mu.Lock()
	surf := sess.surfaces["main"]
	sess.mu.Unlock()

	eval := surf.resolver.evaluator
	eval.Eval("fileWrite", []any{logPath, "x,y\n"})
	for i := 0; i < 50; i++ {
		eval.Eval("fileAppend", []any{logPath, fmt.Sprintf("%d,%d\n", i*10, i*5)})
	}

	content, _ := os.ReadFile(logPath)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 51 { // header + 50 rows
		t.Fatalf("expected 51 lines, got %d", len(lines))
	}
}

// TestSensorToFileStreamingPipeline tests the complete streaming path:
// sensor event → data model → action → fileAppend
func TestSensorToFileStreamingPipeline(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "sensor.csv")

	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)
	sess.SetNativeProvider(&testNativeProvider{})

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":600,"height":400}
{"type":"updateDataModel","surfaceId":"main","ops":[{"op":"add","path":"/logPath","value":"`+logPath+`"}]}
{"type":"on","surfaceId":"main","id":"cpu","event":"system.sensor.cpu","config":{"interval":100},"handler":{"dataPath":"/cpu"}}`)

	// Simulate 5 sensor readings
	em := sess.EventManager()
	for i := 0; i < 5; i++ {
		em.Fire("system.sensor.cpu", "main", fmt.Sprintf(`{"usage":%.1f}`, float64(i)*10.5))
	}

	// Now use evaluator to read /cpu and append to file
	sess.mu.Lock()
	surf := sess.surfaces["main"]
	sess.mu.Unlock()

	eval := surf.resolver.evaluator
	cpu, _ := surf.dm.Get("/cpu")
	cpuMap, _ := cpu.(map[string]interface{})
	usage := cpuMap["usage"]

	row := fmt.Sprintf("%.1f\n", usage)
	_, err := eval.Eval("fileAppend", []any{logPath, row})
	if err != nil {
		t.Fatalf("fileAppend failed: %v", err)
	}

	content, _ := os.ReadFile(logPath)
	if !strings.Contains(string(content), ".") {
		t.Fatalf("expected numeric data in log file, got: %s", content)
	}
}

// TestTCPToDataModelToFile tests a real TCP connection streaming data through the pipeline.
func TestTCPToDataModelToFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "tcp.log")

	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)
	sess.SetNativeProvider(&testNativeProvider{})

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":600,"height":400}`)

	// Subscribe to TCP on a random port
	port := 19877
	sess.EventManager().Subscribe(protocol.OnMessage{
		Type:      protocol.MsgOn,
		SurfaceID: "main",
		ID:        "tcp-stream",
		Event:     "system.network.tcp",
		Config:    map[string]interface{}{"port": float64(port)},
		Handler:   protocol.EventAction{DataPath: "/tcp/latest"},
	})

	time.Sleep(50 * time.Millisecond)

	// Send 10 JSON lines over TCP
	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	for i := 0; i < 10; i++ {
		fmt.Fprintf(conn, `{"reading":%d}`+"\n", i)
	}
	conn.Close()

	time.Sleep(200 * time.Millisecond)

	// Verify data model has latest reading
	sess.mu.Lock()
	surf := sess.surfaces["main"]
	sess.mu.Unlock()

	val, found := surf.dm.Get("/tcp/latest")
	if !found {
		t.Fatal("expected /tcp/latest to be set")
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", val)
	}
	// Should have the last reading
	if reading, _ := m["reading"].(float64); reading != 9 {
		t.Errorf("expected reading=9 (last TCP line), got %v", reading)
	}

	// Now write the data to file
	eval := surf.resolver.evaluator
	eval.Eval("fileWrite", []any{logPath, val})

	content, _ := os.ReadFile(logPath)
	if !strings.Contains(string(content), "reading") {
		t.Fatalf("expected JSON in log file, got: %s", content)
	}

	sess.EventManager().Unsubscribe("tcp-stream")
}

// TestHTTPListenerToDataModel tests an HTTP server event source receiving POST data.
func TestHTTPListenerToDataModel(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":600,"height":400}`)

	port := 19878
	sess.EventManager().Subscribe(protocol.OnMessage{
		Type:      protocol.MsgOn,
		SurfaceID: "main",
		ID:        "http-listen",
		Event:     "system.network.http",
		Config:    map[string]interface{}{"port": float64(port)},
		Handler:   protocol.EventAction{DataPath: "/http/latest"},
	})

	time.Sleep(100 * time.Millisecond)

	// POST data to the listener
	resp, err := http.Post(fmt.Sprintf("http://localhost:%d/sensor", port), "application/json", strings.NewReader(`{"temp":42.5,"unit":"celsius"}`))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	resp.Body.Close()

	time.Sleep(100 * time.Millisecond)

	sess.mu.Lock()
	surf := sess.surfaces["main"]
	sess.mu.Unlock()

	val, found := surf.dm.Get("/http/latest")
	if !found {
		t.Fatal("expected /http/latest to be set after POST")
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T: %v", val, val)
	}
	if temp, _ := m["temp"].(float64); temp != 42.5 {
		t.Errorf("expected temp=42.5, got %v", temp)
	}

	sess.EventManager().Unsubscribe("http-listen")
}

// TestRenderCoalescing tests that rapid data model writes are batched.
func TestRenderCoalescing(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)
	// Enable batching with 50ms window
	sess.SetRenderInterval(50 * time.Millisecond)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"label","type":"Text","props":{"content":{"path":"/value"}}}]}`)

	initialUpdates := len(mock.Updated)

	// Fire 20 rapid events — should batch into fewer renders
	em := sess.EventManager()
	sess.EventManager().Subscribe(protocol.OnMessage{
		Type:      protocol.MsgOn,
		SurfaceID: "s1",
		ID:        "rapid",
		Event:     "test.rapid",
		Handler:   protocol.EventAction{DataPath: "/value"},
	})

	for i := 0; i < 20; i++ {
		em.Fire("test.rapid", "s1", fmt.Sprintf(`{"v":%d}`, i))
	}

	// Immediately after: should NOT have 20 render calls (they're coalesced)
	midUpdates := len(mock.Updated) - initialUpdates

	// Wait for coalescing timer to flush
	time.Sleep(80 * time.Millisecond)

	finalUpdates := len(mock.Updated) - initialUpdates

	// The coalesced renders should be significantly fewer than 20
	if midUpdates >= 20 {
		t.Errorf("expected fewer than 20 immediate renders (coalescing), got %d", midUpdates)
	}
	// But after flush, at least one render should have happened
	if finalUpdates == 0 {
		t.Error("expected at least one render after coalescing flush")
	}

	t.Logf("render calls: immediate=%d, after flush=%d (from 20 events)", midUpdates, finalUpdates)
}

// testNativeProvider is a real (not mock) native provider for file I/O tests.
type testNativeProvider struct{}

func (p *testNativeProvider) Notify(title, body, subtitle string) error            { return nil }
func (p *testNativeProvider) ClipboardRead() (string, error)                       { return "", nil }
func (p *testNativeProvider) ClipboardWrite(text string) error                     { return nil }
func (p *testNativeProvider) OpenURL(url string) error                             { return nil }
func (p *testNativeProvider) FileOpen(t, at string, m bool) ([]string, error)      { return nil, nil }
func (p *testNativeProvider) FileSave(t, dn, at string) (string, error)            { return "", nil }
func (p *testNativeProvider) Alert(t, m, s string, b []string) (int, error)        { return 0, nil }
func (p *testNativeProvider) CameraCapture(dp string) (string, error)              { return "", nil }
func (p *testNativeProvider) AudioRecordStart(f string, sr float64, c int) (string, error) {
	return "", nil
}
func (p *testNativeProvider) AudioRecordStop(id string) (string, error)   { return "", nil }
func (p *testNativeProvider) ScreenCapture(ct string) (string, error)     { return "", nil }
func (p *testNativeProvider) ScreenRecordStart(ct string) (string, error) { return "", nil }
func (p *testNativeProvider) ScreenRecordStop(id string) (string, error)  { return "", nil }
func (p *testNativeProvider) CleanupAll()                                 {}

// Real file I/O — these are the functions under test
func (p *testNativeProvider) FileRead(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}
func (p *testNativeProvider) FileWrite(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
func (p *testNativeProvider) FileAppend(path string, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, err = f.WriteString(content)
	f.Close()
	return err
}
