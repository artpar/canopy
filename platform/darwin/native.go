package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework UserNotifications -framework UniformTypeIdentifiers

#include "native.h"
#include <stdlib.h>
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

// NativeProvider implements renderer.NativeProvider using macOS native APIs.
type NativeProvider struct{}

// NewNativeProvider creates a new NativeProvider.
func NewNativeProvider() *NativeProvider {
	return &NativeProvider{}
}

func (n *NativeProvider) Notify(title, body, subtitle string) error {
	cTitle := C.CString(title)
	cBody := C.CString(body)
	cSubtitle := C.CString(subtitle)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cBody))
	defer C.free(unsafe.Pointer(cSubtitle))
	C.JVSendNotification(cTitle, cBody, cSubtitle)
	return nil
}

func (n *NativeProvider) ClipboardRead() (string, error) {
	cStr := C.JVClipboardRead()
	if cStr == nil {
		return "", nil
	}
	defer C.free(unsafe.Pointer(cStr))
	return C.GoString(cStr), nil
}

func (n *NativeProvider) ClipboardWrite(text string) error {
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))
	C.JVClipboardWrite(cText)
	return nil
}

func (n *NativeProvider) OpenURL(url string) error {
	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))
	C.JVOpenURL(cURL)
	return nil
}

func (n *NativeProvider) FileOpen(title string, allowedTypes string, allowMultiple bool) ([]string, error) {
	cTitle := C.CString(title)
	cTypes := C.CString(allowedTypes)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cTypes))

	multi := C.int(0)
	if allowMultiple {
		multi = C.int(1)
	}

	cResult := C.JVFileOpenPanel(cTitle, cTypes, multi)
	if cResult == nil {
		return nil, nil // user cancelled
	}
	defer C.free(unsafe.Pointer(cResult))

	var paths []string
	if err := json.Unmarshal([]byte(C.GoString(cResult)), &paths); err != nil {
		return nil, fmt.Errorf("fileOpen: parse result: %w", err)
	}
	return paths, nil
}

func (n *NativeProvider) FileSave(title string, defaultName string, allowedTypes string) (string, error) {
	cTitle := C.CString(title)
	cName := C.CString(defaultName)
	cTypes := C.CString(allowedTypes)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cName))
	defer C.free(unsafe.Pointer(cTypes))

	cResult := C.JVFileSavePanel(cTitle, cName, cTypes)
	if cResult == nil {
		return "", nil // user cancelled
	}
	defer C.free(unsafe.Pointer(cResult))
	return C.GoString(cResult), nil
}

func (n *NativeProvider) Alert(title, message, style string, buttons []string) (int, error) {
	cTitle := C.CString(title)
	cMessage := C.CString(message)
	cStyle := C.CString(style)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cMessage))
	defer C.free(unsafe.Pointer(cStyle))

	var cButtons **C.char
	buttonCount := len(buttons)
	if buttonCount > 0 {
		ptrs := make([]*C.char, buttonCount)
		for i, b := range buttons {
			ptrs[i] = C.CString(b)
		}
		defer func() {
			for _, p := range ptrs {
				C.free(unsafe.Pointer(p))
			}
		}()
		cButtons = &ptrs[0]
	}

	idx := C.JVAlert(cTitle, cMessage, cStyle, cButtons, C.int(buttonCount))
	return int(idx), nil
}
