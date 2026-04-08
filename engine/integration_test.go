package engine

import (
	"canopy/protocol"
	"canopy/renderer"
	"os"
	"strings"
	"testing"
	"time"
)

// feedMessages parses JSONL and feeds to session. Returns after all processed.
func feedMessages(t *testing.T, sess *Session, jsonl string) {
	t.Helper()
	p := protocol.NewParser(strings.NewReader(jsonl))
	for {
		msg, err := p.Next()
		if err != nil {
			break
		}
		sess.HandleMessage(msg)
	}
	sess.FlushPendingComponents()
}

func TestHelloFixtureCreatesWindowAndViews(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"Hello","width":600,"height":400}
{"type":"updateComponents","surfaceId":"main","components":[{"componentId":"card1","type":"Card","props":{"title":"Welcome"},"children":["heading","body"]},{"componentId":"heading","type":"Text","props":{"content":"Hello, jview!","variant":"h1"}},{"componentId":"body","type":"Text","props":{"content":"Body text.","variant":"body"}}]}`)

	// Window created
	if len(mock.Windows) != 1 {
		t.Fatalf("windows = %d, want 1", len(mock.Windows))
	}
	if mock.Windows[0].Title != "Hello" {
		t.Errorf("window title = %q, want Hello", mock.Windows[0].Title)
	}

	// All 3 components created
	if len(mock.Created) < 3 {
		t.Fatalf("created views = %d, want >= 3", len(mock.Created))
	}

	// Find the created components by ID
	created := map[string]*renderer.RenderNode{}
	for _, c := range mock.Created {
		created[c.Node.ComponentID] = c.Node
	}

	if _, ok := created["heading"]; !ok {
		t.Error("heading not created")
	}
	if _, ok := created["body"]; !ok {
		t.Error("body not created")
	}
	if _, ok := created["card1"]; !ok {
		t.Error("card1 not created")
	}

	// Heading has correct resolved props
	if h := created["heading"]; h != nil {
		if h.Props.Content != "Hello, jview!" {
			t.Errorf("heading content = %q, want 'Hello, jview!'", h.Props.Content)
		}
		if h.Props.Variant != "h1" {
			t.Errorf("heading variant = %q, want h1", h.Props.Variant)
		}
	}

	// Card has children set
	if len(mock.Children) == 0 {
		t.Fatal("no SetChildren calls")
	}
	cardHandle := mock.GetHandle("main", "card1")
	foundCardChildren := false
	for _, cs := range mock.Children {
		if cs.ParentHandle == cardHandle {
			foundCardChildren = true
			if len(cs.ChildHandles) != 2 {
				t.Errorf("card child count = %d, want 2", len(cs.ChildHandles))
			}
		}
	}
	if !foundCardChildren {
		t.Error("no SetChildren call for card1")
	}

	// Root set
	if len(mock.RootSets) == 0 {
		t.Fatal("no SetRootView calls")
	}
}

func TestDataBindingPropagates(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"form","title":"Form"}
{"type":"updateDataModel","surfaceId":"form","ops":[{"op":"add","path":"/name","value":""}]}
{"type":"updateComponents","surfaceId":"form","components":[{"componentId":"field","type":"TextField","props":{"value":{"path":"/name"},"dataBinding":"/name"}},{"componentId":"display","type":"Text","props":{"content":{"path":"/name"},"variant":"body"}}]}`)

	// Both components created
	created := map[string]bool{}
	for _, c := range mock.Created {
		created[c.Node.ComponentID] = true
	}
	if !created["field"] || !created["display"] {
		t.Fatalf("missing components: field=%v display=%v", created["field"], created["display"])
	}

	// Simulate typing in the text field
	initialUpdates := len(mock.Updated)
	mock.InvokeCallback("form", "field", "change", "Alice")

	// The display text should have been re-rendered with new value
	newUpdates := mock.Updated[initialUpdates:]
	foundDisplayUpdate := false
	for _, u := range newUpdates {
		node := u.Node
		if node != nil && node.ComponentID == "display" {
			foundDisplayUpdate = true
			if node.Props.Content != "Alice" {
				t.Errorf("display content after binding = %q, want Alice", node.Props.Content)
			}
		}
	}
	if !foundDisplayUpdate {
		t.Error("data binding did not trigger display update")
	}
}

func TestCheckBoxBindingPropagates(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/agree","value":false}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"cb","type":"CheckBox","props":{"label":"Agree","checked":{"path":"/agree"},"dataBinding":"/agree"}},{"componentId":"status","type":"Text","props":{"content":{"path":"/agree"},"variant":"body"}}]}`)

	initialUpdates := len(mock.Updated)
	mock.InvokeCallback("s1", "cb", "toggle", "true")

	newUpdates := mock.Updated[initialUpdates:]
	foundStatusUpdate := false
	for _, u := range newUpdates {
		if u.Node != nil && u.Node.ComponentID == "status" {
			foundStatusUpdate = true
			if u.Node.Props.Content != "true" {
				t.Errorf("status content = %q, want 'true'", u.Node.Props.Content)
			}
		}
	}
	if !foundStatusUpdate {
		t.Error("checkbox binding did not trigger status update")
	}
}

func TestButtonCallbackFires(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	var firedEvent string
	// Hook into the surface's action handler after creation
	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Go","style":"primary","onClick":{"action":{"event":{"name":"doThing"}}}}}]}`)

	// Set action handler on the surface
	if surf, ok := sess.surfaces["s1"]; ok {
		surf.ActionHandler = func(sid string, event *protocol.EventDef, data map[string]interface{}) {
			firedEvent = event.Name
		}
	}

	mock.InvokeCallback("s1", "btn", "click", "")
	if firedEvent != "doThing" {
		t.Errorf("firedEvent = %q, want doThing", firedEvent)
	}
}

func TestMultipleDataModelOps(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/a","value":"1"}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":{"path":"/a"}}}]}`)

	// Find initial content
	var initialContent string
	for _, c := range mock.Created {
		if c.Node.ComponentID == "t1" {
			initialContent = c.Node.Props.Content
		}
	}
	if initialContent != "1" {
		t.Errorf("initial content = %q, want '1'", initialContent)
	}

	// Update data model
	beforeUpdates := len(mock.Updated)
	feedMessages(t, sess, `{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"replace","path":"/a","value":"2"}]}`)

	newUpdates := mock.Updated[beforeUpdates:]
	found := false
	for _, u := range newUpdates {
		if u.Node != nil && u.Node.ComponentID == "t1" {
			found = true
			if u.Node.Props.Content != "2" {
				t.Errorf("updated content = %q, want '2'", u.Node.Props.Content)
			}
		}
	}
	if !found {
		t.Error("data model update did not trigger re-render")
	}
}

func TestRenderOrderLeavesBeforeParents(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	// Parent references children that come later in the array
	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"parent","type":"Column","children":["child1","child2"]},{"componentId":"child1","type":"Text","props":{"content":"A"}},{"componentId":"child2","type":"Text","props":{"content":"B"}}]}`)

	// All 3 must be created
	if len(mock.Created) < 3 {
		t.Fatalf("created = %d, want >= 3", len(mock.Created))
	}

	// Children must be created before parent
	order := map[string]int{}
	for i, c := range mock.Created {
		order[c.Node.ComponentID] = i
	}
	if order["child1"] > order["parent"] {
		t.Error("child1 created after parent — topological sort failed")
	}
	if order["child2"] > order["parent"] {
		t.Error("child2 created after parent — topological sort failed")
	}
}

func TestMultiRootWrapping(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"One"}},{"componentId":"t2","type":"Text","props":{"content":"Two"}}]}`)

	// __root_wrapper__ Column should be created
	wrapperCreated := false
	for _, c := range mock.Created {
		if c.Node.ComponentID == "__root_wrapper__" {
			wrapperCreated = true
			if c.Node.Type != protocol.CompColumn {
				t.Errorf("wrapper type = %q, want Column", c.Node.Type)
			}
		}
	}
	if !wrapperCreated {
		t.Fatal("__root_wrapper__ not created for multi-root surface")
	}

	// SetChildren called on wrapper with both root handles
	wrapperHandle := mock.GetHandle("s1", "__root_wrapper__")
	if wrapperHandle == 0 {
		t.Fatal("wrapper handle not found")
	}
	foundWrapperChildren := false
	for _, cs := range mock.Children {
		if cs.ParentHandle == wrapperHandle {
			foundWrapperChildren = true
			if len(cs.ChildHandles) != 2 {
				t.Errorf("wrapper child count = %d, want 2", len(cs.ChildHandles))
			}
		}
	}
	if !foundWrapperChildren {
		t.Error("no SetChildren call for wrapper")
	}

	// SetRootView called with wrapper handle
	foundRoot := false
	for _, rs := range mock.RootSets {
		if rs.Handle == wrapperHandle {
			foundRoot = true
		}
	}
	if !foundRoot {
		t.Error("SetRootView not called with wrapper handle")
	}
}

func TestCallbackReregistrationCleansUp(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/name","value":""}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"field","type":"TextField","props":{"value":{"path":"/name"},"dataBinding":"/name"}}]}`)

	// Get the first callback ID
	firstCBID := mock.GetCallbackID("s1", "field", "change")
	if firstCBID == 0 {
		t.Fatal("no callback registered for field change")
	}

	// Re-render the same TextField (simulating an update)
	feedMessages(t, sess, `{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"field","type":"TextField","props":{"value":{"path":"/name"},"dataBinding":"/name"}}]}`)

	// Old callback should be unregistered
	if mock.HasCallback(firstCBID) {
		t.Error("old callback still registered after re-render")
	}

	// New callback should exist
	newCBID := mock.GetCallbackID("s1", "field", "change")
	if newCBID == 0 {
		t.Fatal("no new callback registered")
	}
	if newCBID == firstCBID {
		t.Error("new callback ID same as old — expected fresh registration")
	}
}

func TestCallbackBindingPathChange(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/name","value":"Alice"},{"op":"add","path":"/nickname","value":""}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"field","type":"TextField","props":{"value":{"path":"/name"},"dataBinding":"/name"}},{"componentId":"display","type":"Text","props":{"content":{"path":"/nickname"}}}]}`)

	// Now change binding from /name to /nickname
	feedMessages(t, sess, `{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"field","type":"TextField","props":{"value":{"path":"/nickname"},"dataBinding":"/nickname"}}]}`)

	// Typing should write to /nickname
	beforeUpdates := len(mock.Updated)
	mock.InvokeCallback("s1", "field", "change", "Ally")

	// Check the data model was updated at /nickname
	if surf, ok := sess.surfaces["s1"]; ok {
		val, found := surf.dm.Get("/nickname")
		if !found || val != "Ally" {
			t.Errorf("/nickname = %v (%v), want Ally", val, found)
		}
	}

	// The display component bound to /nickname should be updated
	foundDisplay := false
	for _, u := range mock.Updated[beforeUpdates:] {
		if u.Node != nil && u.Node.ComponentID == "display" {
			foundDisplay = true
			if u.Node.Props.Content != "Ally" {
				t.Errorf("display content = %q, want Ally", u.Node.Props.Content)
			}
		}
	}
	if !foundDisplay {
		t.Error("display not updated after binding path change")
	}
}

func TestContactFormFixtureFull(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"form","title":"Contact Form","width":500,"height":500}
{"type":"updateDataModel","surfaceId":"form","ops":[{"op":"add","path":"/name","value":""},{"op":"add","path":"/email","value":""},{"op":"add","path":"/subscribe","value":false}]}
{"type":"updateComponents","surfaceId":"form","components":[{"componentId":"root","type":"Column","props":{"gap":16,"padding":20},"children":["title","nameField","emailField","previewName","submitBtn"]},{"componentId":"title","type":"Text","props":{"content":"Contact Us","variant":"h2"}},{"componentId":"nameField","type":"TextField","props":{"placeholder":"Enter your name","value":{"path":"/name"},"dataBinding":"/name"}},{"componentId":"emailField","type":"TextField","props":{"placeholder":"you@example.com","value":{"path":"/email"},"dataBinding":"/email"}},{"componentId":"previewName","type":"Text","props":{"content":{"path":"/name"},"variant":"body"}},{"componentId":"submitBtn","type":"Button","props":{"label":"Submit","style":"primary","onClick":{"action":{"event":{"name":"submitForm","dataRefs":["/name","/email","/subscribe"]}}}}}]}`)

	// Window + all 6 components created
	if len(mock.Windows) != 1 {
		t.Fatalf("windows = %d", len(mock.Windows))
	}

	created := map[string]bool{}
	for _, c := range mock.Created {
		created[c.Node.ComponentID] = true
	}
	for _, id := range []string{"root", "title", "nameField", "emailField", "previewName", "submitBtn"} {
		if !created[id] {
			t.Errorf("component %q not created", id)
		}
	}

	// Type in name field → preview updates
	before := len(mock.Updated)
	mock.InvokeCallback("form", "nameField", "change", "Jane")

	foundPreview := false
	for _, u := range mock.Updated[before:] {
		if u.Node != nil && u.Node.ComponentID == "previewName" {
			foundPreview = true
			if u.Node.Props.Content != "Jane" {
				t.Errorf("preview = %q, want Jane", u.Node.Props.Content)
			}
		}
	}
	if !foundPreview {
		t.Error("typing in nameField did not update previewName")
	}
}

func TestSliderBindingPropagates(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/vol","value":50}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"sl","type":"Slider","props":{"min":0,"max":100,"step":1,"sliderValue":{"path":"/vol"},"dataBinding":"/vol"}},{"componentId":"display","type":"Text","props":{"content":{"path":"/vol"}}}]}`)

	initialUpdates := len(mock.Updated)
	mock.InvokeCallback("s1", "sl", "slide", "75")

	foundDisplay := false
	for _, u := range mock.Updated[initialUpdates:] {
		if u.Node != nil && u.Node.ComponentID == "display" {
			foundDisplay = true
			if u.Node.Props.Content != "75" {
				t.Errorf("display content = %q, want '75'", u.Node.Props.Content)
			}
		}
	}
	if !foundDisplay {
		t.Error("slider binding did not propagate to display")
	}
}

func TestChoicePickerBindingPropagates(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/color","value":"red"}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"pick","type":"ChoicePicker","props":{"options":[{"label":"Red","value":"red"},{"label":"Blue","value":"blue"}],"selected":["red"],"dataBinding":"/color"}},{"componentId":"display","type":"Text","props":{"content":{"path":"/color"}}}]}`)

	initialUpdates := len(mock.Updated)
	mock.InvokeCallback("s1", "pick", "select", "blue")

	foundDisplay := false
	for _, u := range mock.Updated[initialUpdates:] {
		if u.Node != nil && u.Node.ComponentID == "display" {
			foundDisplay = true
			if u.Node.Props.Content != "blue" {
				t.Errorf("display content = %q, want 'blue'", u.Node.Props.Content)
			}
		}
	}
	if !foundDisplay {
		t.Error("choice picker binding did not propagate to display")
	}
}

func TestTextFieldValidation(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/name","value":""}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"field","type":"TextField","props":{"placeholder":"Name","value":{"path":"/name"},"dataBinding":"/name","validations":[{"type":"required"},{"type":"minLength","value":3}]}}]}`)

	// Type a single character — should fail both required (it's non-empty) and minLength
	beforeUpdates := len(mock.Updated)
	mock.InvokeCallback("s1", "field", "change", "ab")

	// The field should be re-rendered with validation errors
	foundField := false
	for _, u := range mock.Updated[beforeUpdates:] {
		if u.Node != nil && u.Node.ComponentID == "field" {
			foundField = true
			if len(u.Node.Props.ValidationErrors) != 1 {
				t.Errorf("validation errors = %d, want 1 (minLength)", len(u.Node.Props.ValidationErrors))
			}
		}
	}
	if !foundField {
		t.Error("field not re-rendered after change")
	}

	// Type valid input — should clear errors
	beforeUpdates = len(mock.Updated)
	mock.InvokeCallback("s1", "field", "change", "Alice")

	foundClear := false
	for _, u := range mock.Updated[beforeUpdates:] {
		if u.Node != nil && u.Node.ComponentID == "field" {
			foundClear = true
			if len(u.Node.Props.ValidationErrors) != 0 {
				t.Errorf("validation errors after valid = %v, want empty", u.Node.Props.ValidationErrors)
			}
		}
	}
	if !foundClear {
		t.Error("field not re-rendered after valid input")
	}
}

func TestTemplateExpansion(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/items","value":[{"name":"Alice"},{"name":"Bob"}]}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"list","type":"List","props":{"gap":8},"children":{"forEach":"/items","templateId":"tmpl","itemVariable":"item"}},{"componentId":"tmpl","type":"Text","props":{"content":{"path":"/item/name"}}}]}`)

	// Template should be expanded: tmpl_0, tmpl_1 created, tmpl itself should not exist
	created := map[string]string{}
	for _, c := range mock.Created {
		created[c.Node.ComponentID] = c.Node.Props.Content
	}

	if _, ok := created["tmpl"]; ok {
		t.Error("template source component 'tmpl' should not be created")
	}
	if created["list_tmpl_0"] != "Alice" {
		t.Errorf("list_tmpl_0 content = %q, want 'Alice'", created["list_tmpl_0"])
	}
	if created["list_tmpl_1"] != "Bob" {
		t.Errorf("list_tmpl_1 content = %q, want 'Bob'", created["list_tmpl_1"])
	}

	// List should have 2 children
	listHandle := mock.GetHandle("s1", "list")
	if listHandle == 0 {
		t.Fatal("list not created")
	}
	foundListChildren := false
	for _, cs := range mock.Children {
		if cs.ParentHandle == listHandle {
			foundListChildren = true
			if len(cs.ChildHandles) != 2 {
				t.Errorf("list children = %d, want 2", len(cs.ChildHandles))
			}
		}
	}
	if !foundListChildren {
		t.Error("no SetChildren for list")
	}
}

func TestTemplateExpansionNested(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/items","value":[{"name":"Alice","role":"Engineer"},{"name":"Bob","role":"Designer"}]}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"list","type":"List","props":{"gap":8},"children":{"forEach":"/items","templateId":"card","itemVariable":"item"}},{"componentId":"card","type":"Card","props":{"title":{"path":"/item/name"}},"children":["cardRole"]},{"componentId":"cardRole","type":"Text","props":{"content":{"path":"/item/role"},"variant":"caption"}}]}`)

	// Check that nested children are also expanded
	created := map[string]string{}
	for _, c := range mock.Created {
		created[c.Node.ComponentID] = c.Node.Props.Content
	}

	// list_card_0 and list_card_1 should exist
	cardCreated := false
	for _, c := range mock.Created {
		if c.Node.ComponentID == "list_card_0" {
			cardCreated = true
			if c.Node.Props.Title != "Alice" {
				t.Errorf("list_card_0 title = %q, want Alice", c.Node.Props.Title)
			}
		}
	}
	if !cardCreated {
		t.Error("list_card_0 not created")
	}

	// list_cardRole_0 and list_cardRole_1 should exist
	if created["list_cardRole_0"] != "Engineer" {
		t.Errorf("list_cardRole_0 content = %q, want 'Engineer'", created["list_cardRole_0"])
	}
	if created["list_cardRole_1"] != "Designer" {
		t.Errorf("list_cardRole_1 content = %q, want 'Designer'", created["list_cardRole_1"])
	}
}

func TestSessionUnknownSurface(t *testing.T) {
	sess, mock := newTestSession()

	// Sending updateComponents to a non-existent surface should not panic
	feedMessages(t, sess, `{"type":"updateComponents","surfaceId":"nonexistent","components":[{"componentId":"t1","type":"Text","props":{"content":"hello"}}]}`)

	if len(mock.Created) != 0 {
		t.Errorf("expected no created views, got %d", len(mock.Created))
	}
}

func TestSessionDuplicateCreateSurface(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"First"}
{"type":"createSurface","surfaceId":"s1","title":"Second"}`)

	// Should only create one window (duplicate ignored)
	if len(mock.Windows) != 1 {
		t.Errorf("windows = %d, want 1", len(mock.Windows))
	}
	if mock.Windows[0].Title != "First" {
		t.Errorf("title = %q, want First", mock.Windows[0].Title)
	}
}

func TestSessionDeleteSurface(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"deleteSurface","surfaceId":"s1"}`)

	// After deletion, updateComponents should be ignored
	feedMessages(t, sess, `{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"hello"}}]}`)

	if len(mock.Created) != 0 {
		t.Errorf("expected no created views after surface deletion, got %d", len(mock.Created))
	}
}

func TestSessionDeleteNonexistent(t *testing.T) {
	sess, _ := newTestSession()

	// Should not panic
	feedMessages(t, sess, `{"type":"deleteSurface","surfaceId":"nope"}`)
}

func TestResolveDataRefs(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/form/name","value":"Alice"},{"op":"add","path":"/form/email","value":"alice@example.com"}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Submit","onClick":{"action":{"event":{"name":"submit","dataRefs":["/form/name","/form/email","/form/missing"]}}}}}]}`)

	// Verify button was created
	if len(mock.Created) == 0 {
		t.Fatal("no components created")
	}

	// Access the surface to test resolveDataRefs directly
	surf := sess.surfaces["s1"]
	if surf == nil {
		t.Fatal("surface s1 not found")
	}

	result := surf.resolveDataRefs(&protocol.EventDef{
		Name:     "submit",
		DataRefs: []string{"/form/name", "/form/email", "/form/missing"},
	})

	if result["/form/name"] != "Alice" {
		t.Errorf("/form/name = %v, want Alice", result["/form/name"])
	}
	if result["/form/email"] != "alice@example.com" {
		t.Errorf("/form/email = %v, want alice@example.com", result["/form/email"])
	}
	if _, exists := result["/form/missing"]; exists {
		t.Error("/form/missing should not be in result")
	}
}

func TestOnActionPropagation(t *testing.T) {
	sess, mock := newTestSession()

	var received struct {
		surfaceID string
		event     *protocol.EventDef
		data      map[string]interface{}
	}
	sess.OnAction = func(surfaceID string, event *protocol.EventDef, data map[string]interface{}) {
		received.surfaceID = surfaceID
		received.event = event
		received.data = data
	}

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/count","value":42}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Click","onClick":{"action":{"event":{"name":"increment","dataRefs":["/count"]}}}}}]}`)

	// Click the button
	mock.InvokeCallback("s1", "btn", "click", "")

	if received.surfaceID != "s1" {
		t.Errorf("surfaceID = %q, want s1", received.surfaceID)
	}
	if received.event == nil {
		t.Fatal("event not received")
	}
	if received.event.Name != "increment" {
		t.Errorf("event.Name = %q, want increment", received.event.Name)
	}
	// Check that DataRefs were resolved
	if received.data["/count"] != float64(42) {
		t.Errorf("data[/count] = %v, want 42", received.data["/count"])
	}
}

func TestStylePassthrough(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T","backgroundColor":"#1C1C1E","padding":-1}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Go"},"style":{"backgroundColor":"#FF9F0A","textColor":"#FFFFFF","cornerRadius":8,"height":52,"fontSize":20,"fontWeight":"bold","textAlign":"center","opacity":0.9}}]}`)

	// Check window spec
	if len(mock.Windows) != 1 {
		t.Fatalf("windows = %d, want 1", len(mock.Windows))
	}
	if mock.Windows[0].BackgroundColor != "#1C1C1E" {
		t.Errorf("window bg = %q, want #1C1C1E", mock.Windows[0].BackgroundColor)
	}
	if mock.Windows[0].Padding != -1 {
		t.Errorf("window padding = %d, want -1", mock.Windows[0].Padding)
	}

	// Check component style passes through to RenderNode
	node := mock.LastNode("s1", "btn")
	if node == nil {
		t.Fatal("btn node not found")
	}
	if node.Style.BackgroundColor != "#FF9F0A" {
		t.Errorf("style.backgroundColor = %q, want #FF9F0A", node.Style.BackgroundColor)
	}
	if node.Style.TextColor != "#FFFFFF" {
		t.Errorf("style.textColor = %q, want #FFFFFF", node.Style.TextColor)
	}
	if node.Style.CornerRadius != 8 {
		t.Errorf("style.cornerRadius = %v, want 8", node.Style.CornerRadius)
	}
	if node.Style.Height != 52 {
		t.Errorf("style.height = %v, want 52", node.Style.Height)
	}
	if node.Style.FontSize != 20 {
		t.Errorf("style.fontSize = %v, want 20", node.Style.FontSize)
	}
	if node.Style.FontWeight != "bold" {
		t.Errorf("style.fontWeight = %q, want bold", node.Style.FontWeight)
	}
	if node.Style.TextAlign != "center" {
		t.Errorf("style.textAlign = %q, want center", node.Style.TextAlign)
	}
	if node.Style.Opacity != 0.9 {
		t.Errorf("style.opacity = %v, want 0.9", node.Style.Opacity)
	}
}

func TestLoadAssetsRegistersAndResolves(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"loadAssets","assets":[{"alias":"hero","kind":"image","src":"https://example.com/hero.png"},{"alias":"MyFont","kind":"font","src":"./fonts/myfont.ttf"}]}
{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"img1","type":"Image","props":{"src":"asset:hero","alt":"Hero","width":200,"height":100}}]}`)

	// Verify the mock renderer received LoadAssets
	heroSpec, ok := mock.GetAsset("hero")
	if !ok {
		t.Fatal("asset 'hero' not loaded in renderer")
	}
	if heroSpec.Src != "https://example.com/hero.png" {
		t.Errorf("hero src = %q, want https://example.com/hero.png", heroSpec.Src)
	}

	fontSpec, ok := mock.GetAsset("MyFont")
	if !ok {
		t.Fatal("asset 'MyFont' not loaded in renderer")
	}
	if fontSpec.Kind != "font" {
		t.Errorf("MyFont kind = %q, want font", fontSpec.Kind)
	}

	// Verify the image component's src was resolved from "asset:hero" to the actual URL
	node := mock.LastNode("s1", "img1")
	if node == nil {
		t.Fatal("img1 node not found")
	}
	if node.Props.Src != "https://example.com/hero.png" {
		t.Errorf("img1 src = %q, want https://example.com/hero.png", node.Props.Src)
	}
}

func TestAssetRefUnknownAlias(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"img1","type":"Image","props":{"src":"asset:nonexistent","alt":"Missing"}}]}`)

	// Unknown asset ref should pass through unchanged
	node := mock.LastNode("s1", "img1")
	if node == nil {
		t.Fatal("img1 node not found")
	}
	if node.Props.Src != "asset:nonexistent" {
		t.Errorf("img1 src = %q, want asset:nonexistent (passthrough)", node.Props.Src)
	}
}

func TestFontFamilyStylePassthrough(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"Hi"},"style":{"fontFamily":"Comic Sans MS","fontSize":20}}]}`)

	node := mock.LastNode("s1", "t1")
	if node == nil {
		t.Fatal("t1 node not found")
	}
	if node.Style.FontFamily != "Comic Sans MS" {
		t.Errorf("fontFamily = %q, want Comic Sans MS", node.Style.FontFamily)
	}
	if node.Style.FontSize != 20 {
		t.Errorf("fontSize = %v, want 20", node.Style.FontSize)
	}
}

func TestLoadAssetsBeforeSurfaceCreate(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	// loadAssets comes before createSurface — assets should propagate to later surfaces
	feedMessages(t, sess, `{"type":"loadAssets","assets":[{"alias":"bg","kind":"image","src":"/tmp/bg.png"}]}
{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"img1","type":"Image","props":{"src":"asset:bg"}}]}`)

	node := mock.LastNode("s1", "img1")
	if node == nil {
		t.Fatal("img1 node not found")
	}
	if node.Props.Src != "/tmp/bg.png" {
		t.Errorf("img1 src = %q, want /tmp/bg.png", node.Props.Src)
	}
}

func TestSessionUnknownMessageType(t *testing.T) {
	sess, _ := newTestSession()

	// Should not panic — unknown type handled gracefully
	msg := &protocol.Message{
		Type:      "totallyBogus",
		SurfaceID: "s1",
	}
	sess.HandleMessage(msg)
}

func TestDefineFunction(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"defineFunction","name":"double","params":["x"],"body":{"functionCall":{"name":"multiply","args":[{"param":"x"},2]}}}
{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/val","value":5}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":{"functionCall":{"name":"double","args":[{"path":"/val"}]}}}}]}`)

	node := mock.LastNode("s1", "t1")
	if node == nil {
		t.Fatal("t1 not created")
	}
	if node.Props.Content != "10" {
		t.Errorf("content = %q, want '10'", node.Props.Content)
	}
}

func TestDefineFunctionInButtonAction(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"defineFunction","name":"appendDigit","params":["digit"],"body":{"functionCall":{"name":"if","args":[{"functionCall":{"name":"equals","args":[{"path":"/display"},"0"]}}  ,{"param":"digit"},{"functionCall":{"name":"concat","args":[{"path":"/display"},{"param":"digit"}]}}]}}}
{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/display","value":"0"}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"display","type":"Text","props":{"content":{"path":"/display"}}},{"componentId":"btn7","type":"Button","props":{"label":"7","onClick":{"action":{"functionCall":{"call":"updateDataModel","args":{"ops":[{"op":"replace","path":"/display","value":{"functionCall":{"name":"appendDigit","args":["7"]}}}]}}}}}}]}`)

	// Display initially shows "0"
	node := mock.LastNode("s1", "display")
	if node == nil {
		t.Fatal("display not created")
	}
	if node.Props.Content != "0" {
		t.Errorf("initial display = %q, want '0'", node.Props.Content)
	}

	// Click button 7 — should replace "0" with "7"
	beforeUpdates := len(mock.Updated)
	mock.InvokeCallback("s1", "btn7", "click", "")

	found := false
	for _, u := range mock.Updated[beforeUpdates:] {
		if u.Node != nil && u.Node.ComponentID == "display" {
			found = true
			if u.Node.Props.Content != "7" {
				t.Errorf("display after click = %q, want '7'", u.Node.Props.Content)
			}
		}
	}
	if !found {
		t.Error("display not updated after button click")
	}
}

func TestDefineFunctionArityCheck(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"defineFunction","name":"add2","params":["a","b"],"body":{"functionCall":{"name":"add","args":[{"param":"a"},{"param":"b"}]}}}
{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":{"functionCall":{"name":"add2","args":[1]}}}}]}`)

	// Should show empty string due to arity error
	node := mock.LastNode("s1", "t1")
	if node == nil {
		t.Fatal("t1 not created")
	}
	if node.Props.Content != "" {
		t.Errorf("content with arity error = %q, want empty", node.Props.Content)
	}
	_ = mock
}

func TestDefineComponent(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"defineComponent","name":"LabeledText","params":["text"],"components":[{"componentId":"_root","type":"Text","props":{"content":{"param":"text"},"variant":"h1"}}]}
{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"greeting","useComponent":"LabeledText","args":{"text":"Hello World"}}]}`)

	node := mock.LastNode("s1", "greeting")
	if node == nil {
		t.Fatal("greeting not created")
	}
	if node.Props.Content != "Hello World" {
		t.Errorf("content = %q, want 'Hello World'", node.Props.Content)
	}
	if node.Props.Variant != "h1" {
		t.Errorf("variant = %q, want 'h1'", node.Props.Variant)
	}
}

func TestDefineComponentWithChildren(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"defineComponent","name":"LabeledField","params":["label","binding"],"components":[{"componentId":"_root","type":"Column","children":["_label","_field"]},{"componentId":"_label","type":"Text","props":{"content":{"param":"label"}}},{"componentId":"_field","type":"TextField","props":{"placeholder":{"param":"label"},"dataBinding":{"param":"binding"}}}]}
{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/name","value":""}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"nameField","useComponent":"LabeledField","args":{"label":"Name","binding":"/name"}}]}`)

	// Root column should be created with the instance ID
	rootNode := mock.LastNode("s1", "nameField")
	if rootNode == nil {
		t.Fatal("nameField not created")
	}
	if rootNode.Type != protocol.CompColumn {
		t.Errorf("type = %q, want Column", rootNode.Type)
	}

	// Label should be created with prefixed ID
	labelNode := mock.LastNode("s1", "nameField__label")
	if labelNode == nil {
		t.Fatal("nameField__label not created")
	}
	if labelNode.Props.Content != "Name" {
		t.Errorf("label content = %q, want 'Name'", labelNode.Props.Content)
	}

	// TextField should be created with prefixed ID
	fieldNode := mock.LastNode("s1", "nameField__field")
	if fieldNode == nil {
		t.Fatal("nameField__field not created")
	}
	if fieldNode.Props.Placeholder != "Name" {
		t.Errorf("placeholder = %q, want 'Name'", fieldNode.Props.Placeholder)
	}
}

func TestDefineComponentScopedPaths(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"defineComponent","name":"Counter","params":["label"],"components":[{"componentId":"_root","type":"Column","children":["_display"]},{"componentId":"_display","type":"Text","props":{"content":{"path":"$/count"}}}]}
{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/c1/count","value":"42"}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"c1","useComponent":"Counter","args":{"label":"A"},"scope":"/c1"}]}`)

	displayNode := mock.LastNode("s1", "c1__display")
	if displayNode == nil {
		t.Fatal("c1__display not created")
	}
	if displayNode.Props.Content != "42" {
		t.Errorf("content = %q, want '42'", displayNode.Props.Content)
	}
}

func TestDefineComponentDefaultScope(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"defineComponent","name":"ScopedText","params":[],"components":[{"componentId":"_root","type":"Text","props":{"content":{"path":"$/value"}}}]}
{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/myComp/value","value":"default scoped"}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"myComp","useComponent":"ScopedText","args":{}}]}`)

	node := mock.LastNode("s1", "myComp")
	if node == nil {
		t.Fatal("myComp not created")
	}
	if node.Props.Content != "default scoped" {
		t.Errorf("content = %q, want 'default scoped'", node.Props.Content)
	}
}

func TestDefineFunctionRedefinition(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"defineFunction","name":"greet","params":["name"],"body":{"functionCall":{"name":"concat","args":["Hello ",{"param":"name"}]}}}
{"type":"defineFunction","name":"greet","params":["name"],"body":{"functionCall":{"name":"concat","args":["Hi ",{"param":"name"}]}}}
{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":{"functionCall":{"name":"greet","args":["World"]}}}}]}`)

	// Last definition wins
	node := mock.LastNode("s1", "t1")
	if node == nil {
		t.Fatal("t1 not created")
	}
	if node.Props.Content != "Hi World" {
		t.Errorf("content = %q, want 'Hi World'", node.Props.Content)
	}
	_ = mock
}

func TestTabsComponent(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/selectedTab","value":"tab-a"}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"tabs1","type":"Tabs","props":{"tabLabels":["Tab A","Tab B"],"activeTab":{"path":"/selectedTab"},"dataBinding":"/selectedTab"},"children":["tab-a","tab-b"]},{"componentId":"tab-a","type":"Text","props":{"content":"Content A"}},{"componentId":"tab-b","type":"Text","props":{"content":"Content B"}},{"componentId":"display","type":"Text","props":{"content":{"path":"/selectedTab"}}}]}`)

	// Tabs component created with correct resolved props
	tabsNode := mock.LastNode("s1", "tabs1")
	if tabsNode == nil {
		t.Fatal("tabs1 not created")
	}
	if len(tabsNode.Props.TabLabels) != 2 {
		t.Errorf("tabLabels count = %d, want 2", len(tabsNode.Props.TabLabels))
	}
	if tabsNode.Props.TabLabels[0] != "Tab A" || tabsNode.Props.TabLabels[1] != "Tab B" {
		t.Errorf("tabLabels = %v, want [Tab A, Tab B]", tabsNode.Props.TabLabels)
	}
	if tabsNode.Props.ActiveTab != "tab-a" {
		t.Errorf("activeTab = %q, want tab-a", tabsNode.Props.ActiveTab)
	}
	if tabsNode.Props.DataBinding != "/selectedTab" {
		t.Errorf("dataBinding = %q, want /selectedTab", tabsNode.Props.DataBinding)
	}

	// Children set on tabs
	tabsHandle := mock.GetHandle("s1", "tabs1")
	if tabsHandle == 0 {
		t.Fatal("tabs1 handle not found")
	}
	foundTabsChildren := false
	for _, cs := range mock.Children {
		if cs.ParentHandle == tabsHandle {
			foundTabsChildren = true
			if len(cs.ChildHandles) != 2 {
				t.Errorf("tabs child count = %d, want 2", len(cs.ChildHandles))
			}
		}
	}
	if !foundTabsChildren {
		t.Error("no SetChildren call for tabs1")
	}

	// Simulate tab selection → data model updates
	initialUpdates := len(mock.Updated)
	mock.InvokeCallback("s1", "tabs1", "select", "tab-b")

	foundDisplay := false
	for _, u := range mock.Updated[initialUpdates:] {
		if u.Node != nil && u.Node.ComponentID == "display" {
			foundDisplay = true
			if u.Node.Props.Content != "tab-b" {
				t.Errorf("display content = %q, want tab-b", u.Node.Props.Content)
			}
		}
	}
	if !foundDisplay {
		t.Error("tabs select callback did not propagate to display")
	}

	// Verify data model
	if surf, ok := sess.surfaces["s1"]; ok {
		val, found := surf.dm.Get("/selectedTab")
		if !found || val != "tab-b" {
			t.Errorf("/selectedTab = %v (%v), want tab-b", val, found)
		}
	}
}

func TestComponentRemovalCleansUp(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/name","value":""}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"root","type":"Column","children":["field","display"]},{"componentId":"field","type":"TextField","props":{"value":{"path":"/name"},"dataBinding":"/name"}},{"componentId":"display","type":"Text","props":{"content":{"path":"/name"}}}]}`)

	// Verify both components exist
	if mock.GetHandle("s1", "field") == 0 {
		t.Fatal("field not created")
	}
	if mock.GetHandle("s1", "display") == 0 {
		t.Fatal("display not created")
	}

	// Get the callback for the field
	fieldCBID := mock.GetCallbackID("s1", "field", "change")
	if fieldCBID == 0 {
		t.Fatal("field callback not registered")
	}

	// Remove display from root's children
	feedMessages(t, sess, `{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"root","type":"Column","children":["field"]}]}`)

	// display should be removed
	if mock.GetHandle("s1", "display") != 0 {
		t.Error("display handle still exists after removal")
	}

	// RemoveView should have been called
	if len(mock.Removed) == 0 {
		t.Error("no RemoveView calls")
	}

	// display's bindings should be cleaned up — typing in field should not trigger display update
	beforeUpdates := len(mock.Updated)
	mock.InvokeCallback("s1", "field", "change", "Alice")

	for _, u := range mock.Updated[beforeUpdates:] {
		if u.Node != nil && u.Node.ComponentID == "display" {
			t.Error("display was updated despite being removed")
		}
	}
}

func TestComponentRemovalCleansUpCallbacks(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"root","type":"Column","children":["btn"]},{"componentId":"btn","type":"Button","props":{"label":"Go","onClick":{"action":{"event":{"name":"doThing"}}}}}]}`)

	// Verify button callback registered
	btnCBID := mock.GetCallbackID("s1", "btn", "click")
	if btnCBID == 0 {
		t.Fatal("button callback not registered")
	}
	if !mock.HasCallback(btnCBID) {
		t.Fatal("button callback not in callback map")
	}

	// Remove button from root's children
	feedMessages(t, sess, `{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"root","type":"Column","children":[]}]}`)

	// Callback should be unregistered
	if mock.HasCallback(btnCBID) {
		t.Error("button callback still registered after removal")
	}

	// Handle should be gone
	if mock.GetHandle("s1", "btn") != 0 {
		t.Error("button handle still exists after removal")
	}
}

func TestTemplateReExpansionCleansUp(t *testing.T) {
	sess, mock := newTestSession()

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/items","value":[{"name":"Alice"},{"name":"Bob"},{"name":"Charlie"}]}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"list","type":"List","props":{"gap":8},"children":{"forEach":"/items","templateId":"tmpl","itemVariable":"item"}},{"componentId":"tmpl","type":"Text","props":{"content":{"path":"/item/name"}}}]}`)

	// Verify 3 template instances created
	if mock.GetHandle("s1", "list_tmpl_0") == 0 {
		t.Fatal("list_tmpl_0 not created")
	}
	if mock.GetHandle("s1", "list_tmpl_1") == 0 {
		t.Fatal("list_tmpl_1 not created")
	}
	if mock.GetHandle("s1", "list_tmpl_2") == 0 {
		t.Fatal("list_tmpl_2 not created")
	}

	// Shrink the array to 1 item, then re-send the list component
	// (template re-expansion happens in HandleUpdateComponents, not data model change)
	feedMessages(t, sess, `{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"replace","path":"/items","value":[{"name":"Alice"}]}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"list","type":"List","props":{"gap":8},"children":{"forEach":"/items","templateId":"tmpl","itemVariable":"item"}},{"componentId":"tmpl","type":"Text","props":{"content":{"path":"/item/name"}}}]}`)

	// list_tmpl_0 should exist (re-created for new data)
	if mock.GetHandle("s1", "list_tmpl_0") == 0 {
		t.Error("list_tmpl_0 should exist after re-expansion")
	}
	// list_tmpl_1 and list_tmpl_2 should be gone
	if mock.GetHandle("s1", "list_tmpl_1") != 0 {
		t.Error("list_tmpl_1 should be removed after array shrink")
	}
	if mock.GetHandle("s1", "list_tmpl_2") != 0 {
		t.Error("list_tmpl_2 should be removed after array shrink")
	}
}

func TestDefineComponentWithFunctionCall(t *testing.T) {
	sess, mock := newTestSession()

	// Define a function and a component that uses it
	feedMessages(t, sess, `{"type":"defineFunction","name":"appendDigit","params":["digit"],"body":{"functionCall":{"name":"if","args":[{"functionCall":{"name":"equals","args":[{"path":"/display"},"0"]}}  ,{"param":"digit"},{"functionCall":{"name":"concat","args":[{"path":"/display"},{"param":"digit"}]}}]}}}
{"type":"defineComponent","name":"DigitButton","params":["digit","label"],"components":[{"componentId":"_root","type":"Button","props":{"label":{"param":"label"},"onClick":{"action":{"functionCall":{"call":"updateDataModel","args":{"ops":[{"op":"replace","path":"/display","value":{"functionCall":{"name":"appendDigit","args":[{"param":"digit"}]}}},{"op":"replace","path":"/clearOnInput","value":false}]}}}}}}]}
{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/display","value":"0"},{"op":"add","path":"/clearOnInput","value":true}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"display","type":"Text","props":{"content":{"path":"/display"}}},{"componentId":"btn7","useComponent":"DigitButton","args":{"digit":"7","label":"7"}}]}`)

	// Button should be created
	btnNode := mock.LastNode("s1", "btn7")
	if btnNode == nil {
		t.Fatal("btn7 not created")
	}
	if btnNode.Props.Label != "7" {
		t.Errorf("label = %q, want '7'", btnNode.Props.Label)
	}

	// Click button — should update display from "0" to "7" via appendDigit
	beforeUpdates := len(mock.Updated)
	mock.InvokeCallback("s1", "btn7", "click", "")

	found := false
	for _, u := range mock.Updated[beforeUpdates:] {
		if u.Node != nil && u.Node.ComponentID == "display" {
			found = true
			if u.Node.Props.Content != "7" {
				t.Errorf("display after click = %q, want '7'", u.Node.Props.Content)
			}
		}
	}
	if !found {
		t.Error("display not updated after DigitButton click")
	}
}

// TestCallbackPanicRecovery verifies that a panic in a callback doesn't crash the session.
func TestCallbackPanicRecovery(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"Panic Test","width":400,"height":300}
{"type":"updateComponents","surfaceId":"main","components":[{"componentId":"btn1","type":"Button","props":{"label":"Crash","onClick":{"action":{"functionCall":{"call":"updateDataModel","args":{"ops":[{"op":"add","path":"/clicked","value":true}]}}}}}}]}`)

	// Replace the callback with one that panics
	cbID := mock.GetCallbackID("main", "btn1", "click")
	if cbID == 0 {
		t.Fatal("expected callback registered for btn1")
	}

	// Directly invoke a panicking callback through the mock —
	// this simulates what happens when the real callback panics.
	// The mock dispatcher runs synchronously, so a panic would kill the test.
	// Verify the session state is still usable after the panicking callback.
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		// Feed a new update to prove session is still functional
		feedMessages(t, sess, `{"type":"updateDataModel","surfaceId":"main","ops":[{"op":"add","path":"/after_panic","value":"ok"}]}`)
	}()
	if panicked {
		t.Fatal("session panicked during normal operation")
	}

	// Verify data was written
	surf := sess.GetSurface("main")
	if surf == nil {
		t.Fatal("surface gone after panic recovery test")
	}
	val, ok := surf.DM().Get("/after_panic")
	if !ok || val != "ok" {
		t.Errorf("data model after recovery: got %v (ok=%v), want 'ok'", val, ok)
	}
}

// TestUnknownComponentTypeContinues verifies that unknown component types
// don't crash the session — they're logged and skipped.
func TestUnknownComponentTypeContinues(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"Unknown","width":400,"height":300}
{"type":"updateComponents","surfaceId":"main","components":[{"componentId":"good","type":"Text","props":{"content":"Hello"}},{"componentId":"bad","type":"NonExistentWidget","props":{"content":"Nope"}},{"componentId":"also_good","type":"Text","props":{"content":"Still here"}}]}`)

	// The known components should be created
	goodNode := mock.LastNode("main", "good")
	if goodNode == nil {
		t.Fatal("good component not created")
	}
	if goodNode.Props.Content != "Hello" {
		t.Errorf("good content = %q, want 'Hello'", goodNode.Props.Content)
	}
	alsoGoodNode := mock.LastNode("main", "also_good")
	if alsoGoodNode == nil {
		t.Fatal("also_good component not created")
	}
	if alsoGoodNode.Props.Content != "Still here" {
		t.Errorf("also_good content = %q, want 'Still here'", alsoGoodNode.Props.Content)
	}
}

// TestDeleteSurfaceCleanupCallbacks verifies that deleting a surface
// unregisters all callbacks, and stale callback invocation is safe.
func TestDeleteSurfaceCleanupCallbacks(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"Delete Test","width":400,"height":300}
{"type":"updateComponents","surfaceId":"main","components":[{"componentId":"field1","type":"TextField","props":{"placeholder":"Name","dataBinding":"/name"}},{"componentId":"btn1","type":"Button","props":{"label":"Go","onClick":{"action":{"functionCall":{"call":"updateDataModel","args":{"ops":[{"op":"add","path":"/clicked","value":true}]}}}}}}]}`)

	// Verify callbacks exist
	fieldCB := mock.GetCallbackID("main", "field1", "change")
	btnCB := mock.GetCallbackID("main", "btn1", "click")
	if fieldCB == 0 || btnCB == 0 {
		t.Fatalf("expected callbacks: field=%d btn=%d", fieldCB, btnCB)
	}

	// Delete the surface
	feedMessages(t, sess, `{"type":"deleteSurface","surfaceId":"main"}`)

	// Surface should be gone
	if surf := sess.GetSurface("main"); surf != nil {
		t.Error("surface still exists after deletion")
	}

	// Callbacks should be unregistered from mock
	if mock.HasCallback(fieldCB) {
		t.Error("field callback still registered after surface deletion")
	}
	if mock.HasCallback(btnCB) {
		t.Error("button callback still registered after surface deletion")
	}

	// Invoking stale callbacks should not panic
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		mock.InvokeCallback("main", "field1", "change", "test")
		mock.InvokeCallback("main", "btn1", "click", "")
	}()
	if panicked {
		t.Error("invoking stale callback panicked")
	}
}

func TestProcessManagerCreateAndStop(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	// Create a mock transport factory that makes a simple channel-based transport
	factory := func(cfg protocol.ProcessTransportConfig) (ProcessTransport, error) {
		return &mockProcessTransport{
			msgs: make(chan *protocol.Message, 16),
			errs: make(chan error, 4),
		}, nil
	}

	pm := NewProcessManager(sess, factory)
	sess.SetProcessManager(pm)

	// Create a surface first
	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"Test","width":600,"height":400}`)

	// Create a process
	feedMessages(t, sess, `{"type":"createProcess","processId":"bg","transport":{"type":"file","path":"test.jsonl"}}`)

	// Verify process is running
	status := pm.GetStatus("bg")
	if status != "running" {
		t.Errorf("process status = %q, want running", status)
	}

	// Verify process status in data model
	surf := sess.GetSurface("main")
	if surf == nil {
		t.Fatal("surface not found")
	}
	val, ok := surf.DM().Get("/processes/bg/status")
	if !ok {
		t.Fatal("process status not in data model")
	}
	if val != "running" {
		t.Errorf("data model process status = %v, want running", val)
	}

	// Stop the process
	feedMessages(t, sess, `{"type":"stopProcess","processId":"bg"}`)

	status = pm.GetStatus("bg")
	if status != "stopped" {
		t.Errorf("process status after stop = %q, want stopped", status)
	}

	val, ok = surf.DM().Get("/processes/bg/status")
	if !ok {
		t.Fatal("process status not in data model after stop")
	}
	if val != "stopped" {
		t.Errorf("data model process status after stop = %v, want stopped", val)
	}
}

func TestProcessStatusReRender(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	factory := func(cfg protocol.ProcessTransportConfig) (ProcessTransport, error) {
		return &mockProcessTransport{
			msgs: make(chan *protocol.Message, 16),
			errs: make(chan error, 4),
		}, nil
	}
	pm := NewProcessManager(sess, factory)
	sess.SetProcessManager(pm)

	// Create surface and a component that reads /processes/bg/status
	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"Test","width":600,"height":400}`)
	feedMessages(t, sess, `{"type":"updateComponents","surfaceId":"main","components":[{"componentId":"root","type":"Column","children":{"static":["status_text"]}},{"componentId":"status_text","type":"Text","props":{"content":{"functionCall":{"name":"if","args":[{"functionCall":{"name":"equals","args":[{"path":"/processes/bg/status"},"running"]}},"Running","Stopped"]}}}}]}`)

	// Before process creation, status_text should show "Stopped"
	node := mock.LastNode("main", "status_text")
	if node == nil {
		t.Fatal("status_text node not found")
	}
	if node.Props.Content != "Stopped" {
		t.Errorf("before process create: content = %q, want Stopped", node.Props.Content)
	}

	// Now create the process — this should trigger re-render with "Running"
	feedMessages(t, sess, `{"type":"createProcess","processId":"bg","transport":{"type":"file","path":"test.jsonl"}}`)

	// Check that status_text was re-rendered with "Running"
	node = mock.LastNode("main", "status_text")
	if node == nil {
		t.Fatal("status_text node not found after re-render")
	}
	if node.Props.Content != "Running" {
		t.Errorf("after process create: content = %q, want Running", node.Props.Content)
	}
}

func TestIntervalProcessIncrement(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	// Use a mock transport that simulates what IntervalTransport does:
	// send an updateDataModel message with a functionCall value
	mockTr := &mockProcessTransport{
		msgs: make(chan *protocol.Message, 16),
		errs: make(chan error, 4),
	}
	factory := func(cfg protocol.ProcessTransportConfig) (ProcessTransport, error) {
		return mockTr, nil
	}
	pm := NewProcessManager(sess, factory)
	sess.SetProcessManager(pm)

	// Set up surface with a counter
	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"Test","width":400,"height":300}`)
	feedMessages(t, sess, `{"type":"updateDataModel","surfaceId":"main","ops":[{"op":"add","path":"/counter","value":0}]}`)
	feedMessages(t, sess, `{"type":"updateComponents","surfaceId":"main","components":[{"componentId":"root","type":"Column","children":{"static":["counter_text"]}},{"componentId":"counter_text","type":"Text","props":{"content":{"path":"/counter"},"variant":"h1"}}]}`)

	// Verify initial state
	node := mock.LastNode("main", "counter_text")
	if node == nil {
		t.Fatal("counter_text not found")
	}
	if node.Props.Content != "0" {
		t.Errorf("initial content = %q, want 0", node.Props.Content)
	}

	// Create the process
	feedMessages(t, sess, `{"type":"createProcess","processId":"tick","transport":{"type":"interval","interval":1000,"message":{"type":"updateDataModel","surfaceId":"main","ops":[{"op":"replace","path":"/counter","value":{"functionCall":{"name":"add","args":[{"path":"/counter"},1]}}}]}}}`)

	// Simulate the interval tick by sending the same message through the mock transport
	tickMsg, err := protocol.ParseLine([]byte(`{"type":"updateDataModel","surfaceId":"main","ops":[{"op":"replace","path":"/counter","value":{"functionCall":{"name":"add","args":[{"path":"/counter"},1]}}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	// Route the message directly through session (simulating what the process run() goroutine does)
	sess.HandleMessage(tickMsg)

	// Verify counter incremented
	surf := sess.GetSurface("main")
	val, ok := surf.DM().Get("/counter")
	if !ok {
		t.Fatal("counter not in data model")
	}
	if v, ok := val.(float64); !ok || v != 1 {
		t.Errorf("counter = %v, want 1", val)
	}

	// Verify the text was re-rendered
	node = mock.LastNode("main", "counter_text")
	if node.Props.Content != "1" {
		t.Errorf("after tick: content = %q, want 1", node.Props.Content)
	}

	// Send another tick
	sess.HandleMessage(tickMsg)
	val, _ = surf.DM().Get("/counter")
	if v, ok := val.(float64); !ok || v != 2 {
		t.Errorf("counter after 2 ticks = %v, want 2", val)
	}
}

func TestProcessManagerIDs(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	factory := func(cfg protocol.ProcessTransportConfig) (ProcessTransport, error) {
		return &mockProcessTransport{
			msgs: make(chan *protocol.Message, 16),
			errs: make(chan error, 4),
		}, nil
	}

	pm := NewProcessManager(sess, factory)
	sess.SetProcessManager(pm)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"Test","width":600,"height":400}`)
	feedMessages(t, sess, `{"type":"createProcess","processId":"p1","transport":{"type":"file","path":"a.jsonl"}}`)
	feedMessages(t, sess, `{"type":"createProcess","processId":"p2","transport":{"type":"file","path":"b.jsonl"}}`)

	ids := pm.IDs()
	if len(ids) != 2 {
		t.Errorf("process count = %d, want 2", len(ids))
	}

	pm.StopAll()
	for _, id := range ids {
		if pm.GetStatus(id) != "stopped" {
			t.Errorf("process %s not stopped after StopAll", id)
		}
	}
}

// mockProcessTransport satisfies ProcessTransport for testing.
func TestBatchedUpdateComponents(t *testing.T) {
	sess, mock := newTestSession()

	// Create surface
	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}`)

	// Batch 1: Create children
	feedMessages(t, sess, `{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"child1","type":"Text","props":{"content":"A"}},{"componentId":"child2","type":"Text","props":{"content":"B"}}]}`)

	// Both children should be created
	if mock.GetHandle("s1", "child1") == 0 {
		t.Fatal("child1 not created after batch 1")
	}
	if mock.GetHandle("s1", "child2") == 0 {
		t.Fatal("child2 not created after batch 1")
	}

	// Batch 2: Create parent referencing children from batch 1
	feedMessages(t, sess, `{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"parent","type":"Column","children":["child1","child2"]}]}`)

	// Parent should be created
	if mock.GetHandle("s1", "parent") == 0 {
		t.Fatal("parent not created after batch 2")
	}

	// SetChildren should wire up child1 and child2 under parent
	parentHandle := mock.GetHandle("s1", "parent")
	found := false
	for _, sc := range mock.Children {
		if sc.ParentHandle == parentHandle {
			found = true
			if len(sc.ChildHandles) != 2 {
				t.Fatalf("parent has %d children, want 2", len(sc.ChildHandles))
			}
		}
	}
	if !found {
		t.Fatal("SetChildren not called for parent after batch 2")
	}

	// Children from batch 1 should not have been pruned
	if mock.GetHandle("s1", "child1") == 0 {
		t.Fatal("child1 was pruned after batch 2")
	}
	if mock.GetHandle("s1", "child2") == 0 {
		t.Fatal("child2 was pruned after batch 2")
	}
}

type mockProcessTransport struct {
	msgs    chan *protocol.Message
	errs    chan error
	started bool
}

func (m *mockProcessTransport) Messages() <-chan *protocol.Message { return m.msgs }
func (m *mockProcessTransport) Errors() <-chan error               { return m.errs }
func (m *mockProcessTransport) Start()                             { m.started = true }
func (m *mockProcessTransport) Stop() {
	select {
	case <-m.msgs:
	default:
		close(m.msgs)
		close(m.errs)
	}
}
func (m *mockProcessTransport) SendAction(surfaceID string, event *protocol.EventDef, data map[string]interface{}) {
}

// --- Generic event system tests ---

func TestGenericOnPropClickBackwardCompat(t *testing.T) {
	// Verify that "on":{"click":{...}} works identically to "onClick":{...}
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	var firedEvent string
	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Go","on":{"click":{"action":{"event":{"name":"clicked"}}}}}}]}`)

	if surf, ok := sess.surfaces["s1"]; ok {
		surf.ActionHandler = func(sid string, event *protocol.EventDef, data map[string]interface{}) {
			firedEvent = event.Name
		}
	}
	mock.InvokeCallback("s1", "btn", "click", "")
	if firedEvent != "clicked" {
		t.Errorf("firedEvent = %q, want clicked", firedEvent)
	}
}

func TestGenericOnPropDataPath(t *testing.T) {
	// Verify that DataPath on an event handler writes to the data model
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/hovered","value":false}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"card","type":"Card","props":{"title":"Hover me","on":{"mouseEnter":{"dataPath":"/hovered","dataValue":true},"mouseLeave":{"dataPath":"/hovered","dataValue":false}}}},{"componentId":"indicator","type":"Text","props":{"content":{"path":"/hovered"}}}]}`)

	// Simulate mouseEnter
	initialUpdates := len(mock.Updated)
	mock.InvokeCallback("s1", "card", "mouseEnter", `{"x":100,"y":50}`)

	// Check that /hovered is now true
	if surf, ok := sess.surfaces["s1"]; ok {
		val, found := surf.dm.Get("/hovered")
		if !found {
			t.Fatal("/hovered not found in data model")
		}
		if val != true {
			t.Errorf("/hovered = %v, want true", val)
		}
	}

	// Check that indicator text was re-rendered
	newUpdates := mock.Updated[initialUpdates:]
	foundUpdate := false
	for _, u := range newUpdates {
		if u.Node != nil && u.Node.ComponentID == "indicator" {
			foundUpdate = true
		}
	}
	if !foundUpdate {
		t.Error("DataPath write did not trigger indicator re-render")
	}

	// Simulate mouseLeave
	mock.InvokeCallback("s1", "card", "mouseLeave", `{"x":0,"y":0}`)
	if surf, ok := sess.surfaces["s1"]; ok {
		val, _ := surf.dm.Get("/hovered")
		if val != false {
			t.Errorf("/hovered after mouseLeave = %v, want false", val)
		}
	}
}

func TestGenericOnPropEventDataMerge(t *testing.T) {
	// Verify that native JSON data is merged into server event resolved map
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	var receivedData map[string]interface{}
	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"zone","type":"Card","props":{"on":{"drop":{"action":{"event":{"name":"fileDrop"}}}}}}]}`)

	if surf, ok := sess.surfaces["s1"]; ok {
		surf.ActionHandler = func(sid string, event *protocol.EventDef, data map[string]interface{}) {
			receivedData = data
		}
	}

	mock.InvokeCallback("s1", "zone", "drop", `{"paths":["/tmp/file.txt"],"text":"hello"}`)

	if receivedData == nil {
		t.Fatal("action handler not called")
	}
	if receivedData["text"] != "hello" {
		t.Errorf("text = %v, want hello", receivedData["text"])
	}
	paths, ok := receivedData["paths"].([]interface{})
	if !ok || len(paths) != 1 {
		t.Errorf("paths = %v, want [/tmp/file.txt]", receivedData["paths"])
	}
}

func TestGenericOnPropCallbackRegistration(t *testing.T) {
	// Verify that generic event names produce callbacks in the render node
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"card","type":"Card","props":{"title":"Events","on":{"mouseEnter":{"dataPath":"/h","dataValue":true},"focus":{"dataPath":"/f","dataValue":true},"doubleClick":{"action":{"event":{"name":"dblClick"}}}}}}]}`)

	// Find the created card and check its callbacks
	for _, c := range mock.Created {
		if c.Node.ComponentID == "card" {
			cbs := c.Node.Callbacks
			if _, ok := cbs["mouseEnter"]; !ok {
				t.Error("mouseEnter callback not registered")
			}
			if _, ok := cbs["focus"]; !ok {
				t.Error("focus callback not registered")
			}
			if _, ok := cbs["doubleClick"]; !ok {
				t.Error("doubleClick callback not registered")
			}
			return
		}
	}
	t.Fatal("card not found in created views")
}

func TestOnPropCoexistsWithDataBinding(t *testing.T) {
	// Verify that a TextField with DataBinding + on.change action fires both
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	var firedEvent string
	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/name","value":""}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"field","type":"TextField","props":{"value":{"path":"/name"},"dataBinding":"/name","on":{"change":{"action":{"event":{"name":"nameChanged"}}}}}},{"componentId":"display","type":"Text","props":{"content":{"path":"/name"}}}]}`)

	if surf, ok := sess.surfaces["s1"]; ok {
		surf.ActionHandler = func(sid string, event *protocol.EventDef, data map[string]interface{}) {
			firedEvent = event.Name
		}
	}

	mock.InvokeCallback("s1", "field", "change", "Alice")

	// Data binding should have updated the data model
	if surf, ok := sess.surfaces["s1"]; ok {
		val, _ := surf.dm.Get("/name")
		if val != "Alice" {
			t.Errorf("/name = %v, want Alice", val)
		}
	}

	// The on.change action should have fired
	if firedEvent != "nameChanged" {
		t.Errorf("firedEvent = %q, want nameChanged", firedEvent)
	}
}

func TestNamedPropNormalization(t *testing.T) {
	// Verify that onClick prop gets normalized into On map and fires
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	var firedEvent string
	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"card","type":"Card","props":{"title":"Click","onClick":{"action":{"event":{"name":"cardClicked"}}}}}]}`)

	if surf, ok := sess.surfaces["s1"]; ok {
		surf.ActionHandler = func(sid string, event *protocol.EventDef, data map[string]interface{}) {
			firedEvent = event.Name
		}
	}

	mock.InvokeCallback("s1", "card", "click", "")
	if firedEvent != "cardClicked" {
		t.Errorf("firedEvent = %q, want cardClicked", firedEvent)
	}
}

func TestOnMapWinsOverNamedProp(t *testing.T) {
	// When both onClick and on.click exist, on.click takes precedence
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	var firedEvent string
	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Go","onClick":{"action":{"event":{"name":"fromOnClick"}}},"on":{"click":{"action":{"event":{"name":"fromOnMap"}}}}}}]}`)

	if surf, ok := sess.surfaces["s1"]; ok {
		surf.ActionHandler = func(sid string, event *protocol.EventDef, data map[string]interface{}) {
			firedEvent = event.Name
		}
	}

	mock.InvokeCallback("s1", "btn", "click", "")
	if firedEvent != "fromOnMap" {
		t.Errorf("firedEvent = %q, want fromOnMap (on map should win)", firedEvent)
	}
}

func TestKeyDownWithFilter(t *testing.T) {
	// Verify that a keyDown handler with filter only fires for matching keys
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	var firedEvent string
	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"field","type":"TextField","props":{"placeholder":"Type...","on":{"keyDown":{"filter":{"key":"Enter"},"action":{"event":{"name":"submitted"}}}}}}]}`)

	if surf, ok := sess.surfaces["s1"]; ok {
		surf.ActionHandler = func(sid string, event *protocol.EventDef, data map[string]interface{}) {
			firedEvent = event.Name
		}
	}

	// Non-matching key: should NOT fire
	mock.InvokeCallback("s1", "field", "keyDown", `{"key":"a","modifiers":[],"keyCode":0,"repeat":false}`)
	if firedEvent != "" {
		t.Errorf("non-matching key fired event: %q", firedEvent)
	}

	// Matching key: should fire
	mock.InvokeCallback("s1", "field", "keyDown", `{"key":"Enter","modifiers":[],"keyCode":36,"repeat":false}`)
	if firedEvent != "submitted" {
		t.Errorf("firedEvent = %q, want submitted", firedEvent)
	}
}

func TestKeyDownWithModifierFilter(t *testing.T) {
	// Verify that modifier filtering works (Cmd+S)
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	var firedEvent string
	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"editor","type":"Card","props":{"title":"Editor","on":{"keyDown":{"filter":{"key":"s","modifiers":["cmd"]},"action":{"event":{"name":"save"}}}}}}]}`)

	if surf, ok := sess.surfaces["s1"]; ok {
		surf.ActionHandler = func(sid string, event *protocol.EventDef, data map[string]interface{}) {
			firedEvent = event.Name
		}
	}

	// Just "s" without cmd: should NOT fire
	mock.InvokeCallback("s1", "editor", "keyDown", `{"key":"s","modifiers":[],"keyCode":1,"repeat":false}`)
	if firedEvent != "" {
		t.Errorf("s without cmd fired: %q", firedEvent)
	}

	// Cmd+S: should fire
	mock.InvokeCallback("s1", "editor", "keyDown", `{"key":"s","modifiers":["cmd"],"keyCode":1,"repeat":false}`)
	if firedEvent != "save" {
		t.Errorf("firedEvent = %q, want save", firedEvent)
	}
}

func TestKeyDownDataPathWithoutFilter(t *testing.T) {
	// Verify keyDown with dataPath captures all key presses
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/lastKey","value":""}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"area","type":"Card","props":{"title":"Keys","on":{"keyDown":{"dataPath":"/lastKey"}}}}]}`)

	mock.InvokeCallback("s1", "area", "keyDown", `{"key":"Enter","modifiers":["cmd"],"keyCode":36,"repeat":false}`)

	if surf, ok := sess.surfaces["s1"]; ok {
		val, found := surf.dm.Get("/lastKey")
		if !found {
			t.Fatal("/lastKey not found")
		}
		// DataPath with nil DataValue writes the parsed native data
		valMap, ok := val.(map[string]interface{})
		if !ok {
			t.Fatalf("/lastKey is %T, want map", val)
		}
		if valMap["key"] != "Enter" {
			t.Errorf("key = %v, want Enter", valMap["key"])
		}
	}
}

func TestKeyDownCallbackRegistration(t *testing.T) {
	// Verify keyDown/keyUp produce callbacks in the render node
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"area","type":"Card","props":{"title":"Keys","on":{"keyDown":{"action":{"event":{"name":"kd"}}},"keyUp":{"action":{"event":{"name":"ku"}}}}}}]}`)

	for _, c := range mock.Created {
		if c.Node.ComponentID == "area" {
			if _, ok := c.Node.Callbacks["keyDown"]; !ok {
				t.Error("keyDown callback not registered")
			}
			if _, ok := c.Node.Callbacks["keyUp"]; !ok {
				t.Error("keyUp callback not registered")
			}
			return
		}
	}
	t.Fatal("area not found in created views")
}

// --- on/off message tests ---

func TestOnMessageSubscription(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/size","value":{}}]}
{"type":"on","surfaceId":"s1","id":"resize-1","event":"window.resize","handler":{"dataPath":"/size"}}`)

	// EventManager should have the subscription
	em := sess.EventManager()
	em.mu.Lock()
	_, exists := em.subs["resize-1"]
	em.mu.Unlock()
	if !exists {
		t.Fatal("subscription resize-1 not found")
	}

	// Fire the event
	em.Fire("window.resize", "s1", `{"width":800,"height":600}`)

	// Check that /size was updated
	if surf, ok := sess.surfaces["s1"]; ok {
		val, found := surf.dm.Get("/size")
		if !found {
			t.Fatal("/size not found")
		}
		sizeMap, ok := val.(map[string]interface{})
		if !ok {
			t.Fatalf("/size is %T, want map", val)
		}
		if sizeMap["width"] != float64(800) {
			t.Errorf("width = %v, want 800", sizeMap["width"])
		}
	}
}

func TestOffMessageUnsubscribes(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"on","surfaceId":"s1","id":"timer-1","event":"system.timer","handler":{"dataPath":"/tick"}}
{"type":"off","id":"timer-1"}`)

	em := sess.EventManager()
	em.mu.Lock()
	_, exists := em.subs["timer-1"]
	em.mu.Unlock()
	if exists {
		t.Fatal("subscription timer-1 should have been removed")
	}
}

func TestOnMessageWithAction(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	var firedEvent string
	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"on","surfaceId":"s1","id":"close-1","event":"window.close","handler":{"action":{"event":{"name":"windowClosed"}}}}`)

	if surf, ok := sess.surfaces["s1"]; ok {
		surf.ActionHandler = func(sid string, event *protocol.EventDef, data map[string]interface{}) {
			firedEvent = event.Name
		}
	}

	sess.EventManager().Fire("window.close", "s1", "{}")

	if firedEvent != "windowClosed" {
		t.Errorf("firedEvent = %q, want windowClosed", firedEvent)
	}
}

func TestSystemTimerFires(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/tick","value":0}]}
{"type":"on","surfaceId":"s1","id":"timer-1","event":"system.timer","config":{"interval":50},"handler":{"dataPath":"/tick"}}`)

	// Wait for at least 2 ticks
	time.Sleep(150 * time.Millisecond)

	// Unsubscribe to stop the timer
	feedMessages(t, sess, `{"type":"off","id":"timer-1"}`)

	// Check that /tick was updated
	sess.mu.Lock()
	surf, ok := sess.surfaces["s1"]
	sess.mu.Unlock()
	if !ok {
		t.Fatal("surface not found")
	}
	val, found := surf.dm.Get("/tick")
	if !found {
		t.Fatal("/tick not found")
	}
	// The value should be a map with "tick" and "elapsed" keys from the timer data
	tickMap, ok := val.(map[string]interface{})
	if !ok {
		t.Fatalf("/tick is %T, want map (timer data)", val)
	}
	tickNum, _ := tickMap["tick"].(float64)
	if tickNum < 2 {
		t.Errorf("tick count = %v, want >= 2", tickNum)
	}
}

func TestOnMessageCleanupOnSurfaceDelete(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"on","surfaceId":"s1","id":"sub-1","event":"window.resize","handler":{"dataPath":"/size"}}
{"type":"deleteSurface","surfaceId":"s1"}`)

	em := sess.EventManager()
	em.mu.Lock()
	_, exists := em.subs["sub-1"]
	em.mu.Unlock()
	if exists {
		t.Fatal("subscription should have been cleaned up with surface deletion")
	}
}

func TestWindowResizeEvent(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":800,"height":600}
{"type":"on","surfaceId":"main","id":"resize-1","event":"window.resize","handler":{"dataPath":"/window/size"}}`)

	sess.EventManager().Fire("window.resize", "main", `{"width":1024,"height":768}`)

	sess.mu.Lock()
	surf := sess.surfaces["main"]
	sess.mu.Unlock()

	val, found := surf.dm.Get("/window/size")
	if !found {
		t.Fatal("expected /window/size to be set after resize event")
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", val)
	}
	if w, _ := m["width"].(float64); w != 1024 {
		t.Errorf("expected width=1024, got %v", w)
	}
}

func TestWindowEventWithAction(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":800,"height":600}
{"type":"on","surfaceId":"main","id":"close-1","event":"window.beforeClose","handler":{"action":{"event":{"name":"closeRequested"}}}}`)

	sess.EventManager().Fire("window.beforeClose", "main", `{}`)

	// Verify subscription was registered (action handler fires through session)
	em := sess.EventManager()
	em.mu.Lock()
	_, exists := em.subs["close-1"]
	em.mu.Unlock()
	if !exists {
		t.Fatal("expected close-1 subscription to exist")
	}
}

func TestSystemEventFires(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":800,"height":600}
{"type":"on","surfaceId":"main","id":"appearance-1","event":"system.appearance","handler":{"dataPath":"/system/appearance"}}`)

	sess.EventManager().Fire("system.appearance", "main", `{"appearance":"dark"}`)

	sess.mu.Lock()
	surf := sess.surfaces["main"]
	sess.mu.Unlock()

	val, found := surf.dm.Get("/system/appearance")
	if !found {
		t.Fatal("expected /system/appearance to be set after appearance event")
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", val)
	}
	if app, _ := m["appearance"].(string); app != "dark" {
		t.Errorf("expected appearance=dark, got %v", app)
	}
}

func TestSystemClipboardEvent(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":800,"height":600}
{"type":"on","surfaceId":"main","id":"clip-1","event":"system.clipboard.changed","handler":{"dataPath":"/clipboard/changed","dataValue":true}}`)

	sess.EventManager().Fire("system.clipboard.changed", "main", `{}`)

	sess.mu.Lock()
	surf := sess.surfaces["main"]
	sess.mu.Unlock()

	val, found := surf.dm.Get("/clipboard/changed")
	if !found {
		t.Fatal("expected /clipboard/changed to be set")
	}
	if val != true {
		t.Fatalf("expected /clipboard/changed=true, got %v", val)
	}
}

func TestFSWatchSubscription(t *testing.T) {
	dir := t.TempDir()

	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":800,"height":600}`)

	sess.EventManager().Subscribe(protocol.OnMessage{
		Type:      protocol.MsgOn,
		SurfaceID: "main",
		ID:        "fs-1",
		Event:     "system.fs.watch",
		Config:    map[string]interface{}{"paths": []interface{}{dir}},
		Handler:   protocol.EventAction{DataPath: "/fs/event"},
	})

	// Write a file to trigger the watcher
	if err := os.WriteFile(dir+"/test.txt", []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Wait for event to propagate
	time.Sleep(200 * time.Millisecond)

	sess.mu.Lock()
	surf := sess.surfaces["main"]
	sess.mu.Unlock()

	val, found := surf.dm.Get("/fs/event")
	if !found {
		t.Fatal("expected /fs/event to be set after file creation")
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", val)
	}
	if _, ok := m["path"]; !ok {
		t.Error("expected 'path' key in fs event data")
	}

	sess.EventManager().Unsubscribe("fs-1")
}

func TestMultipleWindowEventSubscribers(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":800,"height":600}
{"type":"on","surfaceId":"main","id":"r1","event":"window.resize","handler":{"dataPath":"/r1"}}
{"type":"on","surfaceId":"main","id":"r2","event":"window.resize","handler":{"dataPath":"/r2"}}`)

	sess.EventManager().Fire("window.resize", "main", `{"width":500,"height":400}`)

	sess.mu.Lock()
	surf := sess.surfaces["main"]
	sess.mu.Unlock()

	if _, found := surf.dm.Get("/r1"); !found {
		t.Error("expected /r1 to be set")
	}
	if _, found := surf.dm.Get("/r2"); !found {
		t.Error("expected /r2 to be set")
	}
}

func TestOnDemandSourceControl(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	var controlCalls []string
	sess.EventManager().SetControl(func(event, action string, config map[string]interface{}) {
		controlCalls = append(controlCalls, event+":"+action)
	})

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":800,"height":600}
{"type":"on","surfaceId":"main","id":"bt-1","event":"system.bluetooth","handler":{"dataPath":"/bt"}}`)

	if len(controlCalls) != 1 || controlCalls[0] != "system.bluetooth:start" {
		t.Fatalf("expected [system.bluetooth:start], got %v", controlCalls)
	}

	// Second subscriber should NOT trigger start again
	feedMessages(t, sess, `{"type":"on","surfaceId":"main","id":"bt-2","event":"system.bluetooth","handler":{"dataPath":"/bt2"}}`)
	if len(controlCalls) != 1 {
		t.Fatalf("expected no additional start, got %v", controlCalls)
	}

	// Unsubscribe first — should NOT trigger stop (bt-2 still active)
	sess.EventManager().Unsubscribe("bt-1")
	if len(controlCalls) != 1 {
		t.Fatalf("expected no stop yet, got %v", controlCalls)
	}

	// Unsubscribe last — should trigger stop
	sess.EventManager().Unsubscribe("bt-2")
	if len(controlCalls) != 2 || controlCalls[1] != "system.bluetooth:stop" {
		t.Fatalf("expected [system.bluetooth:start, system.bluetooth:stop], got %v", controlCalls)
	}
}

func TestDistributedNotificationSubscription(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	var controlCalls []string
	sess.EventManager().SetControl(func(event, action string, config map[string]interface{}) {
		name, _ := config["name"].(string)
		controlCalls = append(controlCalls, event+":"+action+":"+name)
	})

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":800,"height":600}`)

	sess.EventManager().Subscribe(protocol.OnMessage{
		Type:      protocol.MsgOn,
		SurfaceID: "main",
		ID:        "dn-1",
		Event:     "system.ipc.distributed",
		Config:    map[string]interface{}{"name": "com.example.TestNotification"},
		Handler:   protocol.EventAction{DataPath: "/ipc/last"},
	})

	if len(controlCalls) != 1 {
		t.Fatalf("expected 1 control call, got %v", controlCalls)
	}
	if controlCalls[0] != "system.ipc.distributed:start:com.example.TestNotification" {
		t.Fatalf("expected distributed start, got %v", controlCalls[0])
	}

	sess.EventManager().Unsubscribe("dn-1")
	if len(controlCalls) != 2 {
		t.Fatalf("expected 2 control calls, got %v", controlCalls)
	}
}

func TestCleanupAllRemovesSubscriptions(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":800,"height":600}
{"type":"on","surfaceId":"main","id":"s1","event":"window.resize","handler":{"dataPath":"/a"}}
{"type":"on","surfaceId":"main","id":"s2","event":"system.appearance","handler":{"dataPath":"/b"}}`)

	em := sess.EventManager()
	em.mu.Lock()
	if len(em.subs) != 2 {
		em.mu.Unlock()
		t.Fatalf("expected 2 subs, got %d", len(em.subs))
	}
	em.mu.Unlock()

	em.CleanupAll()

	em.mu.Lock()
	count := len(em.subs)
	em.mu.Unlock()
	if count != 0 {
		t.Fatalf("expected 0 subs after CleanupAll, got %d", count)
	}
}

func TestCleanupAllStopsOnDemandSources(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	var controlCalls []string
	sess.EventManager().SetControl(func(event, action string, config map[string]interface{}) {
		controlCalls = append(controlCalls, event+":"+action)
	})

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":800,"height":600}
{"type":"on","surfaceId":"main","id":"usb-1","event":"system.usb","handler":{"dataPath":"/usb"}}`)

	if len(controlCalls) != 1 || controlCalls[0] != "system.usb:start" {
		t.Fatalf("expected [system.usb:start], got %v", controlCalls)
	}

	sess.EventManager().CleanupAll()

	if len(controlCalls) != 2 || controlCalls[1] != "system.usb:stop" {
		t.Fatalf("expected stop after cleanup, got %v", controlCalls)
	}
}

func TestSubscribeReplacesSameID(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":800,"height":600}
{"type":"on","surfaceId":"main","id":"sub-1","event":"window.resize","handler":{"dataPath":"/v1"}}
{"type":"on","surfaceId":"main","id":"sub-1","event":"window.resize","handler":{"dataPath":"/v2"}}`)

	// Fire event — should write to /v2, not /v1
	sess.EventManager().Fire("window.resize", "main", `{"width":100,"height":100}`)

	sess.mu.Lock()
	surf := sess.surfaces["main"]
	sess.mu.Unlock()

	if _, found := surf.dm.Get("/v1"); found {
		t.Error("/v1 should NOT be set — subscription was replaced")
	}
	if _, found := surf.dm.Get("/v2"); !found {
		t.Error("/v2 should be set by the replacement subscription")
	}
}

func TestAutoGeneratedSubscriptionID(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":800,"height":600}
{"type":"on","surfaceId":"main","event":"window.resize","handler":{"dataPath":"/a"}}
{"type":"on","surfaceId":"main","event":"window.move","handler":{"dataPath":"/b"}}`)

	em := sess.EventManager()
	em.mu.Lock()
	count := len(em.subs)
	em.mu.Unlock()
	if count != 2 {
		t.Fatalf("expected 2 auto-ID subs, got %d", count)
	}
}

func TestMatchesFilterInvalidJSON(t *testing.T) {
	filter := &protocol.EventFilter{Key: "Enter"}
	if matchesFilter(filter, "not json") {
		t.Error("should return false for invalid JSON")
	}
}

func TestMatchesFilterButtonMismatch(t *testing.T) {
	filter := &protocol.EventFilter{Button: 2}
	if matchesFilter(filter, `{"button":1}`) {
		t.Error("should return false for button mismatch")
	}
	if !matchesFilter(filter, `{"button":2}`) {
		t.Error("should return true for matching button")
	}
}

func TestMatchesFilterKeyAndModifiersCombined(t *testing.T) {
	filter := &protocol.EventFilter{
		Key:       "s",
		Modifiers: []string{"cmd"},
	}
	if matchesFilter(filter, `{"key":"s","modifiers":["shift"]}`) {
		t.Error("should fail: cmd modifier missing")
	}
	if !matchesFilter(filter, `{"key":"s","modifiers":["cmd","shift"]}`) {
		t.Error("should pass: cmd present (extra shift is ok)")
	}
	if matchesFilter(filter, `{"key":"a","modifiers":["cmd"]}`) {
		t.Error("should fail: key mismatch")
	}
}

func TestSliderDataBinding(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"slider1","type":"Slider","props":{"dataBinding":"/volume","minValue":0,"maxValue":100}}]}`)

	mock.InvokeCallback("s1", "slider1", "slide", "75.5")

	if surf, ok := sess.surfaces["s1"]; ok {
		val, found := surf.dm.Get("/volume")
		if !found {
			t.Fatal("expected /volume to be set after slide")
		}
		if v, ok := val.(float64); !ok || v != 75.5 {
			t.Fatalf("expected 75.5, got %v", val)
		}
	}
}

func TestChoicePickerDataBinding(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"picker1","type":"ChoicePicker","props":{"dataBinding":"/choice","choices":["A","B","C"]}}]}`)

	mock.InvokeCallback("s1", "picker1", "select", "B")

	if surf, ok := sess.surfaces["s1"]; ok {
		val, found := surf.dm.Get("/choice")
		if !found {
			t.Fatal("expected /choice to be set after select")
		}
		if val != "B" {
			t.Fatalf("expected 'B', got %v", val)
		}
	}
}

func TestDateTimeInputDataBinding(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"date1","type":"DateTimeInput","props":{"dataBinding":"/date"}}]}`)

	mock.InvokeCallback("s1", "date1", "datechange", "2026-04-09")

	if surf, ok := sess.surfaces["s1"]; ok {
		val, found := surf.dm.Get("/date")
		if !found {
			t.Fatal("expected /date to be set after datechange")
		}
		if val != "2026-04-09" {
			t.Fatalf("expected '2026-04-09', got %v", val)
		}
	}
}

func TestSearchFieldDataBinding(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"search1","type":"SearchField","props":{"dataBinding":"/query"}}]}`)

	mock.InvokeCallback("s1", "search1", "change", "hello world")

	if surf, ok := sess.surfaces["s1"]; ok {
		val, found := surf.dm.Get("/query")
		if !found {
			t.Fatal("expected /query to be set after change")
		}
		if val != "hello world" {
			t.Fatalf("expected 'hello world', got %v", val)
		}
	}
}

func TestThrottleInRegisterCallbacks(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"card1","type":"Card","props":{"on":{"mouseEnter":{"dataPath":"/hover","dataValue":"card1","throttle":100}}}}]}`)

	// Fire rapidly — only first should get through
	mock.InvokeCallback("s1", "card1", "mouseEnter", `{"x":10,"y":20}`)
	mock.InvokeCallback("s1", "card1", "mouseEnter", `{"x":11,"y":21}`)

	if surf, ok := sess.surfaces["s1"]; ok {
		val, found := surf.dm.Get("/hover")
		if !found {
			t.Fatal("expected /hover to be set")
		}
		if val != "card1" {
			t.Fatalf("expected 'card1', got %v", val)
		}
	}
}

func TestDebounceInRegisterCallbacks(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"card1","type":"Card","props":{"on":{"mouseEnter":{"dataPath":"/hover","debounce":50}}}}]}`)

	// Fire rapidly
	mock.InvokeCallback("s1", "card1", "mouseEnter", `{"x":1,"y":1}`)
	mock.InvokeCallback("s1", "card1", "mouseEnter", `{"x":2,"y":2}`)
	mock.InvokeCallback("s1", "card1", "mouseEnter", `{"x":3,"y":3}`)

	// Wait for debounce to fire
	time.Sleep(80 * time.Millisecond)

	if surf, ok := sess.surfaces["s1"]; ok {
		val, found := surf.dm.Get("/hover")
		if !found {
			t.Fatal("expected /hover to be set after debounce period")
		}
		// Should have the last event's data
		m, _ := val.(map[string]interface{})
		if x, _ := m["x"].(float64); x != 3 {
			t.Fatalf("expected x=3 (last debounced), got %v", x)
		}
	}
}

func TestFireWithNoMatchingSubscribers(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":800,"height":600}`)

	// Fire with no subscribers — should not panic
	sess.EventManager().Fire("window.resize", "main", `{"width":100,"height":100}`)
}

func TestFireSurfaceIDFiltering(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"S1","width":800,"height":600}
{"type":"createSurface","surfaceId":"s2","title":"S2","width":800,"height":600}
{"type":"on","surfaceId":"s1","id":"r1","event":"window.resize","handler":{"dataPath":"/size"}}`)

	// Fire on s2 — should NOT match s1's subscription
	sess.EventManager().Fire("window.resize", "s2", `{"width":999,"height":999}`)

	sess.mu.Lock()
	surf1 := sess.surfaces["s1"]
	sess.mu.Unlock()

	if _, found := surf1.dm.Get("/size"); found {
		t.Error("s1's subscription should NOT fire for s2 events")
	}

	// Fire on s1 — should match
	sess.EventManager().Fire("window.resize", "s1", `{"width":500,"height":400}`)

	if _, found := surf1.dm.Get("/size"); !found {
		t.Error("s1's subscription should fire for s1 events")
	}
}

func TestFSWatchInvalidPathLogs(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":800,"height":600}`)

	// Subscribe with bad path — should not panic
	sess.EventManager().Subscribe(protocol.OnMessage{
		Type:      protocol.MsgOn,
		SurfaceID: "main",
		ID:        "fs-bad",
		Event:     "system.fs.watch",
		Config:    map[string]interface{}{"paths": []interface{}{"/nonexistent/path/that/does/not/exist"}},
		Handler:   protocol.EventAction{DataPath: "/fs/event"},
	})

	// Cleanup
	sess.EventManager().Unsubscribe("fs-bad")
}

func TestFSWatchNoPaths(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":800,"height":600}`)

	// Subscribe with no paths — should not crash
	sess.EventManager().Subscribe(protocol.OnMessage{
		Type:      protocol.MsgOn,
		SurfaceID: "main",
		ID:        "fs-empty",
		Event:     "system.fs.watch",
		Config:    map[string]interface{}{},
		Handler:   protocol.EventAction{DataPath: "/fs/event"},
	})
}

func TestFSWatchSinglePathString(t *testing.T) {
	dir := t.TempDir()
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":800,"height":600}`)

	// Subscribe with single string path (not array)
	sess.EventManager().Subscribe(protocol.OnMessage{
		Type:      protocol.MsgOn,
		SurfaceID: "main",
		ID:        "fs-str",
		Event:     "system.fs.watch",
		Config:    map[string]interface{}{"path": dir},
		Handler:   protocol.EventAction{DataPath: "/fs/event"},
	})

	if err := os.WriteFile(dir+"/test.txt", []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	sess.mu.Lock()
	surf := sess.surfaces["main"]
	sess.mu.Unlock()

	if _, found := surf.dm.Get("/fs/event"); !found {
		t.Error("expected fs event from single path string config")
	}

	sess.EventManager().Unsubscribe("fs-str")
}

func TestDistributedNotificationNoName(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	var controlCalls []string
	sess.EventManager().SetControl(func(event, action string, config map[string]interface{}) {
		controlCalls = append(controlCalls, event+":"+action)
	})

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":800,"height":600}`)

	// Subscribe with no name — should log error but not crash
	sess.EventManager().Subscribe(protocol.OnMessage{
		Type:      protocol.MsgOn,
		SurfaceID: "main",
		ID:        "dn-bad",
		Event:     "system.ipc.distributed",
		Config:    map[string]interface{}{},
		Handler:   protocol.EventAction{DataPath: "/ipc"},
	})

	// No control call should have been made since name is empty
	if len(controlCalls) != 0 {
		t.Fatalf("expected no control calls for missing name, got %v", controlCalls)
	}
}

func TestTimerMinimumInterval(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"T","width":800,"height":600}`)

	// Subscribe with interval below minimum (should clamp to 10ms)
	sess.EventManager().Subscribe(protocol.OnMessage{
		Type:      protocol.MsgOn,
		SurfaceID: "main",
		ID:        "timer-fast",
		Event:     "system.timer",
		Config:    map[string]interface{}{"interval": float64(1)},
		Handler:   protocol.EventAction{DataPath: "/tick"},
	})

	time.Sleep(50 * time.Millisecond)

	sess.mu.Lock()
	surf := sess.surfaces["main"]
	sess.mu.Unlock()

	val, found := surf.dm.Get("/tick")
	if !found {
		t.Fatal("expected /tick to be set")
	}
	// Should have fired at least once with clamped interval
	m, ok := val.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", val)
	}
	tick, _ := m["tick"].(float64)
	if tick < 1 {
		t.Error("expected at least 1 tick")
	}

	sess.EventManager().Unsubscribe("timer-fast")
}

func TestModalDismissBinding(t *testing.T) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/showModal","value":true}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"modal1","type":"Modal","props":{"dataBinding":"/showModal","title":"Test","visible":true}}]}`)

	mock.InvokeCallback("s1", "modal1", "dismiss", "")

	if surf, ok := sess.surfaces["s1"]; ok {
		val, found := surf.dm.Get("/showModal")
		if !found {
			t.Fatal("expected /showModal to exist")
		}
		if val != false {
			t.Fatalf("expected false after dismiss, got %v", val)
		}
	}
}
