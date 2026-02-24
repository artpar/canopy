package renderer

import "jview/protocol"

// ViewHandle is an opaque pointer to a native view.
type ViewHandle uintptr

// CallbackID identifies a registered callback in the platform layer.
type CallbackID uint64

// RenderNode holds the resolved properties for a single component,
// ready for the platform renderer to create or update a native view.
type RenderNode struct {
	ComponentID string
	Type        protocol.ComponentType
	Props       ResolvedProps
	Style       protocol.StyleProps
	ChildIDs    []string
	Callbacks   map[string]CallbackID // eventType → CallbackID
}

// ResolvedProps contains all resolved (concrete) property values.
// Dynamic values have been evaluated against the data model.
type ResolvedProps struct {
	// Text
	Content string `json:"content,omitempty"`
	Variant string `json:"variant,omitempty"`

	// Layout
	Justify string `json:"justify,omitempty"`
	Align   string `json:"align,omitempty"`
	Gap     int    `json:"gap,omitempty"`
	Padding int    `json:"padding,omitempty"`

	// Card
	Title       string `json:"title,omitempty"`
	Subtitle    string `json:"subtitle,omitempty"`
	Collapsible bool   `json:"collapsible,omitempty"`
	Collapsed   bool   `json:"collapsed,omitempty"`

	// Button
	Label    string `json:"label,omitempty"`
	Style    string `json:"style,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`

	// TextField
	Placeholder      string   `json:"placeholder,omitempty"`
	Value            string   `json:"value,omitempty"`
	InputType        string   `json:"inputType,omitempty"`
	ReadOnly         bool     `json:"readOnly,omitempty"`
	DataBinding      string   `json:"dataBinding,omitempty"`
	ValidationErrors []string `json:"validationErrors,omitempty"`

	// CheckBox
	Checked bool `json:"checked,omitempty"`

	// Slider
	Min         float64 `json:"min,omitempty"`
	Max         float64 `json:"max,omitempty"`
	Step        float64 `json:"step,omitempty"`
	SliderValue float64 `json:"sliderValue,omitempty"`

	// Image
	Src    string `json:"src,omitempty"`
	Alt    string `json:"alt,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`

	// Icon
	Name string `json:"name,omitempty"`
	Size int    `json:"size,omitempty"`

	// ChoicePicker
	Options           []OptionItem `json:"options,omitempty"`
	Selected          []string     `json:"selected,omitempty"`
	MutuallyExclusive bool         `json:"mutuallyExclusive,omitempty"`

	// DateTimeInput
	EnableDate bool   `json:"enableDate,omitempty"`
	EnableTime bool   `json:"enableTime,omitempty"`
	DateValue  string `json:"dateValue,omitempty"`
}

// OptionItem represents a single option in a ChoicePicker.
type OptionItem struct {
	Label string `json:"label,omitempty"`
	Value string `json:"value,omitempty"`
}

// AssetSpec describes a single asset to be loaded by the platform renderer.
type AssetSpec struct {
	Alias string
	Kind  string // "font", "image", "audio", "video"
	Src   string
}

// LayoutInfo holds computed layout properties for a view, used by the test framework.
type LayoutInfo struct {
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
}

// StyleInfo holds computed style properties for a view, used by the test framework.
type StyleInfo struct {
	FontName  string  `json:"fontName,omitempty"`
	FontSize  float64 `json:"fontSize,omitempty"`
	Bold      bool    `json:"bold,omitempty"`
	Italic    bool    `json:"italic,omitempty"`
	TextColor string  `json:"textColor,omitempty"` // hex like "#FF0000"
	BgColor   string  `json:"bgColor,omitempty"`   // hex like "#FFFFFF"
	Hidden    bool    `json:"hidden,omitempty"`
	Opacity   float64 `json:"opacity,omitempty"`
}

// RenderOp is a single rendering operation to be dispatched to the main thread.
type RenderOp struct {
	Kind       RenderOpKind
	Node       *RenderNode
	Handle     ViewHandle
	ParentID   string
	CallbackID CallbackID
}

type RenderOpKind int

const (
	OpCreateView RenderOpKind = iota
	OpUpdateView
	OpSetChildren
	OpRemoveView
	OpCreateWindow
)

// WindowSpec describes a new window to create.
type WindowSpec struct {
	SurfaceID       string
	Title           string
	Width           int
	Height          int
	BackgroundColor string
	Padding         int
}
