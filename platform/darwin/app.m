#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "app.h"

// Map from surfaceID → NSWindow
static NSMutableDictionary<NSString*, NSWindow*> *windowMap = nil;

@interface JVAppDelegate : NSObject <NSApplicationDelegate>
@end

@implementation JVAppDelegate

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    // Activate the app and bring to front
    [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
    [NSApp activateIgnoringOtherApps:YES];
}

- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender {
    return YES;
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
    NSString *sid = [NSString stringWithUTF8String:surfaceID];
    NSString *windowTitle = [NSString stringWithUTF8String:title];

    NSRect frame = NSMakeRect(0, 0, width, height);
    NSWindowStyleMask styleMask = NSWindowStyleMaskTitled |
                                   NSWindowStyleMaskClosable |
                                   NSWindowStyleMaskMiniaturizable |
                                   NSWindowStyleMaskResizable;

    NSWindow *window = [[NSWindow alloc] initWithContentRect:frame
                                                   styleMask:styleMask
                                                     backing:NSBackingStoreBuffered
                                                       defer:NO];
    [window setTitle:windowTitle];
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

void JVDestroyWindow(const char* surfaceID) {
    NSString *sid = [NSString stringWithUTF8String:surfaceID];
    NSWindow *window = windowMap[sid];
    if (window) {
        [window close];
        [windowMap removeObjectForKey:sid];
    }
}

void JVSetWindowRootView(const char* surfaceID, void* view, int padding) {
    NSString *sid = [NSString stringWithUTF8String:surfaceID];
    NSWindow *window = windowMap[sid];
    if (!window) return;

    NSView *nsView = (__bridge NSView*)view;
    nsView.translatesAutoresizingMaskIntoConstraints = NO;

    // Remove existing subviews
    NSView *contentView = window.contentView;
    for (NSView *sub in [contentView.subviews copy]) {
        [sub removeFromSuperview];
    }

    [contentView addSubview:nsView];

    // Use specified padding; default to 20 if not set (padding==0 means no value provided from protocol)
    // To get edge-to-edge, set padding to -1 in protocol which maps to 0 here
    CGFloat inset = (padding > 0) ? (CGFloat)padding : (padding < 0) ? 0.0 : 20.0;
    NSLayoutConstraint *bottom = [nsView.bottomAnchor constraintLessThanOrEqualToAnchor:contentView.bottomAnchor constant:-inset];
    [NSLayoutConstraint activateConstraints:@[
        [nsView.topAnchor constraintEqualToAnchor:contentView.topAnchor constant:inset],
        [nsView.leadingAnchor constraintEqualToAnchor:contentView.leadingAnchor constant:inset],
        [nsView.trailingAnchor constraintEqualToAnchor:contentView.trailingAnchor constant:-inset],
        bottom,
    ]];
}
