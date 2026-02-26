package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "menu.h"
*/
import "C"
import (
	"encoding/json"
	"jview/renderer"
	"unsafe"
)

type menuItemJSON struct {
	ID             string         `json:"id,omitempty"`
	Label          string         `json:"label,omitempty"`
	KeyEquivalent  string         `json:"keyEquivalent,omitempty"`
	KeyModifiers   string         `json:"keyModifiers,omitempty"`
	Separator      bool           `json:"separator,omitempty"`
	StandardAction string         `json:"standardAction,omitempty"`
	CallbackID     uint64         `json:"callbackID,omitempty"`
	Children       []menuItemJSON `json:"children,omitempty"`
	Icon           string         `json:"icon,omitempty"`
	Disabled       bool           `json:"disabled,omitempty"`
}

func specToJSON(specs []renderer.MenuItemSpec) []menuItemJSON {
	items := make([]menuItemJSON, len(specs))
	for i, s := range specs {
		items[i] = menuItemJSON{
			ID:             s.ID,
			Label:          s.Label,
			KeyEquivalent:  s.KeyEquivalent,
			KeyModifiers:   s.KeyModifiers,
			Separator:      s.Separator,
			StandardAction: s.StandardAction,
			CallbackID:     uint64(s.CallbackID),
			Icon:           s.Icon,
			Disabled:       s.Disabled,
		}
		if len(s.Children) > 0 {
			items[i].Children = specToJSON(s.Children)
		}
	}
	return items
}

func updateMenu(surfaceID string, items []renderer.MenuItemSpec) {
	jsonItems := specToJSON(items)
	data, err := json.Marshal(jsonItems)
	if err != nil {
		return
	}

	cSID := C.CString(surfaceID)
	defer C.free(unsafe.Pointer(cSID))
	cJSON := C.CString(string(data))
	defer C.free(unsafe.Pointer(cJSON))

	C.JVUpdateMenu(cSID, cJSON)
}

func performAction(selector string) {
	cSel := C.CString(selector)
	defer C.free(unsafe.Pointer(cSel))
	C.JVPerformAction(cSel)
}
