#import <Cocoa/Cocoa.h>
#include "viewquery.h"

// Helper: convert NSColor to hex string "#RRGGBB".
// Returns a strdup'd C string (caller must free) or empty string.
static const char* colorToHex(NSColor *color) {
    if (!color) return strdup("");

    // Convert to sRGB space
    NSColor *rgb = [color colorUsingColorSpace:[NSColorSpace sRGBColorSpace]];
    if (!rgb) return strdup("");

    CGFloat r, g, b, a;
    [rgb getRed:&r green:&g blue:&b alpha:&a];

    char buf[16];
    snprintf(buf, sizeof(buf), "#%02X%02X%02X",
             (int)(r * 255.0), (int)(g * 255.0), (int)(b * 255.0));
    return strdup(buf);
}

JVViewFrame JVGetViewFrame(void* handle) {
    JVViewFrame frame = {0, 0, 0, 0};
    if (!handle) return frame;

    NSView *view = (__bridge NSView*)handle;

    // Get frame in window coordinates for consistent cross-view positioning
    if (view.window) {
        NSRect windowRect = [view convertRect:view.bounds toView:nil];
        frame.x = windowRect.origin.x;
        frame.y = windowRect.origin.y;
        frame.width = windowRect.size.width;
        frame.height = windowRect.size.height;
    } else {
        // No window yet — use local frame
        NSRect localFrame = view.frame;
        frame.x = localFrame.origin.x;
        frame.y = localFrame.origin.y;
        frame.width = localFrame.size.width;
        frame.height = localFrame.size.height;
    }

    return frame;
}

JVViewStyle JVGetViewStyle(void* handle) {
    JVViewStyle style = {
        .fontName = strdup(""),
        .fontSize = 0,
        .bold = 0,
        .italic = 0,
        .textColor = strdup(""),
        .bgColor = strdup(""),
        .hidden = 0,
        .opacity = 1.0
    };

    if (!handle) return style;

    NSView *view = (__bridge NSView*)handle;
    style.hidden = view.isHidden ? 1 : 0;
    style.opacity = view.alphaValue;

    // Extract font from text-bearing views
    NSFont *font = nil;
    NSColor *textColor = nil;

    if ([view isKindOfClass:[NSTextField class]]) {
        NSTextField *tf = (NSTextField *)view;
        font = tf.font;
        textColor = tf.textColor;
    } else if ([view isKindOfClass:[NSButton class]]) {
        NSButton *btn = (NSButton *)view;
        font = btn.font;
        textColor = btn.contentTintColor;
    }

    if (font) {
        free((void*)style.fontName);
        style.fontName = strdup(font.fontName.UTF8String);
        style.fontSize = font.pointSize;

        NSFontDescriptor *desc = font.fontDescriptor;
        NSFontDescriptorSymbolicTraits traits = desc.symbolicTraits;
        style.bold = (traits & NSFontDescriptorTraitBold) ? 1 : 0;
        style.italic = (traits & NSFontDescriptorTraitItalic) ? 1 : 0;
    }

    if (textColor) {
        free((void*)style.textColor);
        style.textColor = colorToHex(textColor);
    }

    // Background color
    if (view.layer) {
        CGColorRef bg = view.layer.backgroundColor;
        if (bg) {
            NSColor *bgColor = [NSColor colorWithCGColor:bg];
            free((void*)style.bgColor);
            style.bgColor = colorToHex(bgColor);
        }
    }

    return style;
}

void JVFreeViewStyle(JVViewStyle style) {
    if (style.fontName) free((void*)style.fontName);
    if (style.textColor) free((void*)style.textColor);
    if (style.bgColor) free((void*)style.bgColor);
}
