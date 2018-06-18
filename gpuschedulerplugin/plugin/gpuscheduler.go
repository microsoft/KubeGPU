package main

import (
	"github.com/Microsoft/KubeGPU/device-scheduler/types"
	"github.com/Microsoft/KubeGPU/gpuschedulerplugin"
)

func CreateDeviceScheduler() (error, types.DeviceScheduler) {
	gpuScheduler := &gpuschedulerplugin.NvidiaGPUScheduler{}
	return nil, gpuScheduler
}
