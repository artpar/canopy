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

	var firedAction string
	// Hook into the surface's action handler after creation
	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Go","style":"primary","onClick":{"action":{"type":"serverAction","name":"doThing"}}}}]}`)

	// Set action handler on the surface
	if surf, ok := sess.surfaces["s1"]; ok {
		surf.ActionHandler = func(sid string, action *protocol.Action, data map[string]interface{}) {
			firedAction = action.Name
		}
	}

	mock.InvokeCallback("s1", "btn", "click", "")
	if firedAction != "doThing" {
		t.Errorf("firedAction = %q, want doThing", firedAction)
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
{"type":"updateComponents","surfaceId":"form","components":[{"componentId":"root","type":"Column","props":{"gap":16,"padding":20},"children":["title","nameField","emailField","previewName","submitBtn"]},{"componentId":"title","type":"Text","props":{"content":"Contact Us","variant":"h2"}},{"componentId":"nameField","type":"TextField","props":{"placeholder":"Enter your name","value":{"path":"/name"},"dataBinding":"/name"}},{"componentId":"emailField","type":"TextField","props":{"placeholder":"you@example.com","value":{"path":"/email"},"dataBinding":"/email"}},{"componentId":"previewName","type":"Text","props":{"content":{"path":"/name"},"variant":"body"}},{"componentId":"submitBtn","type":"Button","props":{"label":"Submit","style":"primary","onClick":{"action":{"type":"serverAction","name":"submitForm","dataRefs":["/name","/email","/subscribe"]}}}}]}`)

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
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Submit","onClick":{"action":{"type":"serverAction","name":"submit","dataRefs":["/form/name","/form/email","/form/missing"]}}}}]}`)

	// Verify button was created
	if len(mock.Created) == 0 {
		t.Fatal("no components created")
	}

	// Access the surface to test resolveDataRefs directly
	surf := sess.surfaces["s1"]
	if surf == nil {
		t.Fatal("surface s1 not found")
	}

	result := surf.resolveDataRefs(&protocol.Action{
		Type:     "serverAction",
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
		action    *protocol.Action
		data      map[string]interface{}
	}
	sess.OnAction = func(surfaceID string, action *protocol.Action, data map[string]interface{}) {
		received.surfaceID = surfaceID
		received.action = action
		received.data = data
	}

	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/count","value":42}]}
{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn","type":"Button","props":{"label":"Click","onClick":{"action":{"type":"serverAction","name":"increment","dataRefs":["/count"]}}}}]}`)

	// Click the button
	mock.InvokeCallback("s1", "btn", "click", "")

	if received.surfaceID != "s1" {
		t.Errorf("surfaceID = %q, want s1", received.surfaceID)
	}
	if received.action == nil {
		t.Fatal("action not received")
	}
	if received.action.Name != "increment" {
		t.Errorf("action.Name = %q, want increment", received.action.Name)
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

func TestSessionUnknownMessageType(t *testing.T) {
	sess, _ := newTestSession()

	// Should not panic — unknown type handled gracefully
	msg := &protocol.Message{
		Type:      "totallyBogus",
		SurfaceID: "s1",
	}
	sess.HandleMessage(msg)
}
