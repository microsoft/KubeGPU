package main

import (
	"github.com/Microsoft/KubeDevice-API/pkg/devicescheduler"
	"github.com/Microsoft/KubeGPU/gpuschedulerplugin"
)

func CreateDeviceSchedulerPlugin() (error, devicescheduler.DeviceScheduler) {
	gpuScheduler := &gpuschedulerplugin.NvidiaGPUScheduler{}
	return nil, gpuScheduler
}
