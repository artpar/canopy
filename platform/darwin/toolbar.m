#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "toolbar.h"

extern void GoCallbackInvoke(uint64_t callbackID, const char* data);
extern NSMutableDictionary<NSString*, NSWindow*> *windowMap;

static const void *kToolbarDelegateKey = &kToolbarDelegateKey;
static NSString *const kToolbarID = @"JVToolbar";

// --- Toolbar item action target ---

@interface JVToolbarTarget : NSObject
@property (nonatomic, assign) uint64_t callbackID;
- (void)toolbarItemClicked:(id)sender;
@end

@implementation JVToolbarTarget
- (void)toolbarItemClicked:(id)sender {
    GoCallbackInvoke(self.callbackID, "");
}
@end

// --- Search toolbar item delegate ---

@interface JVSearchToolbarDelegate : NSObject <NSSearchFieldDelegate>
@property (nonatomic, assign) uint64_t searchCallbackID;
@end

@implementation JVSearchToolbarDelegate
- (void)controlTextDidChange:(NSNotification *)notification {
    NSSearchField *field = notification.object;
    NSString *text = field.stringValue;
    const char *cText = [text UTF8String];
    GoCallbackInvoke(self.searchCallbackID, cText);
}
@end

// --- Toolbar delegate ---

@interface JVToolbarDelegate : NSObject <NSToolbarDelegate>
@property (nonatomic, strong) NSArray<NSDictionary*> *itemSpecs;
@property (nonatomic, strong) NSMutableArray *targets;
@property (nonatomic, strong) NSMutableArray *searchDelegates;
@end

@implementation JVToolbarDelegate

- (NSArray<NSToolbarItemIdentifier> *)toolbarAllowedItemIdentifiers:(NSToolbar *)toolbar {
    return [self identifiers];
}

- (NSArray<NSToolbarItemIdentifier> *)toolbarDefaultItemIdentifiers:(NSToolbar *)toolbar {
    return [self identifiers];
}

- (NSArray<NSString*> *)identifiers {
    NSMutableArray *ids = [NSMutableArray array];
    for (NSDictionary *spec in self.itemSpecs) {
        if ([spec[@"separator"] boolValue]) {
            [ids addObject:NSToolbarSpaceItemIdentifier];
        } else if ([spec[@"flexible"] boolValue]) {
            [ids addObject:NSToolbarFlexibleSpaceItemIdentifier];
        } else {
            NSString *itemID = spec[@"id"] ?: @"";
            [ids addObject:itemID];
        }
    }
    return ids;
}

- (NSToolbarItem *)toolbar:(NSToolbar *)toolbar itemForItemIdentifier:(NSToolbarItemIdentifier)itemIdentifier willBeInsertedIntoToolbar:(BOOL)flag {
    for (NSDictionary *spec in self.itemSpecs) {
        NSString *specID = spec[@"id"] ?: @"";
        if (![specID isEqualToString:itemIdentifier]) continue;

        // Search field item
        if ([spec[@"searchField"] boolValue]) {
            NSSearchToolbarItem *searchItem = [[NSSearchToolbarItem alloc] initWithItemIdentifier:itemIdentifier];
            searchItem.searchField.placeholderString = spec[@"label"] ?: @"Search";

            NSNumber *searchCbID = spec[@"searchCallbackID"];
            if (searchCbID && [searchCbID unsignedLongLongValue] > 0) {
                JVSearchToolbarDelegate *searchDel = [[JVSearchToolbarDelegate alloc] init];
                searchDel.searchCallbackID = [searchCbID unsignedLongLongValue];
                searchItem.searchField.delegate = searchDel;
                [self.searchDelegates addObject:searchDel];
            }

            return searchItem;
        }

        // Regular toolbar item
        NSToolbarItem *item = [[NSToolbarItem alloc] initWithItemIdentifier:itemIdentifier];
        item.label = spec[@"label"] ?: @"";
        item.toolTip = spec[@"label"] ?: @"";

        // SF Symbol icon
        NSString *iconName = spec[@"icon"];
        if (iconName && [iconName length] > 0) {
            NSImage *image = [NSImage imageWithSystemSymbolName:iconName accessibilityDescription:spec[@"label"]];
            if (image) {
                item.image = image;
            }
        }

        // Standard action (responder chain)
        NSString *stdAction = spec[@"standardAction"];
        if (stdAction && [stdAction length] > 0) {
            item.action = NSSelectorFromString(stdAction);
            item.target = nil; // responder chain
        }

        // Custom callback
        NSNumber *cbID = spec[@"callbackID"];
        if (cbID && [cbID unsignedLongLongValue] > 0) {
            JVToolbarTarget *target = [[JVToolbarTarget alloc] init];
            target.callbackID = [cbID unsignedLongLongValue];
            item.target = target;
            item.action = @selector(toolbarItemClicked:);
            [self.targets addObject:target];
        }

        // Enabled state (default true)
        NSNumber *enabledVal = spec[@"enabled"];
        if (enabledVal) {
            item.enabled = [enabledVal boolValue];
        }

        // Bordered appearance (protocol-driven, macOS 11+)
        if ([spec[@"bordered"] boolValue]) {
            if (@available(macOS 11.0, *)) {
                item.bordered = YES;
            }
        }

        // Toggle items (hasToggle=true) — always create a button with ON/OFF state
        if ([spec[@"hasToggle"] boolValue]) {
            NSButton *btn = [NSButton buttonWithImage:item.image ?: [NSImage new] target:item.target action:item.action];
            btn.buttonType = NSButtonTypePushOnPushOff;
            btn.bezelStyle = NSBezelStyleToolbar;
            btn.state = [spec[@"selected"] boolValue] ? NSControlStateValueOn : NSControlStateValueOff;
            btn.toolTip = item.toolTip;
            if (enabledVal) btn.enabled = [enabledVal boolValue];
            item.view = btn;
        }

        return item;
    }

    return nil;
}

@end

void JVUpdateToolbar(const char* surfaceID, const char* itemsJSON) {
    NSString *sid = [NSString stringWithUTF8String:surfaceID];
    NSWindow *window = windowMap[sid];
    if (!window) return;

    NSData *data = [NSData dataWithBytes:itemsJSON length:strlen(itemsJSON)];
    NSArray *items = [NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
    if (!items) return;

    JVToolbarDelegate *delegate = [[JVToolbarDelegate alloc] init];
    delegate.itemSpecs = items;
    delegate.targets = [[NSMutableArray alloc] init];
    delegate.searchDelegates = [[NSMutableArray alloc] init];

    NSToolbar *toolbar = [[NSToolbar alloc] initWithIdentifier:kToolbarID];
    toolbar.delegate = delegate;
    toolbar.displayMode = NSToolbarDisplayModeIconOnly;

    window.toolbar = toolbar;

    // Retain delegate on the window to prevent dealloc
    objc_setAssociatedObject(window, kToolbarDelegateKey, delegate, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
}
