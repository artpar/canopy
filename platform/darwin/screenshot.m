#import <Cocoa/Cocoa.h>
#include "screenshot.h"

// Declared in app.m
extern NSMutableDictionary<NSString*, NSWindow*> *windowMap;

JVScreenshotResult JVCaptureWindow(const char* surfaceID) {
    JVScreenshotResult result = {NULL, 0};

    NSString *sid = [NSString stringWithUTF8String:surfaceID];
    NSWindow *window = windowMap[sid];
    if (!window) return result;

    NSView *contentView = window.contentView;
    if (!contentView) return result;

    // Force layout before capture
    [contentView layoutSubtreeIfNeeded];

    NSRect bounds = contentView.bounds;
    if (bounds.size.width <= 0 || bounds.size.height <= 0) return result;

    NSBitmapImageRep *rep = [contentView bitmapImageRepForCachingDisplayInRect:bounds];
    if (!rep) return result;

    [contentView cacheDisplayInRect:bounds toBitmapImageRep:rep];

    NSData *pngData = [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
    if (!pngData || pngData.length == 0) return result;

    // Copy to malloc'd buffer so Go can free it
    void *buf = malloc(pngData.length);
    if (!buf) return result;
    memcpy(buf, pngData.bytes, pngData.length);

    result.data = buf;
    result.length = pngData.length;
    return result;
}
