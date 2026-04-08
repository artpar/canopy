package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework AVFoundation

#include <stdlib.h>
#include <stdbool.h>
#include "audiorecorder.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createAudioRecorderView(node *renderer.RenderNode, surfaceID string) renderer.ViewHandle {
	cFormat := C.CString(node.Props.Format)
	defer C.free(unsafe.Pointer(cFormat))

	var startedCbID, stoppedCbID, levelCbID, errorCbID uint64
	if id, ok := node.Callbacks["recordingStarted"]; ok {
		startedCbID = uint64(id)
	}
	if id, ok := node.Callbacks["recordingStopped"]; ok {
		stoppedCbID = uint64(id)
	}
	if id, ok := node.Callbacks["level"]; ok {
		levelCbID = uint64(id)
	}
	if id, ok := node.Callbacks["error"]; ok {
		errorCbID = uint64(id)
	}

	ptr := C.JVCreateAudioRecorder(cFormat,
		C.double(node.Props.SampleRate), C.int(node.Props.RecordChannels),
		C.uint64_t(startedCbID), C.uint64_t(stoppedCbID),
		C.uint64_t(levelCbID), C.uint64_t(errorCbID))
	return renderer.ViewHandle(uintptr(ptr))
}

func updateAudioRecorderView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	cFormat := C.CString(node.Props.Format)
	defer C.free(unsafe.Pointer(cFormat))

	C.JVUpdateAudioRecorder(unsafe.Pointer(handle), cFormat,
		C.double(node.Props.SampleRate), C.int(node.Props.RecordChannels))
}
