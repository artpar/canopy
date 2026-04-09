#import <Cocoa/Cocoa.h>
#import <IOKit/ps/IOPowerSources.h>
#import <IOKit/ps/IOPSKeys.h>
#import <CoreAudio/CoreAudio.h>
#import <mach/mach.h>
#import <mach/processor_info.h>
#import <mach/mach_host.h>
#import <sys/mount.h>
#import <sys/sysctl.h>
#import <ifaddrs.h>
#import <net/if.h>
#import <IOKit/graphics/IOGraphicsLib.h>
#include "sensors.h"

extern void GoSystemEvent(const char* event, const char* data);

// Helper: create and start a dispatch timer
static dispatch_source_t createTimer(int intervalMs, dispatch_block_t handler) {
    dispatch_source_t timer = dispatch_source_create(DISPATCH_SOURCE_TYPE_TIMER, 0, 0, dispatch_get_global_queue(QOS_CLASS_UTILITY, 0));
    uint64_t interval = (uint64_t)intervalMs * NSEC_PER_MSEC;
    dispatch_source_set_timer(timer, dispatch_time(DISPATCH_TIME_NOW, 0), interval, interval / 10);
    dispatch_source_set_event_handler(timer, handler);
    dispatch_resume(timer);
    return timer;
}

// Use JV_STOP_TIMER(varName) macro to cancel and nil a dispatch timer.
// Cannot use a function because ARC disallows passing address of __strong static to __autoreleasing parameter.
#define JV_STOP_TIMER(timer) do { if (timer) { dispatch_source_cancel(timer); timer = nil; } } while(0)

// ============================================================
// TIER 1: Battery
// ============================================================

static dispatch_source_t batteryTimer = nil;

void JVStartBatterySensor(int intervalMs) {
    if (batteryTimer) return;
    if (intervalMs < 1000) intervalMs = 1000;

    batteryTimer = createTimer(intervalMs, ^{
        CFTypeRef blob = IOPSCopyPowerSourcesInfo();
        if (!blob) return;
        CFArrayRef sources = IOPSCopyPowerSourcesList(blob);
        if (!sources || CFArrayGetCount(sources) == 0) {
            if (sources) CFRelease(sources);
            CFRelease(blob);
            // No battery — report plugged in desktop
            GoSystemEvent("system.sensor.battery", "{\"level\":100,\"charging\":false,\"pluggedIn\":true,\"hasBattery\":false}");
            return;
        }

        CFDictionaryRef ps = IOPSGetPowerSourceDescription(blob, CFArrayGetValueAtIndex(sources, 0));
        if (!ps) { CFRelease(sources); CFRelease(blob); return; }

        int currentCap = 0, maxCap = 100;
        BOOL isCharging = NO, pluggedIn = NO;
        int timeRemaining = -1;

        CFNumberRef capRef = CFDictionaryGetValue(ps, CFSTR(kIOPSCurrentCapacityKey));
        if (capRef) CFNumberGetValue(capRef, kCFNumberIntType, &currentCap);
        CFNumberRef maxRef = CFDictionaryGetValue(ps, CFSTR(kIOPSMaxCapacityKey));
        if (maxRef) CFNumberGetValue(maxRef, kCFNumberIntType, &maxCap);

        CFStringRef state = CFDictionaryGetValue(ps, CFSTR(kIOPSPowerSourceStateKey));
        if (state && CFStringCompare(state, CFSTR(kIOPSACPowerValue), 0) == kCFCompareEqualTo) {
            pluggedIn = YES;
        }

        CFBooleanRef chargingRef = CFDictionaryGetValue(ps, CFSTR(kIOPSIsChargingKey));
        if (chargingRef) isCharging = CFBooleanGetValue(chargingRef);

        CFNumberRef timeRef = CFDictionaryGetValue(ps, CFSTR(kIOPSTimeToEmptyKey));
        if (timeRef) CFNumberGetValue(timeRef, kCFNumberIntType, &timeRemaining);

        int level = (maxCap > 0) ? (currentCap * 100 / maxCap) : 0;

        NSString *json = [NSString stringWithFormat:
            @"{\"level\":%d,\"charging\":%s,\"pluggedIn\":%s,\"hasBattery\":true,\"timeRemaining\":%d}",
            level, isCharging ? "true" : "false", pluggedIn ? "true" : "false", timeRemaining];
        GoSystemEvent("system.sensor.battery", [json UTF8String]);

        CFRelease(sources);
        CFRelease(blob);
    });
}

void JVStopBatterySensor(void) { JV_STOP_TIMER(batteryTimer); }

// ============================================================
// TIER 1: Memory
// ============================================================

static dispatch_source_t memoryTimer = nil;

void JVStartMemorySensor(int intervalMs) {
    if (memoryTimer) return;
    if (intervalMs < 500) intervalMs = 500;

    memoryTimer = createTimer(intervalMs, ^{
        vm_size_t pageSize;
        host_page_size(mach_host_self(), &pageSize);

        vm_statistics64_data_t vmStats;
        mach_msg_type_number_t count = HOST_VM_INFO64_COUNT;
        if (host_statistics64(mach_host_self(), HOST_VM_INFO64, (host_info64_t)&vmStats, &count) != KERN_SUCCESS) {
            return;
        }

        uint64_t total = [NSProcessInfo processInfo].physicalMemory;
        uint64_t free = (uint64_t)vmStats.free_count * pageSize;
        uint64_t active = (uint64_t)vmStats.active_count * pageSize;
        uint64_t inactive = (uint64_t)vmStats.inactive_count * pageSize;
        uint64_t wired = (uint64_t)vmStats.wire_count * pageSize;
        uint64_t compressed = (uint64_t)vmStats.compressor_page_count * pageSize;
        uint64_t used = active + wired + compressed;

        NSString *pressure = @"nominal";
        // Approximate memory pressure from ratio
        double usedRatio = (double)used / (double)total;
        if (usedRatio > 0.9) pressure = @"critical";
        else if (usedRatio > 0.8) pressure = @"warning";

        NSString *json = [NSString stringWithFormat:
            @"{\"total\":%llu,\"free\":%llu,\"active\":%llu,\"inactive\":%llu,\"wired\":%llu,\"compressed\":%llu,\"used\":%llu,\"pressure\":\"%@\"}",
            total, free, active, inactive, wired, compressed, used, pressure];
        GoSystemEvent("system.sensor.memory", [json UTF8String]);
    });
}

void JVStopMemorySensor(void) { JV_STOP_TIMER(memoryTimer); }

// ============================================================
// TIER 1: CPU
// ============================================================

static dispatch_source_t cpuTimer = nil;
static processor_info_array_t prevCpuInfo = NULL;
static mach_msg_type_number_t prevCpuInfoCnt = 0;

void JVStartCPUSensor(int intervalMs) {
    if (cpuTimer) return;
    if (intervalMs < 500) intervalMs = 500;

    cpuTimer = createTimer(intervalMs, ^{
        natural_t numCPUs = 0;
        processor_info_array_t cpuInfo;
        mach_msg_type_number_t cpuInfoCnt;

        if (host_processor_info(mach_host_self(), PROCESSOR_CPU_LOAD_INFO, &numCPUs, &cpuInfo, &cpuInfoCnt) != KERN_SUCCESS) {
            return;
        }

        double totalUser = 0, totalSystem = 0, totalIdle = 0;
        NSMutableString *coresJSON = [NSMutableString stringWithString:@"["];

        for (natural_t i = 0; i < numCPUs; i++) {
            double user, system, idle;
            if (prevCpuInfo) {
                user   = cpuInfo[CPU_STATE_MAX * i + CPU_STATE_USER]   - prevCpuInfo[CPU_STATE_MAX * i + CPU_STATE_USER];
                system = cpuInfo[CPU_STATE_MAX * i + CPU_STATE_SYSTEM] - prevCpuInfo[CPU_STATE_MAX * i + CPU_STATE_SYSTEM];
                idle   = cpuInfo[CPU_STATE_MAX * i + CPU_STATE_IDLE]   - prevCpuInfo[CPU_STATE_MAX * i + CPU_STATE_IDLE];
            } else {
                user   = cpuInfo[CPU_STATE_MAX * i + CPU_STATE_USER];
                system = cpuInfo[CPU_STATE_MAX * i + CPU_STATE_SYSTEM];
                idle   = cpuInfo[CPU_STATE_MAX * i + CPU_STATE_IDLE];
            }
            double total = user + system + idle;
            double usage = (total > 0) ? ((user + system) / total * 100.0) : 0.0;

            totalUser += user;
            totalSystem += system;
            totalIdle += idle;

            if (i > 0) [coresJSON appendString:@","];
            [coresJSON appendFormat:@"{\"usage\":%.1f}", usage];
        }
        [coresJSON appendString:@"]"];

        double totalAll = totalUser + totalSystem + totalIdle;
        double overallUsage = (totalAll > 0) ? ((totalUser + totalSystem) / totalAll * 100.0) : 0.0;

        NSString *json = [NSString stringWithFormat:
            @"{\"usage\":%.1f,\"userTime\":%.1f,\"systemTime\":%.1f,\"cores\":%@,\"coreCount\":%u}",
            overallUsage,
            (totalAll > 0) ? (totalUser / totalAll * 100.0) : 0.0,
            (totalAll > 0) ? (totalSystem / totalAll * 100.0) : 0.0,
            coresJSON, numCPUs];
        GoSystemEvent("system.sensor.cpu", [json UTF8String]);

        if (prevCpuInfo) {
            vm_deallocate(mach_task_self(), (vm_address_t)prevCpuInfo, sizeof(integer_t) * prevCpuInfoCnt);
        }
        prevCpuInfo = cpuInfo;
        prevCpuInfoCnt = cpuInfoCnt;
    });
}

void JVStopCPUSensor(void) {
    JV_STOP_TIMER(cpuTimer);
    if (prevCpuInfo) {
        vm_deallocate(mach_task_self(), (vm_address_t)prevCpuInfo, sizeof(integer_t) * prevCpuInfoCnt);
        prevCpuInfo = NULL;
        prevCpuInfoCnt = 0;
    }
}

// ============================================================
// TIER 1: Disk
// ============================================================

static dispatch_source_t diskTimer = nil;

void JVStartDiskSensor(int intervalMs) {
    if (diskTimer) return;
    if (intervalMs < 1000) intervalMs = 1000;

    diskTimer = createTimer(intervalMs, ^{
        struct statfs stat;
        if (statfs("/", &stat) != 0) return;

        uint64_t total = (uint64_t)stat.f_blocks * stat.f_bsize;
        uint64_t free = (uint64_t)stat.f_bavail * stat.f_bsize;
        uint64_t used = total - free;
        double pct = (total > 0) ? ((double)used / (double)total * 100.0) : 0.0;

        NSString *json = [NSString stringWithFormat:
            @"{\"path\":\"/\",\"total\":%llu,\"used\":%llu,\"free\":%llu,\"percentUsed\":%.1f}",
            total, used, free, pct];
        GoSystemEvent("system.sensor.disk", [json UTF8String]);
    });
}

void JVStopDiskSensor(void) { JV_STOP_TIMER(diskTimer); }

// ============================================================
// TIER 2: Network Throughput
// ============================================================

static dispatch_source_t netTimer = nil;
static uint64_t prevBytesIn = 0, prevBytesOut = 0;

void JVStartNetworkThroughputSensor(int intervalMs) {
    if (netTimer) return;
    if (intervalMs < 1000) intervalMs = 1000;

    prevBytesIn = 0;
    prevBytesOut = 0;

    netTimer = createTimer(intervalMs, ^{
        struct ifaddrs *addrs, *cursor;
        if (getifaddrs(&addrs) != 0) return;

        uint64_t totalIn = 0, totalOut = 0;
        for (cursor = addrs; cursor; cursor = cursor->ifa_next) {
            if (cursor->ifa_addr->sa_family != AF_LINK) continue;
            // Skip loopback
            if (cursor->ifa_flags & IFF_LOOPBACK) continue;

            const struct if_data *data = (const struct if_data *)cursor->ifa_data;
            if (data) {
                totalIn += data->ifi_ibytes;
                totalOut += data->ifi_obytes;
            }
        }
        freeifaddrs(addrs);

        uint64_t inPerSec = 0, outPerSec = 0;
        if (prevBytesIn > 0) {
            double secs = (double)intervalMs / 1000.0;
            inPerSec = (uint64_t)((totalIn - prevBytesIn) / secs);
            outPerSec = (uint64_t)((totalOut - prevBytesOut) / secs);
        }
        prevBytesIn = totalIn;
        prevBytesOut = totalOut;

        NSString *json = [NSString stringWithFormat:
            @"{\"bytesIn\":%llu,\"bytesOut\":%llu,\"bytesInPerSec\":%llu,\"bytesOutPerSec\":%llu}",
            totalIn, totalOut, inPerSec, outPerSec];
        GoSystemEvent("system.sensor.network.throughput", [json UTF8String]);
    });
}

void JVStopNetworkThroughputSensor(void) {
    JV_STOP_TIMER(netTimer);
    prevBytesIn = 0;
    prevBytesOut = 0;
}

// ============================================================
// TIER 2: Audio
// ============================================================

static dispatch_source_t audioTimer = nil;

void JVStartAudioSensor(int intervalMs) {
    if (audioTimer) return;
    if (intervalMs < 500) intervalMs = 500;

    audioTimer = createTimer(intervalMs, ^{
        AudioObjectPropertyAddress addr;
        addr.mScope = kAudioObjectPropertyScopeOutput;
        addr.mElement = kAudioObjectPropertyElementMain;

        // Get default output device
        AudioDeviceID outputDevice = kAudioObjectUnknown;
        addr.mSelector = kAudioHardwarePropertyDefaultOutputDevice;
        UInt32 size = sizeof(outputDevice);
        AudioObjectGetPropertyData(kAudioObjectSystemObject, &addr, 0, NULL, &size, &outputDevice);

        if (outputDevice == kAudioObjectUnknown) {
            GoSystemEvent("system.sensor.audio", "{\"outputVolume\":0,\"outputMuted\":true,\"outputDevice\":\"None\"}");
            return;
        }

        // Get volume
        Float32 volume = 0;
        addr.mSelector = kAudioDevicePropertyVolumeScalar;
        addr.mScope = kAudioDevicePropertyScopeOutput;
        size = sizeof(volume);
        AudioObjectGetPropertyData(outputDevice, &addr, 0, NULL, &size, &volume);

        // Get mute
        UInt32 muted = 0;
        addr.mSelector = kAudioDevicePropertyMute;
        size = sizeof(muted);
        AudioObjectGetPropertyData(outputDevice, &addr, 0, NULL, &size, &muted);

        // Get device name
        CFStringRef nameRef = NULL;
        addr.mSelector = kAudioObjectPropertyName;
        addr.mScope = kAudioObjectPropertyScopeGlobal;
        size = sizeof(nameRef);
        AudioObjectGetPropertyData(outputDevice, &addr, 0, NULL, &size, &nameRef);
        NSString *name = nameRef ? (__bridge_transfer NSString *)nameRef : @"Unknown";

        NSString *json = [NSString stringWithFormat:
            @"{\"outputVolume\":%d,\"outputMuted\":%s,\"outputDevice\":\"%@\"}",
            (int)(volume * 100), muted ? "true" : "false", name];
        GoSystemEvent("system.sensor.audio", [json UTF8String]);
    });
}

void JVStopAudioSensor(void) { JV_STOP_TIMER(audioTimer); }

// ============================================================
// TIER 2: Display
// ============================================================

static dispatch_source_t displayTimer = nil;

void JVStartDisplaySensor(int intervalMs) {
    if (displayTimer) return;
    if (intervalMs < 1000) intervalMs = 1000;

    displayTimer = createTimer(intervalMs, ^{
        dispatch_async(dispatch_get_main_queue(), ^{
            NSScreen *main = [NSScreen mainScreen];
            if (!main) return;

            NSRect frame = main.frame;
            CGFloat backingScale = main.backingScaleFactor;

            // Brightness (IOKit)
            float brightness = -1;
            io_iterator_t iterator;
            if (IOServiceGetMatchingServices(kIOMainPortDefault,
                    IOServiceMatching("IODisplayConnect"), &iterator) == kIOReturnSuccess) {
                io_object_t service;
                while ((service = IOIteratorNext(iterator))) {
                    float b;
                    kern_return_t kr = IODisplayGetFloatParameter(service, kNilOptions,
                        CFSTR(kIODisplayBrightnessKey), &b);
                    if (kr == kIOReturnSuccess) {
                        brightness = b * 100.0;
                    }
                    IOObjectRelease(service);
                }
                IOObjectRelease(iterator);
            }

            NSString *json = [NSString stringWithFormat:
                @"{\"width\":%.0f,\"height\":%.0f,\"scale\":%.1f,\"brightness\":%.0f}",
                frame.size.width, frame.size.height, backingScale,
                brightness >= 0 ? brightness : -1.0];
            GoSystemEvent("system.sensor.display", [json UTF8String]);
        });
    });
}

void JVStopDisplaySensor(void) { JV_STOP_TIMER(displayTimer); }

// ============================================================
// TIER 2: Active App
// ============================================================

static dispatch_source_t activeAppTimer = nil;

void JVStartActiveAppSensor(int intervalMs) {
    if (activeAppTimer) return;
    if (intervalMs < 500) intervalMs = 500;

    activeAppTimer = createTimer(intervalMs, ^{
        dispatch_async(dispatch_get_main_queue(), ^{
            NSRunningApplication *app = [[NSWorkspace sharedWorkspace] frontmostApplication];
            if (!app) return;

            NSString *name = app.localizedName ?: @"Unknown";
            NSString *bundleId = app.bundleIdentifier ?: @"";
            pid_t pid = app.processIdentifier;

            // Escape name for JSON
            name = [name stringByReplacingOccurrencesOfString:@"\"" withString:@"\\\""];

            NSString *json = [NSString stringWithFormat:
                @"{\"name\":\"%@\",\"bundleId\":\"%@\",\"pid\":%d}",
                name, bundleId, pid];
            GoSystemEvent("system.sensor.activeApp", [json UTF8String]);
        });
    });
}

void JVStopActiveAppSensor(void) { JV_STOP_TIMER(activeAppTimer); }

// Uptime — uses sysctl, no timer needed (cheap enough to poll)
static dispatch_source_t uptimeTimer = nil;

void JVStartUptimeSensor(int intervalMs) {
    if (uptimeTimer) return;
    if (intervalMs < 1000) intervalMs = 1000;

    uptimeTimer = createTimer(intervalMs, ^{
        struct timeval boottime;
        size_t len = sizeof(boottime);
        int mib[2] = { CTL_KERN, KERN_BOOTTIME };
        if (sysctl(mib, 2, &boottime, &len, NULL, 0) != 0) return;

        time_t now = time(NULL);
        long uptimeSeconds = now - boottime.tv_sec;

        NSString *json = [NSString stringWithFormat:
            @"{\"uptimeSeconds\":%ld,\"bootTimestamp\":%ld}",
            uptimeSeconds, (long)boottime.tv_sec];
        GoSystemEvent("system.sensor.uptime", [json UTF8String]);
    });
}

void JVStopUptimeSensor(void) { JV_STOP_TIMER(uptimeTimer); }

// ============================================================
// TIER 3: Mouse (Global Cursor Position)
// ============================================================

static id mouseMonitor = nil;
static dispatch_source_t mouseTimer = nil;
static double lastMouseX = 0, lastMouseY = 0;

void JVStartMouseSensor(int intervalMs) {
    if (mouseMonitor) return;
    if (intervalMs < 8) intervalMs = 8; // minimum ~120fps

    // Global monitor for mouse moved events (no Accessibility permission needed for observe-only)
    mouseMonitor = [NSEvent addGlobalMonitorForEventsMatchingMask:
        (NSEventMaskMouseMoved | NSEventMaskLeftMouseDragged | NSEventMaskRightMouseDragged | NSEventMaskOtherMouseDragged)
        handler:^(NSEvent *event) {
            NSPoint loc = [NSEvent mouseLocation];
            lastMouseX = loc.x;
            lastMouseY = loc.y;
        }];

    // Also track within our own app windows
    [NSEvent addLocalMonitorForEventsMatchingMask:
        (NSEventMaskMouseMoved | NSEventMaskLeftMouseDragged | NSEventMaskRightMouseDragged)
        handler:^NSEvent *(NSEvent *event) {
            NSPoint loc = [NSEvent mouseLocation];
            lastMouseX = loc.x;
            lastMouseY = loc.y;
            return event;
        }];

    // Fire at the configured interval
    mouseTimer = createTimer(intervalMs, ^{
        NSScreen *main = [NSScreen mainScreen];
        CGFloat screenW = main ? main.frame.size.width : 0;
        CGFloat screenH = main ? main.frame.size.height : 0;
        NSString *json = [NSString stringWithFormat:
            @"{\"x\":%.0f,\"y\":%.0f,\"screenWidth\":%.0f,\"screenHeight\":%.0f}",
            lastMouseX, lastMouseY, screenW, screenH];
        GoSystemEvent("system.sensor.mouse", [json UTF8String]);
    });
}

void JVStopMouseSensor(void) {
    if (mouseMonitor) {
        [NSEvent removeMonitor:mouseMonitor];
        mouseMonitor = nil;
    }
    JV_STOP_TIMER(mouseTimer);
}

// ============================================================
// TIER 3: WiFi
// ============================================================

static dispatch_source_t wifiTimer = nil;

void JVStartWifiSensor(int intervalMs) {
    if (wifiTimer) return;
    if (intervalMs < 2000) intervalMs = 2000;

    wifiTimer = createTimer(intervalMs, ^{
        // CoreWLAN is loaded dynamically to avoid hard link
        Class cwClientClass = NSClassFromString(@"CWWiFiClient");
        if (!cwClientClass) {
            GoSystemEvent("system.sensor.wifi", "{\"error\":\"CoreWLAN not available\"}");
            return;
        }

        id client = [cwClientClass performSelector:@selector(sharedWiFiClient)];
        if (!client) return;

        id iface = [client performSelector:@selector(interface)];
        if (!iface) {
            GoSystemEvent("system.sensor.wifi", "{\"ssid\":null,\"rssi\":0,\"connected\":false}");
            return;
        }

        NSString *ssid = [iface performSelector:@selector(ssid)] ?: @"";
        NSInteger rssi = [[iface valueForKey:@"rssiValue"] integerValue];
        NSInteger noise = [[iface valueForKey:@"noiseMeasurement"] integerValue];
        NSNumber *channelNum = [iface valueForKeyPath:@"wlanChannel.channelNumber"];
        NSInteger channel = channelNum ? [channelNum integerValue] : 0;

        // Escape SSID for JSON
        ssid = [ssid stringByReplacingOccurrencesOfString:@"\"" withString:@"\\\""];

        NSString *json = [NSString stringWithFormat:
            @"{\"ssid\":\"%@\",\"rssi\":%ld,\"channel\":%ld,\"noise\":%ld,\"connected\":%s}",
            ssid, (long)rssi, (long)channel, (long)noise,
            [ssid length] > 0 ? "true" : "false"];
        GoSystemEvent("system.sensor.wifi", [json UTF8String]);
    });
}

void JVStopWifiSensor(void) { JV_STOP_TIMER(wifiTimer); }

// ============================================================
// TIER 3: Processes
// ============================================================

#include <libproc.h>

static dispatch_source_t processesTimer = nil;

void JVStartProcessesSensor(int intervalMs) {
    if (processesTimer) return;
    if (intervalMs < 1000) intervalMs = 1000;

    processesTimer = createTimer(intervalMs, ^{
        // Count running processes
        int count = proc_listpids(PROC_ALL_PIDS, 0, NULL, 0);

        // Load averages
        double loadAvg[3] = {0, 0, 0};
        getloadavg(loadAvg, 3);

        NSString *json = [NSString stringWithFormat:
            @"{\"count\":%d,\"loadAvg1\":%.2f,\"loadAvg5\":%.2f,\"loadAvg15\":%.2f}",
            count, loadAvg[0], loadAvg[1], loadAvg[2]];
        GoSystemEvent("system.sensor.processes", [json UTF8String]);
    });
}

void JVStopProcessesSensor(void) { JV_STOP_TIMER(processesTimer); }

// ============================================================
// TIER 3: Bluetooth Devices
// ============================================================

static dispatch_source_t btDevicesTimer = nil;

void JVStartBluetoothDevicesSensor(int intervalMs) {
    if (btDevicesTimer) return;
    if (intervalMs < 5000) intervalMs = 5000;

    btDevicesTimer = createTimer(intervalMs, ^{
        // Load IOBluetooth dynamically
        Class deviceClass = NSClassFromString(@"IOBluetoothDevice");
        if (!deviceClass) {
            GoSystemEvent("system.sensor.bluetooth.devices", "{\"devices\":[],\"error\":\"IOBluetooth not available\"}");
            return;
        }

        NSArray *paired = [deviceClass performSelector:@selector(pairedDevices)];
        NSMutableString *devicesJSON = [NSMutableString stringWithString:@"["];
        BOOL first = YES;

        for (id device in paired) {
            NSString *name = [device performSelector:@selector(name)] ?: @"Unknown";
            name = [name stringByReplacingOccurrencesOfString:@"\"" withString:@"\\\""];
            BOOL connected = [[device valueForKey:@"isConnected"] boolValue];

            if (!first) [devicesJSON appendString:@","];
            [devicesJSON appendFormat:@"{\"name\":\"%@\",\"connected\":%s}",
                name, connected ? "true" : "false"];
            first = NO;
        }
        [devicesJSON appendString:@"]"];

        NSString *json = [NSString stringWithFormat:@"{\"devices\":%@}", devicesJSON];
        GoSystemEvent("system.sensor.bluetooth.devices", [json UTF8String]);
    });
}

void JVStopBluetoothDevicesSensor(void) { JV_STOP_TIMER(btDevicesTimer); }

// ============================================================
// TIER 3: Disk I/O
// ============================================================

static dispatch_source_t diskIOTimer = nil;
static uint64_t prevDiskRead = 0, prevDiskWrite = 0;

void JVStartDiskIOSensor(int intervalMs) {
    if (diskIOTimer) return;
    if (intervalMs < 1000) intervalMs = 1000;

    prevDiskRead = 0;
    prevDiskWrite = 0;

    diskIOTimer = createTimer(intervalMs, ^{
        // Use getrusage for self I/O (not system-wide, but no root needed)
        struct rusage usage;
        if (getrusage(RUSAGE_SELF, &usage) != 0) return;

        // ru_inblock/ru_oublock are block I/O operations (not bytes)
        uint64_t readBlocks = (uint64_t)usage.ru_inblock;
        uint64_t writeBlocks = (uint64_t)usage.ru_oublock;

        uint64_t readPerSec = 0, writePerSec = 0;
        if (prevDiskRead > 0) {
            double secs = (double)intervalMs / 1000.0;
            readPerSec = (uint64_t)((readBlocks - prevDiskRead) / secs);
            writePerSec = (uint64_t)((writeBlocks - prevDiskWrite) / secs);
        }
        prevDiskRead = readBlocks;
        prevDiskWrite = writeBlocks;

        NSString *json = [NSString stringWithFormat:
            @"{\"readBlocks\":%llu,\"writeBlocks\":%llu,\"readBlocksPerSec\":%llu,\"writeBlocksPerSec\":%llu}",
            readBlocks, writeBlocks, readPerSec, writePerSec];
        GoSystemEvent("system.sensor.diskIO", [json UTF8String]);
    });
}

void JVStopDiskIOSensor(void) {
    JV_STOP_TIMER(diskIOTimer);
    prevDiskRead = 0;
    prevDiskWrite = 0;
}
