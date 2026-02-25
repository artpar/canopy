package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "toolbar.h"
*/
import "C"
import (
	"encoding/json"
	"jview/renderer"
	"unsafe"
)

type toolbarItemJSON struct {
	ID               string `json:"id,omitempty"`
	Icon             string `json:"icon,omitempty"`
	Label            string `json:"label,omitempty"`
	StandardAction   string `json:"standardAction,omitempty"`
	CallbackID       uint64 `json:"callbackID,omitempty"`
	Separator        bool   `json:"separator,omitempty"`
	Flexible         bool   `json:"flexible,omitempty"`
	SearchField      bool   `json:"searchField,omitempty"`
	SearchCallbackID uint64 `json:"searchCallbackID,omitempty"`
}

func updateToolbar(surfaceID string, items []renderer.ToolbarItemSpec) {
	jsonItems := make([]toolbarItemJSON, len(items))
	for i, s := range items {
		jsonItems[i] = toolbarItemJSON{
			ID:               s.ID,
			Icon:             s.Icon,
			Label:            s.Label,
			StandardAction:   s.StandardAction,
			CallbackID:       uint64(s.CallbackID),
			Separator:        s.Separator,
			Flexible:         s.Flexible,
			SearchField:      s.SearchField,
			SearchCallbackID: uint64(s.SearchCallbackID),
		}
	}
	data, err := json.Marshal(jsonItems)
	if err != nil {
		return
	}

	cSID := C.CString(surfaceID)
	defer C.free(unsafe.Pointer(cSID))
	cJSON := C.CString(string(data))
	defer C.free(unsafe.Pointer(cJSON))

	C.JVUpdateToolbar(cSID, cJSON)
}
