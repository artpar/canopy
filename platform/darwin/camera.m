#import <Cocoa/Cocoa.h>
#import <AVFoundation/AVFoundation.h>
#import <CoreMedia/CoreMedia.h>
#include "camera.h"
#import <objc/runtime.h>

extern void GoCallbackInvoke(uint64_t callbackID, const char* data);

static const void *kCameraSessionKey = &kCameraSessionKey;
static const void *kCameraPreviewLayerKey = &kCameraPreviewLayerKey;
static const void *kCameraPhotoOutputKey = &kCameraPhotoOutputKey;
static const void *kCameraCaptureCbIDKey = &kCameraCaptureCbIDKey;
static const void *kCameraErrorCbIDKey = &kCameraErrorCbIDKey;
static const void *kCameraSessionQueueKey = &kCameraSessionQueueKey;
static const void *kCameraDelegateKey = &kCameraDelegateKey;

// Photo capture delegate
@interface JVPhotoCaptureDelegate : NSObject <AVCapturePhotoCaptureDelegate>
@property (nonatomic, assign) uint64_t captureCbID;
@property (nonatomic, assign) uint64_t errorCbID;
@end

@implementation JVPhotoCaptureDelegate

- (void)captureOutput:(AVCapturePhotoOutput *)output
    didFinishProcessingPhoto:(AVCapturePhoto *)photo
                       error:(NSError *)error {
    if (error) {
        if (self.errorCbID != 0) {
            NSString *msg = [NSString stringWithFormat:@"{\"error\":\"%@\"}", error.localizedDescription];
            GoCallbackInvoke(self.errorCbID, msg.UTF8String);
        }
        return;
    }

    NSData *imageData = [photo fileDataRepresentation];
    if (!imageData) {
        if (self.errorCbID != 0) {
            GoCallbackInvoke(self.errorCbID, "{\"error\":\"Failed to get image data\"}");
        }
        return;
    }

    // Save to temp file
    NSString *timestamp = [NSString stringWithFormat:@"%lld", (long long)([[NSDate date] timeIntervalSince1970] * 1000)];
    NSString *filename = [NSString stringWithFormat:@"canopy_photo_%@.jpg", timestamp];
    NSString *path = [NSTemporaryDirectory() stringByAppendingPathComponent:filename];

    if ([imageData writeToFile:path atomically:YES]) {
        if (self.captureCbID != 0) {
            NSString *json = [NSString stringWithFormat:@"{\"path\":\"%@\"}", path];
            GoCallbackInvoke(self.captureCbID, json.UTF8String);
        }
    } else {
        if (self.errorCbID != 0) {
            GoCallbackInvoke(self.errorCbID, "{\"error\":\"Failed to write photo to disk\"}");
        }
    }
}

@end

static void fireError(uint64_t errorCbID, NSString *msg) {
    if (errorCbID == 0) return;
    NSString *json = [NSString stringWithFormat:@"{\"error\":\"%@\"}", msg];
    GoCallbackInvoke(errorCbID, json.UTF8String);
}

static AVCaptureDevicePosition positionFromString(const char* pos) {
    if (pos && strcmp(pos, "back") == 0) {
        return AVCaptureDevicePositionBack;
    }
    return AVCaptureDevicePositionFront;
}

static void startSession(NSView *container, const char* devicePosition, bool mirrored, uint64_t captureCbID, uint64_t errorCbID) {
    dispatch_queue_t sessionQueue = dispatch_queue_create("com.canopy.camera", DISPATCH_QUEUE_SERIAL);
    objc_setAssociatedObject(container, kCameraSessionQueueKey, sessionQueue, OBJC_ASSOCIATION_RETAIN_NONATOMIC);

    AVCaptureDevicePosition pos = positionFromString(devicePosition);

    dispatch_async(sessionQueue, ^{
        // Check permission
        AVAuthorizationStatus status = [AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeVideo];
        if (status == AVAuthorizationStatusDenied || status == AVAuthorizationStatusRestricted) {
            dispatch_async(dispatch_get_main_queue(), ^{
                fireError(errorCbID, @"Camera access denied");
            });
            return;
        }

        if (status == AVAuthorizationStatusNotDetermined) {
            dispatch_semaphore_t sema = dispatch_semaphore_create(0);
            __block BOOL granted = NO;
            [AVCaptureDevice requestAccessForMediaType:AVMediaTypeVideo completionHandler:^(BOOL g) {
                granted = g;
                dispatch_semaphore_signal(sema);
            }];
            dispatch_semaphore_wait(sema, DISPATCH_TIME_FOREVER);
            if (!granted) {
                dispatch_async(dispatch_get_main_queue(), ^{
                    fireError(errorCbID, @"Camera access denied");
                });
                return;
            }
        }

        // Find device
        AVCaptureDevice *device = nil;
        AVCaptureDeviceDiscoverySession *discovery = [AVCaptureDeviceDiscoverySession
            discoverySessionWithDeviceTypes:@[AVCaptureDeviceTypeBuiltInWideAngleCamera]
                                  mediaType:AVMediaTypeVideo
                                   position:pos];
        if (discovery.devices.count > 0) {
            device = discovery.devices.firstObject;
        }
        if (!device) {
            device = [AVCaptureDevice defaultDeviceWithMediaType:AVMediaTypeVideo];
        }
        if (!device) {
            dispatch_async(dispatch_get_main_queue(), ^{
                fireError(errorCbID, @"No camera device found");
            });
            return;
        }

        // Create session
        AVCaptureSession *session = [[AVCaptureSession alloc] init];
        session.sessionPreset = AVCaptureSessionPresetHigh;

        NSError *inputError = nil;
        AVCaptureDeviceInput *input = [AVCaptureDeviceInput deviceInputWithDevice:device error:&inputError];
        if (!input) {
            dispatch_async(dispatch_get_main_queue(), ^{
                fireError(errorCbID, inputError.localizedDescription ?: @"Failed to create camera input");
            });
            return;
        }

        if ([session canAddInput:input]) {
            [session addInput:input];
        }

        // Photo output
        AVCapturePhotoOutput *photoOutput = [[AVCapturePhotoOutput alloc] init];
        if ([session canAddOutput:photoOutput]) {
            [session addOutput:photoOutput];
        }

        // Start session
        [session startRunning];

        dispatch_async(dispatch_get_main_queue(), ^{
            // Create preview layer
            AVCaptureVideoPreviewLayer *previewLayer = [AVCaptureVideoPreviewLayer layerWithSession:session];
            previewLayer.videoGravity = AVLayerVideoGravityResizeAspectFill;
            previewLayer.frame = container.bounds;
            previewLayer.autoresizingMask = kCALayerWidthSizable | kCALayerHeightSizable;

            // Mirror
            if (previewLayer.connection) {
                previewLayer.connection.automaticallyAdjustsVideoMirroring = NO;
                previewLayer.connection.videoMirrored = mirrored;
            }

            container.wantsLayer = YES;
            [container.layer addSublayer:previewLayer];

            // Store references
            objc_setAssociatedObject(container, kCameraSessionKey, session, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
            objc_setAssociatedObject(container, kCameraPreviewLayerKey, previewLayer, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
            objc_setAssociatedObject(container, kCameraPhotoOutputKey, photoOutput, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
            objc_setAssociatedObject(container, kCameraCaptureCbIDKey, @(captureCbID), OBJC_ASSOCIATION_RETAIN_NONATOMIC);
            objc_setAssociatedObject(container, kCameraErrorCbIDKey, @(errorCbID), OBJC_ASSOCIATION_RETAIN_NONATOMIC);
        });
    });
}

void* JVCreateCamera(const char* devicePosition, bool mirrored, uint64_t captureCbID, uint64_t errorCbID) {
    NSView *container = [[NSView alloc] init];
    container.translatesAutoresizingMaskIntoConstraints = NO;
    container.wantsLayer = YES;
    container.layer.backgroundColor = [NSColor blackColor].CGColor;

    startSession(container, devicePosition, mirrored, captureCbID, errorCbID);

    return (__bridge_retained void*)container;
}

void JVUpdateCamera(void* handle, const char* devicePosition, bool mirrored) {
    if (!handle) return;
    NSView *container = (__bridge NSView*)handle;

    // Update mirror
    AVCaptureVideoPreviewLayer *previewLayer = objc_getAssociatedObject(container, kCameraPreviewLayerKey);
    if (previewLayer && previewLayer.connection) {
        previewLayer.connection.automaticallyAdjustsVideoMirroring = NO;
        previewLayer.connection.videoMirrored = mirrored;
    }

    // Update preview layer frame
    if (previewLayer) {
        previewLayer.frame = container.bounds;
    }
}

void JVCameraCapture(void* handle) {
    if (!handle) return;
    NSView *container = (__bridge NSView*)handle;

    AVCapturePhotoOutput *photoOutput = objc_getAssociatedObject(container, kCameraPhotoOutputKey);
    if (!photoOutput) return;

    NSNumber *captureCbNum = objc_getAssociatedObject(container, kCameraCaptureCbIDKey);
    NSNumber *errorCbNum = objc_getAssociatedObject(container, kCameraErrorCbIDKey);

    JVPhotoCaptureDelegate *delegate = [[JVPhotoCaptureDelegate alloc] init];
    delegate.captureCbID = [captureCbNum unsignedLongLongValue];
    delegate.errorCbID = [errorCbNum unsignedLongLongValue];

    // Keep delegate alive until capture completes
    objc_setAssociatedObject(container, kCameraDelegateKey, delegate, OBJC_ASSOCIATION_RETAIN_NONATOMIC);

    AVCapturePhotoSettings *settings = [AVCapturePhotoSettings photoSettings];
    [photoOutput capturePhotoWithSettings:settings delegate:delegate];
}

void JVCleanupCamera(void* handle) {
    if (!handle) return;
    NSView *container = (__bridge NSView*)handle;

    AVCaptureVideoPreviewLayer *previewLayer = objc_getAssociatedObject(container, kCameraPreviewLayerKey);
    if (previewLayer) {
        [previewLayer removeFromSuperlayer];
    }

    dispatch_queue_t sessionQueue = objc_getAssociatedObject(container, kCameraSessionQueueKey);
    AVCaptureSession *session = objc_getAssociatedObject(container, kCameraSessionKey);

    if (session && sessionQueue) {
        dispatch_sync(sessionQueue, ^{
            for (AVCaptureInput *input in session.inputs) { [session removeInput:input]; }
            for (AVCaptureOutput *output in session.outputs) { [session removeOutput:output]; }
            [session stopRunning];
        });
    }

    // Break retain cycles
    objc_setAssociatedObject(container, kCameraSessionKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(container, kCameraPreviewLayerKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(container, kCameraPhotoOutputKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(container, kCameraSessionQueueKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(container, kCameraDelegateKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
}
