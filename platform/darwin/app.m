#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "app.h"
#include "menu.h"

// Map from surfaceID → NSWindow (non-static so screenshot.m can access via extern)
NSMutableDictionary<NSString*, NSWindow*> *windowMap = nil;

// Forward-declare for use in delegate method
static NSStatusItem *statusItem = nil;
static NSMenuItem *refineMenuItem = nil;

@interface JVAppDelegate : NSObject <NSApplicationDelegate> {
    BOOL _forceQuit;
}
- (void)jvForceQuit:(id)sender;
@end

extern void GoFollowUpTriggered(void);
extern void GoStatusMenuAppClicked(const char* appPath);
extern void GoStatusMenuSettingsClicked(void);

@implementation JVAppDelegate

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    // Don't set activation policy here — it's set by JVSetAppMode
}

- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender {
    // In menubar mode, keep running when windows close
    if (statusItem != nil) return NO;
    return YES;
}

- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)sender {
    // In menubar mode, Cmd+Q hides all windows instead of quitting.
    // Only the explicit "Quit jview" menu item (terminate:) bypasses this
    // because we set a flag before calling terminate:.
    if (statusItem != nil && !_forceQuit) {
        // Hide all windows
        for (NSString *sid in windowMap) {
            [windowMap[sid] orderOut:nil];
        }
        return NSTerminateCancel;
    }
    return NSTerminateNow;
}

- (void)refineUI:(id)sender {
    GoFollowUpTriggered();
}

// Show all jview windows and activate the app
- (void)jvShowAllWindows:(id)sender {
    for (NSString *sid in windowMap) {
        NSWindow *w = windowMap[sid];
        [w makeKeyAndOrderFront:nil];
    }
    [NSApp activateIgnoringOtherApps:YES];
}

// Handle "Settings..." menu item click
- (void)jvSettingsClicked:(id)sender {
    GoStatusMenuSettingsClicked();
}

// Handle app launch from Apps submenu
- (void)jvLaunchApp:(id)sender {
    NSString *path = [sender representedObject];
    if (path && [path length] > 0) {
        GoStatusMenuAppClicked([path UTF8String]);
    }
}

// Show customized About panel
- (void)jvShowAbout:(id)sender {
    [NSApp activateIgnoringOtherApps:YES];
    [NSApp orderFrontStandardAboutPanelWithOptions:@{
        @"ApplicationName": @"Canopy",
        @"ApplicationVersion": @"0.1",
        @"Version": @"1",
        @"Copyright": @"Native macOS UI renderer for A2UI protocol.\nNo webview. No Electron. Pure AppKit.",
    }];
}

// Force quit — called by the Quit menu item in the status bar menu
- (void)jvForceQuit:(id)sender {
    _forceQuit = YES;
    [NSApp terminate:nil];
}

// Validate menu items: enable responder-chain actions that this delegate handles
- (BOOL)validateMenuItem:(NSMenuItem *)menuItem {
    if ([menuItem action] == @selector(jvShowAllWindows:)) return [windowMap count] > 0;
    if ([menuItem action] == @selector(jvSettingsClicked:)) return YES;
    if ([menuItem action] == @selector(jvLaunchApp:)) return YES;
    if ([menuItem action] == @selector(jvShowAbout:)) return YES;
    if ([menuItem action] == @selector(jvForceQuit:)) return YES;
    if ([menuItem action] == @selector(refineUI:)) return menuItem.enabled;
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

    refineMenuItem = [[NSMenuItem alloc] initWithTitle:@"Refine UI..."
                                                action:@selector(refineUI:)
                                         keyEquivalent:@"l"];
    refineMenuItem.enabled = NO;
    [appMenu addItem:refineMenuItem];
    [appMenu addItem:[NSMenuItem separatorItem]];

    NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"Quit Canopy"
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

// Tag used to identify the dynamic items section in the status menu.
// Items between two separators with this tag are replaced on update.
static const NSInteger kDynamicSectionTag = 9999;

// Retained targets for dynamic menu item callbacks
static NSMutableArray *statusMenuTargets = nil;

// Build the permanent portion of the status menu (apps submenu, settings, about, quit).
// Called once; dynamic items are inserted later via JVSetStatusMenuDynamic.
static void rebuildStatusMenu(void) {
    if (!statusItem) return;

    NSMenu *menu = statusItem.menu;
    if (!menu) {
        menu = [[NSMenu alloc] init];
        [menu setAutoenablesItems:NO];
        statusItem.menu = menu;
    }

    [menu removeAllItems];

    // --- Show Windows ---
    NSMenuItem *showItem = [[NSMenuItem alloc] initWithTitle:@"Show Windows"
                                                       action:@selector(jvShowAllWindows:)
                                                keyEquivalent:@""];
    showItem.target = nil; // responder chain
    [menu addItem:showItem];

    [menu addItem:[NSMenuItem separatorItem]];

    // --- Dynamic section placeholder ---
    // A tagged separator marks where dynamic items will be inserted
    NSMenuItem *dynStart = [NSMenuItem separatorItem];
    dynStart.tag = kDynamicSectionTag;
    [menu addItem:dynStart];

    // --- Settings ---
    NSMenuItem *settingsItem = [[NSMenuItem alloc] initWithTitle:@"Settings..."
                                                           action:@selector(jvSettingsClicked:)
                                                    keyEquivalent:@","];
    settingsItem.target = nil; // responder chain — handled by app delegate
    if (@available(macOS 11.0, *)) {
        NSImage *gearImg = [NSImage imageWithSystemSymbolName:@"gearshape" accessibilityDescription:@"Settings"];
        if (gearImg) {
            gearImg.size = NSMakeSize(16, 16);
            settingsItem.image = gearImg;
        }
    }
    [menu addItem:settingsItem];

    // --- About ---
    NSMenuItem *aboutItem = [[NSMenuItem alloc] initWithTitle:@"About Canopy"
                                                        action:@selector(jvShowAbout:)
                                                 keyEquivalent:@""];
    aboutItem.target = nil; // responder chain → app delegate
    [menu addItem:aboutItem];

    [menu addItem:[NSMenuItem separatorItem]];

    // --- Quit ---
    NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"Quit Canopy"
                                                       action:@selector(jvForceQuit:)
                                                keyEquivalent:@"q"];
    quitItem.target = nil; // responder chain → app delegate
    [menu addItem:quitItem];
}

void JVSetAppMode(const char* mode, const char* icon, const char* title, uint64_t callbackID) {
    NSString *modeStr = [NSString stringWithUTF8String:mode];

    if ([modeStr isEqualToString:@"menubar"]) {
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

        // Create status item if not exists
        if (!statusItem) {
            statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
            rebuildStatusMenu();
        }

        // Set icon (SF Symbol) or title
        if (icon && strlen(icon) > 0) {
            NSString *iconName = [NSString stringWithUTF8String:icon];
            NSImage *img = [NSImage imageWithSystemSymbolName:iconName accessibilityDescription:nil];
            if (img) {
                statusItem.button.image = img;
                statusItem.button.title = @"";
            } else {
                statusItem.button.image = nil;
                statusItem.button.title = (title && strlen(title) > 0)
                    ? [NSString stringWithUTF8String:title] : @"Canopy";
            }
        } else {
            statusItem.button.image = nil;
            statusItem.button.title = (title && strlen(title) > 0)
                ? [NSString stringWithUTF8String:title] : @"Canopy";
        }

    } else if ([modeStr isEqualToString:@"accessory"]) {
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

        if (statusItem) {
            [[NSStatusBar systemStatusBar] removeStatusItem:statusItem];
            statusItem = nil;
        }

    } else {
        // "normal" — restore dock icon
        [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
        [NSApp activateIgnoringOtherApps:YES];

        if (statusItem) {
            [[NSStatusBar systemStatusBar] removeStatusItem:statusItem];
            statusItem = nil;
        }
    }
}

// Insert or replace dynamic items in the status menu.
// Items are placed right after the tagged separator (kDynamicSectionTag)
// and before the Settings item.
void JVSetStatusMenuDynamic(const char* itemsJSON) {
    if (!statusItem || !statusItem.menu) return;

    NSMenu *menu = statusItem.menu;

    // Find the tagged separator
    NSInteger dynIdx = -1;
    for (NSInteger i = 0; i < [menu numberOfItems]; i++) {
        if ([[menu itemAtIndex:i] tag] == kDynamicSectionTag) {
            dynIdx = i;
            break;
        }
    }
    if (dynIdx < 0) return;

    // Remove old dynamic items (everything between dynIdx+1 and next separator/Settings)
    NSInteger removeStart = dynIdx + 1;
    while (removeStart < [menu numberOfItems]) {
        NSMenuItem *item = [menu itemAtIndex:removeStart];
        // Stop at Settings or About or Quit (permanent items)
        if ([[item title] isEqualToString:@"Settings..."] ||
            [[item title] isEqualToString:@"About Canopy"] ||
            [[item title] isEqualToString:@"Quit Canopy"]) {
            break;
        }
        [menu removeItemAtIndex:removeStart];
    }

    // Parse and insert new dynamic items
    if (!itemsJSON || strlen(itemsJSON) == 0) return;

    NSData *data = [NSData dataWithBytes:itemsJSON length:strlen(itemsJSON)];
    NSArray *items = [NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
    if (!items) return;

    statusMenuTargets = [[NSMutableArray alloc] init];

    NSInteger insertIdx = dynIdx + 1;
    for (NSDictionary *spec in items) {
        NSMenuItem *item = JVBuildMenuItem(spec, statusMenuTargets);
        if (item) {
            [menu insertItem:item atIndex:insertIdx];
            insertIdx++;
        }
    }

    // Add separator after dynamic items if any were added
    if ([items count] > 0) {
        [menu insertItem:[NSMenuItem separatorItem] atIndex:insertIdx];
    }
}

// Set the "Apps" submenu items in the status menu.
// Inserts an "Apps" submenu at the top of the menu (index 0).
void JVSetStatusMenuApps(const char* itemsJSON) {
    if (!statusItem || !statusItem.menu) return;

    NSMenu *menu = statusItem.menu;

    // Remove existing Apps submenu if present (always at index 0 if it exists)
    if ([menu numberOfItems] > 0 && [[[menu itemAtIndex:0] title] isEqualToString:@"Apps"]) {
        [menu removeItemAtIndex:0];
        // Also remove the separator after it if present
        if ([menu numberOfItems] > 0 && [[menu itemAtIndex:0] isSeparatorItem]) {
            // The "Show Windows" item follows — don't remove the separator before it
        }
    }

    if (!itemsJSON || strlen(itemsJSON) == 0) return;

    NSData *data = [NSData dataWithBytes:itemsJSON length:strlen(itemsJSON)];
    NSArray *items = [NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
    if (!items || [items count] == 0) return;

    NSMenu *appsSubmenu = [[NSMenu alloc] initWithTitle:@"Apps"];
    [appsSubmenu setAutoenablesItems:NO];

    for (NSDictionary *spec in items) {
        NSString *label = spec[@"label"] ?: @"";
        NSString *path = spec[@"path"] ?: @"";

        NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:label
                                                       action:@selector(jvLaunchApp:)
                                                keyEquivalent:@""];
        item.target = nil; // responder chain — handled by app delegate
        item.representedObject = path;

        // SF Symbol icon
        NSString *iconName = spec[@"icon"];
        if (iconName && [iconName length] > 0) {
            NSImage *image = [NSImage imageWithSystemSymbolName:iconName accessibilityDescription:label];
            if (image) {
                image.size = NSMakeSize(16, 16);
                item.image = image;
            }
        }

        [appsSubmenu addItem:item];
    }

    NSMenuItem *appsItem = [[NSMenuItem alloc] initWithTitle:@"Apps"
                                                       action:nil
                                                keyEquivalent:@""];
    if (@available(macOS 11.0, *)) {
        NSImage *appsImg = [NSImage imageWithSystemSymbolName:@"square.grid.2x2" accessibilityDescription:@"Apps"];
        if (appsImg) {
            appsImg.size = NSMakeSize(16, 16);
            appsItem.image = appsImg;
        }
    }
    [appsItem setSubmenu:appsSubmenu];
    [menu insertItem:appsItem atIndex:0];
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
