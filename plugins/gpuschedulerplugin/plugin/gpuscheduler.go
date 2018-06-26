package main

import (
	"github.com/Microsoft/KubeGPU/device-scheduler/types"
	"github.com/Microsoft/KubeGPU/plugins/gpuschedulerplugin"
)

func CreateDeviceSchedulerPlugin() (error, types.DeviceScheduler) {
	gpuScheduler := &gpuschedulerplugin.NvidiaGPUScheduler{}
	return nil, gpuScheduler
}
