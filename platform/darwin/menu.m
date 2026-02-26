#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "menu.h"

extern void GoCallbackInvoke(uint64_t callbackID, const char* data);

static const void *kMenuTargetKey = &kMenuTargetKey;

@interface JVMenuTarget : NSObject
@property (nonatomic, assign) uint64_t callbackID;
- (void)menuItemClicked:(id)sender;
@end

@implementation JVMenuTarget

- (void)menuItemClicked:(id)sender {
    GoCallbackInvoke(self.callbackID, "");
}

@end

static NSMenuItem* buildMenuItem(NSDictionary *spec, NSMutableArray *targets) {
    // Separator
    if ([spec[@"separator"] boolValue]) {
        return [NSMenuItem separatorItem];
    }

    NSString *label = spec[@"label"] ?: @"";
    NSString *keyEquiv = spec[@"keyEquivalent"] ?: @"";

    // Determine key equivalent modifier flags
    NSEventModifierFlags modFlags = NSEventModifierFlagCommand;
    NSString *actualKey = keyEquiv;

    // Check explicit keyModifiers field
    NSString *keyMods = spec[@"keyModifiers"];
    if (keyMods && [keyMods length] > 0) {
        modFlags = NSEventModifierFlagCommand;
        if ([keyMods containsString:@"option"]) {
            modFlags |= NSEventModifierFlagOption;
        }
        if ([keyMods containsString:@"shift"]) {
            modFlags |= NSEventModifierFlagShift;
        }
        actualKey = [keyEquiv lowercaseString];
    } else if ([keyEquiv length] == 1) {
        // Uppercase letter means Cmd+Shift (legacy convention)
        unichar ch = [keyEquiv characterAtIndex:0];
        if (ch >= 'A' && ch <= 'Z') {
            modFlags = NSEventModifierFlagCommand | NSEventModifierFlagShift;
            actualKey = [keyEquiv lowercaseString];
        }
    }

    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:label
                                                  action:nil
                                           keyEquivalent:actualKey];
    [item setKeyEquivalentModifierMask:modFlags];

    // Standard action: use NSSelectorFromString with target=nil (responder chain)
    NSString *stdAction = spec[@"standardAction"];
    if (stdAction && [stdAction length] > 0) {
        item.action = NSSelectorFromString(stdAction);
        item.target = nil; // responder chain
    }

    // Custom callback
    NSNumber *cbID = spec[@"callbackID"];
    if (cbID && [cbID unsignedLongLongValue] > 0) {
        JVMenuTarget *target = [[JVMenuTarget alloc] init];
        target.callbackID = [cbID unsignedLongLongValue];
        item.target = target;
        item.action = @selector(menuItemClicked:);
        [targets addObject:target]; // prevent dealloc
    }

    // SF Symbol icon
    NSString *iconName = spec[@"icon"];
    if (iconName && [iconName length] > 0) {
        NSImage *image = [NSImage imageWithSystemSymbolName:iconName accessibilityDescription:label];
        if (image) {
            image.size = NSMakeSize(16, 16);
            item.image = image;
        }
    }

    // Disabled state
    if ([spec[@"disabled"] boolValue]) {
        item.enabled = NO;
    }

    // Children → submenu
    NSArray *children = spec[@"children"];
    if (children && [children count] > 0) {
        NSMenu *submenu = [[NSMenu alloc] initWithTitle:label];
        [submenu setAutoenablesItems:NO];
        for (NSDictionary *child in children) {
            NSMenuItem *childItem = buildMenuItem(child, targets);
            if (childItem) {
                [submenu addItem:childItem];
            }
        }
        [item setSubmenu:submenu];
    }

    return item;
}

void JVUpdateMenu(const char* surfaceID, const char* itemsJSON) {
    NSData *data = [NSData dataWithBytes:itemsJSON length:strlen(itemsJSON)];
    NSArray *items = [NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
    if (!items) return;

    NSMenu *mainMenu = [NSApp mainMenu];
    if (!mainMenu) return;

    // Preserve the app menu (index 0)
    NSMenuItem *appMenuItem = nil;
    if ([mainMenu numberOfItems] > 0) {
        appMenuItem = [mainMenu itemAtIndex:0];
    }

    // Collect targets to retain
    NSMutableArray *targets = [[NSMutableArray alloc] init];

    // Rebuild menu bar
    [mainMenu removeAllItems];
    if (appMenuItem) {
        [mainMenu addItem:appMenuItem];
    }

    for (NSDictionary *spec in items) {
        NSMenuItem *item = buildMenuItem(spec, targets);
        if (item) {
            [mainMenu addItem:item];
        }
    }

    // Retain targets on the menu bar to prevent dealloc
    objc_setAssociatedObject(mainMenu, kMenuTargetKey, targets, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
}

void JVPerformAction(const char* selector) {
    NSString *selStr = [NSString stringWithUTF8String:selector];
    SEL sel = NSSelectorFromString(selStr);
    [NSApp sendAction:sel to:nil from:nil];
}
