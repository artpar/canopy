#import <Cocoa/Cocoa.h>
#include "progressbar.h"

void* JVCreateProgressBar(double min, double max, double value, bool indeterminate) {
    NSProgressIndicator *bar = [[NSProgressIndicator alloc] init];
    bar.translatesAutoresizingMaskIntoConstraints = NO;
    bar.style = NSProgressIndicatorStyleBar;
    bar.minValue = min;
    bar.maxValue = max;
    bar.doubleValue = value;
    bar.indeterminate = indeterminate;
    [bar.widthAnchor constraintGreaterThanOrEqualToConstant:200].active = YES;

    if (indeterminate) {
        [bar startAnimation:nil];
    }

    return (__bridge_retained void*)bar;
}

void JVUpdateProgressBar(void* handle, double min, double max, double value, bool indeterminate) {
    NSProgressIndicator *bar = (__bridge NSProgressIndicator*)handle;
    bar.minValue = min;
    bar.maxValue = max;

    BOOL wasIndeterminate = bar.indeterminate;
    bar.indeterminate = indeterminate;

    if (indeterminate) {
        if (!wasIndeterminate) {
            [bar startAnimation:nil];
        }
    } else {
        if (wasIndeterminate) {
            [bar stopAnimation:nil];
        }
        bar.doubleValue = value;
    }
}
