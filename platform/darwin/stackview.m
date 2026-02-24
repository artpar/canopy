#import <Cocoa/Cocoa.h>
#include "stackview.h"
#import <objc/runtime.h>

static const void *kStretchModeKey = &kStretchModeKey;

static NSLayoutAttribute alignmentForAlign(NSString *align, bool horizontal) {
    if ([align isEqualToString:@"start"]) {
        return horizontal ? NSLayoutAttributeTop : NSLayoutAttributeLeading;
    }
    if ([align isEqualToString:@"end"]) {
        return horizontal ? NSLayoutAttributeBottom : NSLayoutAttributeTrailing;
    }
    if ([align isEqualToString:@"center"]) {
        return horizontal ? NSLayoutAttributeCenterY : NSLayoutAttributeCenterX;
    }
    // Default: stretch (handled in applyAlignment)
    return 0;
}

static void applyDistribution(NSStackView *stack, NSString *justify) {
    if ([justify isEqualToString:@"spaceBetween"]) {
        stack.distribution = NSStackViewDistributionEqualSpacing;
    } else if ([justify isEqualToString:@"spaceAround"]) {
        stack.distribution = NSStackViewDistributionEqualSpacing;
    } else if ([justify isEqualToString:@"center"]) {
        stack.distribution = NSStackViewDistributionEqualSpacing;
    } else if ([justify isEqualToString:@"fillEqually"]) {
        stack.distribution = NSStackViewDistributionFillEqually;
    } else {
        stack.distribution = NSStackViewDistributionFill;
    }
}

static void applyAlignment(NSStackView *stack, NSString *alignStr, bool horizontal) {
    bool isStretch = [alignStr isEqualToString:@"stretch"] || [alignStr length] == 0;
    if (isStretch) {
        // Use leading alignment and pin children in SetChildren
        stack.alignment = horizontal ? NSLayoutAttributeTop : NSLayoutAttributeLeading;
        objc_setAssociatedObject(stack, kStretchModeKey, @YES, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    } else {
        stack.alignment = alignmentForAlign(alignStr, horizontal);
        objc_setAssociatedObject(stack, kStretchModeKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    }
}

void* JVCreateStackView(bool horizontal, const char* justify, const char* align, int gap, int padding) {
    NSStackView *stack = [[NSStackView alloc] init];
    stack.orientation = horizontal ? NSUserInterfaceLayoutOrientationHorizontal
                                   : NSUserInterfaceLayoutOrientationVertical;
    stack.spacing = gap;
    stack.edgeInsets = NSEdgeInsetsMake(padding, padding, padding, padding);
    stack.translatesAutoresizingMaskIntoConstraints = NO;

    NSString *justifyStr = [NSString stringWithUTF8String:justify];
    NSString *alignStr = [NSString stringWithUTF8String:align];

    applyDistribution(stack, justifyStr);
    applyAlignment(stack, alignStr, horizontal);

    return (__bridge_retained void*)stack;
}

void JVUpdateStackView(void* handle, const char* justify, const char* align, int gap, int padding) {
    NSStackView *stack = (__bridge NSStackView*)handle;
    stack.spacing = gap;
    stack.edgeInsets = NSEdgeInsetsMake(padding, padding, padding, padding);

    NSString *justifyStr = [NSString stringWithUTF8String:justify];
    NSString *alignStr = [NSString stringWithUTF8String:align];

    applyDistribution(stack, justifyStr);
    bool horizontal = (stack.orientation == NSUserInterfaceLayoutOrientationHorizontal);
    applyAlignment(stack, alignStr, horizontal);
}

void JVStackViewSetChildren(void* handle, void** children, int count) {
    NSStackView *stack = (__bridge NSStackView*)handle;
    BOOL stretch = [objc_getAssociatedObject(stack, kStretchModeKey) boolValue];
    bool vertical = (stack.orientation == NSUserInterfaceLayoutOrientationVertical);

    // Remove all existing arranged subviews
    NSArray<NSView*> *existing = [stack.arrangedSubviews copy];
    for (NSView *v in existing) {
        [stack removeArrangedSubview:v];
        [v removeFromSuperview];
    }

    // Add new children
    for (int i = 0; i < count; i++) {
        NSView *child = (__bridge NSView*)children[i];
        child.translatesAutoresizingMaskIntoConstraints = NO;
        [stack addArrangedSubview:child];

        // In stretch mode, pin children to fill the cross-axis
        if (stretch) {
            if (vertical) {
                [child.leadingAnchor constraintEqualToAnchor:stack.leadingAnchor constant:stack.edgeInsets.left].active = YES;
                [child.trailingAnchor constraintEqualToAnchor:stack.trailingAnchor constant:-stack.edgeInsets.right].active = YES;
            } else {
                [child.topAnchor constraintEqualToAnchor:stack.topAnchor constant:stack.edgeInsets.top].active = YES;
                [child.bottomAnchor constraintEqualToAnchor:stack.bottomAnchor constant:-stack.edgeInsets.bottom].active = YES;
            }
        }
    }
}
