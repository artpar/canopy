#ifndef JVIEW_EVENTMONITOR_H
#define JVIEW_EVENTMONITOR_H

#include <stdint.h>

// Install an event monitor on a view. Supported event names:
// "mouseEnter", "mouseLeave", "doubleClick", "rightClick", "focus", "blur",
// "keyDown", "keyUp", "magnify", "rotate", "scrollWheel"
void JVInstallEventMonitor(void* handle, const char* eventName, uint64_t callbackID);

// Update the callback ID for an already-installed event monitor.
void JVUpdateEventMonitorCallbackID(void* handle, const char* eventName, uint64_t callbackID);

// Remove all event monitors from a view (called during cleanup).
void JVRemoveAllEventMonitors(void* handle);

#endif
