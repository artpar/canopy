package engine

import (
	"jview/protocol"
	"jview/renderer"
	"strings"
	"testing"
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
	if created["tmpl_0"] != "Alice" {
		t.Errorf("tmpl_0 content = %q, want 'Alice'", created["tmpl_0"])
	}
	if created["tmpl_1"] != "Bob" {
		t.Errorf("tmpl_1 content = %q, want 'Bob'", created["tmpl_1"])
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

	// card_0 and card_1 should exist
	cardCreated := false
	for _, c := range mock.Created {
		if c.Node.ComponentID == "card_0" {
			cardCreated = true
			if c.Node.Props.Title != "Alice" {
				t.Errorf("card_0 title = %q, want Alice", c.Node.Props.Title)
			}
		}
	}
	if !cardCreated {
		t.Error("card_0 not created")
	}

	// cardRole_0 and cardRole_1 should exist
	if created["cardRole_0"] != "Engineer" {
		t.Errorf("cardRole_0 content = %q, want 'Engineer'", created["cardRole_0"])
	}
	if created["cardRole_1"] != "Designer" {
		t.Errorf("cardRole_1 content = %q, want 'Designer'", created["cardRole_1"])
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
