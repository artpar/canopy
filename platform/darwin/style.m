#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "style.h"

static NSColor* colorFromHex(NSString *hex) {
    if ([hex length] < 7 || [hex characterAtIndex:0] != '#') return nil;

    unsigned int r = 0, g = 0, b = 0;
    NSScanner *scanner;

    scanner = [NSScanner scannerWithString:[hex substringWithRange:NSMakeRange(1, 2)]];
    [scanner scanHexInt:&r];
    scanner = [NSScanner scannerWithString:[hex substringWithRange:NSMakeRange(3, 2)]];
    [scanner scanHexInt:&g];
    scanner = [NSScanner scannerWithString:[hex substringWithRange:NSMakeRange(5, 2)]];
    [scanner scanHexInt:&b];

    return [NSColor colorWithSRGBRed:r/255.0 green:g/255.0 blue:b/255.0 alpha:1.0];
}

static NSFontWeight fontWeightFromString(NSString *weight) {
    if ([weight isEqualToString:@"bold"]) return NSFontWeightBold;
    if ([weight isEqualToString:@"medium"]) return NSFontWeightMedium;
    if ([weight isEqualToString:@"light"]) return NSFontWeightLight;
    if ([weight isEqualToString:@"semibold"]) return NSFontWeightSemibold;
    return NSFontWeightRegular;
}

static NSTextAlignment textAlignFromString(NSString *align) {
    if ([align isEqualToString:@"center"]) return NSTextAlignmentCenter;
    if ([align isEqualToString:@"right"]) return NSTextAlignmentRight;
    return NSTextAlignmentLeft;
}

void JVApplyStyle(void* handle, const char* bg, const char* tc,
    double cornerRadius, double width, double height,
    double fontSize, const char* fontWeight, const char* textAlign, double opacity,
    const char* fontFamily, double flexGrow) {

    NSView *view = (__bridge NSView*)handle;
    NSString *bgStr = [NSString stringWithUTF8String:bg];
    NSString *tcStr = [NSString stringWithUTF8String:tc];
    NSString *fwStr = [NSString stringWithUTF8String:fontWeight];
    NSString *taStr = [NSString stringWithUTF8String:textAlign];
    NSString *ffStr = fontFamily ? [NSString stringWithUTF8String:fontFamily] : @"";

    // Background color via layer
    if ([bgStr length] > 0) {
        NSColor *color = colorFromHex(bgStr);
        if (color) {
            view.wantsLayer = YES;

            // For NSButton: switch to borderless so layer bg shows
            if ([view isKindOfClass:[NSButton class]]) {
                NSButton *btn = (NSButton*)view;
                btn.bordered = NO;
                btn.bezelStyle = NSBezelStyleSmallSquare;
            }

            view.layer.backgroundColor = [color CGColor];
        }
    } else if (view.wantsLayer && view.layer.backgroundColor != NULL) {
        view.layer.backgroundColor = NULL;
    }

    // Corner radius
    if (cornerRadius > 0) {
        view.wantsLayer = YES;
        view.layer.cornerRadius = cornerRadius;
        view.layer.masksToBounds = YES;
    }

    // Width constraint
    if (width > 0) {
        [view.widthAnchor constraintEqualToConstant:width].active = YES;
    }

    // Height constraint
    if (height > 0) {
        [view.heightAnchor constraintEqualToConstant:height].active = YES;
    }

    // Opacity
    if (opacity > 0) {
        view.alphaValue = opacity;
    }

    // Text-specific styling (NSTextField used for Text components)
    if ([view isKindOfClass:[NSTextField class]]) {
        NSTextField *tf = (NSTextField*)view;

        // Text color
        if ([tcStr length] > 0) {
            NSColor *color = colorFromHex(tcStr);
            if (color) tf.textColor = color;
        } else {
            tf.textColor = [NSColor labelColor];
        }

        // Font size, weight, and family
        if (fontSize > 0 || [fwStr length] > 0 || [ffStr length] > 0) {
            CGFloat size = fontSize > 0 ? fontSize : tf.font.pointSize;
            if ([ffStr length] > 0) {
                NSFont *customFont = [NSFont fontWithName:ffStr size:size];
                if (customFont) {
                    tf.font = customFont;
                } else {
                    // Fall back to system font
                    NSFontWeight weight = [fwStr length] > 0 ? fontWeightFromString(fwStr) : NSFontWeightRegular;
                    tf.font = [NSFont systemFontOfSize:size weight:weight];
                }
            } else {
                NSFontWeight weight = [fwStr length] > 0 ? fontWeightFromString(fwStr) : NSFontWeightRegular;
                // Preserve bold from variant if fontWeight not set
                if ([fwStr length] == 0) {
                    NSFontDescriptor *desc = [tf.font fontDescriptor];
                    NSFontDescriptorSymbolicTraits traits = [desc symbolicTraits];
                    if (traits & NSFontDescriptorTraitBold) {
                        weight = NSFontWeightBold;
                    }
                }
                tf.font = [NSFont systemFontOfSize:size weight:weight];
            }
        }

        // Text alignment
        if ([taStr length] > 0) {
            tf.alignment = textAlignFromString(taStr);
        }
    }

    // Button-specific styling
    if ([view isKindOfClass:[NSButton class]]) {
        NSButton *btn = (NSButton*)view;

        // Text color on button
        if ([tcStr length] > 0) {
            NSColor *color = colorFromHex(tcStr);
            if (color) btn.contentTintColor = color;
        }

        // Font size, weight, and family on button
        if (fontSize > 0 || [fwStr length] > 0 || [ffStr length] > 0) {
            CGFloat size = fontSize > 0 ? fontSize : btn.font.pointSize;
            if ([ffStr length] > 0) {
                NSFont *customFont = [NSFont fontWithName:ffStr size:size];
                if (customFont) {
                    btn.font = customFont;
                } else {
                    NSFontWeight weight = [fwStr length] > 0 ? fontWeightFromString(fwStr) : NSFontWeightRegular;
                    btn.font = [NSFont systemFontOfSize:size weight:weight];
                }
            } else {
                NSFontWeight weight = [fwStr length] > 0 ? fontWeightFromString(fwStr) : NSFontWeightRegular;
                btn.font = [NSFont systemFontOfSize:size weight:weight];
            }
        }
    }

    // flexGrow: store on view as associated object; applied during SetChildren
    if (flexGrow > 0) {
        extern const void *kJVFlexGrowKey;
        objc_setAssociatedObject(view, kJVFlexGrowKey, @(flexGrow), OBJC_ASSOCIATION_RETAIN_NONATOMIC);
        [view setContentHuggingPriority:1 forOrientation:NSLayoutConstraintOrientationHorizontal];
        [view setContentHuggingPriority:1 forOrientation:NSLayoutConstraintOrientationVertical];
    }
}
