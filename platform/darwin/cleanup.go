package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework AVFoundation -framework AVKit

#include "audio.h"
#include "video.h"
#include "modal.h"
#include "camera.h"
#include "audiorecorder.h"
*/
import "C"
import (
	"canopy/protocol"
	"canopy/renderer"
	"unsafe"
)

// cleanupView performs type-specific cleanup before removing a view.
// Audio/Video need observer removal and player pause; Modal needs delegate nil and panel close.
// Other types are handled by ARC when removeFromSuperview is called.
func cleanupView(handle renderer.ViewHandle, compType protocol.ComponentType) {
	switch compType {
	case protocol.CompAudioPlayer:
		C.JVCleanupAudio(unsafe.Pointer(handle))
	case protocol.CompVideo:
		C.JVCleanupVideo(unsafe.Pointer(handle))
	case protocol.CompModal:
		C.JVCleanupModal(unsafe.Pointer(handle))
	case protocol.CompCameraView:
		C.JVCleanupCamera(unsafe.Pointer(handle))
	case protocol.CompAudioRecorder:
		C.JVCleanupAudioRecorder(unsafe.Pointer(handle))
	}
}
