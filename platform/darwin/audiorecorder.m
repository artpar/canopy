#import <Cocoa/Cocoa.h>
#import <AVFoundation/AVFoundation.h>
#include "audiorecorder.h"
#import <objc/runtime.h>

extern void GoCallbackInvoke(uint64_t callbackID, const char* data);

static const void *kRecorderKey = &kRecorderKey;
static const void *kRecorderFormatKey = &kRecorderFormatKey;
static const void *kRecorderSampleRateKey = &kRecorderSampleRateKey;
static const void *kRecorderChannelsKey = &kRecorderChannelsKey;
static const void *kRecorderStartedCbIDKey = &kRecorderStartedCbIDKey;
static const void *kRecorderStoppedCbIDKey = &kRecorderStoppedCbIDKey;
static const void *kRecorderLevelCbIDKey = &kRecorderLevelCbIDKey;
static const void *kRecorderErrorCbIDKey = &kRecorderErrorCbIDKey;
static const void *kRecorderTimerKey = &kRecorderTimerKey;
static const void *kRecorderButtonKey = &kRecorderButtonKey;
static const void *kRecorderTimeLabelKey = &kRecorderTimeLabelKey;
static const void *kRecorderLevelIndicatorKey = &kRecorderLevelIndicatorKey;
static const void *kRecorderStartTimeKey = &kRecorderStartTimeKey;
static const void *kRecorderPathKey = &kRecorderPathKey;
static const void *kRecorderButtonActionKey = &kRecorderButtonActionKey;

@interface JVRecordButtonAction : NSObject
@property (nonatomic, assign) NSView *container;
- (void)toggleRecord:(NSButton *)sender;
@end

@implementation JVRecordButtonAction
- (void)toggleRecord:(NSButton *)sender {
    JVAudioRecorderToggle((__bridge void*)self.container);
}
@end

static NSString* formatTime(NSTimeInterval seconds) {
    if (isnan(seconds) || seconds < 0) seconds = 0;
    int mins = (int)seconds / 60;
    int secs = (int)seconds % 60;
    return [NSString stringWithFormat:@"%d:%02d", mins, secs];
}

static void fireError(NSView *container, NSString *msg) {
    NSNumber *cbNum = objc_getAssociatedObject(container, kRecorderErrorCbIDKey);
    uint64_t cbID = [cbNum unsignedLongLongValue];
    if (cbID == 0) return;
    NSString *json = [NSString stringWithFormat:@"{\"error\":\"%@\"}", msg];
    GoCallbackInvoke(cbID, json.UTF8String);
}

static void stopMeteringTimer(NSView *container) {
    dispatch_source_t timer = objc_getAssociatedObject(container, kRecorderTimerKey);
    if (timer) {
        dispatch_source_cancel(timer);
        objc_setAssociatedObject(container, kRecorderTimerKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    }
}

static void startMeteringTimer(NSView *container) {
    AVAudioRecorder *recorder = objc_getAssociatedObject(container, kRecorderKey);
    NSNumber *levelCbNum = objc_getAssociatedObject(container, kRecorderLevelCbIDKey);
    uint64_t levelCbID = [levelCbNum unsignedLongLongValue];
    NSLevelIndicator *levelIndicator = objc_getAssociatedObject(container, kRecorderLevelIndicatorKey);
    NSTextField *timeLabel = objc_getAssociatedObject(container, kRecorderTimeLabelKey);

    dispatch_source_t timer = dispatch_source_create(DISPATCH_SOURCE_TYPE_TIMER, 0, 0, dispatch_get_main_queue());
    dispatch_source_set_timer(timer, dispatch_time(DISPATCH_TIME_NOW, 0), NSEC_PER_SEC / 10, NSEC_PER_SEC / 100);

    __weak AVAudioRecorder *weakRecorder = recorder;
    __weak NSView *weakContainer = container;
    dispatch_source_set_event_handler(timer, ^{
        AVAudioRecorder *r = weakRecorder;
        NSView *c = weakContainer;
        if (!r || !c || !r.isRecording) return;

        [r updateMeters];
        float level = [r averagePowerForChannel:0];

        // Update level indicator (normalize from -60..0 dB to 0..1)
        float normalized = (level + 60.0f) / 60.0f;
        if (normalized < 0) normalized = 0;
        if (normalized > 1) normalized = 1;
        levelIndicator.doubleValue = normalized;

        // Update time label
        NSDate *startTime = objc_getAssociatedObject(c, kRecorderStartTimeKey);
        if (startTime) {
            NSTimeInterval elapsed = -[startTime timeIntervalSinceNow];
            timeLabel.stringValue = formatTime(elapsed);
        }

        // Fire level callback
        if (levelCbID != 0) {
            NSString *json = [NSString stringWithFormat:@"{\"level\":%.1f}", level];
            GoCallbackInvoke(levelCbID, json.UTF8String);
        }
    });
    dispatch_resume(timer);
    objc_setAssociatedObject(container, kRecorderTimerKey, timer, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
}

static AVAudioRecorder* createRecorder(NSView *container) {
    NSString *format = objc_getAssociatedObject(container, kRecorderFormatKey);
    NSNumber *sampleRateNum = objc_getAssociatedObject(container, kRecorderSampleRateKey);
    NSNumber *channelsNum = objc_getAssociatedObject(container, kRecorderChannelsKey);

    double sampleRate = [sampleRateNum doubleValue];
    int channels = [channelsNum intValue];

    // Determine file extension and format ID
    NSString *ext = @"m4a";
    AudioFormatID formatID = kAudioFormatMPEG4AAC;
    if ([format isEqualToString:@"wav"]) {
        ext = @"wav";
        formatID = kAudioFormatLinearPCM;
    }

    // Temp file path
    NSString *timestamp = [NSString stringWithFormat:@"%lld", (long long)([[NSDate date] timeIntervalSince1970] * 1000)];
    NSString *filename = [NSString stringWithFormat:@"canopy_recording_%@.%@", timestamp, ext];
    NSString *path = [NSTemporaryDirectory() stringByAppendingPathComponent:filename];
    NSURL *url = [NSURL fileURLWithPath:path];

    NSDictionary *settings = @{
        AVFormatIDKey: @(formatID),
        AVSampleRateKey: @(sampleRate),
        AVNumberOfChannelsKey: @(channels),
    };

    NSError *error = nil;
    AVAudioRecorder *recorder = [[AVAudioRecorder alloc] initWithURL:url settings:settings error:&error];
    if (!recorder) {
        fireError(container, error.localizedDescription ?: @"Failed to create audio recorder");
        return nil;
    }

    recorder.meteringEnabled = YES;
    objc_setAssociatedObject(container, kRecorderKey, recorder, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(container, kRecorderPathKey, path, OBJC_ASSOCIATION_RETAIN_NONATOMIC);

    return recorder;
}

void* JVCreateAudioRecorder(const char* format, double sampleRate, int channels,
    uint64_t startedCbID, uint64_t stoppedCbID, uint64_t levelCbID, uint64_t errorCbID) {

    NSView *container = [[NSView alloc] init];
    container.translatesAutoresizingMaskIntoConstraints = NO;

    // Store config
    NSString *formatStr = [NSString stringWithUTF8String:format];
    objc_setAssociatedObject(container, kRecorderFormatKey, formatStr, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(container, kRecorderSampleRateKey, @(sampleRate), OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(container, kRecorderChannelsKey, @(channels), OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(container, kRecorderStartedCbIDKey, @(startedCbID), OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(container, kRecorderStoppedCbIDKey, @(stoppedCbID), OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(container, kRecorderLevelCbIDKey, @(levelCbID), OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(container, kRecorderErrorCbIDKey, @(errorCbID), OBJC_ASSOCIATION_RETAIN_NONATOMIC);

    // Record button (circle.fill SF Symbol)
    NSButton *recordBtn = [NSButton buttonWithImage:[NSImage imageWithSystemSymbolName:@"circle.fill" accessibilityDescription:@"Record"]
                                              target:nil
                                              action:nil];
    recordBtn.translatesAutoresizingMaskIntoConstraints = NO;
    recordBtn.bordered = NO;
    recordBtn.bezelStyle = NSBezelStyleInline;
    recordBtn.contentTintColor = [NSColor systemRedColor];
    [recordBtn setContentHuggingPriority:NSLayoutPriorityRequired forOrientation:NSLayoutConstraintOrientationHorizontal];

    // Level indicator
    NSLevelIndicator *levelIndicator = [[NSLevelIndicator alloc] init];
    levelIndicator.translatesAutoresizingMaskIntoConstraints = NO;
    levelIndicator.levelIndicatorStyle = NSLevelIndicatorStyleContinuousCapacity;
    levelIndicator.minValue = 0;
    levelIndicator.maxValue = 1;
    levelIndicator.doubleValue = 0;
    levelIndicator.warningValue = 0.8;
    levelIndicator.criticalValue = 0.95;
    [levelIndicator setContentHuggingPriority:NSLayoutPriorityDefaultLow forOrientation:NSLayoutConstraintOrientationHorizontal];

    // Time label
    NSTextField *timeLabel = [NSTextField labelWithString:@"0:00"];
    timeLabel.translatesAutoresizingMaskIntoConstraints = NO;
    timeLabel.font = [NSFont monospacedDigitSystemFontOfSize:11 weight:NSFontWeightRegular];
    timeLabel.textColor = [NSColor secondaryLabelColor];
    [timeLabel setContentHuggingPriority:NSLayoutPriorityRequired forOrientation:NSLayoutConstraintOrientationHorizontal];
    [timeLabel setContentCompressionResistancePriority:NSLayoutPriorityRequired forOrientation:NSLayoutConstraintOrientationHorizontal];

    [container addSubview:recordBtn];
    [container addSubview:levelIndicator];
    [container addSubview:timeLabel];

    // Layout: [recordBtn]-8-[levelIndicator]-8-[timeLabel]
    [NSLayoutConstraint activateConstraints:@[
        [container.heightAnchor constraintEqualToConstant:40],

        [recordBtn.leadingAnchor constraintEqualToAnchor:container.leadingAnchor constant:8],
        [recordBtn.centerYAnchor constraintEqualToAnchor:container.centerYAnchor],
        [recordBtn.widthAnchor constraintEqualToConstant:24],

        [levelIndicator.leadingAnchor constraintEqualToAnchor:recordBtn.trailingAnchor constant:8],
        [levelIndicator.centerYAnchor constraintEqualToAnchor:container.centerYAnchor],

        [timeLabel.leadingAnchor constraintEqualToAnchor:levelIndicator.trailingAnchor constant:8],
        [timeLabel.trailingAnchor constraintEqualToAnchor:container.trailingAnchor constant:-8],
        [timeLabel.centerYAnchor constraintEqualToAnchor:container.centerYAnchor],
    ]];

    // Store UI refs
    objc_setAssociatedObject(container, kRecorderButtonKey, recordBtn, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(container, kRecorderLevelIndicatorKey, levelIndicator, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(container, kRecorderTimeLabelKey, timeLabel, OBJC_ASSOCIATION_RETAIN_NONATOMIC);

    // Wire up button action
    JVRecordButtonAction *btnAction = [[JVRecordButtonAction alloc] init];
    btnAction.container = container;
    recordBtn.target = btnAction;
    recordBtn.action = @selector(toggleRecord:);
    objc_setAssociatedObject(container, kRecorderButtonActionKey, btnAction, OBJC_ASSOCIATION_RETAIN_NONATOMIC);

    return (__bridge_retained void*)container;
}

void JVUpdateAudioRecorder(void* handle, const char* format, double sampleRate, int channels) {
    if (!handle) return;
    NSView *container = (__bridge NSView*)handle;

    NSString *formatStr = [NSString stringWithUTF8String:format];
    objc_setAssociatedObject(container, kRecorderFormatKey, formatStr, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(container, kRecorderSampleRateKey, @(sampleRate), OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(container, kRecorderChannelsKey, @(channels), OBJC_ASSOCIATION_RETAIN_NONATOMIC);
}

void JVAudioRecorderToggle(void* handle) {
    if (!handle) return;
    NSView *container = (__bridge NSView*)handle;

    AVAudioRecorder *recorder = objc_getAssociatedObject(container, kRecorderKey);
    NSButton *recordBtn = objc_getAssociatedObject(container, kRecorderButtonKey);
    NSTextField *timeLabel = objc_getAssociatedObject(container, kRecorderTimeLabelKey);

    if (recorder && recorder.isRecording) {
        // Stop recording
        NSTimeInterval duration = recorder.currentTime;
        [recorder stop];
        stopMeteringTimer(container);

        recordBtn.contentTintColor = [NSColor systemRedColor];
        recordBtn.image = [NSImage imageWithSystemSymbolName:@"circle.fill" accessibilityDescription:@"Record"];

        NSString *path = objc_getAssociatedObject(container, kRecorderPathKey);
        NSNumber *stoppedCbNum = objc_getAssociatedObject(container, kRecorderStoppedCbIDKey);
        uint64_t stoppedCbID = [stoppedCbNum unsignedLongLongValue];
        if (stoppedCbID != 0 && path) {
            NSString *json = [NSString stringWithFormat:@"{\"path\":\"%@\",\"duration\":%.1f}", path, duration];
            GoCallbackInvoke(stoppedCbID, json.UTF8String);
        }

        // Clear recorder so next toggle creates a new file
        objc_setAssociatedObject(container, kRecorderKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    } else {
        // Check mic permission
        AVAuthorizationStatus status = [AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio];
        if (status == AVAuthorizationStatusDenied || status == AVAuthorizationStatusRestricted) {
            fireError(container, @"Microphone access denied");
            return;
        }
        if (status == AVAuthorizationStatusNotDetermined) {
            [AVCaptureDevice requestAccessForMediaType:AVMediaTypeAudio completionHandler:^(BOOL granted) {
                if (granted) {
                    dispatch_async(dispatch_get_main_queue(), ^{
                        JVAudioRecorderToggle(handle);
                    });
                } else {
                    dispatch_async(dispatch_get_main_queue(), ^{
                        fireError(container, @"Microphone access denied");
                    });
                }
            }];
            return;
        }

        // Create new recorder and start
        recorder = createRecorder(container);
        if (!recorder) return;

        if ([recorder record]) {
            objc_setAssociatedObject(container, kRecorderStartTimeKey, [NSDate date], OBJC_ASSOCIATION_RETAIN_NONATOMIC);
            recordBtn.contentTintColor = [NSColor systemRedColor];
            recordBtn.image = [NSImage imageWithSystemSymbolName:@"stop.fill" accessibilityDescription:@"Stop"];
            timeLabel.stringValue = @"0:00";

            startMeteringTimer(container);

            NSNumber *startedCbNum = objc_getAssociatedObject(container, kRecorderStartedCbIDKey);
            uint64_t startedCbID = [startedCbNum unsignedLongLongValue];
            if (startedCbID != 0) {
                GoCallbackInvoke(startedCbID, "{}");
            }
        } else {
            fireError(container, @"Failed to start recording");
        }
    }
}

void JVCleanupAudioRecorder(void* handle) {
    if (!handle) return;
    NSView *container = (__bridge NSView*)handle;

    stopMeteringTimer(container);

    AVAudioRecorder *recorder = objc_getAssociatedObject(container, kRecorderKey);
    if (recorder && recorder.isRecording) {
        [recorder stop];
    }

    // Break retain cycles
    objc_setAssociatedObject(container, kRecorderKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(container, kRecorderTimerKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(container, kRecorderButtonActionKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
}
