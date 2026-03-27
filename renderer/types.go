package renderer

import "jview/protocol"

// ViewHandle is an opaque pointer to a native view.
type ViewHandle uintptr

// CallbackID identifies a registered callback in the platform layer.
type CallbackID uint64

// ResolvedStyleProps contains all resolved (concrete) style property values.
// Dynamic values have been evaluated against the data model.
type ResolvedStyleProps struct {
	BackgroundColor string  `json:"backgroundColor,omitempty"`
	TextColor       string  `json:"textColor,omitempty"`
	CornerRadius    float64 `json:"cornerRadius,omitempty"`
	Width           float64 `json:"width,omitempty"`
	Height          float64 `json:"height,omitempty"`
	FontSize        float64 `json:"fontSize,omitempty"`
	FontWeight      string  `json:"fontWeight,omitempty"` // bold, medium, light
	TextAlign       string  `json:"textAlign,omitempty"`  // left, center, right
	Opacity         float64 `json:"opacity,omitempty"`
	FontFamily      string  `json:"fontFamily,omitempty"`
	FlexGrow        float64 `json:"flexGrow,omitempty"`
}

// RenderNode holds the resolved properties for a single component,
// ready for the platform renderer to create or update a native view.
type RenderNode struct {
	ComponentID string
	Type        protocol.ComponentType
	Props       ResolvedProps
	Style       ResolvedStyleProps
	ChildIDs    []string
	Callbacks   map[string]CallbackID // eventType → CallbackID
}

// ResolvedProps contains all resolved (concrete) property values.
// Dynamic values have been evaluated against the data model.
type ResolvedProps struct {
	// Text
	Content  string `json:"content,omitempty"`
	Variant  string `json:"variant,omitempty"`
	MaxLines int    `json:"maxLines,omitempty"`

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

	// Tabs
	TabLabels []string `json:"tabLabels,omitempty"`
	ActiveTab string   `json:"activeTab,omitempty"`

	// Modal
	Visible bool `json:"visible,omitempty"`

	// Video
	Autoplay bool `json:"autoplay,omitempty"`
	Loop     bool `json:"loop,omitempty"`
	Controls bool `json:"controls,omitempty"`
	Muted    bool `json:"muted,omitempty"`

	// SplitView
	DividerStyle  string `json:"dividerStyle,omitempty"`
	Vertical      bool   `json:"vertical,omitempty"`
	CollapsedPane int    `json:"collapsedPane,omitempty"`

	// SearchField (reuses Placeholder, Value, DataBinding)

	// OutlineView
	OutlineData string `json:"outlineData,omitempty"`
	LabelKey    string `json:"labelKey,omitempty"`
	ChildrenKey string `json:"childrenKey,omitempty"`
	IconKey     string `json:"iconKey,omitempty"`
	IDKey       string `json:"idKey,omitempty"`
	SelectedID  string `json:"selectedId,omitempty"`
	BadgeKey    string `json:"badgeKey,omitempty"`

	// RichTextEditor
	RichContent   string `json:"richContent,omitempty"`
	Editable      bool   `json:"editable,omitempty"`
	FormatBinding string `json:"formatBinding,omitempty"`

	// ProgressBar
	ProgressValue float64 `json:"progressValue,omitempty"`
	Indeterminate bool    `json:"indeterminate,omitempty"`

	// Universal
	ContextMenu string `json:"contextMenu,omitempty"` // serialized []MenuItemSpec JSON
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

// MenuItemSpec describes a menu item for the native menu bar.
type MenuItemSpec struct {
	ID             string
	Label          string
	KeyEquivalent  string
	KeyModifiers   string // "option", "shift", "option+shift" — Cmd always included
	Separator      bool
	StandardAction string
	CallbackID     CallbackID
	Children       []MenuItemSpec
	Icon           string
	Disabled       bool
}

// ToolbarItemSpec describes a single toolbar item for the native toolbar.
type ToolbarItemSpec struct {
	ID               string
	Icon             string // SF Symbol name
	Label            string
	StandardAction   string
	CallbackID       CallbackID
	Separator        bool
	Flexible         bool
	SearchField      bool
	SearchCallbackID CallbackID // for search field change events
	Enabled          bool
	Selected         bool
	HasToggle        bool // item has a selected binding (renders as toggle button)
	Bordered         bool // rounded button appearance
}

// WindowSpec describes a new window to create.
type WindowSpec struct {
	SurfaceID       string
	Title           string
	Width           int
	Height          int
	BackgroundColor string
	Padding         int
}
