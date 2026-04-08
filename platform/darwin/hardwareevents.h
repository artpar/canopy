#ifndef JVIEW_HARDWAREEVENTS_H
#define JVIEW_HARDWAREEVENTS_H

#include <stdint.h>

// Bluetooth: Start/stop monitoring for Bluetooth state changes.
// Requires CoreBluetooth framework and NSBluetoothAlwaysUsageDescription.
void JVStartBluetoothObserver(void);
void JVStopBluetoothObserver(void);

// Location: Start/stop monitoring significant location changes.
// Requires CoreLocation framework and NSLocationUsageDescription.
void JVStartLocationObserver(void);
void JVStopLocationObserver(void);

// USB: Start/stop monitoring USB device connect/disconnect via IOKit.
void JVStartUSBObserver(void);
void JVStopUSBObserver(void);

// Distributed Notifications: observe/unobserve a named notification from other apps.
void JVObserveDistributedNotification(const char* name, uint64_t subscriptionID);
void JVUnobserveDistributedNotification(uint64_t subscriptionID);

#endif
