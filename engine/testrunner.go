package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"

	"jview/protocol"
	"jview/renderer"
)

// TestResult holds the outcome of a single test.
type TestResult struct {
	Name       string
	Passed     bool
	Assertions int
	Skipped    int
	Error      string // non-empty on failure
}

// CapturedAction records an action fired during test execution.
type CapturedAction struct {
	SurfaceID string
	Name      string
	Data      map[string]interface{}
}

// RunTestFile reads a JSONL file and executes all test messages using the given
// renderer and dispatcher. For e2e tests, pass real darwin.Renderer+Dispatcher.
func RunTestFile(path string, rend renderer.Renderer, disp renderer.Dispatcher) ([]TestResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return RunTests(f, rend, disp)
}

// RunTests reads JSONL from r, builds app state from non-test messages,
// then executes test messages in order.
func RunTests(r io.Reader, rend renderer.Renderer, disp renderer.Dispatcher) ([]TestResult, error) {
	sess := NewSession(rend, disp)

	// Attach channel manager so channel messages work in tests
	cm := NewChannelManager(sess)
	sess.SetChannelManager(cm)

	// Collect actions fired during tests
	var actions []CapturedAction
	sess.OnAction = func(surfaceID string, event *protocol.EventDef, data map[string]interface{}) {
		actions = append(actions, CapturedAction{
			SurfaceID: surfaceID,
			Name:      event.Name,
			Data:      data,
		})
	}

	// Parse all messages, separating tests from app messages
	var tests []protocol.TestMessage
	parser := protocol.NewParser(r)
	for {
		msg, err := parser.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse: %w", err)
		}
		if msg.Type == protocol.MsgTest {
			tests = append(tests, msg.Body.(protocol.TestMessage))
		} else {
			sess.HandleMessage(msg)
		}
	}

	// Flush any buffered components before running tests
	sess.FlushPendingComponents()

	// Execute tests sequentially
	var results []TestResult
	for _, tm := range tests {
		// Clear captured actions at the start of each test
		actions = actions[:0]
		result := executeTest(sess, rend, &actions, tm)
		results = append(results, result)
	}

	return results, nil
}

func executeTest(sess *Session, rend renderer.Renderer, actions *[]CapturedAction, tm protocol.TestMessage) TestResult {
	result := TestResult{Name: tm.Name, Passed: true}

	for i, step := range tm.Steps {
		var errMsg string
		if step.Simulate != "" {
			errMsg = executeSimulate(sess, rend, tm.SurfaceID, step)
		} else if step.Assert != "" {
			errMsg = executeAssert(sess, rend, actions, tm.SurfaceID, step)
			result.Assertions++
		}
		if errMsg != "" {
			result.Passed = false
			result.Error = fmt.Sprintf("step %d: %s", i+1, errMsg)
			return result
		}
	}

	return result
}

func executeSimulate(sess *Session, rend renderer.Renderer, surfaceID string, step protocol.TestStep) string {
	switch step.Simulate {
	case "event":
		rend.InvokeCallback(surfaceID, step.ComponentID, step.Event, step.EventData)
		return ""
	case "updateDataModel":
		surf, ok := sess.surfaces[surfaceID]
		if !ok {
			return fmt.Sprintf("surface %q not found", surfaceID)
		}
		surf.HandleUpdateDataModel(protocol.UpdateDataModel{
			Ops: []protocol.DataModelOp{{Op: "replace", Path: step.Path, Value: step.Value}},
		})
		return ""
	default:
		return fmt.Sprintf("unknown simulate type: %s", step.Simulate)
	}
}

func executeAssert(sess *Session, rend renderer.Renderer, actions *[]CapturedAction, surfaceID string, step protocol.TestStep) string {
	switch step.Assert {
	case "component":
		return assertComponent(sess, surfaceID, step)
	case "dataModel":
		return assertDataModel(sess, surfaceID, step)
	case "children":
		return assertChildren(sess, surfaceID, step)
	case "notExists":
		return assertNotExists(sess, surfaceID, step)
	case "count":
		return assertCount(sess, surfaceID, step)
	case "action":
		return assertAction(actions, step)
	case "layout":
		return assertLayout(rend, surfaceID, step)
	case "style":
		return assertStyle(rend, surfaceID, step)
	default:
		return fmt.Sprintf("unknown assert type: %s", step.Assert)
	}
}

func assertComponent(sess *Session, surfaceID string, step protocol.TestStep) string {
	surf, ok := sess.surfaces[surfaceID]
	if !ok {
		return fmt.Sprintf("surface %q not found", surfaceID)
	}

	comp, ok := surf.tree.Get(step.ComponentID)
	if !ok {
		return fmt.Sprintf("component %q not found", step.ComponentID)
	}

	// Check component type if specified
	if step.ComponentType != "" {
		if string(comp.Type) != step.ComponentType {
			return fmt.Sprintf("component %q type = %q, want %q", step.ComponentID, comp.Type, step.ComponentType)
		}
	}

	if len(step.Props) == 0 {
		return ""
	}

	// Re-resolve the component to get current props
	node := surf.resolver.Resolve(comp)

	// Marshal resolved props to map for subset matching
	propsJSON, err := json.Marshal(node.Props)
	if err != nil {
		return fmt.Sprintf("marshal resolved props: %v", err)
	}
	var actual map[string]interface{}
	if err := json.Unmarshal(propsJSON, &actual); err != nil {
		return fmt.Sprintf("unmarshal resolved props: %v", err)
	}

	// Subset match: every key in step.Props must match
	for key, expected := range step.Props {
		got, exists := actual[key]
		if !exists {
			// Check if the expected value is a zero value — omitempty may have dropped it
			if isZeroValue(expected) {
				continue
			}
			return fmt.Sprintf("assertComponent %s: prop %q not present, want %v", step.ComponentID, key, expected)
		}
		if !jsonEqual(got, expected) {
			return fmt.Sprintf("assertComponent %s: props.%s = %v, want %v", step.ComponentID, key, got, expected)
		}
	}

	return ""
}

func assertDataModel(sess *Session, surfaceID string, step protocol.TestStep) string {
	surf, ok := sess.surfaces[surfaceID]
	if !ok {
		return fmt.Sprintf("surface %q not found", surfaceID)
	}

	got, found := surf.dm.Get(step.Path)
	if !found {
		return fmt.Sprintf("assertDataModel: path %q not found", step.Path)
	}

	if !jsonEqual(got, step.Value) {
		return fmt.Sprintf("assertDataModel: %s = %v, want %v", step.Path, got, step.Value)
	}

	return ""
}

func assertChildren(sess *Session, surfaceID string, step protocol.TestStep) string {
	surf, ok := sess.surfaces[surfaceID]
	if !ok {
		return fmt.Sprintf("surface %q not found", surfaceID)
	}

	if _, exists := surf.tree.Get(step.ComponentID); !exists {
		return fmt.Sprintf("assertChildren: component %q not found", step.ComponentID)
	}

	children := surf.tree.Children(step.ComponentID)
	if len(children) != len(step.Children) {
		return fmt.Sprintf("assertChildren %s: got %v, want %v", step.ComponentID, children, step.Children)
	}
	for i, want := range step.Children {
		if children[i] != want {
			return fmt.Sprintf("assertChildren %s: child[%d] = %q, want %q", step.ComponentID, i, children[i], want)
		}
	}

	return ""
}

func assertNotExists(sess *Session, surfaceID string, step protocol.TestStep) string {
	surf, ok := sess.surfaces[surfaceID]
	if !ok {
		return fmt.Sprintf("surface %q not found", surfaceID)
	}

	if _, exists := surf.tree.Get(step.ComponentID); exists {
		return fmt.Sprintf("assertNotExists: component %q exists but should not", step.ComponentID)
	}

	return ""
}

func assertCount(sess *Session, surfaceID string, step protocol.TestStep) string {
	surf, ok := sess.surfaces[surfaceID]
	if !ok {
		return fmt.Sprintf("surface %q not found", surfaceID)
	}

	if _, exists := surf.tree.Get(step.ComponentID); !exists {
		return fmt.Sprintf("assertCount: component %q not found", step.ComponentID)
	}

	children := surf.tree.Children(step.ComponentID)
	if len(children) != step.Count {
		return fmt.Sprintf("assertCount %s: len(children) = %d, want %d", step.ComponentID, len(children), step.Count)
	}

	return ""
}

func assertAction(actions *[]CapturedAction, step protocol.TestStep) string {
	for _, a := range *actions {
		if a.Name != step.ActionName {
			continue
		}
		// Name matches — check data if specified
		if len(step.ActionData) == 0 {
			return ""
		}
		// Match on data: every expected key must be present and equal
		for key, expected := range step.ActionData {
			got, exists := a.Data[key]
			if !exists {
				return fmt.Sprintf("assertAction %s: data[%s] not present", step.ActionName, key)
			}
			if !jsonEqual(got, expected) {
				return fmt.Sprintf("assertAction %s: data[%s] = %v, want %v", step.ActionName, key, got, expected)
			}
		}
		return ""
	}

	return fmt.Sprintf("assertAction: no action with name %q fired", step.ActionName)
}

func assertLayout(rend renderer.Renderer, surfaceID string, step protocol.TestStep) string {
	info := rend.QueryLayout(surfaceID, step.ComponentID)

	// Marshal layout info to map for subset matching
	infoJSON, err := json.Marshal(info)
	if err != nil {
		return fmt.Sprintf("assertLayout: marshal error: %v", err)
	}
	var actual map[string]interface{}
	if err := json.Unmarshal(infoJSON, &actual); err != nil {
		return fmt.Sprintf("assertLayout: unmarshal error: %v", err)
	}

	for key, expected := range step.Layout {
		got, exists := actual[key]
		if !exists {
			if isZeroValue(expected) {
				continue
			}
			return fmt.Sprintf("assertLayout %s: %q not present, want %v", step.ComponentID, key, expected)
		}
		if !jsonEqual(got, expected) {
			return fmt.Sprintf("assertLayout %s: %s = %v, want %v", step.ComponentID, key, got, expected)
		}
	}

	return ""
}

func assertStyle(rend renderer.Renderer, surfaceID string, step protocol.TestStep) string {
	info := rend.QueryStyle(surfaceID, step.ComponentID)

	// Marshal style info to map for subset matching
	infoJSON, err := json.Marshal(info)
	if err != nil {
		return fmt.Sprintf("assertStyle: marshal error: %v", err)
	}
	var actual map[string]interface{}
	if err := json.Unmarshal(infoJSON, &actual); err != nil {
		return fmt.Sprintf("assertStyle: unmarshal error: %v", err)
	}

	for key, expected := range step.Style {
		got, exists := actual[key]
		if !exists {
			if isZeroValue(expected) {
				continue
			}
			return fmt.Sprintf("assertStyle %s: %q not present, want %v", step.ComponentID, key, expected)
		}
		if !jsonEqual(got, expected) {
			return fmt.Sprintf("assertStyle %s: %s = %v, want %v", step.ComponentID, key, got, expected)
		}
	}

	return ""
}

// ExecuteTestLite runs test assertions that only need Surface state.
// Layout/style assertions are skipped (they require main thread access).
// If rend is non-nil, simulations are executed via rend.InvokeCallback;
// if nil (headless mode), simulations are skipped.
func ExecuteTestLite(sess *Session, rend renderer.Renderer, tm protocol.TestMessage) TestResult {
	result := TestResult{Name: tm.Name, Passed: true}

	surf := sess.GetSurface(tm.SurfaceID)
	if surf == nil {
		result.Passed = false
		result.Error = fmt.Sprintf("surface %q not found", tm.SurfaceID)
		return result
	}

	skipped := 0
	for i, step := range tm.Steps {
		if step.Simulate != "" {
			if rend != nil {
				errMsg := executeSimulate(sess, rend, tm.SurfaceID, step)
				if errMsg != "" {
					result.Passed = false
					result.Error = fmt.Sprintf("step %d: %s", i+1, errMsg)
					return result
				}
			} else {
				skipped++
			}
			continue
		}
		if step.Assert == "" {
			continue
		}

		var errMsg string
		switch step.Assert {
		case "layout", "style":
			skipped++
			continue
		case "component":
			errMsg = assertComponent(sess, tm.SurfaceID, step)
		case "dataModel":
			errMsg = assertDataModel(sess, tm.SurfaceID, step)
		case "children":
			errMsg = assertChildren(sess, tm.SurfaceID, step)
		case "count":
			errMsg = assertCount(sess, tm.SurfaceID, step)
		case "notExists":
			errMsg = assertNotExists(sess, tm.SurfaceID, step)
		default:
			errMsg = fmt.Sprintf("unknown assert type: %s", step.Assert)
		}
		result.Assertions++

		if errMsg != "" {
			result.Passed = false
			result.Error = fmt.Sprintf("step %d: %s", i+1, errMsg)
			return result
		}
	}

	if skipped > 0 {
		result.Skipped = skipped
	}
	return result
}

// FormatTestResult formats a TestResult as a human-readable string.
func FormatTestResult(r TestResult) string {
	if r.Passed {
		msg := fmt.Sprintf("PASSED: %q (%d assertions)", r.Name, r.Assertions)
		if r.Skipped > 0 {
			msg += fmt.Sprintf(" [%d steps skipped: simulations/layout/style not available during generation]", r.Skipped)
		}
		return msg
	}
	return fmt.Sprintf("FAILED: %q — %s", r.Name, r.Error)
}

// jsonEqual compares two values that may have come from JSON (handling float64/int normalization).
func jsonEqual(a, b interface{}) bool {
	// Normalize both sides through JSON roundtrip for consistent types
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

// isZeroValue checks if a JSON-decoded value is a zero value.
func isZeroValue(v interface{}) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.IsZero()
}
