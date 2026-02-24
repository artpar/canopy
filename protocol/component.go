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
)

// StyleProps holds visual styling overrides applicable to any component.
type StyleProps struct {
	BackgroundColor string  `json:"backgroundColor,omitempty"`
	TextColor       string  `json:"textColor,omitempty"`
	CornerRadius    float64 `json:"cornerRadius,omitempty"`
	Width           float64 `json:"width,omitempty"`
	Height          float64 `json:"height,omitempty"`
	FontSize        float64 `json:"fontSize,omitempty"`
	FontWeight      string  `json:"fontWeight,omitempty"` // bold, medium, light
	TextAlign       string  `json:"textAlign,omitempty"`  // left, center, right
	Opacity         float64 `json:"opacity,omitempty"`
}

// Component is a single A2UI component definition.
type Component struct {
	ComponentID string        `json:"componentId"`
	Type        ComponentType `json:"type"`
	ParentID    string        `json:"parentId,omitempty"`
	Children    *ChildList    `json:"children,omitempty"`
	Props       Props         `json:"props,omitempty"`
	Style       StyleProps    `json:"style,omitempty"`
}

// Props holds all possible component properties across types.
// Only the fields relevant to a component's Type are used.
type Props struct {
	// Text
	Content *DynamicString `json:"content,omitempty"`
	Variant string         `json:"variant,omitempty"` // h1, h2, h3, h4, h5, body, caption

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
	Tabs json.RawMessage `json:"tabs,omitempty"` // deferred to Phase 3

	// Modal
	Visible   *DynamicBoolean `json:"visible,omitempty"`
	OnDismiss *EventAction    `json:"onDismiss,omitempty"`

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
}
