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

static NSString* attributedStringToMarkdown(NSAttributedString *attrStr) {
    // Simple approach: return the plain text, preserving markdown syntax
    // The editor stores displayed text with unicode bullets/checkboxes,
    // so we convert back
    NSMutableString *result = [NSMutableString string];
    NSString *plain = attrStr.string;
    NSArray<NSString*> *lines = [plain componentsSeparatedByString:@"\n"];

    for (NSString *line in lines) {
        NSString *converted = line;

        // Convert checkbox unicode back to markdown
        if ([line hasPrefix:@"\u2611 "]) {
            converted = [NSString stringWithFormat:@"- [x] %@", [line substringFromIndex:2]];
        } else if ([line hasPrefix:@"\u2610 "]) {
            converted = [NSString stringWithFormat:@"- [ ] %@", [line substringFromIndex:2]];
        } else if ([line hasPrefix:@"\u2022 "]) {
            converted = [NSString stringWithFormat:@"- %@", [line substringFromIndex:2]];
        }

        [result appendString:converted];
        [result appendString:@"\n"];
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

@end

// --- Checklist insertion via responder chain ---

@interface NSTextView (JVChecklist)
- (void)insertChecklistItem:(id)sender;
@end

@implementation NSTextView (JVChecklist)
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

    return (__bridge_retained void*)scrollView;
}

void JVUpdateRichTextEditor(void* handle, const char* content, bool editable) {
    NSScrollView *scrollView = (__bridge NSScrollView*)handle;
    NSTextView *textView = objc_getAssociatedObject(scrollView, kRTETextViewKey);
    JVRichTextDelegate *delegate = objc_getAssociatedObject(scrollView, kRTEDelegateKey);
    if (!textView) return;

    textView.editable = editable;

    NSString *markdown = [NSString stringWithUTF8String:content];
    NSAttributedString *attrStr = markdownToAttributedString(markdown);
    delegate.suppressCallback = YES;
    [textView.textStorage setAttributedString:attrStr];
    delegate.suppressCallback = NO;
}
