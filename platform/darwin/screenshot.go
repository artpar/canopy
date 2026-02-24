package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include "screenshot.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// CaptureWindow captures the window content as a PNG image.
func (r *DarwinRenderer) CaptureWindow(surfaceID string) ([]byte, error) {
	cSID := C.CString(surfaceID)
	defer C.free(unsafe.Pointer(cSID))

	result := C.JVCaptureWindow(cSID)
	if result.data == nil {
		return nil, fmt.Errorf("capture failed for surface %s", surfaceID)
	}
	defer C.free(unsafe.Pointer(result.data))

	return C.GoBytes(result.data, C.int(result.length)), nil
}
