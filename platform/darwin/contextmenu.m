#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "contextmenu.h"

extern void GoCallbackInvoke(uint64_t callbackID, const char* data);

static const void *kContextMenuTargetKey = &kContextMenuTargetKey;

@interface JVContextMenuTarget : NSObject
@property (nonatomic, assign) uint64_t callbackID;
- (void)menuItemClicked:(id)sender;
@end

@implementation JVContextMenuTarget
- (void)menuItemClicked:(id)sender {
    GoCallbackInvoke(self.callbackID, "");
}
@end

static NSMenuItem* buildContextMenuItem(NSDictionary *spec, NSMutableArray *targets) {
    if ([spec[@"separator"] boolValue]) {
        return [NSMenuItem separatorItem];
    }

    NSString *label = spec[@"label"] ?: @"";
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:label action:nil keyEquivalent:@""];

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

    // Standard action
    NSString *stdAction = spec[@"standardAction"];
    if (stdAction && [stdAction length] > 0) {
        item.action = NSSelectorFromString(stdAction);
        item.target = nil;
    }

    // Custom callback
    NSNumber *cbID = spec[@"callbackID"];
    if (cbID && [cbID unsignedLongLongValue] > 0) {
        JVContextMenuTarget *target = [[JVContextMenuTarget alloc] init];
        target.callbackID = [cbID unsignedLongLongValue];
        item.target = target;
        item.action = @selector(menuItemClicked:);
        [targets addObject:target];
    }

    // Children → submenu
    NSArray *children = spec[@"children"];
    if (children && [children count] > 0) {
        NSMenu *submenu = [[NSMenu alloc] initWithTitle:label];
        [submenu setAutoenablesItems:NO];
        for (NSDictionary *child in children) {
            NSMenuItem *childItem = buildContextMenuItem(child, targets);
            if (childItem) {
                [submenu addItem:childItem];
            }
        }
        [item setSubmenu:submenu];
    }

    return item;
}

void JVAttachContextMenu(void* handle, const char* menuJSON) {
    NSView *view = (__bridge NSView*)handle;

    NSData *data = [NSData dataWithBytes:menuJSON length:strlen(menuJSON)];
    NSArray *items = [NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
    if (!items || [items count] == 0) {
        view.menu = nil;
        return;
    }

    NSMutableArray *targets = [[NSMutableArray alloc] init];
    NSMenu *menu = [[NSMenu alloc] init];
    [menu setAutoenablesItems:NO];

    for (NSDictionary *spec in items) {
        NSMenuItem *item = buildContextMenuItem(spec, targets);
        if (item) {
            [menu addItem:item];
        }
    }

    // For scroll views (OutlineView, RichTextEditor), attach menu to documentView
    // so the inner view's menuForEvent: fires correctly
    NSView *targetView = view;
    if ([view isKindOfClass:[NSScrollView class]]) {
        NSScrollView *scrollView = (NSScrollView*)view;
        if (scrollView.documentView) {
            targetView = scrollView.documentView;
        }
    }
    targetView.menu = menu;

    // Retain targets on the view to prevent dealloc
    objc_setAssociatedObject(targetView, kContextMenuTargetKey, targets, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
}
