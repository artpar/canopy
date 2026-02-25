#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "splitview.h"

static const void *kSplitViewDelegateKey = &kSplitViewDelegateKey;

static const void *kSplitViewInitializedKey = &kSplitViewInitializedKey;

@interface JVSplitViewDelegate : NSObject <NSSplitViewDelegate>
@end

@implementation JVSplitViewDelegate

- (CGFloat)splitView:(NSSplitView *)splitView constrainMinCoordinate:(CGFloat)proposedMinimumPosition ofSubviewAt:(NSInteger)dividerIndex {
    return proposedMinimumPosition + 100;
}

- (CGFloat)splitView:(NSSplitView *)splitView constrainMaxCoordinate:(CGFloat)proposedMaximumPosition ofSubviewAt:(NSInteger)dividerIndex {
    return proposedMaximumPosition - 100;
}

- (void)splitView:(NSSplitView *)splitView resizeSubviewsWithOldSize:(NSSize)oldSize {
    NSArray<NSView*> *subs = splitView.subviews;
    NSInteger count = subs.count;
    if (count == 0) return;

    // Check if this is the first real layout (old size was zero)
    NSNumber *initialized = objc_getAssociatedObject(splitView, kSplitViewInitializedKey);
    BOOL isInitial = (initialized == nil) && (oldSize.width == 0 || oldSize.height == 0);

    if (isInitial) {
        // First layout: use preferred widths from children, distribute remaining space equally
        objc_setAssociatedObject(splitView, kSplitViewInitializedKey, @YES, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
        CGFloat totalSize = splitView.vertical ? splitView.bounds.size.width : splitView.bounds.size.height;
        CGFloat crossSize = splitView.vertical ? splitView.bounds.size.height : splitView.bounds.size.width;
        CGFloat dividerThickness = splitView.dividerThickness;
        CGFloat available = totalSize - (dividerThickness * (count - 1));

        // Read preferred sizes from children's width/height constraints
        CGFloat *preferred = calloc(count, sizeof(CGFloat));
        NSInteger flexCount = 0;
        CGFloat fixedTotal = 0;
        NSLayoutAttribute sizeAttr = splitView.vertical ? NSLayoutAttributeWidth : NSLayoutAttributeHeight;

        for (NSInteger i = 0; i < count; i++) {
            preferred[i] = -1; // -1 means no preference (flex)
            NSView *container = subs[i];
            NSView *child = container.subviews.firstObject;
            if (child) {
                for (NSLayoutConstraint *c in child.constraints) {
                    if (c.firstAttribute == sizeAttr && c.secondItem == nil && c.relation == NSLayoutRelationEqual) {
                        preferred[i] = c.constant;
                        fixedTotal += c.constant;
                        break;
                    }
                }
            }
            if (preferred[i] < 0) flexCount++;
        }

        CGFloat flexSize = flexCount > 0 ? (available - fixedTotal) / flexCount : 0;
        if (flexSize < 100) flexSize = 100;

        CGFloat offset = 0;
        for (NSInteger i = 0; i < count; i++) {
            CGFloat w = preferred[i] >= 0 ? preferred[i] : flexSize;
            if (i == count - 1) w = totalSize - offset; // last pane gets remainder
            if (splitView.vertical) {
                subs[i].frame = NSMakeRect(offset, 0, w, crossSize);
            } else {
                subs[i].frame = NSMakeRect(0, offset, crossSize, w);
            }
            offset += w + dividerThickness;
        }
        free(preferred);
    } else {
        // Subsequent resizes: let NSSplitView handle proportionally
        [splitView adjustSubviews];
    }
}

@end

void* JVCreateSplitView(const char* dividerStyle, bool vertical) {
    NSSplitView *splitView = [[NSSplitView alloc] init];
    splitView.translatesAutoresizingMaskIntoConstraints = NO;
    splitView.vertical = vertical;

    NSString *styleStr = [NSString stringWithUTF8String:dividerStyle];
    if ([styleStr isEqualToString:@"thick"]) {
        splitView.dividerStyle = NSSplitViewDividerStyleThick;
    } else if ([styleStr isEqualToString:@"paneSplitter"]) {
        splitView.dividerStyle = NSSplitViewDividerStylePaneSplitter;
    } else {
        splitView.dividerStyle = NSSplitViewDividerStyleThin;
    }

    JVSplitViewDelegate *delegate = [[JVSplitViewDelegate alloc] init];
    splitView.delegate = delegate;
    objc_setAssociatedObject(splitView, kSplitViewDelegateKey, delegate, OBJC_ASSOCIATION_RETAIN_NONATOMIC);

    return (__bridge_retained void*)splitView;
}

void JVUpdateSplitView(void* handle, const char* dividerStyle, bool vertical) {
    NSSplitView *splitView = (__bridge NSSplitView*)handle;
    splitView.vertical = vertical;

    NSString *styleStr = [NSString stringWithUTF8String:dividerStyle];
    if ([styleStr isEqualToString:@"thick"]) {
        splitView.dividerStyle = NSSplitViewDividerStyleThick;
    } else if ([styleStr isEqualToString:@"paneSplitter"]) {
        splitView.dividerStyle = NSSplitViewDividerStylePaneSplitter;
    } else {
        splitView.dividerStyle = NSSplitViewDividerStyleThin;
    }
}

void JVSplitViewSetChildren(void* handle, void** children, int count) {
    NSSplitView *splitView = (__bridge NSSplitView*)handle;

    // Remove existing subviews (containers from previous call)
    NSArray<NSView*> *existing = [splitView.subviews copy];
    for (NSView *view in existing) {
        [view removeFromSuperview];
    }

    // Wrap each child in a frame-based container so NSSplitView can manage pane frames
    // while the child uses Auto Layout inside the container
    for (int i = 0; i < count; i++) {
        NSView *child = (__bridge NSView*)children[i];
        NSView *container = [[NSView alloc] init];
        container.translatesAutoresizingMaskIntoConstraints = YES;
        container.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;

        child.translatesAutoresizingMaskIntoConstraints = NO;
        [container addSubview:child];

        // Pin child to container edges
        [child.topAnchor constraintEqualToAnchor:container.topAnchor].active = YES;
        [child.bottomAnchor constraintEqualToAnchor:container.bottomAnchor].active = YES;
        [child.leadingAnchor constraintEqualToAnchor:container.leadingAnchor].active = YES;
        [child.trailingAnchor constraintEqualToAnchor:container.trailingAnchor].active = YES;

        // Lower priority of any width/height constraints from style so they act as
        // preferred pane sizes rather than fighting with container pinning
        NSLayoutAttribute sizeAttr = splitView.vertical ? NSLayoutAttributeWidth : NSLayoutAttributeHeight;
        for (NSLayoutConstraint *c in [child.constraints copy]) {
            if (c.firstAttribute == sizeAttr && c.secondItem == nil && c.relation == NSLayoutRelationEqual) {
                c.priority = NSLayoutPriorityDefaultLow; // 250 — preference, not requirement
            }
        }

        [splitView addSubview:container];
    }

    // Set equal holding priorities
    for (int i = 0; i < count; i++) {
        [splitView setHoldingPriority:250 forSubviewAtIndex:i];
    }
}
