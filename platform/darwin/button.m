#import <Cocoa/Cocoa.h>
#include "button.h"
#import <objc/runtime.h>

// Go callback bridge
extern void GoCallbackInvoke(uint64_t callbackID, const char* data);

static const void *kCallbackIDKey = &kCallbackIDKey;

@interface JVButtonTarget : NSObject
@property (nonatomic, assign) uint64_t callbackID;
- (void)buttonClicked:(id)sender;
@end

@implementation JVButtonTarget

- (void)buttonClicked:(id)sender {
    GoCallbackInvoke(self.callbackID, "");
}

@end

void* JVCreateButton(const char* label, const char* style, bool disabled, uint64_t callbackID) {
    NSString *labelStr = [NSString stringWithUTF8String:label];
    NSString *styleStr = [NSString stringWithUTF8String:style];

    NSButton *button;

    if ([styleStr isEqualToString:@"primary"]) {
        button = [NSButton buttonWithTitle:labelStr target:nil action:nil];
        button.bezelStyle = NSBezelStyleRounded;
        button.keyEquivalent = @"\r";
        button.bezelColor = [NSColor controlAccentColor];
    } else if ([styleStr isEqualToString:@"destructive"]) {
        button = [NSButton buttonWithTitle:labelStr target:nil action:nil];
        button.bezelStyle = NSBezelStyleRounded;
        if (@available(macOS 12.0, *)) {
            button.hasDestructiveAction = YES;
        } else {
            button.contentTintColor = [NSColor systemRedColor];
        }
    } else {
        button = [NSButton buttonWithTitle:labelStr target:nil action:nil];
        button.bezelStyle = NSBezelStyleRounded;
    }

    button.enabled = !disabled;
    button.translatesAutoresizingMaskIntoConstraints = NO;

    // Set up target-action
    JVButtonTarget *target = [[JVButtonTarget alloc] init];
    target.callbackID = callbackID;
    button.target = target;
    button.action = @selector(buttonClicked:);

    // Prevent target from being deallocated
    objc_setAssociatedObject(button, kCallbackIDKey, target, OBJC_ASSOCIATION_RETAIN_NONATOMIC);

    return (__bridge_retained void*)button;
}

void JVUpdateButton(void* handle, const char* label, const char* style, bool disabled) {
    NSButton *button = (__bridge NSButton*)handle;
    button.title = [NSString stringWithUTF8String:label];
    button.enabled = !disabled;
}

void JVUpdateButtonCallbackID(void* handle, uint64_t callbackID) {
    NSButton *button = (__bridge NSButton*)handle;
    JVButtonTarget *target = objc_getAssociatedObject(button, kCallbackIDKey);
    if (target) {
        target.callbackID = callbackID;
    }
}
