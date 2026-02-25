#import <Cocoa/Cocoa.h>
#include "clickgesture.h"
#import <objc/runtime.h>

extern void GoCallbackInvoke(uint64_t callbackID, const char* data);

static const void *kClickGestureTargetKey = &kClickGestureTargetKey;

@interface JVClickGestureTarget : NSObject
@property (nonatomic, assign) uint64_t callbackID;
- (void)handleClick:(NSClickGestureRecognizer *)recognizer;
@end

@implementation JVClickGestureTarget

- (void)handleClick:(NSClickGestureRecognizer *)recognizer {
    GoCallbackInvoke(self.callbackID, "");
}

@end

void JVAttachClickGesture(void* handle, uint64_t callbackID) {
    NSView *view = (__bridge NSView*)handle;

    JVClickGestureTarget *target = [[JVClickGestureTarget alloc] init];
    target.callbackID = callbackID;

    NSClickGestureRecognizer *gesture = [[NSClickGestureRecognizer alloc]
        initWithTarget:target action:@selector(handleClick:)];
    gesture.numberOfClicksRequired = 1;
    [view addGestureRecognizer:gesture];

    // Prevent target from being deallocated
    objc_setAssociatedObject(view, kClickGestureTargetKey, target, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
}
