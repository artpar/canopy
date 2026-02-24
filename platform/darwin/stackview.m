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

// Shared key for flexGrow associated object (also used by style.m)
const void *kJVFlexGrowKey = &kJVFlexGrowKey;

// Pin a child on the cross-axis based on alignment.
static void pinCrossAxis(NSView *child, NSStackView *stack, BOOL stretch,
                         bool horizontal) {
    NSEdgeInsets insets = stack.edgeInsets;
    if (horizontal) {
        if (stretch) {
            [child.topAnchor constraintEqualToAnchor:stack.topAnchor constant:insets.top].active = YES;
            [child.bottomAnchor constraintEqualToAnchor:stack.bottomAnchor constant:-insets.bottom].active = YES;
        } else if (stack.alignment == NSLayoutAttributeTop) {
            [child.topAnchor constraintEqualToAnchor:stack.topAnchor constant:insets.top].active = YES;
            [child.bottomAnchor constraintLessThanOrEqualToAnchor:stack.bottomAnchor constant:-insets.bottom].active = YES;
        } else if (stack.alignment == NSLayoutAttributeBottom) {
            [child.bottomAnchor constraintEqualToAnchor:stack.bottomAnchor constant:-insets.bottom].active = YES;
            [child.topAnchor constraintGreaterThanOrEqualToAnchor:stack.topAnchor constant:insets.top].active = YES;
        } else {
            // center (default)
            [child.centerYAnchor constraintEqualToAnchor:stack.centerYAnchor].active = YES;
            [child.topAnchor constraintGreaterThanOrEqualToAnchor:stack.topAnchor constant:insets.top].active = YES;
            [child.bottomAnchor constraintLessThanOrEqualToAnchor:stack.bottomAnchor constant:-insets.bottom].active = YES;
        }
    } else {
        if (stretch) {
            [child.leadingAnchor constraintEqualToAnchor:stack.leadingAnchor constant:insets.left].active = YES;
            [child.trailingAnchor constraintEqualToAnchor:stack.trailingAnchor constant:-insets.right].active = YES;
        } else if (stack.alignment == NSLayoutAttributeLeading) {
            [child.leadingAnchor constraintEqualToAnchor:stack.leadingAnchor constant:insets.left].active = YES;
            [child.trailingAnchor constraintLessThanOrEqualToAnchor:stack.trailingAnchor constant:-insets.right].active = YES;
        } else if (stack.alignment == NSLayoutAttributeTrailing) {
            [child.trailingAnchor constraintEqualToAnchor:stack.trailingAnchor constant:-insets.right].active = YES;
            [child.leadingAnchor constraintGreaterThanOrEqualToAnchor:stack.leadingAnchor constant:insets.left].active = YES;
        } else {
            // center
            [child.centerXAnchor constraintEqualToAnchor:stack.centerXAnchor].active = YES;
            [child.leadingAnchor constraintGreaterThanOrEqualToAnchor:stack.leadingAnchor constant:insets.left].active = YES;
            [child.trailingAnchor constraintLessThanOrEqualToAnchor:stack.trailingAnchor constant:-insets.right].active = YES;
        }
    }
}

void JVStackViewSetChildren(void* handle, void** children, int count) {
    NSStackView *stack = (__bridge NSStackView*)handle;
    BOOL stretch = [objc_getAssociatedObject(stack, kStretchModeKey) boolValue];
    bool vertical = (stack.orientation == NSUserInterfaceLayoutOrientationVertical);
    bool horizontal = !vertical;

    // Remove all existing views (arranged + regular subviews)
    NSArray<NSView*> *existing = [stack.arrangedSubviews copy];
    for (NSView *v in existing) {
        [stack removeArrangedSubview:v];
        [v removeFromSuperview];
    }
    for (NSView *v in [stack.subviews copy]) {
        [v removeFromSuperview];
    }

    // Check if any child has flexGrow
    BOOL hasFlex = NO;
    for (int i = 0; i < count; i++) {
        NSView *child = (__bridge NSView*)children[i];
        NSNumber *fg = objc_getAssociatedObject(child, kJVFlexGrowKey);
        if (fg && [fg doubleValue] > 0) {
            hasFlex = YES;
        }
    }

    if (hasFlex) {
        // Manual layout: bypass NSStackView distribution entirely.
        // Add as regular subviews and chain with explicit constraints.
        NSEdgeInsets insets = stack.edgeInsets;
        CGFloat spacing = stack.spacing;

        for (int i = 0; i < count; i++) {
            NSView *child = (__bridge NSView*)children[i];
            child.translatesAutoresizingMaskIntoConstraints = NO;
            [stack addSubview:child];
        }

        NSView *prevChild = nil;
        for (int i = 0; i < count; i++) {
            NSView *child = (__bridge NSView*)children[i];
            NSNumber *fg = objc_getAssociatedObject(child, kJVFlexGrowKey);
            BOOL isFlex = fg && [fg doubleValue] > 0;

            // Main axis chaining
            if (horizontal) {
                if (prevChild) {
                    [child.leadingAnchor constraintEqualToAnchor:prevChild.trailingAnchor constant:spacing].active = YES;
                } else {
                    [child.leadingAnchor constraintEqualToAnchor:stack.leadingAnchor constant:insets.left].active = YES;
                }
            } else {
                if (prevChild) {
                    [child.topAnchor constraintEqualToAnchor:prevChild.bottomAnchor constant:spacing].active = YES;
                } else {
                    [child.topAnchor constraintEqualToAnchor:stack.topAnchor constant:insets.top].active = YES;
                }
            }

            // Cross-axis alignment
            pinCrossAxis(child, stack, stretch, horizontal);

            // Flex vs non-flex sizing on main axis
            NSLayoutConstraintOrientation mainAxis = horizontal
                ? NSLayoutConstraintOrientationHorizontal
                : NSLayoutConstraintOrientationVertical;
            if (isFlex) {
                [child setContentHuggingPriority:1 forOrientation:mainAxis];
                [child setContentCompressionResistancePriority:250 forOrientation:mainAxis];
            } else {
                [child setContentHuggingPriority:750 forOrientation:mainAxis];
                [child setContentCompressionResistancePriority:750 forOrientation:mainAxis];
            }

            prevChild = child;
        }

        // Pin last child to trailing/bottom edge
        if (prevChild) {
            if (horizontal) {
                [prevChild.trailingAnchor constraintEqualToAnchor:stack.trailingAnchor constant:-insets.right].active = YES;
            } else {
                [prevChild.bottomAnchor constraintEqualToAnchor:stack.bottomAnchor constant:-insets.bottom].active = YES;
            }
        }

    } else {
        // Standard NSStackView layout (no flex children)
        for (int i = 0; i < count; i++) {
            NSView *child = (__bridge NSView*)children[i];
            child.translatesAutoresizingMaskIntoConstraints = NO;
            [stack addArrangedSubview:child];

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
}
