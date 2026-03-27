#import <Cocoa/Cocoa.h>
#import <UserNotifications/UserNotifications.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#import <objc/runtime.h>

#include "native.h"

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

// --- File Open Panel ---

const char* JVFileOpenPanel(const char *title, const char *allowedTypes, int allowMultiple) {
    __block NSString *result = nil;

    dispatch_block_t block = ^{
        NSOpenPanel *panel = [NSOpenPanel openPanel];
        if (title) panel.title = [NSString stringWithUTF8String:title];
        panel.allowsMultipleSelection = (allowMultiple != 0);
        panel.canChooseFiles = YES;
        panel.canChooseDirectories = NO;

        if (allowedTypes && strlen(allowedTypes) > 0) {
            NSString *typesStr = [NSString stringWithUTF8String:allowedTypes];
            NSArray<NSString *> *exts = [typesStr componentsSeparatedByString:@","];
            NSMutableArray<UTType *> *types = [NSMutableArray array];
            for (NSString *ext in exts) {
                NSString *trimmed = [ext stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
                UTType *type = [UTType typeWithFilenameExtension:trimmed];
                if (type) [types addObject:type];
            }
            if (types.count > 0) {
                panel.allowedContentTypes = types;
            }
        }

        NSModalResponse response = [panel runModal];
        if (response != NSModalResponseOK) return;

        NSMutableArray *paths = [NSMutableArray array];
        for (NSURL *url in panel.URLs) {
            [paths addObject:url.path];
        }

        NSError *jsonErr = nil;
        NSData *data = [NSJSONSerialization dataWithJSONObject:paths options:0 error:&jsonErr];
        if (!jsonErr) {
            result = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
        }
    };

    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }

    if (!result) return NULL;
    return strdup([result UTF8String]);
}

// --- File Save Panel ---

const char* JVFileSavePanel(const char *title, const char *defaultName, const char *allowedTypes) {
    __block NSString *result = nil;

    dispatch_block_t block = ^{
        NSSavePanel *panel = [NSSavePanel savePanel];
        if (title) panel.title = [NSString stringWithUTF8String:title];
        if (defaultName) panel.nameFieldStringValue = [NSString stringWithUTF8String:defaultName];

        if (allowedTypes && strlen(allowedTypes) > 0) {
            NSString *typesStr = [NSString stringWithUTF8String:allowedTypes];
            NSArray<NSString *> *exts = [typesStr componentsSeparatedByString:@","];
            NSMutableArray<UTType *> *types = [NSMutableArray array];
            for (NSString *ext in exts) {
                NSString *trimmed = [ext stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
                UTType *type = [UTType typeWithFilenameExtension:trimmed];
                if (type) [types addObject:type];
            }
            if (types.count > 0) {
                panel.allowedContentTypes = types;
            }
        }

        NSModalResponse response = [panel runModal];
        if (response == NSModalResponseOK) {
            result = panel.URL.path;
        }
    };

    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }

    if (!result) return NULL;
    return strdup([result UTF8String]);
}

// --- Alert ---

int JVAlert(const char *title, const char *message, const char *style, const char **buttons, int buttonCount) {
    __block int clickedIndex = 0;

    dispatch_block_t block = ^{
        NSAlert *alert = [[NSAlert alloc] init];
        if (title) alert.messageText = [NSString stringWithUTF8String:title];
        if (message) alert.informativeText = [NSString stringWithUTF8String:message];

        if (style) {
            NSString *s = [NSString stringWithUTF8String:style];
            if ([s isEqualToString:@"warning"]) {
                alert.alertStyle = NSAlertStyleWarning;
            } else if ([s isEqualToString:@"critical"]) {
                alert.alertStyle = NSAlertStyleCritical;
            } else {
                alert.alertStyle = NSAlertStyleInformational;
            }
        }

        if (buttonCount > 0 && buttons) {
            for (int i = 0; i < buttonCount; i++) {
                if (buttons[i]) {
                    [alert addButtonWithTitle:[NSString stringWithUTF8String:buttons[i]]];
                }
            }
        } else {
            [alert addButtonWithTitle:@"OK"];
        }

        NSModalResponse response = [alert runModal];
        // NSAlertFirstButtonReturn = 1000, second = 1001, etc.
        clickedIndex = (int)(response - NSAlertFirstButtonReturn);
    };

    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }

    return clickedIndex;
}
