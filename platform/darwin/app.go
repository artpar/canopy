package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "app.h"
*/
import "C"
import "unsafe"

// AppInit initializes NSApplication. Must be called from main thread.
func AppInit() {
	C.JVAppInit()
}

// AppRun starts the NSApplication run loop. Blocks forever. Must be on main thread.
func AppRun() {
	C.JVAppRun()
}

// AppStop stops the NSApplication run loop.
func AppStop() {
	C.JVAppStop()
}

// AppRunUntilIdle processes all pending events and returns. Used by test mode
// to let Auto Layout compute frames before running assertions.
func AppRunUntilIdle() {
	C.JVAppRunUntilIdle()
}

// ForceLayout forces a layout pass on a surface's window content view.
func ForceLayout(surfaceID string) {
	cSID := C.CString(surfaceID)
	defer C.free(unsafe.Pointer(cSID))
	C.JVForceLayout(cSID)
}

// ShowSplashWindow shows a splash/loading window with a spinner and status text.
func ShowSplashWindow(title string, width, height int) {
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	C.JVShowSplashWindow(cTitle, C.int(width), C.int(height))
}

// UpdateSplashStatus updates the status label on the splash window.
func UpdateSplashStatus(status string) {
	cStatus := C.CString(status)
	defer C.free(unsafe.Pointer(cStatus))
	C.JVUpdateSplashStatus(cStatus)
}

// DismissSplash closes the splash window. Safe to call if already dismissed.
func DismissSplash() {
	C.JVDismissSplash()
}
