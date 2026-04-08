package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework CoreBluetooth -framework CoreLocation -framework IOKit

#include <stdlib.h>
#include "hardwareevents.h"
*/
import "C"
import "unsafe"

// hardwareEventHandler receives distributed notification events keyed by subscription ID.
var hardwareEventHandler func(uint64, string)

// SetHardwareEventHandler sets the callback for hardware-specific events.
func SetHardwareEventHandler(fn func(subscriptionID uint64, data string)) {
	hardwareEventHandler = fn
}

//export GoHardwareEvent
func GoHardwareEvent(subscriptionID C.uint64_t, data *C.char) {
	id := uint64(subscriptionID)
	d := C.GoString(data)
	if hardwareEventHandler != nil {
		go hardwareEventHandler(id, d)
	}
}

// StartBluetoothObserver begins monitoring Bluetooth state.
func StartBluetoothObserver() { C.JVStartBluetoothObserver() }

// StopBluetoothObserver stops monitoring Bluetooth state.
func StopBluetoothObserver() { C.JVStopBluetoothObserver() }

// StartLocationObserver begins monitoring significant location changes.
func StartLocationObserver() { C.JVStartLocationObserver() }

// StopLocationObserver stops monitoring location.
func StopLocationObserver() { C.JVStopLocationObserver() }

// StartUSBObserver begins monitoring USB device connect/disconnect.
func StartUSBObserver() { C.JVStartUSBObserver() }

// StopUSBObserver stops monitoring USB devices.
func StopUSBObserver() { C.JVStopUSBObserver() }

// ObserveDistributedNotification starts observing a named distributed notification.
func ObserveDistributedNotification(name string, subscriptionID uint64) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	C.JVObserveDistributedNotification(cName, C.uint64_t(subscriptionID))
}

// UnobserveDistributedNotification stops observing a distributed notification.
func UnobserveDistributedNotification(subscriptionID uint64) {
	C.JVUnobserveDistributedNotification(C.uint64_t(subscriptionID))
}
