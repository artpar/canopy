#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#import <objc/runtime.h>
#include "splitview.h"

static const void *kSplitViewDelegateKey = &kSplitViewDelegateKey;
static const void *kSplitViewInitializedKey = &kSplitViewInitializedKey;
static const void *kCollapsedPaneKey = &kCollapsedPaneKey;

@interface JVSplitViewDelegate : NSObject <NSSplitViewDelegate>
@end

@implementation JVSplitViewDelegate

- (BOOL)splitView:(NSSplitView *)splitView canCollapseSubview:(NSView *)subview {
    // Allow programmatic collapse of any pane
    NSNumber *collapsed = objc_getAssociatedObject(splitView, kCollapsedPaneKey);
    if (collapsed && [collapsed intValue] >= 0) {
        NSInteger idx = [splitView.subviews indexOfObject:subview];
        if (idx == (NSUInteger)[collapsed intValue]) return YES;
    }
    return NO;
}

- (CGFloat)splitView:(NSSplitView *)splitView constrainMinCoordinate:(CGFloat)proposedMinimumPosition ofSubviewAt:(NSInteger)dividerIndex {
    // Allow position 0 when collapsing pane at dividerIndex
    NSNumber *collapsed = objc_getAssociatedObject(splitView, kCollapsedPaneKey);
    if (collapsed && [collapsed intValue] == (int)dividerIndex) {
        return proposedMinimumPosition;
    }
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

static const void *kSavedDiv0Key = &kSavedDiv0Key;
static const void *kSavedDiv1Key = &kSavedDiv1Key;

void JVUpdateSplitView(void* handle, const char* dividerStyle, bool vertical, int collapsedPane) {
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

    // Handle pane collapse
    NSNumber *prevCollapsed = objc_getAssociatedObject(splitView, kCollapsedPaneKey);
    int prevPane = prevCollapsed ? [prevCollapsed intValue] : -1;
    NSInteger paneCount = splitView.subviews.count;
    if (collapsedPane != prevPane && paneCount > 0) {
        objc_setAssociatedObject(splitView, kCollapsedPaneKey, @(collapsedPane), OBJC_ASSOCIATION_RETAIN_NONATOMIC);

        if (collapsedPane == 0 && paneCount >= 2) {
            // Collapse first pane — save divider positions, shift remaining dividers
            CGFloat div0Pos = splitView.vertical
                ? NSMaxX(splitView.subviews[0].frame)
                : NSMaxY(splitView.subviews[0].frame);
            objc_setAssociatedObject(splitView, kSavedDiv0Key, @(div0Pos), OBJC_ASSOCIATION_RETAIN_NONATOMIC);

            if (paneCount >= 3) {
                CGFloat div1Pos = splitView.vertical
                    ? NSMaxX(splitView.subviews[1].frame)
                    : NSMaxY(splitView.subviews[1].frame);
                objc_setAssociatedObject(splitView, kSavedDiv1Key, @(div1Pos), OBJC_ASSOCIATION_RETAIN_NONATOMIC);
                CGFloat newDiv1 = div1Pos - div0Pos;

                [NSAnimationContext runAnimationGroup:^(NSAnimationContext *ctx) {
                    ctx.duration = 0.2;
                    ctx.timingFunction = [CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseInEaseOut];
                    [splitView.animator setPosition:0 ofDividerAtIndex:0];
                    [splitView.animator setPosition:newDiv1 ofDividerAtIndex:1];
                }];
            } else {
                [NSAnimationContext runAnimationGroup:^(NSAnimationContext *ctx) {
                    ctx.duration = 0.2;
                    ctx.timingFunction = [CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseInEaseOut];
                    [splitView.animator setPosition:0 ofDividerAtIndex:0];
                }];
            }
        } else if (collapsedPane < 0 && prevPane == 0) {
            // Restore first pane from saved positions
            NSNumber *savedDiv0 = objc_getAssociatedObject(splitView, kSavedDiv0Key);
            CGFloat div0Pos = savedDiv0 ? [savedDiv0 doubleValue] : 200;

            if (paneCount >= 3) {
                NSNumber *savedDiv1 = objc_getAssociatedObject(splitView, kSavedDiv1Key);
                CGFloat div1Pos = savedDiv1 ? [savedDiv1 doubleValue] : 450;

                [NSAnimationContext runAnimationGroup:^(NSAnimationContext *ctx) {
                    ctx.duration = 0.2;
                    ctx.timingFunction = [CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseInEaseOut];
                    [splitView.animator setPosition:div0Pos ofDividerAtIndex:0];
                    [splitView.animator setPosition:div1Pos ofDividerAtIndex:1];
                }];
            } else {
                [NSAnimationContext runAnimationGroup:^(NSAnimationContext *ctx) {
                    ctx.duration = 0.2;
                    ctx.timingFunction = [CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseInEaseOut];
                    [splitView.animator setPosition:div0Pos ofDividerAtIndex:0];
                }];
            }
        }
    }
}

void JVSplitViewSetChildren(void* handle, void** children, int count) {
    NSSplitView *splitView = (__bridge NSSplitView*)handle;

    // Skip if children are the same (prevents resetting divider positions on re-render)
    NSArray<NSView*> *existing = splitView.subviews;
    if ((int)existing.count == count) {
        BOOL same = YES;
        for (int i = 0; i < count; i++) {
            NSView *child = (__bridge NSView*)children[i];
            NSView *container = existing[i];
            if (container.subviews.count == 0 || container.subviews[0] != child) {
                same = NO;
                break;
            }
        }
        if (same) return;
    }

    // Remove existing subviews (containers from previous call)
    existing = [splitView.subviews copy];
    for (NSView *view in existing) {
        [view removeFromSuperview];
    }

    // Wrap each child in a frame-based container so NSSplitView can manage pane frames
    // while the child uses Auto Layout inside the container
    for (int i = 0; i < count; i++) {
        NSView *child = (__bridge NSView*)children[i];
        NSView *container;

        // First pane of a vertical split: use NSVisualEffectView for sidebar vibrancy
        if (i == 0 && splitView.vertical && count >= 2) {
            NSVisualEffectView *effectView = [[NSVisualEffectView alloc] init];
            effectView.material = NSVisualEffectMaterialSidebar;
            effectView.blendingMode = NSVisualEffectBlendingModeBehindWindow;
            effectView.state = NSVisualEffectStateFollowsWindowActiveState;
            container = effectView;
        } else {
            container = [[NSView alloc] init];
        }
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
