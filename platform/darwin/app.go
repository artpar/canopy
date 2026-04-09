package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "app.h"
*/
import "C"
import (
	"encoding/json"
	"canopy/jlog"
	"canopy/renderer"
	"unsafe"
)

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

// SetAppMode switches the application activation mode.
// mode: "normal" (dock+windows), "menubar" (status bar item), "accessory" (background).
// icon: SF Symbol name for status bar (menubar mode only).
// title: text for status bar (menubar mode, fallback when no icon).
// callbackID: invoked when status item clicked (0 = default toggle windows).
func SetAppMode(mode, icon, title string, callbackID uint64) {
	cMode := C.CString(mode)
	cIcon := C.CString(icon)
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cMode))
	defer C.free(unsafe.Pointer(cIcon))
	defer C.free(unsafe.Pointer(cTitle))
	C.JVSetAppMode(cMode, cIcon, cTitle, C.uint64_t(callbackID))
}

// OnFollowUpTriggered is called when the user presses Cmd+L.
// Set this from main.go to wire the follow-up prompt flow.
var OnFollowUpTriggered func()

//export GoFollowUpTriggered
func GoFollowUpTriggered() {
	if OnFollowUpTriggered != nil {
		OnFollowUpTriggered()
	} else {
		jlog.Infof("app", "", "Cmd+L pressed but no follow-up handler registered")
	}
}

// SetFollowUpEnabled enables or disables the "Refine UI..." (Cmd+L) menu item.
func SetFollowUpEnabled(enabled bool) {
	e := C.int(0)
	if enabled {
		e = C.int(1)
	}
	C.JVSetFollowUpEnabled(e)
}

// SetStatusMenuDynamic sets the dynamic items section in the status bar menu.
func SetStatusMenuDynamic(items []renderer.MenuItemSpec) {
	jsonItems := specToJSON(items)
	data, err := json.Marshal(jsonItems)
	if err != nil {
		return
	}
	cJSON := C.CString(string(data))
	defer C.free(unsafe.Pointer(cJSON))
	C.JVSetStatusMenuDynamic(cJSON)
}

// StatusMenuApp represents a sample app entry for the status bar Apps submenu.
type StatusMenuApp struct {
	Label string `json:"label"`
	Path  string `json:"path"`
	Icon  string `json:"icon,omitempty"`
}

// SetStatusMenuApps sets the Apps submenu in the status bar menu.
func SetStatusMenuApps(apps []StatusMenuApp) {
	data, err := json.Marshal(apps)
	if err != nil {
		return
	}
	cJSON := C.CString(string(data))
	defer C.free(unsafe.Pointer(cJSON))
	C.JVSetStatusMenuApps(cJSON)
}

// OnStatusMenuAppClicked is called when user clicks an app in the Apps submenu.
var OnStatusMenuAppClicked func(appPath string)

//export GoStatusMenuAppClicked
func GoStatusMenuAppClicked(appPath *C.char) {
	path := C.GoString(appPath)
	if OnStatusMenuAppClicked != nil {
		OnStatusMenuAppClicked(path)
	} else {
		jlog.Infof("app", "", "app menu clicked but no handler: %s", path)
	}
}

// OnStatusMenuSettingsClicked is called when user clicks Settings.
var OnStatusMenuSettingsClicked func()

//export GoStatusMenuSettingsClicked
func GoStatusMenuSettingsClicked() {
	if OnStatusMenuSettingsClicked != nil {
		OnStatusMenuSettingsClicked()
	} else {
		jlog.Infof("app", "", "settings clicked but no handler registered")
	}
}
