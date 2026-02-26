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

func TestParseComponentStyle(t *testing.T) {
	input := `{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn1","type":"Button","props":{"label":"Go"},"style":{"backgroundColor":"#FF9F0A","textColor":"#FFFFFF","cornerRadius":8,"width":100,"height":52,"fontSize":20,"fontWeight":"bold","textAlign":"center","opacity":0.9}}]}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	uc := msg.Body.(UpdateComponents)
	s := uc.Components[0].Style
	if s.BackgroundColor == nil || s.BackgroundColor.Literal != "#FF9F0A" {
		t.Errorf("backgroundColor = %v, want #FF9F0A", s.BackgroundColor)
	}
	if s.TextColor == nil || s.TextColor.Literal != "#FFFFFF" {
		t.Errorf("textColor = %v, want #FFFFFF", s.TextColor)
	}
	if s.CornerRadius == nil || s.CornerRadius.Literal != 8 {
		t.Errorf("cornerRadius = %v, want 8", s.CornerRadius)
	}
	if s.Width == nil || s.Width.Literal != 100 {
		t.Errorf("width = %v, want 100", s.Width)
	}
	if s.Height == nil || s.Height.Literal != 52 {
		t.Errorf("height = %v, want 52", s.Height)
	}
	if s.FontSize == nil || s.FontSize.Literal != 20 {
		t.Errorf("fontSize = %v, want 20", s.FontSize)
	}
	if s.FontWeight == nil || s.FontWeight.Literal != "bold" {
		t.Errorf("fontWeight = %v, want bold", s.FontWeight)
	}
	if s.TextAlign == nil || s.TextAlign.Literal != "center" {
		t.Errorf("textAlign = %v, want center", s.TextAlign)
	}
	if s.Opacity == nil || s.Opacity.Literal != 0.9 {
		t.Errorf("opacity = %v, want 0.9", s.Opacity)
	}
}

func TestParseCreateSurfaceWithStyle(t *testing.T) {
	input := `{"type":"createSurface","surfaceId":"main","title":"Test","backgroundColor":"#1C1C1E","padding":-1}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	cs := msg.Body.(CreateSurface)
	if cs.BackgroundColor != "#1C1C1E" {
		t.Errorf("backgroundColor = %q, want #1C1C1E", cs.BackgroundColor)
	}
	if cs.Padding != -1 {
		t.Errorf("padding = %d, want -1", cs.Padding)
	}
}

func TestParseLoadLibrary(t *testing.T) {
	input := `{"type":"loadLibrary","path":"/usr/local/lib/mylib.dylib","prefix":"mylib","functions":[{"name":"add","symbol":"mylib_add","returnType":"double","paramTypes":["double","double"]},{"name":"reverse","symbol":"mylib_reverse","returnType":"string","paramTypes":["string"],"fixedArgs":0}]}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != MsgLoadLibrary {
		t.Fatalf("expected loadLibrary, got %s", msg.Type)
	}
	ll := msg.Body.(LoadLibrary)
	if ll.Path != "/usr/local/lib/mylib.dylib" {
		t.Errorf("path = %q, want /usr/local/lib/mylib.dylib", ll.Path)
	}
	if ll.Prefix != "mylib" {
		t.Errorf("prefix = %q, want mylib", ll.Prefix)
	}
	if len(ll.Functions) != 2 {
		t.Fatalf("got %d functions, want 2", len(ll.Functions))
	}
	if ll.Functions[0].Name != "add" || ll.Functions[0].Symbol != "mylib_add" {
		t.Errorf("func 0 = %+v, want {add, mylib_add}", ll.Functions[0])
	}
	if ll.Functions[0].ReturnType != "double" {
		t.Errorf("func 0 returnType = %q, want double", ll.Functions[0].ReturnType)
	}
	if len(ll.Functions[0].ParamTypes) != 2 || ll.Functions[0].ParamTypes[0] != "double" {
		t.Errorf("func 0 paramTypes = %v, want [double, double]", ll.Functions[0].ParamTypes)
	}
	if ll.Functions[1].Name != "reverse" || ll.Functions[1].Symbol != "mylib_reverse" {
		t.Errorf("func 1 = %+v, want {reverse, mylib_reverse}", ll.Functions[1])
	}
	if ll.Functions[1].ReturnType != "string" {
		t.Errorf("func 1 returnType = %q, want string", ll.Functions[1].ReturnType)
	}
}

func TestParseLoadAssets(t *testing.T) {
	input := `{"type":"loadAssets","assets":[{"alias":"BrandFont","kind":"font","src":"./fonts/brand.ttf"},{"alias":"hero","kind":"image","src":"https://example.com/hero.png"}]}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != MsgLoadAssets {
		t.Fatalf("expected loadAssets, got %s", msg.Type)
	}
	la := msg.Body.(LoadAssets)
	if len(la.Assets) != 2 {
		t.Fatalf("got %d assets, want 2", len(la.Assets))
	}
	if la.Assets[0].Alias != "BrandFont" || la.Assets[0].Kind != "font" || la.Assets[0].Src != "./fonts/brand.ttf" {
		t.Errorf("asset 0 = %+v", la.Assets[0])
	}
	if la.Assets[1].Alias != "hero" || la.Assets[1].Kind != "image" || la.Assets[1].Src != "https://example.com/hero.png" {
		t.Errorf("asset 1 = %+v", la.Assets[1])
	}
}

func TestParseFontFamilyStyle(t *testing.T) {
	input := `{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"t1","type":"Text","props":{"content":"Hi"},"style":{"fontFamily":"Comic Sans MS","fontSize":18}}]}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	uc := msg.Body.(UpdateComponents)
	s := uc.Components[0].Style
	if s.FontFamily == nil || s.FontFamily.Literal != "Comic Sans MS" {
		t.Errorf("fontFamily = %v, want Comic Sans MS", s.FontFamily)
	}
	if s.FontSize == nil || s.FontSize.Literal != 18 {
		t.Errorf("fontSize = %v, want 18", s.FontSize)
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

func TestParseDefineFunction(t *testing.T) {
	input := `{"type":"defineFunction","name":"double","params":["x"],"body":{"functionCall":{"name":"multiply","args":[{"param":"x"},2]}}}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != MsgDefineFunction {
		t.Errorf("type = %q, want defineFunction", msg.Type)
	}
	df := msg.Body.(DefineFunction)
	if df.Name != "double" {
		t.Errorf("name = %q, want double", df.Name)
	}
	if len(df.Params) != 1 || df.Params[0] != "x" {
		t.Errorf("params = %v, want [x]", df.Params)
	}
	if df.Body == nil {
		t.Fatal("body is nil")
	}
}

func TestParseDefineComponent(t *testing.T) {
	input := `{"type":"defineComponent","name":"Btn","params":["label"],"components":[{"componentId":"_root","type":"Button","props":{"label":{"param":"label"}}}]}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != MsgDefineComponent {
		t.Errorf("type = %q, want defineComponent", msg.Type)
	}
	dc := msg.Body.(DefineComponent)
	if dc.Name != "Btn" {
		t.Errorf("name = %q, want Btn", dc.Name)
	}
	if len(dc.Params) != 1 || dc.Params[0] != "label" {
		t.Errorf("params = %v, want [label]", dc.Params)
	}
	if len(dc.Components) != 1 {
		t.Fatalf("components = %d, want 1", len(dc.Components))
	}
}

func TestParseInclude(t *testing.T) {
	input := `{"type":"include","path":"components/buttons.jsonl"}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != MsgInclude {
		t.Errorf("type = %q, want include", msg.Type)
	}
	inc := msg.Body.(Include)
	if inc.Path != "components/buttons.jsonl" {
		t.Errorf("path = %q, want components/buttons.jsonl", inc.Path)
	}
}

func TestParseUseComponent(t *testing.T) {
	input := `{"type":"updateComponents","surfaceId":"s1","components":[{"componentId":"btn7","useComponent":"DigitButton","args":{"digit":"7","label":"7"}}]}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	uc := msg.Body.(UpdateComponents)
	if len(uc.Components) != 1 {
		t.Fatalf("components = %d, want 1", len(uc.Components))
	}
	comp := uc.Components[0]
	if comp.UseComponent != "DigitButton" {
		t.Errorf("useComponent = %q, want DigitButton", comp.UseComponent)
	}
	if comp.Args["digit"] != "7" {
		t.Errorf("args.digit = %v, want '7'", comp.Args["digit"])
	}
	if comp.Args["label"] != "7" {
		t.Errorf("args.label = %v, want '7'", comp.Args["label"])
	}
}

func TestParseCreateProcess(t *testing.T) {
	input := `{"type":"createProcess","processId":"bg","transport":{"type":"file","path":"bg.jsonl"}}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != MsgCreateProcess {
		t.Fatalf("expected createProcess, got %s", msg.Type)
	}
	cp := msg.Body.(CreateProcess)
	if cp.ProcessID != "bg" {
		t.Errorf("processId = %q, want bg", cp.ProcessID)
	}
	if cp.Transport.Type != "file" {
		t.Errorf("transport.type = %q, want file", cp.Transport.Type)
	}
	if cp.Transport.Path != "bg.jsonl" {
		t.Errorf("transport.path = %q, want bg.jsonl", cp.Transport.Path)
	}
}

func TestParseCreateProcessInterval(t *testing.T) {
	input := `{"type":"createProcess","processId":"tick","transport":{"type":"interval","interval":1000,"message":{"type":"updateDataModel","surfaceId":"main","ops":[{"op":"replace","path":"/tick","value":true}]}}}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	cp := msg.Body.(CreateProcess)
	if cp.ProcessID != "tick" {
		t.Errorf("processId = %q, want tick", cp.ProcessID)
	}
	if cp.Transport.Type != "interval" {
		t.Errorf("transport.type = %q, want interval", cp.Transport.Type)
	}
	if cp.Transport.Interval != 1000 {
		t.Errorf("transport.interval = %d, want 1000", cp.Transport.Interval)
	}
	if cp.Transport.Message == nil {
		t.Fatal("transport.message is nil")
	}
}

func TestParseCreateProcessLLM(t *testing.T) {
	input := `{"type":"createProcess","processId":"agent","transport":{"type":"llm","provider":"anthropic","model":"claude-sonnet-4-5-20250929","prompt":"You are a helper"}}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	cp := msg.Body.(CreateProcess)
	if cp.ProcessID != "agent" {
		t.Errorf("processId = %q, want agent", cp.ProcessID)
	}
	if cp.Transport.Type != "llm" {
		t.Errorf("transport.type = %q, want llm", cp.Transport.Type)
	}
	if cp.Transport.Provider != "anthropic" {
		t.Errorf("transport.provider = %q, want anthropic", cp.Transport.Provider)
	}
	if cp.Transport.Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("transport.model = %q, want claude-sonnet-4-5-20250929", cp.Transport.Model)
	}
	if cp.Transport.Prompt != "You are a helper" {
		t.Errorf("transport.prompt = %q, want 'You are a helper'", cp.Transport.Prompt)
	}
}

func TestParseStopProcess(t *testing.T) {
	input := `{"type":"stopProcess","processId":"bg"}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != MsgStopProcess {
		t.Fatalf("expected stopProcess, got %s", msg.Type)
	}
	sp := msg.Body.(StopProcess)
	if sp.ProcessID != "bg" {
		t.Errorf("processId = %q, want bg", sp.ProcessID)
	}
}

func TestParseSendToProcess(t *testing.T) {
	input := `{"type":"sendToProcess","processId":"agent","message":{"type":"updateDataModel","surfaceId":"main","ops":[{"op":"add","path":"/input","value":"hello"}]}}`
	p := NewParser(strings.NewReader(input))

	msg, err := p.Next()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != MsgSendToProcess {
		t.Fatalf("expected sendToProcess, got %s", msg.Type)
	}
	stp := msg.Body.(SendToProcess)
	if stp.ProcessID != "agent" {
		t.Errorf("processId = %q, want agent", stp.ProcessID)
	}
	if stp.Message == nil {
		t.Fatal("message is nil")
	}
}
