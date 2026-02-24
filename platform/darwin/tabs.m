#import <Cocoa/Cocoa.h>
#include "tabs.h"
#import <objc/runtime.h>

extern void GoCallbackInvoke(uint64_t callbackID, const char* data);

static const void *kTabsTargetKey = &kTabsTargetKey;
static const void *kTabsChildIDsKey = &kTabsChildIDsKey;

@interface JVTabsDelegate : NSObject <NSTabViewDelegate>
@property (nonatomic, assign) uint64_t callbackID;
@end

@implementation JVTabsDelegate

- (void)tabView:(NSTabView *)tabView didSelectTabViewItem:(NSTabViewItem *)tabViewItem {
    if (self.callbackID == 0) return;

    NSArray<NSString*> *childIDs = objc_getAssociatedObject(tabView, kTabsChildIDsKey);
    NSInteger idx = [tabView indexOfTabViewItem:tabViewItem];
    if (childIDs && idx >= 0 && idx < (NSInteger)childIDs.count) {
        NSString *selectedID = childIDs[idx];
        GoCallbackInvoke(self.callbackID, [selectedID UTF8String]);
    }
}

@end

void* JVCreateTabs(const char** labels, int count, const char* activeTab, uint64_t callbackID) {
    NSTabView *tabView = [[NSTabView alloc] initWithFrame:NSZeroRect];
    tabView.translatesAutoresizingMaskIntoConstraints = NO;

    for (int i = 0; i < count; i++) {
        NSString *label = [NSString stringWithUTF8String:labels[i]];
        NSTabViewItem *item = [[NSTabViewItem alloc] initWithIdentifier:label];
        item.label = label;
        [tabView addTabViewItem:item];
    }

    JVTabsDelegate *delegate = [[JVTabsDelegate alloc] init];
    delegate.callbackID = callbackID;
    tabView.delegate = delegate;

    objc_setAssociatedObject(tabView, kTabsTargetKey, delegate, OBJC_ASSOCIATION_RETAIN_NONATOMIC);

    // Select active tab by matching child ID (set later via childIDs, for now use index from activeTab)
    // activeTab will be matched in JVTabsSetChildren when childIDs are known

    return (__bridge_retained void*)tabView;
}

void JVUpdateTabs(void* handle, const char** labels, int count, const char* activeTab) {
    NSTabView *tabView = (__bridge NSTabView*)handle;

    // Update tab labels — remove excess, add missing, update existing
    NSInteger existing = tabView.numberOfTabViewItems;

    for (int i = 0; i < count; i++) {
        NSString *label = [NSString stringWithUTF8String:labels[i]];
        if (i < existing) {
            NSTabViewItem *item = [tabView tabViewItemAtIndex:i];
            item.label = label;
        } else {
            NSTabViewItem *item = [[NSTabViewItem alloc] initWithIdentifier:label];
            item.label = label;
            [tabView addTabViewItem:item];
        }
    }

    // Remove excess tabs from end
    while (tabView.numberOfTabViewItems > count) {
        NSTabViewItem *last = [tabView tabViewItemAtIndex:tabView.numberOfTabViewItems - 1];
        [tabView removeTabViewItem:last];
    }

    // Select active tab by child ID
    NSString *activeStr = [NSString stringWithUTF8String:activeTab];
    if (activeStr.length > 0) {
        NSArray<NSString*> *childIDs = objc_getAssociatedObject(tabView, kTabsChildIDsKey);
        if (childIDs) {
            for (NSUInteger i = 0; i < childIDs.count && i < (NSUInteger)tabView.numberOfTabViewItems; i++) {
                if ([childIDs[i] isEqualToString:activeStr]) {
                    [tabView selectTabViewItemAtIndex:i];
                    break;
                }
            }
        }
    }
}

void JVTabsSetChildIDs(void* handle, const char** childIDs, int count) {
    NSTabView *tabView = (__bridge NSTabView*)handle;
    NSMutableArray<NSString*> *ids = [NSMutableArray arrayWithCapacity:count];
    for (int i = 0; i < count; i++) {
        [ids addObject:[NSString stringWithUTF8String:childIDs[i]]];
    }
    objc_setAssociatedObject(tabView, kTabsChildIDsKey, ids, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
}

void JVTabsSetChildren(void* handle, void** children, int count) {
    NSTabView *tabView = (__bridge NSTabView*)handle;

    // Ensure we have enough tab items
    while (tabView.numberOfTabViewItems < count) {
        NSTabViewItem *item = [[NSTabViewItem alloc] initWithIdentifier:@""];
        item.label = @"";
        [tabView addTabViewItem:item];
    }

    // Set each child view as the content of the corresponding tab item
    for (int i = 0; i < count && i < tabView.numberOfTabViewItems; i++) {
        NSTabViewItem *item = [tabView tabViewItemAtIndex:i];
        NSView *childView = (__bridge NSView*)children[i];

        // Remove existing content
        NSView *oldView = item.view;
        if (oldView) {
            for (NSView *sub in [oldView.subviews copy]) {
                [sub removeFromSuperview];
            }
        }

        // Set the child as the tab item's view.
        // Container keeps translatesAutoresizingMaskIntoConstraints=YES (default)
        // so NSTabView can manage its frame. Child uses Auto Layout inside it.
        NSView *container = [[NSView alloc] initWithFrame:NSZeroRect];
        childView.translatesAutoresizingMaskIntoConstraints = NO;
        [container addSubview:childView];

        [NSLayoutConstraint activateConstraints:@[
            [childView.topAnchor constraintEqualToAnchor:container.topAnchor],
            [childView.leadingAnchor constraintEqualToAnchor:container.leadingAnchor],
            [childView.trailingAnchor constraintEqualToAnchor:container.trailingAnchor],
            [childView.bottomAnchor constraintLessThanOrEqualToAnchor:container.bottomAnchor],
        ]];

        item.view = container;
    }
}
