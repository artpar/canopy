package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework CoreText

#include <stdlib.h>
#include "assets.h"
*/
import "C"
import (
	"jview/renderer"
	"unsafe"
)

func loadAssets(assets []renderer.AssetSpec) {
	for _, a := range assets {
		cSrc := C.CString(a.Src)
		switch a.Kind {
		case "font":
			C.JVRegisterFont(cSrc)
		case "image":
			cAlias := C.CString(a.Alias)
			C.JVPreloadImage(cAlias, cSrc)
			C.free(unsafe.Pointer(cAlias))
		}
		C.free(unsafe.Pointer(cSrc))
	}
}
