package engine

import (
	"encoding/json"
	"fmt"
	"canopy/protocol"
	"canopy/renderer"
	"strings"
)

// Resolver evaluates dynamic values against a data model and tracks bindings.
type Resolver struct {
	dm        *DataModel
	tracker   *BindingTracker
	evaluator *Evaluator
	assets    *AssetRegistry
}

func NewResolver(dm *DataModel, tracker *BindingTracker, evaluator *Evaluator) *Resolver {
	return &Resolver{dm: dm, tracker: tracker, evaluator: evaluator}
}

// Resolve takes a protocol Component and returns a RenderNode with all
// dynamic values resolved to concrete values.
func (r *Resolver) Resolve(comp *protocol.Component) *renderer.RenderNode {
	node := &renderer.RenderNode{
		ComponentID: comp.ComponentID,
		Type:        comp.Type,
	}

	// Resolve children
	if comp.Children != nil && comp.Children.Static != nil {
		node.ChildIDs = comp.Children.Static
	}

	// Resolve props
	p := &node.Props
	cp := &comp.Props

	switch comp.Type {
	case protocol.CompText:
		p.Content = r.resolveString(comp.ComponentID, cp.Content)
		p.Variant = cp.Variant
		if p.Variant == "" {
			p.Variant = "body"
		}
		p.MaxLines = cp.MaxLines

	case protocol.CompRow:
		p.Justify = cp.Justify
		p.Align = cp.Align
		p.Gap = cp.Gap
		if p.Gap == 0 {
			p.Gap = 8
		}
		p.Padding = cp.Padding

	case protocol.CompColumn:
		p.Justify = cp.Justify
		p.Align = cp.Align
		p.Gap = cp.Gap
		if p.Gap == 0 {
			p.Gap = 8
		}
		p.Padding = cp.Padding

	case protocol.CompCard:
		p.Title = r.resolveString(comp.ComponentID, cp.Title)
		p.Subtitle = r.resolveString(comp.ComponentID, cp.Subtitle)
		p.Collapsible = r.resolveBool(comp.ComponentID, cp.Collapsible)
		p.Collapsed = r.resolveBool(comp.ComponentID, cp.Collapsed)
		p.Padding = cp.Padding

	case protocol.CompButton:
		p.Label = r.resolveString(comp.ComponentID, cp.Label)
		p.Style = cp.Style
		if p.Style == "" {
			p.Style = "secondary"
		}
		p.Disabled = r.resolveBool(comp.ComponentID, cp.Disabled)

	case protocol.CompTextField:
		p.Placeholder = r.resolveString(comp.ComponentID, cp.Placeholder)
		if cp.Value != nil {
			p.Value = r.resolveString(comp.ComponentID, cp.Value)
		} else if cp.DataBinding != "" {
			// Auto-read from data binding path when value prop is not explicitly set
			if val, ok := r.dm.Get(cp.DataBinding); ok {
				p.Value = toString(val)
			}
		}
		p.InputType = cp.InputType
		if p.InputType == "" {
			p.InputType = "shortText"
		}
		p.ReadOnly = r.resolveBool(comp.ComponentID, cp.ReadOnly)
		p.DataBinding = cp.DataBinding

	case protocol.CompCheckBox:
		p.Label = r.resolveString(comp.ComponentID, cp.Label)
		p.Checked = r.resolveBool(comp.ComponentID, cp.Checked)
		p.DataBinding = cp.DataBinding

	case protocol.CompDivider:
		// No dynamic props

	case protocol.CompIcon:
		p.Name = r.resolveString(comp.ComponentID, cp.Name)
		p.Size = cp.Size
		if p.Size == 0 {
			p.Size = 16
		}

	case protocol.CompImage:
		p.Src = r.resolveString(comp.ComponentID, cp.Src)
		p.Alt = r.resolveString(comp.ComponentID, cp.Alt)
		p.Width = cp.Width
		p.Height = cp.Height

	case protocol.CompSlider:
		p.Min = r.resolveNumber(comp.ComponentID, cp.Min)
		p.Max = r.resolveNumber(comp.ComponentID, cp.Max)
		if p.Max == 0 && cp.Max == nil {
			p.Max = 100
		}
		p.Step = r.resolveNumber(comp.ComponentID, cp.Step)
		if p.Step == 0 && cp.Step == nil {
			p.Step = 1
		}
		p.SliderValue = r.resolveNumber(comp.ComponentID, cp.SliderValue)
		p.DataBinding = cp.DataBinding

	case protocol.CompChoicePicker:
		p.Options = r.resolveOptions(cp.Options)
		p.Selected = r.resolveStringList(comp.ComponentID, cp.Selected)
		p.MutuallyExclusive = r.resolveBool(comp.ComponentID, cp.MutuallyExclusive)
		p.DataBinding = cp.DataBinding

	case protocol.CompDateTimeInput:
		p.EnableDate = r.resolveBoolDefault(comp.ComponentID, cp.EnableDate, true)
		p.EnableTime = r.resolveBool(comp.ComponentID, cp.EnableTime)
		p.DateValue = r.resolveString(comp.ComponentID, cp.DateValue)
		p.DataBinding = cp.DataBinding

	case protocol.CompList:
		p.Justify = cp.Justify
		p.Align = cp.Align
		p.Gap = cp.Gap
		if p.Gap == 0 {
			p.Gap = 8
		}
		p.Padding = cp.Padding

	case protocol.CompTabs:
		p.TabLabels = r.resolveTabLabels(cp.TabLabels)
		p.ActiveTab = r.resolveString(comp.ComponentID, cp.ActiveTab)
		p.DataBinding = cp.DataBinding

	case protocol.CompModal:
		p.Title = r.resolveString(comp.ComponentID, cp.Title)
		p.Visible = r.resolveBool(comp.ComponentID, cp.Visible)
		p.DataBinding = cp.DataBinding
		p.Width = cp.Width
		p.Height = cp.Height

	case protocol.CompVideo:
		p.Src = r.resolveString(comp.ComponentID, cp.Src)
		p.Width = cp.Width
		p.Height = cp.Height
		p.Autoplay = r.resolveBool(comp.ComponentID, cp.Autoplay)
		p.Loop = r.resolveBool(comp.ComponentID, cp.Loop)
		p.Controls = r.resolveBoolDefault(comp.ComponentID, cp.Controls, true)
		p.Muted = r.resolveBool(comp.ComponentID, cp.Muted)

	case protocol.CompAudioPlayer:
		p.Src = r.resolveString(comp.ComponentID, cp.Src)
		p.Autoplay = r.resolveBool(comp.ComponentID, cp.Autoplay)
		p.Loop = r.resolveBool(comp.ComponentID, cp.Loop)

	case protocol.CompSplitView:
		p.DividerStyle = cp.DividerStyle
		if p.DividerStyle == "" {
			p.DividerStyle = "thin"
		}
		p.Vertical = r.resolveBoolDefault(comp.ComponentID, cp.Vertical, true)
		if cp.CollapsedPane != nil {
			p.CollapsedPane = int(r.resolveNumber(comp.ComponentID, cp.CollapsedPane))
		} else {
			p.CollapsedPane = -1
		}

	case protocol.CompSearchField:
		p.Placeholder = r.resolveString(comp.ComponentID, cp.Placeholder)
		if cp.Value != nil {
			p.Value = r.resolveString(comp.ComponentID, cp.Value)
		} else if cp.DataBinding != "" {
			if val, ok := r.dm.Get(cp.DataBinding); ok {
				p.Value = toString(val)
			}
		}
		p.DataBinding = cp.DataBinding

	case protocol.CompOutlineView:
		p.OutlineData = r.resolveJSON(comp.ComponentID, cp.OutlineData)
		p.LabelKey = cp.LabelKey
		if p.LabelKey == "" {
			p.LabelKey = "name"
		}
		p.ChildrenKey = cp.ChildrenKey
		if p.ChildrenKey == "" {
			p.ChildrenKey = "children"
		}
		p.IconKey = cp.IconKey
		p.IDKey = cp.IDKey
		if p.IDKey == "" {
			p.IDKey = "id"
		}
		p.SelectedID = r.resolveString(comp.ComponentID, cp.SelectedID)
		p.BadgeKey = cp.BadgeKey
		p.DataBinding = cp.DataBinding

	case protocol.CompProgressBar:
		p.Min = r.resolveNumber(comp.ComponentID, cp.Min)
		p.Max = r.resolveNumber(comp.ComponentID, cp.Max)
		if p.Max == 0 && cp.Max == nil {
			p.Max = 100
		}
		p.ProgressValue = r.resolveNumber(comp.ComponentID, cp.ProgressValue)
		p.Indeterminate = r.resolveBool(comp.ComponentID, cp.Indeterminate)

	case protocol.CompRichTextEditor:
		p.RichContent = r.resolveString(comp.ComponentID, cp.RichContent)
		p.Editable = r.resolveBoolDefault(comp.ComponentID, cp.Editable, true)
		p.DataBinding = cp.DataBinding
		p.FormatBinding = cp.FormatBinding

	case protocol.CompCameraView:
		p.DevicePosition = r.resolveString(comp.ComponentID, cp.DevicePosition)
		if p.DevicePosition == "" {
			p.DevicePosition = "front"
		}
		p.Mirrored = r.resolveBoolDefault(comp.ComponentID, cp.Mirrored, true)

	case protocol.CompAudioRecorder:
		p.Format = cp.Format
		if p.Format == "" {
			p.Format = "m4a"
		}
		p.SampleRate = r.resolveNumber(comp.ComponentID, cp.SampleRate)
		if p.SampleRate == 0 {
			p.SampleRate = 44100
		}
		p.RecordChannels = cp.RecordChannels
		if p.RecordChannels == 0 {
			p.RecordChannels = 1
		}
	}

	node.Style = r.resolveStyle(comp.ComponentID, comp.Style)
	return node
}

// resolveStyle resolves all dynamic style fields into concrete ResolvedStyleProps.
func (r *Resolver) resolveStyle(componentID string, ds protocol.DynamicStyleProps) renderer.ResolvedStyleProps {
	return renderer.ResolvedStyleProps{
		BackgroundColor: r.resolveString(componentID, ds.BackgroundColor),
		TextColor:       r.resolveString(componentID, ds.TextColor),
		CornerRadius:    r.resolveNumber(componentID, ds.CornerRadius),
		Width:           r.resolveNumber(componentID, ds.Width),
		Height:          r.resolveNumber(componentID, ds.Height),
		FontSize:        r.resolveNumber(componentID, ds.FontSize),
		FontWeight:      r.resolveString(componentID, ds.FontWeight),
		TextAlign:       r.resolveString(componentID, ds.TextAlign),
		Opacity:         r.resolveNumber(componentID, ds.Opacity),
		FontFamily:      r.resolveString(componentID, ds.FontFamily),
		FlexGrow:        r.resolveNumber(componentID, ds.FlexGrow),
	}
}

func (r *Resolver) resolveString(componentID string, dv *protocol.DynamicString) string {
	if dv == nil {
		return ""
	}
	var result string
	if dv.IsFunc && dv.FunctionCall != nil {
		r.registerFuncBindings(componentID, dv.FunctionCall.Args)
		val, err := r.evaluator.Eval(dv.FunctionCall.Name, dv.FunctionCall.Args)
		if err != nil {
			logWarn("resolver", componentID, fmt.Sprintf("evaluator error in %s: %v", dv.FunctionCall.Name, err))
			return ""
		}
		result = toString(val)
	} else if dv.IsPath {
		r.tracker.Register(dv.Path, componentID)
		val, ok := r.dm.Get(dv.Path)
		if !ok {
			return ""
		}
		result = fmt.Sprintf("%v", val)
	} else {
		result = dv.Literal
	}
	return r.resolveAssetRef(result)
}

// resolveAssetRef replaces "asset:<alias>" references with the registered src.
func (r *Resolver) resolveAssetRef(val string) string {
	if r.assets == nil {
		return val
	}
	if alias, ok := strings.CutPrefix(val, "asset:"); ok {
		if src := r.assets.Resolve(alias); src != "" {
			return src
		}
	}
	return val
}

// resolveJSON resolves a dynamic string but serializes complex values (arrays, maps) as JSON.
func (r *Resolver) resolveJSON(componentID string, dv *protocol.DynamicString) string {
	if dv == nil {
		return ""
	}
	if dv.IsPath {
		r.tracker.Register(dv.Path, componentID)
		val, ok := r.dm.Get(dv.Path)
		if !ok {
			return ""
		}
		switch val.(type) {
		case string:
			return val.(string)
		default:
			data, err := json.Marshal(val)
			if err != nil {
				return ""
			}
			return string(data)
		}
	}
	return r.resolveString(componentID, dv)
}

func (r *Resolver) resolveNumber(componentID string, dv *protocol.DynamicNumber) float64 {
	if dv == nil {
		return 0
	}
	if dv.IsFunc && dv.FunctionCall != nil {
		r.registerFuncBindings(componentID, dv.FunctionCall.Args)
		val, err := r.evaluator.Eval(dv.FunctionCall.Name, dv.FunctionCall.Args)
		if err != nil {
			logWarn("resolver", componentID, fmt.Sprintf("evaluator error in %s: %v", dv.FunctionCall.Name, err))
			return 0
		}
		f, _ := toFloat(val)
		return f
	}
	if dv.IsPath {
		r.tracker.Register(dv.Path, componentID)
		val, ok := r.dm.Get(dv.Path)
		if !ok {
			return 0
		}
		if f, ok := val.(float64); ok {
			return f
		}
		return 0
	}
	return dv.Literal
}

func (r *Resolver) resolveBool(componentID string, dv *protocol.DynamicBoolean) bool {
	if dv == nil {
		return false
	}
	if dv.IsFunc && dv.FunctionCall != nil {
		r.registerFuncBindings(componentID, dv.FunctionCall.Args)
		val, err := r.evaluator.Eval(dv.FunctionCall.Name, dv.FunctionCall.Args)
		if err != nil {
			logWarn("resolver", componentID, fmt.Sprintf("evaluator error in %s: %v", dv.FunctionCall.Name, err))
			return false
		}
		b, _ := toBool(val)
		return b
	}
	if dv.IsPath {
		r.tracker.Register(dv.Path, componentID)
		val, ok := r.dm.Get(dv.Path)
		if !ok {
			return false
		}
		if b, ok := val.(bool); ok {
			return b
		}
		return false
	}
	return dv.Literal
}

func (r *Resolver) resolveBoolDefault(componentID string, dv *protocol.DynamicBoolean, defaultVal bool) bool {
	if dv == nil {
		return defaultVal
	}
	return r.resolveBool(componentID, dv)
}

func (r *Resolver) resolveStringList(componentID string, dv *protocol.DynamicStringList) []string {
	if dv == nil {
		return nil
	}
	if dv.IsPath {
		r.tracker.Register(dv.Path, componentID)
		val, ok := r.dm.Get(dv.Path)
		if !ok {
			return nil
		}
		if arr, ok := val.([]interface{}); ok {
			result := make([]string, len(arr))
			for i, item := range arr {
				result[i] = fmt.Sprintf("%v", item)
			}
			return result
		}
		if arr, ok := val.([]string); ok {
			return arr
		}
		return nil
	}
	return dv.Literal
}

func (r *Resolver) resolveOptions(raw []byte) []renderer.OptionItem {
	if len(raw) == 0 {
		return nil
	}
	var items []renderer.OptionItem
	// Try parsing as array of {label, value} objects
	type rawOption struct {
		Label string `json:"label"`
		Value string `json:"value"`
	}
	var opts []rawOption
	if err := json.Unmarshal(raw, &opts); err != nil {
		return nil
	}
	for _, o := range opts {
		items = append(items, renderer.OptionItem{Label: o.Label, Value: o.Value})
	}
	return items
}

func (r *Resolver) resolveTabLabels(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var labels []string
	if err := json.Unmarshal(raw, &labels); err != nil {
		return nil
	}
	return labels
}

// registerFuncBindings walks function call args and registers bindings for any path references.
func (r *Resolver) registerFuncBindings(componentID string, args []interface{}) {
	for _, path := range PathsInArgs(args) {
		r.tracker.Register(path, componentID)
	}
}
