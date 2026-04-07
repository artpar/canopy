package engine

import (
	"canopy/protocol"
	"testing"
)

func newTestResolver() (*Resolver, *DataModel, *BindingTracker) {
	dm := NewDataModel()
	tracker := NewBindingTracker()
	evaluator := NewEvaluator(dm)
	return NewResolver(dm, tracker, evaluator), dm, tracker
}

func TestResolveTextLiteral(t *testing.T) {
	r, _, _ := newTestResolver()
	comp := &protocol.Component{
		ComponentID: "t1",
		Type:        protocol.CompText,
		Props: protocol.Props{
			Content: &protocol.DynamicString{Literal: "hello"},
			Variant: "h1",
		},
	}
	node := r.Resolve(comp)
	if node.Props.Content != "hello" {
		t.Errorf("content = %q, want hello", node.Props.Content)
	}
	if node.Props.Variant != "h1" {
		t.Errorf("variant = %q, want h1", node.Props.Variant)
	}
}

func TestResolveTextPath(t *testing.T) {
	r, dm, tracker := newTestResolver()
	dm.Set("/name", "Alice")

	comp := &protocol.Component{
		ComponentID: "t1",
		Type:        protocol.CompText,
		Props: protocol.Props{
			Content: &protocol.DynamicString{Path: "/name", IsPath: true},
		},
	}
	node := r.Resolve(comp)
	if node.Props.Content != "Alice" {
		t.Errorf("content = %q, want Alice", node.Props.Content)
	}

	// Binding should be registered
	affected := tracker.Affected([]string{"/name"})
	found := false
	for _, id := range affected {
		if id == "t1" {
			found = true
		}
	}
	if !found {
		t.Error("binding not registered for /name → t1")
	}
}

func TestResolveTextPathMissing(t *testing.T) {
	r, _, _ := newTestResolver()
	comp := &protocol.Component{
		ComponentID: "t1",
		Type:        protocol.CompText,
		Props: protocol.Props{
			Content: &protocol.DynamicString{Path: "/missing", IsPath: true},
		},
	}
	node := r.Resolve(comp)
	if node.Props.Content != "" {
		t.Errorf("content = %q, want empty string for missing path", node.Props.Content)
	}
}

func TestResolveTextDefaultVariant(t *testing.T) {
	r, _, _ := newTestResolver()
	comp := &protocol.Component{
		ComponentID: "t1",
		Type:        protocol.CompText,
		Props: protocol.Props{
			Content: &protocol.DynamicString{Literal: "text"},
		},
	}
	node := r.Resolve(comp)
	if node.Props.Variant != "body" {
		t.Errorf("variant = %q, want body (default)", node.Props.Variant)
	}
}

func TestResolveButtonDefaultStyle(t *testing.T) {
	r, _, _ := newTestResolver()
	comp := &protocol.Component{
		ComponentID: "b1",
		Type:        protocol.CompButton,
		Props: protocol.Props{
			Label: &protocol.DynamicString{Literal: "Click"},
		},
	}
	node := r.Resolve(comp)
	if node.Props.Style != "secondary" {
		t.Errorf("style = %q, want secondary (default)", node.Props.Style)
	}
}

func TestResolveTextFieldDefaultInputType(t *testing.T) {
	r, _, _ := newTestResolver()
	comp := &protocol.Component{
		ComponentID: "f1",
		Type:        protocol.CompTextField,
		Props: protocol.Props{
			Placeholder: &protocol.DynamicString{Literal: "Type here"},
		},
	}
	node := r.Resolve(comp)
	if node.Props.InputType != "shortText" {
		t.Errorf("inputType = %q, want shortText (default)", node.Props.InputType)
	}
}

func TestResolveBoolPathTrue(t *testing.T) {
	r, dm, _ := newTestResolver()
	dm.Set("/agreed", true)

	comp := &protocol.Component{
		ComponentID: "cb1",
		Type:        protocol.CompCheckBox,
		Props: protocol.Props{
			Label:   &protocol.DynamicString{Literal: "Agree"},
			Checked: &protocol.DynamicBoolean{Path: "/agreed", IsPath: true},
		},
	}
	node := r.Resolve(comp)
	if !node.Props.Checked {
		t.Error("checked = false, want true")
	}
}

func TestResolveBoolPathWrongType(t *testing.T) {
	r, dm, _ := newTestResolver()
	dm.Set("/agreed", "yes") // string, not bool

	comp := &protocol.Component{
		ComponentID: "cb1",
		Type:        protocol.CompCheckBox,
		Props: protocol.Props{
			Checked: &protocol.DynamicBoolean{Path: "/agreed", IsPath: true},
		},
	}
	node := r.Resolve(comp)
	if node.Props.Checked {
		t.Error("checked = true, want false (wrong type)")
	}
}

func TestResolveNumberPath(t *testing.T) {
	r, dm, tracker := newTestResolver()
	dm.Set("/val", 42.0)

	// Test resolveNumber directly via resolving a component that uses it
	// Slider isn't fully resolved yet, but we can test the resolver method
	result := r.resolveNumber("test", &protocol.DynamicNumber{Path: "/val", IsPath: true})
	if result != 42.0 {
		t.Errorf("resolveNumber = %f, want 42", result)
	}

	// Binding registered
	affected := tracker.Affected([]string{"/val"})
	found := false
	for _, id := range affected {
		if id == "test" {
			found = true
		}
	}
	if !found {
		t.Error("binding not registered for /val → test")
	}
}

func TestResolveNumberPathWrongType(t *testing.T) {
	r, dm, _ := newTestResolver()
	dm.Set("/val", "not a number")

	result := r.resolveNumber("test", &protocol.DynamicNumber{Path: "/val", IsPath: true})
	if result != 0 {
		t.Errorf("resolveNumber = %f, want 0 (wrong type)", result)
	}
}
