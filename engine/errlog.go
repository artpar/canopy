package engine

import (
	"fmt"
	"canopy/jlog"
	"runtime"
)

// logWarn emits a structured warning log with component and surface context.
func logWarn(component, surfaceID, msg string) {
	jlog.Warn(component, surfaceID, msg)
}

// logError emits a structured error log with component and surface context.
func logError(component, surfaceID, msg string) {
	jlog.Error(component, surfaceID, msg)
}

// logRecover is a defer target that catches panics and logs them with a stack trace.
// Usage: defer logRecover("component", "surfaceID", "context")
func logRecover(component, surfaceID, context string) {
	if r := recover(); r != nil {
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		jlog.Errorf(component, surfaceID, "panic in %s: %v\n%s", context, r, string(buf[:n]))
		fmt.Printf("[PANIC RECOVERED] %s: %v\n", context, r)
	}
}
