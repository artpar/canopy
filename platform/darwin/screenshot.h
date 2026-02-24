#ifndef JVIEW_SCREENSHOT_H
#define JVIEW_SCREENSHOT_H

#include <stddef.h>

// JVScreenshotResult holds the PNG data and its length.
typedef struct {
    const void* data;
    size_t length;
} JVScreenshotResult;

// JVCaptureWindow captures the content view of the window for the given surfaceID as PNG.
// Returns {NULL, 0} on failure. Caller must free data with free().
JVScreenshotResult JVCaptureWindow(const char* surfaceID);

#endif
