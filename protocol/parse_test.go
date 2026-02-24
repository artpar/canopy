package protocol

import (
	"io"
	"strings"
	"testing"
)

func TestParseCreateSurface(t *testing.T) {
	input := `{"type":"createSurface","surfaceId":"main","title":"Test","width":800,"height":600}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != MsgCreateSurface {
		t.Fatalf("expected createSurface, got %s", msg.Type)
	}
	cs := msg.Body.(CreateSurface)
	if cs.SurfaceID != "main" {
		t.Errorf("surfaceId = %q, want %q", cs.SurfaceID, "main")
	}
	if cs.Title != "Test" {
		t.Errorf("title = %q, want %q", cs.Title, "Test")
	}
	if cs.Width != 800 {
		t.Errorf("width = %d, want 800", cs.Width)
	}
}

func TestParseUpdateComponents(t *testing.T) {
	input := `{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"hello","variant":"h1"}},{"componentId":"c1","type":"Card","props":{"title":"Card"},"children":["t1"]}]}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	uc := msg.Body.(UpdateComponents)
	if len(uc.Components) != 2 {
		t.Fatalf("got %d components, want 2", len(uc.Components))
	}

	text := uc.Components[0]
	if text.ComponentID != "t1" {
		t.Errorf("component 0 id = %q, want t1", text.ComponentID)
	}
	if text.Type != CompText {
		t.Errorf("component 0 type = %q, want Text", text.Type)
	}
	if text.Props.Content == nil || text.Props.Content.Literal != "hello" {
		t.Errorf("component 0 content = %v, want 'hello'", text.Props.Content)
	}

	card := uc.Components[1]
	if card.Children == nil || len(card.Children.Static) != 1 || card.Children.Static[0] != "t1" {
		t.Errorf("card children = %v, want [t1]", card.Children)
	}
}

func TestParseDynamicStringPath(t *testing.T) {
	input := `{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":{"path":"/name"}}}]}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	uc := msg.Body.(UpdateComponents)
	content := uc.Components[0].Props.Content
	if !content.IsPath {
		t.Fatal("expected IsPath=true")
	}
	if content.Path != "/name" {
		t.Errorf("path = %q, want /name", content.Path)
	}
}

func TestParseUpdateDataModel(t *testing.T) {
	input := `{"type":"updateDataModel","surfaceId":"s1","ops":[{"op":"add","path":"/name","value":"Alice"},{"op":"replace","path":"/age","value":30}]}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	udm := msg.Body.(UpdateDataModel)
	if len(udm.Ops) != 2 {
		t.Fatalf("got %d ops, want 2", len(udm.Ops))
	}
	if udm.Ops[0].Op != "add" || udm.Ops[0].Path != "/name" {
		t.Errorf("op 0 = %v, want add /name", udm.Ops[0])
	}
	if udm.Ops[1].Op != "replace" || udm.Ops[1].Path != "/age" {
		t.Errorf("op 1 = %v, want replace /age", udm.Ops[1])
	}
}

func TestParseMultipleLines(t *testing.T) {
	input := `{"type":"createSurface","surfaceId":"s1","title":"T"}
{"type":"updateComponents","surfaceId":"s1","components":[]}
`
	p := NewParser(strings.NewReader(input))

	msg1, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	if msg1.Type != MsgCreateSurface {
		t.Errorf("msg1 type = %s, want createSurface", msg1.Type)
	}

	msg2, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	if msg2.Type != MsgUpdateComponents {
		t.Errorf("msg2 type = %s, want updateComponents", msg2.Type)
	}

	_, err = p.Next()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestParseSkipsBlankLines(t *testing.T) {
	input := "\n\n" + `{"type":"createSurface","surfaceId":"s1","title":"T"}` + "\n\n"
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != MsgCreateSurface {
		t.Errorf("type = %s, want createSurface", msg.Type)
	}
}

func TestParseInvalidJSON(t *testing.T) {
	p := NewParser(strings.NewReader("this is not json"))
	_, err := p.Next()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseUnknownMessageType(t *testing.T) {
	p := NewParser(strings.NewReader(`{"type":"foobar","surfaceId":"s1"}`))
	_, err := p.Next()
	if err == nil {
		t.Fatal("expected error for unknown message type")
	}
}

func TestParseMissingType(t *testing.T) {
	p := NewParser(strings.NewReader(`{"surfaceId":"s1"}`))
	_, err := p.Next()
	if err == nil {
		t.Fatal("expected error for missing type")
	}
}

func TestParseMalformedComponents(t *testing.T) {
	// components should be an array, not a string
	p := NewParser(strings.NewReader(`{"type":"updateComponents","surfaceId":"s1","components":"bad"}`))
	_, err := p.Next()
	if err == nil {
		t.Fatal("expected error for malformed components")
	}
}

func TestParseMissingComponentId(t *testing.T) {
	p := NewParser(strings.NewReader(`{"type":"updateComponents","surfaceId":"s1","components":[{"type":"Text","props":{"content":"no id"}}]}`))
	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	uc := msg.Body.(UpdateComponents)
	if uc.Components[0].ComponentID != "" {
		t.Errorf("expected empty componentId, got %q", uc.Components[0].ComponentID)
	}
}

func TestParseEmptyInput(t *testing.T) {
	p := NewParser(strings.NewReader(""))
	_, err := p.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestParseDynamicNumberLiteral(t *testing.T) {
	input := `{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"s1","type":"Slider","props":{"min":0,"max":100,"step":1}}]}`
	p := NewParser(strings.NewReader(input))
	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	uc := msg.Body.(UpdateComponents)
	if uc.Components[0].Props.Max == nil || uc.Components[0].Props.Max.Literal != 100 {
		t.Errorf("max = %v, want 100", uc.Components[0].Props.Max)
	}
}

func TestParseDynamicBooleanLiteral(t *testing.T) {
	input := `{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"b1","type":"Button","props":{"label":"Go","disabled":true}}]}`
	p := NewParser(strings.NewReader(input))
	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	uc := msg.Body.(UpdateComponents)
	if uc.Components[0].Props.Disabled == nil || !uc.Components[0].Props.Disabled.Literal {
		t.Errorf("disabled = %v, want true", uc.Components[0].Props.Disabled)
	}
}

func TestParseChildListStaticObject(t *testing.T) {
	input := `{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"row1","type":"Row","children":{"static":["btn1","btn2","btn3"]}}]}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	uc := msg.Body.(UpdateComponents)
	cl := uc.Components[0].Children
	if cl == nil {
		t.Fatal("expected children")
	}
	if len(cl.Static) != 3 {
		t.Fatalf("static children = %d, want 3", len(cl.Static))
	}
	if cl.Static[0] != "btn1" || cl.Static[1] != "btn2" || cl.Static[2] != "btn3" {
		t.Errorf("static = %v, want [btn1 btn2 btn3]", cl.Static)
	}
}

func TestParseChildListTemplate(t *testing.T) {
	input := `{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"list1","type":"Column","children":{"forEach":"/items","templateId":"item_tmpl","itemVariable":"item"}}]}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	uc := msg.Body.(UpdateComponents)
	cl := uc.Components[0].Children
	if cl.Template == nil {
		t.Fatal("expected template child list")
	}
	if cl.Template.ForEach != "/items" {
		t.Errorf("forEach = %q, want /items", cl.Template.ForEach)
	}
	if cl.Template.TemplateID != "item_tmpl" {
		t.Errorf("templateId = %q, want item_tmpl", cl.Template.TemplateID)
	}
}
