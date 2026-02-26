#import <Cocoa/Cocoa.h>
#include "textfield.h"
#import <objc/runtime.h>

extern void GoCallbackInvoke(uint64_t callbackID, const char* data);

static const void *kTextFieldCallbackKey = &kTextFieldCallbackKey;
static const void *kTextFieldDelegateKey = &kTextFieldDelegateKey;
static const void *kTextFieldErrorStackKey = &kTextFieldErrorStackKey;
static const void *kTextFieldInnerFieldKey = &kTextFieldInnerFieldKey;

@interface JVTextFieldDelegate : NSObject <NSTextFieldDelegate>
@property (nonatomic, assign) uint64_t callbackID;
@end

@implementation JVTextFieldDelegate

- (void)controlTextDidChange:(NSNotification *)notification {
    NSTextField *field = notification.object;
    const char *val = [field.stringValue UTF8String];
    GoCallbackInvoke(self.callbackID, val);
}

@end

void* JVCreateTextField(const char* placeholder, const char* value,
                         const char* inputType, bool readOnly, uint64_t callbackID) {
    NSString *placeholderStr = [NSString stringWithUTF8String:placeholder];
    NSString *valueStr = [NSString stringWithUTF8String:value];
    NSString *inputTypeStr = [NSString stringWithUTF8String:inputType];

    NSTextField *field;

    if ([inputTypeStr isEqualToString:@"obscured"]) {
        NSSecureTextField *secureField = [[NSSecureTextField alloc] init];
        field = secureField;
    } else {
        field = [[NSTextField alloc] init];
    }

    field.placeholderString = placeholderStr;
    field.stringValue = valueStr;
    field.editable = !readOnly;
    field.bezeled = YES;
    field.bezelStyle = NSTextFieldRoundedBezel;
    field.translatesAutoresizingMaskIntoConstraints = NO;
    [field.widthAnchor constraintGreaterThanOrEqualToConstant:200].active = YES;

    // Set up delegate for change notifications
    JVTextFieldDelegate *delegate = [[JVTextFieldDelegate alloc] init];
    delegate.callbackID = callbackID;
    field.delegate = delegate;

    // Prevent delegate from being deallocated
    objc_setAssociatedObject(field, kTextFieldDelegateKey, delegate, OBJC_ASSOCIATION_RETAIN_NONATOMIC);

    // Wrap in a stack view to support error labels
    NSStackView *wrapper = [[NSStackView alloc] init];
    wrapper.orientation = NSUserInterfaceLayoutOrientationVertical;
    wrapper.spacing = 4;
    wrapper.translatesAutoresizingMaskIntoConstraints = NO;
    wrapper.alignment = NSLayoutAttributeLeading;

    [wrapper addArrangedSubview:field];

    // Store references
    objc_setAssociatedObject(wrapper, kTextFieldInnerFieldKey, field, OBJC_ASSOCIATION_RETAIN_NONATOMIC);

    return (__bridge_retained void*)wrapper;
}

void JVUpdateTextField(void* handle, const char* placeholder, const char* value,
                        const char* inputType, bool readOnly) {
    NSStackView *wrapper = (__bridge NSStackView*)handle;
    NSTextField *field = objc_getAssociatedObject(wrapper, kTextFieldInnerFieldKey);
    if (!field) return;

    field.placeholderString = [NSString stringWithUTF8String:placeholder];

    // Only update value if it actually changed (avoid cursor jump during typing)
    NSString *newValue = [NSString stringWithUTF8String:value];
    if (![field.stringValue isEqualToString:newValue]) {
        field.stringValue = newValue;
    }

    field.editable = !readOnly;
}

void JVSetTextFieldErrors(void* handle, const char** errors, int count) {
    NSStackView *wrapper = (__bridge NSStackView*)handle;
    NSTextField *field = objc_getAssociatedObject(wrapper, kTextFieldInnerFieldKey);
    if (!field) return;

    // Remove old error labels (everything after the text field)
    NSMutableArray<NSView*> *errorLabels = objc_getAssociatedObject(wrapper, kTextFieldErrorStackKey);
    if (errorLabels) {
        for (NSView *label in errorLabels) {
            [wrapper removeArrangedSubview:label];
            [label removeFromSuperview];
        }
    }

    NSMutableArray<NSView*> *newLabels = [NSMutableArray array];

    if (count > 0) {
        // Red border on the field
        field.wantsLayer = YES;
        field.layer.borderColor = [NSColor systemRedColor].CGColor;
        field.layer.borderWidth = 1.0;
        field.layer.cornerRadius = 4.0;

        for (int i = 0; i < count; i++) {
            NSString *errStr = [NSString stringWithUTF8String:errors[i]];
            NSTextField *errorLabel = [NSTextField labelWithString:errStr];
            errorLabel.font = [NSFont systemFontOfSize:11];
            errorLabel.textColor = [NSColor systemRedColor];
            errorLabel.lineBreakMode = NSLineBreakByWordWrapping;
            errorLabel.maximumNumberOfLines = 0;
            [wrapper addArrangedSubview:errorLabel];
            [newLabels addObject:errorLabel];
        }
    } else {
        // Clear border
        field.wantsLayer = YES;
        field.layer.borderColor = nil;
        field.layer.borderWidth = 0;
    }

    objc_setAssociatedObject(wrapper, kTextFieldErrorStackKey, newLabels, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
}
