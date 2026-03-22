package protocol

import "encoding/json"

// ComponentType identifies A2UI component kinds.
type ComponentType string

const (
	CompText          ComponentType = "Text"
	CompRow           ComponentType = "Row"
	CompColumn        ComponentType = "Column"
	CompCard          ComponentType = "Card"
	CompButton        ComponentType = "Button"
	CompTextField     ComponentType = "TextField"
	CompCheckBox      ComponentType = "CheckBox"
	CompSlider        ComponentType = "Slider"
	CompImage         ComponentType = "Image"
	CompIcon          ComponentType = "Icon"
	CompDivider       ComponentType = "Divider"
	CompList          ComponentType = "List"
	CompTabs          ComponentType = "Tabs"
	CompModal         ComponentType = "Modal"
	CompChoicePicker  ComponentType = "ChoicePicker"
	CompDateTimeInput ComponentType = "DateTimeInput"
	CompVideo         ComponentType = "Video"
	CompAudioPlayer   ComponentType = "AudioPlayer"
	CompSplitView     ComponentType = "SplitView"
	CompSearchField   ComponentType = "SearchField"
	CompOutlineView   ComponentType = "OutlineView"
	CompRichTextEditor ComponentType = "RichTextEditor"
	CompProgressBar    ComponentType = "ProgressBar"
)

// DynamicStyleProps holds visual styling overrides applicable to any component.
// All fields accept dynamic values (path references, function calls, or literals).
type DynamicStyleProps struct {
	BackgroundColor *DynamicString `json:"backgroundColor,omitempty"`
	TextColor       *DynamicString `json:"textColor,omitempty"`
	CornerRadius    *DynamicNumber `json:"cornerRadius,omitempty"`
	Width           *DynamicNumber `json:"width,omitempty"`
	Height          *DynamicNumber `json:"height,omitempty"`
	FontSize        *DynamicNumber `json:"fontSize,omitempty"`
	FontWeight      *DynamicString `json:"fontWeight,omitempty"` // bold, medium, light
	TextAlign       *DynamicString `json:"textAlign,omitempty"`  // left, center, right
	Opacity         *DynamicNumber `json:"opacity,omitempty"`
	FontFamily      *DynamicString `json:"fontFamily,omitempty"`
	FlexGrow        *DynamicNumber `json:"flexGrow,omitempty"` // expand to fill available space in parent stack
}

// Component is a single A2UI component definition.
type Component struct {
	ComponentID  string                 `json:"componentId"`
	Type         ComponentType          `json:"type,omitempty"`
	ParentID     string                 `json:"parentId,omitempty"`
	Children     *ChildList             `json:"children,omitempty"`
	Props        Props                  `json:"props,omitempty"`
	Style        DynamicStyleProps      `json:"style,omitempty"`
	UseComponent string                 `json:"useComponent,omitempty"`
	Args         map[string]interface{} `json:"args,omitempty"`
	Scope        string                 `json:"scope,omitempty"`
}

// componentTypeAliases maps common LLM-hallucinated type names to valid types.
var componentTypeAliases = map[ComponentType]ComponentType{
	"ScrollView": CompList,
	"Scroll":     CompList,
	"Stack":      CompColumn,
	"VStack":     CompColumn,
	"HStack":     CompRow,
	"Label":      CompText,
	"Input":      CompTextField,
	"TextInput":  CompTextField,
	"Separator":  CompDivider,
}

// UnmarshalJSON handles both "id" and "componentId" for the component ID field,
// and normalizes hallucinated component type names to valid types.
func (c *Component) UnmarshalJSON(data []byte) error {
	type Alias Component
	aux := &struct {
		ID string `json:"id"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	// "id" is the alias — use it if componentId was not set
	if c.ComponentID == "" && aux.ID != "" {
		c.ComponentID = aux.ID
	}
	// Normalize hallucinated type names
	if mapped, ok := componentTypeAliases[c.Type]; ok {
		c.Type = mapped
	}
	return nil
}

// Props holds all possible component properties across types.
// Only the fields relevant to a component's Type are used.
type Props struct {
	// Text
	Content  *DynamicString `json:"content,omitempty"`
	Variant  string         `json:"variant,omitempty"` // h1, h2, h3, h4, h5, body, caption
	MaxLines int            `json:"maxLines,omitempty"` // 0 = unlimited (default), 1+ = truncate with ellipsis

	// Layout (Row, Column)
	Justify string `json:"justify,omitempty"` // start, center, end, spaceBetween, spaceAround
	Align   string `json:"align,omitempty"`   // start, center, end, stretch
	Gap     int    `json:"gap,omitempty"`
	Padding int    `json:"padding,omitempty"`

	// Card
	Title       *DynamicString `json:"title,omitempty"`
	Subtitle    *DynamicString `json:"subtitle,omitempty"`
	Collapsible *DynamicBoolean `json:"collapsible,omitempty"`
	Collapsed   *DynamicBoolean `json:"collapsed,omitempty"`

	// Button
	Label    *DynamicString  `json:"label,omitempty"`
	Style    string          `json:"style,omitempty"` // primary, secondary, destructive
	Disabled *DynamicBoolean `json:"disabled,omitempty"`
	OnClick  *EventAction    `json:"onClick,omitempty"`

	// TextField
	Placeholder  *DynamicString  `json:"placeholder,omitempty"`
	Value        *DynamicString  `json:"value,omitempty"`
	InputType    string          `json:"inputType,omitempty"` // shortText, longText, number, obscured
	ReadOnly     *DynamicBoolean `json:"readOnly,omitempty"`
	OnChange     *EventAction    `json:"onChange,omitempty"`
	DataBinding  string          `json:"dataBinding,omitempty"` // JSON Pointer for two-way binding
	Validations  json.RawMessage `json:"validations,omitempty"`

	// CheckBox
	Checked     *DynamicBoolean `json:"checked,omitempty"`
	OnToggle    *EventAction    `json:"onToggle,omitempty"`

	// Slider
	Min         *DynamicNumber `json:"min,omitempty"`
	Max         *DynamicNumber `json:"max,omitempty"`
	Step        *DynamicNumber `json:"step,omitempty"`
	SliderValue *DynamicNumber `json:"sliderValue,omitempty"`
	OnSlide     *EventAction   `json:"onSlide,omitempty"`

	// Image
	Src     *DynamicString `json:"src,omitempty"`
	Alt     *DynamicString `json:"alt,omitempty"`
	Width   int            `json:"width,omitempty"`
	Height  int            `json:"height,omitempty"`

	// Icon
	Name *DynamicString `json:"name,omitempty"`
	Size int            `json:"size,omitempty"`

	// Tabs
	TabLabels json.RawMessage `json:"tabLabels,omitempty"` // ["Label1", "Label2", ...]
	ActiveTab *DynamicString   `json:"activeTab,omitempty"` // selected child ID

	// Modal
	Visible   *DynamicBoolean `json:"visible,omitempty"`
	OnDismiss *EventAction    `json:"onDismiss,omitempty"`

	// Video
	Autoplay *DynamicBoolean `json:"autoplay,omitempty"`
	Loop     *DynamicBoolean `json:"loop,omitempty"`
	Controls *DynamicBoolean `json:"controls,omitempty"`
	Muted    *DynamicBoolean `json:"muted,omitempty"`
	OnEnded  *EventAction    `json:"onEnded,omitempty"`

	// ChoicePicker
	Options           json.RawMessage `json:"options,omitempty"`
	Selected          *DynamicStringList `json:"selected,omitempty"`
	MutuallyExclusive *DynamicBoolean    `json:"mutuallyExclusive,omitempty"`
	OnSelect          *EventAction       `json:"onSelect,omitempty"`

	// DateTimeInput
	EnableDate *DynamicBoolean `json:"enableDate,omitempty"`
	EnableTime *DynamicBoolean `json:"enableTime,omitempty"`
	DateValue  *DynamicString  `json:"dateValue,omitempty"`
	OnDateChange *EventAction  `json:"onDateChange,omitempty"`

	// SplitView
	DividerStyle  string          `json:"dividerStyle,omitempty"`  // thin, thick, paneSplitter
	Vertical      *DynamicBoolean `json:"vertical,omitempty"`      // default true (vertical dividers = horizontal panes)
	CollapsedPane *DynamicNumber  `json:"collapsedPane,omitempty"` // pane index to collapse (-1 = none)

	// SearchField
	OnSearch *EventAction `json:"onSearch,omitempty"`

	// OutlineView
	OutlineData *DynamicString `json:"outlineData,omitempty"` // JSON pointer to tree array
	LabelKey    string         `json:"labelKey,omitempty"`    // key for display text (default "name")
	ChildrenKey string         `json:"childrenKey,omitempty"` // key for nested items (default "children")
	IconKey     string         `json:"iconKey,omitempty"`     // key for SF Symbol name
	IDKey       string         `json:"idKey,omitempty"`       // key for item identifier (default "id")
	SelectedID  *DynamicString `json:"selectedId,omitempty"`  // currently selected item ID
	BadgeKey    string         `json:"badgeKey,omitempty"`    // key for numeric badge (hidden when 0)

	// RichTextEditor
	RichContent   *DynamicString  `json:"richContent,omitempty"`   // markdown content
	Editable      *DynamicBoolean `json:"editable,omitempty"`      // default true
	OnRichChange  *EventAction    `json:"onRichChange,omitempty"`  // fired on content change
	FormatBinding string          `json:"formatBinding,omitempty"` // JSON Pointer for cursor format state

	// ProgressBar
	ProgressValue   *DynamicNumber  `json:"progressValue,omitempty"`
	Indeterminate   *DynamicBoolean `json:"indeterminate,omitempty"`

	// Universal props
	ContextMenu json.RawMessage `json:"contextMenu,omitempty"` // []MenuItem for right-click menu
}
