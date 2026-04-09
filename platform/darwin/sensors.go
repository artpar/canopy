package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework IOKit -framework CoreAudio

#include "sensors.h"
*/
import "C"

// Tier 1 sensors

func StartBatterySensor(intervalMs int)  { C.JVStartBatterySensor(C.int(intervalMs)) }
func StopBatterySensor()                 { C.JVStopBatterySensor() }
func StartMemorySensor(intervalMs int)   { C.JVStartMemorySensor(C.int(intervalMs)) }
func StopMemorySensor()                  { C.JVStopMemorySensor() }
func StartCPUSensor(intervalMs int)      { C.JVStartCPUSensor(C.int(intervalMs)) }
func StopCPUSensor()                     { C.JVStopCPUSensor() }
func StartDiskSensor(intervalMs int)     { C.JVStartDiskSensor(C.int(intervalMs)) }
func StopDiskSensor()                    { C.JVStopDiskSensor() }
func StartUptimeSensor(intervalMs int)   { C.JVStartUptimeSensor(C.int(intervalMs)) }
func StopUptimeSensor()                  { C.JVStopUptimeSensor() }

// Tier 2 sensors

func StartNetworkThroughputSensor(intervalMs int) { C.JVStartNetworkThroughputSensor(C.int(intervalMs)) }
func StopNetworkThroughputSensor()                { C.JVStopNetworkThroughputSensor() }
func StartAudioSensor(intervalMs int)             { C.JVStartAudioSensor(C.int(intervalMs)) }
func StopAudioSensor()                            { C.JVStopAudioSensor() }
func StartDisplaySensor(intervalMs int)           { C.JVStartDisplaySensor(C.int(intervalMs)) }
func StopDisplaySensor()                          { C.JVStopDisplaySensor() }
func StartActiveAppSensor(intervalMs int)         { C.JVStartActiveAppSensor(C.int(intervalMs)) }
func StopActiveAppSensor()                        { C.JVStopActiveAppSensor() }
