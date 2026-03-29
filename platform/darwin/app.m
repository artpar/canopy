#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "app.h"

// Map from surfaceID → NSWindow (non-static so screenshot.m can access via extern)
NSMutableDictionary<NSString*, NSWindow*> *windowMap = nil;

// Forward-declare for use in delegate method
static NSStatusItem *statusItem = nil;
static NSMenuItem *refineMenuItem = nil;

@interface JVAppDelegate : NSObject <NSApplicationDelegate>
@end

extern void GoFollowUpTriggered(void);

@implementation JVAppDelegate

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    // Activate the app and bring to front
    [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
    [NSApp activateIgnoringOtherApps:YES];
}

- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender {
    // In menubar mode, keep running when windows close
    if (statusItem != nil) return NO;
    return YES;
}

- (void)refineUI:(id)sender {
    GoFollowUpTriggered();
}

@end

void JVAppInit(void) {
    [NSApplication sharedApplication];
    windowMap = [[NSMutableDictionary alloc] init];

    JVAppDelegate *delegate = [[JVAppDelegate alloc] init];
    [NSApp setDelegate:delegate];

    // Create a basic menu bar
    NSMenu *menuBar = [[NSMenu alloc] init];
    NSMenuItem *appMenuItem = [[NSMenuItem alloc] init];
    [menuBar addItem:appMenuItem];
    [NSApp setMainMenu:menuBar];

    NSMenu *appMenu = [[NSMenu alloc] init];

    refineMenuItem = [[NSMenuItem alloc] initWithTitle:@"Refine UI..."
                                                action:@selector(refineUI:)
                                         keyEquivalent:@"l"];
    refineMenuItem.enabled = NO;
    [appMenu addItem:refineMenuItem];
    [appMenu addItem:[NSMenuItem separatorItem]];

    NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"Quit jview"
                                                      action:@selector(terminate:)
                                               keyEquivalent:@"q"];
    [appMenu addItem:quitItem];
    [appMenuItem setSubmenu:appMenu];
}

void JVAppRun(void) {
    [NSApp run];
}

void JVAppStop(void) {
    [NSApp stop:nil];
    // Post a dummy event to break out of the run loop
    NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
                                        location:NSZeroPoint
                                   modifierFlags:0
                                       timestamp:0
                                    windowNumber:0
                                         context:nil
                                         subtype:0
                                           data1:0
                                           data2:0];
    [NSApp postEvent:event atStart:YES];
}

void JVAppRunUntilIdle(void) {
    // Process all pending events until idle, then return.
    // This lets Auto Layout compute frames before test assertions run.
    while (true) {
        NSEvent *event = [NSApp nextEventMatchingMask:NSEventMaskAny
                                            untilDate:[NSDate distantPast]
                                               inMode:NSDefaultRunLoopMode
                                              dequeue:YES];
        if (!event) break;
        [NSApp sendEvent:event];
    }
}

void JVForceLayout(const char* surfaceID) {
    NSString *sid = [NSString stringWithUTF8String:surfaceID];
    NSWindow *window = windowMap[sid];
    if (!window) return;
    [window.contentView layoutSubtreeIfNeeded];
}

static NSColor* jvColorFromHex(NSString *hex) {
    if ([hex length] < 7 || [hex characterAtIndex:0] != '#') return nil;
    unsigned int r = 0, g = 0, b = 0;
    NSScanner *scanner;
    scanner = [NSScanner scannerWithString:[hex substringWithRange:NSMakeRange(1, 2)]];
    [scanner scanHexInt:&r];
    scanner = [NSScanner scannerWithString:[hex substringWithRange:NSMakeRange(3, 2)]];
    [scanner scanHexInt:&g];
    scanner = [NSScanner scannerWithString:[hex substringWithRange:NSMakeRange(5, 2)]];
    [scanner scanHexInt:&b];
    return [NSColor colorWithSRGBRed:r/255.0 green:g/255.0 blue:b/255.0 alpha:1.0];
}

void* JVCreateWindow(const char* title, int width, int height, const char* surfaceID, const char* backgroundColor) {
    JVDismissSplash();
    NSString *sid = [NSString stringWithUTF8String:surfaceID];
    NSString *windowTitle = [NSString stringWithUTF8String:title];

    NSRect frame = NSMakeRect(0, 0, width, height);
    NSWindowStyleMask styleMask = NSWindowStyleMaskTitled |
                                   NSWindowStyleMaskClosable |
                                   NSWindowStyleMaskMiniaturizable |
                                   NSWindowStyleMaskResizable |
                                   NSWindowStyleMaskFullSizeContentView;

    NSWindow *window = [[NSWindow alloc] initWithContentRect:frame
                                                   styleMask:styleMask
                                                     backing:NSBackingStoreBuffered
                                                       defer:NO];
    window.releasedWhenClosed = NO; // prevent double-release under ARC
    [window setTitle:windowTitle];
    window.titlebarAppearsTransparent = YES;
    window.titleVisibility = NSWindowTitleHidden;
    if (@available(macOS 11.0, *)) {
        window.toolbarStyle = NSWindowToolbarStyleUnified;
    }
    [window center];
    [window makeKeyAndOrderFront:nil];

    // Apply background color if specified
    NSString *bgStr = [NSString stringWithUTF8String:backgroundColor];
    if ([bgStr length] > 0) {
        NSColor *color = jvColorFromHex(bgStr);
        if (color) {
            window.backgroundColor = color;
        }
    }

    windowMap[sid] = window;

    // Add loading spinner + label, centered in window.
    // Automatically removed when JVSetWindowRootView clears all subviews.
    NSView *spinnerContainer = [[NSView alloc] init];
    spinnerContainer.translatesAutoresizingMaskIntoConstraints = NO;

    NSProgressIndicator *spinner = [[NSProgressIndicator alloc] init];
    spinner.style = NSProgressIndicatorStyleSpinning;
    spinner.controlSize = NSControlSizeRegular;
    spinner.translatesAutoresizingMaskIntoConstraints = NO;
    [spinner startAnimation:nil];

    NSTextField *loadingLabel = [NSTextField labelWithString:@"Loading..."];
    loadingLabel.textColor = [NSColor secondaryLabelColor];
    loadingLabel.font = [NSFont systemFontOfSize:14];
    loadingLabel.translatesAutoresizingMaskIntoConstraints = NO;

    [spinnerContainer addSubview:spinner];
    [spinnerContainer addSubview:loadingLabel];

    [NSLayoutConstraint activateConstraints:@[
        [spinner.centerXAnchor constraintEqualToAnchor:spinnerContainer.centerXAnchor],
        [spinner.topAnchor constraintEqualToAnchor:spinnerContainer.topAnchor],
        [loadingLabel.centerXAnchor constraintEqualToAnchor:spinnerContainer.centerXAnchor],
        [loadingLabel.topAnchor constraintEqualToAnchor:spinner.bottomAnchor constant:12],
        [loadingLabel.bottomAnchor constraintEqualToAnchor:spinnerContainer.bottomAnchor],
    ]];

    NSView *contentView = window.contentView;
    [contentView addSubview:spinnerContainer];

    [NSLayoutConstraint activateConstraints:@[
        [spinnerContainer.centerXAnchor constraintEqualToAnchor:contentView.centerXAnchor],
        [spinnerContainer.centerYAnchor constraintEqualToAnchor:contentView.centerYAnchor],
    ]];

    return (__bridge void*)window;
}

// Recursively nil all delegates on component views to prevent AppKit from
// firing delegate callbacks during view teardown (which causes SIGSEGV when
// the delegate references a partially-destroyed view hierarchy).
static void nilDelegatesRecursively(NSView *view) {
    for (NSView *subview in [view.subviews copy]) {
        nilDelegatesRecursively(subview);
    }
    if ([view isKindOfClass:[NSSplitView class]]) {
        ((NSSplitView *)view).delegate = nil;
    } else if ([view isKindOfClass:[NSOutlineView class]]) {
        ((NSOutlineView *)view).delegate = nil;
        ((NSOutlineView *)view).dataSource = nil;
    } else if ([view isKindOfClass:[NSSearchField class]]) {
        ((NSSearchField *)view).delegate = nil;
    } else if ([view isKindOfClass:[NSTextField class]]) {
        ((NSTextField *)view).delegate = nil;
    } else if ([view isKindOfClass:[NSTabView class]]) {
        ((NSTabView *)view).delegate = nil;
    } else if ([view isKindOfClass:[NSScrollView class]]) {
        NSView *docView = ((NSScrollView *)view).documentView;
        if ([docView isKindOfClass:[NSOutlineView class]]) {
            ((NSOutlineView *)docView).delegate = nil;
            ((NSOutlineView *)docView).dataSource = nil;
        } else if ([docView isKindOfClass:[NSTextView class]]) {
            ((NSTextView *)docView).delegate = nil;
        }
    }
}

void JVDestroyWindow(const char* surfaceID) {
    NSString *sid = [NSString stringWithUTF8String:surfaceID];
    NSWindow *window = windowMap[sid];
    if (window) {
        // Resign first responder to prevent AppKit from accessing
        // deallocated text editors / field editors during teardown
        [window makeFirstResponder:nil];

        // Nil the delegate so AppKit won't call windowWillClose: etc.
        // on a potentially deallocated object
        window.delegate = nil;

        // Nil all component-level delegates before removing subviews —
        // prevents SIGSEGV from delegate callbacks during teardown
        nilDelegatesRecursively([window contentView]);

        // Detach all subviews before closing — prevents AppKit from
        // walking the view tree and hitting freed ObjC objects
        [[window contentView] setSubviews:@[]];

        // Remove toolbar to avoid toolbar-delegate callbacks during close
        window.toolbar = nil;

        // Order out first (hides window), then close (releases)
        [window orderOut:nil];
        [window close];
        [windowMap removeObjectForKey:sid];
    }
}

static void invalidateLayersRecursively(NSView *view) {
    if (view.layer) {
        [view.layer setNeedsDisplay];
    }
    view.needsDisplay = YES;
    for (NSView *subview in view.subviews) {
        invalidateLayersRecursively(subview);
    }
}

void JVSetWindowTheme(const char* surfaceID, const char* theme) {
    NSString *sid = [NSString stringWithUTF8String:surfaceID];
    NSWindow *window = windowMap[sid];
    if (!window) return;

    NSString *themeStr = [NSString stringWithUTF8String:theme];
    NSAppearance *appearance = nil;
    if ([themeStr isEqualToString:@"dark"]) {
        appearance = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
    } else if ([themeStr isEqualToString:@"light"]) {
        appearance = [NSAppearance appearanceNamed:NSAppearanceNameAqua];
    }
    window.appearance = appearance;
    window.backgroundColor = [NSColor windowBackgroundColor];
    invalidateLayersRecursively(window.contentView);
    [window invalidateShadow];
}

void JVRemoveView(void* view) {
    if (!view) return;
    NSView *nsView = (__bridge NSView*)view;
    // Nil delegates before removal to prevent AppKit from firing callbacks
    // on partially-destroyed view hierarchies (causes SIGSEGV)
    nilDelegatesRecursively(nsView);
    [nsView removeFromSuperview];
}

void JVUpdateWindow(const char* surfaceID, const char* title, int minWidth, int minHeight) {
    NSString *sid = [NSString stringWithUTF8String:surfaceID];
    NSWindow *window = windowMap[sid];
    if (!window) return;

    NSString *titleStr = [NSString stringWithUTF8String:title];
    if ([titleStr length] > 0) {
        [window setTitle:titleStr];
    }
    if (minWidth > 0 || minHeight > 0) {
        CGFloat w = (minWidth > 0) ? (CGFloat)minWidth : window.minSize.width;
        CGFloat h = (minHeight > 0) ? (CGFloat)minHeight : window.minSize.height;
        window.minSize = NSMakeSize(w, h);
    }
}

// --- App mode (menubar/accessory/normal) ---

extern void GoCallbackInvoke(uint64_t callbackID, const char* data);

static uint64_t statusItemCallbackID = 0;

@interface JVStatusItemTarget : NSObject
@end

@implementation JVStatusItemTarget
- (void)statusItemClicked:(id)sender {
    if (statusItemCallbackID != 0) {
        GoCallbackInvoke(statusItemCallbackID, "");
    } else {
        // Default: toggle visibility of all windows
        BOOL anyVisible = NO;
        for (NSString *sid in windowMap) {
            NSWindow *w = windowMap[sid];
            if ([w isVisible]) { anyVisible = YES; break; }
        }
        for (NSString *sid in windowMap) {
            NSWindow *w = windowMap[sid];
            if (anyVisible) {
                [w orderOut:nil];
            } else {
                [w makeKeyAndOrderFront:nil];
                [NSApp activateIgnoringOtherApps:YES];
            }
        }
    }
}
@end

static JVStatusItemTarget *statusItemTarget = nil;

void JVSetAppMode(const char* mode, const char* icon, const char* title, uint64_t callbackID) {
    NSString *modeStr = [NSString stringWithUTF8String:mode];

    if ([modeStr isEqualToString:@"menubar"]) {
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

        // Create status item if not exists
        if (!statusItem) {
            statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
            statusItemTarget = [[JVStatusItemTarget alloc] init];
            statusItem.button.action = @selector(statusItemClicked:);
            statusItem.button.target = statusItemTarget;
        }
        statusItemCallbackID = callbackID;

        // Set icon (SF Symbol) or title
        if (icon && strlen(icon) > 0) {
            NSString *iconName = [NSString stringWithUTF8String:icon];
            NSImage *img = [NSImage imageWithSystemSymbolName:iconName accessibilityDescription:nil];
            if (img) {
                statusItem.button.image = img;
                statusItem.button.title = @"";
            } else {
                // Fallback to title if symbol not found
                statusItem.button.image = nil;
                statusItem.button.title = (title && strlen(title) > 0)
                    ? [NSString stringWithUTF8String:title] : @"jview";
            }
        } else {
            statusItem.button.image = nil;
            statusItem.button.title = (title && strlen(title) > 0)
                ? [NSString stringWithUTF8String:title] : @"jview";
        }

        // Don't terminate when last window closes in menubar mode
        // (handled by checking mode in delegate)

    } else if ([modeStr isEqualToString:@"accessory"]) {
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

        // Remove status item if exists
        if (statusItem) {
            [[NSStatusBar systemStatusBar] removeStatusItem:statusItem];
            statusItem = nil;
            statusItemTarget = nil;
        }

    } else {
        // "normal" — restore dock icon
        [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
        [NSApp activateIgnoringOtherApps:YES];

        // Remove status item if exists
        if (statusItem) {
            [[NSStatusBar systemStatusBar] removeStatusItem:statusItem];
            statusItem = nil;
            statusItemTarget = nil;
        }
    }
}

// --- Splash window ---

static NSWindow *splashWindow = nil;
static NSTextField *splashStatusLabel = nil;

void JVShowSplashWindow(const char* title, int width, int height) {
    NSString *windowTitle = [NSString stringWithUTF8String:title];
    NSRect frame = NSMakeRect(0, 0, width, height);
    NSWindowStyleMask styleMask = NSWindowStyleMaskTitled |
                                   NSWindowStyleMaskClosable |
                                   NSWindowStyleMaskFullSizeContentView;

    splashWindow = [[NSWindow alloc] initWithContentRect:frame
                                               styleMask:styleMask
                                                 backing:NSBackingStoreBuffered
                                                   defer:NO];
    [splashWindow setTitle:windowTitle];
    splashWindow.titlebarAppearsTransparent = YES;
    splashWindow.titleVisibility = NSWindowTitleHidden;
    [splashWindow center];
    [splashWindow makeKeyAndOrderFront:nil];

    NSView *contentView = splashWindow.contentView;

    NSProgressIndicator *spinner = [[NSProgressIndicator alloc] init];
    spinner.style = NSProgressIndicatorStyleSpinning;
    spinner.controlSize = NSControlSizeRegular;
    spinner.translatesAutoresizingMaskIntoConstraints = NO;
    [spinner startAnimation:nil];

    splashStatusLabel = [NSTextField labelWithString:@"Initializing..."];
    splashStatusLabel.textColor = [NSColor secondaryLabelColor];
    splashStatusLabel.font = [NSFont systemFontOfSize:14];
    splashStatusLabel.translatesAutoresizingMaskIntoConstraints = NO;

    NSView *container = [[NSView alloc] init];
    container.translatesAutoresizingMaskIntoConstraints = NO;
    [container addSubview:spinner];
    [container addSubview:splashStatusLabel];

    [NSLayoutConstraint activateConstraints:@[
        [spinner.centerXAnchor constraintEqualToAnchor:container.centerXAnchor],
        [spinner.topAnchor constraintEqualToAnchor:container.topAnchor],
        [splashStatusLabel.centerXAnchor constraintEqualToAnchor:container.centerXAnchor],
        [splashStatusLabel.topAnchor constraintEqualToAnchor:spinner.bottomAnchor constant:12],
        [splashStatusLabel.bottomAnchor constraintEqualToAnchor:container.bottomAnchor],
    ]];

    [contentView addSubview:container];
    [NSLayoutConstraint activateConstraints:@[
        [container.centerXAnchor constraintEqualToAnchor:contentView.centerXAnchor],
        [container.centerYAnchor constraintEqualToAnchor:contentView.centerYAnchor],
    ]];
}

void JVUpdateSplashStatus(const char* status) {
    if (!splashStatusLabel) return;
    NSString *str = [NSString stringWithUTF8String:status];
    [splashStatusLabel setStringValue:str];
}

void JVDismissSplash(void) {
    if (!splashWindow) return;
    [splashWindow orderOut:nil];
    [splashWindow close];
    splashWindow = nil;
    splashStatusLabel = nil;
}

void JVSetWindowRootView(const char* surfaceID, void* view, int padding) {
    if (!view) return;
    NSString *sid = [NSString stringWithUTF8String:surfaceID];
    NSWindow *window = windowMap[sid];
    if (!window) return;

    NSView *nsView = (__bridge NSView*)view;
    NSView *contentView = window.contentView;

    // If this view is already the window's root, skip re-attachment
    // Re-adding breaks NSSplitView and other views with no intrinsic size
    if (nsView.superview == contentView) return;

    nsView.translatesAutoresizingMaskIntoConstraints = NO;

    // Remove existing subviews (loading spinner, previous root, etc.)
    for (NSView *sub in [contentView.subviews copy]) {
        [sub removeFromSuperview];
    }

    [contentView addSubview:nsView];

    // Use specified padding; default to 20 if not set (padding==0 means no value provided from protocol)
    // To get edge-to-edge, set padding to -1 in protocol which maps to 0 here
    CGFloat inset = (padding > 0) ? (CGFloat)padding : (padding < 0) ? 0.0 : 20.0;

    // Check if root view needs tight bottom constraint:
    // - NSStackView with flexGrow children (kJVNeedsFlexExpansionKey)
    // - NSSplitView (no intrinsic content size, must always fill)
    extern const void *kJVNeedsFlexExpansionKey;
    NSNumber *needsFlex = objc_getAssociatedObject(nsView, kJVNeedsFlexExpansionKey);
    BOOL isSplitView = [nsView isKindOfClass:[NSSplitView class]];
    NSLayoutConstraint *bottom;
    if ((needsFlex && [needsFlex boolValue]) || isSplitView) {
        // Tight: root fills window so flexGrow children / split panes can expand
        bottom = [nsView.bottomAnchor constraintEqualToAnchor:contentView.bottomAnchor constant:-inset];
    } else {
        // Loose: root sizes to content, sits at top of window
        bottom = [nsView.bottomAnchor constraintLessThanOrEqualToAnchor:contentView.bottomAnchor constant:-inset];
    }

    // Use contentLayoutGuide for top anchor so content starts below toolbar (not behind it)
    NSLayoutGuide *layoutGuide = window.contentLayoutGuide;
    [NSLayoutConstraint activateConstraints:@[
        [nsView.topAnchor constraintEqualToAnchor:layoutGuide.topAnchor constant:inset],
        [nsView.leadingAnchor constraintEqualToAnchor:contentView.leadingAnchor constant:inset],
        [nsView.trailingAnchor constraintEqualToAnchor:contentView.trailingAnchor constant:-inset],
        bottom,
    ]];
}

// --- Follow-up prompt panel (Cmd+L) ---

extern void GoNativeDialogResult(uint64_t requestID, const char* result);

void JVSetFollowUpEnabled(int enabled) {
    if (refineMenuItem) {
        refineMenuItem.enabled = (enabled != 0);
    }
}

void JVShowFollowUpPanel(uint64_t requestID) {
    NSAlert *alert = [[NSAlert alloc] init];
    alert.alertStyle = NSAlertStyleInformational;
    alert.messageText = @"Refine UI";
    alert.informativeText = @"Describe what you'd like to change:";
    [alert addButtonWithTitle:@"Send"];
    [alert addButtonWithTitle:@"Cancel"];

    NSTextField *textField = [[NSTextField alloc] initWithFrame:NSMakeRect(0, 0, 400, 24)];
    textField.placeholderString = @"e.g. make the sidebar wider, add a reset button...";
    alert.accessoryView = textField;

    // Make text field first responder after sheet appears
    dispatch_async(dispatch_get_main_queue(), ^{
        [alert.window makeFirstResponder:textField];
    });

    NSWindow *keyWindow = [NSApp keyWindow];
    if (keyWindow) {
        [alert beginSheetModalForWindow:keyWindow completionHandler:^(NSModalResponse returnCode) {
            if (returnCode == NSAlertFirstButtonReturn) {
                NSString *text = textField.stringValue;
                if ([text length] > 0) {
                    GoNativeDialogResult(requestID, [text UTF8String]);
                } else {
                    GoNativeDialogResult(requestID, NULL);
                }
            } else {
                GoNativeDialogResult(requestID, NULL);
            }
        }];
    } else {
        // No key window — run as modal dialog
        NSModalResponse response = [alert runModal];
        if (response == NSAlertFirstButtonReturn) {
            NSString *text = textField.stringValue;
            if ([text length] > 0) {
                GoNativeDialogResult(requestID, [text UTF8String]);
            } else {
                GoNativeDialogResult(requestID, NULL);
            }
        } else {
            GoNativeDialogResult(requestID, NULL);
        }
    }
}
