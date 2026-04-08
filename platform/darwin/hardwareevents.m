#import <Cocoa/Cocoa.h>
#import <CoreBluetooth/CoreBluetooth.h>
#import <CoreLocation/CoreLocation.h>
#import <IOKit/IOKitLib.h>
#import <IOKit/usb/IOUSBLib.h>
#include "hardwareevents.h"

extern void GoSystemEvent(const char* event, const char* data);
extern void GoHardwareEvent(uint64_t subscriptionID, const char* data);

// --- Bluetooth Observer ---

@interface JVBluetoothDelegate : NSObject <CBCentralManagerDelegate>
@property (nonatomic, strong) CBCentralManager *manager;
@end

@implementation JVBluetoothDelegate

- (void)centralManagerDidUpdateState:(CBCentralManager *)central {
    NSString *state = @"unknown";
    switch (central.state) {
        case CBManagerStatePoweredOn: state = @"poweredOn"; break;
        case CBManagerStatePoweredOff: state = @"poweredOff"; break;
        case CBManagerStateResetting: state = @"resetting"; break;
        case CBManagerStateUnauthorized: state = @"unauthorized"; break;
        case CBManagerStateUnsupported: state = @"unsupported"; break;
        default: break;
    }
    NSString *json = [NSString stringWithFormat:@"{\"state\":\"%@\"}", state];
    GoSystemEvent("system.bluetooth", [json UTF8String]);
}

@end

static JVBluetoothDelegate *bluetoothDelegate = nil;

void JVStartBluetoothObserver(void) {
    if (bluetoothDelegate) return;
    bluetoothDelegate = [[JVBluetoothDelegate alloc] init];
    // Initialize on main queue — state update comes via delegate callback
    bluetoothDelegate.manager = [[CBCentralManager alloc]
        initWithDelegate:bluetoothDelegate
                   queue:dispatch_get_main_queue()
                 options:@{CBCentralManagerOptionShowPowerAlertKey: @NO}];
}

void JVStopBluetoothObserver(void) {
    if (bluetoothDelegate) {
        bluetoothDelegate.manager.delegate = nil;
        bluetoothDelegate.manager = nil;
        bluetoothDelegate = nil;
    }
}

// --- Location Observer ---

@interface JVLocationDelegate : NSObject <CLLocationManagerDelegate>
@property (nonatomic, strong) CLLocationManager *manager;
@end

@implementation JVLocationDelegate

- (void)locationManager:(CLLocationManager *)manager didUpdateLocations:(NSArray<CLLocation *> *)locations {
    CLLocation *loc = locations.lastObject;
    if (!loc) return;
    NSString *json = [NSString stringWithFormat:
        @"{\"latitude\":%.6f,\"longitude\":%.6f,\"altitude\":%.1f,\"accuracy\":%.1f}",
        loc.coordinate.latitude, loc.coordinate.longitude,
        loc.altitude, loc.horizontalAccuracy];
    GoSystemEvent("system.location", [json UTF8String]);
}

- (void)locationManager:(CLLocationManager *)manager didFailWithError:(NSError *)error {
    NSString *json = [NSString stringWithFormat:@"{\"error\":\"%@\"}", error.localizedDescription];
    GoSystemEvent("system.location.error", [json UTF8String]);
}

- (void)locationManagerDidChangeAuthorization:(CLLocationManager *)manager {
    NSString *status = @"unknown";
    switch (manager.authorizationStatus) {
        case kCLAuthorizationStatusAuthorized: status = @"authorized"; break;
        case kCLAuthorizationStatusDenied: status = @"denied"; break;
        case kCLAuthorizationStatusRestricted: status = @"restricted"; break;
        case kCLAuthorizationStatusNotDetermined: status = @"notDetermined"; break;
        default: break;
    }
    NSString *json = [NSString stringWithFormat:@"{\"authorization\":\"%@\"}", status];
    GoSystemEvent("system.location.authorization", [json UTF8String]);
}

@end

static JVLocationDelegate *locationDelegate = nil;

void JVStartLocationObserver(void) {
    if (locationDelegate) return;
    locationDelegate = [[JVLocationDelegate alloc] init];
    locationDelegate.manager = [[CLLocationManager alloc] init];
    locationDelegate.manager.delegate = locationDelegate;
    locationDelegate.manager.desiredAccuracy = kCLLocationAccuracyHundredMeters;
    [locationDelegate.manager startMonitoringSignificantLocationChanges];
}

void JVStopLocationObserver(void) {
    if (locationDelegate) {
        [locationDelegate.manager stopMonitoringSignificantLocationChanges];
        locationDelegate.manager.delegate = nil;
        locationDelegate.manager = nil;
        locationDelegate = nil;
    }
}

// --- USB Observer (IOKit) ---

static IONotificationPortRef usbNotifyPort = NULL;
static io_iterator_t usbAddedIter = 0;
static io_iterator_t usbRemovedIter = 0;

static void usbDeviceAdded(void *refcon, io_iterator_t iterator) {
    io_service_t device;
    while ((device = IOIteratorNext(iterator))) {
        CFStringRef nameRef = NULL;
        IORegistryEntryGetName(device, (char[128]){});

        // Get product name
        nameRef = IORegistryEntryCreateCFProperty(device,
            CFSTR(kUSBProductString), kCFAllocatorDefault, 0);

        NSString *name = @"Unknown";
        if (nameRef) {
            name = (__bridge_transfer NSString *)nameRef;
        }

        // Get vendor/product IDs
        CFNumberRef vendorRef = IORegistryEntryCreateCFProperty(device,
            CFSTR(kUSBVendorID), kCFAllocatorDefault, 0);
        CFNumberRef productRef = IORegistryEntryCreateCFProperty(device,
            CFSTR(kUSBProductID), kCFAllocatorDefault, 0);

        int vendorID = 0, productID = 0;
        if (vendorRef) { CFNumberGetValue(vendorRef, kCFNumberIntType, &vendorID); CFRelease(vendorRef); }
        if (productRef) { CFNumberGetValue(productRef, kCFNumberIntType, &productID); CFRelease(productRef); }

        NSString *json = [NSString stringWithFormat:
            @"{\"action\":\"connected\",\"name\":\"%@\",\"vendorId\":%d,\"productId\":%d}",
            name, vendorID, productID];
        GoSystemEvent("system.usb", [json UTF8String]);

        IOObjectRelease(device);
    }
}

static void usbDeviceRemoved(void *refcon, io_iterator_t iterator) {
    io_service_t device;
    while ((device = IOIteratorNext(iterator))) {
        NSString *json = @"{\"action\":\"disconnected\"}";
        GoSystemEvent("system.usb", [json UTF8String]);
        IOObjectRelease(device);
    }
}

void JVStartUSBObserver(void) {
    if (usbNotifyPort) return;

    usbNotifyPort = IONotificationPortCreate(kIOMasterPortDefault);
    if (!usbNotifyPort) return;

    CFRunLoopSourceRef runLoopSource = IONotificationPortGetRunLoopSource(usbNotifyPort);
    CFRunLoopAddSource(CFRunLoopGetMain(), runLoopSource, kCFRunLoopDefaultMode);

    CFMutableDictionaryRef matchingDict;

    // Watch USB device additions
    matchingDict = IOServiceMatching(kIOUSBDeviceClassName);
    IOServiceAddMatchingNotification(usbNotifyPort,
        kIOFirstMatchNotification, matchingDict,
        usbDeviceAdded, NULL, &usbAddedIter);
    // Drain initial iterator
    usbDeviceAdded(NULL, usbAddedIter);

    // Watch USB device removals
    matchingDict = IOServiceMatching(kIOUSBDeviceClassName);
    IOServiceAddMatchingNotification(usbNotifyPort,
        kIOTerminatedNotification, matchingDict,
        usbDeviceRemoved, NULL, &usbRemovedIter);
    usbDeviceRemoved(NULL, usbRemovedIter);
}

void JVStopUSBObserver(void) {
    if (usbNotifyPort) {
        CFRunLoopSourceRef runLoopSource = IONotificationPortGetRunLoopSource(usbNotifyPort);
        CFRunLoopRemoveSource(CFRunLoopGetMain(), runLoopSource, kCFRunLoopDefaultMode);
        IONotificationPortDestroy(usbNotifyPort);
        usbNotifyPort = NULL;
    }
    if (usbAddedIter) { IOObjectRelease(usbAddedIter); usbAddedIter = 0; }
    if (usbRemovedIter) { IOObjectRelease(usbRemovedIter); usbRemovedIter = 0; }
}

// --- Distributed Notifications ---

// Map from subscriptionID → observer token (for removal)
static NSMutableDictionary<NSNumber*, id> *distNotifObservers = nil;

void JVObserveDistributedNotification(const char* name, uint64_t subscriptionID) {
    if (!distNotifObservers) {
        distNotifObservers = [[NSMutableDictionary alloc] init];
    }

    NSString *notifName = [NSString stringWithUTF8String:name];
    NSNumber *key = @(subscriptionID);

    // Remove existing observer for this ID
    id existing = distNotifObservers[key];
    if (existing) {
        [[NSDistributedNotificationCenter defaultCenter] removeObserver:existing];
    }

    id observer = [[NSDistributedNotificationCenter defaultCenter]
        addObserverForName:notifName
                    object:nil
                     queue:[NSOperationQueue mainQueue]
                usingBlock:^(NSNotification * _Nonnull note) {
        // Serialize userInfo to JSON if present
        NSString *userInfoJSON = @"{}";
        if (note.userInfo) {
            NSData *data = [NSJSONSerialization dataWithJSONObject:note.userInfo options:0 error:nil];
            if (data) {
                userInfoJSON = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
            }
        }
        NSString *json = [NSString stringWithFormat:
            @"{\"name\":\"%@\",\"object\":\"%@\",\"userInfo\":%@}",
            note.name, note.object ?: @"", userInfoJSON];
        GoHardwareEvent(subscriptionID, [json UTF8String]);
    }];

    distNotifObservers[key] = observer;
}

void JVUnobserveDistributedNotification(uint64_t subscriptionID) {
    if (!distNotifObservers) return;
    NSNumber *key = @(subscriptionID);
    id observer = distNotifObservers[key];
    if (observer) {
        [[NSDistributedNotificationCenter defaultCenter] removeObserver:observer];
        [distNotifObservers removeObjectForKey:key];
    }
}
