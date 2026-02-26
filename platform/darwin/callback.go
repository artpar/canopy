package darwin

/*
#include <stdint.h>
*/
import "C"
import "jview/jlog"

//export GoCallbackInvoke
func GoCallbackInvoke(callbackID C.uint64_t, data *C.char) {
	if globalRegistry.Suppressed() {
		return
	}
	id := uint64(callbackID)
	d := C.GoString(data)
	jlog.Infof("callback", "", "GoCallbackInvoke id=%d data=%q", id, d)
	globalRegistry.Invoke(id, d)
}
