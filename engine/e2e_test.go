package engine

import (
	"jview/protocol"
	"jview/renderer"
	"jview/transport"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// parseMessage parses a single JSONL line into a Message.
func parseMessage(t *testing.T, jsonl string) *protocol.Message {
	t.Helper()
	p := protocol.NewParser(strings.NewReader(jsonl))
	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

func fixtureDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata")
}

func projectDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..")
}

// TestE2EHelloFixture reads the actual hello.jsonl through file transport
// into the engine with a mock renderer and validates the output.
func TestE2EHelloFixture(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	ft := transport.NewFileTransport(filepath.Join(fixtureDir(), "hello.jsonl"))
	ft.Start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range ft.Messages() {
			sess.HandleMessage(msg)
		}
		sess.FlushPendingComponents()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for fixture")
	}

	if len(mock.Windows) != 1 {
		t.Fatalf("windows = %d, want 1", len(mock.Windows))
	}
	if mock.Windows[0].Title != "Hello jview" {
		t.Errorf("title = %q", mock.Windows[0].Title)
	}

	created := map[string]string{}
	for _, c := range mock.Created {
		created[c.Node.ComponentID] = c.Node.Props.Content
	}

	if _, ok := created["card1"]; !ok {
		t.Error("card1 not created")
	}
	if created["heading"] != "Hello, jview!" {
		t.Errorf("heading content = %q", created["heading"])
	}
}

// TestE2EContactFormFixture validates the contact form with data binding.
func TestE2EContactFormFixture(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	ft := transport.NewFileTransport(filepath.Join(fixtureDir(), "contact_form.jsonl"))
	ft.Start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range ft.Messages() {
			sess.HandleMessage(msg)
		}
		sess.FlushPendingComponents()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	// All expected components exist
	created := map[string]bool{}
	for _, c := range mock.Created {
		created[c.Node.ComponentID] = true
	}
	for _, id := range []string{"root", "title", "nameField", "emailField", "previewCard", "subscribeCheck", "submitBtn"} {
		if !created[id] {
			t.Errorf("missing component: %s", id)
		}
	}

	// Data binding works: type in name field, preview updates
	before := len(mock.Updated)
	mock.InvokeCallback("form", "nameField", "change", "Test User")

	foundUpdate := false
	for _, u := range mock.Updated[before:] {
		if u.Node != nil && u.Node.ComponentID == "previewName" && u.Node.Props.Content == "Test User" {
			foundUpdate = true
		}
	}
	if !foundUpdate {
		t.Error("name binding did not propagate to previewName")
	}
}

// TestE2EFunctionCallsFixture validates function call evaluation.
func TestE2EFunctionCallsFixture(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	ft := transport.NewFileTransport(filepath.Join(fixtureDir(), "function_calls.jsonl"))
	ft.Start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range ft.Messages() {
			sess.HandleMessage(msg)
		}
		sess.FlushPendingComponents()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	created := map[string]string{}
	for _, c := range mock.Created {
		created[c.Node.ComponentID] = c.Node.Props.Content
	}

	// greeting: concat("Hello, ", {path:"/name"}, "!") → "Hello, world!"
	if created["greeting"] != "Hello, world!" {
		t.Errorf("greeting = %q, want 'Hello, world!'", created["greeting"])
	}

	// upper: toUpperCase({path:"/name"}) → "WORLD"
	if created["upper"] != "WORLD" {
		t.Errorf("upper = %q, want 'WORLD'", created["upper"])
	}

	// computed: format("Name: {0}, Length: {1}", /name, length(/name)) → "Name: world, Length: 5"
	if created["computed"] != "Name: world, Length: 5" {
		t.Errorf("computed = %q, want 'Name: world, Length: 5'", created["computed"])
	}

	// Verify data binding: updating /name should re-render function call components
	before := len(mock.Updated)
	sess.HandleMessage(parseMessage(t, `{"type":"updateDataModel","surfaceId":"funcs","ops":[{"op":"replace","path":"/name","value":"test"}]}`))

	foundGreeting := false
	for _, u := range mock.Updated[before:] {
		if u.Node != nil && u.Node.ComponentID == "greeting" {
			foundGreeting = true
			if u.Node.Props.Content != "Hello, test!" {
				t.Errorf("greeting after update = %q, want 'Hello, test!'", u.Node.Props.Content)
			}
		}
	}
	if !foundGreeting {
		t.Error("function call component not re-rendered after data model update")
	}
}

// TestE2EListFixture validates template expansion and List component.
func TestE2EListFixture(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	ft := transport.NewFileTransport(filepath.Join(fixtureDir(), "list.jsonl"))
	ft.Start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range ft.Messages() {
			sess.HandleMessage(msg)
		}
		sess.FlushPendingComponents()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	// Check expanded components exist
	created := map[string]string{}
	createdTitles := map[string]string{}
	for _, c := range mock.Created {
		created[c.Node.ComponentID] = c.Node.Props.Content
		createdTitles[c.Node.ComponentID] = c.Node.Props.Title
	}

	// 3 items in data model → 3 card+role pairs
	if createdTitles["itemList_itemCard_0"] != "Alice" {
		t.Errorf("itemList_itemCard_0 title = %q, want Alice", createdTitles["itemList_itemCard_0"])
	}
	if createdTitles["itemList_itemCard_1"] != "Bob" {
		t.Errorf("itemList_itemCard_1 title = %q, want Bob", createdTitles["itemList_itemCard_1"])
	}
	if createdTitles["itemList_itemCard_2"] != "Charlie" {
		t.Errorf("itemList_itemCard_2 title = %q, want Charlie", createdTitles["itemList_itemCard_2"])
	}
	if created["itemList_itemRole_0"] != "Engineer" {
		t.Errorf("itemList_itemRole_0 = %q, want Engineer", created["itemList_itemRole_0"])
	}
}

// TestE2ELayoutFixture validates nested row/column layout.
func TestE2ELayoutFixture(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	ft := transport.NewFileTransport(filepath.Join(fixtureDir(), "layout.jsonl"))
	ft.Start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range ft.Messages() {
			sess.HandleMessage(msg)
		}
		sess.FlushPendingComponents()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	// Find the Row component and verify it has 2 children
	contentHandle := mock.GetHandle("layout", "content")
	if contentHandle == 0 {
		t.Fatal("content (Row) not created")
	}

	foundRow := false
	for _, cs := range mock.Children {
		if cs.ParentHandle == contentHandle {
			foundRow = true
			if len(cs.ChildHandles) != 2 {
				t.Errorf("Row children = %d, want 2", len(cs.ChildHandles))
			}
		}
	}
	if !foundRow {
		t.Error("no SetChildren for content Row")
	}
}

// TestE2ETabsTests runs the tabs test fixture through the test runner.
func TestE2ETabsTests(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}

	results, err := RunTestFile(filepath.Join(fixtureDir(), "tabs_test.jsonl"), mock, disp)
	if err != nil {
		t.Fatalf("RunTestFile: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("no tests found in tabs_test.jsonl")
	}

	passed := 0
	totalAssertions := 0
	for _, r := range results {
		totalAssertions += r.Assertions
		if r.Passed {
			passed++
		} else {
			t.Errorf("FAIL: %s: %s", r.Name, r.Error)
		}
	}

	t.Logf("%d/%d tests passed, %d assertions total", passed, len(results), totalAssertions)
}

// TestE2ECalculatorTests runs the calculator test fixture through the test runner.
func TestE2ECalculatorTests(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}

	results, err := RunTestFile(filepath.Join(fixtureDir(), "calculator_test.jsonl"), mock, disp)
	if err != nil {
		t.Fatalf("RunTestFile: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("no tests found in calculator_test.jsonl")
	}

	passed := 0
	totalAssertions := 0
	for _, r := range results {
		totalAssertions += r.Assertions
		if r.Passed {
			passed++
		} else {
			t.Errorf("FAIL: %s: %s", r.Name, r.Error)
		}
	}

	t.Logf("%d/%d tests passed, %d assertions total", passed, len(results), totalAssertions)
}

func TestE2ECustomFunctionsFixture(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	ft := transport.NewFileTransport(filepath.Join(fixtureDir(), "custom_functions.jsonl"))
	ft.Start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range ft.Messages() {
			sess.HandleMessage(msg)
		}
		sess.FlushPendingComponents()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for file transport")
	}

	// Display should show "0"
	node := mock.LastNode("main", "display")
	if node == nil {
		t.Fatal("display not created")
	}
	if node.Props.Content != "0" {
		t.Errorf("display = %q, want '0'", node.Props.Content)
	}

	// Click button 1 — should update display to "1"
	beforeUpdates := len(mock.Updated)
	mock.InvokeCallback("main", "btn1", "click", "")

	found := false
	for _, u := range mock.Updated[beforeUpdates:] {
		if u.Node != nil && u.Node.ComponentID == "display" {
			found = true
			if u.Node.Props.Content != "1" {
				t.Errorf("display after btn1 click = %q, want '1'", u.Node.Props.Content)
			}
		}
	}
	if !found {
		t.Error("display not updated after btn1 click")
	}

	// Click button 2 — should append to get "12"
	beforeUpdates = len(mock.Updated)
	mock.InvokeCallback("main", "btn2", "click", "")

	found = false
	for _, u := range mock.Updated[beforeUpdates:] {
		if u.Node != nil && u.Node.ComponentID == "display" {
			found = true
			if u.Node.Props.Content != "12" {
				t.Errorf("display after btn2 click = %q, want '12'", u.Node.Props.Content)
			}
		}
	}
	if !found {
		t.Error("display not updated after btn2 click")
	}
}

func TestE2EComponentDefsFixture(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	ft := transport.NewFileTransport(filepath.Join(fixtureDir(), "component_defs.jsonl"))
	ft.Start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range ft.Messages() {
			sess.HandleMessage(msg)
		}
		sess.FlushPendingComponents()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for file transport")
	}

	// DigitButton instances should be expanded
	btn1Node := mock.LastNode("main", "btn1")
	if btn1Node == nil {
		t.Fatal("btn1 not created")
	}
	if btn1Node.Props.Label != "1" {
		t.Errorf("btn1 label = %q, want '1'", btn1Node.Props.Label)
	}
	if btn1Node.Type != "Button" {
		t.Errorf("btn1 type = %q, want Button", btn1Node.Type)
	}

	// OpButton instance
	btnAddNode := mock.LastNode("main", "btnAdd")
	if btnAddNode == nil {
		t.Fatal("btnAdd not created")
	}
	if btnAddNode.Props.Label != "+" {
		t.Errorf("btnAdd label = %q, want '+'", btnAddNode.Props.Label)
	}
}

func TestE2EIncludeFixture(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	ft := transport.NewFileTransport(filepath.Join(fixtureDir(), "includes", "main.jsonl"))
	ft.Start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range ft.Messages() {
			sess.HandleMessage(msg)
		}
		sess.FlushPendingComponents()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for file transport")
	}

	// The greeting should use the included greet function
	node := mock.LastNode("main", "greeting")
	if node == nil {
		t.Fatal("greeting not created")
	}
	if node.Props.Content != "Hello, World!" {
		t.Errorf("greeting = %q, want 'Hello, World!'", node.Props.Content)
	}
}

func TestE2ECalculatorV2Fixture(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	ft := transport.NewFileTransport(filepath.Join(fixtureDir(), "calculator_v2", "main.jsonl"))
	ft.Start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range ft.Messages() {
			sess.HandleMessage(msg)
		}
		sess.FlushPendingComponents()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for file transport")
	}

	// Display should show "0"
	displayNode := mock.LastNode("calculator", "displayText")
	if displayNode == nil {
		t.Fatal("displayText not created")
	}
	if displayNode.Props.Content != "0" {
		t.Errorf("display = %q, want '0'", displayNode.Props.Content)
	}

	// Digit buttons should be expanded from DigitButton components
	btn7Node := mock.LastNode("calculator", "btn7")
	if btn7Node == nil {
		t.Fatal("btn7 not created")
	}
	if btn7Node.Props.Label != "7" {
		t.Errorf("btn7 label = %q, want '7'", btn7Node.Props.Label)
	}

	// Op buttons should be expanded
	btnAddNode := mock.LastNode("calculator", "btnAdd")
	if btnAddNode == nil {
		t.Fatal("btnAdd not created")
	}
	if btnAddNode.Props.Label != "+" {
		t.Errorf("btnAdd label = %q, want '+'", btnAddNode.Props.Label)
	}

	// Test calculation: 7 + 3 =
	mock.InvokeCallback("calculator", "btn7", "click", "")
	mock.InvokeCallback("calculator", "btnAdd", "click", "")
	mock.InvokeCallback("calculator", "btn3", "click", "")

	beforeUpdates := len(mock.Updated)
	mock.InvokeCallback("calculator", "btnEquals", "click", "")

	found := false
	for _, u := range mock.Updated[beforeUpdates:] {
		if u.Node != nil && u.Node.ComponentID == "displayText" {
			found = true
			if u.Node.Props.Content != "10" {
				t.Errorf("display after 7+3= = %q, want '10'", u.Node.Props.Content)
			}
		}
	}
	if !found {
		t.Error("display not updated after equals")
	}
}

func TestE2ETabsFixture(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	ft := transport.NewFileTransport(filepath.Join(fixtureDir(), "tabs.jsonl"))
	ft.Start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range ft.Messages() {
			sess.HandleMessage(msg)
		}
		sess.FlushPendingComponents()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	// Tabs component created with correct resolved props
	tabsNode := mock.LastNode("tabs", "tabs1")
	if tabsNode == nil {
		t.Fatal("tabs1 not created")
	}
	if len(tabsNode.Props.TabLabels) != 3 {
		t.Errorf("tabLabels count = %d, want 3", len(tabsNode.Props.TabLabels))
	}
	if tabsNode.Props.ActiveTab != "tab-overview" {
		t.Errorf("activeTab = %q, want tab-overview", tabsNode.Props.ActiveTab)
	}

	// Tab children created
	for _, id := range []string{"tab-overview", "tab-settings", "tab-about"} {
		h := mock.GetHandle("tabs", id)
		if h == 0 {
			t.Errorf("child %q not created", id)
		}
	}

	// SetChildren called on tabs
	tabsHandle := mock.GetHandle("tabs", "tabs1")
	foundChildren := false
	for _, cs := range mock.Children {
		if cs.ParentHandle == tabsHandle {
			foundChildren = true
			if len(cs.ChildHandles) != 3 {
				t.Errorf("tabs children = %d, want 3", len(cs.ChildHandles))
			}
		}
	}
	if !foundChildren {
		t.Error("no SetChildren for tabs1")
	}

	// Display shows concat result
	displayNode := mock.LastNode("tabs", "display")
	if displayNode == nil {
		t.Fatal("display not created")
	}
	if displayNode.Props.Content != "Selected tab: tab-overview" {
		t.Errorf("display = %q, want 'Selected tab: tab-overview'", displayNode.Props.Content)
	}

	// Simulate tab selection
	beforeUpdates := len(mock.Updated)
	mock.InvokeCallback("tabs", "tabs1", "select", "tab-settings")

	foundDisplay := false
	for _, u := range mock.Updated[beforeUpdates:] {
		if u.Node != nil && u.Node.ComponentID == "display" {
			foundDisplay = true
			if u.Node.Props.Content != "Selected tab: tab-settings" {
				t.Errorf("display after select = %q, want 'Selected tab: tab-settings'", u.Node.Props.Content)
			}
		}
	}
	if !foundDisplay {
		t.Error("tabs select did not propagate to display")
	}
}

func TestE2EScopedComponentsFixture(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	ft := transport.NewFileTransport(filepath.Join(fixtureDir(), "scoped_components.jsonl"))
	ft.Start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range ft.Messages() {
			sess.HandleMessage(msg)
		}
		sess.FlushPendingComponents()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for file transport")
	}

	// Counter A should show "0" from /c1/count
	c1Display := mock.LastNode("main", "c1__display")
	if c1Display == nil {
		t.Fatal("c1__display not created")
	}
	if c1Display.Props.Content != "0" {
		t.Errorf("c1 display = %q, want '0'", c1Display.Props.Content)
	}

	// Counter B should show "100" from /c2/count
	c2Display := mock.LastNode("main", "c2__display")
	if c2Display == nil {
		t.Fatal("c2__display not created")
	}
	if c2Display.Props.Content != "100" {
		t.Errorf("c2 display = %q, want '100'", c2Display.Props.Content)
	}

	// Click increment on Counter A — should update c1's count from "0" to "1"
	beforeUpdates := len(mock.Updated)
	mock.InvokeCallback("main", "c1__btn", "click", "")

	found := false
	for _, u := range mock.Updated[beforeUpdates:] {
		if u.Node != nil && u.Node.ComponentID == "c1__display" {
			found = true
			if u.Node.Props.Content != "1" {
				t.Errorf("c1 display after increment = %q, want '1'", u.Node.Props.Content)
			}
		}
	}
	if !found {
		t.Error("c1__display not updated after increment")
	}

	// Counter B should be unchanged
	c2Node := mock.LastNode("main", "c2__display")
	if c2Node.Props.Content != "100" {
		t.Errorf("c2 display after c1 increment = %q, want '100'", c2Node.Props.Content)
	}
}

func TestE2EModalFixture(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	ft := transport.NewFileTransport(filepath.Join(fixtureDir(), "modal.jsonl"))
	ft.Start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range ft.Messages() {
			sess.HandleMessage(msg)
		}
		sess.FlushPendingComponents()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	// Modal component created with correct resolved props
	modalNode := mock.LastNode("modal", "modal1")
	if modalNode == nil {
		t.Fatal("modal1 not created")
	}
	if modalNode.Type != protocol.CompModal {
		t.Errorf("modal1 type = %q, want Modal", modalNode.Type)
	}
	if modalNode.Props.Title != "My Modal" {
		t.Errorf("modal1 title = %q, want 'My Modal'", modalNode.Props.Title)
	}
	if modalNode.Props.Visible != false {
		t.Errorf("modal1 visible = %v, want false", modalNode.Props.Visible)
	}

	// Modal children created
	contentHandle := mock.GetHandle("modal", "modalContent")
	if contentHandle == 0 {
		t.Error("modalContent not created")
	}

	// SetChildren called on modal
	modalHandle := mock.GetHandle("modal", "modal1")
	foundChildren := false
	for _, cs := range mock.Children {
		if cs.ParentHandle == modalHandle {
			foundChildren = true
			if len(cs.ChildHandles) != 1 {
				t.Errorf("modal children = %d, want 1", len(cs.ChildHandles))
			}
		}
	}
	if !foundChildren {
		t.Error("no SetChildren for modal1")
	}

	// Status shows "closed" initially
	statusNode := mock.LastNode("modal", "status")
	if statusNode == nil {
		t.Fatal("status not created")
	}
	if statusNode.Props.Content != "Modal is closed" {
		t.Errorf("status = %q, want 'Modal is closed'", statusNode.Props.Content)
	}

	// Click open button → visible becomes true
	beforeUpdates := len(mock.Updated)
	mock.InvokeCallback("modal", "openBtn", "click", "")

	foundModal := false
	foundStatus := false
	for _, u := range mock.Updated[beforeUpdates:] {
		if u.Node != nil && u.Node.ComponentID == "modal1" {
			foundModal = true
			if !u.Node.Props.Visible {
				t.Errorf("modal1 visible after open = %v, want true", u.Node.Props.Visible)
			}
		}
		if u.Node != nil && u.Node.ComponentID == "status" {
			foundStatus = true
			if u.Node.Props.Content != "Modal is open" {
				t.Errorf("status after open = %q, want 'Modal is open'", u.Node.Props.Content)
			}
		}
	}
	if !foundModal {
		t.Error("modal1 not updated after open click")
	}
	if !foundStatus {
		t.Error("status not updated after open click")
	}

	// Dismiss callback → visible becomes false
	beforeUpdates = len(mock.Updated)
	mock.InvokeCallback("modal", "modal1", "dismiss", "")

	foundModal = false
	for _, u := range mock.Updated[beforeUpdates:] {
		if u.Node != nil && u.Node.ComponentID == "modal1" {
			foundModal = true
			if u.Node.Props.Visible {
				t.Errorf("modal1 visible after dismiss = %v, want false", u.Node.Props.Visible)
			}
		}
	}
	if !foundModal {
		t.Error("modal1 not updated after dismiss")
	}
}

func TestE2EVideoFixture(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	ft := transport.NewFileTransport(filepath.Join(fixtureDir(), "video.jsonl"))
	ft.Start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range ft.Messages() {
			sess.HandleMessage(msg)
		}
		sess.FlushPendingComponents()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	// Video component created with correct resolved props
	playerNode := mock.LastNode("video", "player")
	if playerNode == nil {
		t.Fatal("player not created")
	}
	if playerNode.Type != protocol.CompVideo {
		t.Errorf("player type = %q, want Video", playerNode.Type)
	}
	if playerNode.Props.Src == "" {
		t.Error("player src is empty")
	}
	if playerNode.Props.Width != 560 {
		t.Errorf("player width = %d, want 560", playerNode.Props.Width)
	}
	if playerNode.Props.Height != 315 {
		t.Errorf("player height = %d, want 315", playerNode.Props.Height)
	}
	if !playerNode.Props.Controls {
		t.Error("player controls should be true")
	}
	if playerNode.Props.Autoplay {
		t.Error("player autoplay should be false")
	}
	if playerNode.Props.Loop {
		t.Error("player loop should be false")
	}
	if !playerNode.Props.Muted {
		t.Error("player muted should be true")
	}
}

func TestE2EVideoTests(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}

	results, err := RunTestFile(filepath.Join(fixtureDir(), "video_test.jsonl"), mock, disp)
	if err != nil {
		t.Fatalf("RunTestFile: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("no tests found in video_test.jsonl")
	}

	passed := 0
	totalAssertions := 0
	for _, r := range results {
		totalAssertions += r.Assertions
		if r.Passed {
			passed++
		} else {
			t.Errorf("FAIL: %s: %s", r.Name, r.Error)
		}
	}

	t.Logf("%d/%d tests passed, %d assertions total", passed, len(results), totalAssertions)
}

func TestE2EModalTests(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}

	results, err := RunTestFile(filepath.Join(fixtureDir(), "modal_test.jsonl"), mock, disp)
	if err != nil {
		t.Fatalf("RunTestFile: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("no tests found in modal_test.jsonl")
	}

	passed := 0
	totalAssertions := 0
	for _, r := range results {
		totalAssertions += r.Assertions
		if r.Passed {
			passed++
		} else {
			t.Errorf("FAIL: %s: %s", r.Name, r.Error)
		}
	}

	t.Logf("%d/%d tests passed, %d assertions total", passed, len(results), totalAssertions)
}

func TestE2EAudioFixture(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	ft := transport.NewFileTransport(filepath.Join(fixtureDir(), "audio.jsonl"))
	ft.Start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range ft.Messages() {
			sess.HandleMessage(msg)
		}
		sess.FlushPendingComponents()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	playerNode := mock.LastNode("audio", "player")
	if playerNode == nil {
		t.Fatal("player not created")
	}
	if playerNode.Type != protocol.CompAudioPlayer {
		t.Errorf("player type = %q, want AudioPlayer", playerNode.Type)
	}
	if playerNode.Props.Src == "" {
		t.Error("player src is empty")
	}
	if playerNode.Props.Autoplay {
		t.Error("player autoplay should be false")
	}
	if playerNode.Props.Loop {
		t.Error("player loop should be false")
	}
}

func TestE2EAudioTests(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}

	results, err := RunTestFile(filepath.Join(fixtureDir(), "audio_test.jsonl"), mock, disp)
	if err != nil {
		t.Fatalf("RunTestFile: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("no tests found in audio_test.jsonl")
	}

	passed := 0
	totalAssertions := 0
	for _, r := range results {
		totalAssertions += r.Assertions
		if r.Passed {
			passed++
		} else {
			t.Errorf("FAIL: %s: %s", r.Name, r.Error)
		}
	}

	t.Logf("%d/%d tests passed, %d assertions total", passed, len(results), totalAssertions)
}

func runSampleAppTests(t *testing.T, relPath string) {
	t.Helper()
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}

	results, err := RunTestFile(filepath.Join(projectDir(), relPath), mock, disp)
	if err != nil {
		t.Fatalf("RunTestFile: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("no tests found in " + relPath)
	}

	passed := 0
	totalAssertions := 0
	for _, r := range results {
		totalAssertions += r.Assertions
		if r.Passed {
			passed++
		} else {
			t.Errorf("FAIL: %s: %s", r.Name, r.Error)
		}
	}

	t.Logf("%d/%d tests passed, %d assertions total", passed, len(results), totalAssertions)
}

func TestE2EThemeSwitcherTests(t *testing.T) {
	runSampleAppTests(t, "sample_apps/theme_switcher/prompt.jsonl")
}

func TestE2EDynamicListTests(t *testing.T) {
	runSampleAppTests(t, "samples/dynamic_list.jsonl")
}

func TestE2EScrollableFeedTests(t *testing.T) {
	runSampleAppTests(t, "sample_apps/scrollable_feed/prompt.jsonl")
}

func TestE2ESysinfoTests(t *testing.T) {
	runSampleAppTests(t, "sample_apps/sysinfo/prompt.jsonl")
}

func TestE2EChannelFixture(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	cm := NewChannelManager(sess)
	sess.SetChannelManager(cm)

	ft := transport.NewFileTransport(filepath.Join(fixtureDir(), "channel.jsonl"))
	ft.Start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range ft.Messages() {
			sess.HandleMessage(msg)
		}
		sess.FlushPendingComponents()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	surf := sess.GetSurface("main")
	if surf == nil {
		t.Fatal("surface not created")
	}

	// Channel value should be in data model
	val, ok := surf.DM().Get("/channels/events/value")
	if !ok {
		t.Fatal("channel value not set in data model")
	}
	if val != "Build complete!" {
		t.Errorf("channel value = %v, want 'Build complete!'", val)
	}

	// Subscriber target path should be set
	notif, ok := surf.DM().Get("/notification")
	if !ok {
		t.Fatal("/notification not set")
	}
	if notif != "Build complete!" {
		t.Errorf("/notification = %v, want 'Build complete!'", notif)
	}

	// Status should be active
	status, ok := surf.DM().Get("/channels/events/status")
	if !ok {
		t.Fatal("channel status not set")
	}
	if status != "active" {
		t.Errorf("status = %v, want 'active'", status)
	}
}

func TestE2EChannelTests(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}

	results, err := RunTestFile(filepath.Join(fixtureDir(), "channel_test.jsonl"), mock, disp)
	if err != nil {
		t.Fatalf("RunTestFile: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("no tests found in channel_test.jsonl")
	}

	passed := 0
	totalAssertions := 0
	for _, r := range results {
		totalAssertions += r.Assertions
		if r.Passed {
			passed++
		} else {
			t.Errorf("FAIL: %s: %s", r.Name, r.Error)
		}
	}

	t.Logf("%d/%d tests passed, %d assertions total", passed, len(results), totalAssertions)
}

func TestE2ECalculatorV2Tests(t *testing.T) {
	runSampleAppTests(t, "sample_apps/calculator/prompt.jsonl")
}

func TestE2EChannelDemoTests(t *testing.T) {
	runSampleAppTests(t, "sample_apps/channel_demo/prompt.jsonl")
}

func TestE2ESplitViewTests(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	results, err := RunTestFile(filepath.Join(fixtureDir(), "splitview_test.jsonl"), mock, disp)
	if err != nil {
		t.Fatalf("RunTestFile: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no tests found in splitview_test.jsonl")
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("FAIL: %s: %s", r.Name, r.Error)
		}
	}
	t.Logf("%d tests passed", len(results))
}

func TestE2ESearchFieldTests(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	results, err := RunTestFile(filepath.Join(fixtureDir(), "searchfield_test.jsonl"), mock, disp)
	if err != nil {
		t.Fatalf("RunTestFile: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no tests found in searchfield_test.jsonl")
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("FAIL: %s: %s", r.Name, r.Error)
		}
	}
	t.Logf("%d tests passed", len(results))
}

func TestE2EShellTests(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	results, err := RunTestFile(filepath.Join(fixtureDir(), "shell_test.jsonl"), mock, disp)
	if err != nil {
		t.Fatalf("RunTestFile: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no tests found in shell_test.jsonl")
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("FAIL: %s: %s", r.Name, r.Error)
		}
	}
	t.Logf("%d tests passed", len(results))
}

func TestE2ENotesAppTests(t *testing.T) {
	runSampleAppTests(t, "sample_apps/notes/prompt.jsonl")
}

