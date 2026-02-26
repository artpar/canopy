#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "richtexteditor.h"

extern void GoCallbackInvoke(uint64_t callbackID, const char* data);

static const void *kRTEDelegateKey = &kRTEDelegateKey;
static const void *kRTETextViewKey = &kRTETextViewKey;

// --- Markdown → NSAttributedString ---

static NSFont* systemFontRegular(CGFloat size) {
    return [NSFont systemFontOfSize:size weight:NSFontWeightRegular];
}
static NSFont* systemFontBold(CGFloat size) {
    return [NSFont systemFontOfSize:size weight:NSFontWeightBold];
}
static NSFont* systemFontSemibold(CGFloat size) {
    return [NSFont systemFontOfSize:size weight:NSFontWeightSemibold];
}
static NSFont* systemFontMedium(CGFloat size) {
    return [NSFont systemFontOfSize:size weight:NSFontWeightMedium];
}
static NSFont* systemFontMono(CGFloat size) {
    return [NSFont monospacedSystemFontOfSize:size weight:NSFontWeightRegular];
}

static void applyInlineStyles(NSMutableAttributedString *line, NSRange range, NSFont *baseFont) {
    NSString *text = [line.string substringWithRange:range];

    // Bold: **text**
    NSRegularExpression *boldRegex = [NSRegularExpression regularExpressionWithPattern:@"\\*\\*(.+?)\\*\\*" options:0 error:nil];
    NSArray *boldMatches = [boldRegex matchesInString:text options:0 range:NSMakeRange(0, text.length)];
    for (NSTextCheckingResult *match in [boldMatches reverseObjectEnumerator]) {
        NSRange fullRange = NSMakeRange(range.location + match.range.location, match.range.length);
        NSRange contentRange = NSMakeRange(range.location + [match rangeAtIndex:1].location, [match rangeAtIndex:1].length);
        NSString *content = [line.string substringWithRange:contentRange];

        NSFont *boldFont = [[NSFontManager sharedFontManager] convertFont:baseFont toHaveTrait:NSBoldFontMask];
        NSDictionary *attrs = @{NSFontAttributeName: boldFont};

        [line replaceCharactersInRange:fullRange withString:content];
        [line addAttributes:attrs range:NSMakeRange(fullRange.location, content.length)];
    }

    // Re-fetch text after modifications
    text = [line.string substringWithRange:NSMakeRange(range.location, MIN(range.length, line.length - range.location))];
    NSUInteger newLen = MIN(range.length, line.length - range.location);

    // Italic: *text*
    NSRegularExpression *italicRegex = [NSRegularExpression regularExpressionWithPattern:@"(?<!\\*)\\*(?!\\*)(.+?)(?<!\\*)\\*(?!\\*)" options:0 error:nil];
    NSArray *italicMatches = [italicRegex matchesInString:text options:0 range:NSMakeRange(0, newLen)];
    for (NSTextCheckingResult *match in [italicMatches reverseObjectEnumerator]) {
        NSRange fullRange = NSMakeRange(range.location + match.range.location, match.range.length);
        NSRange contentRange = NSMakeRange(range.location + [match rangeAtIndex:1].location, [match rangeAtIndex:1].length);
        NSString *content = [line.string substringWithRange:contentRange];

        NSFont *italicFont = [[NSFontManager sharedFontManager] convertFont:baseFont toHaveTrait:NSItalicFontMask];
        NSDictionary *attrs = @{NSFontAttributeName: italicFont};

        [line replaceCharactersInRange:fullRange withString:content];
        [line addAttributes:attrs range:NSMakeRange(fullRange.location, content.length)];
    }

    // Strikethrough: ~~text~~
    text = [line.string substringWithRange:NSMakeRange(range.location, MIN(range.length, line.length - range.location))];
    newLen = MIN(range.length, line.length - range.location);

    NSRegularExpression *strikeRegex = [NSRegularExpression regularExpressionWithPattern:@"~~(.+?)~~" options:0 error:nil];
    NSArray *strikeMatches = [strikeRegex matchesInString:text options:0 range:NSMakeRange(0, newLen)];
    for (NSTextCheckingResult *match in [strikeMatches reverseObjectEnumerator]) {
        NSRange fullRange = NSMakeRange(range.location + match.range.location, match.range.length);
        NSRange contentRange = NSMakeRange(range.location + [match rangeAtIndex:1].location, [match rangeAtIndex:1].length);
        NSString *content = [line.string substringWithRange:contentRange];

        [line replaceCharactersInRange:fullRange withString:content];
        [line addAttribute:NSStrikethroughStyleAttributeName value:@(NSUnderlineStyleSingle) range:NSMakeRange(fullRange.location, content.length)];
    }

    // Monospace: `text`
    text = [line.string substringWithRange:NSMakeRange(range.location, MIN(range.length, line.length - range.location))];
    newLen = MIN(range.length, line.length - range.location);

    NSRegularExpression *codeRegex = [NSRegularExpression regularExpressionWithPattern:@"`(.+?)`" options:0 error:nil];
    NSArray *codeMatches = [codeRegex matchesInString:text options:0 range:NSMakeRange(0, newLen)];
    for (NSTextCheckingResult *match in [codeMatches reverseObjectEnumerator]) {
        NSRange fullRange = NSMakeRange(range.location + match.range.location, match.range.length);
        NSRange contentRange = NSMakeRange(range.location + [match rangeAtIndex:1].location, [match rangeAtIndex:1].length);
        NSString *content = [line.string substringWithRange:contentRange];

        CGFloat fontSize = baseFont.pointSize;
        NSFont *monoFont = systemFontMono(fontSize);
        NSDictionary *attrs = @{
            NSFontAttributeName: monoFont,
            NSBackgroundColorAttributeName: [NSColor colorWithWhite:0.9 alpha:1.0]
        };

        [line replaceCharactersInRange:fullRange withString:content];
        [line addAttributes:attrs range:NSMakeRange(fullRange.location, content.length)];
    }
}

static NSAttributedString* markdownToAttributedString(NSString *markdown) {
    NSMutableAttributedString *result = [[NSMutableAttributedString alloc] init];
    if (!markdown || markdown.length == 0) return result;

    NSArray<NSString*> *lines = [markdown componentsSeparatedByString:@"\n"];
    NSColor *textColor = [NSColor textColor];

    for (NSUInteger i = 0; i < lines.count; i++) {
        NSString *line = lines[i];

        NSFont *font = systemFontRegular(14);
        NSMutableParagraphStyle *paraStyle = [[NSMutableParagraphStyle alloc] init];
        paraStyle.paragraphSpacing = 4;
        NSString *displayText = line;

        // Headings
        if ([line hasPrefix:@"### "]) {
            font = systemFontMedium(18);
            displayText = [line substringFromIndex:4];
            paraStyle.paragraphSpacingBefore = 8;
        } else if ([line hasPrefix:@"## "]) {
            font = systemFontSemibold(22);
            displayText = [line substringFromIndex:3];
            paraStyle.paragraphSpacingBefore = 12;
        } else if ([line hasPrefix:@"# "]) {
            font = systemFontBold(28);
            displayText = [line substringFromIndex:2];
            paraStyle.paragraphSpacingBefore = 16;
        }
        // Checklist
        else if ([line hasPrefix:@"- [x] "] || [line hasPrefix:@"- [X] "]) {
            displayText = [NSString stringWithFormat:@"\u2611 %@", [line substringFromIndex:6]];
            paraStyle.headIndent = 24;
            paraStyle.firstLineHeadIndent = 8;
        } else if ([line hasPrefix:@"- [ ] "]) {
            displayText = [NSString stringWithFormat:@"\u2610 %@", [line substringFromIndex:6]];
            paraStyle.headIndent = 24;
            paraStyle.firstLineHeadIndent = 8;
        }
        // Bullets
        else if ([line hasPrefix:@"- "]) {
            displayText = [NSString stringWithFormat:@"\u2022 %@", [line substringFromIndex:2]];
            paraStyle.headIndent = 24;
            paraStyle.firstLineHeadIndent = 8;
        }
        // Numbered list
        else if (line.length >= 3) {
            NSRegularExpression *numRegex = [NSRegularExpression regularExpressionWithPattern:@"^(\\d+)\\. " options:0 error:nil];
            NSTextCheckingResult *numMatch = [numRegex firstMatchInString:line options:0 range:NSMakeRange(0, line.length)];
            if (numMatch) {
                NSString *num = [line substringWithRange:[numMatch rangeAtIndex:1]];
                NSString *rest = [line substringFromIndex:[numMatch range].length];
                displayText = [NSString stringWithFormat:@"%@. %@", num, rest];
                paraStyle.headIndent = 24;
                paraStyle.firstLineHeadIndent = 8;
            }
        }

        // Add newline between lines (except last)
        if (i < lines.count - 1) {
            displayText = [displayText stringByAppendingString:@"\n"];
        }

        NSDictionary *attrs = @{
            NSFontAttributeName: font,
            NSForegroundColorAttributeName: textColor,
            NSParagraphStyleAttributeName: paraStyle,
        };

        NSMutableAttributedString *attrLine = [[NSMutableAttributedString alloc] initWithString:displayText attributes:attrs];
        applyInlineStyles(attrLine, NSMakeRange(0, attrLine.length), font);
        [result appendAttributedString:attrLine];
    }

    return result;
}

// --- NSAttributedString → Markdown ---

// Convert a line's inline content back to markdown by examining font traits and attributes.
// `lineRange` is the range in attrStr for the content portion (after checkbox/bullet prefix).
// `isHeading` suppresses bold detection (headings are inherently bold).
static NSString* inlineContentToMarkdown(NSAttributedString *attrStr, NSRange lineRange, BOOL isHeading) {
    if (lineRange.length == 0) return @"";

    NSFontManager *fm = [NSFontManager sharedFontManager];
    NSMutableString *content = [NSMutableString string];
    NSUInteger pos = lineRange.location;
    NSUInteger end = NSMaxRange(lineRange);

    while (pos < end) {
        NSRange effectiveRange;
        NSDictionary *attrs = [attrStr attributesAtIndex:pos effectiveRange:&effectiveRange];

        // Clamp to line range
        NSUInteger runEnd = MIN(NSMaxRange(effectiveRange), end);
        NSRange runRange = NSMakeRange(pos, runEnd - pos);
        NSString *runText = [attrStr.string substringWithRange:runRange];

        NSFont *font = attrs[NSFontAttributeName] ?: [NSFont systemFontOfSize:14];
        NSFontTraitMask traits = [fm traitsOfFont:font];
        BOOL isBold = !isHeading && (traits & NSBoldFontMask) != 0;
        BOOL isItalic = (traits & NSItalicFontMask) != 0;
        BOOL isMono = [font.fontName containsString:@"Mono"] || [font.familyName containsString:@"Mono"];

        BOOL isStrike = NO;
        NSNumber *strikeVal = attrs[NSStrikethroughStyleAttributeName];
        if (strikeVal && [strikeVal integerValue] != 0) isStrike = YES;

        if (isMono) {
            [content appendFormat:@"`%@`", runText];
        } else {
            if (isStrike) [content appendString:@"~~"];
            if (isBold && isItalic) [content appendString:@"***"];
            else if (isBold) [content appendString:@"**"];
            else if (isItalic) [content appendString:@"*"];

            [content appendString:runText];

            if (isBold && isItalic) [content appendString:@"***"];
            else if (isBold) [content appendString:@"**"];
            else if (isItalic) [content appendString:@"*"];
            if (isStrike) [content appendString:@"~~"];
        }

        pos = runEnd;
    }

    return content;
}

static NSString* attributedStringToMarkdown(NSAttributedString *attrStr) {
    if (attrStr.length == 0) return @"";

    NSMutableString *result = [NSMutableString string];
    NSString *plain = attrStr.string;
    NSArray<NSString*> *lines = [plain componentsSeparatedByString:@"\n"];

    NSUInteger charIndex = 0;
    for (NSUInteger i = 0; i < lines.count; i++) {
        NSString *line = lines[i];
        NSRange lineRange = NSMakeRange(charIndex, line.length);

        if (line.length == 0) {
            [result appendString:@"\n"];
            charIndex += 1; // skip the \n
            continue;
        }

        // Detect heading by font size at line start
        NSDictionary *startAttrs = [attrStr attributesAtIndex:lineRange.location effectiveRange:NULL];
        NSFont *startFont = startAttrs[NSFontAttributeName] ?: [NSFont systemFontOfSize:14];
        CGFloat fontSize = startFont.pointSize;

        NSString *prefix = @"";
        BOOL isHeading = NO;
        if (fontSize >= 26) { prefix = @"# "; isHeading = YES; }
        else if (fontSize >= 20) { prefix = @"## "; isHeading = YES; }
        else if (fontSize >= 16) { prefix = @"### "; isHeading = YES; }

        // Handle checkbox/bullet prefixes (unicode → markdown)
        NSUInteger contentOffset = 0;
        if ([line hasPrefix:@"\u2611 "]) {
            prefix = @"- [x] "; contentOffset = 2;
        } else if ([line hasPrefix:@"\u2610 "]) {
            prefix = @"- [ ] "; contentOffset = 2;
        } else if ([line hasPrefix:@"\u2022 "]) {
            prefix = @"- "; contentOffset = 2;
        }

        [result appendString:prefix];

        // Convert inline content with formatting detection
        NSRange contentRange = NSMakeRange(lineRange.location + contentOffset, lineRange.length - contentOffset);
        NSString *inlineContent = inlineContentToMarkdown(attrStr, contentRange, isHeading);
        [result appendString:inlineContent];
        [result appendString:@"\n"];

        charIndex += line.length + 1; // +1 for the \n separator
    }

    // Remove trailing newline
    if (result.length > 0 && [result characterAtIndex:result.length - 1] == '\n') {
        [result deleteCharactersInRange:NSMakeRange(result.length - 1, 1)];
    }

    return [result copy];
}

// --- Delegate ---

@interface JVRichTextDelegate : NSObject <NSTextViewDelegate>
@property (nonatomic, assign) uint64_t callbackID;
@property (nonatomic, assign) uint64_t formatCallbackID;
@property (nonatomic, assign) BOOL suppressCallback;
@property (nonatomic, weak) NSTextView *textView;
@end

@implementation JVRichTextDelegate

- (void)textDidChange:(NSNotification *)notification {
    if (self.suppressCallback) return;

    NSString *markdown = attributedStringToMarkdown(self.textView.textStorage);
    const char *val = [markdown UTF8String];
    GoCallbackInvoke(self.callbackID, val);
}

- (void)textViewDidChangeSelection:(NSNotification *)notification {
    if (self.suppressCallback || self.formatCallbackID == 0) return;

    NSTextView *tv = self.textView;
    if (!tv) return;

    NSDictionary *attrs;
    NSUInteger loc = tv.selectedRange.location;
    if (loc > 0 && loc <= tv.textStorage.length) {
        attrs = [tv.textStorage attributesAtIndex:loc - 1 effectiveRange:NULL];
    } else if (tv.textStorage.length > 0) {
        attrs = [tv.textStorage attributesAtIndex:0 effectiveRange:NULL];
    } else {
        attrs = tv.typingAttributes;
    }

    BOOL bold = NO;
    BOOL italic = NO;
    BOOL underline = NO;
    BOOL strikethrough = NO;

    NSFont *font = attrs[NSFontAttributeName];
    if (font) {
        NSFontTraitMask traits = [[NSFontManager sharedFontManager] traitsOfFont:font];
        bold = (traits & NSBoldFontMask) != 0;
        italic = (traits & NSItalicFontMask) != 0;
    }

    NSNumber *underlineVal = attrs[NSUnderlineStyleAttributeName];
    if (underlineVal && [underlineVal integerValue] != 0) {
        underline = YES;
    }

    NSNumber *strikeVal = attrs[NSStrikethroughStyleAttributeName];
    if (strikeVal && [strikeVal integerValue] != 0) {
        strikethrough = YES;
    }

    NSString *json = [NSString stringWithFormat:@"{\"bold\":%@,\"italic\":%@,\"underline\":%@,\"strikethrough\":%@}",
                      bold ? @"true" : @"false",
                      italic ? @"true" : @"false",
                      underline ? @"true" : @"false",
                      strikethrough ? @"true" : @"false"];
    GoCallbackInvoke(self.formatCallbackID, [json UTF8String]);
}

@end

// --- Rich text formatting via responder chain ---
// These selectors are referenced by toolbar standardAction items.
// underline: is already on NSText; the others don't exist in AppKit
// (they're iOS UIResponder methods), so we implement them here.

@interface NSTextView (JVFormatting)
- (void)toggleBoldface:(id)sender;
- (void)toggleItalics:(id)sender;
- (void)addStrikethrough:(id)sender;
- (void)insertChecklistItem:(id)sender;
@end

@implementation NSTextView (JVFormatting)

static void toggleFontTrait(NSTextView *tv, NSFontTraitMask trait) {
    NSFontManager *fm = [NSFontManager sharedFontManager];
    NSRange sel = tv.selectedRange;

    if (sel.length > 0) {
        [tv.textStorage beginEditing];
        [tv.textStorage enumerateAttribute:NSFontAttributeName inRange:sel options:0 usingBlock:^(id value, NSRange range, BOOL *stop) {
            NSFont *font = value ?: [NSFont systemFontOfSize:14];
            NSFontTraitMask traits = [fm traitsOfFont:font];
            NSFont *newFont;
            if (traits & trait) {
                newFont = [fm convertFont:font toNotHaveTrait:trait];
            } else {
                newFont = [fm convertFont:font toHaveTrait:trait];
            }
            [tv.textStorage addAttribute:NSFontAttributeName value:newFont range:range];
        }];
        [tv.textStorage endEditing];
    } else {
        NSMutableDictionary *attrs = [tv.typingAttributes mutableCopy];
        NSFont *font = attrs[NSFontAttributeName] ?: [NSFont systemFontOfSize:14];
        NSFontTraitMask traits = [fm traitsOfFont:font];
        NSFont *newFont;
        if (traits & trait) {
            newFont = [fm convertFont:font toNotHaveTrait:trait];
        } else {
            newFont = [fm convertFont:font toHaveTrait:trait];
        }
        attrs[NSFontAttributeName] = newFont;
        tv.typingAttributes = attrs;
    }
}

- (void)toggleBoldface:(id)sender {
    toggleFontTrait(self, NSBoldFontMask);
}

- (void)toggleItalics:(id)sender {
    toggleFontTrait(self, NSItalicFontMask);
}

- (void)addStrikethrough:(id)sender {
    NSRange sel = self.selectedRange;
    if (sel.length > 0) {
        // Check current strikethrough state at selection start
        NSDictionary *attrs = [self.textStorage attributesAtIndex:sel.location effectiveRange:NULL];
        NSNumber *current = attrs[NSStrikethroughStyleAttributeName];
        BOOL hasStrike = current && [current integerValue] != 0;
        NSNumber *newVal = hasStrike ? @(0) : @(NSUnderlineStyleSingle);
        [self.textStorage addAttribute:NSStrikethroughStyleAttributeName value:newVal range:sel];
    } else {
        NSMutableDictionary *attrs = [self.typingAttributes mutableCopy];
        NSNumber *current = attrs[NSStrikethroughStyleAttributeName];
        BOOL hasStrike = current && [current integerValue] != 0;
        attrs[NSStrikethroughStyleAttributeName] = hasStrike ? @(0) : @(NSUnderlineStyleSingle);
        self.typingAttributes = attrs;
    }
}

- (void)insertChecklistItem:(id)sender {
    NSRange sel = self.selectedRange;
    NSString *text = self.string;

    // Find start of current line
    NSUInteger lineStart = sel.location;
    while (lineStart > 0 && [text characterAtIndex:lineStart - 1] != '\n') {
        lineStart--;
    }

    // Check if current line already has a checklist prefix
    NSString *lineFromStart = [text substringFromIndex:lineStart];
    if ([lineFromStart hasPrefix:@"\u2610 "] || [lineFromStart hasPrefix:@"\u2611 "]) {
        // Remove the checklist prefix
        [self setSelectedRange:NSMakeRange(lineStart, 2)];
        [self insertText:@"" replacementRange:NSMakeRange(lineStart, 2)];
    } else {
        // Insert checklist prefix at line start
        [self insertText:@"\u2610 " replacementRange:NSMakeRange(lineStart, 0)];
    }
}
@end

// --- C API ---

void* JVCreateRichTextEditor(const char* content, bool editable, uint64_t callbackID) {
    NSTextView *textView = [[NSTextView alloc] initWithFrame:NSMakeRect(0, 0, 400, 300)];
    textView.richText = YES;
    textView.allowsUndo = YES;
    textView.usesFontPanel = NO;
    textView.editable = editable;
    textView.selectable = YES;
    textView.textContainerInset = NSMakeSize(16, 16);
    textView.font = systemFontRegular(14);

    // Standard NSTextView-in-NSScrollView configuration
    textView.minSize = NSMakeSize(0, 0);
    textView.maxSize = NSMakeSize(FLT_MAX, FLT_MAX);
    textView.verticallyResizable = YES;
    textView.horizontallyResizable = NO;
    textView.autoresizingMask = NSViewWidthSizable;
    textView.textContainer.containerSize = NSMakeSize(400, FLT_MAX);
    textView.textContainer.widthTracksTextView = YES;

    // Set initial content
    NSString *markdown = [NSString stringWithUTF8String:content];
    NSAttributedString *attrStr = markdownToAttributedString(markdown);
    [textView.textStorage setAttributedString:attrStr];

    // Set up delegate
    JVRichTextDelegate *delegate = [[JVRichTextDelegate alloc] init];
    delegate.callbackID = callbackID;
    delegate.textView = textView;
    textView.delegate = delegate;

    // Wrap in scroll view
    NSScrollView *scrollView = [[NSScrollView alloc] init];
    scrollView.translatesAutoresizingMaskIntoConstraints = NO;
    scrollView.hasVerticalScroller = YES;
    scrollView.hasHorizontalScroller = NO;
    scrollView.autohidesScrollers = YES;
    scrollView.borderType = NSNoBorder;
    scrollView.documentView = textView;
    scrollView.drawsBackground = YES;

    // Store references
    objc_setAssociatedObject(scrollView, kRTEDelegateKey, delegate, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(scrollView, kRTETextViewKey, textView, OBJC_ASSOCIATION_RETAIN_NONATOMIC);

    // Make the text view first responder once it's in a window,
    // so toolbar standardAction items (B/I/U/S) are validated and enabled.
    dispatch_async(dispatch_get_main_queue(), ^{
        NSWindow *window = scrollView.window;
        if (window && editable) {
            [window makeFirstResponder:textView];
        }
    });

    return (__bridge_retained void*)scrollView;
}

void JVUpdateRichTextEditor(void* handle, const char* content, bool editable) {
    NSScrollView *scrollView = (__bridge NSScrollView*)handle;
    NSTextView *textView = objc_getAssociatedObject(scrollView, kRTETextViewKey);
    JVRichTextDelegate *delegate = objc_getAssociatedObject(scrollView, kRTEDelegateKey);
    if (!textView) return;

    textView.editable = editable;

    NSString *markdown = [NSString stringWithUTF8String:content];

    // Only update content if it actually changed (avoid cursor jump during typing)
    NSString *currentMarkdown = attributedStringToMarkdown(textView.textStorage);
    if (![currentMarkdown isEqualToString:markdown]) {
        NSAttributedString *attrStr = markdownToAttributedString(markdown);
        delegate.suppressCallback = YES;
        [textView.textStorage setAttributedString:attrStr];
        delegate.suppressCallback = NO;
    }
}

void JVRichTextEditorSetFormatCallbackID(void* handle, uint64_t callbackID) {
    NSScrollView *scrollView = (__bridge NSScrollView*)handle;
    JVRichTextDelegate *delegate = objc_getAssociatedObject(scrollView, kRTEDelegateKey);
    if (delegate) {
        delegate.formatCallbackID = callbackID;
    }
}
