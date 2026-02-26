package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "contextmenu.h"
*/
import "C"
import "unsafe"

func attachContextMenu(handle uintptr, menuJSON string) {
	cJSON := C.CString(menuJSON)
	defer C.free(unsafe.Pointer(cJSON))
	C.JVAttachContextMenu(unsafe.Pointer(handle), cJSON)
}
