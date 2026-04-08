package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework UserNotifications -framework UniformTypeIdentifiers -framework AVFoundation -framework ScreenCaptureKit -framework CoreGraphics -framework ImageIO

#include "native.h"
#include <stdlib.h>
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"unsafe"
)

// dialogResult is sent from the ObjC completion handler back to the waiting goroutine.
type dialogResult struct {
	value *string // nil = cancelled/no result
}

// pendingDialogs maps requestID → result channel.
var (
	pendingDialogs   = make(map[uint64]chan dialogResult)
	pendingDialogsMu sync.Mutex
	nextRequestID    atomic.Uint64
)

func init() {
	nextRequestID.Store(1)
}

// allocRequest creates a pending request and returns its ID and result channel.
func allocRequest() (uint64, chan dialogResult) {
	id := nextRequestID.Add(1) - 1
	ch := make(chan dialogResult, 1)
	pendingDialogsMu.Lock()
	pendingDialogs[id] = ch
	pendingDialogsMu.Unlock()
	return id, ch
}

// GoNativeDialogResult is called from ObjC completion handlers.
//
//export GoNativeDialogResult
func GoNativeDialogResult(requestID C.uint64_t, result *C.char) {
	id := uint64(requestID)
	pendingDialogsMu.Lock()
	ch, ok := pendingDialogs[id]
	if ok {
		delete(pendingDialogs, id)
	}
	pendingDialogsMu.Unlock()

	if !ok {
		return
	}

	if result == nil {
		ch <- dialogResult{value: nil}
	} else {
		s := C.GoString(result)
		ch <- dialogResult{value: &s}
	}
}

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
	rc := C.JVSendNotification(cTitle, cBody, cSubtitle)
	if rc != 0 {
		return fmt.Errorf("notifications unavailable (no app bundle)")
	}
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

// FileOpen shows a file open dialog. The dialog runs non-blocking on the main
// thread; the calling goroutine blocks on a channel until the user responds.
// The main thread's run loop remains free for rendering and MCP calls.
func (n *NativeProvider) FileOpen(title string, allowedTypes string, allowMultiple bool) ([]string, error) {
	cTitle := C.CString(title)
	cTypes := C.CString(allowedTypes)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cTypes))

	multi := C.int(0)
	if allowMultiple {
		multi = C.int(1)
	}

	reqID, ch := allocRequest()
	C.JVFileOpenPanelAsync(cTitle, cTypes, multi, C.uint64_t(reqID))

	res := <-ch
	if res.value == nil {
		return nil, nil // cancelled
	}

	var paths []string
	if err := json.Unmarshal([]byte(*res.value), &paths); err != nil {
		return nil, fmt.Errorf("fileOpen: parse result: %w", err)
	}
	return paths, nil
}

// FileSave shows a file save dialog. Non-blocking on main thread.
func (n *NativeProvider) FileSave(title string, defaultName string, allowedTypes string) (string, error) {
	cTitle := C.CString(title)
	cName := C.CString(defaultName)
	cTypes := C.CString(allowedTypes)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cName))
	defer C.free(unsafe.Pointer(cTypes))

	reqID, ch := allocRequest()
	C.JVFileSavePanelAsync(cTitle, cName, cTypes, C.uint64_t(reqID))

	res := <-ch
	if res.value == nil {
		return "", nil // cancelled
	}
	return *res.value, nil
}

// Alert shows an alert dialog. Non-blocking on main thread (sheet on key window).
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

	reqID, ch := allocRequest()
	C.JVAlertAsync(cTitle, cMessage, cStyle, cButtons, C.int(buttonCount), C.uint64_t(reqID))

	res := <-ch
	if res.value == nil {
		return 0, nil
	}
	idx := 0
	fmt.Sscanf(*res.value, "%d", &idx)
	return idx, nil
}

// CameraCapture takes a one-shot photo without a preview component.
func (n *NativeProvider) CameraCapture(devicePosition string) (string, error) {
	cPos := C.CString(devicePosition)
	defer C.free(unsafe.Pointer(cPos))

	reqID, ch := allocRequest()
	C.JVCameraCaptureAsync(cPos, C.uint64_t(reqID))

	res := <-ch
	if res.value == nil {
		return "", fmt.Errorf("camera capture failed or access denied")
	}
	return *res.value, nil
}

// activeRecorders tracks headless audio recordings by ID.
var (
	activeRecorders   = make(map[string]activeRecording)
	activeRecordersMu sync.Mutex
	nextRecorderID    atomic.Uint64
)

type activeRecording struct {
	handle unsafe.Pointer
	path   string
}

func init() {
	nextRecorderID.Store(1)
}

// AudioRecordStart begins recording audio to a temp file.
func (n *NativeProvider) AudioRecordStart(format string, sampleRate float64, channels int) (string, error) {
	ext := "m4a"
	if format == "wav" {
		ext = "wav"
	}
	timestamp := fmt.Sprintf("%d", nextRecorderID.Add(1)-1)
	path := fmt.Sprintf("%s/canopy_recording_%s.%s", os.TempDir(), timestamp, ext)

	cPath := C.CString(path)
	cFormat := C.CString(format)
	defer C.free(unsafe.Pointer(cPath))
	defer C.free(unsafe.Pointer(cFormat))

	handle := C.JVAudioRecordStart(cPath, cFormat, C.double(sampleRate), C.int(channels))
	if handle == nil {
		return "", fmt.Errorf("failed to start audio recording")
	}

	id := fmt.Sprintf("rec_%s", timestamp)
	activeRecordersMu.Lock()
	activeRecorders[id] = activeRecording{handle: handle, path: path}
	activeRecordersMu.Unlock()

	return id, nil
}

// AudioRecordStop stops a recording and returns the file path.
func (n *NativeProvider) AudioRecordStop(recordingID string) (string, error) {
	activeRecordersMu.Lock()
	rec, ok := activeRecorders[recordingID]
	if ok {
		delete(activeRecorders, recordingID)
	}
	activeRecordersMu.Unlock()

	if !ok {
		return "", fmt.Errorf("recording %q not found", recordingID)
	}

	C.JVAudioRecordStop(rec.handle)
	return rec.path, nil
}

// ScreenCapture takes a screenshot and returns the file path.
func (n *NativeProvider) ScreenCapture(captureType string) (string, error) {
	cType := C.CString(captureType)
	defer C.free(unsafe.Pointer(cType))

	reqID, ch := allocRequest()
	C.JVScreenCaptureAsync(cType, C.uint64_t(reqID))

	res := <-ch
	if res.value == nil {
		return "", fmt.Errorf("screen capture failed or access denied")
	}
	return *res.value, nil
}

// ScreenRecordStart begins recording the screen (not yet implemented).
func (n *NativeProvider) ScreenRecordStart(captureType string) (string, error) {
	return "", fmt.Errorf("screen recording not yet implemented")
}

// ScreenRecordStop stops a screen recording (not yet implemented).
func (n *NativeProvider) ScreenRecordStop(recordingID string) (string, error) {
	return "", fmt.Errorf("screen recording not yet implemented")
}

// CleanupAll stops all active headless recordings and releases resources.
func (n *NativeProvider) CleanupAll() {
	activeRecordersMu.Lock()
	for id, rec := range activeRecorders {
		C.JVAudioRecordStop(rec.handle)
		delete(activeRecorders, id)
	}
	activeRecordersMu.Unlock()
}
