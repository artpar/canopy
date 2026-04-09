#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "chatwindow.h"

extern void GoChatSendMessage(const char* text);

// Flipped view so messages stack from top.
@interface JVChatFlippedView : NSView
@end

@implementation JVChatFlippedView
- (BOOL)isFlipped { return YES; }
@end

@interface JVChatInputDelegate : NSObject <NSTextFieldDelegate>
@property (nonatomic, weak) NSTextField *textField;
@end

@implementation JVChatInputDelegate

- (BOOL)control:(NSControl *)control textView:(NSTextView *)textView doCommandBySelector:(SEL)commandSelector {
    if (commandSelector == @selector(insertNewline:)) {
        NSString *text = self.textField.stringValue;
        if ([text length] > 0) {
            GoChatSendMessage([text UTF8String]);
            self.textField.stringValue = @"";
        }
        return YES;
    }
    return NO;
}

@end

static NSPanel *chatPanel = nil;
static NSStackView *messageStack = nil;
static NSScrollView *scrollView = nil;
static NSProgressIndicator *spinner = nil;
static JVChatInputDelegate *inputDelegate = nil;
static NSView *emptyStateView = nil;

static NSView* createEmptyState(void) {
    NSStackView *stack = [[NSStackView alloc] init];
    stack.translatesAutoresizingMaskIntoConstraints = NO;
    stack.orientation = NSUserInterfaceLayoutOrientationVertical;
    stack.alignment = NSLayoutAttributeCenterX;
    stack.spacing = 12;

    // Icon
    NSImageView *icon = [[NSImageView alloc] init];
    icon.translatesAutoresizingMaskIntoConstraints = NO;
    NSImage *img = [NSImage imageWithSystemSymbolName:@"bubble.left.and.text.bubble.right"
                              accessibilityDescription:@"Chat"];
    if (img) {
        NSImageSymbolConfiguration *cfg = [NSImageSymbolConfiguration configurationWithPointSize:36
                                                                                          weight:NSFontWeightUltraLight];
        icon.image = [img imageWithSymbolConfiguration:cfg];
        icon.contentTintColor = [NSColor tertiaryLabelColor];
    }
    [icon.widthAnchor constraintEqualToConstant:48].active = YES;
    [icon.heightAnchor constraintEqualToConstant:48].active = YES;

    // Text
    NSTextField *label = [NSTextField labelWithString:@"Ask anything to refine your UI"];
    label.translatesAutoresizingMaskIntoConstraints = NO;
    label.font = [NSFont systemFontOfSize:13 weight:NSFontWeightRegular];
    label.textColor = [NSColor tertiaryLabelColor];
    label.alignment = NSTextAlignmentCenter;

    [stack addArrangedSubview:icon];
    [stack addArrangedSubview:label];

    return stack;
}

static void ensureChatWindow(void) {
    if (chatPanel) return;

    chatPanel = [[NSPanel alloc] initWithContentRect:NSMakeRect(0, 0, 380, 480)
                                           styleMask:(NSWindowStyleMaskTitled |
                                                      NSWindowStyleMaskClosable |
                                                      NSWindowStyleMaskResizable |
                                                      NSWindowStyleMaskFullSizeContentView)
                                             backing:NSBackingStoreBuffered
                                               defer:YES];
    chatPanel.title = @"Canopy";
    chatPanel.titleVisibility = NSWindowTitleHidden;
    chatPanel.titlebarAppearsTransparent = YES;
    chatPanel.floatingPanel = YES;
    chatPanel.becomesKeyOnlyIfNeeded = NO;
    chatPanel.releasedWhenClosed = NO;
    chatPanel.minSize = NSMakeSize(320, 300);
    chatPanel.backgroundColor = [NSColor clearColor];
    [chatPanel setFrameAutosaveName:@"JVChatWindow"];

    NSView *cv = chatPanel.contentView;

    // --- Vibrancy background for message area ---
    NSVisualEffectView *vibrancy = [[NSVisualEffectView alloc] init];
    vibrancy.translatesAutoresizingMaskIntoConstraints = NO;
    vibrancy.material = NSVisualEffectMaterialSidebar;
    vibrancy.blendingMode = NSVisualEffectBlendingModeBehindWindow;
    vibrancy.state = NSVisualEffectStateActive;
    [cv addSubview:vibrancy];
    [NSLayoutConstraint activateConstraints:@[
        [vibrancy.topAnchor constraintEqualToAnchor:cv.topAnchor],
        [vibrancy.leadingAnchor constraintEqualToAnchor:cv.leadingAnchor],
        [vibrancy.trailingAnchor constraintEqualToAnchor:cv.trailingAnchor],
        [vibrancy.bottomAnchor constraintEqualToAnchor:cv.bottomAnchor],
    ]];

    // --- Message area ---
    messageStack = [[NSStackView alloc] init];
    messageStack.orientation = NSUserInterfaceLayoutOrientationVertical;
    messageStack.alignment = NSLayoutAttributeWidth;
    messageStack.spacing = 12;
    messageStack.translatesAutoresizingMaskIntoConstraints = NO;
    messageStack.edgeInsets = NSEdgeInsetsMake(16, 16, 16, 16);

    JVChatFlippedView *container = [[JVChatFlippedView alloc] init];
    container.translatesAutoresizingMaskIntoConstraints = NO;
    [container addSubview:messageStack];

    [NSLayoutConstraint activateConstraints:@[
        [messageStack.topAnchor constraintEqualToAnchor:container.topAnchor],
        [messageStack.leadingAnchor constraintEqualToAnchor:container.leadingAnchor],
        [messageStack.trailingAnchor constraintEqualToAnchor:container.trailingAnchor],
        [messageStack.bottomAnchor constraintEqualToAnchor:container.bottomAnchor],
    ]];

    scrollView = [[NSScrollView alloc] init];
    scrollView.translatesAutoresizingMaskIntoConstraints = NO;
    scrollView.hasVerticalScroller = YES;
    scrollView.hasHorizontalScroller = NO;
    scrollView.autohidesScrollers = YES;
    scrollView.borderType = NSNoBorder;
    scrollView.drawsBackground = NO;
    scrollView.documentView = container;

    [container.widthAnchor constraintEqualToAnchor:scrollView.contentView.widthAnchor].active = YES;

    // --- Empty state ---
    emptyStateView = createEmptyState();
    [container addSubview:emptyStateView];
    [NSLayoutConstraint activateConstraints:@[
        [emptyStateView.centerXAnchor constraintEqualToAnchor:container.centerXAnchor],
        [emptyStateView.topAnchor constraintEqualToAnchor:container.topAnchor constant:120],
    ]];

    // --- Input area with visual effect background ---
    NSVisualEffectView *inputBg = [[NSVisualEffectView alloc] init];
    inputBg.translatesAutoresizingMaskIntoConstraints = NO;
    inputBg.material = NSVisualEffectMaterialContentBackground;
    inputBg.blendingMode = NSVisualEffectBlendingModeWithinWindow;

    NSTextField *inputField = [[NSTextField alloc] init];
    inputField.translatesAutoresizingMaskIntoConstraints = NO;
    inputField.placeholderString = @"Describe what you'd like to change...";
    inputField.bezelStyle = NSTextFieldRoundedBezel;
    inputField.font = [NSFont systemFontOfSize:14];
    [inputField.heightAnchor constraintGreaterThanOrEqualToConstant:32].active = YES;

    inputDelegate = [[JVChatInputDelegate alloc] init];
    inputDelegate.textField = inputField;
    inputField.delegate = inputDelegate;

    spinner = [[NSProgressIndicator alloc] init];
    spinner.translatesAutoresizingMaskIntoConstraints = NO;
    spinner.style = NSProgressIndicatorStyleSpinning;
    spinner.indeterminate = YES;
    spinner.displayedWhenStopped = NO;
    [spinner.widthAnchor constraintEqualToConstant:18].active = YES;
    [spinner.heightAnchor constraintEqualToConstant:18].active = YES;

    [inputBg addSubview:inputField];
    [inputBg addSubview:spinner];
    [NSLayoutConstraint activateConstraints:@[
        [inputField.topAnchor constraintEqualToAnchor:inputBg.topAnchor constant:10],
        [inputField.leadingAnchor constraintEqualToAnchor:inputBg.leadingAnchor constant:12],
        [inputField.bottomAnchor constraintEqualToAnchor:inputBg.bottomAnchor constant:-10],
        [inputField.trailingAnchor constraintEqualToAnchor:spinner.leadingAnchor constant:-8],
        [spinner.trailingAnchor constraintEqualToAnchor:inputBg.trailingAnchor constant:-12],
        [spinner.centerYAnchor constraintEqualToAnchor:inputBg.centerYAnchor],
    ]];

    // Thin separator above input
    NSBox *separator = [[NSBox alloc] init];
    separator.translatesAutoresizingMaskIntoConstraints = NO;
    separator.boxType = NSBoxSeparator;

    // --- Root layout ---
    NSStackView *rootStack = [[NSStackView alloc] init];
    rootStack.translatesAutoresizingMaskIntoConstraints = NO;
    rootStack.orientation = NSUserInterfaceLayoutOrientationVertical;
    rootStack.spacing = 0;
    [rootStack addArrangedSubview:scrollView];
    [rootStack addArrangedSubview:separator];
    [rootStack addArrangedSubview:inputBg];

    // Scroll view takes all available space
    [scrollView setContentHuggingPriority:NSLayoutPriorityDefaultLow forOrientation:NSLayoutConstraintOrientationVertical];
    [inputBg setContentHuggingPriority:NSLayoutPriorityDefaultHigh forOrientation:NSLayoutConstraintOrientationVertical];

    [vibrancy addSubview:rootStack];
    [NSLayoutConstraint activateConstraints:@[
        [rootStack.topAnchor constraintEqualToAnchor:vibrancy.safeAreaLayoutGuide.topAnchor],
        [rootStack.leadingAnchor constraintEqualToAnchor:vibrancy.leadingAnchor],
        [rootStack.trailingAnchor constraintEqualToAnchor:vibrancy.trailingAnchor],
        [rootStack.bottomAnchor constraintEqualToAnchor:vibrancy.bottomAnchor],
    ]];

    // Center relative to key window or screen
    NSWindow *keyWindow = [NSApp keyWindow];
    if (keyWindow && keyWindow != chatPanel) {
        NSRect parentFrame = keyWindow.frame;
        CGFloat x = NSMidX(parentFrame) - 190;
        CGFloat y = NSMidY(parentFrame) - 240;
        [chatPanel setFrameOrigin:NSMakePoint(x, y)];
    } else {
        [chatPanel center];
    }
}

static void hideEmptyState(void) {
    if (emptyStateView && !emptyStateView.hidden) {
        emptyStateView.hidden = YES;
    }
}

static void scrollToBottom(void) {
    if (!scrollView || !scrollView.documentView) return;
    [messageStack layoutSubtreeIfNeeded];
    NSView *docView = scrollView.documentView;
    CGFloat docHeight = NSHeight(docView.frame);
    CGFloat clipHeight = NSHeight(scrollView.contentView.bounds);
    if (docHeight > clipHeight) {
        [docView scrollPoint:NSMakePoint(0, docHeight - clipHeight)];
    }
}

static NSView* createUserBubble(NSString *text) {
    NSTextField *label = [NSTextField wrappingLabelWithString:text];
    label.translatesAutoresizingMaskIntoConstraints = NO;
    label.selectable = YES;
    label.font = [NSFont systemFontOfSize:13];
    label.textColor = [NSColor labelColor];

    // Rounded bubble
    NSBox *bubble = [[NSBox alloc] init];
    bubble.translatesAutoresizingMaskIntoConstraints = NO;
    bubble.boxType = NSBoxCustom;
    bubble.borderType = NSNoBorder;
    bubble.cornerRadius = 12;
    bubble.fillColor = [NSColor.systemBlueColor colorWithAlphaComponent:0.12];
    bubble.contentViewMargins = NSMakeSize(0, 0);

    [bubble.contentView addSubview:label];
    [NSLayoutConstraint activateConstraints:@[
        [label.topAnchor constraintEqualToAnchor:bubble.contentView.topAnchor constant:8],
        [label.leadingAnchor constraintEqualToAnchor:bubble.contentView.leadingAnchor constant:12],
        [label.trailingAnchor constraintEqualToAnchor:bubble.contentView.trailingAnchor constant:-12],
        [label.bottomAnchor constraintEqualToAnchor:bubble.contentView.bottomAnchor constant:-8],
    ]];

    // Row that right-aligns the bubble
    NSStackView *row = [[NSStackView alloc] init];
    row.translatesAutoresizingMaskIntoConstraints = NO;
    row.orientation = NSUserInterfaceLayoutOrientationHorizontal;
    row.alignment = NSLayoutAttributeTop;
    row.spacing = 0;

    // Spacer pushes bubble to the right
    NSView *spacer = [[NSView alloc] init];
    spacer.translatesAutoresizingMaskIntoConstraints = NO;
    [spacer setContentHuggingPriority:1 forOrientation:NSLayoutConstraintOrientationHorizontal];

    [row addArrangedSubview:spacer];
    [row addArrangedSubview:bubble];

    // Bubble max width: 75% of row
    [bubble.widthAnchor constraintLessThanOrEqualToAnchor:row.widthAnchor multiplier:0.75].active = YES;

    return row;
}

static NSView* createStatusBubble(NSString *text) {
    // Icon
    NSImageView *icon = [[NSImageView alloc] init];
    icon.translatesAutoresizingMaskIntoConstraints = NO;
    NSImage *img = [NSImage imageWithSystemSymbolName:@"sparkles"
                              accessibilityDescription:@"AI"];
    if (img) {
        NSImageSymbolConfiguration *cfg = [NSImageSymbolConfiguration configurationWithPointSize:12
                                                                                          weight:NSFontWeightRegular];
        icon.image = [img imageWithSymbolConfiguration:cfg];
        icon.contentTintColor = [NSColor secondaryLabelColor];
    }
    [icon.widthAnchor constraintEqualToConstant:16].active = YES;
    [icon.heightAnchor constraintEqualToConstant:16].active = YES;

    // Text
    NSTextField *label = [NSTextField labelWithString:text];
    label.translatesAutoresizingMaskIntoConstraints = NO;
    label.font = [NSFont systemFontOfSize:12 weight:NSFontWeightMedium];
    label.textColor = [NSColor secondaryLabelColor];

    // Row
    NSStackView *row = [[NSStackView alloc] init];
    row.translatesAutoresizingMaskIntoConstraints = NO;
    row.orientation = NSUserInterfaceLayoutOrientationHorizontal;
    row.alignment = NSLayoutAttributeCenterY;
    row.spacing = 6;

    [row addArrangedSubview:icon];
    [row addArrangedSubview:label];

    return row;
}

// --- Public API ---

void JVShowChatWindow(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        ensureChatWindow();
        [chatPanel makeKeyAndOrderFront:nil];
        [NSApp activateIgnoringOtherApps:YES];
    });
}

void JVHideChatWindow(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [chatPanel orderOut:nil];
    });
}

void JVChatAddUserMessage(const char* text) {
    NSString *str = [NSString stringWithUTF8String:text];
    dispatch_async(dispatch_get_main_queue(), ^{
        ensureChatWindow();
        hideEmptyState();
        NSView *row = createUserBubble(str);
        [messageStack addArrangedSubview:row];
        scrollToBottom();
    });
}

void JVChatAddStatusMessage(const char* text) {
    NSString *str = [NSString stringWithUTF8String:text];
    dispatch_async(dispatch_get_main_queue(), ^{
        ensureChatWindow();
        hideEmptyState();
        NSView *row = createStatusBubble(str);
        [messageStack addArrangedSubview:row];
        scrollToBottom();
    });
}

void JVChatSetBusy(bool busy) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (busy) {
            [spinner startAnimation:nil];
        } else {
            [spinner stopAnimation:nil];
        }
    });
}
