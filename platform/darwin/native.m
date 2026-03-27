#import <Cocoa/Cocoa.h>
#import <UserNotifications/UserNotifications.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#import <objc/runtime.h>

#include "native.h"

// Go callback for async dialog results.
extern void GoNativeDialogResult(uint64_t requestID, const char* result);

extern NSMutableDictionary<NSString*, NSWindow*> *windowMap;

// --- Notifications ---

static BOOL notificationAuthRequested = NO;

static void ensureNotificationAuth(void) {
    if (notificationAuthRequested) return;
    notificationAuthRequested = YES;

    UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
    [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
                         completionHandler:^(BOOL granted, NSError *error) {
        // Fire-and-forget; if denied, notifications just won't show.
    }];
}

void JVSendNotification(const char *title, const char *body, const char *subtitle) {
    ensureNotificationAuth();

    NSString *nsTitle = title ? [NSString stringWithUTF8String:title] : @"";
    NSString *nsBody = body ? [NSString stringWithUTF8String:body] : @"";
    NSString *nsSubtitle = subtitle ? [NSString stringWithUTF8String:subtitle] : @"";

    UNMutableNotificationContent *content = [[UNMutableNotificationContent alloc] init];
    content.title = nsTitle;
    content.body = nsBody;
    if (nsSubtitle.length > 0) {
        content.subtitle = nsSubtitle;
    }
    content.sound = [UNNotificationSound defaultSound];

    NSString *identifier = [[NSUUID UUID] UUIDString];
    UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:identifier
                                                                          content:content
                                                                          trigger:nil]; // Deliver immediately
    [[UNUserNotificationCenter currentNotificationCenter] addNotificationRequest:request
                                                           withCompletionHandler:nil];
}

// --- Clipboard ---

const char* JVClipboardRead(void) {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    NSString *text = [pb stringForType:NSPasteboardTypeString];
    if (!text) return NULL;
    return strdup([text UTF8String]);
}

void JVClipboardWrite(const char *text) {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    [pb clearContents];
    if (text) {
        [pb setString:[NSString stringWithUTF8String:text] forType:NSPasteboardTypeString];
    }
}

// --- Open URL ---

void JVOpenURL(const char *url) {
    if (!url) return;
    NSString *urlStr = [NSString stringWithUTF8String:url];
    NSURL *nsURL = [NSURL URLWithString:urlStr];
    if (!nsURL) {
        // Try as file path
        nsURL = [NSURL fileURLWithPath:urlStr];
    }
    [[NSWorkspace sharedWorkspace] openURL:nsURL];
}

// --- Helper: parse allowedTypes string into UTType array ---

static NSArray<UTType*>* parseAllowedTypes(const char *allowedTypes) {
    if (!allowedTypes || strlen(allowedTypes) == 0) return nil;

    NSString *typesStr = [NSString stringWithUTF8String:allowedTypes];
    NSArray<NSString *> *exts = [typesStr componentsSeparatedByString:@","];
    NSMutableArray<UTType *> *types = [NSMutableArray array];
    for (NSString *ext in exts) {
        NSString *trimmed = [ext stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
        UTType *type = [UTType typeWithFilenameExtension:trimmed];
        if (type) [types addObject:type];
    }
    return types.count > 0 ? types : nil;
}

// --- File Open Panel (non-blocking) ---
// Dispatches panel to main thread, calls GoNativeDialogResult with requestID when done.
// Does NOT block the main thread — uses beginWithCompletionHandler.

void JVFileOpenPanelAsync(const char *title, const char *allowedTypes, int allowMultiple, uint64_t requestID) {
    NSString *nsTitle = title ? [NSString stringWithUTF8String:title] : @"Open";
    NSArray<UTType*> *types = parseAllowedTypes(allowedTypes);
    BOOL multi = (allowMultiple != 0);

    dispatch_async(dispatch_get_main_queue(), ^{
        NSOpenPanel *panel = [NSOpenPanel openPanel];
        panel.title = nsTitle;
        panel.allowsMultipleSelection = multi;
        panel.canChooseFiles = YES;
        panel.canChooseDirectories = NO;
        if (types) panel.allowedContentTypes = types;

        // Use beginWithCompletionHandler — does NOT block the main thread.
        // The panel runs as a modeless sheet or standalone window.
        [panel beginWithCompletionHandler:^(NSModalResponse response) {
            if (response != NSModalResponseOK) {
                GoNativeDialogResult(requestID, NULL);
                return;
            }

            NSMutableArray *paths = [NSMutableArray array];
            for (NSURL *url in panel.URLs) {
                [paths addObject:url.path];
            }

            NSError *jsonErr = nil;
            NSData *data = [NSJSONSerialization dataWithJSONObject:paths options:0 error:&jsonErr];
            if (jsonErr) {
                GoNativeDialogResult(requestID, NULL);
                return;
            }

            NSString *json = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
            GoNativeDialogResult(requestID, [json UTF8String]);
        }];
    });
}

// --- File Save Panel (non-blocking) ---

void JVFileSavePanelAsync(const char *title, const char *defaultName, const char *allowedTypes, uint64_t requestID) {
    NSString *nsTitle = title ? [NSString stringWithUTF8String:title] : @"Save";
    NSString *nsDefaultName = defaultName ? [NSString stringWithUTF8String:defaultName] : nil;
    NSArray<UTType*> *types = parseAllowedTypes(allowedTypes);

    dispatch_async(dispatch_get_main_queue(), ^{
        NSSavePanel *panel = [NSSavePanel savePanel];
        panel.title = nsTitle;
        if (nsDefaultName) panel.nameFieldStringValue = nsDefaultName;
        if (types) panel.allowedContentTypes = types;

        [panel beginWithCompletionHandler:^(NSModalResponse response) {
            if (response != NSModalResponseOK) {
                GoNativeDialogResult(requestID, NULL);
                return;
            }
            GoNativeDialogResult(requestID, [panel.URL.path UTF8String]);
        }];
    });
}

// --- Alert (non-blocking) ---
// Uses beginSheetModalForWindow on key window, falls back to standalone if no window.

void JVAlertAsync(const char *title, const char *message, const char *style,
                  const char **buttons, int buttonCount, uint64_t requestID) {
    NSString *nsTitle = title ? [NSString stringWithUTF8String:title] : @"";
    NSString *nsMessage = message ? [NSString stringWithUTF8String:message] : @"";
    NSString *nsStyle = style ? [NSString stringWithUTF8String:style] : @"informational";

    // Copy button titles before async dispatch (C strings may be freed)
    NSMutableArray<NSString*> *btnTitles = [NSMutableArray array];
    if (buttonCount > 0 && buttons) {
        for (int i = 0; i < buttonCount; i++) {
            if (buttons[i]) {
                [btnTitles addObject:[NSString stringWithUTF8String:buttons[i]]];
            }
        }
    }

    dispatch_async(dispatch_get_main_queue(), ^{
        NSAlert *alert = [[NSAlert alloc] init];
        alert.messageText = nsTitle;
        alert.informativeText = nsMessage;

        if ([nsStyle isEqualToString:@"warning"]) {
            alert.alertStyle = NSAlertStyleWarning;
        } else if ([nsStyle isEqualToString:@"critical"]) {
            alert.alertStyle = NSAlertStyleCritical;
        } else {
            alert.alertStyle = NSAlertStyleInformational;
        }

        if (btnTitles.count > 0) {
            for (NSString *t in btnTitles) {
                [alert addButtonWithTitle:t];
            }
        } else {
            [alert addButtonWithTitle:@"OK"];
        }

        // Try to present as sheet on key window (non-blocking).
        NSWindow *keyWin = [NSApp keyWindow];
        if (keyWin) {
            [alert beginSheetModalForWindow:keyWin completionHandler:^(NSModalResponse response) {
                int idx = (int)(response - NSAlertFirstButtonReturn);
                char buf[16];
                snprintf(buf, sizeof(buf), "%d", idx);
                GoNativeDialogResult(requestID, buf);
            }];
        } else {
            // No window available — run modally (still in async dispatch, only blocks this invocation)
            NSModalResponse response = [alert runModal];
            int idx = (int)(response - NSAlertFirstButtonReturn);
            char buf[16];
            snprintf(buf, sizeof(buf), "%d", idx);
            GoNativeDialogResult(requestID, buf);
        }
    });
}
