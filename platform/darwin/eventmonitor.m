#import <Cocoa/Cocoa.h>
#include "eventmonitor.h"
#import <objc/runtime.h>

extern void GoCallbackInvoke(uint64_t callbackID, const char* data);

static const void *kEventMonitorSetKey = &kEventMonitorSetKey;

// JVEventMonitorSet tracks all event monitors installed on a single NSView.
// It acts as NSTrackingArea owner, gesture recognizer target, and KVO observer.
@interface JVEventMonitorSet : NSObject
@property (nonatomic, weak) NSView *view;

// Mouse tracking
@property (nonatomic, strong) NSTrackingArea *trackingArea;
@property (nonatomic, assign) uint64_t mouseEnterCbID;
@property (nonatomic, assign) uint64_t mouseLeaveCbID;

// Double-click gesture
@property (nonatomic, strong) NSClickGestureRecognizer *doubleClickGesture;
@property (nonatomic, assign) uint64_t doubleClickCbID;

// Right-click gesture
@property (nonatomic, strong) NSClickGestureRecognizer *rightClickGesture;
@property (nonatomic, assign) uint64_t rightClickCbID;

// Focus/blur via window firstResponder KVO
@property (nonatomic, assign) uint64_t focusCbID;
@property (nonatomic, assign) uint64_t blurCbID;
@property (nonatomic, assign) BOOL isFocused;
@property (nonatomic, assign) BOOL observingFirstResponder;

// Keyboard events via NSEvent local monitors
@property (nonatomic, assign) uint64_t keyDownCbID;
@property (nonatomic, assign) uint64_t keyUpCbID;
@property (nonatomic, strong) id keyDownMonitor;
@property (nonatomic, strong) id keyUpMonitor;

// Magnify gesture
@property (nonatomic, strong) NSMagnificationGestureRecognizer *magnifyGesture;
@property (nonatomic, assign) uint64_t magnifyCbID;

// Rotate gesture
@property (nonatomic, strong) NSRotationGestureRecognizer *rotateGesture;
@property (nonatomic, assign) uint64_t rotateCbID;

// Scroll wheel via NSEvent local monitor
@property (nonatomic, assign) uint64_t scrollWheelCbID;
@property (nonatomic, strong) id scrollWheelMonitor;
@end

@implementation JVEventMonitorSet

- (void)dealloc {
    [self stopFocusObservation];
    [self removeMouseTracking];
    [self removeDoubleClickGesture];
    [self removeRightClickGesture];
    [self removeKeyboardMonitors];
    [self removeMagnifyGesture];
    [self removeRotateGesture];
    [self removeScrollWheelMonitor];
}

#pragma mark - Mouse Enter/Leave (NSTrackingArea)

- (void)installMouseTracking {
    NSView *view = self.view;
    if (!view) return;

    // Remove existing tracking area
    [self removeMouseTracking];

    NSTrackingAreaOptions opts = NSTrackingMouseEnteredAndExited |
                                 NSTrackingActiveInActiveApp |
                                 NSTrackingInVisibleRect;
    self.trackingArea = [[NSTrackingArea alloc]
        initWithRect:NSZeroRect
             options:opts
               owner:self
            userInfo:nil];
    [view addTrackingArea:self.trackingArea];
}

- (void)removeMouseTracking {
    if (self.trackingArea && self.view) {
        [self.view removeTrackingArea:self.trackingArea];
    }
    self.trackingArea = nil;
    self.mouseEnterCbID = 0;
    self.mouseLeaveCbID = 0;
}

- (void)mouseEntered:(NSEvent *)event {
    if (!self.mouseEnterCbID) return;
    NSPoint loc = event.locationInWindow;
    if (self.view) {
        loc = [self.view convertPoint:loc fromView:nil];
    }
    NSString *json = [NSString stringWithFormat:@"{\"x\":%.1f,\"y\":%.1f}", loc.x, loc.y];
    GoCallbackInvoke(self.mouseEnterCbID, [json UTF8String]);
}

- (void)mouseExited:(NSEvent *)event {
    if (!self.mouseLeaveCbID) return;
    NSPoint loc = event.locationInWindow;
    if (self.view) {
        loc = [self.view convertPoint:loc fromView:nil];
    }
    NSString *json = [NSString stringWithFormat:@"{\"x\":%.1f,\"y\":%.1f}", loc.x, loc.y];
    GoCallbackInvoke(self.mouseLeaveCbID, [json UTF8String]);
}

#pragma mark - Double-Click Gesture

- (void)installDoubleClickGesture {
    NSView *view = self.view;
    if (!view) return;

    [self removeDoubleClickGesture];

    NSClickGestureRecognizer *gesture = [[NSClickGestureRecognizer alloc]
        initWithTarget:self action:@selector(handleDoubleClick:)];
    gesture.numberOfClicksRequired = 2;
    [view addGestureRecognizer:gesture];
    self.doubleClickGesture = gesture;
}

- (void)removeDoubleClickGesture {
    if (self.doubleClickGesture && self.view) {
        [self.view removeGestureRecognizer:self.doubleClickGesture];
    }
    self.doubleClickGesture = nil;
    self.doubleClickCbID = 0;
}

- (void)handleDoubleClick:(NSClickGestureRecognizer *)recognizer {
    if (!self.doubleClickCbID) return;
    NSPoint loc = [recognizer locationInView:self.view];
    NSString *json = [NSString stringWithFormat:
        @"{\"x\":%.1f,\"y\":%.1f,\"clickCount\":2}", loc.x, loc.y];
    GoCallbackInvoke(self.doubleClickCbID, [json UTF8String]);
}

#pragma mark - Right-Click Gesture

- (void)installRightClickGesture {
    NSView *view = self.view;
    if (!view) return;

    [self removeRightClickGesture];

    NSClickGestureRecognizer *gesture = [[NSClickGestureRecognizer alloc]
        initWithTarget:self action:@selector(handleRightClick:)];
    gesture.buttonMask = 0x2; // secondary button
    gesture.numberOfClicksRequired = 1;
    [view addGestureRecognizer:gesture];
    self.rightClickGesture = gesture;
}

- (void)removeRightClickGesture {
    if (self.rightClickGesture && self.view) {
        [self.view removeGestureRecognizer:self.rightClickGesture];
    }
    self.rightClickGesture = nil;
    self.rightClickCbID = 0;
}

- (void)handleRightClick:(NSClickGestureRecognizer *)recognizer {
    if (!self.rightClickCbID) return;
    NSPoint loc = [recognizer locationInView:self.view];
    NSString *json = [NSString stringWithFormat:
        @"{\"x\":%.1f,\"y\":%.1f,\"button\":1}", loc.x, loc.y];
    GoCallbackInvoke(self.rightClickCbID, [json UTF8String]);
}

#pragma mark - Focus/Blur (firstResponder KVO)

- (void)startFocusObservation {
    NSWindow *window = self.view.window;
    if (!window || self.observingFirstResponder) return;

    [window addObserver:self
             forKeyPath:@"firstResponder"
                options:NSKeyValueObservingOptionNew | NSKeyValueObservingOptionOld
                context:NULL];
    self.observingFirstResponder = YES;

    // Set initial state
    self.isFocused = [self isViewFocused];
}

- (void)stopFocusObservation {
    if (!self.observingFirstResponder) return;
    NSWindow *window = self.view.window;
    if (window) {
        @try {
            [window removeObserver:self forKeyPath:@"firstResponder"];
        } @catch (NSException *e) {
            // Observer already removed — ignore
        }
    }
    self.observingFirstResponder = NO;
}

- (BOOL)isViewFocused {
    NSView *view = self.view;
    if (!view || !view.window) return NO;
    NSResponder *fr = view.window.firstResponder;
    if (fr == view) return YES;
    // Field editor or child view may be the first responder
    if ([fr isKindOfClass:[NSView class]]) {
        return [(NSView *)fr isDescendantOf:view];
    }
    return NO;
}

- (void)observeValueForKeyPath:(NSString *)keyPath
                      ofObject:(id)object
                        change:(NSDictionary *)change
                       context:(void *)context {
    if (![keyPath isEqualToString:@"firstResponder"]) return;

    BOOL nowFocused = [self isViewFocused];
    if (nowFocused && !self.isFocused) {
        self.isFocused = YES;
        if (self.focusCbID) {
            GoCallbackInvoke(self.focusCbID, "{}");
        }
    } else if (!nowFocused && self.isFocused) {
        self.isFocused = NO;
        if (self.blurCbID) {
            GoCallbackInvoke(self.blurCbID, "{}");
        }
    }
}

- (void)installFocusMonitor {
    // Try to start immediately; if the view has no window yet,
    // defer to after the current run loop iteration.
    if (self.view.window) {
        [self startFocusObservation];
    } else {
        __weak JVEventMonitorSet *weakSelf = self;
        dispatch_async(dispatch_get_main_queue(), ^{
            [weakSelf startFocusObservation];
        });
    }
}

- (void)removeFocusMonitor {
    [self stopFocusObservation];
    self.focusCbID = 0;
    self.blurCbID = 0;
    self.isFocused = NO;
}

#pragma mark - Keyboard Events (NSEvent local monitor)

// Build a JSON string for key event data.
// Format: {"key":"Enter","modifiers":["cmd","shift"],"characters":"\\r","keyCode":36,"repeat":false}
static NSString* keyEventJSON(NSEvent *event) {
    // Map special keys to readable names
    NSString *key = event.charactersIgnoringModifiers ?: @"";
    unichar ch = key.length > 0 ? [key characterAtIndex:0] : 0;

    // Named keys for special characters
    switch (ch) {
        case NSCarriageReturnCharacter: key = @"Enter"; break;
        case NSTabCharacter: key = @"Tab"; break;
        case 0x1B: key = @"Escape"; break;
        case NSBackspaceCharacter:
        case NSDeleteCharacter: key = @"Backspace"; break;
        case NSDeleteFunctionKey: key = @"Delete"; break;
        case NSUpArrowFunctionKey: key = @"ArrowUp"; break;
        case NSDownArrowFunctionKey: key = @"ArrowDown"; break;
        case NSLeftArrowFunctionKey: key = @"ArrowLeft"; break;
        case NSRightArrowFunctionKey: key = @"ArrowRight"; break;
        case NSHomeFunctionKey: key = @"Home"; break;
        case NSEndFunctionKey: key = @"End"; break;
        case NSPageUpFunctionKey: key = @"PageUp"; break;
        case NSPageDownFunctionKey: key = @"PageDown"; break;
        case ' ': key = @"Space"; break;
        default:
            // For F-keys
            if (ch >= NSF1FunctionKey && ch <= NSF35FunctionKey) {
                key = [NSString stringWithFormat:@"F%d", (int)(ch - NSF1FunctionKey + 1)];
            }
            break;
    }

    // Build modifiers array
    NSEventModifierFlags flags = event.modifierFlags;
    NSMutableArray *mods = [NSMutableArray array];
    if (flags & NSEventModifierFlagCommand) [mods addObject:@"\"cmd\""];
    if (flags & NSEventModifierFlagShift)   [mods addObject:@"\"shift\""];
    if (flags & NSEventModifierFlagOption)  [mods addObject:@"\"option\""];
    if (flags & NSEventModifierFlagControl) [mods addObject:@"\"ctrl\""];

    NSString *modsStr = [mods componentsJoinedByString:@","];

    // Escape the characters for JSON
    NSString *chars = event.characters ?: @"";
    chars = [chars stringByReplacingOccurrencesOfString:@"\\" withString:@"\\\\"];
    chars = [chars stringByReplacingOccurrencesOfString:@"\"" withString:@"\\\""];
    chars = [chars stringByReplacingOccurrencesOfString:@"\n" withString:@"\\n"];
    chars = [chars stringByReplacingOccurrencesOfString:@"\r" withString:@"\\r"];
    chars = [chars stringByReplacingOccurrencesOfString:@"\t" withString:@"\\t"];

    return [NSString stringWithFormat:
        @"{\"key\":\"%@\",\"modifiers\":[%@],\"characters\":\"%@\",\"keyCode\":%hu,\"repeat\":%@}",
        key, modsStr, chars, event.keyCode, event.isARepeat ? @"true" : @"false"];
}

- (BOOL)isViewOrDescendantFirstResponder {
    NSView *view = self.view;
    if (!view || !view.window) return NO;
    NSResponder *fr = view.window.firstResponder;
    if (fr == view) return YES;
    if ([fr isKindOfClass:[NSView class]]) {
        return [(NSView *)fr isDescendantOf:view];
    }
    return NO;
}

- (void)installKeyDownMonitor {
    if (self.keyDownMonitor) return;

    __weak JVEventMonitorSet *weakSelf = self;
    self.keyDownMonitor = [NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskKeyDown
        handler:^NSEvent *(NSEvent *event) {
            JVEventMonitorSet *strongSelf = weakSelf;
            if (!strongSelf || !strongSelf.keyDownCbID) return event;
            if (![strongSelf isViewOrDescendantFirstResponder]) return event;

            NSString *json = keyEventJSON(event);
            GoCallbackInvoke(strongSelf.keyDownCbID, [json UTF8String]);
            return event;
        }];
}

- (void)installKeyUpMonitor {
    if (self.keyUpMonitor) return;

    __weak JVEventMonitorSet *weakSelf = self;
    self.keyUpMonitor = [NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskKeyUp
        handler:^NSEvent *(NSEvent *event) {
            JVEventMonitorSet *strongSelf = weakSelf;
            if (!strongSelf || !strongSelf.keyUpCbID) return event;
            if (![strongSelf isViewOrDescendantFirstResponder]) return event;

            NSString *json = keyEventJSON(event);
            GoCallbackInvoke(strongSelf.keyUpCbID, [json UTF8String]);
            return event;
        }];
}

- (void)removeKeyboardMonitors {
    if (self.keyDownMonitor) {
        [NSEvent removeMonitor:self.keyDownMonitor];
        self.keyDownMonitor = nil;
    }
    if (self.keyUpMonitor) {
        [NSEvent removeMonitor:self.keyUpMonitor];
        self.keyUpMonitor = nil;
    }
    self.keyDownCbID = 0;
    self.keyUpCbID = 0;
}

#pragma mark - Magnify Gesture

static NSString* gesturePhaseString(NSGestureRecognizerState state) {
    switch (state) {
        case NSGestureRecognizerStateBegan: return @"began";
        case NSGestureRecognizerStateChanged: return @"changed";
        case NSGestureRecognizerStateEnded: return @"ended";
        case NSGestureRecognizerStateCancelled: return @"cancelled";
        default: return @"unknown";
    }
}

- (void)installMagnifyGesture {
    NSView *view = self.view;
    if (!view) return;

    [self removeMagnifyGesture];

    NSMagnificationGestureRecognizer *gesture = [[NSMagnificationGestureRecognizer alloc]
        initWithTarget:self action:@selector(handleMagnify:)];
    [view addGestureRecognizer:gesture];
    self.magnifyGesture = gesture;
}

- (void)removeMagnifyGesture {
    if (self.magnifyGesture && self.view) {
        [self.view removeGestureRecognizer:self.magnifyGesture];
    }
    self.magnifyGesture = nil;
    self.magnifyCbID = 0;
}

- (void)handleMagnify:(NSMagnificationGestureRecognizer *)recognizer {
    if (!self.magnifyCbID) return;
    NSString *phase = gesturePhaseString(recognizer.state);
    NSString *json = [NSString stringWithFormat:
        @"{\"magnification\":%.4f,\"phase\":\"%@\"}", recognizer.magnification, phase];
    GoCallbackInvoke(self.magnifyCbID, [json UTF8String]);
}

#pragma mark - Rotate Gesture

- (void)installRotateGesture {
    NSView *view = self.view;
    if (!view) return;

    [self removeRotateGesture];

    NSRotationGestureRecognizer *gesture = [[NSRotationGestureRecognizer alloc]
        initWithTarget:self action:@selector(handleRotate:)];
    [view addGestureRecognizer:gesture];
    self.rotateGesture = gesture;
}

- (void)removeRotateGesture {
    if (self.rotateGesture && self.view) {
        [self.view removeGestureRecognizer:self.rotateGesture];
    }
    self.rotateGesture = nil;
    self.rotateCbID = 0;
}

- (void)handleRotate:(NSRotationGestureRecognizer *)recognizer {
    if (!self.rotateCbID) return;
    NSString *phase = gesturePhaseString(recognizer.state);
    NSString *json = [NSString stringWithFormat:
        @"{\"rotation\":%.4f,\"phase\":\"%@\"}", recognizer.rotation, phase];
    GoCallbackInvoke(self.rotateCbID, [json UTF8String]);
}

#pragma mark - Scroll Wheel (NSEvent local monitor)

- (void)installScrollWheelMonitor {
    if (self.scrollWheelMonitor) return;

    __weak JVEventMonitorSet *weakSelf = self;
    self.scrollWheelMonitor = [NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskScrollWheel
        handler:^NSEvent *(NSEvent *event) {
            JVEventMonitorSet *strongSelf = weakSelf;
            if (!strongSelf || !strongSelf.scrollWheelCbID) return event;

            // Check if the scroll event targets this view
            NSView *view = strongSelf.view;
            if (!view || !view.window) return event;
            NSPoint loc = [view convertPoint:event.locationInWindow fromView:nil];
            if (![view mouse:loc inRect:view.bounds]) return event;

            NSString *phase = @"none";
            if (event.phase == NSEventPhaseBegan) phase = @"began";
            else if (event.phase == NSEventPhaseChanged) phase = @"changed";
            else if (event.phase == NSEventPhaseEnded) phase = @"ended";
            else if (event.phase == NSEventPhaseCancelled) phase = @"cancelled";

            NSString *json = [NSString stringWithFormat:
                @"{\"deltaX\":%.2f,\"deltaY\":%.2f,\"phase\":\"%@\"}",
                event.scrollingDeltaX, event.scrollingDeltaY, phase];
            GoCallbackInvoke(strongSelf.scrollWheelCbID, [json UTF8String]);
            return event;
        }];
}

- (void)removeScrollWheelMonitor {
    if (self.scrollWheelMonitor) {
        [NSEvent removeMonitor:self.scrollWheelMonitor];
        self.scrollWheelMonitor = nil;
    }
    self.scrollWheelCbID = 0;
}

#pragma mark - Cleanup

- (void)removeAll {
    [self removeMouseTracking];
    [self removeDoubleClickGesture];
    [self removeRightClickGesture];
    [self removeFocusMonitor];
    [self removeKeyboardMonitors];
    [self removeMagnifyGesture];
    [self removeRotateGesture];
    [self removeScrollWheelMonitor];
}

@end

// Get or create the event monitor set for a view.
static JVEventMonitorSet* getOrCreateMonitorSet(void* handle) {
    if (!handle) return nil;
    NSView *view = (__bridge NSView*)handle;

    JVEventMonitorSet *set = objc_getAssociatedObject(view, kEventMonitorSetKey);
    if (!set) {
        set = [[JVEventMonitorSet alloc] init];
        set.view = view;
        objc_setAssociatedObject(view, kEventMonitorSetKey, set, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    }
    return set;
}

static JVEventMonitorSet* getMonitorSet(void* handle) {
    if (!handle) return nil;
    NSView *view = (__bridge NSView*)handle;
    return objc_getAssociatedObject(view, kEventMonitorSetKey);
}

void JVInstallEventMonitor(void* handle, const char* eventName, uint64_t callbackID) {
    if (!handle || !eventName) return;
    NSString *name = [NSString stringWithUTF8String:eventName];
    JVEventMonitorSet *set = getOrCreateMonitorSet(handle);

    if ([name isEqualToString:@"mouseEnter"]) {
        set.mouseEnterCbID = callbackID;
        if (!set.trackingArea) [set installMouseTracking];
    } else if ([name isEqualToString:@"mouseLeave"]) {
        set.mouseLeaveCbID = callbackID;
        if (!set.trackingArea) [set installMouseTracking];
    } else if ([name isEqualToString:@"doubleClick"]) {
        set.doubleClickCbID = callbackID;
        [set installDoubleClickGesture];
    } else if ([name isEqualToString:@"rightClick"]) {
        set.rightClickCbID = callbackID;
        [set installRightClickGesture];
    } else if ([name isEqualToString:@"focus"]) {
        set.focusCbID = callbackID;
        if (!set.observingFirstResponder) [set installFocusMonitor];
    } else if ([name isEqualToString:@"blur"]) {
        set.blurCbID = callbackID;
        if (!set.observingFirstResponder) [set installFocusMonitor];
    } else if ([name isEqualToString:@"keyDown"]) {
        set.keyDownCbID = callbackID;
        [set installKeyDownMonitor];
    } else if ([name isEqualToString:@"keyUp"]) {
        set.keyUpCbID = callbackID;
        [set installKeyUpMonitor];
    } else if ([name isEqualToString:@"magnify"]) {
        set.magnifyCbID = callbackID;
        [set installMagnifyGesture];
    } else if ([name isEqualToString:@"rotate"]) {
        set.rotateCbID = callbackID;
        [set installRotateGesture];
    } else if ([name isEqualToString:@"scrollWheel"]) {
        set.scrollWheelCbID = callbackID;
        [set installScrollWheelMonitor];
    }
}

void JVUpdateEventMonitorCallbackID(void* handle, const char* eventName, uint64_t callbackID) {
    if (!handle || !eventName) return;
    NSString *name = [NSString stringWithUTF8String:eventName];
    JVEventMonitorSet *set = getMonitorSet(handle);
    if (!set) {
        // Not yet installed — install fresh
        JVInstallEventMonitor(handle, eventName, callbackID);
        return;
    }

    if ([name isEqualToString:@"mouseEnter"]) {
        set.mouseEnterCbID = callbackID;
    } else if ([name isEqualToString:@"mouseLeave"]) {
        set.mouseLeaveCbID = callbackID;
    } else if ([name isEqualToString:@"doubleClick"]) {
        set.doubleClickCbID = callbackID;
    } else if ([name isEqualToString:@"rightClick"]) {
        set.rightClickCbID = callbackID;
    } else if ([name isEqualToString:@"focus"]) {
        set.focusCbID = callbackID;
    } else if ([name isEqualToString:@"blur"]) {
        set.blurCbID = callbackID;
    } else if ([name isEqualToString:@"keyDown"]) {
        set.keyDownCbID = callbackID;
    } else if ([name isEqualToString:@"keyUp"]) {
        set.keyUpCbID = callbackID;
    } else if ([name isEqualToString:@"magnify"]) {
        set.magnifyCbID = callbackID;
    } else if ([name isEqualToString:@"rotate"]) {
        set.rotateCbID = callbackID;
    } else if ([name isEqualToString:@"scrollWheel"]) {
        set.scrollWheelCbID = callbackID;
    }
}

void JVRemoveAllEventMonitors(void* handle) {
    if (!handle) return;
    JVEventMonitorSet *set = getMonitorSet(handle);
    if (set) {
        [set removeAll];
    }
    NSView *view = (__bridge NSView*)handle;
    objc_setAssociatedObject(view, kEventMonitorSetKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
}
